package model

import "context"

// Provider defines the interface for LLM providers
type Provider interface {
	// Complete sends a completion request and returns the response
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// StreamComplete sends a completion request and streams the response chunks.
	// The channel is closed when the response is complete.
	// If streaming is not supported, falls back to Complete and sends a single chunk.
	StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)

	// Name returns the provider name
	Name() string

	// SupportedFeatures returns the features supported by this provider
	SupportedFeatures() FeatureSet
}

// StreamChunk represents a single chunk of a streamed response
type StreamChunk struct {
	// Delta is the new text content in this chunk
	Delta string
	// ToolCall is set when the model is calling a tool (accumulated)
	ToolCall *ToolCall
	// Done indicates this is the final chunk
	Done bool
	// Response is set on the final chunk with the complete response
	Response *CompletionResponse
	// Error is set if streaming encountered an error
	Error error
}

// FeatureSet represents features supported by a provider
type FeatureSet struct {
	ToolCalling     bool
	VisionSupport   bool
	SystemMessages  bool
	StreamingOutput bool
}
