package core

import (
	"fmt"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
)

// SetProvider dynamically switches the model provider
func (o *Orchestrator) SetProvider(providerName string, modelName string) error {
	def, err := model.LookupProvider(providerName)
	if err != nil {
		return fmt.Errorf("unsupported provider: %s\n\nSupported providers:\n  %s\n\nSet the appropriate API key environment variable",
			providerName, formatProviderList())
	}

	// Resolve API key
	apiKey, err := model.ResolveAPIKey(def)
	if err != nil {
		return err
	}

	// Use default model if none specified
	if modelName == "" {
		modelName = def.DefaultModel
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

// GetCurrentProvider returns the current provider name and model
func (o *Orchestrator) GetCurrentProvider() (string, string) {
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
