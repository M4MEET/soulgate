package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers/approval"
	"github.com/M4MEET/soulgate/internal/brokers/exec"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	"github.com/M4MEET/soulgate/internal/brokers/net"
	"github.com/M4MEET/soulgate/internal/brokers/secrets"
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
	scopedPolicyEngine  *policy.ScopedEngine
	fileBroker          *files.Broker
	execBroker          *exec.Broker
	netBroker           *net.Broker
	secretBroker        *secrets.SecretBroker // encrypted secret store
	approvalBroker      *approval.Broker // async approval queue for require_approval decisions
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
	heartbeat           *Heartbeat                // Optional periodic health-check
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
	providerMu          sync.RWMutex              // Protects provider, actualModelName, and config model fields
	conversationHistory []model.Message           // Persistent conversation history across runs
	historyMu           sync.RWMutex              // Protects conversationHistory
	costTracker         *CostTracker              // Tracks API cost per session/day/provider
	fallbackChain       *FallbackChain            // Optional ordered list of backup providers
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

	// Initialize scoped policy engine (hierarchical: global → team → user → agent).
	// The scoped policy file lives next to the base policy file.
	scopedPolicyEngine, err := policy.NewScopedEngine(workspace.Config.Policy.ScopedFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize scoped policy engine: %w", err)
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

	// Initialize approval broker — persists pending requests in the config dir
	// so the gateway web UI can surface them even after a restart.
	approvalBroker := approval.NewBroker(workspace.ConfigDir)

	// Initialize secret broker — stores credentials encrypted at rest.
	// Non-fatal: failure prints a warning but does not prevent startup.
	var secretBroker *secrets.SecretBroker
	if sb, sbErr := secrets.NewBroker(workspace.ConfigDir, auditLogger); sbErr != nil {
		fmt.Fprintf(os.Stderr, "warning: secret broker: %v\n", sbErr)
	} else {
		secretBroker = sb
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
	pluginMgr := plugins.NewManager(pluginDir, workspace.Config.Plugins.Timeout, workspace.Config.Plugins.MaxMemory)
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
		policyEngine:       policyEngine,
		scopedPolicyEngine: scopedPolicyEngine,
		fileBroker:        fileBroker,
		execBroker:        execBroker,
		netBroker:         netBroker,
		approvalBroker:    approvalBroker,
		secretBroker:      secretBroker,
		integrationsReg:   integrationsReg,
		integrationsStore: integrationsStore,
		memoryStore:       memoryStore,
		agentManager:      NewAgentManager(workspace.ConfigDir),
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
		fallbackChain:     NewFallbackChain(workspace.Config.Model.FallbackChain),
	}
	if orch.policyEngine != nil {
		orch.policyEngine.SetBypassChecker(orch.IsTrustMode)
	}

	// Replace the placeholder watch callback with one that closes over orch.
	// This must happen after orch is built so the thinkingCallback field is
	// accessible.
	// Configure symlink boundary enforcement for the file watcher.
	watchMgr.SetWorkspaceRoot(workspace.Root)
	watchMgr.ReplaceCallback(orch.makeWatchCallback())

	// Initialize heartbeat if configured.
	hbCfg := workspace.Config.Heartbeat
	if hbCfg.Interval <= 0 {
		hbCfg.Interval = 30 * time.Minute
	}
	if hbCfg.PromptFile == "" {
		hbCfg.PromptFile = ".soulgate/HEARTBEAT.md"
	}
	orch.heartbeat = NewHeartbeat(orch, hbCfg)
	if hbCfg.Enabled {
		orch.heartbeat.Start()
	}

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

// GetPolicyEngine returns the policy engine
func (o *Orchestrator) GetPolicyEngine() *policy.Engine {
	return o.policyEngine
}

// GetScopedPolicyEngine returns the hierarchical scoped policy engine.
// The scoped engine layers on top of the base engine:
// global → team → user → agent rules, with the most-specific scope winning.
func (o *Orchestrator) GetScopedPolicyEngine() *policy.ScopedEngine {
	return o.scopedPolicyEngine
}

// GetApprovalBroker returns the approval broker so the gateway can wire its
// HTTP approval endpoints directly to the running broker instance.
func (o *Orchestrator) GetApprovalBroker() *approval.Broker {
	return o.approvalBroker
}

// GetAllToolSchemas returns the full catalog of all available tool schemas.
// This initialises the tool registry if it has not been populated yet.
func (o *Orchestrator) GetAllToolSchemas() []model.ToolSchema {
	o.initToolRegistry()
	o.toolRegistry.mu.RLock()
	defer o.toolRegistry.mu.RUnlock()
	out := make([]model.ToolSchema, len(o.toolRegistry.allTools))
	copy(out, o.toolRegistry.allTools)
	return out
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

// GetFileBroker returns the file broker for direct use by trusted callers such
// as the gateway's /api/files and /api/file endpoints. The broker enforces
// policy and audit logging on every operation.
func (o *Orchestrator) GetFileBroker() *files.Broker {
	return o.fileBroker
}

// GetExecBroker returns the exec broker for direct use by trusted callers such
// as the gateway's /api/exec endpoint. The broker enforces policy and audit
// logging on every operation.
func (o *Orchestrator) GetExecBroker() *exec.Broker {
	return o.execBroker
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

// contextMaxMessages returns the configured (or default) max history messages.
func (o *Orchestrator) contextMaxMessages() int {
	if n := o.workspace.Config.Context.MaxHistoryMessages; n > 0 {
		return n
	}
	return 50
}

// contextMaxChars returns the configured (or default) max history chars.
func (o *Orchestrator) contextMaxChars() int {
	if n := o.workspace.Config.Context.MaxHistoryChars; n > 0 {
		return n
	}
	return 100000
}

// contextCompactionEnabled returns whether LLM compaction is turned on.
func (o *Orchestrator) contextCompactionEnabled() bool {
	return o.workspace.Config.Context.CompactionEnabled
}

// contextCompactionThreshold returns the fraction (0–1) of max chars that
// triggers compaction.
func (o *Orchestrator) contextCompactionThreshold() float64 {
	t := o.workspace.Config.Context.CompactionThreshold
	if t <= 0 || t > 1 {
		return 0.8
	}
	return t
}

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

	maxMessages := o.contextMaxMessages()
	maxChars := o.contextMaxChars()

	// Cap history by message count
	if len(o.conversationHistory) > maxMessages {
		o.conversationHistory = o.conversationHistory[len(o.conversationHistory)-maxMessages:]
	}

	// Cap history by total character count (trim oldest messages first)
	for {
		total := 0
		for _, m := range o.conversationHistory {
			total += len(m.Content)
		}
		if total <= maxChars || len(o.conversationHistory) <= 2 {
			break
		}
		o.conversationHistory = o.conversationHistory[1:]
	}
}

// summarizeToolInteraction builds a concise breadcrumb string from an
// assistant message containing tool calls and the resulting tool responses.
// Example: "[Used exec_command(\"ls\"): 15 lines output]"
func summarizeToolInteraction(assistantMsg model.Message, toolResults []model.Message) string {
	var sb strings.Builder
	for i, tc := range assistantMsg.ToolCalls {
		// Parse first argument value for a compact description
		argSnippet := ""
		if len(tc.Input) > 0 {
			var args map[string]interface{}
			if err := json.Unmarshal(tc.Input, &args); err == nil {
				// Use the first string-valued arg as the snippet
				for _, v := range args {
					if s, ok := v.(string); ok {
						if len(s) > 60 {
							s = s[:60] + "..."
						}
						argSnippet = fmt.Sprintf("%q", s)
						break
					}
				}
			}
		}

		// Build the call description
		if argSnippet != "" {
			sb.WriteString(fmt.Sprintf("[Used %s(%s)", tc.Name, argSnippet))
		} else {
			sb.WriteString(fmt.Sprintf("[Used %s()", tc.Name))
		}

		// Match the result by ToolCallID
		for _, r := range toolResults {
			if r.ToolCallID == tc.ID {
				resultLen := len(r.Content)
				if strings.HasPrefix(r.Content, "Error:") || strings.HasPrefix(r.Content, "error:") {
					errMsg := r.Content
					if len(errMsg) > 80 {
						errMsg = errMsg[:80] + "..."
					}
					sb.WriteString(fmt.Sprintf(": %s", errMsg))
				} else if resultLen > 200 {
					sb.WriteString(fmt.Sprintf(": %d chars output", resultLen))
				} else if resultLen > 0 {
					// Short results — include inline
					snippet := r.Content
					if len(snippet) > 100 {
						snippet = snippet[:100] + "..."
					}
					sb.WriteString(fmt.Sprintf(": %s", snippet))
				} else {
					sb.WriteString(": ok")
				}
				break
			}
		}
		sb.WriteString("]")

		if i < len(assistantMsg.ToolCalls)-1 {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

// appendToolSummaryToHistory generates a compact breadcrumb for the tool
// interaction and appends it as a single synthetic assistant message to the
// conversation history. If the assistant message also had text content, that
// text is prepended before the breadcrumb.
func (o *Orchestrator) appendToolSummaryToHistory(assistantMsg model.Message, toolResults []model.Message) {
	summary := summarizeToolInteraction(assistantMsg, toolResults)
	if summary == "" {
		return
	}

	content := summary
	if assistantMsg.Content != "" {
		content = assistantMsg.Content + "\n" + summary
	}

	o.appendToHistory(model.Message{
		Role:    model.RoleAssistant,
		Content: content,
	})
}

// historyCharCount returns the total character count of the conversation history.
// Caller must hold at least historyMu.RLock.
func (o *Orchestrator) historyCharCount() int {
	total := 0
	for _, m := range o.conversationHistory {
		total += len(m.Content)
	}
	return total
}

// compactHistory uses an LLM call to summarise older conversation history,
// keeping recent messages intact. It replaces the older portion with a single
// "[Conversation summary: ...]" message.
func (o *Orchestrator) compactHistory(ctx context.Context) error {
	o.historyMu.Lock()
	histLen := len(o.conversationHistory)

	// Need at least 12 messages for compaction to be meaningful
	// (keep last 10, summarise at least 2).
	if histLen < 12 {
		o.historyMu.Unlock()
		return nil
	}

	// Split: keep last 10 messages as "recent", summarise the rest.
	splitIdx := histLen - 10
	toSummarise := make([]model.Message, splitIdx)
	copy(toSummarise, o.conversationHistory[:splitIdx])
	recent := make([]model.Message, 10)
	copy(recent, o.conversationHistory[splitIdx:])
	o.historyMu.Unlock()

	// Build summarisation prompt
	var sb strings.Builder
	sb.WriteString("Summarise the following conversation history into a concise paragraph. ")
	sb.WriteString("Preserve key facts, decisions, file paths, and tool outcomes. ")
	sb.WriteString("Do NOT include greetings or filler. Output ONLY the summary.\n\n")
	for _, m := range toSummarise {
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, m.Content))
	}

	executor := &llmTaskExecutor{orch: o}
	summaryText, err := executor.Complete(ctx, sb.String(), false)
	if err != nil {
		return fmt.Errorf("compaction LLM call failed: %w", err)
	}

	// Replace history with summary + recent messages
	summaryMsg := model.Message{
		Role:    model.RoleAssistant,
		Content: fmt.Sprintf("[Conversation summary: %s]", strings.TrimSpace(summaryText)),
	}

	o.historyMu.Lock()
	newHistory := make([]model.Message, 0, 1+len(recent))
	newHistory = append(newHistory, summaryMsg)
	newHistory = append(newHistory, recent...)
	o.conversationHistory = newHistory
	o.historyMu.Unlock()

	return nil
}

// maybeCompact triggers history compaction if the history exceeds the
// configured threshold. Returns true if compaction was performed.
func (o *Orchestrator) maybeCompact(ctx context.Context) bool {
	if !o.contextCompactionEnabled() {
		return false
	}

	o.historyMu.RLock()
	chars := o.historyCharCount()
	o.historyMu.RUnlock()

	threshold := int(float64(o.contextMaxChars()) * o.contextCompactionThreshold())
	if chars < threshold {
		return false
	}

	if err := o.compactHistory(ctx); err != nil {
		// Non-fatal: log and continue
		fmt.Fprintf(os.Stderr, "warning: history compaction failed: %v\n", err)
		return false
	}

	return true
}

// GetHeartbeat returns the heartbeat subsystem. The returned pointer is never
// nil — a Heartbeat is always constructed during NewOrchestrator, but it is
// only started automatically when HeartbeatConfig.Enabled is true.
func (o *Orchestrator) GetHeartbeat() *Heartbeat {
	return o.heartbeat
}

// runHeartbeat executes a single heartbeat prompt through the agentic loop
// without modifying the user-visible conversation history. This keeps
// heartbeat checks isolated from the ongoing chat context.
func (o *Orchestrator) runHeartbeat(ctx context.Context, prompt string) (string, error) {
	run := o.session.CreateRun(prompt)
	run.Start()

	response, err := o.executeAgenticLoop(ctx, prompt, run.ID)
	if err != nil {
		run.SetResult(fmt.Sprintf("Error: %v", err))
		return "", err
	}

	run.SetResult(response)
	return response, nil
}

// Close cleans up resources
func (o *Orchestrator) Close() error {
	// Stop heartbeat
	if o.heartbeat != nil {
		o.heartbeat.Stop()
	}

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

	// Close secret broker — zeroes in-memory plaintext values.
	if o.secretBroker != nil {
		_ = o.secretBroker.Close()
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
