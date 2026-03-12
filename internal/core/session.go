package core

import (
	"fmt"
	"time"
)

// SessionStatus represents the state of a session
type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionCompleted SessionStatus = "completed"
	SessionError     SessionStatus = "error"
)

// Session represents a user interaction session
type Session struct {
	ID          string
	WorkspaceID string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Status      SessionStatus
	Context     map[string]interface{}
	Runs        []*Run
}

// NewSession creates a new session
func NewSession(workspaceID string) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:          generateSessionID(),
		WorkspaceID: workspaceID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Status:      SessionActive,
		Context:     make(map[string]interface{}),
		Runs:        make([]*Run, 0),
	}
}

// CreateRun creates a new run within this session
func (s *Session) CreateRun(prompt string) *Run {
	run := NewRun(s.ID, prompt)
	s.Runs = append(s.Runs, run)
	s.UpdatedAt = time.Now().UTC()
	return run
}

// GetRun retrieves a run by ID
func (s *Session) GetRun(runID string) (*Run, error) {
	for _, run := range s.Runs {
		if run.ID == runID {
			return run, nil
		}
	}
	return nil, fmt.Errorf("run not found: %s", runID)
}

// Complete marks the session as completed
func (s *Session) Complete() {
	s.Status = SessionCompleted
	s.UpdatedAt = time.Now().UTC()
}

// MarkError marks the session as errored
func (s *Session) MarkError() {
	s.Status = SessionError
	s.UpdatedAt = time.Now().UTC()
}

// RunStatus represents the state of a run
type RunStatus string

const (
	RunPending    RunStatus = "pending"
	RunInProgress RunStatus = "in_progress"
	RunCompleted  RunStatus = "completed"
	RunError      RunStatus = "error"
)

// Run represents a single execution within a session
type Run struct {
	ID             string
	SessionID      string
	Prompt         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Status         RunStatus
	ModelCalls     []*ModelCall
	ToolExecutions []*ToolExecution
	Result         string
	Error          string
}

// NewRun creates a new run
func NewRun(sessionID, prompt string) *Run {
	now := time.Now().UTC()
	return &Run{
		ID:             generateRunID(),
		SessionID:      sessionID,
		Prompt:         prompt,
		CreatedAt:      now,
		UpdatedAt:      now,
		Status:         RunPending,
		ModelCalls:     make([]*ModelCall, 0),
		ToolExecutions: make([]*ToolExecution, 0),
	}
}

// AddModelCall records a model API call
func (r *Run) AddModelCall(call *ModelCall) {
	r.ModelCalls = append(r.ModelCalls, call)
	r.UpdatedAt = time.Now().UTC()
}

// AddToolExecution records a tool execution
func (r *Run) AddToolExecution(execution *ToolExecution) {
	r.ToolExecutions = append(r.ToolExecutions, execution)
	r.UpdatedAt = time.Now().UTC()
}

// SetResult sets the final result
func (r *Run) SetResult(result string) {
	r.Result = result
	r.Status = RunCompleted
	r.UpdatedAt = time.Now().UTC()
}

// SetError sets an error and marks the run as errored
func (r *Run) SetError(err error) {
	if err != nil {
		r.Error = err.Error()
		r.Status = RunError
		r.UpdatedAt = time.Now().UTC()
	}
}

// Start marks the run as in progress
func (r *Run) Start() {
	r.Status = RunInProgress
	r.UpdatedAt = time.Now().UTC()
}

// ModelCall represents a call to an LLM provider
type ModelCall struct {
	ID         string
	Provider   string
	Model      string
	Timestamp  time.Time
	TokensUsed int
	ToolCalls  []string // Tool call IDs
}

// NewModelCall creates a new model call record
func NewModelCall(provider, model string) *ModelCall {
	return &ModelCall{
		ID:        generateID(),
		Provider:  provider,
		Model:     model,
		Timestamp: time.Now().UTC(),
		ToolCalls: make([]string, 0),
	}
}

// ToolExecution represents the execution of a tool
type ToolExecution struct {
	ID        string
	PluginID  string
	ToolName  string
	Timestamp time.Time
	Input     map[string]interface{}
	Output    map[string]interface{}
	Error     string
	Duration  time.Duration
}

// NewToolExecution creates a new tool execution record
func NewToolExecution(pluginID, toolName string) *ToolExecution {
	return &ToolExecution{
		ID:        generateID(),
		PluginID:  pluginID,
		ToolName:  toolName,
		Timestamp: time.Now().UTC(),
	}
}

// ID generation functions
func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func generateRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
