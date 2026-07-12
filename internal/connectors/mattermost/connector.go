// Package mattermost provides a Mattermost connector for SoulGate.
//
// It bridges Mattermost to the SoulGate HTTP /api/chat endpoint using the
// Mattermost WebSocket API for real-time event delivery and the Mattermost
// REST API v4 for posting replies — no external SDK required.
//
// Architecture:
//
//	Connector connects to wss://<server>/api/v4/websocket
//	    -> Authenticates via "authentication_challenge" message
//	    -> Listens for "posted" events
//	    -> Parses the post JSON from event data
//	    -> Filters: ignores own messages, only handles DMs or @mentions
//	    -> POSTs message to SoulGate /api/chat
//	    -> POSTs reply via POST /api/v4/posts (threaded via root_id)
//
// The connector uses gorilla/websocket, which is already a project dependency.
//
// References:
//
//	https://api.mattermost.com/
//	https://developers.mattermost.com/integrate/reference/websocket/
package mattermost

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultGatewayURL       = "http://localhost:8080"
	mattermostMaxMessageLen = 16383   // Mattermost post character limit
	wsReadLimit             = 1 << 20 // 1 MiB per WebSocket message
	wsReconnectDelay        = 5 * time.Second
	wsPingInterval          = 30 * time.Second
)

// Config holds all configuration for the Mattermost connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// ServerURL is the base URL of the Mattermost server.
	// Example: https://mattermost.example.com
	ServerURL string

	// Token is the personal access token or bot token used to authenticate
	// with the Mattermost REST API and WebSocket API.
	Token string

	// BotUsername is the bot's Mattermost username, used to filter out the
	// bot's own messages and detect @mentions.
	BotUsername string
}

// wsEvent is a Mattermost WebSocket event envelope.
type wsEvent struct {
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data"`
	Broadcast struct {
		ChannelID string `json:"channel_id"`
		TeamID    string `json:"team_id"`
		UserID    string `json:"user_id"`
	} `json:"broadcast"`
	Seq int64 `json:"seq"`
}

// wsAuthChallenge is the auth challenge message sent to the WebSocket.
type wsAuthChallenge struct {
	Seq    int    `json:"seq"`
	Action string `json:"action"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

// postedEventData is the "data" field in a "posted" WebSocket event.
type postedEventData struct {
	ChannelType string `json:"channel_type"` // D = direct, O = public, P = private
	Post        string `json:"post"`         // JSON-encoded post object
	SenderName  string `json:"sender_name"`  // poster's username (with @)
	TeamID      string `json:"team_id"`
}

// post is the Mattermost Post object (subset of fields we use).
type post struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"` // non-empty if this is a threaded reply
	Type      string `json:"type"`    // "" = normal user post; others are system messages
}

// createPostRequest is the body for POST /api/v4/posts.
type createPostRequest struct {
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id,omitempty"` // thread root
}

// gatewayRequest is the payload sent to POST /api/chat.
type gatewayRequest struct {
	Message string `json:"message"`
}

// gatewayResponse is the payload returned from POST /api/chat.
type gatewayResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// mattermostUser holds the bot's own user record fetched at startup.
type mattermostUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// Connector bridges Mattermost to the SoulGate HTTP API.
type Connector struct {
	config     Config
	httpClient *http.Client

	// botUserID is resolved from the API at startup.
	botUserID string

	// mu guards ws and seq.
	mu  sync.Mutex
	ws  *websocket.Conn
	seq int
}

// New creates a new Mattermost connector.  It validates the configuration but
// does not yet open any network connections — call Start to connect.
func New(config Config) (*Connector, error) {
	if config.ServerURL == "" {
		return nil, fmt.Errorf("mattermost: ServerURL is required")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("mattermost: Token is required")
	}
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}

	// Strip trailing slash from ServerURL for consistent URL construction.
	config.ServerURL = strings.TrimRight(config.ServerURL, "/")

	return &Connector{
		config: config,
		// No timeout on gateway calls — agentic loops can take minutes.
		httpClient: &http.Client{Timeout: 0},
	}, nil
}

// Start connects to the Mattermost WebSocket API and blocks until ctx is
// cancelled.  If the connection drops it reconnects automatically.
func (c *Connector) Start(ctx context.Context) error {
	// Resolve bot user ID once at startup.
	if err := c.resolveBotUser(ctx); err != nil {
		return fmt.Errorf("mattermost: failed to resolve bot user: %w", err)
	}

	log.Printf("mattermost connector: bot user: %s (id=%s)", c.config.BotUsername, c.botUserID)
	log.Printf("mattermost connector: gateway URL: %s", c.config.GatewayURL)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if err := c.runWebSocket(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("mattermost connector: WebSocket error: %v — reconnecting in %s", err, wsReconnectDelay)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wsReconnectDelay):
			}
		}
	}
}

// runWebSocket establishes a single WebSocket connection and runs the receive
// loop.  It returns when the connection is lost or ctx is cancelled.
func (c *Connector) runWebSocket(ctx context.Context) error {
	wsURL, err := buildWebSocketURL(c.config.ServerURL)
	if err != nil {
		return fmt.Errorf("build WebSocket URL: %w", err)
	}

	dialer := websocket.Dialer{
		// Allow self-signed certs if the server URL uses https with custom certs.
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: false}, //nolint:gosec
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, http.Header{
		"Authorization": []string{"Bearer " + c.config.Token},
	})
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	conn.SetReadLimit(wsReadLimit)

	c.mu.Lock()
	c.ws = conn
	c.seq = 1
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.ws = nil
		c.mu.Unlock()
		conn.Close()
	}()

	log.Printf("mattermost connector: connected to %s", wsURL)

	// Authenticate via the WebSocket challenge.
	if err := c.sendAuthChallenge(conn); err != nil {
		return fmt.Errorf("auth challenge: %w", err)
	}

	// Keep-alive ping loop.
	go c.pingLoop(ctx, conn)

	// Receive loop.
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}

		var evt wsEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			log.Printf("mattermost connector: failed to unmarshal event: %v", err)
			continue
		}

		if evt.Event == "posted" {
			go c.handlePostedEvent(ctx, evt)
		}
	}
}

// sendAuthChallenge sends the authentication_challenge WebSocket action.
func (c *Connector) sendAuthChallenge(conn *websocket.Conn) error {
	c.mu.Lock()
	seq := c.seq
	c.seq++
	c.mu.Unlock()

	msg := wsAuthChallenge{
		Seq:    seq,
		Action: "authentication_challenge",
	}
	msg.Data.Token = c.config.Token

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal auth challenge: %w", err)
	}

	c.mu.Lock()
	err = conn.WriteMessage(websocket.TextMessage, payload)
	c.mu.Unlock()
	return err
}

// pingLoop sends periodic WebSocket pings to keep the connection alive.
func (c *Connector) pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(wsPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			err := conn.WriteMessage(websocket.PingMessage, nil)
			c.mu.Unlock()
			if err != nil {
				log.Printf("mattermost connector: ping failed: %v", err)
				return
			}
		}
	}
}

// handlePostedEvent processes a "posted" WebSocket event.
func (c *Connector) handlePostedEvent(ctx context.Context, evt wsEvent) {
	var data postedEventData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		log.Printf("mattermost connector: failed to parse posted event data: %v", err)
		return
	}

	// Decode the nested post JSON.
	var p post
	if err := json.Unmarshal([]byte(data.Post), &p); err != nil {
		log.Printf("mattermost connector: failed to parse post JSON: %v", err)
		return
	}

	// Ignore system messages (join/leave notices, etc.).
	if p.Type != "" {
		return
	}

	// Ignore our own messages to prevent echo loops.
	if p.UserID == c.botUserID {
		return
	}

	// Determine whether this message warrants a response:
	// - Direct messages (channel type "D") always get a response.
	// - Group messages / channel posts only get a response on @mention.
	isDirect := data.ChannelType == "D"
	isMention := c.isMentioned(p.Message)

	if !isDirect && !isMention {
		return
	}

	// Strip the @mention so the model sees clean text.
	text := stripMention(p.Message, c.config.BotUsername)
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	log.Printf("mattermost connector: message in channel=%s (type=%s): %q", p.ChannelID, data.ChannelType, text)

	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("mattermost connector: gateway error: %v", err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	// Determine thread root: if the incoming post is already in a thread, reply
	// to the same root; otherwise use the post's own ID as the root.
	rootID := p.RootID
	if rootID == "" {
		rootID = p.ID
	}

	chunks := splitMessage(response, mattermostMaxMessageLen)
	for i, chunk := range chunks {
		// Only thread the first chunk; subsequent chunks flow naturally in the thread.
		rid := rootID
		if i > 0 {
			rid = rootID // keep all chunks in the same thread
		}
		if err := c.createPost(ctx, p.ChannelID, rid, chunk); err != nil {
			log.Printf("mattermost connector: failed to send reply chunk %d/%d: %v", i+1, len(chunks), err)
		}
	}
}

// isMentioned returns true if the message contains a @mention of the bot.
func (c *Connector) isMentioned(message string) bool {
	if c.config.BotUsername == "" {
		return false
	}
	mention := "@" + c.config.BotUsername
	return strings.Contains(strings.ToLower(message), strings.ToLower(mention))
}

// sendToGateway POSTs the message to the SoulGate /api/chat endpoint and
// returns the text response.
func (c *Connector) sendToGateway(message string) (string, error) {
	payload, err := json.Marshal(gatewayRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	apiURL := strings.TrimRight(c.config.GatewayURL, "/") + "/api/chat"
	resp, err := c.httpClient.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	var gwResp gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gwResp); err != nil {
		return "", fmt.Errorf("decode gateway response (HTTP %d): %w", resp.StatusCode, err)
	}

	if gwResp.Error != "" {
		return "", fmt.Errorf("gateway error: %s", gwResp.Error)
	}
	if gwResp.Response == "" {
		return "", fmt.Errorf("gateway returned empty response (HTTP %d)", resp.StatusCode)
	}

	log.Printf("mattermost connector: gateway response: %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// createPost sends a new post to a Mattermost channel via the REST API.
// Setting rootID to a non-empty value creates a threaded reply.
func (c *Connector) createPost(ctx context.Context, channelID, rootID, message string) error {
	body := createPostRequest{
		ChannelID: channelID,
		Message:   message,
		RootID:    rootID,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal post request: %w", err)
	}

	apiURL := c.config.ServerURL + "/api/v4/posts"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	// Use a short-timeout client for outbound REST API calls.
	outClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := outClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Mattermost API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// resolveBotUser fetches the bot's own user record from the Mattermost API and
// sets c.botUserID.  If BotUsername is empty the /api/v4/users/me endpoint is
// used to discover both the ID and username.
func (c *Connector) resolveBotUser(ctx context.Context) error {
	apiURL := c.config.ServerURL + "/api/v4/users/me"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("create GET request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	shortClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := shortClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET /api/v4/users/me returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var user mattermostUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return fmt.Errorf("decode user response: %w", err)
	}
	if user.ID == "" {
		return fmt.Errorf("Mattermost returned empty user ID")
	}

	c.botUserID = user.ID

	// Back-fill BotUsername from the API if the caller did not configure it.
	if c.config.BotUsername == "" {
		c.config.BotUsername = user.Username
	}

	return nil
}

// buildWebSocketURL converts an HTTP(S) server URL to a ws(s):// WebSocket URL
// for the Mattermost WebSocket endpoint.
func buildWebSocketURL(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("parse server URL: %w", err)
	}

	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	case "wss", "ws":
		// Already a WebSocket scheme — leave as-is.
	default:
		return "", fmt.Errorf("unsupported scheme %q in ServerURL (expected http or https)", u.Scheme)
	}

	u.Path = "/api/v4/websocket"
	return u.String(), nil
}

// stripMention removes @botUsername mentions from the message text so the
// model receives clean input.
func stripMention(message, botUsername string) string {
	if botUsername == "" {
		return message
	}
	mention := "@" + botUsername
	// Case-insensitive replacement.
	lower := strings.ToLower(message)
	lowerMention := strings.ToLower(mention)

	var result strings.Builder
	remaining := message
	lowerRemaining := lower

	for {
		idx := strings.Index(lowerRemaining, lowerMention)
		if idx < 0 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:idx])
		remaining = remaining[idx+len(mention):]
		lowerRemaining = lowerRemaining[idx+len(lowerMention):]
	}

	return result.String()
}

// splitMessage divides text into chunks of at most maxLen bytes, preferring to
// break on paragraph or line boundaries to avoid cutting sentences mid-word.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		chunk := text[:maxLen]
		// Prefer a double-newline (paragraph) break.
		idx := strings.LastIndex(chunk, "\n\n")
		if idx <= 0 {
			idx = strings.LastIndex(chunk, "\n")
		}
		if idx <= 0 {
			idx = strings.LastIndex(chunk, " ")
		}
		if idx <= 0 {
			idx = maxLen
		}

		chunks = append(chunks, strings.TrimRight(text[:idx], " "))
		text = strings.TrimLeft(text[idx:], " \n")
	}
	return chunks
}
