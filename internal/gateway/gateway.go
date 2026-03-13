package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/auth"
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
	config *Config
}

// Config holds Gateway configuration
type Config struct {
	Address     string
	Port        int
	SessionsDir string // Directory for session JSONL files
	AuthEnabled bool   // Enable authentication (default: false for backward compatibility)
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

	return &Gateway{
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
	}, nil
}

// Start starts the Gateway server
func (g *Gateway) Start(ctx context.Context) error {
	http.HandleFunc("/ws", g.handleWebSocket)
	http.HandleFunc("/health", g.handleHealth)

	addr := fmt.Sprintf("%s:%d", g.config.Address, g.config.Port)
	fmt.Printf("🚀 Gateway listening on ws://%s/ws\n", addr)

	server := &http.Server{
		Addr:    addr,
		Handler: nil,
	}

	// Shutdown handler
	go func() {
		<-ctx.Done()
		fmt.Println("Shutting down Gateway...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
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

// handleHealth handles health check requests
func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	g.clientsMux.RLock()
	clientCount := len(g.clients)
	g.clientsMux.RUnlock()

	g.sessionMux.RLock()
	sessionCount := len(g.sessions)
	g.sessionMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","clients":%d,"sessions":%d}`, clientCount, sessionCount)
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
