package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/auth"
	"github.com/M4MEET/soulgate/internal/gateway/webui"
	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/M4MEET/soulgate/internal/session"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking
		return true
	},
}

// Gateway is the central control plane for routing messages
type Gateway struct {
	// Client management
	clients    map[string]*Client
	clientsMux sync.RWMutex

	// Role-based routing
	agents   map[string]*Client
	channels map[string]*Client
	uis      map[string]*Client
	nodes    map[string]*Client
	roleMux  sync.RWMutex

	// Session management
	sessions       map[string]*Session
	sessionMux     sync.RWMutex
	sessionStorage *session.Storage

	// Routing
	router *Router

	// Authentication
	tokenManager   *auth.TokenManager
	pairingManager *auth.PairingManager
	authEnabled    bool

	// Configuration
	config    *Config
	startedAt time.Time

	// Webhook and notification subsystems (optional — nil until LoadWebhooks called)
	webhookStore      *WebhookStore
	notificationStore *NotificationStore

	// Health monitoring
	monitor *healthMonitor
}

// ChatHandler processes a chat message and returns a response.
// This allows the gateway to serve an HTTP /api/chat endpoint without
// importing the core package directly.
type ChatHandler func(ctx context.Context, message string) (string, error)

// Config holds Gateway configuration
type Config struct {
	Address     string
	Port        int
	SessionsDir string      // Directory for session JSONL files
	AuthEnabled bool        // Enable authentication (default: false for backward compatibility)
	OnChat      ChatHandler // If set, gateway serves POST /api/chat

	// Metadata surfaced on the /api/status endpoint and the web UI.
	Provider string // e.g., "anthropic", "openai"
	Model    string // e.g., "claude-opus-4-5"

	// Webhook and notification config files (optional).
	// When non-empty Start() will load them automatically before serving.
	WebhooksFile      string // Path to .soulgate/webhooks.json
	NotificationsFile string // Path to .soulgate/notifications.json
}

// NewGateway creates a new Gateway
func NewGateway(config *Config) (*Gateway, error) {
	if config == nil {
		config = &Config{
			Address:     "0.0.0.0",
			Port:        8080,
			SessionsDir: "sessions",
		}
	}

	if config.SessionsDir == "" {
		config.SessionsDir = "sessions"
	}

	// Create session storage
	sessionStorage, err := session.NewStorage(config.SessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create session storage: %w", err)
	}

	// Create router with smart strategy
	router := NewRouter(StrategySmart)

	// Create authentication system
	tokenManager := auth.NewTokenManager()
	pairingManager := auth.NewPairingManager(tokenManager)

	// Start cleanup routines if auth is enabled
	authEnabled := config.AuthEnabled
	if authEnabled {
		fmt.Println("🔒 Authentication enabled")
		// Cleanup expired tokens and codes every hour
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				tokenCount := tokenManager.CleanupExpired()
				codeCount := pairingManager.CleanupExpired()
				if tokenCount > 0 || codeCount > 0 {
					fmt.Printf("🧹 Cleaned up %d expired tokens and %d expired pairing codes\n", tokenCount, codeCount)
				}
			}
		}()
	}

	gw := &Gateway{
		clients:        make(map[string]*Client),
		agents:         make(map[string]*Client),
		channels:       make(map[string]*Client),
		uis:            make(map[string]*Client),
		nodes:          make(map[string]*Client),
		sessions:       make(map[string]*Session),
		sessionStorage: sessionStorage,
		router:         router,
		tokenManager:   tokenManager,
		pairingManager: pairingManager,
		authEnabled:    authEnabled,
		config:         config,
		startedAt:      time.Now(),
	}
	gw.monitor = newHealthMonitor(gw)
	return gw, nil
}

// LoadWebhooks initialises the inbound webhook subsystem from a JSON config file.
// Call this before Start() to enable the /webhook/{name} routes.
func (g *Gateway) LoadWebhooks(path string) error {
	s := newWebhookStore(path)
	if err := s.load(); err != nil {
		return fmt.Errorf("load webhooks: %w", err)
	}
	g.webhookStore = s
	return nil
}

// LoadNotifications initialises the outbound notification subsystem from a JSON
// config file. Call this before Start() to enable event-driven notifications.
func (g *Gateway) LoadNotifications(path string) error {
	s := newNotificationStore(path)
	if err := s.load(); err != nil {
		return fmt.Errorf("load notifications: %w", err)
	}
	g.notificationStore = s
	return nil
}

// WebhookStore returns the underlying webhook store so callers (e.g. CLI) can
// read and mutate the persisted webhook list. Returns nil if LoadWebhooks has
// not been called.
func (g *Gateway) WebhookStore() *WebhookStore {
	return g.webhookStore
}

// NotificationStore returns the underlying notification store so callers can
// read and mutate the persisted notification list. Returns nil if
// LoadNotifications has not been called.
func (g *Gateway) NotificationStore() *NotificationStore {
	return g.notificationStore
}

// Start starts the Gateway server
func (g *Gateway) Start(ctx context.Context) error {
	// Auto-load webhook and notification configs from paths specified in Config.
	if g.config.WebhooksFile != "" && g.webhookStore == nil {
		if err := g.LoadWebhooks(g.config.WebhooksFile); err != nil {
			// Non-fatal: log but continue without webhooks.
			fmt.Printf("Warning: could not load webhooks: %v\n", err)
		}
	}
	if g.config.NotificationsFile != "" && g.notificationStore == nil {
		if err := g.LoadNotifications(g.config.NotificationsFile); err != nil {
			fmt.Printf("Warning: could not load notifications: %v\n", err)
		}
	}

	mux := http.NewServeMux()

	// WebSocket endpoint
	mux.HandleFunc("/ws", g.handleWebSocket)

	// Health endpoints
	mux.HandleFunc("/health", g.handleHealth)
	mux.HandleFunc("/api/health", g.handleHealth)

	// REST API
	mux.HandleFunc("/api/status", g.handleAPIStatus)
	mux.HandleFunc("/api/sessions", g.handleAPISessions)

	// Serve the HTTP chat API if a chat handler is configured
	if g.config.OnChat != nil {
		mux.HandleFunc("/api/chat", g.handleAPIChat)
		fmt.Println("HTTP API enabled: POST /api/chat")
	}

	// Inbound webhooks — enabled whenever a webhook store is present
	if g.webhookStore != nil {
		mux.HandleFunc("/webhook/", g.handleWebhook)
		fmt.Println("Webhooks enabled: POST /webhook/{name}")
	}

	// Web UI — served at /  (index.html, app.js, style.css via embed.FS)
	mux.Handle("/", http.FileServer(http.FS(webui.Assets)))

	addr := fmt.Sprintf("%s:%d", g.config.Address, g.config.Port)
	fmt.Printf("Gateway listening on http://%s  (WebSocket: ws://%s/ws)\n", addr, addr)

	server := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	// Shutdown handler
	go func() {
		<-ctx.Done()
		fmt.Println("Shutting down Gateway...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx) //nolint:errcheck
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// handleWebSocket handles WebSocket connections
func (g *Gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to upgrade connection: %v\n", err)
		return
	}

	// Generate client ID
	clientID := uuid.New().String()

	// Wait for connect frame
	_, message, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}

	frame, err := protocol.ParseFrame(message)
	if err != nil {
		conn.Close()
		return
	}

	connectFrame, ok := frame.(*protocol.ConnectFrame)
	if !ok {
		conn.Close()
		return
	}

	// Use provided client ID if available
	if connectFrame.ClientID != "" {
		clientID = connectFrame.ClientID
	}

	// Create client
	client := NewClient(clientID, connectFrame.Role, conn, g)
	client.metadata = connectFrame.Metadata

	// Register client
	if err := g.Register(client); err != nil {
		errorFrame := &protocol.EventErrorFrame{
			Type:      protocol.FrameEventError,
			Error:     err.Error(),
			Timestamp: time.Now().Unix(),
		}
		data, _ := protocol.ToJSON(errorFrame)
		conn.WriteMessage(websocket.TextMessage, data)
		conn.Close()
		return
	}

	// Send connect acknowledgment
	ackFrame := &protocol.ConnectAckFrame{
		Type:      protocol.FrameConnectAck,
		ClientID:  clientID,
		Version:   protocol.ProtocolVersion,
		Timestamp: time.Now().Unix(),
	}
	client.Send(ackFrame)

	fmt.Printf("✓ Client connected: %s (role: %s)\n", clientID, connectFrame.Role)

	// Start client pumps with a background context (don't tie to HTTP request lifecycle)
	go client.Start(context.Background())
}

// handleAPIStatus returns a rich JSON status payload consumed by the web UI.
func (g *Gateway) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	g.clientsMux.RLock()
	totalClients := len(g.clients)
	g.clientsMux.RUnlock()

	g.roleMux.RLock()
	agentCount   := len(g.agents)
	channelCount := len(g.channels)
	uiCount      := len(g.uis)
	nodeCount    := len(g.nodes)

	// Build per-role client maps so the UI can render individual client rows.
	agentIDs   := clientIDs(g.agents)
	channelIDs := clientIDs(g.channels)
	uiIDs      := clientIDs(g.uis)
	nodeIDs    := clientIDs(g.nodes)
	g.roleMux.RUnlock()

	g.sessionMux.RLock()
	sessionCount := len(g.sessions)
	g.sessionMux.RUnlock()

	uptimeSeconds := int64(time.Since(g.startedAt).Seconds())

	port := g.config.Port
	if port == 0 {
		port = 8080
	}

	payload := map[string]interface{}{
		"status":          "healthy",
		"clients":         totalClients,
		"sessions":        sessionCount,
		"agents":          agentCount,
		"channels":        channelCount,
		"uis":             uiCount,
		"nodes":           nodeCount,
		"agent_clients":   agentIDs,
		"channel_clients": channelIDs,
		"ui_clients":      uiIDs,
		"node_clients":    nodeIDs,
		"provider":        g.config.Provider,
		"model":           g.config.Model,
		"port":            port,
		"uptime_seconds":  uptimeSeconds,
		"started_at":      g.startedAt.UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload) //nolint:errcheck
}

// clientIDs returns a map of clientID -> empty struct for JSON serialisation as
// a plain object. The web UI iterates Object.entries() to show individual rows.
func clientIDs(m map[string]*Client) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for id := range m {
		out[id] = struct{}{}
	}
	return out
}

// handleAPISessions returns the list of active gateway sessions as JSON.
func (g *Gateway) handleAPISessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	g.sessionMux.RLock()
	snapshots := make([]map[string]interface{}, 0, len(g.sessions))
	for _, s := range g.sessions {
		snapshots = append(snapshots, map[string]interface{}{
			"id":             s.ID,
			"conversation_id": s.ConversationID,
			"channel":        s.Channel,
			"state":          s.GetState(),
			"message_count":  s.GetMessageCount(),
			"assigned_agent": s.GetAssignedAgent(),
			"created_at":     s.CreatedAt.UTC().Format(time.RFC3339),
			"updated_at":     s.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	g.sessionMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"sessions": snapshots}) //nolint:errcheck
}

// Register registers a new client
func (g *Gateway) Register(client *Client) error {
	g.clientsMux.Lock()
	defer g.clientsMux.Unlock()

	// Check if client already exists
	if _, exists := g.clients[client.ID()]; exists {
		return fmt.Errorf("client %s already registered", client.ID())
	}

	g.clients[client.ID()] = client

	// Add to role-specific map
	g.roleMux.Lock()
	defer g.roleMux.Unlock()

	switch client.Role() {
	case protocol.RoleAgent:
		g.agents[client.ID()] = client
	case protocol.RoleChannel:
		g.channels[client.ID()] = client
	case protocol.RoleUI:
		g.uis[client.ID()] = client
	case protocol.RoleNode:
		g.nodes[client.ID()] = client
	}

	return nil
}

// ValidateAuth validates authentication token if auth is enabled
func (g *Gateway) ValidateAuth(connectFrame *protocol.ConnectFrame) error {
	if !g.authEnabled {
		// Auth disabled, allow connection
		return nil
	}

	// Auth enabled, require token
	if connectFrame.Token == "" {
		return fmt.Errorf("authentication required: no token provided")
	}

	// Validate token
	token, err := g.tokenManager.ValidateToken(connectFrame.Token)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Verify role matches
	if token.Role != connectFrame.Role {
		return fmt.Errorf("role mismatch: token is for %s but client requested %s", token.Role, connectFrame.Role)
	}

	// Verify client ID matches (if token has client ID)
	if token.ClientID != "" && token.ClientID != connectFrame.ClientID {
		return fmt.Errorf("client ID mismatch")
	}

	return nil
}

// Unregister unregisters a client
func (g *Gateway) Unregister(client *Client) {
	g.clientsMux.Lock()
	defer g.clientsMux.Unlock()

	delete(g.clients, client.ID())

	// Remove from role-specific map
	g.roleMux.Lock()
	defer g.roleMux.Unlock()

	switch client.Role() {
	case protocol.RoleAgent:
		delete(g.agents, client.ID())
		// Reset agent load when agent disconnects
		g.router.ResetLoad(client.ID())

		// Unassign sessions that were assigned to this agent
		g.unassignAgentSessions(client.ID())

	case protocol.RoleChannel:
		delete(g.channels, client.ID())
	case protocol.RoleUI:
		delete(g.uis, client.ID())
	case protocol.RoleNode:
		delete(g.nodes, client.ID())
	}

	fmt.Printf("✗ Client disconnected: %s (role: %s)\n", client.ID(), client.Role())
}

// unassignAgentSessions unassigns all sessions assigned to an agent
func (g *Gateway) unassignAgentSessions(agentID string) {
	g.sessionMux.Lock()
	defer g.sessionMux.Unlock()

	for _, session := range g.sessions {
		if session.GetAssignedAgent() == agentID {
			session.UnassignAgent()
			fmt.Printf("Session %s unassigned from disconnected agent %s\n", session.ID, agentID)
		}
	}
}

// Auth helper methods

// GeneratePairingCode generates a new pairing code for device pairing
func (g *Gateway) GeneratePairingCode(clientID string, role protocol.ClientRole, duration time.Duration) (*auth.PairingCode, error) {
	return g.pairingManager.GeneratePairingCode(clientID, role, duration)
}

// PairDevice pairs a device using a pairing code
func (g *Gateway) PairDevice(code, deviceID string) (*auth.Token, error) {
	return g.pairingManager.ValidatePairingCode(code, deviceID)
}

// GenerateToken generates an authentication token
func (g *Gateway) GenerateToken(clientID string, role protocol.ClientRole, duration time.Duration) (*auth.Token, error) {
	return g.tokenManager.GenerateToken(clientID, role, duration)
}

// RevokeToken revokes an authentication token
func (g *Gateway) RevokeToken(tokenValue string) error {
	return g.tokenManager.RevokeToken(tokenValue)
}

// ListActivePairingCodes lists all active pairing codes
func (g *Gateway) ListActivePairingCodes() []*auth.PairingCode {
	return g.pairingManager.ListActiveCodes()
}

// GetAuthStats returns authentication statistics
func (g *Gateway) GetAuthStats() map[string]interface{} {
	return map[string]interface{}{
		"auth_enabled":         g.authEnabled,
		"active_tokens":        g.tokenManager.TokenCount(),
		"active_pairing_codes": g.pairingManager.ActiveCodeCount(),
	}
}

// RouteFrame routes a frame to the appropriate handler
func (g *Gateway) RouteFrame(ctx context.Context, sender *Client, frame interface{}) error {
	switch f := frame.(type) {
	case *protocol.EventMessageFrame:
		return g.handleEventMessage(ctx, sender, f)

	case *protocol.EventToolStartFrame:
		// Log to JSONL
		if f.SessionID != "" {
			if err := g.sessionStorage.LogFrame(f.SessionID, f); err != nil {
				fmt.Printf("Warning: failed to log frame to session: %v\n", err)
			}
		}
		return g.broadcastToUIs(f)

	case *protocol.EventToolEndFrame:
		// Log to JSONL
		if f.SessionID != "" {
			if err := g.sessionStorage.LogFrame(f.SessionID, f); err != nil {
				fmt.Printf("Warning: failed to log frame to session: %v\n", err)
			}
		}
		return g.broadcastToUIs(f)

	case *protocol.EventToolLogFrame:
		// Log to JSONL
		if f.SessionID != "" {
			if err := g.sessionStorage.LogFrame(f.SessionID, f); err != nil {
				fmt.Printf("Warning: failed to log frame to session: %v\n", err)
			}
		}
		return g.broadcastToUIs(f)

	case *protocol.CmdChannelSendFrame:
		return g.handleCmdChannelSend(ctx, sender, f)

	case *protocol.CmdApproveFrame:
		return g.handleCmdApprove(ctx, sender, f)

	case *protocol.CmdRejectFrame:
		return g.handleCmdReject(ctx, sender, f)

	default:
		return fmt.Errorf("unsupported frame type")
	}
}

// handleEventMessage handles incoming messages from channels
func (g *Gateway) handleEventMessage(ctx context.Context, sender *Client, frame *protocol.EventMessageFrame) error {
	// Get or create session
	session := g.GetOrCreateSession(frame.ConversationID, frame.Channel)
	frame.SessionID = session.ID

	// Add message to session history
	session.AddMessage(frame)

	// Log to JSONL
	if err := g.sessionStorage.LogFrame(session.ID, frame); err != nil {
		fmt.Printf("Warning: failed to log frame to session: %v\n", err)
	}

	// Broadcast to UI observers
	g.broadcastToUIs(frame)

	// Get available agents
	g.roleMux.RLock()
	availableAgents := make(map[string]*Client)
	for id, agent := range g.agents {
		availableAgents[id] = agent
	}
	g.roleMux.RUnlock()

	if len(availableAgents) == 0 {
		return fmt.Errorf("no agents available")
	}

	// Use router to select best agent
	agent, err := g.router.SelectAgent(session, availableAgents)
	if err != nil {
		return err
	}

	// Assign agent to session
	session.AssignAgent(agent.ID())

	// Increment agent load
	g.router.IncrementLoad(agent.ID())

	// Send to agent
	if err := agent.Send(frame); err != nil {
		// Decrement load on error
		g.router.DecrementLoad(agent.ID())
		return err
	}

	return nil
}

// handleCmdChannelSend handles send commands from agents
func (g *Gateway) handleCmdChannelSend(ctx context.Context, sender *Client, frame *protocol.CmdChannelSendFrame) error {
	// Log to JSONL if session ID is present
	if frame.SessionID != "" {
		if err := g.sessionStorage.LogFrame(frame.SessionID, frame); err != nil {
			fmt.Printf("Warning: failed to log frame to session: %v\n", err)
		}
	}

	// Find channel client
	g.roleMux.RLock()
	var targetChannel *Client
	for _, channel := range g.channels {
		// TODO: Match by channel name/type in metadata
		// For now, send to first available channel
		targetChannel = channel
		break
	}
	g.roleMux.RUnlock()

	if targetChannel == nil {
		return fmt.Errorf("no channel found: %s", frame.Channel)
	}

	// Broadcast to UI observers
	g.broadcastToUIs(frame)

	return targetChannel.Send(frame)
}

// handleCmdApprove handles approval commands
func (g *Gateway) handleCmdApprove(ctx context.Context, sender *Client, frame *protocol.CmdApproveFrame) error {
	// TODO: Implement approval logic
	return nil
}

// handleCmdReject handles rejection commands
func (g *Gateway) handleCmdReject(ctx context.Context, sender *Client, frame *protocol.CmdRejectFrame) error {
	// TODO: Implement rejection logic
	return nil
}

// broadcastToUIs broadcasts a frame to all UI clients
func (g *Gateway) broadcastToUIs(frame interface{}) error {
	g.roleMux.RLock()
	defer g.roleMux.RUnlock()

	for _, ui := range g.uis {
		if err := ui.Send(frame); err != nil {
			fmt.Printf("Failed to send to UI %s: %v\n", ui.ID(), err)
		}
	}

	return nil
}

// getAvailableAgent returns an available agent client
func (g *Gateway) getAvailableAgent() *Client {
	g.roleMux.RLock()
	defer g.roleMux.RUnlock()

	// Simple round-robin: return first agent
	for _, agent := range g.agents {
		return agent
	}

	return nil
}

// GetOrCreateSession gets or creates a session for a conversation
func (g *Gateway) GetOrCreateSession(conversationID, channel string) *Session {
	g.sessionMux.Lock()
	defer g.sessionMux.Unlock()

	// Create session key
	sessionKey := fmt.Sprintf("%s:%s", channel, conversationID)

	session, exists := g.sessions[sessionKey]
	if !exists {
		session = NewSession(sessionKey, conversationID, channel)
		g.sessions[sessionKey] = session
		fmt.Printf("✓ Created session: %s\n", session.ID)
	}

	return session
}

// handleAPIChat handles HTTP POST /api/chat — runs the orchestrator directly
func (g *Gateway) handleAPIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
		UserID  string `json:"user_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Message == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "message is required"})
		return
	}

	fmt.Printf("📨 /api/chat: %q\n", req.Message)

	g.Notify("message.received", map[string]interface{}{
		"source":  "api",
		"message": req.Message,
		"user_id": req.UserID,
	})

	response, err := g.config.OnChat(r.Context(), req.Message)
	if err != nil {
		fmt.Printf("❌ Chat error: %v\n", err)
		g.Notify("error", map[string]interface{}{
			"source": "api",
			"error":  err.Error(),
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("AI error: %v", err)})
		return
	}

	fmt.Printf("✅ Response: %d chars\n", len(response))
	g.Notify("agent.completed", map[string]interface{}{
		"source":   "api",
		"message":  req.Message,
		"response": response,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"response": response})
}

// corsMiddleware adds CORS headers for cross-origin API access
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetClientCount returns the number of connected clients by role
func (g *Gateway) GetClientCount() map[protocol.ClientRole]int {
	g.roleMux.RLock()
	defer g.roleMux.RUnlock()

	return map[protocol.ClientRole]int{
		protocol.RoleAgent:   len(g.agents),
		protocol.RoleChannel: len(g.channels),
		protocol.RoleUI:      len(g.uis),
		protocol.RoleNode:    len(g.nodes),
	}
}
