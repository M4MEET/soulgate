package core

import "time"

// ThinkingEventKind identifies the type of thinking event
type ThinkingEventKind string

const (
	ThinkingIteration  ThinkingEventKind = "iteration"   // New agentic loop iteration
	ThinkingModelCall  ThinkingEventKind = "model_call"  // Calling the model
	ThinkingModelDone  ThinkingEventKind = "model_done"  // Model responded
	ThinkingToolStart  ThinkingEventKind = "tool_start"  // Starting a tool call
	ThinkingToolDone   ThinkingEventKind = "tool_done"   // Tool call completed
	ThinkingTokenUsage ThinkingEventKind = "token_usage" // Token usage update
	ThinkingStatus     ThinkingEventKind = "status"      // General status message
)

// ThinkingEvent represents a live event from the agentic loop
type ThinkingEvent struct {
	Kind      ThinkingEventKind
	Timestamp time.Time

	// Iteration info
	Iteration int

	// Model info
	Provider string
	Model    string

	// Tool info
	ToolName   string
	ToolArgs   string // Abbreviated args
	ToolResult string // Abbreviated result
	Duration   time.Duration

	// Token info
	TokensUsed int
	StopReason string

	// General
	Message string
}

// SetThinkingCallback sets a callback for live thinking events
func (o *Orchestrator) SetThinkingCallback(cb func(ThinkingEvent)) {
	o.thinkingCallback = cb
}

// GetThinkingCallback returns the current thinking callback (may be nil).
func (o *Orchestrator) GetThinkingCallback() func(ThinkingEvent) {
	return o.thinkingCallback
}

// emitThinking sends a thinking event if a callback is registered
func (o *Orchestrator) emitThinking(event ThinkingEvent) {
	if o.thinkingCallback != nil {
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		o.thinkingCallback(event)
	}
}
