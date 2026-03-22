package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/model"
)

// agentContextKey is the context key for the currently-executing agent ID.
type agentContextKey struct{}

// withAgentID stores the agent ID in the context so that tool dispatch can
// determine which agent is making the call (used by agent_delegate and
// agent_message).
func withAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentContextKey{}, agentID)
}

// agentIDFromContext retrieves the agent ID from context. Returns empty string
// when the call originates from the top-level orchestrator loop.
func agentIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(agentContextKey{}).(string); ok {
		return v
	}
	return ""
}

// AgentStatus represents the current state of a background agent
type AgentStatus string

const (
	AgentRunning   AgentStatus = "running"
	AgentCompleted AgentStatus = "completed"
	AgentFailed    AgentStatus = "failed"
	AgentStopped   AgentStatus = "stopped"
)

// AgentRole represents the specialization of a background agent
type AgentRole string

const (
	AgentRoleGeneral  AgentRole = "general"
	AgentRoleCoder    AgentRole = "coder"
	AgentRoleResearch AgentRole = "research"
	AgentRoleOps      AgentRole = "ops"
)

// agentRoleDescriptions maps each role to a short directive for the system prompt.
var agentRoleDescriptions = map[AgentRole]string{
	AgentRoleGeneral:  "You are a general-purpose agent. Complete any task assigned to you.",
	AgentRoleCoder:    "You are a specialist coding agent. Focus on writing, reviewing, and debugging code. Prefer precise, idiomatic code with clear explanations.",
	AgentRoleResearch: "You are a specialist research agent. Focus on gathering, summarising, and synthesising information. Cite sources and provide concise findings.",
	AgentRoleOps:      "You are a specialist operations agent. Focus on system administration, infrastructure, automation, and monitoring tasks.",
}

// agentRoleCapabilities maps each role to the tool names it may use.
// An empty slice means the role inherits the full filtered tool set.
var agentRoleCapabilities = map[AgentRole][]string{
	AgentRoleGeneral:  {}, // unrestricted
	AgentRoleCoder:    {"files_read", "files_write", "files_list", "files_delete", "exec_command", "apply_patch", "process_start", "process_list", "process_poll", "process_log", "process_kill"},
	AgentRoleResearch: {"web_search", "web_fetch", "files_read", "files_write", "files_list", "net_request", "memory_write", "memory_get", "memory_search"},
	AgentRoleOps:      {"exec_command", "files_read", "files_write", "files_list", "files_delete", "net_request", "process_start", "process_list", "process_poll", "process_log", "process_write", "process_kill"},
}

// AgentMessage is a message queued from one agent to another.
type AgentMessage struct {
	FromID    string    `json:"from_id"`
	FromName  string    `json:"from_name"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// AgentLogEntry represents a single activity log line from a background agent.
type AgentLogEntry struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"` // "iteration", "model_call", "model_done", "tool_start", "tool_done", "text"
	Message string    `json:"message"`
}

// AgentConfig holds runtime configuration overrides for a background agent.
// Zero values mean "use the orchestrator default".
type AgentConfig struct {
	Model          string   `json:"model"`           // model override (empty = use default)
	Provider       string   `json:"provider"`        // provider override
	AllowedTools   []string `json:"allowed_tools"`   // tool allowlist (empty = all)
	MaxTokens      int      `json:"max_tokens"`      // token budget (0 = unlimited)
	MaxCostUSD     float64  `json:"max_cost_usd"`    // cost limit (0 = unlimited)
	ThinkingLevel  string   `json:"thinking_level"`  // off/low/medium/high
	Temperature    float64  `json:"temperature"`     // 0.0–2.0
	SystemPrompt   string   `json:"system_prompt"`   // custom instructions prepended to task
	TimeoutSeconds int      `json:"timeout_seconds"` // max runtime in seconds (0 = use default)
	AutoRestart    bool     `json:"auto_restart"`    // restart on crash
}

// AgentMetrics holds observable runtime counters for a background agent.
// All numeric counters are updated atomically so they can be read without
// holding any lock.
type AgentMetrics struct {
	TokensUsed     int64     `json:"tokens_used"`
	CostUSD        float64   `json:"cost_usd"`
	ToolCallCount  int64     `json:"tool_call_count"`
	ModelCallCount int64     `json:"model_call_count"`
	ErrorCount     int64     `json:"error_count"`
	AvgResponseMs  float64   `json:"avg_response_ms"`
	StartedAt      time.Time `json:"started_at"`
	Duration       string    `json:"duration"`
}

// BackgroundAgent represents a task-specific agent running in the background
type BackgroundAgent struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Task         string        `json:"task"`
	Status       AgentStatus   `json:"status"`
	Role         AgentRole     `json:"role"`
	Capabilities []string      `json:"capabilities,omitempty"`
	ParentID     string        `json:"parent_id,omitempty"`
	ChildIDs     []string      `json:"child_ids,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Result       string        `json:"result,omitempty"`
	Error        string        `json:"error,omitempty"`

	// Config holds runtime configuration overrides (protected by cfgMu).
	Config AgentConfig

	// Atomic metric counters — updated via the Incr*/Add* helpers.
	tokensUsed     int64
	costUSDMicros  int64 // stored as micro-dollars to allow atomic updates
	toolCallCount  int64
	modelCallCount int64
	errorCount     int64

	// Response-time tracking for AvgResponseMs (protected by metricsMu).
	metricsMu      sync.Mutex
	responseTimeMs []float64 // sliding window of recent response times

	cfgMu       sync.RWMutex
	cancel      context.CancelFunc
	logMu       sync.RWMutex
	activityLog []AgentLogEntry // ring buffer, max 200 entries
	msgMu       sync.Mutex
	inbox       []AgentMessage // pending messages from other agents
}

const maxAgentLogEntries = 200

// AppendLog adds an entry to the agent's activity log (thread-safe).
func (a *BackgroundAgent) AppendLog(kind, message string) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	a.activityLog = append(a.activityLog, AgentLogEntry{
		Time:    time.Now(),
		Kind:    kind,
		Message: message,
	})
	if len(a.activityLog) > maxAgentLogEntries {
		a.activityLog = a.activityLog[len(a.activityLog)-maxAgentLogEntries:]
	}
}

// GetLog returns a snapshot of the agent's activity log (thread-safe).
func (a *BackgroundAgent) GetLog() []AgentLogEntry {
	a.logMu.RLock()
	defer a.logMu.RUnlock()
	out := make([]AgentLogEntry, len(a.activityLog))
	copy(out, a.activityLog)
	return out
}

// GetLogTail returns the last n entries of the agent's activity log.
func (a *BackgroundAgent) GetLogTail(n int) []AgentLogEntry {
	a.logMu.RLock()
	defer a.logMu.RUnlock()
	if n >= len(a.activityLog) {
		out := make([]AgentLogEntry, len(a.activityLog))
		copy(out, a.activityLog)
		return out
	}
	out := make([]AgentLogEntry, n)
	copy(out, a.activityLog[len(a.activityLog)-n:])
	return out
}

// GetFullLog returns all entries in the agent's activity log (thread-safe).
// Unlike GetLog it is guaranteed to return the full ring-buffer even if
// the caller intends to iterate over every entry.
func (a *BackgroundAgent) GetFullLog() []AgentLogEntry {
	return a.GetLog()
}

// GetMetrics returns a snapshot of the agent's runtime metrics.
func (a *BackgroundAgent) GetMetrics() AgentMetrics {
	tokens := atomic.LoadInt64(&a.tokensUsed)
	costMicros := atomic.LoadInt64(&a.costUSDMicros)
	toolCalls := atomic.LoadInt64(&a.toolCallCount)
	modelCalls := atomic.LoadInt64(&a.modelCallCount)
	errors := atomic.LoadInt64(&a.errorCount)

	a.metricsMu.Lock()
	var avg float64
	if n := len(a.responseTimeMs); n > 0 {
		sum := 0.0
		for _, v := range a.responseTimeMs {
			sum += v
		}
		avg = sum / float64(n)
	}
	a.metricsMu.Unlock()

	dur := ""
	if !a.CreatedAt.IsZero() {
		end := time.Now()
		if a.CompletedAt != nil {
			end = *a.CompletedAt
		}
		dur = end.Sub(a.CreatedAt).Round(time.Second).String()
	}

	return AgentMetrics{
		TokensUsed:     tokens,
		CostUSD:        float64(costMicros) / 1_000_000,
		ToolCallCount:  toolCalls,
		ModelCallCount: modelCalls,
		ErrorCount:     errors,
		AvgResponseMs:  avg,
		StartedAt:      a.CreatedAt,
		Duration:       dur,
	}
}

// GetConfig returns a copy of the agent's current configuration (thread-safe).
func (a *BackgroundAgent) GetConfig() AgentConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.Config
}

// SetConfig replaces the agent's configuration (thread-safe).
func (a *BackgroundAgent) SetConfig(cfg AgentConfig) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	a.Config = cfg
}

// IncrToolCall increments the tool-call counter atomically.
func (a *BackgroundAgent) IncrToolCall() {
	atomic.AddInt64(&a.toolCallCount, 1)
}

// IncrModelCall increments the model-call counter atomically.
func (a *BackgroundAgent) IncrModelCall() {
	atomic.AddInt64(&a.modelCallCount, 1)
}

// IncrError increments the error counter atomically.
func (a *BackgroundAgent) IncrError() {
	atomic.AddInt64(&a.errorCount, 1)
}

// AddTokens adds n to the total tokens-used counter atomically.
func (a *BackgroundAgent) AddTokens(n int) {
	atomic.AddInt64(&a.tokensUsed, int64(n))
}

// AddCost adds the given USD amount to the cumulative cost counter.
// The value is stored internally as micro-dollars to enable atomic updates.
func (a *BackgroundAgent) AddCost(usd float64) {
	atomic.AddInt64(&a.costUSDMicros, int64(usd*1_000_000))
}

// recordResponseTime appends a response duration in milliseconds for the
// running average.  Only the most recent 100 samples are retained.
func (a *BackgroundAgent) recordResponseTime(ms float64) {
	const maxSamples = 100
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	a.responseTimeMs = append(a.responseTimeMs, ms)
	if len(a.responseTimeMs) > maxSamples {
		a.responseTimeMs = a.responseTimeMs[len(a.responseTimeMs)-maxSamples:]
	}
}

// AgentManager tracks and manages background agents
type AgentManager struct {
	mu     sync.RWMutex
	agents map[string]*BackgroundAgent
	nextID int
}

// NewAgentManager creates a new agent manager
func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents: make(map[string]*BackgroundAgent),
	}
}

// Create spawns a new background agent that runs the given task.
// role selects the agent's specialisation; parentID links it to the spawning agent.
func (am *AgentManager) Create(orch *Orchestrator, name, task string, role AgentRole, parentID string) *BackgroundAgent {
	if role == "" {
		role = AgentRoleGeneral
	}

	caps := agentRoleCapabilities[role]

	am.mu.Lock()
	am.nextID++
	id := fmt.Sprintf("agent_%d", am.nextID)

	agent := &BackgroundAgent{
		ID:           id,
		Name:         name,
		Task:         task,
		Status:       AgentRunning,
		Role:         role,
		Capabilities: caps,
		ParentID:     parentID,
		CreatedAt:    time.Now().UTC(),
	}
	am.agents[id] = agent

	// Register this agent as a child of its parent
	if parentID != "" {
		if parent, ok := am.agents[parentID]; ok {
			parent.ChildIDs = append(parent.ChildIDs, id)
		}
	}
	am.mu.Unlock()

	// Run the agent in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	agent.cancel = cancel

	go am.runAgent(ctx, orch, agent)

	return agent
}

// Delegate spawns a sub-agent from a parent. If wait is true it blocks until
// the sub-agent finishes and returns its result; otherwise it returns immediately
// with the new agent ID.
func (am *AgentManager) Delegate(orch *Orchestrator, parentID, task string, role AgentRole, wait bool) (agentID string, result string, err error) {
	// Derive a name from the task (first 40 chars)
	name := task
	if len(name) > 40 {
		name = name[:40]
	}

	agent := am.Create(orch, name, task, role, parentID)

	if !wait {
		return agent.ID, "", nil
	}

	// Poll until the agent is done; respect context cancellation via the agent's own timeout.
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		am.mu.RLock()
		status := agent.Status
		agentResult := agent.Result
		agentErr := agent.Error
		am.mu.RUnlock()

		switch status {
		case AgentCompleted:
			return agent.ID, agentResult, nil
		case AgentFailed:
			return agent.ID, "", fmt.Errorf("delegated agent failed: %s", agentErr)
		case AgentStopped:
			return agent.ID, "", fmt.Errorf("delegated agent was stopped before completing")
		}
	}
	return agent.ID, "", fmt.Errorf("delegate polling loop exited unexpectedly")
}

// SendMessage queues a message to a target agent's inbox.
// fromID may be "orchestrator" (the top-level run) or a registered agent ID.
func (am *AgentManager) SendMessage(fromID, toID, message string) error {
	am.mu.RLock()
	from, fromIsAgent := am.agents[fromID]
	to, toOK := am.agents[toID]
	am.mu.RUnlock()

	if !toOK {
		return fmt.Errorf("recipient agent not found: %s", toID)
	}

	fromName := fromID // default: use the ID as display name
	if fromIsAgent {
		fromName = from.Name
	}

	msg := AgentMessage{
		FromID:    fromID,
		FromName:  fromName,
		Message:   message,
		Timestamp: time.Now().UTC(),
	}

	to.msgMu.Lock()
	to.inbox = append(to.inbox, msg)
	to.msgMu.Unlock()
	return nil
}

// drainInbox atomically returns and clears the agent's inbox.
func (a *BackgroundAgent) drainInbox() []AgentMessage {
	a.msgMu.Lock()
	defer a.msgMu.Unlock()
	if len(a.inbox) == 0 {
		return nil
	}
	msgs := make([]AgentMessage, len(a.inbox))
	copy(msgs, a.inbox)
	a.inbox = a.inbox[:0]
	return msgs
}

// SelectBestAgent returns the running agent whose role best matches the task,
// or nil if no suitable running agent exists.
func (am *AgentManager) SelectBestAgent(task string) *BackgroundAgent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	taskLower := strings.ToLower(task)

	// Role relevance heuristics: keywords that hint at a role
	roleKeywords := map[AgentRole][]string{
		AgentRoleCoder:    {"code", "implement", "fix", "refactor", "debug", "function", "class", "test", "build", "compile"},
		AgentRoleResearch: {"research", "find", "search", "look up", "summarise", "summarize", "fetch", "web", "url", "news"},
		AgentRoleOps:      {"deploy", "run", "execute", "server", "process", "monitor", "restart", "install", "configure", "ops"},
	}

	// Score each running agent
	bestScore := -1
	var bestAgent *BackgroundAgent
	for _, a := range am.agents {
		if a.Status != AgentRunning {
			continue
		}
		score := 0
		if keywords, ok := roleKeywords[a.Role]; ok {
			for _, kw := range keywords {
				if strings.Contains(taskLower, kw) {
					score++
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestAgent = a
		}
	}

	if bestScore <= 0 {
		return nil // No sufficiently relevant agent found
	}
	return bestAgent
}

// runAgent executes the agent's task using its own agentic loop
func (am *AgentManager) runAgent(ctx context.Context, orch *Orchestrator, agent *BackgroundAgent) {
	defer agent.cancel()

	// Build role directive
	roleDirective := agentRoleDescriptions[agent.Role]
	if roleDirective == "" {
		roleDirective = agentRoleDescriptions[AgentRoleGeneral]
	}

	// Build agent-specific prompt (include parent context if delegated)
	parentClause := ""
	if agent.ParentID != "" {
		parentClause = fmt.Sprintf(" You were delegated this task by agent '%s'.", agent.ParentID)
	}

	agentPrompt := fmt.Sprintf(
		"%s%s\n\nYour name is '%s' and your specific task is:\n\n%s\n\n"+
			"Complete this task thoroughly, then provide a clear summary of what you did and the results. "+
			"Be concise but complete.",
		roleDirective, parentClause, agent.Name, agent.Task,
	)

	// Create a dedicated run for this agent
	run := orch.session.CreateRun(fmt.Sprintf("[agent:%s] %s", agent.Name, agent.Task))
	run.Start()

	// Log agent start
	orch.audit.Log(ctx, audit.NewEvent("agent_start", audit.CategoryRun).
		WithSessionID(orch.session.ID).
		WithRunID(run.ID).
		WithMetadata("agent_id", agent.ID).
		WithMetadata("agent_name", agent.Name).
		WithMetadata("task", agent.Task))

	// Attach agent ID to context so tool dispatch can route inter-agent calls
	ctx = withAgentID(ctx, agent.ID)

	// Execute the agentic loop
	response, err := orch.executeAgentLoop(ctx, agentPrompt, run.ID, agent)

	am.mu.Lock()
	now := time.Now().UTC()
	agent.CompletedAt = &now

	if err != nil {
		agent.Status = AgentFailed
		agent.Error = err.Error()
		run.SetError(err)
	} else {
		agent.Status = AgentCompleted
		agent.Result = response
		run.SetResult(response)
	}
	am.mu.Unlock()

	// Log agent completion
	auditCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orch.audit.Log(auditCtx, audit.NewEvent("agent_complete", audit.CategoryRun).
		WithSessionID(orch.session.ID).
		WithRunID(run.ID).
		WithMetadata("agent_id", agent.ID).
		WithMetadata("status", string(agent.Status)))
}

// List returns all agents
func (am *AgentManager) List() []*BackgroundAgent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agents := make([]*BackgroundAgent, 0, len(am.agents))
	for _, a := range am.agents {
		agents = append(agents, a)
	}
	return agents
}

// Get returns a specific agent by ID
func (am *AgentManager) Get(id string) (*BackgroundAgent, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	a, ok := am.agents[id]
	return a, ok
}

// Stop cancels a running agent
func (am *AgentManager) Stop(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	agent, ok := am.agents[id]
	if !ok {
		return fmt.Errorf("agent not found: %s", id)
	}

	if agent.Status != AgentRunning {
		return fmt.Errorf("agent is not running (status: %s)", agent.Status)
	}

	agent.cancel()
	now := time.Now().UTC()
	agent.Status = AgentStopped
	agent.CompletedAt = &now
	return nil
}

// executeAgentLoop is like executeAgenticLoop but for background agents.
// It shares the orchestrator's brokers and providers but uses a separate conversation.
// The agent parameter receives live activity log entries.
func (o *Orchestrator) executeAgentLoop(ctx context.Context, prompt string, runID string, agent *BackgroundAgent) (string, error) {
	defaults := DefaultExecutionLimits()
	limits := ExecutionLimits{
		MaxIterations:     defaults.MaxIterations,
		TotalTimeout:      3 * time.Minute,
		IterationTimeout:  defaults.IterationTimeout,
		APICallTimeout:    defaults.APICallTimeout,
		MaxTokens:         defaults.MaxTokens,
		MaxToolResultSize: defaults.MaxToolResultSize,
	}
	tracker := NewExecutionTracker(limits)

	messages := []model.Message{
		{Role: model.RoleUser, Content: prompt},
	}

	tools := o.getToolSchemas()

	// Build the set of tools disallowed for sub-agents
	topLevelAgentTools := map[string]bool{
		"agent_create": true,
		"agent_list":   true,
		"agent_stop":   true,
	}
	// Sub-agents get agent_delegate and agent_message but not top-level creation
	// Top-level agents (no parent) keep all agent tools
	isSubAgent := agent.ParentID != ""

	// Build capability allow-set (nil means allow all)
	var capSet map[string]bool
	if len(agent.Capabilities) > 0 {
		capSet = make(map[string]bool, len(agent.Capabilities))
		for _, c := range agent.Capabilities {
			capSet[c] = true
		}
		// Sub-agents always get inter-agent communication tools
		capSet["agent_delegate"] = true
		capSet["agent_message"] = true
	}

	filtered := make([]model.ToolSchema, 0, len(tools))
	for _, t := range tools {
		// Sub-agents cannot spawn top-level agents
		if isSubAgent && topLevelAgentTools[t.Name] {
			continue
		}
		// Apply capability filter if set
		if capSet != nil && !capSet[t.Name] {
			continue
		}
		filtered = append(filtered, t)
	}
	tools = filtered

	var toolNames []string
	for _, t := range tools {
		toolNames = append(toolNames, t.Name)
	}

	currentProvider, currentModel := o.GetCurrentProvider()
	systemPrompt := buildSystemPrompt(o.workspace.Root, o.workspace.ConfigDir, toolNames, currentProvider, currentModel)

	for {
		if err := tracker.BeginIteration(); err != nil {
			return "", err
		}

		agent.AppendLog("iteration", fmt.Sprintf("iteration %d", tracker.iterations))

		// Inject any pending inbox messages as a user message so the model
		// can react to inter-agent communication before calling the API.
		if inboxMsgs := agent.drainInbox(); len(inboxMsgs) > 0 {
			var sb strings.Builder
			sb.WriteString("[Agent inbox — messages received from other agents]\n")
			for _, m := range inboxMsgs {
				sb.WriteString(fmt.Sprintf("From %s (%s): %s\n", m.FromName, m.FromID, m.Message))
			}
			messages = append(messages, model.Message{
				Role:    model.RoleUser,
				Content: sb.String(),
			})
		}

		req := model.CompletionRequest{
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   o.workspace.Config.Model.OpenAI.MaxTokens,
			Temperature: o.workspace.Config.Model.OpenAI.Temperature,
			System:      systemPrompt,
		}

		agent.AppendLog("model_call", fmt.Sprintf("calling %s...", o.provider.Name()))
		agent.IncrModelCall()

		modelStart := time.Now()
		resp, err := o.callModelWithFallback(ctx, tracker, req)
		if err != nil {
			agent.AppendLog("error", fmt.Sprintf("model error: %v", err))
			agent.IncrError()
			return "", fmt.Errorf("model provider error: %w", err)
		}

		modelDur := time.Since(modelStart).Round(time.Millisecond)
		agent.recordResponseTime(float64(modelDur.Milliseconds()))

		modelName := resp.Model
		if modelName == "" {
			modelName = o.provider.Name()
		}
		agent.AppendLog("model_done", fmt.Sprintf("%s responded (%s, %d tok, %s)",
			modelName, resp.StopReason, resp.Usage.TotalTokens, modelDur))

		agent.AddTokens(resp.Usage.TotalTokens)
		if err := tracker.AddTokens(resp.Usage.TotalTokens); err != nil {
			return "", err
		}

		assistantMsg := resp.Message
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		messages = append(messages, assistantMsg)

		// Log assistant text (truncated) so viewers can see what the agent is thinking
		if resp.Message.Content != "" {
			text := resp.Message.Content
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			agent.AppendLog("text", text)
		}

		if resp.StopReason == model.StopReasonEndTurn || resp.StopReason == model.StopReasonMaxTokens {
			agent.AppendLog("text", "agent finished")
			return resp.Message.Content, nil
		}

		if resp.StopReason == model.StopReasonToolUse && len(resp.ToolCalls) > 0 {
			// Log each tool call
			for _, tc := range resp.ToolCalls {
				argsSummary := string(tc.Input)
				if len(argsSummary) > 120 {
					argsSummary = argsSummary[:120] + "..."
				}
				agent.AppendLog("tool_start", fmt.Sprintf("%s %s", tc.Name, argsSummary))
				agent.IncrToolCall()
			}

			toolResults := o.executeToolCallsParallel(ctx, runID, tracker, resp.ToolCalls)

			// Log tool results
			for _, msg := range toolResults {
				result := msg.Content
				if len(result) > 200 {
					result = result[:200] + "..."
				}
				agent.AppendLog("tool_done", fmt.Sprintf("%s → %s", msg.Name, result))
			}

			messages = append(messages, toolResults...)
			continue
		}

		return "", fmt.Errorf("unexpected stop reason: %s", resp.StopReason)
	}
}

// handleAgentCreate handles the agent_create tool call
func (o *Orchestrator) handleAgentCreate(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Name string `json:"name"`
		Task string `json:"task"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	if params.Name == "" {
		return "", fmt.Errorf("agent name is required")
	}
	if params.Task == "" {
		return "", fmt.Errorf("agent task is required")
	}

	role := AgentRole(params.Role)
	if role == "" {
		role = AgentRoleGeneral
	}

	agent := o.agentManager.Create(o, params.Name, params.Task, role, "")

	result, _ := json.Marshal(map[string]string{
		"status":  "created",
		"id":      agent.ID,
		"name":    agent.Name,
		"role":    string(agent.Role),
		"message": fmt.Sprintf("Agent '%s' (role: %s) created and running in background. Use agent_list to check status or agent_stop to cancel.", agent.Name, agent.Role),
	})
	return string(result), nil
}

// handleAgentDelegate handles the agent_delegate tool call.
// It spawns a sub-agent. If wait=true it blocks until the sub-agent finishes.
// The caller's agent ID is resolved from the request context.
func (o *Orchestrator) handleAgentDelegate(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Task string `json:"task"`
		Role string `json:"role"`
		Wait bool   `json:"wait"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}
	if params.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	role := AgentRole(params.Role)
	if role == "" {
		role = AgentRoleGeneral
	}

	// The caller may be an agent or the top-level orchestrator loop
	callerID := agentIDFromContext(ctx)

	subAgentID, agentResult, err := o.agentManager.Delegate(o, callerID, params.Task, role, params.Wait)
	if err != nil {
		return "", fmt.Errorf("delegation failed: %w", err)
	}

	if params.Wait {
		result, _ := json.Marshal(map[string]string{
			"status":   "completed",
			"agent_id": subAgentID,
			"result":   agentResult,
		})
		return string(result), nil
	}

	result, _ := json.Marshal(map[string]string{
		"status":   "delegated",
		"agent_id": subAgentID,
		"message":  fmt.Sprintf("Sub-agent %s started in background. Use agent_list to monitor.", subAgentID),
	})
	return string(result), nil
}

// handleAgentMessage handles the agent_message tool call.
// The caller's agent ID is resolved from the request context.
func (o *Orchestrator) handleAgentMessage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		AgentID string `json:"agent_id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}
	if params.AgentID == "" {
		return "", fmt.Errorf("agent_id is required")
	}
	if params.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	callerID := agentIDFromContext(ctx)
	if callerID == "" {
		// Called from the top-level orchestrator; use a placeholder sender
		callerID = "orchestrator"
	}

	if err := o.agentManager.SendMessage(callerID, params.AgentID, params.Message); err != nil {
		return "", err
	}

	result, _ := json.Marshal(map[string]string{
		"status":  "sent",
		"to":      params.AgentID,
		"message": "Message delivered to agent's inbox",
	})
	return string(result), nil
}

// handleAgentList handles the agent_list tool call
func (o *Orchestrator) handleAgentList(ctx context.Context, input json.RawMessage) (string, error) {
	agents := o.agentManager.List()

	if len(agents) == 0 {
		return `{"agents": [], "message": "No agents running"}`, nil
	}

	type agentInfo struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Task         string   `json:"task"`
		Status       string   `json:"status"`
		Role         string   `json:"role"`
		Capabilities []string `json:"capabilities,omitempty"`
		ParentID     string   `json:"parent_id,omitempty"`
		ChildIDs     []string `json:"child_ids,omitempty"`
		CreatedAt    string   `json:"created_at"`
		CompletedAt  string   `json:"completed_at,omitempty"`
		Result       string   `json:"result,omitempty"`
		Error        string   `json:"error,omitempty"`
	}

	infos := make([]agentInfo, 0, len(agents))
	for _, a := range agents {
		info := agentInfo{
			ID:           a.ID,
			Name:         a.Name,
			Task:         a.Task,
			Status:       string(a.Status),
			Role:         string(a.Role),
			Capabilities: a.Capabilities,
			ParentID:     a.ParentID,
			ChildIDs:     a.ChildIDs,
			CreatedAt:    a.CreatedAt.Format(time.RFC3339),
		}
		if a.CompletedAt != nil {
			info.CompletedAt = a.CompletedAt.Format(time.RFC3339)
		}
		if a.Result != "" {
			// Truncate long results
			result := a.Result
			if len(result) > 500 {
				result = result[:500] + "..."
			}
			info.Result = result
		}
		if a.Error != "" {
			info.Error = a.Error
		}
		infos = append(infos, info)
	}

	result, _ := json.Marshal(map[string]interface{}{
		"agents": infos,
		"count":  len(infos),
	})
	return string(result), nil
}

// handleAgentStop handles the agent_stop tool call
func (o *Orchestrator) handleAgentStop(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	if err := o.agentManager.Stop(params.ID); err != nil {
		return "", err
	}

	result, _ := json.Marshal(map[string]string{
		"status":  "stopped",
		"id":      params.ID,
		"message": fmt.Sprintf("Agent %s stopped", params.ID),
	})
	return string(result), nil
}
