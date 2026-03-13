package model

// This file contains model lists for each provider
// Used by the two-step model selector

// BuildModelOptionsForProvider returns models for a specific provider
func BuildModelOptionsForProvider(provider string) []ModelOption {
	switch provider {
	case "openai":
		return []ModelOption{
			{ID: "gpt-4.1", Name: "GPT-4.1", Description: "Latest flagship - Complex coding & analysis"},
			{ID: "gpt-4.1-mini", Name: "GPT-4.1-mini", Description: "Fast & economical - Balanced tasks"},
			{ID: "gpt-4.1-nano", Name: "GPT-4.1-nano", Description: "Fastest & cheapest - Simple tasks"},
			{ID: "o3", Name: "o3", Description: "Deep reasoning - Complex problem-solving"},
			{ID: "o4-mini", Name: "o4-mini", Description: "Efficient reasoning - Cost-effective"},
		}
	case "anthropic":
		return []ModelOption{
			{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Description: "Deep reasoning & analysis"},
			{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Description: "Balanced - Great for most tasks"},
			{ID: "claude-haiku-4-20250501", Name: "Claude Haiku 4", Description: "Fast & efficient - Quick responses"},
		}
	case "groq":
		return []ModelOption{
			{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Description: "Meta's latest - Lightning fast inference"},
			{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Description: "Mistral MoE - Large context window"},
			{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Description: "Google's efficient model - Fast"},
		}
	case "google":
		return []ModelOption{
			// Gemini 3 Series (Latest)
			{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Description: "Latest - Complex reasoning & coding (1M context)"},
			{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash Preview", Description: "Latest - Fast multimodal understanding"},
			{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image", Description: "High-fidelity image generation"},
			// Gemini 2.5 Series (Current Stable)
			{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Description: "Most powerful stable - Complex tasks"},
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Description: "Fast & efficient - Real-time capable"},
			{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Description: "Lightweight - Quick responses"},
			{ID: "gemini-2.5-flash-image", Name: "Gemini 2.5 Flash Image", Description: "Image generation"},
			// Gemini 2.0 Series (Deprecated - shutting down March 31, 2026)
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash (Deprecated)", Description: "⚠️ Shutting down March 31, 2026"},
		}
	case "mistral":
		return []ModelOption{
			{ID: "mistral-large-latest", Name: "Mistral Large", Description: "Most capable - Complex reasoning"},
			{ID: "mistral-medium-latest", Name: "Mistral Medium", Description: "Balanced - Good for most tasks"},
			{ID: "mistral-small-latest", Name: "Mistral Small", Description: "Fast & economical - Simple tasks"},
		}
	case "cohere":
		return []ModelOption{
			{ID: "command-r-plus", Name: "Command R+", Description: "Most capable - RAG & tools"},
			{ID: "command-r", Name: "Command R", Description: "Balanced performance"},
		}
	case "deepseek":
		return []ModelOption{
			{ID: "deepseek-chat", Name: "DeepSeek V3", Description: "General chat - Strong coding ability"},
			{ID: "deepseek-coder", Name: "DeepSeek Coder", Description: "Specialized for code generation"},
		}
	case "openrouter":
		return []ModelOption{
			{ID: "openrouter/auto", Name: "Auto", Description: "Best available model - Auto-selected"},
			{ID: "openai/gpt-4.1", Name: "GPT-4.1 (OpenRouter)", Description: "OpenAI via OpenRouter - Fallback routing"},
			{ID: "anthropic/claude-opus-4", Name: "Claude Opus 4 (OpenRouter)", Description: "Claude via OpenRouter - Fallback routing"},
		}
	case "together":
		return []ModelOption{
			{ID: "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo", Name: "Llama 3.1 405B", Description: "Largest open model - Powerful"},
			{ID: "Qwen/Qwen2.5-72B-Instruct-Turbo", Name: "Qwen 2.5 72B", Description: "Strong multilingual"},
		}
	case "perplexity":
		return []ModelOption{
			{ID: "sonar", Name: "Perplexity Sonar", Description: "Real-time web search - Current info"},
			{ID: "sonar-pro", Name: "Perplexity Sonar Pro", Description: "Advanced search - Deep research"},
		}
	case "ollama":
		return []ModelOption{
			{ID: "llama3.2", Name: "Llama 3.2", Description: "Local - Privacy & no API costs"},
			{ID: "mistral", Name: "Mistral", Description: "Local - Fast inference"},
			{ID: "codellama", Name: "CodeLlama", Description: "Local - Code generation"},
		}
	case "xai":
		return []ModelOption{
			{ID: "grok-4-1-fast-reasoning", Name: "Grok 4.1 Fast (Reasoning)", Description: "Latest - Deep reasoning (2M context)"},
			{ID: "grok-4-1-fast-non-reasoning", Name: "Grok 4.1 Fast", Description: "Latest - Faster responses (2M context)"},
			{ID: "grok-4-fast-reasoning", Name: "Grok 4 Fast (Reasoning)", Description: "Grok 4 with reasoning (2M context)"},
			{ID: "grok-3", Name: "Grok 3", Description: "Most powerful - Complex tasks (131K context)"},
			{ID: "grok-3-mini", Name: "Grok 3 Mini", Description: "Fast & economical (131K context)"},
			{ID: "grok-code-fast-1", Name: "Grok Code Fast", Description: "Code specialist (256K context)"},
			{ID: "grok-2-vision-1212", Name: "Grok 2 Vision", Description: "Multimodal - Image understanding (32K context)"},
		}
	default:
		return []ModelOption{}
	}
}

// ModelOption represents a model option
type ModelOption struct {
	ID          string
	Name        string
	Description string
}
