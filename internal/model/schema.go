package model

import "encoding/json"

// Message represents a conversational message
type Message struct {
	Role       string     `json:"role"` // "user", "assistant", "system", "tool"
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`         // For tool messages
	ToolUse    *ToolUse   `json:"tool_use,omitempty"`     // For Anthropic tool use
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // For assistant messages with tool calls
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool result messages
}

// ToolUse represents a tool use block (Anthropic format)
type ToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolSchema defines a tool that can be called by the model
type ToolSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema object
}

// ToolCall represents a request from the model to call a tool
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"` // JSON object
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// CompletionRequest represents a request to a model provider
type CompletionRequest struct {
	Messages    []Message    `json:"messages"`
	Tools       []ToolSchema `json:"tools,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
	System      string       `json:"system,omitempty"` // System prompt
}

// CompletionResponse represents a response from a model provider
type CompletionResponse struct {
	Message    Message    `json:"message"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"` // "end_turn", "tool_use", "max_tokens"
	Usage      TokenUsage `json:"usage"`
	Model      string     `json:"model,omitempty"` // Actual model used (from API response)
}

// StopReason constants
const (
	StopReasonEndTurn   = "end_turn"
	StopReasonToolUse   = "tool_use"
	StopReasonMaxTokens = "max_tokens"
	StopReasonError     = "error"
)

// Role constants
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleTool      = "tool"
)
