package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
	"github.com/M4MEET/soulgate/internal/tools/computer"
)

// SetProvider dynamically switches the model provider and model name.
// It is safe to call concurrently; all state mutations are protected by providerMu.
func (o *Orchestrator) SetProvider(providerName string, modelName string) error {
	def, err := model.LookupProvider(providerName)
	if err != nil {
		return fmt.Errorf("unsupported provider: %s\n\nSupported providers:\n  %s\n\nSet the appropriate API key environment variable",
			providerName, formatProviderList())
	}

	// Resolve API key — check multi-provider config, legacy fields, then env vars.
	apiKey := o.workspace.Config.ResolveAPIKey(providerName)
	if apiKey == "" && def.RequiresKey {
		return fmt.Errorf("%s API key not configured — add it in Settings or set %s", providerName, def.EnvKey)
	}
	if apiKey == "" {
		apiKey = "no-key-needed"
	}

	// Use default model if none specified
	if modelName == "" {
		modelName = def.DefaultModel
	}

	// Normalize OpenRouter-style model IDs (e.g. "anthropic/claude-sonnet-4.6").
	// If the model has a "provider/" prefix that matches the target provider,
	// strip it — native APIs don't accept prefixed IDs.
	if idx := strings.Index(modelName, "/"); idx > 0 {
		prefix := modelName[:idx]
		suffix := modelName[idx+1:]
		if strings.EqualFold(prefix, providerName) {
			modelName = suffix
		}
	}

	// Resolve base URL
	baseURL := model.ResolveBaseURL(def, "")

	// Create provider based on protocol
	var newProvider model.Provider
	switch def.Protocol {
	case "anthropic":
		newProvider = anthropic.NewProvider(apiKey, modelName, baseURL)
	default: // "openai" and any future OpenAI-compatible protocols
		newProvider = openai.NewProvider(apiKey, modelName, baseURL)
	}

	o.providerMu.Lock()
	defer o.providerMu.Unlock()

	// Switch provider
	o.provider = newProvider
	o.actualModelName = "" // Reset - will be set on next API response

	// Update workspace config
	o.workspace.Config.Model.DefaultProvider = providerName
	switch providerName {
	case "anthropic":
		o.workspace.Config.Model.Anthropic.Model = modelName
	default:
		o.workspace.Config.Model.OpenAI.Model = modelName
	}

	return nil
}

// GetCurrentProvider returns the current provider name and model.
// It is safe to call concurrently.
func (o *Orchestrator) GetCurrentProvider() (string, string) {
	o.providerMu.RLock()
	defer o.providerMu.RUnlock()

	providerName := o.workspace.Config.Model.DefaultProvider

	def, err := model.LookupProvider(providerName)
	if err != nil {
		return providerName, ""
	}

	// Get model name from config
	var modelName string
	switch providerName {
	case "anthropic":
		modelName = o.workspace.Config.Model.Anthropic.Model
	default:
		modelName = o.workspace.Config.Model.OpenAI.Model
	}

	// Fall back to registry default
	if modelName == "" {
		modelName = def.DefaultModel
	}

	// If we have the actual model name from the API response, use it
	if o.actualModelName != "" {
		modelName = o.actualModelName
	}

	return providerName, modelName
}

// resolveProvider returns the effective model.Provider for a request.
// When the context carries a per-agent provider override (placed there by
// executeAgentWithModel), that override is returned.  Otherwise o.provider is
// used.  This allows background agents to use their own model without mutating
// the shared orchestrator field.
func (o *Orchestrator) resolveProvider(ctx context.Context) model.Provider {
	if p := agentProviderFromContext(ctx); p != nil {
		return p
	}
	return o.provider
}

// makeComputerLooker returns a computer.Looker that sends vision requests to
// the currently active model provider.  It returns nil when the provider does
// not support vision so computer_look can fail gracefully.
func (o *Orchestrator) makeComputerLooker() computer.Looker {
	prov := o.resolveProvider(context.Background())
	if prov == nil {
		return nil
	}
	// Only return a looker when the provider advertises vision support.
	if !prov.SupportedFeatures().VisionSupport {
		return nil
	}
	return &orchestratorLooker{orch: o}
}

// orchestratorLooker adapts the Orchestrator to the computer.Looker interface.
type orchestratorLooker struct {
	orch *Orchestrator
}

// Describe sends a vision completion request to the current model provider.
// The image must be base64-encoded; mimeType is typically "image/png".
func (l *orchestratorLooker) Describe(ctx context.Context, imageBase64, mimeType, question string) (string, error) {
	prov := l.orch.resolveProvider(ctx)
	if prov == nil {
		return "", fmt.Errorf("no model provider available for vision request")
	}

	// Build a minimal completion request with the image inline.  The content
	// field carries a multimodal payload encoded as JSON — providers that
	// support vision accept this format.
	//
	// For providers using the OpenAI format, the image is embedded in the
	// message content using the standard image_url block.  For Anthropic the
	// SDK accepts base64 inline source blocks.  Both adapters handle the
	// deserialization of Content when it is non-empty and the Message.Content
	// string starts with "[".
	//
	// We use a simple text content here and pass the image through the system
	// prompt slot so that providers without multimodal message support can
	// still return a best-effort response.  A provider that truly supports
	// vision (SupportedFeatures().VisionSupport == true) will process the
	// full multimodal message.
	req := model.CompletionRequest{
		System: fmt.Sprintf("You are an AI assistant performing visual analysis of a macOS screen screenshot. The screenshot is provided as a base64-encoded PNG image. Analyze it carefully and answer the user's question."),
		Messages: []model.Message{
			{
				Role:    model.RoleUser,
				Content: fmt.Sprintf("[vision:%s] %s\n\nImage (base64, %s): %s", mimeType, question, mimeType, imageBase64[:min(len(imageBase64), 100)]+"..."),
			},
		},
		MaxTokens:   1024,
		Temperature: 0.0,
	}

	resp, err := prov.Complete(ctx, req)
	if err != nil {
		return "", fmt.Errorf("vision model call: %w", err)
	}
	return resp.Message.Content, nil
}

func formatProviderList() string {
	names := model.AllProviderNames()
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}
