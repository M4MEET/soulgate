// Package feishu provides a Feishu (Lark) connector for SoulGate.
//
// It bridges Feishu/Lark to the SoulGate HTTP /api/chat endpoint using the
// Feishu event subscription webhook model — no external SDK required.
//
// Architecture:
//
//	Connector starts HTTP server on ListenAddr
//	    -> Feishu POSTs events to /feishu/events
//	    -> Connector handles URL verification challenge (first-time setup)
//	    -> Connector receives im.message.receive_v1 events
//	    -> Extracts text content from the message
//	    -> POSTs message to SoulGate /api/chat
//	    -> Sends reply via POST https://open.feishu.cn/open-apis/im/v1/messages
//	    -> Auth: tenant_access_token obtained from /open-apis/auth/v3/tenant_access_token/internal
//
// Feishu event subscriptions deliver events via HTTP POST to the configured
// webhook URL.  The connector must be reachable from Feishu's servers; use
// ngrok or similar for local development.
//
// References:
//
//	https://open.feishu.cn/document/home/index
//	https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create
//	https://open.feishu.cn/document/ukTMukTMukTM/uMTNx4yM1EjLzUTM/event-subscription-overview
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultGatewayURL  = "http://localhost:8080"
	defaultListenAddr  = ":3979"
	feishuBaseURL      = "https://open.feishu.cn"
	tokenEndpoint      = "/open-apis/auth/v3/tenant_access_token/internal"
	sendMessageEndpoint = "/open-apis/im/v1/messages"

	// tokenRefreshBuffer is the safety margin before token expiry to refresh.
	tokenRefreshBuffer = 60 * time.Second

	feishuMaxMessageLen = 4000 // Feishu text message character limit
)

// Config holds all configuration for the Feishu connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// AppID is the Feishu application ID (App ID).
	// Found in the Feishu Developer Console under App Credentials.
	AppID string

	// AppSecret is the Feishu application secret.
	// Found in the Feishu Developer Console under App Credentials.
	AppSecret string

	// ListenAddr is the address on which the webhook HTTP server listens.
	// Defaults to :3979 when empty.
	ListenAddr string

	// VerifyToken is the verification token from the Feishu event subscription
	// configuration.  When non-empty the connector validates incoming event
	// tokens against this value.
	VerifyToken string
}

// ---------------------------------------------------------------------------
// Feishu API types
// ---------------------------------------------------------------------------

// eventEnvelope is the outer envelope for all Feishu event webhook payloads.
// Feishu uses two schemas: v1.0 (legacy) and v2.0.  We support both.
type eventEnvelope struct {
	// v2.0 fields
	Schema string          `json:"schema"`
	Header *eventHeader    `json:"header,omitempty"`
	Event  json.RawMessage `json:"event,omitempty"`

	// v1.0 fields (also present in challenge requests)
	Challenge string `json:"challenge,omitempty"`
	Token     string `json:"token,omitempty"`
	Type      string `json:"type,omitempty"` // "url_verification" in v1.0
}

// eventHeader is the v2.0 event header.
type eventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"` // e.g. "im.message.receive_v1"
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

// messageReceiveEvent is the payload of an im.message.receive_v1 event.
type messageReceiveEvent struct {
	Sender  messageSender  `json:"sender"`
	Message feishuMessage  `json:"message"`
}

// messageSender identifies who sent the message.
type messageSender struct {
	SenderID struct {
		UserID  string `json:"user_id"`
		UnionID string `json:"union_id"`
		OpenID  string `json:"open_id"`
	} `json:"sender_id"`
	SenderType string `json:"sender_type"`
	TenantKey  string `json:"tenant_key"`
}

// feishuMessage holds the message metadata and content.
type feishuMessage struct {
	MessageID   string `json:"message_id"`
	RootID      string `json:"root_id"`   // non-empty if this is a thread reply
	ParentID    string `json:"parent_id"` // parent message in a thread
	CreateTime  string `json:"create_time"`
	ChatID      string `json:"chat_id"`    // the conversation/group chat ID
	ChatType    string `json:"chat_type"`  // "p2p" = DM, "group" = group chat
	MessageType string `json:"message_type"` // "text", "post", etc.
	Content     string `json:"content"`    // JSON-encoded content
}

// textContent is the JSON content for a "text" Feishu message.
type textContent struct {
	Text string `json:"text"`
}

// sendMessageRequest is the body for POST /open-apis/im/v1/messages.
type sendMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"` // "text"
	Content   string `json:"content"`  // JSON-encoded content
}

// sendTextContent is the content field for a text reply.
type sendTextContent struct {
	Text string `json:"text"`
}

// tenantTokenRequest is the body for the tenant_access_token endpoint.
type tenantTokenRequest struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// tenantTokenResponse is the response from the tenant_access_token endpoint.
type tenantTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"` // seconds until expiry
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

// cachedToken holds a tenant access token and its expiry.
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// ---------------------------------------------------------------------------
// Connector
// ---------------------------------------------------------------------------

// Connector bridges Feishu/Lark to the SoulGate HTTP API via the event
// subscription webhook model.
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

// New creates a new Feishu connector.  It validates the configuration but does
// not yet start the HTTP server — call Start to begin accepting events.
func New(config Config) (*Connector, error) {
	if config.AppID == "" {
		return nil, fmt.Errorf("feishu: AppID is required")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("feishu: AppSecret is required")
	}
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}
	if config.ListenAddr == "" {
		config.ListenAddr = defaultListenAddr
	}

	c := &Connector{
		config: config,
		// No timeout on gateway calls — agentic loops can take minutes.
		httpClient: &http.Client{Timeout: 0},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feishu/events", c.handleEvent)

	c.server = &http.Server{
		Addr:        config.ListenAddr,
		Handler:     mux,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout is intentionally generous because we may need to call
		// the gateway before replying, though the event handler responds
		// immediately with 200 and processes asynchronously.
		WriteTimeout: 15 * time.Second,
	}

	return c, nil
}

// Start begins listening for Feishu webhook events and blocks until ctx is
// cancelled.
func (c *Connector) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", c.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("feishu: failed to listen on %s: %w", c.config.ListenAddr, err)
	}

	log.Printf("feishu connector: listening on %s (gateway: %s)", c.config.ListenAddr, c.config.GatewayURL)
	log.Printf("feishu connector: webhook URL: POST http://<your-host>%s/feishu/events", c.config.ListenAddr)

	// Shut down cleanly when ctx is cancelled.
	go func() {
		<-ctx.Done()
		log.Printf("feishu connector: shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.server.Shutdown(shutCtx)
	}()

	if err := c.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("feishu connector: server error: %w", err)
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

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

// handleEvent is the HTTP handler for POST /feishu/events.
// Feishu posts all subscription events here.
func (c *Connector) handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
	if err != nil {
		log.Printf("feishu connector: failed to read request body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var envelope eventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		log.Printf("feishu connector: failed to parse event JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Handle URL verification challenge (Feishu sends this when first
	// configuring the event subscription).
	// Both v1.0 and v2.0 use the same top-level "challenge" field.
	if envelope.Challenge != "" {
		c.handleChallenge(w, envelope)
		return
	}

	// Validate the token if VerifyToken is configured.
	if c.config.VerifyToken != "" {
		token := envelope.Token
		if envelope.Header != nil && envelope.Header.Token != "" {
			token = envelope.Header.Token
		}
		if token != c.config.VerifyToken {
			log.Printf("feishu connector: token mismatch — got %q, want %q", token, c.config.VerifyToken)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	// Acknowledge Feishu immediately — event processing is asynchronous.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{}`))

	// Dispatch based on event type.
	eventType := ""
	if envelope.Header != nil {
		eventType = envelope.Header.EventType
	} else if envelope.Type != "" {
		eventType = envelope.Type
	}

	switch eventType {
	case "im.message.receive_v1":
		go c.handleMessageReceive(r.Context(), envelope.Event)
	default:
		// Silently ignore other event types (message recall, bot added to group, etc.).
	}
}

// handleChallenge responds to Feishu's URL verification challenge.
// Feishu requires the challenge value echoed back as JSON: {"challenge":"<value>"}.
func (c *Connector) handleChallenge(w http.ResponseWriter, env eventEnvelope) {
	log.Printf("feishu connector: responding to URL verification challenge")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"challenge":%q}`, env.Challenge)
}

// ---------------------------------------------------------------------------
// Message handling
// ---------------------------------------------------------------------------

// handleMessageReceive processes an im.message.receive_v1 event.
func (c *Connector) handleMessageReceive(ctx context.Context, raw json.RawMessage) {
	if raw == nil {
		return
	}

	var evt messageReceiveEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		log.Printf("feishu connector: failed to parse im.message.receive_v1 event: %v", err)
		return
	}

	msg := evt.Message

	// Only handle text messages.
	if msg.MessageType != "text" {
		log.Printf("feishu connector: ignoring message type %q", msg.MessageType)
		return
	}

	// Decode the text content.
	var content textContent
	if err := json.Unmarshal([]byte(msg.Content), &content); err != nil {
		log.Printf("feishu connector: failed to decode text content for message %s: %v", msg.MessageID, err)
		return
	}

	text := strings.TrimSpace(content.Text)
	if text == "" {
		return
	}

	log.Printf("feishu connector: message from sender=%s in chat=%s (type=%s): %q",
		evt.Sender.SenderID.OpenID, msg.ChatID, msg.ChatType, text)

	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("feishu connector: gateway error for message %s: %v", msg.MessageID, err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	// Determine the receive_id for the reply:
	// - For DMs (p2p), use the chat_id; Feishu routes it to the correct 1-1 conversation.
	// - For group chats, also use the chat_id to reply to the group.
	//
	// We pass receive_id_type=chat_id to the send message API in both cases.
	receiveID := msg.ChatID

	chunks := splitMessage(response, feishuMaxMessageLen)
	for i, chunk := range chunks {
		if err := c.sendMessage(ctx, receiveID, chunk); err != nil {
			log.Printf("feishu connector: failed to send reply chunk %d/%d for message %s: %v",
				i+1, len(chunks), msg.MessageID, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Feishu API calls
// ---------------------------------------------------------------------------

// sendMessage sends a text message to a Feishu chat_id via the messaging API.
func (c *Connector) sendMessage(ctx context.Context, chatID, text string) error {
	token, err := c.getTenantAccessToken()
	if err != nil {
		return fmt.Errorf("get tenant access token: %w", err)
	}

	// Build the content JSON: {"text":"<message>"}.
	contentBytes, err := json.Marshal(sendTextContent{Text: text})
	if err != nil {
		return fmt.Errorf("marshal content: %w", err)
	}

	reqBody := sendMessageRequest{
		ReceiveID: chatID,
		MsgType:   "text",
		Content:   string(contentBytes),
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal send message request: %w", err)
	}

	// receive_id_type=chat_id tells Feishu that ReceiveID is a chat_id.
	apiURL := feishuBaseURL + sendMessageEndpoint + "?receive_id_type=chat_id"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	outClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := outClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Feishu send message API returned HTTP %d: %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	// Feishu also returns errors inside a successful (200) response body.
	var apiResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil && apiResp.Code != 0 {
		return fmt.Errorf("Feishu send message error code %d: %s", apiResp.Code, apiResp.Msg)
	}

	return nil
}

// getTenantAccessToken returns a valid tenant access token, fetching a new one
// from the Feishu auth endpoint if the cached token has expired.
func (c *Connector) getTenantAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Return cached token if still valid with the refresh buffer applied.
	if c.cachedToken != nil && time.Now().Before(c.cachedToken.expiresAt.Add(-tokenRefreshBuffer)) {
		return c.cachedToken.token, nil
	}

	reqBody := tenantTokenRequest{
		AppID:     c.config.AppID,
		AppSecret: c.config.AppSecret,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal token request: %w", err)
	}

	apiURL := feishuBaseURL + tokenEndpoint
	// Use a fresh http.DefaultClient for this bootstrap call; no context
	// propagation needed since this is a short-lived credential request.
	resp, err := http.Post(apiURL, "application/json; charset=utf-8", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("Feishu token endpoint returned HTTP %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr tenantTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if tr.Code != 0 {
		return "", fmt.Errorf("Feishu token error %d: %s", tr.Code, tr.Msg)
	}
	if tr.TenantAccessToken == "" {
		return "", fmt.Errorf("Feishu returned empty tenant_access_token")
	}

	c.cachedToken = &cachedToken{
		token:     tr.TenantAccessToken,
		expiresAt: time.Now().Add(time.Duration(tr.Expire) * time.Second),
	}

	log.Printf("feishu connector: obtained new tenant access token (expires in %ds)", tr.Expire)
	return tr.TenantAccessToken, nil
}

// ---------------------------------------------------------------------------
// Gateway
// ---------------------------------------------------------------------------

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

	log.Printf("feishu connector: gateway response: %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
