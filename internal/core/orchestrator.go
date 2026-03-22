package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers/exec"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	"github.com/M4MEET/soulgate/internal/brokers/net"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/mcp"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
	"github.com/M4MEET/soulgate/internal/plugins"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/M4MEET/soulgate/internal/tools/browser"
	"github.com/M4MEET/soulgate/internal/tools/canvas"
	"github.com/M4MEET/soulgate/internal/tools/cron"
	"github.com/M4MEET/soulgate/internal/tools/embeddings"
	"github.com/M4MEET/soulgate/internal/tools/filewatcher"
	"github.com/M4MEET/soulgate/internal/tools/process"
)

// Orchestrator coordinates model calls, plugin execution, and broker access
type Orchestrator struct {
	workspace           *config.Workspace
	audit               audit.Logger
	session             *Session
	provider            model.Provider
	policyEngine        *policy.Engine
	fileBroker          *files.Broker
	execBroker          *exec.Broker
	netBroker           *net.Broker
	integrationsReg     *integrations.Registry
	integrationsStore   *integrations.Store
	memoryStore         *MemoryStore
	agentManager        *AgentManager
	processManager      *process.Manager
	cronScheduler       *cron.Scheduler
	watchManager        *filewatcher.Manager
	browserManager      *browser.Manager
	vectorStore         *embeddings.VectorStore
	mcpManager          *mcp.Manager
	pluginManager       *plugins.Manager
	canvasManager       *canvas.Manager
	canvasPreviewMgr    *canvas.PreviewManager
	toolRegistry        *ToolRegistry
	directives          *Directives
	loopDetector        *LoopDetector
	branchManager       *BranchManager
	streaming           bool
	streamCallback      func(chunk string)        // Called for each streamed token
	thinkingCallback    func(event ThinkingEvent) // Called for live thinking output
	permissionCallback  PermissionCallback        // Optional: called when permission is needed
	actualModelName     string                    // Actual model name from last API response
	trustMode           bool                      // When true, bypass all permission checks
	trustModeExpiry     *time.Time                // Auto-disable trust mode after this time
	trustMu             sync.RWMutex              // Protects trust mode state
	conversationHistory []model.Message           // Persistent conversation history across runs
	historyMu           sync.RWMutex              // Protects conversationHistory
	costTracker         *CostTracker              // Tracks API cost per session/day/provider
}

// RunResult represents the result of a run
type RunResult struct {
	RunID    string
	Response string
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(workspace *config.Workspace) (*Orchestrator, error) {
	// Initialize audit logger
	auditLogger, err := audit.NewJSONLLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	// Initialize policy engine
	policyEngine, err := initializePolicyEngine(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize policy engine: %w", err)
	}

	// Initialize file broker
	fileBroker, err := files.NewBroker(workspace.Root, policyEngine, auditLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create file broker: %w", err)
	}

	// Initialize exec broker
	execBroker, err := exec.NewBroker(workspace.Root, policyEngine, auditLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec broker: %w", err)
	}

	// Initialize network broker
	netBroker, err := net.NewBroker(policyEngine, auditLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create network broker: %w", err)
	}

	// Initialize integrations
	integrationsStore, err := integrations.NewStore(workspace.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create integrations store: %w", err)
	}

	integrationsReg := integrations.NewRegistry()

	// Register available integrations
	if err := registerDefaultIntegrations(integrationsReg); err != nil {
		return nil, fmt.Errorf("failed to register integrations: %w", err)
	}

	// Load and configure integrations from store
	for _, name := range integrationsStore.List() {
		config, _ := integrationsStore.Get(name)
		integration, err := integrationsReg.Get(name)
		if err == nil {
			integration.Setup(context.Background(), config)
		}
	}

	// Initialize memory store
	memoryStore, err := NewMemoryStore(workspace.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory store: %w", err)
	}

	// Initialize vector store for semantic memory (best-effort — needs embedding API key)
	var vectorStore *embeddings.VectorStore
	embeddingKey := os.Getenv("OPENAI_API_KEY")
	if embeddingKey == "" {
		embeddingKey = workspace.Config.Model.OpenAI.APIKey
	}
	if embeddingKey != "" {
		embProvider := embeddings.NewOpenAIProvider(embeddingKey, "", "")
		vs, err := embeddings.NewVectorStore(filepath.Join(workspace.ConfigDir, "vectors"), embProvider)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: vector store: %v\n", err)
		} else {
			vectorStore = vs
		}
	}

	// Initialize model provider
	provider, err := initializeProvider(workspace.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model provider: %w", err)
	}

	// Create session
	session := NewSession(workspace.Root)

	// Initialize cost tracker (append-only, loads historical data on startup)
	costTracker := NewCostTracker(workspace.ConfigDir, session.ID)

	// Log session start
	event := audit.NewEvent(audit.EventSessionStart, audit.CategorySession).
		WithSessionID(session.ID).
		WithMetadata("workspace", workspace.Root).
		WithMetadata("provider", provider.Name())

	if err := auditLogger.Log(context.Background(), event); err != nil {
		return nil, fmt.Errorf("failed to log session start: %w", err)
	}

	// Initialize MCP manager
	mcpMgr := mcp.NewManager()
	for _, srv := range workspace.Config.MCP.Servers {
		enabled := srv.Enabled == nil || *srv.Enabled
		if !enabled {
			continue
		}
		env := make([]string, 0, len(srv.Env))
		for k, v := range srv.Env {
			env = append(env, k+"="+v)
		}
		_ = mcpMgr.AddServer(mcp.ServerConfig{
			Name:    srv.Name,
			Command: srv.Command,
			Args:    srv.Args,
			Env:     env,
			WorkDir: srv.WorkDir,
			Enabled: true,
		})
	}
	// Start MCP servers in background (non-blocking, warnings printed to stderr)
	if len(workspace.Config.MCP.Servers) > 0 {
		startCtx, startCancel := context.WithTimeout(context.Background(), 30*time.Second)
		mcpMgr.StartAll(startCtx)
		startCancel()
	}

	// Initialize plugin manager
	pluginDir := filepath.Join(workspace.Root, workspace.Config.Plugins.Dir)
	pluginMgr := plugins.NewManager(pluginDir, workspace.Config.Plugins.Timeout)
	if err := pluginMgr.LoadAll(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: plugin loading: %v\n", err)
	}

	// Initialize canvas manager (best-effort — failure is non-fatal)
	canvasDir := filepath.Join(workspace.ConfigDir, "canvas")
	canvasMgr, canvasErr := canvas.NewManager(canvasDir)
	if canvasErr != nil {
		fmt.Fprintf(os.Stderr, "warning: canvas manager: %v\n", canvasErr)
	}

	// Initialize file watcher manager.  The callback placeholder below is
	// replaced after construction so that it can close over the Orchestrator.
	// We use a sentinel no-op here to satisfy the non-nil requirement; the
	// real callback is injected immediately after orch is built.
	watchMgr := filewatcher.NewManager(func(watchID string, event filewatcher.EventType, path string) {
		// Replaced post-construction via replaceWatchCallback.
	})

	orch := &Orchestrator{
		workspace:         workspace,
		audit:             auditLogger,
		session:           session,
		provider:          provider,
		policyEngine:      policyEngine,
		fileBroker:        fileBroker,
		execBroker:        execBroker,
		netBroker:         netBroker,
		integrationsReg:   integrationsReg,
		integrationsStore: integrationsStore,
		memoryStore:       memoryStore,
		agentManager:      NewAgentManager(),
		processManager:    process.NewManagerWithWorkspace(workspace.Root),
		cronScheduler:     cron.NewScheduler(workspace.ConfigDir),
		watchManager:      watchMgr,
		browserManager:    browser.NewManager(),
		vectorStore:       vectorStore,
		mcpManager:        mcpMgr,
		pluginManager:     pluginMgr,
		canvasManager:     canvasMgr,
		canvasPreviewMgr:  canvas.NewPreviewManager(),
		toolRegistry:      NewToolRegistry(),
		directives:        DefaultDirectives(),
		loopDetector:      NewLoopDetector(),
		branchManager:     NewBranchManager(workspace.ConfigDir),
		costTracker:       costTracker,
	}
	if orch.policyEngine != nil {
		orch.policyEngine.SetBypassChecker(orch.IsTrustMode)
	}

	// Replace the placeholder watch callback with one that closes over orch.
	// This must happen after orch is built so the thinkingCallback field is
	// accessible.
	watchMgr.ReplaceCallback(orch.makeWatchCallback())

	return orch, nil
}

// makeWatchCallback returns a filewatcher.Callback that surfaces file-change
// events as thinking stream entries.  If no thinkingCallback is set the event
// is written to stderr as a low-priority notice.
func (o *Orchestrator) makeWatchCallback() filewatcher.Callback {
	return func(watchID string, event filewatcher.EventType, path string) {
		if o.thinkingCallback != nil {
			o.thinkingCallback(ThinkingEvent{
				Kind:    ThinkingStatus,
				Message: fmt.Sprintf("[filewatcher] %s: %s %s", watchID, event, path),
			})
		} else {
			fmt.Fprintf(os.Stderr, "[filewatcher] %s: %s %s\n", watchID, event, path)
		}
	}
}

// initializePolicyEngine initializes the policy engine from workspace config
func initializePolicyEngine(workspace *config.Workspace) (*policy.Engine, error) {
	policyPath := workspace.Config.Policy.FilePath

	// Check if policy file exists
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		// No policy file - use default deny
		return policy.NewEngine(nil), nil
	}

	// Load policy
	pol, err := policy.LoadPolicy(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	return policy.NewEngine(pol), nil
}

// initializeProvider initializes the model provider based on config
func initializeProvider(cfg *config.Config) (model.Provider, error) {
	providerName := cfg.Model.DefaultProvider

	def, err := model.LookupProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("unsupported model provider: %s\n\nSupported providers: %s\n\nRun 'soulgate chat' to configure a provider",
			providerName, formatProviderList())
	}

	// Get API key — prefer config, fall back to env
	var apiKey string
	switch providerName {
	case "anthropic":
		apiKey = cfg.Model.Anthropic.APIKey
	default:
		apiKey = cfg.Model.OpenAI.APIKey
	}
	if apiKey == "" {
		apiKey, err = model.ResolveAPIKey(def)
		if err != nil {
			return nil, fmt.Errorf("%s API key not configured (set %s environment variable)", providerName, def.EnvKey)
		}
	}

	// Get model name from config
	var modelName string
	switch providerName {
	case "anthropic":
		modelName = cfg.Model.Anthropic.Model
	default:
		modelName = cfg.Model.OpenAI.Model
	}
	if modelName == "" {
		modelName = def.DefaultModel
	}

	// Get base URL — prefer config, fall back to registry
	var baseURL string
	switch providerName {
	case "anthropic":
		baseURL = cfg.Model.Anthropic.BaseURL
	default:
		baseURL = cfg.Model.OpenAI.BaseURL
	}
	baseURL = model.ResolveBaseURL(def, baseURL)

	// Create provider based on protocol
	switch def.Protocol {
	case "anthropic":
		return anthropic.NewProvider(apiKey, modelName, baseURL), nil
	default:
		return openai.NewProvider(apiKey, modelName, baseURL), nil
	}
}

// Run executes a run with the given prompt
func (o *Orchestrator) Run(ctx context.Context, prompt string) (*RunResult, error) {
	// Create run
	run := o.session.CreateRun(prompt)
	run.Start()

	// Log run start
	event := audit.NewEvent(audit.EventRunStart, audit.CategoryRun).
		WithSessionID(o.session.ID).
		WithRunID(run.ID).
		WithMetadata("prompt", prompt)

	if err := o.audit.Log(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to log run start: %w", err)
	}

	// Execute agentic loop
	response, err := o.executeAgenticLoop(ctx, prompt, run.ID)
	if err != nil {
		run.SetResult(fmt.Sprintf("Error: %v", err))

		// Log run error with fresh context (in case parent context timed out)
		auditCtx, cancel := auditContext()
		defer cancel()

		errorEvent := audit.NewEvent(audit.EventRunComplete, audit.CategoryRun).
			WithSessionID(o.session.ID).
			WithRunID(run.ID).
			WithMetadata("error", err.Error()).
			WithStatus(audit.StatusError)

		if logErr := o.audit.Log(auditCtx, errorEvent); logErr != nil {
			// Don't fail the whole operation if audit logging fails
			// Just log to stderr for debugging
			fmt.Fprintf(os.Stderr, "Warning: failed to log run error to audit: %v\n", logErr)
		}

		return nil, err
	}

	result := &RunResult{
		RunID:    run.ID,
		Response: response,
	}

	run.SetResult(result.Response)

	// Log run complete with fresh context
	auditCtx, cancel := auditContext()
	defer cancel()

	completeEvent := audit.NewEvent(audit.EventRunComplete, audit.CategoryRun).
		WithSessionID(o.session.ID).
		WithRunID(run.ID).
		WithMetadata("result", result.Response)

	if err := o.audit.Log(auditCtx, completeEvent); err != nil {
		// Don't fail the operation if audit logging fails
		fmt.Fprintf(os.Stderr, "Warning: failed to log run complete to audit: %v\n", err)
	}

	return result, nil
}

// GetSession returns the current session
func (o *Orchestrator) GetSession() *Session {
	return o.session
}

// GetAuditLogger returns the audit logger
func (o *Orchestrator) GetAuditLogger() audit.Logger {
	return o.audit
}

// GetWorkspace returns the current workspace
func (o *Orchestrator) GetWorkspace() *config.Workspace {
	return o.workspace
}

// GetMemoryStore returns the memory store
func (o *Orchestrator) GetMemoryStore() *MemoryStore {
	return o.memoryStore
}

// GetCostTracker returns the cost tracker
func (o *Orchestrator) GetCostTracker() *CostTracker {
	return o.costTracker
}

// SetStreaming enables or disables streaming mode
func (o *Orchestrator) SetStreaming(enabled bool, callback func(chunk string)) {
	o.streaming = enabled
	o.streamCallback = callback
}

// IsStreaming returns whether streaming mode is enabled
func (o *Orchestrator) IsStreaming() bool {
	return o.streaming
}

// GetAgentManager returns the agent manager
func (o *Orchestrator) GetAgentManager() *AgentManager {
	return o.agentManager
}

// GetProcessManager returns the process manager
func (o *Orchestrator) GetProcessManager() *process.Manager {
	return o.processManager
}

// GetCronScheduler returns the cron scheduler
func (o *Orchestrator) GetCronScheduler() *cron.Scheduler {
	return o.cronScheduler
}

// GetWatchManager returns the file watcher manager
func (o *Orchestrator) GetWatchManager() *filewatcher.Manager {
	return o.watchManager
}

// GetDirectives returns the current session directives
func (o *Orchestrator) GetDirectives() *Directives {
	return o.directives
}

// GetLoopDetector returns the loop detector
func (o *Orchestrator) GetLoopDetector() *LoopDetector {
	return o.loopDetector
}

// GetMCPManager returns the MCP server manager
func (o *Orchestrator) GetMCPManager() *mcp.Manager {
	return o.mcpManager
}

// GetBranchManager returns the conversation branch manager.
func (o *Orchestrator) GetBranchManager() *BranchManager {
	return o.branchManager
}

// ForkConversation saves the current conversation history into the active branch,
// then creates a new branch from forkPoint in that history, and switches to it.
// The orchestrator's conversationHistory is replaced with the forked slice.
// Returns the new branch ID.
func (o *Orchestrator) ForkConversation(label string, forkPoint int) (string, error) {
	// Snapshot current history into the active branch before forking.
	current := o.GetConversationHistory()
	if err := o.branchManager.SyncMessages(current); err != nil {
		return "", fmt.Errorf("failed to sync current branch: %w", err)
	}

	newID, err := o.branchManager.Fork(label, forkPoint)
	if err != nil {
		return "", fmt.Errorf("failed to fork branch: %w", err)
	}

	// Switch active branch and restore its (truncated) history.
	if err := o.branchManager.Switch(newID); err != nil {
		return "", fmt.Errorf("failed to switch to new branch: %w", err)
	}

	forked := o.branchManager.GetCurrentMessages()
	o.SetConversationHistory(forked)

	return newID, nil
}

// SwitchBranch saves the current conversation history, then switches to
// branchID and restores its stored messages into the orchestrator's history.
func (o *Orchestrator) SwitchBranch(branchID string) error {
	// Persist current history to the active branch before leaving.
	current := o.GetConversationHistory()
	if err := o.branchManager.SyncMessages(current); err != nil {
		return fmt.Errorf("failed to sync current branch: %w", err)
	}

	if err := o.branchManager.Switch(branchID); err != nil {
		return fmt.Errorf("failed to switch branch: %w", err)
	}

	restored := o.branchManager.GetCurrentMessages()
	o.SetConversationHistory(restored)
	return nil
}

// MergeBranch merges the unique messages from branchID into the active branch's
// history and updates the orchestrator's conversationHistory accordingly.
func (o *Orchestrator) MergeBranch(branchID string) error {
	// Sync current history before merging.
	current := o.GetConversationHistory()
	if err := o.branchManager.SyncMessages(current); err != nil {
		return fmt.Errorf("failed to sync current branch: %w", err)
	}

	if err := o.branchManager.Merge(branchID); err != nil {
		return fmt.Errorf("failed to merge branch: %w", err)
	}

	merged := o.branchManager.GetCurrentMessages()
	o.SetConversationHistory(merged)
	return nil
}

// GetConversationHistory returns the accumulated conversation messages.
// Only user and assistant text messages are returned (no tool calls/results),
// as those are ephemeral to individual runs and cannot be replayed across sessions.
func (o *Orchestrator) GetConversationHistory() []model.Message {
	o.historyMu.RLock()
	defer o.historyMu.RUnlock()
	cp := make([]model.Message, len(o.conversationHistory))
	copy(cp, o.conversationHistory)
	return cp
}

// SetConversationHistory replaces the conversation history (used for session restore).
func (o *Orchestrator) SetConversationHistory(messages []model.Message) {
	o.historyMu.Lock()
	defer o.historyMu.Unlock()
	o.conversationHistory = messages
}

// maxHistoryMessages is the maximum number of messages to keep in conversation history.
// Each user+assistant pair counts as 2, so this allows ~10 turns of conversation.
const maxHistoryMessages = 20

// maxHistoryChars is the approximate character budget for conversation history.
// ~4 chars per token, so 20k chars ≈ 5k tokens of context.
const maxHistoryChars = 20000

// appendToHistory adds messages to the persistent conversation history.
// Only user and assistant text messages are kept. Tool calls, tool results,
// and system messages are excluded because they contain ephemeral IDs that
// cannot be replayed in future API calls.
func (o *Orchestrator) appendToHistory(msgs ...model.Message) {
	o.historyMu.Lock()
	defer o.historyMu.Unlock()
	for _, msg := range msgs {
		switch msg.Role {
		case model.RoleUser:
			o.conversationHistory = append(o.conversationHistory, model.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		case model.RoleAssistant:
			// Only keep text content, strip tool calls
			if msg.Content != "" {
				o.conversationHistory = append(o.conversationHistory, model.Message{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
			// Skip tool results and system messages
		}
	}

	// Cap history by message count
	if len(o.conversationHistory) > maxHistoryMessages {
		o.conversationHistory = o.conversationHistory[len(o.conversationHistory)-maxHistoryMessages:]
	}

	// Cap history by total character count (trim oldest messages first)
	for {
		total := 0
		for _, m := range o.conversationHistory {
			total += len(m.Content)
		}
		if total <= maxHistoryChars || len(o.conversationHistory) <= 2 {
			break
		}
		o.conversationHistory = o.conversationHistory[1:]
	}
}

// Close cleans up resources
func (o *Orchestrator) Close() error {
	// Stop MCP servers
	if o.mcpManager != nil {
		_ = o.mcpManager.StopAll()
	}

	// Stop cron scheduler
	o.cronScheduler.Stop()

	// Stop all active file watchers
	if o.watchManager != nil {
		o.watchManager.StopAll()
	}

	// Close browser manager (no-op if Chrome was never started)
	o.browserManager.Close()

	// Stop any active canvas preview servers
	if o.canvasPreviewMgr != nil {
		o.canvasPreviewMgr.StopAll()
	}

	// Log session end
	event := audit.NewEvent(audit.EventSessionEnd, audit.CategorySession).
		WithSessionID(o.session.ID)

	if err := o.audit.Log(context.Background(), event); err != nil {
		return fmt.Errorf("failed to log session end: %w", err)
	}

	// Close audit logger
	if err := o.audit.Close(); err != nil {
		return fmt.Errorf("failed to close audit logger: %w", err)
	}

	return nil
}

// auditContext creates a fresh context for audit logging with a reasonable timeout.
// This ensures audit logging doesn't fail even if the parent context is cancelled.
func auditContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
