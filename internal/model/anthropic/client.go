package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
		modelName = "claude-3-5-sonnet-20241022"
	}

	// Create secure HTTP client
	secureConfig := httpclient.DefaultSecureConfig()
	secureConfig.TotalTimeout = 90 * time.Second
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

// convertRequest converts common format to Anthropic format
func (p *Provider) convertRequest(req model.CompletionRequest) anthropicRequest {
	anthropicReq := anthropicRequest{
		Model:       p.model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		System:      req.System,
		Messages:    make([]anthropicMessage, 0),
	}

	// Convert messages
	for _, msg := range req.Messages {
		anthropicMsg := anthropicMessage{
			Role:    msg.Role,
			Content: make([]anthropicContent, 0),
		}

		// Add text content
		if msg.Content != "" {
			anthropicMsg.Content = append(anthropicMsg.Content, anthropicContent{
				Type: "text",
				Text: msg.Content,
			})
		}

		// Add tool result content
		if msg.Role == model.RoleTool {
			anthropicMsg.Role = "user" // Tool results go in user messages
			anthropicMsg.Content = append(anthropicMsg.Content, anthropicContent{
				Type:       "tool_result",
				ToolUseID:  msg.ToolCallID,
				Content:    msg.Content,
			})
		}

		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMsg)
	}

	// Convert tool schemas
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, tool := range req.Tools {
			var schema map[string]interface{}
			json.Unmarshal(tool.InputSchema, &schema)

			anthropicReq.Tools[i] = anthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schema,
			}
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
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
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
type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
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
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
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
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
