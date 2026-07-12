package config

import (
	"fmt"
	"strings"

	"github.com/M4MEET/soulgate/internal/model"
)

// ValidateAPIKey validates API key format for different providers
func ValidateAPIKey(provider, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	switch provider {
	case "openai":
		if !strings.HasPrefix(key, "sk-") && !strings.HasPrefix(key, "sk-proj-") {
			return fmt.Errorf("OpenAI API keys start with 'sk-' or 'sk-proj-'")
		}
		if len(key) < 20 {
			return fmt.Errorf("API key seems too short")
		}
	case "anthropic":
		if !strings.HasPrefix(key, "sk-ant-") {
			return fmt.Errorf("Anthropic API keys start with 'sk-ant-'")
		}
		if len(key) < 20 {
			return fmt.Errorf("API key seems too short")
		}
	case "google":
		// Google uses project-specific keys that vary in format
		if len(key) < 20 {
			return fmt.Errorf("API key seems too short")
		}
	case "groq":
		if !strings.HasPrefix(key, "gsk_") {
			return fmt.Errorf("Groq API keys start with 'gsk_'")
		}
	case "mistral":
		// Mistral keys don't have a specific prefix
		if len(key) < 20 {
			return fmt.Errorf("API key seems too short")
		}
	case "cohere":
		// Cohere keys don't have a specific prefix
		if len(key) < 20 {
			return fmt.Errorf("API key seems too short")
		}
	case "deepseek":
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("DeepSeek API keys start with 'sk-'")
		}
	case "xai":
		if !strings.HasPrefix(key, "xai-") {
			return fmt.Errorf("xAI API keys start with 'xai-'")
		}
	case "openrouter":
		if !strings.HasPrefix(key, "sk-or-") {
			return fmt.Errorf("OpenRouter API keys start with 'sk-or-'")
		}
	case "together":
		// Together AI keys don't have a specific prefix
		if len(key) < 20 {
			return fmt.Errorf("API key seems too short")
		}
	case "perplexity":
		if !strings.HasPrefix(key, "pplx-") {
			return fmt.Errorf("Perplexity API keys start with 'pplx-'")
		}
	case "ollama":
		// Ollama is local, no API key needed
		return nil
	default:
		// Unknown provider, just check length
		if len(key) < 10 {
			return fmt.Errorf("API key seems too short")
		}
	}

	return nil
}

// ModelSelection represents a model choice during setup
type ModelSelection struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string // Optional custom base URL
}

// ApplyModelSelection applies a model selection to the configuration
func ApplyModelSelection(cfg *Config, selection ModelSelection) error {
	// Validate inputs
	if selection.Provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}
	if selection.Model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	// For non-local providers, validate API key
	if selection.Provider != "ollama" && selection.APIKey != "" {
		if err := ValidateAPIKey(selection.Provider, selection.APIKey); err != nil {
			return fmt.Errorf("invalid API key: %w", err)
		}
	}

	// Set the active provider
	cfg.Model.DefaultProvider = selection.Provider

	// Configure the specific provider
	// NOTE: Only OpenAI and Anthropic are currently supported in the config
	// Other providers can be added as the config structure is extended
	switch selection.Provider {
	case "openai":
		cfg.Model.OpenAI.Model = selection.Model
		if selection.APIKey != "" {
			cfg.Model.OpenAI.APIKey = selection.APIKey
		}
		if selection.BaseURL != "" {
			cfg.Model.OpenAI.BaseURL = selection.BaseURL
		}

	case "anthropic":
		cfg.Model.Anthropic.Model = selection.Model
		if selection.APIKey != "" {
			cfg.Model.Anthropic.APIKey = selection.APIKey
		}
		if selection.BaseURL != "" {
			cfg.Model.Anthropic.BaseURL = selection.BaseURL
		}

	default:
		// For other providers, we can only set the default provider
		// The API keys will need to be set via environment variables
		// This allows the onboarding to support all providers even if
		// the config struct doesn't have fields for them yet
		return nil
	}

	return nil
}

// GetModelOptions returns quick-start model options for onboarding
// These are simplified presets combining provider + model for easy selection
func GetModelOptions() []ModelOptionPreset {
	return []ModelOptionPreset{
		{
			ID:          "gpt-5.2",
			Name:        "GPT-5.2",
			Provider:    "openai",
			Model:       "gpt-5.2",
			Description: "Latest OpenAI flagship - Complex coding & analysis",
			Icon:        "🧠",
			Recommended: true,
		},
		{
			ID:          "gpt-5-mini",
			Name:        "GPT-5-mini",
			Provider:    "openai",
			Model:       "gpt-5-mini",
			Description: "Fast & economical - Simple tasks & quick responses",
			Icon:        "⚡",
			Recommended: false,
		},
		{
			ID:          "claude-sonnet-5",
			Name:        "Claude Sonnet 5",
			Provider:    "anthropic",
			Model:       "claude-sonnet-5",
			Description: "Balanced Anthropic model - Great for most tasks",
			Icon:        "🎭",
			Recommended: true,
		},
		{
			ID:          "claude-opus-4-8",
			Name:        "Claude Opus 4.8",
			Provider:    "anthropic",
			Model:       "claude-opus-4-8",
			Description: "Most capable Anthropic model - Deep reasoning",
			Icon:        "🎪",
			Recommended: false,
		},
		{
			ID:          "gemini-2.5-pro",
			Name:        "Gemini 2.5 Pro",
			Provider:    "google",
			Model:       "gemini-2.5-pro",
			Description: "Google's most powerful - Multimodal understanding",
			Icon:        "🔮",
			Recommended: false,
		},
		{
			ID:          "llama-3.3-70b",
			Name:        "Llama 3.3 70B (Groq)",
			Provider:    "groq",
			Model:       "llama-3.3-70b-versatile",
			Description: "Lightning fast inference - Open source",
			Icon:        "🦙",
			Recommended: false,
		},
		{
			ID:          "ollama-local",
			Name:        "Ollama (Local)",
			Provider:    "ollama",
			Model:       "llama3.2",
			Description: "Run locally - Complete privacy & no API costs",
			Icon:        "🏠",
			Recommended: false,
		},
	}
}

// ModelOptionPreset represents a quick-start model preset
type ModelOptionPreset struct {
	ID          string // Unique identifier for the preset
	Name        string // Display name
	Provider    string // Provider name (openai, anthropic, etc.)
	Model       string // Specific model ID
	Description string // Description for users
	Icon        string // Icon for display
	Recommended bool   // Whether this is a recommended option
}

// GetProviderDisplayName returns a user-friendly provider name
func GetProviderDisplayName(provider string) string {
	switch provider {
	case "openai":
		return "OpenAI"
	case "anthropic":
		return "Anthropic"
	case "google":
		return "Google (Gemini)"
	case "groq":
		return "Groq"
	case "mistral":
		return "Mistral AI"
	case "cohere":
		return "Cohere"
	case "deepseek":
		return "DeepSeek"
	case "xai":
		return "xAI (Grok)"
	case "openrouter":
		return "OpenRouter"
	case "together":
		return "Together AI"
	case "perplexity":
		return "Perplexity"
	case "ollama":
		return "Ollama (Local)"
	default:
		return provider
	}
}

// GetProviderAPIKeyInstructions returns instructions for obtaining an API key
func GetProviderAPIKeyInstructions(provider string) string {
	switch provider {
	case "openai":
		return "Get your API key from: https://platform.openai.com/api-keys"
	case "anthropic":
		return "Get your API key from: https://console.anthropic.com/settings/keys"
	case "google":
		return "Get your API key from: https://makersuite.google.com/app/apikey"
	case "groq":
		return "Get your API key from: https://console.groq.com/keys"
	case "mistral":
		return "Get your API key from: https://console.mistral.ai/api-keys"
	case "cohere":
		return "Get your API key from: https://dashboard.cohere.com/api-keys"
	case "deepseek":
		return "Get your API key from: https://platform.deepseek.com/api_keys"
	case "xai":
		return "Get your API key from: https://console.x.ai"
	case "openrouter":
		return "Get your API key from: https://openrouter.ai/keys"
	case "together":
		return "Get your API key from: https://api.together.xyz/settings/api-keys"
	case "perplexity":
		return "Get your API key from: https://www.perplexity.ai/settings/api"
	case "ollama":
		return "Ollama runs locally. Install from: https://ollama.ai"
	default:
		return "Refer to your provider's documentation for API key instructions"
	}
}

// GetProviderModels returns available models for a provider
// This is a convenience wrapper around model.BuildModelOptionsForProvider
func GetProviderModels(provider string) []model.ModelOption {
	return model.BuildModelOptionsForProvider(provider)
}
