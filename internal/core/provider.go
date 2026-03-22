package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
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
