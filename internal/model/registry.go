package model

import (
	"fmt"
	"os"
)

// ProviderDef defines a model provider's configuration and behavior
type ProviderDef struct {
	// Name is the canonical provider name (e.g., "openai", "anthropic")
	Name string

	// Protocol is "openai" or "anthropic" — determines which client adapter to use
	Protocol string

	// BaseURL is the API base URL (empty = use adapter default)
	BaseURL string

	// EnvKey is the environment variable name for the API key
	EnvKey string

	// RequiresKey indicates whether an API key is required
	RequiresKey bool

	// DefaultModel is the model ID to use when none is specified
	DefaultModel string

	// Models is the list of available models for this provider
	Models []ModelOption
}

// Registry holds all known provider definitions
var Registry = map[string]ProviderDef{
	"openai": {
		Name:         "openai",
		Protocol:     "openai",
		EnvKey:       "OPENAI_API_KEY",
		RequiresKey:  true,
		DefaultModel: "gpt-4.1-mini",
		Models: []ModelOption{
			{ID: "gpt-4.1", Name: "GPT-4.1", Description: "Latest flagship - Complex coding & analysis"},
			{ID: "gpt-4.1-mini", Name: "GPT-4.1-mini", Description: "Fast & economical - Balanced tasks"},
			{ID: "gpt-4.1-nano", Name: "GPT-4.1-nano", Description: "Fastest & cheapest - Simple tasks"},
			{ID: "o3", Name: "o3", Description: "Deep reasoning - Complex problem-solving"},
			{ID: "o4-mini", Name: "o4-mini", Description: "Efficient reasoning - Cost-effective"},
		},
	},
	"anthropic": {
		Name:         "anthropic",
		Protocol:     "anthropic",
		EnvKey:       "ANTHROPIC_API_KEY",
		RequiresKey:  true,
		DefaultModel: "claude-sonnet-4-20250514",
		Models: []ModelOption{
			{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Description: "Deep reasoning & analysis"},
			{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Description: "Balanced - Great for most tasks"},
			{ID: "claude-haiku-4-20250501", Name: "Claude Haiku 4", Description: "Fast & efficient - Quick responses"},
		},
	},
	"groq": {
		Name:         "groq",
		Protocol:     "openai",
		BaseURL:      "https://api.groq.com/openai/v1",
		EnvKey:       "GROQ_API_KEY",
		RequiresKey:  true,
		DefaultModel: "llama-3.3-70b-versatile",
		Models: []ModelOption{
			{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Description: "Meta's latest - Lightning fast inference"},
			{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Description: "Mistral MoE - Large context window"},
			{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Description: "Google's efficient model - Fast"},
		},
	},
	"google": {
		Name:         "google",
		Protocol:     "openai",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/openai",
		EnvKey:       "GOOGLE_API_KEY",
		RequiresKey:  true,
		DefaultModel: "gemini-2.5-flash",
		Models: []ModelOption{
			{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Description: "Latest - Complex reasoning & coding (1M context)"},
			{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash Preview", Description: "Latest - Fast multimodal understanding"},
			{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image", Description: "High-fidelity image generation"},
			{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Description: "Most powerful stable - Complex tasks"},
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Description: "Fast & efficient - Real-time capable"},
			{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Description: "Lightweight - Quick responses"},
			{ID: "gemini-2.5-flash-image", Name: "Gemini 2.5 Flash Image", Description: "Image generation"},
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash (Deprecated)", Description: "Shutting down March 31, 2026"},
		},
	},
	"mistral": {
		Name:         "mistral",
		Protocol:     "openai",
		BaseURL:      "https://api.mistral.ai/v1",
		EnvKey:       "MISTRAL_API_KEY",
		RequiresKey:  true,
		DefaultModel: "mistral-large-latest",
		Models: []ModelOption{
			{ID: "mistral-large-latest", Name: "Mistral Large", Description: "Most capable - Complex reasoning"},
			{ID: "mistral-medium-latest", Name: "Mistral Medium", Description: "Balanced - Good for most tasks"},
			{ID: "mistral-small-latest", Name: "Mistral Small", Description: "Fast & economical - Simple tasks"},
		},
	},
	"cohere": {
		Name:         "cohere",
		Protocol:     "openai",
		BaseURL:      "https://api.cohere.ai/v1",
		EnvKey:       "COHERE_API_KEY",
		RequiresKey:  true,
		DefaultModel: "command-r-plus",
		Models: []ModelOption{
			{ID: "command-r-plus", Name: "Command R+", Description: "Most capable - RAG & tools"},
			{ID: "command-r", Name: "Command R", Description: "Balanced performance"},
		},
	},
	"deepseek": {
		Name:         "deepseek",
		Protocol:     "openai",
		BaseURL:      "https://api.deepseek.com/v1",
		EnvKey:       "DEEPSEEK_API_KEY",
		RequiresKey:  true,
		DefaultModel: "deepseek-chat",
		Models: []ModelOption{
			{ID: "deepseek-chat", Name: "DeepSeek V3", Description: "General chat - Strong coding ability"},
			{ID: "deepseek-coder", Name: "DeepSeek Coder", Description: "Specialized for code generation"},
		},
	},
	"openrouter": {
		Name:         "openrouter",
		Protocol:     "openai",
		BaseURL:      "https://openrouter.ai/api/v1",
		EnvKey:       "OPENROUTER_API_KEY",
		RequiresKey:  true,
		DefaultModel: "openrouter/auto",
		Models: []ModelOption{
			{ID: "openrouter/auto", Name: "Auto", Description: "Best available model - Auto-selected"},
			{ID: "openai/gpt-4.1", Name: "GPT-4.1 (OpenRouter)", Description: "OpenAI via OpenRouter - Fallback routing"},
			{ID: "anthropic/claude-opus-4", Name: "Claude Opus 4 (OpenRouter)", Description: "Claude via OpenRouter - Fallback routing"},
		},
	},
	"together": {
		Name:         "together",
		Protocol:     "openai",
		BaseURL:      "https://api.together.xyz/v1",
		EnvKey:       "TOGETHER_API_KEY",
		RequiresKey:  true,
		DefaultModel: "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo",
		Models: []ModelOption{
			{ID: "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo", Name: "Llama 3.1 405B", Description: "Largest open model - Powerful"},
			{ID: "Qwen/Qwen2.5-72B-Instruct-Turbo", Name: "Qwen 2.5 72B", Description: "Strong multilingual"},
		},
	},
	"perplexity": {
		Name:         "perplexity",
		Protocol:     "openai",
		BaseURL:      "https://api.perplexity.ai",
		EnvKey:       "PERPLEXITY_API_KEY",
		RequiresKey:  true,
		DefaultModel: "sonar",
		Models: []ModelOption{
			{ID: "sonar", Name: "Perplexity Sonar", Description: "Real-time web search - Current info"},
			{ID: "sonar-pro", Name: "Perplexity Sonar Pro", Description: "Advanced search - Deep research"},
		},
	},
	"xai": {
		Name:         "xai",
		Protocol:     "openai",
		BaseURL:      "https://api.x.ai/v1",
		EnvKey:       "XAI_API_KEY",
		RequiresKey:  true,
		DefaultModel: "grok-4-1-fast-reasoning",
		Models: []ModelOption{
			{ID: "grok-4-1-fast-reasoning", Name: "Grok 4.1 Fast (Reasoning)", Description: "Latest - Deep reasoning (2M context)"},
			{ID: "grok-4-1-fast-non-reasoning", Name: "Grok 4.1 Fast", Description: "Latest - Faster responses (2M context)"},
			{ID: "grok-4-fast-reasoning", Name: "Grok 4 Fast (Reasoning)", Description: "Grok 4 with reasoning (2M context)"},
			{ID: "grok-3", Name: "Grok 3", Description: "Most powerful - Complex tasks (131K context)"},
			{ID: "grok-3-mini", Name: "Grok 3 Mini", Description: "Fast & economical (131K context)"},
			{ID: "grok-code-fast-1", Name: "Grok Code Fast", Description: "Code specialist (256K context)"},
			{ID: "grok-2-vision-1212", Name: "Grok 2 Vision", Description: "Multimodal - Image understanding (32K context)"},
		},
	},
	"ollama": {
		Name:         "ollama",
		Protocol:     "openai",
		BaseURL:      "http://localhost:11434/v1",
		EnvKey:       "",
		RequiresKey:  false,
		DefaultModel: "llama3.2",
		Models: []ModelOption{
			{ID: "llama3.2", Name: "Llama 3.2", Description: "Local - Privacy & no API costs"},
			{ID: "mistral", Name: "Mistral", Description: "Local - Fast inference"},
			{ID: "codellama", Name: "CodeLlama", Description: "Local - Code generation"},
		},
	},
	"azure": {
		Name:         "azure",
		Protocol:     "openai",
		EnvKey:       "AZURE_OPENAI_API_KEY",
		RequiresKey:  true,
		DefaultModel: "gpt-4.1",
		Models: []ModelOption{
			{ID: "gpt-4.1", Name: "GPT-4.1", Description: "Azure OpenAI"},
		},
	},
}

// LookupProvider returns the provider definition for the given name.
// Returns an error if the provider is not found.
func LookupProvider(name string) (ProviderDef, error) {
	def, ok := Registry[name]
	if !ok {
		return ProviderDef{}, fmt.Errorf("unknown provider: %s", name)
	}
	return def, nil
}

// ResolveAPIKey returns the API key for a provider from env vars.
// Returns ("", nil) for providers that don't require a key (e.g., ollama).
// For Azure, also checks AZURE_OPENAI_ENDPOINT.
func ResolveAPIKey(def ProviderDef) (string, error) {
	if !def.RequiresKey {
		return "no-key-needed", nil
	}

	apiKey := os.Getenv(def.EnvKey)
	if apiKey == "" {
		return "", fmt.Errorf("%s environment variable not set", def.EnvKey)
	}
	return apiKey, nil
}

// ResolveBaseURL returns the base URL for a provider.
// For Azure, uses the AZURE_OPENAI_ENDPOINT env var.
// baseURLOverride takes precedence if non-empty.
func ResolveBaseURL(def ProviderDef, baseURLOverride string) string {
	if baseURLOverride != "" {
		return baseURLOverride
	}
	if def.Name == "azure" {
		if endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT"); endpoint != "" {
			return endpoint
		}
	}
	return def.BaseURL
}

// AllProviderNames returns all registered provider names in display order.
func AllProviderNames() []string {
	return []string{
		"openai", "anthropic",
		"groq", "google", "mistral", "cohere", "deepseek",
		"openrouter", "together", "perplexity", "xai",
		"ollama", "azure",
	}
}
