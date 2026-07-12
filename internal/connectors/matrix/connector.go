// Package matrix provides a Matrix connector for SoulGate.
//
// It bridges Matrix to the SoulGate HTTP /api/chat endpoint using the Matrix
// Client-Server API (CS API) directly over HTTP/JSON — no SDK required.
//
// Architecture:
//
//	Connector long-polls /_matrix/client/v3/sync?since=<token>
//	    -> Filters for m.room.message events
//	    -> Skips own messages (sender == UserID)
//	    -> POSTs message to SoulGate /api/chat
//	    -> PUTs m.room.message event back to homeserver
//
// The "since" token is persisted to a local file so the connector does not
// re-process old messages after a restart.
//
// Supported message types: m.text.  Other types (m.image, m.file, etc.) are
// silently ignored.
//
// References:
//
//	https://spec.matrix.org/latest/client-server-api/
package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultGatewayURL   = "http://localhost:8080"
	syncTimeout         = 30000 // milliseconds — long-poll window
	syncRetryDelay      = 5 * time.Second
	matrixMaxMessageLen = 16384 // practical limit for a single Matrix message body
	sinceTokenFilename  = ".matrix_since_token"
)

// Config holds all configuration for the Matrix connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// HomeserverURL is the base URL of the Matrix homeserver (CS API root).
	// Example: https://matrix.org
	HomeserverURL string

	// AccessToken is the Matrix access token for the bot account.
	// Obtain one via POST /_matrix/client/v3/login or from Element's
	// Settings > Help & About > Access Token.
	AccessToken string

	// UserID is the full Matrix user ID of the bot account.
	// Example: @mybot:matrix.org
	UserID string

	// SinceTokenPath is the file path used to persist the /sync since token
	// across restarts.  Defaults to .matrix_since_token in the current directory.
	SinceTokenPath string
}

// syncResponse is a partial representation of the /_matrix/client/v3/sync
// response body.  We only decode the fields we need.
type syncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]joinedRoom `json:"join"`
	} `json:"rooms"`
}

// joinedRoom contains the timeline events for a room the bot has joined.
type joinedRoom struct {
	Timeline struct {
		Events []timelineEvent `json:"events"`
	} `json:"timeline"`
}

// timelineEvent represents a single event in a room timeline.
type timelineEvent struct {
	Type    string          `json:"type"`
	EventID string          `json:"event_id"`
	Sender  string          `json:"sender"`
	Content json.RawMessage `json:"content"`
}

// messageContent is the content of an m.room.message event.
type messageContent struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
}

// sendEventBody is the body of a PUT m.room.message send request.
type sendEventBody struct {
	MsgType string `json:"msgtype"`
	Body    string `json:"body"`
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

// Connector bridges Matrix to the SoulGate HTTP API via the Matrix CS API.
type Connector struct {
	config     Config
	httpClient *http.Client

	// txnCounter provides unique transaction IDs for PUT /send requests.
	txnCounter atomic.Uint64
}

// New creates a new Matrix connector.  It validates the configuration but does
// not yet connect to the homeserver — call Start to begin the sync loop.
func New(config Config) (*Connector, error) {
	if config.HomeserverURL == "" {
		return nil, fmt.Errorf("matrix: HomeserverURL is required")
	}
	if config.AccessToken == "" {
		return nil, fmt.Errorf("matrix: AccessToken is required")
	}
	if config.UserID == "" {
		return nil, fmt.Errorf("matrix: UserID is required")
	}
	if !strings.HasPrefix(config.UserID, "@") {
		return nil, fmt.Errorf("matrix: UserID must be a full Matrix ID starting with @ (e.g. @bot:matrix.org), got %q", config.UserID)
	}
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}
	if config.SinceTokenPath == "" {
		config.SinceTokenPath = sinceTokenFilename
	}

	// Normalise HomeserverURL — strip trailing slash.
	config.HomeserverURL = strings.TrimRight(config.HomeserverURL, "/")

	return &Connector{
		config:     config,
		httpClient: &http.Client{Timeout: 0}, // no timeout — sync long-polls and gateway may be slow
	}, nil
}

// Start runs the Matrix /sync loop and blocks until ctx is cancelled.
func (c *Connector) Start(ctx context.Context) error {
	log.Printf("matrix connector: starting (homeserver: %s, user: %s, gateway: %s)",
		c.config.HomeserverURL, c.config.UserID, c.config.GatewayURL)

	since := c.loadSinceToken()
	if since != "" {
		log.Printf("matrix connector: resuming from since token %q", since)
	} else {
		log.Printf("matrix connector: no since token found, starting from current state (old messages will be skipped)")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		nextBatch, err := c.sync(ctx, since)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("matrix connector: sync error: %v — retrying in %s", err, syncRetryDelay)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(syncRetryDelay):
			}
			continue
		}

		if nextBatch != "" && nextBatch != since {
			since = nextBatch
			c.saveSinceToken(since)
		}
	}
}

// sync performs one /_matrix/client/v3/sync call, processes events, and
// returns the next_batch token.
func (c *Connector) sync(ctx context.Context, since string) (string, error) {
	u, err := url.Parse(c.config.HomeserverURL + "/_matrix/client/v3/sync")
	if err != nil {
		return "", fmt.Errorf("parse sync URL: %w", err)
	}

	q := u.Query()
	q.Set("timeout", strconv.Itoa(syncTimeout))
	// Request only joined room timelines to reduce payload size.
	q.Set("filter", `{"room":{"timeline":{"limit":50}}}`)
	if since != "" {
		q.Set("since", since)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create sync request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.config.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET /sync: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("/sync returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var syncResp syncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return "", fmt.Errorf("decode sync response: %w", err)
	}

	c.processSyncResponse(ctx, syncResp)
	return syncResp.NextBatch, nil
}

// processSyncResponse iterates over all joined rooms and dispatches message
// events to processMessage.
func (c *Connector) processSyncResponse(ctx context.Context, syncResp syncResponse) {
	for roomID, room := range syncResp.Rooms.Join {
		for _, event := range room.Timeline.Events {
			if event.Type != "m.room.message" {
				continue
			}
			// Ignore our own messages to prevent echo loops.
			if event.Sender == c.config.UserID {
				continue
			}

			var content messageContent
			if err := json.Unmarshal(event.Content, &content); err != nil {
				log.Printf("matrix connector: failed to decode message content for event %s: %v", event.EventID, err)
				continue
			}

			// Only handle plain text messages.
			if content.MsgType != "m.text" {
				continue
			}

			text := strings.TrimSpace(content.Body)
			if text == "" {
				continue
			}

			// Strip any @mention of the bot from the text.
			text = stripBotMention(text, c.config.UserID)
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}

			log.Printf("matrix connector: message from %s in room %s: %q", event.Sender, roomID, text)

			// Process each message in a goroutine so the sync loop is not blocked.
			go c.processMessage(ctx, roomID, text)
		}
	}
}

// processMessage calls the gateway and sends the response back to the Matrix room.
func (c *Connector) processMessage(ctx context.Context, roomID, text string) {
	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("matrix connector: gateway error for room %s: %v", roomID, err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	chunks := splitMessage(response, matrixMaxMessageLen)
	for i, chunk := range chunks {
		if err := c.sendMessage(ctx, roomID, chunk); err != nil {
			log.Printf("matrix connector: failed to send message chunk %d/%d to room %s: %v",
				i+1, len(chunks), roomID, err)
		}
	}
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

	log.Printf("matrix connector: gateway response: %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendMessage sends a single m.room.message event to a Matrix room via the
// PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message/{txnId} endpoint.
func (c *Connector) sendMessage(ctx context.Context, roomID, text string) error {
	txnID := c.txnCounter.Add(1)

	body := sendEventBody{
		MsgType: "m.text",
		Body:    text,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal message body: %w", err)
	}

	// URL-encode the room ID — it contains a colon which is valid in a path
	// segment per Matrix spec, but we encode it for safety.
	encodedRoomID := url.PathEscape(roomID)
	endpoint := fmt.Sprintf("%s/_matrix/client/v3/rooms/%s/send/m.room.message/%d",
		c.config.HomeserverURL, encodedRoomID, txnID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.AccessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("homeserver returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// loadSinceToken reads the persisted /sync since token from disk.
// Returns an empty string if the file does not exist or cannot be read.
func (c *Connector) loadSinceToken() string {
	data, err := os.ReadFile(c.config.SinceTokenPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("matrix connector: failed to read since token from %s: %v", c.config.SinceTokenPath, err)
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

// saveSinceToken writes the /sync since token to disk atomically by writing to
// a temp file and renaming.  Logging any error but not returning it — a lost
// token only means re-processing a small window of events on restart.
func (c *Connector) saveSinceToken(token string) {
	dir := filepath.Dir(c.config.SinceTokenPath)
	tmp, err := os.CreateTemp(dir, ".matrix_since_*.tmp")
	if err != nil {
		log.Printf("matrix connector: failed to create temp file for since token: %v", err)
		return
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(token); err != nil {
		log.Printf("matrix connector: failed to write since token: %v", err)
		tmp.Close()
		_ = os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		log.Printf("matrix connector: failed to close since token temp file: %v", err)
		_ = os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, c.config.SinceTokenPath); err != nil {
		log.Printf("matrix connector: failed to rename since token file: %v", err)
		_ = os.Remove(tmpName)
	}
}

// stripBotMention removes @mention patterns for the bot's own user ID from the
// message text.  Matrix clients typically write "@bot:server.org: " at the
// start of a message when mentioning someone.
func stripBotMention(text, userID string) string {
	// Strip the full @userID mention.
	text = strings.ReplaceAll(text, userID, "")
	// Also strip just the localpart (e.g. "@bot" without ":server.org").
	localpart := userID
	if idx := strings.Index(userID, ":"); idx != -1 {
		localpart = userID[:idx]
	}
	text = strings.ReplaceAll(text, localpart, "")
	// Clean up any leading punctuation that gets left behind (colon, comma, etc.)
	text = strings.TrimLeft(text, ":, \t")
	return text
}

// splitMessage divides text into chunks of at most maxLen bytes, preferring to
// break on paragraph or line boundaries.
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
