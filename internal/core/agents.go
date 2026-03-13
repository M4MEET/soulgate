package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/model"
)

// AgentStatus represents the current state of a background agent
type AgentStatus string

const (
	AgentRunning   AgentStatus = "running"
	AgentCompleted AgentStatus = "completed"
	AgentFailed    AgentStatus = "failed"
	AgentStopped   AgentStatus = "stopped"
)

// BackgroundAgent represents a task-specific agent running in the background
type BackgroundAgent struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Task        string      `json:"task"`
	Status      AgentStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Result      string      `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
	cancel      context.CancelFunc
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

// Create spawns a new background agent that runs the given task
func (am *AgentManager) Create(orch *Orchestrator, name, task string) *BackgroundAgent {
	am.mu.Lock()
	am.nextID++
	id := fmt.Sprintf("agent_%d", am.nextID)

	agent := &BackgroundAgent{
		ID:        id,
		Name:      name,
		Task:      task,
		Status:    AgentRunning,
		CreatedAt: time.Now().UTC(),
	}
	am.agents[id] = agent
	am.mu.Unlock()

	// Run the agent in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	agent.cancel = cancel

	go am.runAgent(ctx, orch, agent)

	return agent
}

// runAgent executes the agent's task using its own agentic loop
func (am *AgentManager) runAgent(ctx context.Context, orch *Orchestrator, agent *BackgroundAgent) {
	defer agent.cancel()

	// Build agent-specific prompt
	agentPrompt := fmt.Sprintf(
		"You are a background agent named '%s'. Your specific task is:\n\n%s\n\n"+
			"Complete this task thoroughly, then provide a clear summary of what you did and the results. "+
			"Be concise but complete.",
		agent.Name, agent.Task,
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

	// Execute the agentic loop
	response, err := orch.executeAgentLoop(ctx, agentPrompt, run.ID)

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
func (o *Orchestrator) executeAgentLoop(ctx context.Context, prompt string, runID string) (string, error) {
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

	// Filter out agent tools to prevent recursive agent creation
	filtered := make([]model.ToolSchema, 0, len(tools))
	for _, t := range tools {
		if t.Name != "agent_create" && t.Name != "agent_list" && t.Name != "agent_stop" {
			filtered = append(filtered, t)
		}
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

		req := model.CompletionRequest{
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   o.workspace.Config.Model.OpenAI.MaxTokens,
			Temperature: o.workspace.Config.Model.OpenAI.Temperature,
			System:      systemPrompt,
		}

		apiCtx, apiCancel := tracker.APICallContext(ctx)
		resp, err := o.provider.Complete(apiCtx, req)
		apiCancel()
		if err != nil {
			return "", fmt.Errorf("model provider error: %w", err)
		}

		if err := tracker.AddTokens(resp.Usage.TotalTokens); err != nil {
			return "", err
		}

		assistantMsg := resp.Message
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		messages = append(messages, assistantMsg)

		if resp.StopReason == model.StopReasonEndTurn || resp.StopReason == model.StopReasonMaxTokens {
			return resp.Message.Content, nil
		}

		if resp.StopReason == model.StopReasonToolUse && len(resp.ToolCalls) > 0 {
			toolResults := o.executeToolCallsParallel(ctx, runID, tracker, resp.ToolCalls)
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

	agent := o.agentManager.Create(o, params.Name, params.Task)

	result, _ := json.Marshal(map[string]string{
		"status":  "created",
		"id":      agent.ID,
		"name":    agent.Name,
		"message": fmt.Sprintf("Agent '%s' created and running in background. Use agent_list to check status or agent_stop to cancel.", agent.Name),
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
		ID          string `json:"id"`
		Name        string `json:"name"`
		Task        string `json:"task"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		CompletedAt string `json:"completed_at,omitempty"`
		Result      string `json:"result,omitempty"`
		Error       string `json:"error,omitempty"`
	}

	infos := make([]agentInfo, 0, len(agents))
	for _, a := range agents {
		info := agentInfo{
			ID:        a.ID,
			Name:      a.Name,
			Task:      a.Task,
			Status:    string(a.Status),
			CreatedAt: a.CreatedAt.Format(time.RFC3339),
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
