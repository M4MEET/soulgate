package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model selection UI
// These functions handle the interactive model selection interface

// buildModelOptions builds list of available models across all providers
func (m *InteractiveChatModel) buildModelOptions() []modelOption {
	return m.buildStaticModelOptions()
}

// buildStaticModelOptions returns hardcoded model list as fallback
func (m *InteractiveChatModel) buildStaticModelOptions() []modelOption {
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

	if hasOpenAI {
		options = append(options,
			modelOption{num, "GPT-4.1", "openai", "gpt-4.1", "Latest OpenAI flagship - Complex coding & analysis"},
			modelOption{num + 1, "GPT-4.1-mini", "openai", "gpt-4.1-mini", "Fast & economical - Balanced tasks"},
			modelOption{num + 2, "GPT-4.1-nano", "openai", "gpt-4.1-nano", "Fastest & cheapest - Simple tasks"},
			modelOption{num + 3, "o3", "openai", "o3", "Deep reasoning - Complex problem-solving"},
			modelOption{num + 4, "o4-mini", "openai", "o4-mini", "Efficient reasoning - Cost-effective"},
		)
		num += 5
	}

	if hasAnthropic {
		options = append(options,
			modelOption{num, "Claude Opus 4", "anthropic", "claude-opus-4-20250514", "Most capable Claude - Deep reasoning"},
			modelOption{num + 1, "Claude Sonnet 4", "anthropic", "claude-sonnet-4-20250514", "Balanced - Great for most tasks"},
			modelOption{num + 2, "Claude Haiku 4", "anthropic", "claude-haiku-4-20250501", "Fast & efficient - Quick responses"},
		)
		num += 3
	}

	if hasGroq {
		options = append(options,
			modelOption{num, "Llama 3.3 70B (Groq)", "groq", "llama-3.3-70b-versatile", "Meta's latest - Lightning fast inference"},
			modelOption{num + 1, "Mixtral 8x7B (Groq)", "groq", "mixtral-8x7b-32768", "Mistral MoE - Large context window"},
			modelOption{num + 2, "Gemma 2 9B (Groq)", "groq", "gemma2-9b-it", "Google's efficient model - Fast"},
		)
		num += 3
	}

	if hasGoogle {
		options = append(options,
			modelOption{num, "Gemini 2.5 Pro", "google", "gemini-2.5-pro", "Most powerful stable - Complex tasks"},
			modelOption{num + 1, "Gemini 2.5 Flash", "google", "gemini-2.5-flash", "Fast & efficient - Real-time capable"},
			modelOption{num + 2, "Gemini 2.5 Flash Lite", "google", "gemini-2.5-flash-lite", "Lightweight - Quick responses"},
		)
		num += 3
	}

	if hasMistral {
		options = append(options,
			modelOption{num, "Mistral Large", "mistral", "mistral-large-latest", "Most capable Mistral - Complex reasoning"},
			modelOption{num + 1, "Mistral Medium", "mistral", "mistral-medium-latest", "Balanced - Good for most tasks"},
			modelOption{num + 2, "Mistral Small", "mistral", "mistral-small-latest", "Fast & economical - Simple tasks"},
		)
		num += 3
	}

	if hasCohere {
		options = append(options,
			modelOption{num, "Command R+", "cohere", "command-r-plus", "Most capable Cohere - RAG & tools"},
			modelOption{num + 1, "Command R", "cohere", "command-r", "Balanced - Good performance"},
		)
		num += 2
	}

	if hasDeepSeek {
		options = append(options,
			modelOption{num, "DeepSeek V3", "deepseek", "deepseek-chat", "Chinese model - Strong coding ability"},
			modelOption{num + 1, "DeepSeek Coder", "deepseek", "deepseek-coder", "Specialized for code generation"},
		)
		num += 2
	}

	if hasOpenRouter {
		options = append(options,
			modelOption{num, "Auto (OpenRouter)", "openrouter", "openrouter/auto", "Best available model - Auto-selected"},
			modelOption{num + 1, "GPT-4.1 (OpenRouter)", "openrouter", "openai/gpt-4.1", "OpenAI via OpenRouter - Fallback routing"},
			modelOption{num + 2, "Claude Opus 4 (OpenRouter)", "openrouter", "anthropic/claude-opus-4", "Claude via OpenRouter - Fallback routing"},
		)
		num += 3
	}

	if hasTogether {
		options = append(options,
			modelOption{num, "Llama 3.1 405B (Together)", "together", "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo", "Largest open model - Powerful"},
			modelOption{num + 1, "Qwen 2.5 72B (Together)", "together", "Qwen/Qwen2.5-72B-Instruct-Turbo", "Alibaba's model - Strong multilingual"},
		)
		num += 2
	}

	if hasPerplexity {
		options = append(options,
			modelOption{num, "Perplexity Sonar", "perplexity", "sonar", "Real-time web search - Current info"},
			modelOption{num + 1, "Perplexity Sonar Pro", "perplexity", "sonar-pro", "Advanced search - Deep research"},
		)
		num += 2
	}

	// Ollama (local models - always available)
	options = append(options,
		modelOption{num, "Llama 3.2 (Ollama)", "ollama", "llama3.2", "Local - Privacy & no API costs"},
		modelOption{num + 1, "Mistral (Ollama)", "ollama", "mistral", "Local - Fast inference"},
		modelOption{num + 2, "CodeLlama (Ollama)", "ollama", "codellama", "Local - Code generation"},
	)

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
		if m.selectedModelIndex >= 0 && m.selectedModelIndex < len(providers) {
			selectedProvider := providers[m.selectedModelIndex]
			m.selectedProvider = selectedProvider.id

			if !selectedProvider.hasAPIKey {
				m.showAPIKeyPrompt = true
				m.apiKeyProvider = selectedProvider.id
				m.showModelSelector = false

				apiKeyInput := textinput.New()
				apiKeyInput.Placeholder = "Enter your API key..."
				apiKeyInput.Focus()
				apiKeyInput.CharLimit = 200
				apiKeyInput.Width = 60
				apiKeyInput.EchoMode = textinput.EchoPassword
				apiKeyInput.EchoCharacter = '*'
				m.apiKeyInput = apiKeyInput
			} else {
				m.modelSelectionStep = 2
				m.selectedModelIndex = 0
				m.modelOptions = m.buildModelOptionsForProvider(m.selectedProvider)
			}
		}
		return *m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		number := int(key[0] - '0')
		if number > 0 && number <= len(providers) {
			selectedProvider := providers[number-1]
			m.selectedProvider = selectedProvider.id

			if !selectedProvider.hasAPIKey {
				m.showAPIKeyPrompt = true
				m.apiKeyProvider = selectedProvider.id
				m.showModelSelector = false

				apiKeyInput := textinput.New()
				apiKeyInput.Placeholder = "Enter your API key..."
				apiKeyInput.Focus()
				apiKeyInput.CharLimit = 200
				apiKeyInput.Width = 60
				apiKeyInput.EchoMode = textinput.EchoPassword
				apiKeyInput.EchoCharacter = '*'
				m.apiKeyInput = apiKeyInput
			} else {
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

// handleModelSelection handles model selection (step 2)
func (m *InteractiveChatModel) handleModelSelection(key string) (InteractiveChatModel, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.selectedModelIndex > 0 {
			m.selectedModelIndex--
		}
		return *m, nil

	case "down", "j":
		if m.selectedModelIndex < len(m.modelOptions)-1 {
			m.selectedModelIndex++
		}
		return *m, nil

	case "enter", " ":
		if m.selectedModelIndex >= 0 && m.selectedModelIndex < len(m.modelOptions) {
			opt := m.modelOptions[m.selectedModelIndex]
			m.showModelSelector = false
			if err := m.orch.SetProvider(opt.provider, opt.model); err != nil {
				m.addMessage(colorError(fmt.Sprintf("Failed to switch: %s", err.Error())))
			} else {
				m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
				if err := m.orch.GetWorkspace().SaveConfig(); err != nil {
					m.addMessage(colorWarn(fmt.Sprintf("Model switched but config save failed: %s", err.Error())))
				}
				m.addMessage(colorSuccess(fmt.Sprintf("Switched to %s - %s", opt.name, opt.description)))
			}
		}
		return *m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		number := int(key[0] - '0')
		for _, opt := range m.modelOptions {
			if opt.number == number {
				m.showModelSelector = false
				if err := m.orch.SetProvider(opt.provider, opt.model); err != nil {
					m.addMessage(colorError(fmt.Sprintf("Failed to switch: %s", err.Error())))
				} else {
					m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
					if err := m.orch.GetWorkspace().SaveConfig(); err != nil {
						m.addMessage(colorWarn(fmt.Sprintf("Model switched but config save failed: %s", err.Error())))
					}
					m.addMessage(colorSuccess(fmt.Sprintf("Switched to %s - %s", opt.name, opt.description)))
				}
				return *m, nil
			}
		}
		return *m, nil

	case "esc", "q", "Q", "backspace":
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
		{id: "ollama", name: "Ollama (Local)", description: "Run models locally - Private", modelCount: 3, hasAPIKey: true},
	}
}

// buildModelOptionsForProvider builds model list for a specific provider
func (m *InteractiveChatModel) buildModelOptionsForProvider(provider string) []modelOption {
	options := []modelOption{}

	// Try dynamic discovery first
	if m.modelDiscovery != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		discoveredModels, err := m.modelDiscovery.DiscoverModels(ctx, provider)
		if err == nil && len(discoveredModels) > 0 {
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
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	hl := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	providers := m.getAvailableProviders()

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Select Provider") + dim.Render("  step 1/2") + "\n\n")

	for i, provider := range providers {
		isSelected := (i == m.selectedModelIndex)

		// Selection indicator
		prefix := "    "
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		if isSelected {
			prefix = "  " + hl.Render("> ")
			nameStyle = hl
		}

		// API key status
		var status string
		if provider.hasAPIKey {
			status = ok.Render(" ready")
		} else {
			status = warn.Render(" needs key")
		}

		sb.WriteString(prefix + dim.Render(fmt.Sprintf("%2d ", i+1)) + nameStyle.Render(provider.name) + status + "\n")
		sb.WriteString("      " + dim.Render(provider.description) + "\n")
	}

	// Count saved keys
	savedCount := 0
	for _, p := range providers {
		if p.hasAPIKey {
			savedCount++
		}
	}

	sb.WriteString("\n")
	sb.WriteString("  " + ok.Render(fmt.Sprintf("%d", savedCount)) + dim.Render(fmt.Sprintf("/%d providers with API keys saved", len(providers))) + "\n\n")
	sb.WriteString("  " + dim.Render("press ") + cmd.Render("up/down") + dim.Render(" to navigate, ") + cmd.Render("enter") + dim.Render(" to select, ") + cmd.Render("esc") + dim.Render(" to cancel") + "\n")

	return sb.String()
}

// renderModelSelector renders the model selection UI (step 2)
func (m *InteractiveChatModel) renderModelSelector() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	hl := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	providerName := strings.Title(m.selectedProvider)

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render(providerName+" Models") + dim.Render("  step 2/2") + "\n\n")

	// Scrolling window for models
	total := len(m.modelOptions)
	maxVisible := 10
	start := 0
	end := total

	if total > maxVisible {
		start = m.selectedModelIndex - maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > total {
			end = total
			start = end - maxVisible
		}
	}

	if start > 0 {
		sb.WriteString("    " + dim.Render(fmt.Sprintf("... %d above", start)) + "\n")
	}

	for i := start; i < end; i++ {
		opt := m.modelOptions[i]
		isSelected := (i == m.selectedModelIndex)
		isCurrent := (m.currentProvider == opt.provider && strings.Contains(m.currentModel, opt.model))

		prefix := "    "
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		if isSelected {
			prefix = "  " + hl.Render("> ")
			nameStyle = hl
		}

		marker := ""
		if isCurrent {
			marker = ok.Render(" current")
		}

		sb.WriteString(prefix + dim.Render(fmt.Sprintf("%2d ", opt.number)) + nameStyle.Render(opt.name) + marker + "\n")
		sb.WriteString("      " + dim.Render(opt.description) + "\n")
	}

	if end < total {
		sb.WriteString("    " + dim.Render(fmt.Sprintf("... %d below", total-end)) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + dim.Render("press ") + cmd.Render("up/down") + dim.Render(" to navigate, ") + cmd.Render("enter") + dim.Render(" to select, ") + cmd.Render("esc") + dim.Render(" to go back") + "\n")

	return sb.String()
}

// stripAnsi removes ANSI escape codes for length calculation
func stripAnsi(str string) string {
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(str, "")
}
