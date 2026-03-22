package core

import (
	"context"
	"fmt"
	"time"
)

// ExecutionLimits defines resource limits for agentic loop execution
type ExecutionLimits struct {
	// Maximum number of iterations in the agentic loop
	MaxIterations int

	// Maximum total execution time
	TotalTimeout time.Duration

	// Timeout per iteration (model call + tool execution)
	IterationTimeout time.Duration

	// Timeout for API calls to model provider
	APICallTimeout time.Duration

	// Maximum cumulative token budget
	MaxTokens int

	// Maximum size of tool result (bytes)
	MaxToolResultSize int
}

// DefaultExecutionLimits returns defaults with no artificial caps.
// The agentic loop runs until the model is done or the context is cancelled.
func DefaultExecutionLimits() ExecutionLimits {
	return ExecutionLimits{
		MaxIterations:     0,                // 0 = unlimited
		TotalTimeout:      0,                // 0 = no timeout
		IterationTimeout:  0,                // 0 = no timeout
		APICallTimeout:    0,                // 0 = no timeout
		MaxTokens:         0,                // 0 = unlimited
		MaxToolResultSize: 10 * 1024 * 1024, // 10MB
	}
}

// ExecutionTracker tracks resource usage during execution
type ExecutionTracker struct {
	limits             ExecutionLimits
	startTime          time.Time
	iterations         int
	tokensUsed         int
	lastIterationStart time.Time
}

// NewExecutionTracker creates a new execution tracker
func NewExecutionTracker(limits ExecutionLimits) *ExecutionTracker {
	return &ExecutionTracker{
		limits:    limits,
		startTime: time.Now(),
	}
}

// BeginIteration marks the start of an iteration
func (t *ExecutionTracker) BeginIteration() error {
	t.iterations++
	t.lastIterationStart = time.Now()

	if t.limits.MaxIterations > 0 && t.iterations > t.limits.MaxIterations {
		return &LimitExceededError{
			Limit: "max_iterations",
			Value: t.iterations,
			Max:   t.limits.MaxIterations,
		}
	}

	if t.limits.TotalTimeout > 0 {
		elapsed := time.Since(t.startTime)
		if elapsed > t.limits.TotalTimeout {
			return &LimitExceededError{
				Limit: "total_timeout",
				Value: int(elapsed.Seconds()),
				Max:   int(t.limits.TotalTimeout.Seconds()),
			}
		}
	}

	return nil
}

// AddTokens adds token usage and checks against budget
func (t *ExecutionTracker) AddTokens(tokens int) error {
	t.tokensUsed += tokens

	if t.limits.MaxTokens > 0 && t.tokensUsed > t.limits.MaxTokens {
		return &LimitExceededError{
			Limit: "max_tokens",
			Value: t.tokensUsed,
			Max:   t.limits.MaxTokens,
		}
	}

	return nil
}

// ValidateToolResultSize checks tool result size
func (t *ExecutionTracker) ValidateToolResultSize(result string) error {
	size := len(result)
	if size > t.limits.MaxToolResultSize {
		return &LimitExceededError{
			Limit: "tool_result_size",
			Value: size,
			Max:   t.limits.MaxToolResultSize,
		}
	}
	return nil
}

// IterationContext creates a context with iteration timeout
func (t *ExecutionTracker) IterationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if t.limits.IterationTimeout > 0 {
		return context.WithTimeout(parent, t.limits.IterationTimeout)
	}
	return context.WithCancel(parent)
}

// APICallContext creates a context with API call timeout
func (t *ExecutionTracker) APICallContext(parent context.Context) (context.Context, context.CancelFunc) {
	if t.limits.APICallTimeout > 0 {
		return context.WithTimeout(parent, t.limits.APICallTimeout)
	}
	return context.WithCancel(parent)
}

// Stats returns current execution statistics
func (t *ExecutionTracker) Stats() ExecutionStats {
	return ExecutionStats{
		Iterations: t.iterations,
		TokensUsed: t.tokensUsed,
		Elapsed:    time.Since(t.startTime),
	}
}

// ExecutionStats represents execution statistics
type ExecutionStats struct {
	Iterations int
	TokensUsed int
	Elapsed    time.Duration
}

// LimitExceededError indicates a resource limit was exceeded
type LimitExceededError struct {
	Limit string
	Value int
	Max   int
}

func (e *LimitExceededError) Error() string {
	return fmt.Sprintf("resource limit exceeded: %s=%d (max: %d)", e.Limit, e.Value, e.Max)
}
