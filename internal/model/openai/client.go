package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/httpclient"
	"github.com/M4MEET/soulgate/internal/model"
)

// Provider implements the OpenAI provider
type Provider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewProvider creates a new OpenAI provider with secure HTTP client
func NewProvider(apiKey, modelName, baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if modelName == "" {
		modelName = "gpt-4"
	}

	// Create secure HTTP client
	secureConfig := httpclient.DefaultSecureConfig()
	secureConfig.TotalTimeout = 90 * time.Second // Longer for large responses
	secureConfig.UserAgent = "SoulGate-OpenAI/0.1"

	return &Provider{
		apiKey:  apiKey,
		model:   modelName,
		baseURL: baseURL,
		client:  httpclient.NewSecureClient(secureConfig),
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "openai"
}

// SupportedFeatures returns the features supported by OpenAI
func (p *Provider) SupportedFeatures() model.FeatureSet {
	return model.FeatureSet{
		ToolCalling:     true,
		VisionSupport:   true,
		SystemMessages:  true,
		StreamingOutput: true,
	}
}

// usesMaxCompletionTokens determines if the model uses max_completion_tokens instead of max_tokens
func (p *Provider) usesMaxCompletionTokens() bool {
	modelLower := strings.ToLower(p.model)

	// GPT-5 and newer models use max_completion_tokens
	if strings.HasPrefix(modelLower, "gpt-5") {
		return true
	}

	// GPT-4o models from 2024-08-06 onwards use max_completion_tokens
	if strings.HasPrefix(modelLower, "gpt-4o-2024-08") ||
	   strings.HasPrefix(modelLower, "gpt-4o-2024-09") ||
	   strings.HasPrefix(modelLower, "gpt-4o-2024-10") ||
	   strings.HasPrefix(modelLower, "gpt-4o-2024-11") ||
	   strings.HasPrefix(modelLower, "gpt-4o-2024-12") ||
	   strings.HasPrefix(modelLower, "gpt-4o-2025") ||
	   strings.HasPrefix(modelLower, "gpt-4o-2026") {
		return true
	}

	// o1 models use max_completion_tokens
	if strings.HasPrefix(modelLower, "o1-") || strings.HasPrefix(modelLower, "o1") {
		return true
	}

	// Default to old parameter for backwards compatibility
	return false
}

// Complete sends a completion request to OpenAI
func (p *Provider) Complete(ctx context.Context, req model.CompletionRequest) (*model.CompletionResponse, error) {
	// Convert to OpenAI format
	openAIReq := p.convertRequest(req)

	// Marshal request
	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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
	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to common format
	return p.convertResponse(openAIResp), nil
}

// convertRequest converts common format to OpenAI format
func (p *Provider) convertRequest(req model.CompletionRequest) openAIRequest {
	openAIReq := openAIRequest{
		Model:       p.model,
		Messages:    make([]openAIMessage, 0),
		Temperature: req.Temperature,
	}

	// Use appropriate token limit parameter based on model
	// Newer models (GPT-5+, GPT-4o-2024-08-06+) require max_completion_tokens
	// Older models use max_tokens
	if p.usesMaxCompletionTokens() {
		openAIReq.MaxCompletionTokens = req.MaxTokens
	} else {
		openAIReq.MaxTokens = req.MaxTokens
	}

	// Add system message if present
	if req.System != "" {
		openAIReq.Messages = append(openAIReq.Messages, openAIMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	// Convert messages
	for _, msg := range req.Messages {
		openAIMsg := openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Handle tool call results
		if msg.Role == model.RoleTool {
			openAIMsg.ToolCallID = msg.ToolCallID
			openAIMsg.Name = msg.Name
		}

		// Handle assistant messages with tool calls
		if msg.Role == model.RoleAssistant && len(msg.ToolCalls) > 0 {
			openAIMsg.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				openAIMsg.ToolCalls[i] = openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      tc.Name,
						Arguments: string(tc.Input),
					},
				}
			}
		}

		openAIReq.Messages = append(openAIReq.Messages, openAIMsg)
	}

	// Convert tool schemas
	if len(req.Tools) > 0 {
		openAIReq.Tools = make([]openAITool, len(req.Tools))
		for i, tool := range req.Tools {
			var schema map[string]interface{}
			json.Unmarshal(tool.InputSchema, &schema)

			openAIReq.Tools[i] = openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  schema,
				},
			}
		}
	}

	return openAIReq
}

// convertResponse converts OpenAI format to common format
func (p *Provider) convertResponse(resp openAIResponse) *model.CompletionResponse {
	if len(resp.Choices) == 0 {
		return &model.CompletionResponse{
			Message: model.Message{
				Role:    model.RoleAssistant,
				Content: "",
			},
			StopReason: model.StopReasonError,
		}
	}

	choice := resp.Choices[0]

	result := &model.CompletionResponse{
		Message: model.Message{
			Role:    choice.Message.Role,
			Content: choice.Message.Content,
		},
		Usage: model.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Convert stop reason
	switch choice.FinishReason {
	case "stop":
		result.StopReason = model.StopReasonEndTurn
	case "tool_calls":
		result.StopReason = model.StopReasonToolUse
	case "length":
		result.StopReason = model.StopReasonMaxTokens
	default:
		result.StopReason = choice.FinishReason
	}

	// Convert tool calls
	if len(choice.Message.ToolCalls) > 0 {
		result.ToolCalls = make([]model.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			result.ToolCalls[i] = model.ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			}
		}
	}

	return result
}

// OpenAI API types
type openAIRequest struct {
	Model                string          `json:"model"`
	Messages             []openAIMessage `json:"messages"`
	Tools                []openAITool    `json:"tools,omitempty"`
	MaxTokens            int             `json:"max_tokens,omitempty"`             // For older models
	MaxCompletionTokens  int             `json:"max_completion_tokens,omitempty"`  // For newer models (GPT-5+, GPT-4o-2024-08-06+)
	Temperature          float64         `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role       string            `json:"role"`
	Content    string            `json:"content"`
	ToolCalls  []openAIToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	Name       string            `json:"name,omitempty"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function openAIFunctionCall   `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []openAIChoice  `json:"choices"`
	Usage   openAIUsage     `json:"usage"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
