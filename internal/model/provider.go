package model

import "context"

// Provider defines the interface for LLM providers
type Provider interface {
	// Complete sends a completion request and returns the response
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Name returns the provider name
	Name() string

	// SupportedFeatures returns the features supported by this provider
	SupportedFeatures() FeatureSet
}

// FeatureSet represents features supported by a provider
type FeatureSet struct {
	ToolCalling     bool
	VisionSupport   bool
	SystemMessages  bool
	StreamingOutput bool
}
