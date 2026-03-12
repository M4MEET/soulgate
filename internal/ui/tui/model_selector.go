package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/model"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// Model selection UI
// These functions handle the interactive model selection interface

// buildModelOptions builds list of available models across all providers
func (m *InteractiveChatModel) buildModelOptions() []modelOption {
	// Use static list for now (dynamic discovery can be enabled later)
	// This ensures /model always works
	return m.buildStaticModelOptions()

	/* Dynamic discovery (disabled for now - needs debugging)
	options := []modelOption{}
	num := 1

	// Check if modelDiscovery is initialized
	if m.modelDiscovery == nil {
		return m.buildStaticModelOptions()
	}

	// Use dynamic model discovery
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	allModels, err := m.modelDiscovery.GetAllModels(ctx)
	if err != nil || len(allModels) == 0 {
		// Fallback to static list if discovery fails
		return m.buildStaticModelOptions()
	}

	// Convert discovered models to modelOption format
	for _, modelInfo := range allModels {
		options = append(options, modelOption{
			number:      num,
			name:        modelInfo.Name,
			provider:    modelInfo.Provider,
			model:       modelInfo.ID,
			description: modelInfo.Description,
		})
		num++
	}

	return options
	*/
}

// buildStaticModelOptions returns hardcoded model list as fallback
func (m *InteractiveChatModel) buildStaticModelOptions() []modelOption {
	// Check which API keys are available
	hasOpenAI := os.Getenv("OPENAI_API_KEY") != ""
	hasAnthropic := os.Getenv("ANTHROPIC_API_KEY") != ""
	hasGroq := os.Getenv("GROQ_API_KEY") != ""
	hasGoogle := os.Getenv("GOOGLE_API_KEY") != ""
	hasMistral := os.Getenv("MISTRAL_API_KEY") != ""
	hasCohere := os.Getenv("COHERE_API_KEY") != ""
	hasDeepSeek := os.Getenv("DEEPSEEK_API_KEY") != ""
	hasOpenRouter := os.Getenv("OPENROUTER_API_KEY") != ""
	hasTogether := os.Getenv("TOGETHER_API_KEY") != ""
	hasPerplexity := os.Getenv("PERPLEXITY_API_KEY") != ""

	options := []modelOption{}
	num := 1

	// OpenAI models (if API key available)
	if hasOpenAI {
		options = append(options, modelOption{
			number:      num,
			name:        "GPT-4o",
			provider:    "openai",
			model:       "gpt-4o",
			description: "Most capable OpenAI - Complex coding & analysis",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "GPT-4o-mini",
			provider:    "openai",
			model:       "gpt-4o-mini",
			description: "Fast & economical - Simple tasks",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "GPT-4-Turbo",
			provider:    "openai",
			model:       "gpt-4-turbo",
			description: "Previous gen - Reliable for most tasks",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "GPT-3.5-Turbo",
			provider:    "openai",
			model:       "gpt-3.5-turbo",
			description: "Budget friendly - Quick responses",
		})
		num++
	}

	// Anthropic models (if API key available)
	if hasAnthropic {
		options = append(options, modelOption{
			number:      num,
			name:        "Claude Opus 4",
			provider:    "anthropic",
			model:       "claude-opus-4-20250514",
			description: "Most capable Claude - Deep reasoning",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Claude Sonnet 4",
			provider:    "anthropic",
			model:       "claude-sonnet-4-20250514",
			description: "Balanced - Great for most tasks",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Claude Haiku 4",
			provider:    "anthropic",
			model:       "claude-haiku-4-20250501",
			description: "Fast & efficient - Quick responses",
		})
		num++
	}

	// Groq models (if API key available)
	if hasGroq {
		options = append(options, modelOption{
			number:      num,
			name:        "Llama 3.3 70B (Groq)",
			provider:    "groq",
			model:       "llama-3.3-70b-versatile",
			description: "Meta's latest - Lightning fast inference",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Mixtral 8x7B (Groq)",
			provider:    "groq",
			model:       "mixtral-8x7b-32768",
			description: "Mistral MoE - Large context window",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Gemma 2 9B (Groq)",
			provider:    "groq",
			model:       "gemma2-9b-it",
			description: "Google's efficient model - Fast",
		})
		num++
	}

	// Google models (if API key available)
	if hasGoogle {
		options = append(options, modelOption{
			number:      num,
			name:        "Gemini 2.0 Flash",
			provider:    "google",
			model:       "gemini-2.0-flash-exp",
			description: "Latest Gemini - Experimental features",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Gemini 1.5 Pro",
			provider:    "google",
			model:       "gemini-1.5-pro",
			description: "Powerful - 2M context window",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Gemini 1.5 Flash",
			provider:    "google",
			model:       "gemini-1.5-flash",
			description: "Fast & efficient - Quick tasks",
		})
		num++
	}

	// Mistral models (if API key available)
	if hasMistral {
		options = append(options, modelOption{
			number:      num,
			name:        "Mistral Large",
			provider:    "mistral",
			model:       "mistral-large-latest",
			description: "Most capable Mistral - Complex reasoning",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Mistral Medium",
			provider:    "mistral",
			model:       "mistral-medium-latest",
			description: "Balanced - Good for most tasks",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Mistral Small",
			provider:    "mistral",
			model:       "mistral-small-latest",
			description: "Fast & economical - Simple tasks",
		})
		num++
	}

	// Cohere models (if API key available)
	if hasCohere {
		options = append(options, modelOption{
			number:      num,
			name:        "Command R+",
			provider:    "cohere",
			model:       "command-r-plus",
			description: "Most capable Cohere - RAG & tools",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Command R",
			provider:    "cohere",
			model:       "command-r",
			description: "Balanced - Good performance",
		})
		num++
	}

	// DeepSeek models (if API key available)
	if hasDeepSeek {
		options = append(options, modelOption{
			number:      num,
			name:        "DeepSeek V3",
			provider:    "deepseek",
			model:       "deepseek-chat",
			description: "Chinese model - Strong coding ability",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "DeepSeek Coder",
			provider:    "deepseek",
			model:       "deepseek-coder",
			description: "Specialized for code generation",
		})
		num++
	}

	// OpenRouter models (if API key available)
	if hasOpenRouter {
		options = append(options, modelOption{
			number:      num,
			name:        "Auto (OpenRouter)",
			provider:    "openrouter",
			model:       "openrouter/auto",
			description: "Best available model - Auto-selected",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "GPT-4o (OpenRouter)",
			provider:    "openrouter",
			model:       "openai/gpt-4o",
			description: "OpenAI via OpenRouter - Fallback routing",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Claude Opus 4 (OpenRouter)",
			provider:    "openrouter",
			model:       "anthropic/claude-opus-4",
			description: "Claude via OpenRouter - Fallback routing",
		})
		num++
	}

	// Together AI models (if API key available)
	if hasTogether {
		options = append(options, modelOption{
			number:      num,
			name:        "Llama 3.1 405B (Together)",
			provider:    "together",
			model:       "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo",
			description: "Largest open model - Powerful",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Qwen 2.5 72B (Together)",
			provider:    "together",
			model:       "Qwen/Qwen2.5-72B-Instruct-Turbo",
			description: "Alibaba's model - Strong multilingual",
		})
		num++
	}

	// Perplexity models (if API key available)
	if hasPerplexity {
		options = append(options, modelOption{
			number:      num,
			name:        "Perplexity Sonar",
			provider:    "perplexity",
			model:       "sonar",
			description: "Real-time web search - Current info",
		})
		num++

		options = append(options, modelOption{
			number:      num,
			name:        "Perplexity Sonar Pro",
			provider:    "perplexity",
			model:       "sonar-pro",
			description: "Advanced search - Deep research",
		})
		num++
	}

	// Ollama (local models - always available)
	options = append(options, modelOption{
		number:      num,
		name:        "Llama 3.2 (Ollama)",
		provider:    "ollama",
		model:       "llama3.2",
		description: "Local - Privacy & no API costs",
	})
	num++

	options = append(options, modelOption{
		number:      num,
		name:        "Mistral (Ollama)",
		provider:    "ollama",
		model:       "mistral",
		description: "Local - Fast inference",
	})
	num++

	options = append(options, modelOption{
		number:      num,
		name:        "CodeLlama (Ollama)",
		provider:    "ollama",
		model:       "codellama",
		description: "Local - Code generation",
	})

	return options
}

// handleProviderSelection handles provider selection (step 1)
func (m *InteractiveChatModel) handleProviderSelection(key string) (InteractiveChatModel, tea.Cmd) {
	providers := m.getAvailableProviders()

	switch key {
	case "up", "k":
		if m.selectedModelIndex > 0 {
			m.selectedModelIndex--
		}
		return *m, nil

	case "down", "j":
		if m.selectedModelIndex < len(providers)-1 {
			m.selectedModelIndex++
		}
		return *m, nil

	case "enter", " ":
		// Select provider and check if API key is needed
		if m.selectedModelIndex >= 0 && m.selectedModelIndex < len(providers) {
			selectedProvider := providers[m.selectedModelIndex]
			m.selectedProvider = selectedProvider.id

			// Check if API key is available
			if !selectedProvider.hasAPIKey {
				// Show API key prompt
				m.showAPIKeyPrompt = true
				m.apiKeyProvider = selectedProvider.id
				m.showModelSelector = false

				// Initialize API key input
				apiKeyInput := textinput.New()
				apiKeyInput.Placeholder = "Enter your API key..."
				apiKeyInput.Focus()
				apiKeyInput.CharLimit = 200
				apiKeyInput.Width = 60
				apiKeyInput.EchoMode = textinput.EchoPassword
				apiKeyInput.EchoCharacter = '•'
				m.apiKeyInput = apiKeyInput
			} else {
				// API key available, proceed to model selection
				m.modelSelectionStep = 2
				m.selectedModelIndex = 0 // Reset for model selection
				m.modelOptions = m.buildModelOptionsForProvider(m.selectedProvider)
			}
		}
		return *m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick number selection
		number := int(key[0] - '0')
		if number > 0 && number <= len(providers) {
			selectedProvider := providers[number-1]
			m.selectedProvider = selectedProvider.id

			// Check if API key is available
			if !selectedProvider.hasAPIKey {
				// Show API key prompt
				m.showAPIKeyPrompt = true
				m.apiKeyProvider = selectedProvider.id
				m.showModelSelector = false

				// Initialize API key input
				apiKeyInput := textinput.New()
				apiKeyInput.Placeholder = "Enter your API key..."
				apiKeyInput.Focus()
				apiKeyInput.CharLimit = 200
				apiKeyInput.Width = 60
				apiKeyInput.EchoMode = textinput.EchoPassword
				apiKeyInput.EchoCharacter = '•'
				m.apiKeyInput = apiKeyInput
			} else {
				// API key available, proceed to model selection
				m.modelSelectionStep = 2
				m.selectedModelIndex = 0
				m.modelOptions = m.buildModelOptionsForProvider(m.selectedProvider)
			}
		}
		return *m, nil

	case "esc", "q", "Q":
		m.showModelSelector = false
		m.addMessage(colorMuted("Model selection cancelled"))
		return *m, nil
	}

	return *m, nil
}

// handleModelSelection handles model selection (step 2) with grid navigation
func (m *InteractiveChatModel) handleModelSelection(key string) (InteractiveChatModel, tea.Cmd) {
	numColumns := 3 // Match the column count in renderModelSelector

	switch key {
	case "up", "k":
		// Move up one row (3 positions back)
		newIndex := m.selectedModelIndex - numColumns
		if newIndex >= 0 {
			m.selectedModelIndex = newIndex
		} else {
			// Wrap to bottom row, same column
			col := m.selectedModelIndex % numColumns
			numRows := (len(m.modelOptions) + numColumns - 1) / numColumns
			lastRowIndex := (numRows-1)*numColumns + col
			if lastRowIndex < len(m.modelOptions) {
				m.selectedModelIndex = lastRowIndex
			}
		}
		return *m, nil

	case "down", "j":
		// Move down one row (3 positions forward)
		newIndex := m.selectedModelIndex + numColumns
		if newIndex < len(m.modelOptions) {
			m.selectedModelIndex = newIndex
		} else {
			// Wrap to top row, same column
			m.selectedModelIndex = m.selectedModelIndex % numColumns
		}
		return *m, nil

	case "left", "h":
		// Move left one column
		if m.selectedModelIndex > 0 {
			m.selectedModelIndex--
		} else {
			// Wrap to last item
			m.selectedModelIndex = len(m.modelOptions) - 1
		}
		return *m, nil

	case "right", "l":
		// Move right one column
		if m.selectedModelIndex < len(m.modelOptions)-1 {
			m.selectedModelIndex++
		} else {
			// Wrap to first item
			m.selectedModelIndex = 0
		}
		return *m, nil

	case "enter", " ":
		// Select model and switch
		if m.selectedModelIndex >= 0 && m.selectedModelIndex < len(m.modelOptions) {
			opt := m.modelOptions[m.selectedModelIndex]
			m.showModelSelector = false
			if err := m.orch.SetProvider(opt.provider, opt.model); err != nil {
				m.addMessage(colorError(fmt.Sprintf("✗ Failed to switch: %s", err.Error())))
			} else {
				m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
				// Save config to persist the change
				if err := m.orch.GetWorkspace().SaveConfig(); err != nil {
					m.addMessage(colorWarn(fmt.Sprintf("⚠ Model switched but config save failed: %s", err.Error())))
				}
				m.addMessage(colorSuccess(fmt.Sprintf("✓ Switched to %s - %s", opt.name, opt.description)))
			}
		}
		return *m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Quick number selection
		number := int(key[0] - '0')
		for _, opt := range m.modelOptions {
			if opt.number == number {
				m.showModelSelector = false
				if err := m.orch.SetProvider(opt.provider, opt.model); err != nil {
					m.addMessage(colorError(fmt.Sprintf("✗ Failed to switch: %s", err.Error())))
				} else {
					m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
					// Save config to persist the change
					if err := m.orch.GetWorkspace().SaveConfig(); err != nil {
						m.addMessage(colorWarn(fmt.Sprintf("⚠ Model switched but config save failed: %s", err.Error())))
					}
					m.addMessage(colorSuccess(fmt.Sprintf("✓ Switched to %s - %s", opt.name, opt.description)))
				}
				return *m, nil
			}
		}
		return *m, nil

	case "esc", "q", "Q", "backspace":
		// Go back to provider selection
		m.modelSelectionStep = 1
		m.selectedModelIndex = 0
		m.selectedProvider = ""
		return *m, nil
	}

	return *m, nil
}

// renderModelSelectorPrompt renders the interactive model selector
func (m *InteractiveChatModel) renderModelSelectorPrompt() string {
	if m.modelSelectionStep == 1 {
		return m.renderProviderSelector()
	}
	return m.renderModelSelector()
}

// Provider info struct for selection
type providerInfoStruct struct {
	id          string
	name        string
	description string
	modelCount  int
	hasAPIKey   bool
}

// getAvailableProviders returns list of all supported providers
func (m *InteractiveChatModel) getAvailableProviders() []providerInfoStruct {
	// Return ALL providers - user should see all options
	// We'll show which ones need API key configuration
	return []providerInfoStruct{
		{id: "openai", name: "OpenAI", description: "GPT-4o, GPT-4o-mini, GPT-4-Turbo", modelCount: 4, hasAPIKey: os.Getenv("OPENAI_API_KEY") != ""},
		{id: "anthropic", name: "Anthropic", description: "Claude Opus, Sonnet, Haiku", modelCount: 3, hasAPIKey: os.Getenv("ANTHROPIC_API_KEY") != ""},
		{id: "groq", name: "Groq", description: "Llama 3.3, Mixtral - Lightning fast", modelCount: 3, hasAPIKey: os.Getenv("GROQ_API_KEY") != ""},
		{id: "google", name: "Google", description: "Gemini 3, 2.5 Pro/Flash - Multimodal", modelCount: 8, hasAPIKey: os.Getenv("GOOGLE_API_KEY") != ""},
		{id: "mistral", name: "Mistral AI", description: "Large, Medium, Small", modelCount: 3, hasAPIKey: os.Getenv("MISTRAL_API_KEY") != ""},
		{id: "cohere", name: "Cohere", description: "Command R+, Command R", modelCount: 2, hasAPIKey: os.Getenv("COHERE_API_KEY") != ""},
		{id: "deepseek", name: "DeepSeek", description: "V3, Coder - Strong coding", modelCount: 2, hasAPIKey: os.Getenv("DEEPSEEK_API_KEY") != ""},
		{id: "openrouter", name: "OpenRouter", description: "Access 100+ models", modelCount: 3, hasAPIKey: os.Getenv("OPENROUTER_API_KEY") != ""},
		{id: "together", name: "Together AI", description: "Llama 3.1 405B, Qwen 2.5", modelCount: 2, hasAPIKey: os.Getenv("TOGETHER_API_KEY") != ""},
		{id: "perplexity", name: "Perplexity", description: "Sonar - Real-time web search", modelCount: 2, hasAPIKey: os.Getenv("PERPLEXITY_API_KEY") != ""},
		{id: "xai", name: "xAI (Grok)", description: "Grok 4.1, Grok 3, Vision - Real-time", modelCount: 7, hasAPIKey: os.Getenv("XAI_API_KEY") != ""},
		{id: "ollama", name: "Ollama (Local)", description: "Run models locally - Private", modelCount: 3, hasAPIKey: true}, // Always available
	}
}

// buildModelOptionsForProvider builds model list for a specific provider
// Uses dynamic discovery to fetch latest models from provider APIs
func (m *InteractiveChatModel) buildModelOptionsForProvider(provider string) []modelOption {
	options := []modelOption{}

	// Try dynamic discovery first (to get latest models automatically)
	if m.modelDiscovery != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		discoveredModels, err := m.modelDiscovery.DiscoverModels(ctx, provider)
		if err == nil && len(discoveredModels) > 0 {
			// Successfully discovered models dynamically
			for i, modelInfo := range discoveredModels {
				options = append(options, modelOption{
					number:      i + 1,
					name:        modelInfo.Name,
					provider:    provider,
					model:       modelInfo.ID,
					description: modelInfo.Description,
				})
			}
			return options
		}
		// If discovery failed, fall through to static list
	}

	// Fallback: Use static model list
	providerModels := model.BuildModelOptionsForProvider(provider)
	for i, pm := range providerModels {
		options = append(options, modelOption{
			number:      i + 1,
			name:        pm.Name,
			provider:    provider,
			model:       pm.ID,
			description: pm.Description,
		})
	}

	return options
}

// renderProviderSelector renders the provider selection UI (step 1)
func (m *InteractiveChatModel) renderProviderSelector() string {
	var sb strings.Builder
	providers := m.getAvailableProviders()

	// Header with proper border
	borderWidth := 58
	titleText := " 🤖 Select LLM Provider (Step 1/2) "
	titleLen := len(titleText)
	remainingDashes := borderWidth - titleLen - 2

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true).
		Render("  ╭─" + titleText + strings.Repeat("─", remainingDashes) + "╮"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Provider list
	for i, provider := range providers {
		isSelected := (i == m.selectedModelIndex)

		// Highlight selected provider
		prefix := "  │ "
		if isSelected {
			prefix = "  │ " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Background(lipgloss.Color("236")).
				Render("►")
		} else {
			prefix = "  │  "
		}

		nameStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)
		if isSelected {
			nameStyle = nameStyle.
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("208"))
		}

		// API key status indicator
		var statusIndicator string
		var statusStyle lipgloss.Style
		if provider.hasAPIKey {
			statusIndicator = " ✓ Ready"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Green
		} else {
			statusIndicator = " ⚠ Needs API Key"
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
		}
		if isSelected {
			statusStyle = statusStyle.Background(lipgloss.Color("236"))
		}

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render(prefix) +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true).
				Render(fmt.Sprintf("%d. ", i+1)) +
			nameStyle.Render(provider.name) +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render(fmt.Sprintf(" (%d models)", provider.modelCount)) +
			statusStyle.Render(statusIndicator) + "\n")

		descPrefix := "  │    "
		if isSelected {
			descPrefix = "  │     "
		}

		descStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true)
		if isSelected {
			descStyle = descStyle.
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("252"))
		}

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render(descPrefix) +
			descStyle.Render(provider.description) + "\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │") + "\n")
	}

	// Instructions
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Bold(true).
			Render("↑↓") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Navigate  •  ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true).
			Render("Enter/Space") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Select  •  ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render("Esc") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Cancel") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Add API key setup hint
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Render("Tip: Set API keys with ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Render("export OPENAI_API_KEY=\"sk-...\"") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Footer with proper border matching header
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  ╰" + strings.Repeat("─", borderWidth) + "╯"))
	sb.WriteString("\n")

	return sb.String()
}

// renderModelSelector renders the model selection UI (step 2) with compact multi-column layout
func (m *InteractiveChatModel) renderModelSelector() string {
	var sb strings.Builder

	// Header with proper border - wider for 3 columns
	borderWidth := 110
	providerName := strings.Title(m.selectedProvider)
	titleText := fmt.Sprintf(" %s Models (Step 2/2) ", providerName)
	titleLen := len(titleText)
	remainingDashes := borderWidth - titleLen - 2

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true).
		Render("  ╭─" + titleText + strings.Repeat("─", remainingDashes) + "╮"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Display models in 3 columns for compact view
	numColumns := 3
	columnWidth := 35 // Width per column

	// Process models in rows of 3
	for rowStart := 0; rowStart < len(m.modelOptions); rowStart += numColumns {
		rowEnd := rowStart + numColumns
		if rowEnd > len(m.modelOptions) {
			rowEnd = len(m.modelOptions)
		}

		// Build the row with up to 3 columns
		var columns []string
		for col := rowStart; col < rowEnd; col++ {
			opt := m.modelOptions[col]
			isCurrent := (m.currentProvider == opt.provider && strings.Contains(m.currentModel, opt.model))
			isSelected := (col == m.selectedModelIndex)

			var cellContent strings.Builder

			// Selection indicator
			if isSelected {
				cellContent.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Background(lipgloss.Color("236")).
					Render("►"))
			} else {
				cellContent.WriteString(" ")
			}

			// Number and name
			numStr := lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true).
				Render(fmt.Sprintf("%2d.", opt.number))

			nameStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Bold(true)
			if isSelected {
				nameStyle = nameStyle.
					Background(lipgloss.Color("236")).
					Foreground(lipgloss.Color("208"))
			}

			// Truncate name if too long
			displayName := opt.name
			maxNameLen := 25
			if len(displayName) > maxNameLen {
				displayName = displayName[:maxNameLen-3] + "..."
			}

			marker := ""
			if isCurrent {
				marker = lipgloss.NewStyle().
					Foreground(lipgloss.Color("82")).
					Render("✓")
			}

			cellContent.WriteString(numStr)
			cellContent.WriteString(" ")

			if isSelected {
				cellContent.WriteString(lipgloss.NewStyle().
					Background(lipgloss.Color("236")).
					Width(columnWidth - 5).
					Render(nameStyle.Render(displayName) + marker))
			} else {
				cellContent.WriteString(nameStyle.Render(displayName) + marker)
			}

			// Pad to column width
			cell := cellContent.String()
			// Remove ANSI codes for width calculation
			plainLen := len(stripAnsi(cell))
			if plainLen < columnWidth {
				cell += strings.Repeat(" ", columnWidth-plainLen)
			}

			columns = append(columns, cell)
		}

		// Render the row
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(strings.Join(columns, " "))
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Instructions
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Bold(true).
			Render("↑↓←→") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Navigate  •  ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true).
			Render("Enter/Space") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Select  •  ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render("Esc") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Back") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Footer with proper border matching header
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  ╰" + strings.Repeat("─", borderWidth) + "╯"))
	sb.WriteString("\n")

	return sb.String()
}

// stripAnsi removes ANSI escape codes for length calculation
func stripAnsi(str string) string {
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(str, "")
}
