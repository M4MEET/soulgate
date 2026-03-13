package core

import (
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
)

// SetProvider dynamically switches the model provider
func (o *Orchestrator) SetProvider(providerName string, modelName string) error {
	var newProvider model.Provider

	switch providerName {
	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "gpt-4.1-mini"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "")

	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "claude-sonnet-4-20250514"
		}
		newProvider = anthropic.NewProvider(apiKey, modelName, "")

	case "groq":
		apiKey := os.Getenv("GROQ_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("GROQ_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "llama-3.3-70b-versatile"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.groq.com/openai/v1")

	case "google":
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("GOOGLE_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "gemini-2.5-flash"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://generativelanguage.googleapis.com/v1beta/openai")

	case "mistral":
		apiKey := os.Getenv("MISTRAL_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("MISTRAL_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "mistral-large-latest"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.mistral.ai/v1")

	case "cohere":
		apiKey := os.Getenv("COHERE_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("COHERE_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "command-r-plus"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.cohere.ai/v1")

	case "deepseek":
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("DEEPSEEK_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "deepseek-chat"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.deepseek.com/v1")

	case "openrouter":
		apiKey := os.Getenv("OPENROUTER_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "openrouter/auto"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://openrouter.ai/api/v1")

	case "together":
		apiKey := os.Getenv("TOGETHER_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("TOGETHER_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.together.xyz/v1")

	case "perplexity":
		apiKey := os.Getenv("PERPLEXITY_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("PERPLEXITY_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "sonar"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.perplexity.ai")

	case "xai":
		apiKey := os.Getenv("XAI_API_KEY")
		if apiKey == "" {
			return fmt.Errorf("XAI_API_KEY environment variable not set")
		}
		if modelName == "" {
			modelName = "grok-4-1-fast-reasoning"
		}
		newProvider = openai.NewProvider(apiKey, modelName, "https://api.x.ai/v1")

	case "ollama":
		if modelName == "" {
			modelName = "llama3.2"
		}
		newProvider = openai.NewProvider("ollama-no-key", modelName, "http://localhost:11434/v1")

	case "azure":
		apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
		endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
		if apiKey == "" || endpoint == "" {
			return fmt.Errorf("AZURE_OPENAI_API_KEY and AZURE_OPENAI_ENDPOINT environment variables required")
		}
		if modelName == "" {
			modelName = "gpt-4.1"
		}
		newProvider = openai.NewProvider(apiKey, modelName, endpoint)

	default:
		return fmt.Errorf("unsupported provider: %s\n\nSupported providers:\n  • openai, anthropic (native)\n  • groq, google, mistral, cohere, deepseek\n  • openrouter, together, perplexity, xai\n  • ollama (local), azure\n\nSet the appropriate API key environment variable", providerName)
	}

	// Switch provider
	o.provider = newProvider
	o.actualModelName = "" // Reset - will be set on next API response

	// Update workspace config
	o.workspace.Config.Model.DefaultProvider = providerName

	// IMPORTANT: Also update the model name in the config
	switch providerName {
	case "openai":
		o.workspace.Config.Model.OpenAI.Model = modelName
	case "anthropic":
		o.workspace.Config.Model.Anthropic.Model = modelName
	case "groq", "google", "mistral", "cohere", "deepseek", "openrouter", "together", "perplexity", "xai", "ollama", "azure":
		// OpenAI-compatible providers store model in OpenAI config
		o.workspace.Config.Model.OpenAI.Model = modelName
	}

	return nil
}

// GetCurrentProvider returns the current provider name and model
func (o *Orchestrator) GetCurrentProvider() (string, string) {
	providerName := o.workspace.Config.Model.DefaultProvider
	modelName := ""

	switch providerName {
	case "openai":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "gpt-4.1-mini"
		}
	case "anthropic":
		modelName = o.workspace.Config.Model.Anthropic.Model
		if modelName == "" {
			modelName = "claude-sonnet-4-20250514"
		}
	case "groq":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "llama-3.3-70b-versatile"
		}
	case "google":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "gemini-2.5-flash"
		}
	case "mistral":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "mistral-large-latest"
		}
	case "cohere":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "command-r-plus"
		}
	case "deepseek":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "deepseek-chat"
		}
	case "openrouter":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "openrouter/auto"
		}
	case "together":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo"
		}
	case "perplexity":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "sonar"
		}
	case "xai":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "grok-4-1-fast-reasoning"
		}
	case "ollama":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "llama3.2"
		}
	case "azure":
		modelName = o.workspace.Config.Model.OpenAI.Model
		if modelName == "" {
			modelName = "gpt-4.1"
		}
	}

	// If we have the actual model name from the API response, use it
	if o.actualModelName != "" {
		modelName = o.actualModelName
	}

	return providerName, modelName
}
