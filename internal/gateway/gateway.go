package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
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

	// API token auth + rate limiting for HTTP endpoints (nil when auth disabled)
	apiAuth    *APIAuth
	apiDevMode bool // bypass auth for localhost when true

	// User management (optional — nil when UsersFile is not configured)
	userManager *auth.UserManager

	// Configuration
	config    *Config
	startedAt time.Time

	// Webhook and notification subsystems (optional — nil until LoadWebhooks called)
	webhookStore      *WebhookStore
	notificationStore *NotificationStore
	inbox             *InboxStore

	// Health monitoring
	monitor *healthMonitor

	// Prometheus-compatible metrics collector
	metrics *MetricsCollector

	// Spawned connectors — tracks connector processes started via the web UI.
	// HTTP-based connectors (Slack, Discord, etc.) don't register as WebSocket
	// clients, so this map is the only way to track them.
	spawnedConnectors   map[string]*SpawnedConnector
	spawnedConnectorsMu sync.RWMutex
}

// SpawnedConnector tracks a connector process launched via POST /api/connectors.
type SpawnedConnector struct {
	Type      string    `json:"type"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // "running", "stopped", "error"
}

// ChatHandler processes a chat message and returns a response.
type ChatHandler func(ctx context.Context, message string) (string, error)

// ThinkingEvent is a real-time event from the agentic loop, sent to the
// web UI via Server-Sent Events so users can watch the AI think.
type ThinkingEvent struct {
	Kind    string `json:"kind"`    // iteration, model_call, model_done, tool_start, tool_done, stream, status, done
	Message string `json:"message"` // human-readable description
	Data    string `json:"data,omitempty"`    // tool name, model name, etc.
	Tokens  int    `json:"tokens,omitempty"`
}

// StreamingChatHandler processes a chat message and streams thinking events.
// If nil, the gateway falls back to OnChat (non-streaming).
type StreamingChatHandler func(ctx context.Context, message string, events chan<- ThinkingEvent) (string, error)

// GatewayAPI holds optional callback functions that the gateway uses to serve
// the rich REST API consumed by the web UI. All fields are optional; when nil
// the corresponding endpoint returns an appropriate empty or "not available"
// response rather than an error, so the gateway degrades gracefully when
// wired up without a full orchestrator.
type GatewayAPI struct {
	// GetConfig returns a sanitised snapshot of the current configuration.
	// Implementations MUST NOT include raw API keys in the returned map.
	GetConfig func() map[string]interface{}

	// SetConfig applies a single key=value configuration update.
	SetConfig func(key, value string) error

	// GetTools returns the list of all known tools with name/description.
	GetTools func() []map[string]interface{}

	// GetAgents returns the list of background agents and their status.
	GetAgents func() []map[string]interface{}

	// GetMemory returns all memory entries from the active memory store.
	GetMemory func() []map[string]interface{}

	// GetCosts returns cost summary data (today, total, by provider, etc.).
	GetCosts func() map[string]interface{}

	// GetAudit returns recent audit events. limit <= 0 defaults to 50.
	GetAudit func(limit int) []map[string]interface{}

	// CreateAgent spawns a new background agent. Returns agent metadata on success.
	CreateAgent func(name, task string) (map[string]interface{}, error)

	// StopAgent cancels a running background agent by ID.
	StopAgent func(id string) error

	// GetAgentDetail returns full observability data for a single agent:
	// identity, status, config, metrics, and the complete activity log.
	GetAgentDetail func(id string) (map[string]interface{}, error)

	// GetAgentLog returns the last N activity log entries for an agent.
	// limit <= 0 should be treated as "return all".
	GetAgentLog func(id string, limit int) ([]map[string]interface{}, error)

	// SetAgentConfig applies configuration overrides to a live agent.
	// The config map uses the same keys as AgentConfig JSON fields.
	SetAgentConfig func(id string, config map[string]interface{}) error

	// SendAgentMessage delivers a message to a running agent's inbox.
	SendAgentMessage func(id string, message string) error

	// ListFiles returns the directory listing at the given workspace-relative
	// path. Each entry is a map with keys: name, is_dir, size.
	ListFiles func(path string) ([]map[string]interface{}, error)

	// ReadFile returns the text content of the workspace-relative file at path.
	ReadFile func(path string) (string, error)

	// ExecCommand executes the given shell command inside the workspace and
	// returns output and exit_code. The broker enforces policy on every call.
	ExecCommand func(command string) (string, int, error)

	// GetScopedPolicies returns all scoped policy rules.
	// Each rule is serialised as a plain map for JSON transport.
	GetScopedPolicies func() []map[string]interface{}

	// AddScopedPolicy appends a new scoped rule to the engine and persists it.
	// The rule is provided as a plain map matching the ScopedRule YAML/JSON schema.
	AddScopedPolicy func(rule map[string]interface{}) error

	// DeleteScopedPolicy removes a scoped rule by name and persists the change.
	DeleteScopedPolicy func(name string) error

	// ListApprovals returns all pending approval requests as plain maps.
	// Each map contains: id, action, resource, reason, requested_by, requested_at,
	// status, expires_at.
	ListApprovals func() []map[string]interface{}

	// ApproveRequest approves the approval request with the given ID.
	// decidedBy is the identifier of the operator making the decision.
	ApproveRequest func(id, decidedBy string) error

	// DenyRequest denies the approval request with the given ID.
	DenyRequest func(id, decidedBy string) error

	// GetHeartbeatStatus returns a snapshot of the heartbeat subsystem state.
	// Returns nil when heartbeat is not wired up.
	GetHeartbeatStatus func() map[string]interface{}

	// RunHeartbeatNow triggers an immediate heartbeat run outside the normal
	// ticker schedule. Returns the raw AI response or an error.
	RunHeartbeatNow func() (string, error)

	// ToggleHeartbeat enables or disables the heartbeat. Returns the new state.
	ToggleHeartbeat func(enabled bool) bool

	// ListProviders returns all available provider names.
	ListProviders func() []string

	// ListModels returns available models for a provider (dynamic + registry).
	ListModels func(provider string) []map[string]interface{}

	// GetAPIKeyStatus returns which providers have API keys configured (without exposing keys).
	GetAPIKeyStatus func() map[string]bool

	// SetAPIKey stores an API key for a provider and persists to config.
	SetAPIKey func(provider, key string) error

	// GetThreads returns all persisted chat threads.
	GetThreads func() ([]map[string]interface{}, error)

	// SaveThread persists a single chat thread (insert or update by id).
	SaveThread func(thread map[string]interface{}) error

	// DeleteThread removes a chat thread by its id.
	DeleteThread func(id string) error

	// SpawnConnector starts a connector process in the background.
	// connectorType is e.g. "telegram", "discord", etc.
	// config is a map of credentials (e.g. {"token": "bot123..."}).
	// Returns a status message or error.
	SpawnConnector func(connectorType string, config map[string]string) (string, error)

	// HubSearch searches the SoulGate Hub for plugins/tools.
	HubSearch func(query string) ([]map[string]interface{}, error)

	// HubInstall installs a plugin from the Hub by name.
	HubInstall func(name string) error

	// HubUninstall removes an installed plugin by name.
	HubUninstall func(name string) error

	// HubInstalled returns the list of installed Hub items.
	HubInstalled func() []map[string]interface{}
}

// Config holds Gateway configuration
type Config struct {
	Address     string
	Port        int
	SessionsDir string      // Directory for session JSONL files
	AuthEnabled bool        // Enable authentication (default: false for backward compatibility)
	OnChat       ChatHandler         // If set, gateway serves POST /api/chat
	OnStreamChat StreamingChatHandler // If set, streams thinking events via SSE

	// Metadata surfaced on the /api/status endpoint and the web UI.
	Provider string // e.g., "anthropic", "openai"
	Model    string // e.g., "claude-opus-4-5"

	// Webhook and notification config files (optional).
	// When non-empty Start() will load them automatically before serving.
	WebhooksFile      string // Path to .soulgate/webhooks.json
	NotificationsFile string // Path to .soulgate/notifications.json

	// API wires up the rich REST endpoints consumed by the web UI.
	// All fields inside GatewayAPI are optional; nil fields are handled
	// gracefully so the gateway works even without a full orchestrator.
	API *GatewayAPI

	// APIAuthEnabled enables Bearer-token authentication + rate limiting on all
	// HTTP /api/* endpoints. Tokens are managed via the `soulgate token` CLI.
	APIAuthEnabled bool

	// APIDevMode, when true, bypasses API auth for requests from localhost.
	// Defaults to true so local development does not require a token.
	APIDevMode bool

	// APIRateLimit is the number of requests per minute allowed per API token.
	// Defaults to 60 when <= 0.
	APIRateLimit float64

	// APITokensFile is the path to the JSON file that persists API tokens.
	// Defaults to .soulgate/api_tokens.json in the workspace config dir.
	// Only used when APIAuthEnabled is true.
	APITokensFile string

	// UsersFile is the directory passed to auth.NewUserManager for user/team
	// persistence. When non-empty, the gateway enables the /api/users and
	// /api/teams endpoints and wires user context into every authenticated
	// request. Typically set to the workspace .soulgate config directory.
	UsersFile string
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

	// Create API auth manager when enabled.
	var apiAuth *APIAuth
	if config.APIAuthEnabled {
		var authErr error
		if config.APITokensFile != "" {
			apiAuth, authErr = NewAPIAuthFromFile(config.APITokensFile, config.APIRateLimit)
		} else {
			apiAuth = NewAPIAuth(config.APIRateLimit)
		}
		if authErr != nil {
			return nil, fmt.Errorf("load api tokens: %w", authErr)
		}
		fmt.Println("API authentication enabled (Bearer token required for /api/* endpoints)")
	}
	// Default dev-mode to true so callers that don't set the field still get
	// the localhost-bypass behaviour that was present before this feature.
	apiDevMode := true
	if config.APIAuthEnabled {
		apiDevMode = config.APIDevMode
	}

	// Initialise the user manager when a config directory is provided.
	var userManager *auth.UserManager
	if config.UsersFile != "" {
		var umErr error
		userManager, umErr = auth.NewUserManager(config.UsersFile)
		if umErr != nil {
			return nil, fmt.Errorf("init user manager: %w", umErr)
		}
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
		apiAuth:        apiAuth,
		apiDevMode:     apiDevMode,
		userManager:    userManager,
		config:         config,
		startedAt:      time.Now(),
		metrics:           newMetricsCollector(),
		spawnedConnectors: make(map[string]*SpawnedConnector),
	}
	gw.monitor = newHealthMonitor(gw)
	return gw, nil
}

// GetAPIAuth returns the API authentication manager. Returns nil when API auth
// is not enabled.
func (g *Gateway) GetAPIAuth() *APIAuth {
	return g.apiAuth
}

// TrackConnector registers a spawned connector process so it shows up
// in the connectors status endpoint even if it uses HTTP (not WebSocket).
func (g *Gateway) TrackConnector(connectorType string, pid int) {
	g.spawnedConnectorsMu.Lock()
	defer g.spawnedConnectorsMu.Unlock()
	g.spawnedConnectors[connectorType] = &SpawnedConnector{
		Type:      connectorType,
		PID:       pid,
		StartedAt: time.Now(),
		Status:    "running",
	}
}

// refreshSpawnedConnectors checks if spawned processes are still alive.
func (g *Gateway) refreshSpawnedConnectors() {
	g.spawnedConnectorsMu.Lock()
	defer g.spawnedConnectorsMu.Unlock()
	for key, sc := range g.spawnedConnectors {
		if sc.Status != "running" {
			continue
		}
		proc, err := os.FindProcess(sc.PID)
		if err != nil {
			delete(g.spawnedConnectors, key)
			continue
		}
		// On Unix, FindProcess always succeeds; send signal 0 to probe.
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			delete(g.spawnedConnectors, key)
		}
	}
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

	// Initialize persistent inbox for user-facing notifications.
	inboxPath := filepath.Join(g.config.SessionsDir, "inbox.json")
	g.inbox = NewInboxStore(inboxPath)

	mux := http.NewServeMux()

	// WebSocket endpoint — never gated by API auth (uses its own token scheme)
	mux.HandleFunc("/ws", g.handleWebSocket)

	// Health endpoints — always public so load-balancers can probe without a token
	mux.HandleFunc("/health", g.handleHealth)
	mux.HandleFunc("/api/health", g.handleHealth)

	// apiHandler optionally wraps an http.HandlerFunc with API auth middleware
	// and, when a UserManager is configured, resolves the caller's User from
	// their sg_user_ API key and stores it in the request context.
	// Endpoints that should be public (health, ws) are registered directly above;
	// all /api/* endpoints below are run through this helper.
	apiHandler := func(h http.HandlerFunc) http.Handler {
		// Build the chain from innermost to outermost.
		// Execution order: userAuthMiddleware → apiAuthMiddleware → h
		// (outer middleware runs first).
		var base http.Handler = h
		// 1. User context enrichment runs closest to the handler so it can
		//    read the validated bearer token after apiAuthMiddleware passes it.
		base = userAuthMiddleware(g.userManager, base)
		// 2. Token validation / rate limiting is the outermost gate.
		if g.apiAuth != nil {
			base = apiAuthMiddleware(g.apiAuth, g.apiDevMode, base)
		}
		return base
	}

	// REST API
	mux.Handle("/api/status", apiHandler(g.handleAPIStatus))
	mux.Handle("/api/sessions", apiHandler(g.handleAPISessions))
	mux.Handle("/api/sessions/", apiHandler(g.handleAPISessionDetail))
	mux.Handle("/api/connectors", apiHandler(g.handleAPIConnectors))
	mux.Handle("/api/connectors/", apiHandler(g.handleAPIConnectorAction))
	mux.Handle("/api/hub", apiHandler(g.handleAPIHub))
	mux.Handle("/api/hub/", apiHandler(g.handleAPIHub))
	mux.Handle("/api/activity", apiHandler(g.handleAPIActivity))

	// Rich web-UI endpoints — always registered; handlers degrade gracefully
	// when the corresponding GatewayAPI callbacks are not wired up.
	mux.Handle("/api/providers", apiHandler(g.handleAPIProviders))
	mux.Handle("/api/providers/", apiHandler(g.handleAPIProviderModels))
	mux.Handle("/api/apikeys", apiHandler(g.handleAPIKeys))
	mux.Handle("/api/config", apiHandler(g.handleAPIConfig))
	mux.Handle("/api/tools", apiHandler(g.handleAPITools))
	mux.Handle("/api/agents", apiHandler(g.handleAPIAgents))
	mux.Handle("/api/agents/", apiHandler(g.handleAPIAgentDetail))
	mux.Handle("/api/agent", apiHandler(g.handleAPIAgent))
	mux.Handle("/api/memory", apiHandler(g.handleAPIMemory))
	mux.Handle("/api/costs", apiHandler(g.handleAPICosts))
	mux.Handle("/api/audit", apiHandler(g.handleAPIAudit))
	mux.Handle("/api/notifications", apiHandler(g.handleAPINotifications))
	mux.Handle("/api/notifications/", apiHandler(g.handleAPINotificationDetail))
	mux.Handle("/api/files", apiHandler(g.handleAPIFiles))
	mux.Handle("/api/file", apiHandler(g.handleAPIFile))
	mux.Handle("/api/exec", apiHandler(g.handleAPIExec))
	mux.Handle("/api/policies", apiHandler(g.handleAPIPolicies))
	mux.Handle("/api/policies/", apiHandler(g.handleAPIPolicies))
	mux.Handle("/api/approvals", apiHandler(g.handleAPIApprovals))
	mux.Handle("/api/approvals/", apiHandler(g.handleAPIApprovals))
	mux.Handle("/api/heartbeat", apiHandler(g.handleAPIHeartbeat))
	mux.Handle("/api/heartbeat/run", apiHandler(g.handleAPIHeartbeatRun))
	mux.Handle("/api/threads", apiHandler(g.handleAPIThreads))
	mux.Handle("/api/threads/", apiHandler(g.handleAPIThreads))

	// Serve the HTTP chat API if a chat handler is configured
	if g.config.OnChat != nil {
		mux.Handle("/api/chat", apiHandler(g.handleAPIChat))
		fmt.Println("HTTP API enabled: POST /api/chat")
	}

	// User and team management endpoints — active whenever a UserManager is configured.
	if g.userManager != nil {
		mux.Handle("/api/users", apiHandler(g.handleAPIUsers))
		mux.Handle("/api/users/", apiHandler(g.handleAPIUserDetail))
		mux.Handle("/api/teams", apiHandler(g.handleAPITeams))
		mux.Handle("/api/teams/", apiHandler(g.handleAPITeamDetail))
		fmt.Println("User management enabled: /api/users, /api/teams")
	}

	// Inbound webhooks — enabled whenever a webhook store is present
	if g.webhookStore != nil {
		mux.HandleFunc("/webhook/", g.handleWebhook)
		fmt.Println("Webhooks enabled: POST /webhook/{name}")
	}

	// Prometheus metrics endpoint
	mux.HandleFunc("/metrics", g.handleMetrics)

	// Web UI — React app served from embedded dist/ directory
	if uiFS, err := webui.Assets(); err == nil {
		mux.Handle("/", http.FileServer(http.FS(uiFS)))
	}

	addr := fmt.Sprintf("%s:%d", g.config.Address, g.config.Port)
	fmt.Printf("Gateway listening on http://%s  (WebSocket: ws://%s/ws)\n", addr, addr)

	// Wrap the mux with metrics middleware so every HTTP request is counted
	// by endpoint path, then apply CORS on top.
	server := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(metricsMiddleware(mux, g.metrics)),
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

	// Push notification for connector connections
	if connectFrame.Role == protocol.RoleChannel {
		ch := "channel"
		if v, ok := connectFrame.Metadata["channel"].(string); ok && v != "" {
			ch = v
		}
		detail := fmt.Sprintf("Client %s", clientID)
		if v, ok := connectFrame.Metadata["bot_username"].(string); ok && v != "" {
			detail = fmt.Sprintf("@%s (%s)", v, clientID)
		}
		g.PushNotification("connection", ch+" connector online", detail)
	}

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
		"provider":        g.liveProvider(),
		"model":           g.liveModel(),
		"port":            port,
		"uptime_seconds":  uptimeSeconds,
		"started_at":      g.startedAt.UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload) //nolint:errcheck
}

// liveProvider returns the current provider name from the live config API,
// falling back to the startup value if the API is not wired up.
func (g *Gateway) liveProvider() string {
	if g.config.API != nil && g.config.API.GetConfig != nil {
		cfg := g.config.API.GetConfig()
		if p, ok := cfg["provider"].(string); ok && p != "" {
			return p
		}
	}
	return g.config.Provider
}

// liveModel returns the current model name from the live config API,
// falling back to the startup value if the API is not wired up.
func (g *Gateway) liveModel() string {
	if g.config.API != nil && g.config.API.GetConfig != nil {
		cfg := g.config.API.GetConfig()
		if m, ok := cfg["model"].(string); ok && m != "" {
			return m
		}
	}
	return g.config.Model
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
			"last_activity":  s.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	g.sessionMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"sessions": snapshots}) //nolint:errcheck
}

// handleAPISessionDetail returns messages for a specific session.
// GET /api/sessions/{id} → session messages
func (g *Gateway) handleAPISessionDetail(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path: /api/sessions/{id}
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID = strings.TrimSuffix(sessionID, "/")
	if sessionID == "" {
		http.Error(w, `{"error":"session ID required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		g.handleAPISessionDetailGet(w, sessionID)
	case http.MethodPost:
		g.handleAPISessionReply(w, r, sessionID)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handleAPISessionDetailGet(w http.ResponseWriter, sessionID string) {
	// Read session entries from storage
	if g.sessionStorage == nil {
		http.Error(w, `{"error":"session storage not available"}`, http.StatusServiceUnavailable)
		return
	}

	entries, err := g.sessionStorage.ReadSession(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Get session metadata from in-memory state
	g.sessionMux.RLock()
	sess := g.sessions[sessionID]
	var meta map[string]interface{}
	if sess != nil {
		meta = map[string]interface{}{
			"id":              sess.ID,
			"conversation_id": sess.ConversationID,
			"channel":         sess.Channel,
			"state":           sess.GetState(),
			"message_count":   sess.GetMessageCount(),
			"assigned_agent":  sess.GetAssignedAgent(),
			"created_at":      sess.CreatedAt.UTC().Format(time.RFC3339),
			"last_activity":   sess.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	g.sessionMux.RUnlock()

	// Convert entries to JSON-friendly format
	messages := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		messages = append(messages, map[string]interface{}{
			"ts":   e.Timestamp,
			"type": e.Type,
			"data": e.Data,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"meta":       meta,
		"messages":   messages,
	}) //nolint:errcheck
}

// handleAPISessionReply sends a message from the web UI into an existing
// connector session. The message is processed by the built-in chat handler
// (the orchestrator) and the reply is sent back to the original channel.
// POST /api/sessions/{id} { "message": "..." }
func (g *Gateway) handleAPISessionReply(w http.ResponseWriter, r *http.Request, sessionID string) {
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if body.Message == "" {
		http.Error(w, `{"error":"message is required"}`, http.StatusBadRequest)
		return
	}

	// Look up session — if not in memory, reconstruct from the session ID.
	// Session IDs are formatted as "channel:conversationID" (e.g. "telegram:12345").
	// After a gateway restart the in-memory map is empty but the JSONL files persist.
	g.sessionMux.RLock()
	sess := g.sessions[sessionID]
	g.sessionMux.RUnlock()

	if sess == nil {
		// Try to reconstruct from session ID format
		parts := strings.SplitN(sessionID, ":", 2)
		if len(parts) == 2 {
			sess = g.GetOrCreateSession(parts[1], parts[0])
		}
	}

	if sess == nil {
		http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
		return
	}

	// Find the channel client that matches this session's channel
	g.roleMux.RLock()
	var targetChannel *Client
	for _, c := range g.channels {
		if c.metadata["channel"] == sess.Channel {
			targetChannel = c
			break
		}
	}
	g.roleMux.RUnlock()

	if targetChannel == nil {
		http.Error(w, fmt.Sprintf(`{"error":"no connected %s channel"}`, sess.Channel), http.StatusServiceUnavailable)
		return
	}

	// Log the operator's input message to the session so it appears in the activity feed.
	operatorMsg := &protocol.EventMessageFrame{
		Type:           protocol.FrameEventMessage,
		Channel:        sess.Channel,
		ConversationID: sess.ConversationID,
		Text:           body.Message,
		SessionID:      sess.ID,
		Sender: protocol.Sender{
			ID:       "operator",
			Name:     "Operator",
			Username: "Web UI",
		},
		Timestamp: time.Now().Unix(),
	}
	if g.sessionStorage != nil {
		_ = g.sessionStorage.LogFrame(sess.ID, operatorMsg)
	}
	sess.AddMessage(operatorMsg)
	g.broadcastToUIs(operatorMsg)

	// Send the operator's message to the channel so the user sees it in Telegram/Discord/etc.
	operatorRelay := &protocol.CmdChannelSendFrame{
		Type:           protocol.FrameCmdChannelSend,
		Channel:        sess.Channel,
		ConversationID: sess.ConversationID,
		Text:           "[Operator]: " + body.Message,
		SessionID:      sess.ID,
		Timestamp:      time.Now().Unix(),
	}
	_ = targetChannel.Send(operatorRelay)

	// Process through the built-in chat handler if available
	responseText := body.Message
	if g.config.OnChat != nil {
		resp, err := g.config.OnChat(r.Context(), body.Message)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
			return
		}
		responseText = resp
	}

	// Build reply frame and send to the connector channel
	reply := &protocol.CmdChannelSendFrame{
		Type:           protocol.FrameCmdChannelSend,
		Channel:        sess.Channel,
		ConversationID: sess.ConversationID,
		Text:           responseText,
		SessionID:      sess.ID,
		Timestamp:      time.Now().Unix(),
	}

	// Log the AI reply
	if g.sessionStorage != nil {
		if err := g.sessionStorage.LogFrame(sess.ID, reply); err != nil {
			fmt.Printf("Warning: failed to log reply to session: %v\n", err)
		}
	}
	sess.AddMessage(reply)
	g.broadcastToUIs(reply)

	// Send to channel client (this delivers the AI reply to Telegram/Discord/etc.)
	if err := targetChannel.Send(reply); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to send to channel: " + err.Error()}) //nolint:errcheck
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "sent",
		"response": responseText,
	}) //nolint:errcheck
}

// handleAPIConnectors returns the status of all connected clients grouped by channel.
// GET  /api/connectors → connector status
// POST /api/connectors → spawn a connector process
func (g *Gateway) handleAPIConnectors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		g.handleAPIConnectorsGet(w, r)
	case http.MethodPost:
		g.handleAPIConnectorsPost(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handleAPIConnectorsGet(w http.ResponseWriter, _ *http.Request) {
	// Gather connected channel clients
	g.roleMux.RLock()
	channelClients := make([]map[string]interface{}, 0, len(g.channels))
	for _, c := range g.channels {
		channelClients = append(channelClients, map[string]interface{}{
			"client_id": c.ID(),
			"channel":   c.metadata["channel"],
			"metadata":  c.metadata,
		})
	}

	agentClients := make([]map[string]interface{}, 0, len(g.agents))
	for _, c := range g.agents {
		agentClients = append(agentClients, map[string]interface{}{
			"client_id": c.ID(),
			"metadata":  c.metadata,
		})
	}

	uiClients := make([]map[string]interface{}, 0, len(g.uis))
	for _, c := range g.uis {
		uiClients = append(uiClients, map[string]interface{}{
			"client_id": c.ID(),
		})
	}
	g.roleMux.RUnlock()

	// Count sessions per channel
	g.sessionMux.RLock()
	channelSessions := make(map[string]int)
	for _, s := range g.sessions {
		channelSessions[s.Channel]++
	}
	g.sessionMux.RUnlock()

	// Include spawned connector processes (HTTP-based connectors that
	// don't show up as WebSocket clients)
	g.refreshSpawnedConnectors()
	g.spawnedConnectorsMu.RLock()
	spawned := make([]map[string]interface{}, 0, len(g.spawnedConnectors))
	for _, sc := range g.spawnedConnectors {
		spawned = append(spawned, map[string]interface{}{
			"type":       sc.Type,
			"pid":        sc.PID,
			"started_at": sc.StartedAt.UTC().Format(time.RFC3339),
			"status":     sc.Status,
		})
	}
	g.spawnedConnectorsMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"channels":            channelClients,
		"agents":              agentClients,
		"uis":                 uiClients,
		"sessions_by_channel": channelSessions,
		"spawned":             spawned,
	}) //nolint:errcheck
}

func (g *Gateway) handleAPIConnectorsPost(w http.ResponseWriter, r *http.Request) {
	if g.config.API == nil || g.config.API.SpawnConnector == nil {
		http.Error(w, `{"error":"connector spawning not available"}`, http.StatusNotImplemented)
		return
	}

	var body struct {
		Type   string            `json:"type"`
		Config map[string]string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if body.Type == "" {
		http.Error(w, `{"error":"type is required"}`, http.StatusBadRequest)
		return
	}

	msg, err := g.config.API.SpawnConnector(body.Type, body.Config)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
		return
	}

	// Extract PID from message (format: "... (pid 12345)") and track it.
	if pidIdx := strings.Index(msg, "(pid "); pidIdx >= 0 {
		var pid int
		if _, scanErr := fmt.Sscanf(msg[pidIdx:], "(pid %d)", &pid); scanErr == nil && pid > 0 {
			g.TrackConnector(body.Type, pid)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "spawned",
		"message": msg,
	}) //nolint:errcheck
}

// handleAPIHub handles Hub operations: search, install, uninstall, list installed.
//   GET    /api/hub?q=...         → search hub
//   GET    /api/hub/installed     → list installed
//   POST   /api/hub/install       → install plugin
//   POST   /api/hub/uninstall     → uninstall plugin
func (g *Gateway) handleAPIHub(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/hub")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && (path == "" || path == "/"):
		// Search
		query := r.URL.Query().Get("q")
		if g.config.API == nil || g.config.API.HubSearch == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}, "error": "hub not configured"}) //nolint:errcheck
			return
		}
		results, err := g.config.API.HubSearch(query)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}, "error": err.Error()}) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"results": results}) //nolint:errcheck

	case r.Method == http.MethodGet && path == "installed":
		if g.config.API == nil || g.config.API.HubInstalled == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"installed": []interface{}{}}) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"installed": g.config.API.HubInstalled()}) //nolint:errcheck

	case r.Method == http.MethodPost && path == "install":
		if g.config.API == nil || g.config.API.HubInstall == nil {
			http.Error(w, `{"error":"hub install not available"}`, http.StatusNotImplemented)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		if err := g.config.API.HubInstall(body.Name); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "installed", "name": body.Name}) //nolint:errcheck

	case r.Method == http.MethodPost && path == "uninstall":
		if g.config.API == nil || g.config.API.HubUninstall == nil {
			http.Error(w, `{"error":"hub uninstall not available"}`, http.StatusNotImplemented)
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
			return
		}
		if err := g.config.API.HubUninstall(body.Name); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "uninstalled", "name": body.Name}) //nolint:errcheck

	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

// handleAPIConnectorAction handles per-connector actions.
// DELETE /api/connectors/{type} → stop/disconnect a connector
func (g *Gateway) handleAPIConnectorAction(w http.ResponseWriter, r *http.Request) {
	connectorType := strings.TrimPrefix(r.URL.Path, "/api/connectors/")
	connectorType = strings.TrimSuffix(connectorType, "/")
	if connectorType == "" {
		http.Error(w, `{"error":"connector type required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		g.handleDisconnectConnector(w, connectorType)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleDisconnectConnector kills a spawned connector process and removes
// any matching WebSocket clients.
func (g *Gateway) handleDisconnectConnector(w http.ResponseWriter, connectorType string) {
	killed := 0

	// Kill spawned process if tracked
	g.spawnedConnectorsMu.Lock()
	if sc, ok := g.spawnedConnectors[connectorType]; ok && sc.Status == "running" {
		if proc, err := os.FindProcess(sc.PID); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			killed++
		}
		delete(g.spawnedConnectors, connectorType)
	}
	g.spawnedConnectorsMu.Unlock()

	// Disconnect any WebSocket channel clients for this connector type
	g.roleMux.RLock()
	var toDisconnect []*Client
	for _, c := range g.channels {
		ch, _ := c.metadata["channel"].(string)
		if strings.EqualFold(ch, connectorType) {
			toDisconnect = append(toDisconnect, c)
		}
	}
	g.roleMux.RUnlock()

	for _, c := range toDisconnect {
		_ = c.Close()
		g.Unregister(c)
		killed++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "disconnected",
		"type":         connectorType,
		"killed_count": killed,
	}) //nolint:errcheck
}

// handleAPIActivity returns a unified activity feed across all sessions.
// GET /api/activity?limit=50&channel=telegram → recent activity
func (g *Gateway) handleAPIActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit > 200 {
			limit = 200
		}
	}
	channelFilter := r.URL.Query().Get("channel")

	if g.sessionStorage == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"activity": []interface{}{}}) //nolint:errcheck
		return
	}

	// List all sessions and gather recent entries
	sessionIDs, err := g.sessionStorage.ListSessions()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}

	type activityEntry struct {
		SessionID string      `json:"session_id"`
		Channel   string      `json:"channel"`
		Timestamp int64       `json:"ts"`
		Type      string      `json:"type"`
		Data      interface{} `json:"data"`
	}

	var allEntries []activityEntry
	for _, sid := range sessionIDs {
		// Apply channel filter
		if channelFilter != "" && !strings.HasPrefix(sid, channelFilter+":") && !strings.HasPrefix(sid, channelFilter) {
			continue
		}

		// Extract channel from session ID (format: channel:conversationID)
		channel := sid
		if idx := strings.Index(sid, ":"); idx > 0 {
			channel = sid[:idx]
		}

		entries, err := g.sessionStorage.ReadSession(sid)
		if err != nil {
			continue
		}

		for _, e := range entries {
			allEntries = append(allEntries, activityEntry{
				SessionID: sid,
				Channel:   channel,
				Timestamp: e.Timestamp,
				Type:      e.Type,
				Data:      e.Data,
			})
		}
	}

	// Sort by timestamp descending (most recent first)
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp > allEntries[j].Timestamp
	})

	// Limit results
	if len(allEntries) > limit {
		allEntries = allEntries[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"activity": allEntries}) //nolint:errcheck
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

	// Push notification for new messages
	sender_name := frame.Sender.Username
	if sender_name == "" {
		sender_name = frame.Sender.Name
	}
	if sender_name == "" {
		sender_name = frame.Sender.ID
	}
	msgPreview := frame.Text
	if len(msgPreview) > 80 {
		msgPreview = msgPreview[:80] + "..."
	}
	g.PushNotification("activity", fmt.Sprintf("Message from %s", sender_name),
		fmt.Sprintf("[%s] %s", frame.Channel, msgPreview))

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
		// No WebSocket agents connected — fall back to built-in chat handler.
		// This runs the orchestrator directly and sends the reply back to the channel.
		if g.config.OnChat != nil {
			go g.handleBuiltinChat(ctx, sender, session, frame)
			return nil
		}
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

// handleBuiltinChat processes a channel message through the built-in OnChat
// handler (the orchestrator) and sends the reply back to the originating channel.
// This is the fallback path when no dedicated WebSocket agent is connected.
func (g *Gateway) handleBuiltinChat(ctx context.Context, sender *Client, session *Session, frame *protocol.EventMessageFrame) {
	response, err := g.config.OnChat(ctx, frame.Text)
	if err != nil {
		fmt.Printf("Warning: built-in chat handler error for %s: %v\n", session.ID, err)
		response = fmt.Sprintf("Sorry, I encountered an error: %v", err)
	}

	// Build reply frame
	reply := &protocol.CmdChannelSendFrame{
		Type:           protocol.FrameCmdChannelSend,
		Channel:        frame.Channel,
		ConversationID: frame.ConversationID,
		Text:           response,
		SessionID:      session.ID,
		Timestamp:      time.Now().Unix(),
	}

	// Log the reply to the session JSONL
	if err := g.sessionStorage.LogFrame(session.ID, reply); err != nil {
		fmt.Printf("Warning: failed to log reply to session: %v\n", err)
	}

	// Add to in-memory session
	session.AddMessage(reply)

	// Broadcast to UI observers
	g.broadcastToUIs(reply)

	// Send back to the originating channel client
	if err := sender.Send(reply); err != nil {
		fmt.Printf("Warning: failed to send reply to channel %s: %v\n", frame.Channel, err)
	}
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
		Message        string `json:"message"`
		UserID         string `json:"user_id,omitempty"`
		Channel        string `json:"channel,omitempty"`
		ConversationID string `json:"conversation_id,omitempty"`
		SenderID       string `json:"sender_id,omitempty"`
		SenderName     string `json:"sender_name,omitempty"`
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

	// If the request has connector metadata (channel, conversation_id),
	// log the incoming message and response to the session system so they
	// appear in the Activity Hub.
	var sess *Session
	if req.Channel != "" && req.ConversationID != "" {
		sess = g.GetOrCreateSession(req.ConversationID, req.Channel)

		// Log incoming message
		inMsg := &protocol.EventMessageFrame{
			Type:           protocol.FrameEventMessage,
			Channel:        req.Channel,
			ConversationID: req.ConversationID,
			Text:           req.Message,
			SessionID:      sess.ID,
			Sender: protocol.Sender{
				ID:       req.SenderID,
				Name:     req.SenderName,
				Username: req.SenderName,
			},
			Timestamp: time.Now().Unix(),
		}
		if g.sessionStorage != nil {
			_ = g.sessionStorage.LogFrame(sess.ID, inMsg)
		}
		sess.AddMessage(inMsg)
		g.broadcastToUIs(inMsg)

		// Push notification
		senderName := req.SenderName
		if senderName == "" {
			senderName = req.SenderID
		}
		preview := req.Message
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		g.PushNotification("activity", fmt.Sprintf("Message from %s", senderName),
			fmt.Sprintf("[%s] %s", req.Channel, preview))
	}

	// Check if client wants SSE streaming (Accept: text/event-stream or ?stream=true)
	wantsStream := r.Header.Get("Accept") == "text/event-stream" ||
		r.URL.Query().Get("stream") == "true"

	if wantsStream && g.config.OnStreamChat != nil {
		g.handleStreamingChat(w, r, req.Message)
		return
	}

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

	// Log the AI response to the session
	if sess != nil {
		replyFrame := &protocol.CmdChannelSendFrame{
			Type:           protocol.FrameCmdChannelSend,
			Channel:        req.Channel,
			ConversationID: req.ConversationID,
			Text:           response,
			SessionID:      sess.ID,
			Timestamp:      time.Now().Unix(),
		}
		if g.sessionStorage != nil {
			_ = g.sessionStorage.LogFrame(sess.ID, replyFrame)
		}
		sess.AddMessage(replyFrame)
		g.broadcastToUIs(replyFrame)
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

// handleStreamingChat sends thinking events as Server-Sent Events (SSE)
// so the web UI can show real-time AI thinking with animations.
func (g *Gateway) handleStreamingChat(w http.ResponseWriter, r *http.Request, message string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Fallback to non-streaming
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	events := make(chan ThinkingEvent, 100)

	go func() {
		response, err := g.config.OnStreamChat(r.Context(), message, events)
		if err != nil {
			events <- ThinkingEvent{Kind: "error", Message: err.Error()}
		} else {
			events <- ThinkingEvent{Kind: "done", Message: response}
		}
		close(events)
	}()

	for evt := range events {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// corsMiddleware adds CORS headers for cross-origin API access
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- Rich web-UI API handlers -----------------------------------------
//
// All handlers below check whether the relevant GatewayAPI callback is wired
// up. When nil they return a well-formed JSON response indicating the feature
// is not available rather than a 500. This keeps the gateway operational even
// when launched without a full orchestrator.

// writeJSON is a small helper that sets Content-Type and encodes v as JSON.
func writeGatewayJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// handleAPIProviders returns all available providers from the live catalog.
// GET /api/providers → [{id, name, models}]
func (g *Gateway) handleAPIProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.ListProviders == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"providers": []string{}})
		return
	}
	providers := g.config.API.ListProviders()
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}

// handleAPIProviderModels returns models for a specific provider.
// GET /api/providers/{name} → {provider, models: [{id, name, description}]}
func (g *Gateway) handleAPIProviderModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	provider := strings.TrimPrefix(r.URL.Path, "/api/providers/")
	provider = strings.TrimSuffix(provider, "/")
	if provider == "" {
		http.Error(w, `{"error":"provider name required"}`, http.StatusBadRequest)
		return
	}
	if g.config.API == nil || g.config.API.ListModels == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"provider": provider, "models": []interface{}{}})
		return
	}
	models := g.config.API.ListModels(provider)
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"provider": provider, "models": models})
}

// handleAPIKeys handles API key management.
// GET  /api/apikeys → which providers have keys (no values exposed)
// POST /api/apikeys → {"provider":"openai","key":"sk-..."} to save a key
func (g *Gateway) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if g.config.API == nil || g.config.API.GetAPIKeyStatus == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"keys": map[string]bool{}})
			return
		}
		status := g.config.API.GetAPIKeyStatus()
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"keys": status})

	case http.MethodPost:
		if g.config.API == nil || g.config.API.SetAPIKey == nil {
			http.Error(w, `{"error":"API key management not available"}`, http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Provider string `json:"provider"`
			Key      string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if req.Provider == "" || req.Key == "" {
			http.Error(w, `{"error":"provider and key are required"}`, http.StatusBadRequest)
			return
		}
		if err := g.config.API.SetAPIKey(req.Provider, req.Key); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "saved"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAPIConfig handles GET /api/config and POST /api/config.
//
// GET  — returns a sanitised configuration snapshot (no API keys).
// POST — accepts {"key":"...", "value":"..."} to update a single setting.
func (g *Gateway) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if g.config.API == nil || g.config.API.GetConfig == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
				"available": false,
				"message":   "config API not wired up",
			})
			return
		}
		cfg := g.config.API.GetConfig()
		writeGatewayJSON(w, http.StatusOK, cfg)

	case http.MethodPost, http.MethodPut:
		if g.config.API == nil || g.config.API.SetConfig == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "config update not available",
			})
			return
		}
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Key == "" {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "key is required"})
			return
		}
		if err := g.config.API.SetConfig(req.Key, req.Value); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		// Push notification for config changes
		g.PushNotification("agent", fmt.Sprintf("Config: %s changed", req.Key), fmt.Sprintf("Set to: %s", req.Value))
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "updated"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAPITools handles GET /api/tools.
// Returns the complete list of available tools with name and description.
func (g *Gateway) handleAPITools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.GetTools == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"tools":     []interface{}{},
			"available": false,
		})
		return
	}
	tools := g.config.API.GetTools()
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	})
}

// handleAPIAgents handles GET /api/agents (list) and POST /api/agents (create).
func (g *Gateway) handleAPIAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if g.config.API == nil || g.config.API.GetAgents == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
				"agents":    []interface{}{},
				"available": false,
			})
			return
		}
		agents := g.config.API.GetAgents()
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"agents": agents,
			"count":  len(agents),
		})

	case http.MethodPost:
		// Create agent — same logic as POST /api/agent
		g.handleAPIAgent(w, r)

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAPIAgent handles POST /api/agent (create) and DELETE /api/agent/{id} (stop).
//
// POST   — body: {"name":"...", "task":"..."}
// DELETE — URL path: /api/agent/{id}
func (g *Gateway) handleAPIAgent(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if g.config.API == nil || g.config.API.CreateAgent == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "agent creation not available",
			})
			return
		}
		var req struct {
			Name string `json:"name"`
			Task string `json:"task"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Task == "" {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "task is required"})
			return
		}
		if req.Name == "" {
			req.Name = req.Task
			if len(req.Name) > 40 {
				req.Name = req.Name[:40]
			}
		}
		agent, err := g.config.API.CreateAgent(req.Name, req.Task)
		if err != nil {
			writeGatewayJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusCreated, agent)

	case http.MethodDelete:
		if g.config.API == nil || g.config.API.StopAgent == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "agent stop not available",
			})
			return
		}
		// Extract agent ID from URL: /api/agent/{id}
		id := strings.TrimPrefix(r.URL.Path, "/api/agent/")
		id = strings.TrimSpace(id)
		if id == "" || id == "agent" {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "agent id is required in path: /api/agent/{id}"})
			return
		}
		if err := g.config.API.StopAgent(id); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "stopped", "id": id})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAPIMemory handles GET /api/memory.
// Returns all memory entries from the active memory store.
func (g *Gateway) handleAPIMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.GetMemory == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"entries":   []interface{}{},
			"available": false,
		})
		return
	}
	entries := g.config.API.GetMemory()
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

// handleAPICosts handles GET /api/costs.
// Returns cost summary data: today, total, by provider, last 7 days.
func (g *Gateway) handleAPICosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.GetCosts == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"available": false,
			"message":   "cost tracking not available",
		})
		return
	}
	costs := g.config.API.GetCosts()
	writeGatewayJSON(w, http.StatusOK, costs)
}

// ── Persistent Notification Inbox ────────────────────────────────────────────

func (g *Gateway) handleAPINotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		entries := g.inbox.SortedByTime()
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"notifications": entries,
			"unread":        g.inbox.UnreadCount(),
			"total":         len(entries),
		})
	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
			Kind   string `json:"kind,omitempty"`
			Title  string `json:"title,omitempty"`
			Detail string `json:"detail,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "push":
			if req.Title == "" {
				http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
				return
			}
			entry := g.inbox.Push(req.Kind, req.Title, req.Detail, nil)
			writeGatewayJSON(w, http.StatusOK, entry)
		case "mark_all_read":
			g.inbox.MarkAllRead()
			writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		case "clear_read":
			for _, e := range g.inbox.List() {
				if e.Read && !e.Pinned {
					g.inbox.Delete(e.ID)
				}
			}
			writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		default:
			http.Error(w, `{"error":"use action: push, mark_all_read, clear_read"}`, http.StatusBadRequest)
		}
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (g *Gateway) handleAPINotificationDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		entry, ok := g.inbox.Get(id)
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		writeGatewayJSON(w, http.StatusOK, entry)
	case http.MethodPost:
		g.inbox.MarkRead(id)
		entry, _ := g.inbox.Get(id)
		writeGatewayJSON(w, http.StatusOK, entry)
	case http.MethodDelete:
		g.inbox.Delete(id)
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// PushNotification adds a notification to the persistent inbox.
func (g *Gateway) PushNotification(kind, title, detail string) {
	if g.inbox != nil {
		g.inbox.Push(kind, title, detail, nil)
	}
}

// handleAPIAudit handles GET /api/audit?limit=N.
// Returns the most recent audit events, newest first.
// The optional "limit" query parameter caps results (default 50, max 500).
func (g *Gateway) handleAPIAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.GetAudit == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"events":    []interface{}{},
			"available": false,
		})
		return
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := fmt.Sscanf(raw, "%d", &limit); parsed != 1 || err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit parameter"})
			return
		}
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	events := g.config.API.GetAudit(limit)
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

// handleAPIFiles handles GET /api/files?path=<workspace-relative-path>.
// Returns a JSON array of directory entries with name, is_dir, and size.
func (g *Gateway) handleAPIFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.ListFiles == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"entries":   []interface{}{},
			"available": false,
		})
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	entries, err := g.config.API.ListFiles(path)
	if err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"path":    path,
	})
}

// handleAPIFile handles GET /api/file?path=<workspace-relative-path>.
// Returns the raw text content of the requested file.
func (g *Gateway) handleAPIFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.ReadFile == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "file read API not available",
		})
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	content, err := g.config.API.ReadFile(path)
	if err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"path":    path,
		"content": content,
	})
}

// handleAPIAgentDetail is the router for the /api/agents/{id} subtree.
//
// Routes dispatched:
//   GET    /api/agents/{id}         → full agent detail (identity + metrics + config + log)
//   GET    /api/agents/{id}/log     → last N log entries (?limit=N)
//   POST   /api/agents/{id}/config  → update agent configuration
//   POST   /api/agents/{id}/message → send a message to the agent
//   DELETE /api/agents/{id}         → stop the agent
func (g *Gateway) handleAPIAgentDetail(w http.ResponseWriter, r *http.Request) {
	// Strip the leading "/api/agents/" prefix to get the remainder.
	rest := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	rest = strings.Trim(rest, "/")

	// Split on "/" to separate the agent ID from any sub-resource.
	parts := strings.SplitN(rest, "/", 2)
	id := parts[0]
	subResource := ""
	if len(parts) == 2 {
		subResource = parts[1]
	}

	if id == "" {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "agent id is required"})
		return
	}

	switch subResource {
	case "":
		g.handleAgentDetailRoot(w, r, id)
	case "log":
		g.handleAgentLog(w, r, id)
	case "config":
		g.handleAgentConfig(w, r, id)
	case "message":
		g.handleAgentMessage(w, r, id)
	default:
		writeGatewayJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("unknown sub-resource: %s", subResource),
		})
	}
}

// handleAgentDetailRoot handles GET /api/agents/{id}.
func (g *Gateway) handleAgentDetailRoot(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		if g.config.API == nil || g.config.API.GetAgentDetail == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "agent detail API not available",
			})
			return
		}
		detail, err := g.config.API.GetAgentDetail(id)
		if err != nil {
			writeGatewayJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusOK, detail)

	case http.MethodDelete:
		if g.config.API == nil || g.config.API.StopAgent == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "agent stop not available",
			})
			return
		}
		if err := g.config.API.StopAgent(id); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "stopped", "id": id})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAgentLog handles GET /api/agents/{id}/log[?limit=N].
func (g *Gateway) handleAgentLog(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.GetAgentLog == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "agent log API not available",
		})
		return
	}

	limit := 0 // 0 = return all
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := fmt.Sscanf(raw, "%d", &limit); n != 1 || err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit parameter"})
			return
		}
	}

	entries, err := g.config.API.GetAgentLog(id, limit)
	if err != nil {
		writeGatewayJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"id":      id,
		"entries": entries,
		"count":   len(entries),
	})
}

// handleAgentConfig handles POST /api/agents/{id}/config.
func (g *Gateway) handleAgentConfig(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.SetAgentConfig == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "agent config update not available",
		})
		return
	}

	var cfg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := g.config.API.SetAgentConfig(id, cfg); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": id})
}

// handleAgentMessage handles POST /api/agents/{id}/message.
func (g *Gateway) handleAgentMessage(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.SendAgentMessage == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "agent messaging not available",
		})
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Message == "" {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}
	if err := g.config.API.SendAgentMessage(id, req.Message); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "sent", "id": id})
}

// handleAPIExec handles POST /api/exec.
// Body: {"command": "ls -la"}
// Response: {"output": "...", "exit_code": 0}
func (g *Gateway) handleAPIExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if g.config.API == nil || g.config.API.ExecCommand == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "exec API not available",
		})
		return
	}

	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Command == "" {
		writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "command is required"})
		return
	}

	output, exitCode, err := g.config.API.ExecCommand(req.Command)
	if err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":     err.Error(),
			"output":    output,
			"exit_code": exitCode,
		})
		return
	}

	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"output":    output,
		"exit_code": exitCode,
	})
}

// handleAPIPolicies handles CRUD for scoped policy rules.
//
//	GET    /api/policies               — list all scoped rules
//	POST   /api/policies               — add a new scoped rule (body: JSON ScopedRule)
//	DELETE /api/policies/{name}        — remove a rule by name
func (g *Gateway) handleAPIPolicies(w http.ResponseWriter, r *http.Request) {
	api := g.config.API

	// DELETE /api/policies/{name}
	if r.Method == http.MethodDelete {
		name := strings.TrimPrefix(r.URL.Path, "/api/policies/")
		if name == "" {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "rule name required in path"})
			return
		}
		if api == nil || api.DeleteScopedPolicy == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "scoped policy management not available",
			})
			return
		}
		if err := api.DeleteScopedPolicy(name); err != nil {
			writeGatewayJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// POST /api/policies — add rule
	if r.Method == http.MethodPost {
		if api == nil || api.AddScopedPolicy == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "scoped policy management not available",
			})
			return
		}
		var rule map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if err := api.AddScopedPolicy(rule); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusCreated, map[string]string{"status": "created"})
		return
	}

	// GET /api/policies — list rules
	if r.Method == http.MethodGet || r.Method == "" {
		if api == nil || api.GetScopedPolicies == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"rules": []interface{}{}})
			return
		}
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"rules": api.GetScopedPolicies()})
		return
	}

	http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
}

// handleAPIApprovals serves the approval queue endpoints.
//
//	GET  /api/approvals              — list pending approval requests
//	POST /api/approvals/{id}/approve — approve a request (body: {"decided_by":"..."}  optional)
//	POST /api/approvals/{id}/deny    — deny a request   (body: {"decided_by":"..."}  optional)
func (g *Gateway) handleAPIApprovals(w http.ResponseWriter, r *http.Request) {
	api := g.config.API

	// Determine sub-path after /api/approvals/
	rest := strings.TrimPrefix(r.URL.Path, "/api/approvals")
	rest = strings.TrimPrefix(rest, "/")

	// POST /api/approvals/{id}/approve  or  /api/approvals/{id}/deny
	if r.Method == http.MethodPost && rest != "" {
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{
				"error": "expected path: /api/approvals/{id}/approve or /api/approvals/{id}/deny",
			})
			return
		}
		id := parts[0]
		action := parts[1] // "approve" or "deny"

		// Parse optional decided_by from body.
		var body struct {
			DecidedBy string 
		}
		// Best-effort decode; an empty or absent body is fine.
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DecidedBy == "" {
			body.DecidedBy = "gateway-api"
		}

		switch action {
		case "approve":
			if api == nil || api.ApproveRequest == nil {
				writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
					"error": "approval API not available",
				})
				return
			}
			if err := api.ApproveRequest(id, body.DecidedBy); err != nil {
				writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})

		case "deny":
			if api == nil || api.DenyRequest == nil {
				writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
					"error": "approval API not available",
				})
				return
			}
			if err := api.DenyRequest(id, body.DecidedBy); err != nil {
				writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "denied", "id": id})

		default:
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{
				"error": "unknown action " + action + "; use approve or deny",
			})
		}
		return
	}

	// GET /api/approvals — list pending requests
	if r.Method == http.MethodGet || r.Method == "" {
		if api == nil || api.ListApprovals == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
				"approvals": []interface{}{},
				"count":     0,
			})
			return
		}
		approvals := api.ListApprovals()
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"approvals": approvals,
			"count":     len(approvals),
		})
		return
	}

	http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
}

// handleAPIHeartbeat serves GET /api/heartbeat — returns the heartbeat status.
func (g *Gateway) handleAPIHeartbeat(w http.ResponseWriter, r *http.Request) {
	api := g.config.API

	switch r.Method {
	case http.MethodGet, "":
		if api == nil || api.GetHeartbeatStatus == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
				"enabled": false,
				"message": "heartbeat not available",
			})
			return
		}
		writeGatewayJSON(w, http.StatusOK, api.GetHeartbeatStatus())

	case http.MethodPost:
		// Toggle heartbeat on/off
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if api == nil || api.ToggleHeartbeat == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
				"enabled": false,
				"message": "heartbeat not available",
			})
			return
		}
		newState := api.ToggleHeartbeat(req.Enabled)
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": newState,
			"message": map[bool]string{true: "heartbeat enabled", false: "heartbeat disabled"}[newState],
		})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleAPIHeartbeatRun serves POST /api/heartbeat/run — triggers an immediate
// heartbeat check and returns the AI response.
func (g *Gateway) handleAPIHeartbeatRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	api := g.config.API
	if api == nil || api.RunHeartbeatNow == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "heartbeat not available",
		})
		return
	}

	response, err := api.RunHeartbeatNow()
	if err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"response": response,
	})
}

// handleAPIThreads handles CRUD for web UI chat threads.
//
// GET    /api/threads        — return all stored threads
// POST   /api/threads        — save/update one thread (full JSON object)
// DELETE /api/threads/{id}   — delete a thread by id
func (g *Gateway) handleAPIThreads(w http.ResponseWriter, r *http.Request) {
	api := g.config.API

	switch r.Method {
	case http.MethodGet:
		if api == nil || api.GetThreads == nil {
			writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
				"threads":   []interface{}{},
				"available": false,
			})
			return
		}
		threads, err := api.GetThreads()
		if err != nil {
			writeGatewayJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if threads == nil {
			threads = []map[string]interface{}{}
		}
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"threads": threads,
			"count":   len(threads),
		})

	case http.MethodPost:
		if api == nil || api.SaveThread == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "thread storage not available",
			})
			return
		}
		var thread map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&thread); err != nil {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if _, ok := thread["id"]; !ok {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "thread.id is required"})
			return
		}
		if err := api.SaveThread(thread); err != nil {
			writeGatewayJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	case http.MethodDelete:
		if api == nil || api.DeleteThread == nil {
			writeGatewayJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error": "thread storage not available",
			})
			return
		}
		// Extract id from path: /api/threads/{id}
		id := strings.TrimPrefix(r.URL.Path, "/api/threads/")
		id = strings.TrimSuffix(id, "/")
		if id == "" {
			writeGatewayJSON(w, http.StatusBadRequest, map[string]string{"error": "thread id required"})
			return
		}
		if err := api.DeleteThread(id); err != nil {
			writeGatewayJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
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
