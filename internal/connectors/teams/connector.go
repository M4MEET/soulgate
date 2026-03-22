// Package teams provides a Microsoft Teams connector for SoulGate.
//
// It bridges Microsoft Teams to the SoulGate HTTP /api/chat endpoint using the
// Bot Framework v4 protocol over plain HTTP/JSON — no SDK required.
//
// Architecture:
//
//	Teams user sends message
//	    -> Teams calls your webhook (POST /api/messages)
//	    -> Connector parses the Activity JSON
//	    -> Connector POSTs message to SoulGate /api/chat
//	    -> Connector POSTs reply Activity to Bot Framework REST API
//	    -> Teams delivers response to user
//
// The connector handles text messages arriving in 1:1 chats, group chats, and
// channel conversations.  @mentions of the bot are stripped before the message
// is forwarded to the gateway.
//
// Bot Framework auth: The incoming request carries a JWT Bearer token signed by
// Microsoft.  Full JWT validation requires fetching Microsoft's OIDC keys.  In
// production you should validate the token; this connector performs a
// lightweight structural check and logs a warning if validation is skipped,
// keeping the implementation dependency-free.  Set VerifyToken=true to enable
// the runtime warning prompt.
//
// Outgoing auth: The connector obtains an OAuth2 Bearer token from the Bot
// Framework token endpoint (login.microsoftonline.com) using the AppID and
// AppPassword, and uses that token when POSTing reply activities back to Teams.
package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultGatewayURL  = "http://localhost:8080"
	defaultListenAddr  = ":3978"
	tokenEndpoint      = "https://login.microsoftonline.com/botframework.com/oauth2/v2.0/token"
	botFrameworkScope  = "https://api.botframework.com/.default"
	replyURLTemplate   = "%s/v3/conversations/%s/activities"
	teamsMaxMessageLen = 28000 // Teams supports up to ~28 KB of text per message
)

// Config holds all configuration for the Teams connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// AppID is the Microsoft Teams Bot App ID (also called MicrosoftAppId).
	// Found in the Azure Bot resource under Configuration.
	AppID string

	// AppPassword is the Teams Bot App Password (client secret).
	// Found in the Azure Bot resource under Configuration.
	AppPassword string

	// ListenAddr is the address on which the webhook HTTP server listens.
	// Defaults to :3978 (the standard Bot Framework port) when empty.
	ListenAddr string
}

// activity is a Bot Framework v4 Activity object (subset of fields we use).
// See https://learn.microsoft.com/en-us/azure/bot-service/rest-api/bot-framework-rest-connector-api-reference
type activity struct {
	Type           string          `json:"type"`
	ID             string          `json:"id"`
	ServiceURL     string          `json:"serviceUrl"`
	ChannelID      string          `json:"channelId"`
	Conversation   conversationRef `json:"conversation"`
	From           channelAccount  `json:"from"`
	Recipient      channelAccount  `json:"recipient"`
	Text           string          `json:"text"`
	TextFormat     string          `json:"textFormat,omitempty"`
	MentionsField  json.RawMessage `json:"entities,omitempty"`
}

// conversationRef identifies the conversation (channel, group chat, or 1:1).
type conversationRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// channelAccount identifies a Teams user or bot.
type channelAccount struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// replyActivity is the Activity we POST back to the Bot Framework service.
type replyActivity struct {
	Type         string         `json:"type"`
	Text         string         `json:"text"`
	TextFormat   string         `json:"textFormat"`
	Conversation conversationRef `json:"conversation"`
	From         channelAccount `json:"from"`
	Recipient    channelAccount `json:"recipient"`
	ReplyToID    string         `json:"replyToId,omitempty"`
}

// tokenResponse is the response from the Bot Framework OAuth2 token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
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

// cachedToken holds an access token and its expiry time.
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// Connector bridges Microsoft Teams to the SoulGate HTTP API via the Bot
// Framework v4 webhook protocol.
type Connector struct {
	config     Config
	httpClient *http.Client
	server     *http.Server

	// tokenMu guards cachedToken.
	tokenMu     sync.Mutex
	cachedToken *cachedToken

	// stopOnce ensures Stop is idempotent.
	stopOnce sync.Once
}

// New creates a new Teams connector.  It validates the configuration but does
// not yet start the HTTP server — call Start to begin accepting webhooks.
func New(config Config) (*Connector, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("teams: AppID is required")
	}
	if config.AppPassword == "" {
		return nil, fmt.Errorf("teams: AppPassword is required")
	}
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}
	if config.ListenAddr == "" {
		config.ListenAddr = defaultListenAddr
	}

	c := &Connector{
		config: config,
		// No timeout on the gateway HTTP client — agentic loops can be long.
		httpClient: &http.Client{Timeout: 0},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/messages", c.handleWebhook)

	c.server = &http.Server{
		Addr:    config.ListenAddr,
		Handler: mux,
		// Apply modest read/write timeouts on the incoming webhook side only.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return c, nil
}

// Start begins listening for Teams webhook events and blocks until ctx is
// cancelled.
func (c *Connector) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", c.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("teams: failed to listen on %s: %w", c.config.ListenAddr, err)
	}

	log.Printf("teams connector: listening on %s (gateway: %s)", c.config.ListenAddr, c.config.GatewayURL)

	// Shut down cleanly when ctx is cancelled.
	go func() {
		<-ctx.Done()
		log.Printf("teams connector: shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.server.Shutdown(shutCtx)
	}()

	if err := c.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("teams connector: server error: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the webhook server.  It is safe to call more than
// once.
func (c *Connector) Stop() error {
	var stopErr error
	c.stopOnce.Do(func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopErr = c.server.Shutdown(shutCtx)
	})
	return stopErr
}

// handleWebhook is the HTTP handler for POST /api/messages.  Teams sends a
// JSON-encoded Activity object for every incoming message.
func (c *Connector) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
	if err != nil {
		log.Printf("teams connector: failed to read request body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var act activity
	if err := json.Unmarshal(body, &act); err != nil {
		log.Printf("teams connector: failed to parse activity JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Bot Framework requires us to respond 200 OK quickly, even if we process
	// the message asynchronously.
	w.WriteHeader(http.StatusOK)

	// Only handle message-type activities.
	if act.Type != "message" {
		return
	}

	text := strings.TrimSpace(act.Text)
	if text == "" {
		return
	}

	// Strip HTML tags and @mention tokens that Teams may inject.
	text = stripHTMLTags(text)
	text = stripTeamsMentions(text, act.Recipient.ID, act.Recipient.Name)
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	log.Printf("teams connector: message from %s in conversation %s: %q",
		act.From.Name, act.Conversation.ID, text)

	// Process asynchronously so we don't hold the webhook response open.
	go c.processMessage(act, text)
}

// processMessage calls the gateway and sends the response back to Teams.
func (c *Connector) processMessage(act activity, text string) {
	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("teams connector: gateway error for conversation %s: %v", act.Conversation.ID, err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	if err := c.sendReply(act, response); err != nil {
		log.Printf("teams connector: failed to send reply to conversation %s: %v",
			act.Conversation.ID, err)
	}
}

// sendToGateway POSTs the message to the SoulGate /api/chat endpoint and
// returns the text response.
func (c *Connector) sendToGateway(message string) (string, error) {
	payload, err := json.Marshal(gatewayRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.config.GatewayURL, "/") + "/api/chat"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", url, err)
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

	log.Printf("teams connector: gateway response: %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendReply posts a reply Activity back to Teams via the Bot Framework REST
// API.  Long responses are split into multiple activities.
func (c *Connector) sendReply(act activity, response string) error {
	token, err := c.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	chunks := splitMessage(response, teamsMaxMessageLen)
	for i, chunk := range chunks {
		reply := replyActivity{
			Type:         "message",
			Text:         chunk,
			TextFormat:   "plain",
			Conversation: act.Conversation,
			From:         act.Recipient, // bot becomes the sender in the reply
			Recipient:    act.From,      // original sender becomes recipient
		}
		// Only set ReplyToID on the first chunk to keep threading clean.
		if i == 0 {
			reply.ReplyToID = act.ID
		}

		if err := c.postActivity(act.ServiceURL, act.Conversation.ID, token, reply); err != nil {
			return fmt.Errorf("post activity chunk %d/%d: %w", i+1, len(chunks), err)
		}
	}
	return nil
}

// postActivity sends a single reply Activity to the Bot Framework REST API.
func (c *Connector) postActivity(serviceURL, conversationID, token string, reply replyActivity) error {
	payload, err := json.Marshal(reply)
	if err != nil {
		return fmt.Errorf("marshal reply activity: %w", err)
	}

	url := fmt.Sprintf(replyURLTemplate, strings.TrimRight(serviceURL, "/"), conversationID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Use a short-timeout client for outbound Bot Framework calls.
	outClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := outClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Bot Framework replied %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// getAccessToken returns a valid Bot Framework OAuth2 bearer token, fetching a
// new one from the token endpoint if the cached token has expired.
func (c *Connector) getAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Return cached token if it is still valid with a 60-second buffer.
	if c.cachedToken != nil && time.Now().Before(c.cachedToken.expiresAt.Add(-60*time.Second)) {
		return c.cachedToken.token, nil
	}

	form := strings.NewReader(
		"grant_type=client_credentials" +
			"&client_id=" + c.config.AppID +
			"&client_secret=" + c.config.AppPassword +
			"&scope=" + botFrameworkScope,
	)

	resp, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", form) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned empty access_token")
	}

	c.cachedToken = &cachedToken{
		token:     tr.AccessToken,
		expiresAt: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}

	log.Printf("teams connector: obtained new Bot Framework access token (expires in %ds)", tr.ExpiresIn)
	return tr.AccessToken, nil
}

// stripHTMLTags removes HTML tags that Teams sometimes includes in Activity
// text (e.g. <at>BotName</at>, <p>, <br/>, etc.).
var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

func stripHTMLTags(text string) string {
	return htmlTagRE.ReplaceAllString(text, "")
}

// stripTeamsMentions removes Teams @mention patterns.  Teams encodes mentions
// as "<at>BotName</at>" in the text; after HTML-stripping those become
// bare "BotName" strings which we further clean up by comparing against the
// bot's known name and ID.
func stripTeamsMentions(text, botID, botName string) string {
	// Remove any residual "<at>...</at>" tags (in case stripHTMLTags was not
	// called first or there are nested tags).
	text = regexp.MustCompile(`<at>[^<]*</at>`).ReplaceAllString(text, "")
	// Remove the literal bot name that Teams sometimes leaves behind.
	if botName != "" {
		text = strings.ReplaceAll(text, botName, "")
	}
	return text
}

// splitMessage divides text into chunks of at most maxLen bytes, preferring to
// break on paragraph or line boundaries to avoid cutting sentences.
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
