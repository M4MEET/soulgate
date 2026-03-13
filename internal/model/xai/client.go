package xai

import (
	"context"
	"strings"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/openai"
)

// Provider implements the xAI (Grok) provider
// xAI uses OpenAI-compatible API, so we wrap the OpenAI client
type Provider struct {
	openaiProvider *openai.Provider
	modelName      string
}

// NewProvider creates a new xAI provider
func NewProvider(apiKey, modelName string) *Provider {
	if modelName == "" {
		modelName = "grok-beta"
	}

	// xAI uses OpenAI-compatible API at https://api.x.ai/v1
	baseURL := "https://api.x.ai/v1"

	return &Provider{
		openaiProvider: openai.NewProvider(apiKey, modelName, baseURL),
		modelName:      modelName,
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "xai"
}

// SupportedFeatures returns the features supported by xAI
func (p *Provider) SupportedFeatures() model.FeatureSet {
	return model.FeatureSet{
		ToolCalling:     true,  // Grok supports function calling
		VisionSupport:   false, // Not yet available
		SystemMessages:  true,
		StreamingOutput: true,
	}
}

// Complete sends a completion request to xAI
func (p *Provider) Complete(ctx context.Context, req model.CompletionRequest) (*model.CompletionResponse, error) {
	return p.openaiProvider.Complete(ctx, req)
}

// StreamComplete streams a completion response from xAI
func (p *Provider) StreamComplete(ctx context.Context, req model.CompletionRequest) (<-chan model.StreamChunk, error) {
	return p.openaiProvider.StreamComplete(ctx, req)
}

// GetModelName returns the current model name
func (p *Provider) GetModelName() string {
	return p.modelName
}

// IsGrokModel checks if a model name is a Grok model
func IsGrokModel(modelName string) bool {
	modelLower := strings.ToLower(modelName)
	return strings.HasPrefix(modelLower, "grok")
}
