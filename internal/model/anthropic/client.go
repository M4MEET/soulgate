package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/M4MEET/soulgate/internal/httpclient"
	"github.com/M4MEET/soulgate/internal/model"
)

// Provider implements the Anthropic provider
type Provider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewProvider creates a new Anthropic provider with secure HTTP client
func NewProvider(apiKey, modelName, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	if modelName == "" {
		modelName = "claude-sonnet-4-20250514"
	}

	// Create secure HTTP client with no timeouts (context handles cancellation)
	secureConfig := httpclient.DefaultSecureConfig()
	secureConfig.TotalTimeout = 0
	secureConfig.ResponseTimeout = 0
	secureConfig.UserAgent = "SoulGate-Anthropic/0.1"

	return &Provider{
		apiKey:  apiKey,
		model:   modelName,
		baseURL: baseURL,
		client:  httpclient.NewSecureClient(secureConfig),
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "anthropic"
}

// SupportedFeatures returns the features supported by Anthropic
func (p *Provider) SupportedFeatures() model.FeatureSet {
	return model.FeatureSet{
		ToolCalling:     true,
		VisionSupport:   true,
		SystemMessages:  true,
		StreamingOutput: true,
	}
}

// StreamComplete falls back to Complete for Anthropic (streaming not yet implemented)
func (p *Provider) StreamComplete(ctx context.Context, req model.CompletionRequest) (<-chan model.StreamChunk, error) {
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	ch := make(chan model.StreamChunk, 2)
	go func() {
		defer close(ch)
		if resp.Message.Content != "" {
			ch <- model.StreamChunk{Delta: resp.Message.Content}
		}
		ch <- model.StreamChunk{Done: true, Response: resp}
	}()
	return ch, nil
}

// Complete sends a completion request to Anthropic
func (p *Provider) Complete(ctx context.Context, req model.CompletionRequest) (*model.CompletionResponse, error) {
	// Convert to Anthropic format
	anthropicReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	// Enable prompt caching — reduces input token cost by 90% on cache hits
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	// Send request
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to common format
	return p.convertResponse(anthropicResp), nil
}

// convertRequest converts common format to Anthropic format.
// Anthropic requires strict alternation between user and assistant roles,
// and tool results must be grouped into a single user message.
func (p *Provider) convertRequest(req model.CompletionRequest) anthropicRequest {
	anthropicReq := anthropicRequest{
		Model:       p.model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Messages:    make([]anthropicMessage, 0),
	}

	// Build system as a content block array with cache_control so Anthropic
	// can cache the (typically large, rarely changing) system prompt.
	if req.System != "" {
		anthropicReq.System = []anthropicSystemBlock{
			{
				Type:         "text",
				Text:         req.System,
				CacheControl: &anthropicCacheControl{Type: "ephemeral"},
			},
		}
	}

	// Convert messages, grouping consecutive tool results and enforcing alternation.
	for _, msg := range req.Messages {
		switch msg.Role {
		case model.RoleTool:
			// Tool results become user messages with tool_result content blocks.
			// If the last message is already a user message (from a prior tool result),
			// merge into it to satisfy Anthropic's alternation requirement.
			toolContent := anthropicContent{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}

			n := len(anthropicReq.Messages)
			if n > 0 && anthropicReq.Messages[n-1].Role == "user" {
				anthropicReq.Messages[n-1].Content = append(
					anthropicReq.Messages[n-1].Content, toolContent)
			} else {
				anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
					Role:    "user",
					Content: []anthropicContent{toolContent},
				})
			}

		case model.RoleAssistant:
			content := make([]anthropicContent, 0)
			if msg.Content != "" {
				content = append(content, anthropicContent{
					Type: "text",
					Text: msg.Content,
				})
			}
			// Add tool_use blocks for tool calls
			for _, tc := range msg.ToolCalls {
				content = append(content, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			if len(content) == 0 {
				// Anthropic requires non-empty content
				content = append(content, anthropicContent{Type: "text", Text: " "})
			}

			// Merge consecutive assistant messages (shouldn't happen, but be safe)
			n := len(anthropicReq.Messages)
			if n > 0 && anthropicReq.Messages[n-1].Role == "assistant" {
				anthropicReq.Messages[n-1].Content = append(
					anthropicReq.Messages[n-1].Content, content...)
			} else {
				anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
					Role:    "assistant",
					Content: content,
				})
			}

		case model.RoleUser:
			content := []anthropicContent{{Type: "text", Text: msg.Content}}

			// Merge consecutive user messages
			n := len(anthropicReq.Messages)
			if n > 0 && anthropicReq.Messages[n-1].Role == "user" {
				anthropicReq.Messages[n-1].Content = append(
					anthropicReq.Messages[n-1].Content, content...)
			} else {
				anthropicReq.Messages = append(anthropicReq.Messages, anthropicMessage{
					Role:    "user",
					Content: content,
				})
			}

		case model.RoleSystem:
			// Skip system messages — they go in the system field, not messages
			continue
		}
	}

	// Convert tool schemas.
	// Cache control is placed on the LAST tool so the entire tool list is cached
	// as a single cache entry — the cache boundary is set at that position.
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		last := len(req.Tools) - 1
		for i, tool := range req.Tools {
			var schema map[string]interface{}
			json.Unmarshal(tool.InputSchema, &schema)

			t := anthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schema,
			}
			if i == last {
				t.CacheControl = &anthropicCacheControl{Type: "ephemeral"}
			}
			anthropicReq.Tools[i] = t
		}
	}

	return anthropicReq
}

// convertResponse converts Anthropic format to common format
func (p *Provider) convertResponse(resp anthropicResponse) *model.CompletionResponse {
	result := &model.CompletionResponse{
		Message: model.Message{
			Role:    resp.Role,
			Content: "",
		},
		StopReason: resp.StopReason,
		Usage: model.TokenUsage{
			PromptTokens:        resp.Usage.InputTokens,
			CompletionTokens:    resp.Usage.OutputTokens,
			TotalTokens:         resp.Usage.InputTokens + resp.Usage.OutputTokens,
			CacheCreationTokens: resp.Usage.CacheCreationInputTokens,
			CacheReadTokens:     resp.Usage.CacheReadInputTokens,
		},
		Model: resp.Model,
	}

	// Extract text and tool calls from content blocks
	var textParts []string
	var toolCalls []model.ToolCall

	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			if content.Text != "" {
				textParts = append(textParts, content.Text)
			}
		case "tool_use":
			toolCalls = append(toolCalls, model.ToolCall{
				ID:    content.ID,
				Name:  content.Name,
				Input: content.Input,
			})
		}
	}

	// Join text parts
	if len(textParts) > 0 {
		result.Message.Content = textParts[0]
		for i := 1; i < len(textParts); i++ {
			result.Message.Content += "\n" + textParts[i]
		}
	}

	// Add tool calls
	if len(toolCalls) > 0 {
		result.ToolCalls = toolCalls
	}

	return result
}

// Anthropic API types

// anthropicCacheControl instructs Anthropic to cache the associated content block.
type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// anthropicSystemBlock is a single content block inside the system array.
// Using an array (rather than a plain string) is required to attach cache_control.
type anthropicSystemBlock struct {
	Type         string                 `json:"type"` // always "text"
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicRequest struct {
	Model       string                 `json:"model"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature,omitempty"`
	System      []anthropicSystemBlock `json:"system,omitempty"`
	Messages    []anthropicMessage     `json:"messages"`
	Tools       []anthropicTool        `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type      string          `json:"type"` // "text", "tool_use", "tool_result"
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Role       string             `json:"role"`
	Content    []anthropicContent `json:"content"`
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}
