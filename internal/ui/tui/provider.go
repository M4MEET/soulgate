package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Provider and API key management
// These functions handle API key input and storage

// handleAPIKeyInput handles API key input
func (m *InteractiveChatModel) handleAPIKeyInput(msg tea.KeyMsg) (InteractiveChatModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Save API key and proceed
		apiKey := m.apiKeyInput.Value()
		if apiKey == "" {
			m.addMessage(colorError("✗ API key cannot be empty"))
			return *m, nil
		}

		// Save to workspace config
		if err := m.saveAPIKey(m.apiKeyProvider, apiKey); err != nil {
			m.addMessage(colorError(fmt.Sprintf("✗ Failed to save API key: %s", err.Error())))
			m.showAPIKeyPrompt = false
			return *m, nil
		}

		// Close API key prompt
		m.showAPIKeyPrompt = false

		// Show success message
		providerName := strings.Title(m.apiKeyProvider)
		m.addMessage(colorSuccess(fmt.Sprintf("✓ API key saved for %s", providerName)))

		// Proceed to model selection
		m.showModelSelector = true
		m.modelSelectionStep = 2
		m.selectedModelIndex = 0
		m.modelOptions = m.buildModelOptionsForProvider(m.apiKeyProvider)

		return *m, nil

	case "esc":
		// Cancel API key input
		m.showAPIKeyPrompt = false
		m.showModelSelector = true  // Go back to provider selection
		m.modelSelectionStep = 1
		m.selectedModelIndex = 0
		m.addMessage(colorMuted("API key entry cancelled"))
		return *m, nil

	default:
		// Update text input with the key message
		var cmd tea.Cmd
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		return *m, cmd
	}
}

// saveAPIKey saves an API key to the workspace config
func (m *InteractiveChatModel) saveAPIKey(provider string, apiKey string) error {
	// Save to environment variable for current session
	envVarName := strings.ToUpper(provider) + "_API_KEY"
	os.Setenv(envVarName, apiKey)

	// Also save to workspace config for persistence
	switch provider {
	case "openai":
		m.orch.GetWorkspace().Config.Model.OpenAI.APIKey = apiKey
	case "anthropic":
		m.orch.GetWorkspace().Config.Model.Anthropic.APIKey = apiKey
	case "groq", "google", "mistral", "cohere", "deepseek", "openrouter", "together", "perplexity", "xai":
		// These use OpenAI-compatible config
		m.orch.GetWorkspace().Config.Model.OpenAI.APIKey = apiKey
	}

	// Save config to file
	return m.orch.GetWorkspace().SaveConfig()
}

// renderAPIKeyPrompt renders the API key input prompt
func (m *InteractiveChatModel) renderAPIKeyPrompt() string {
	var sb strings.Builder

	// Get provider name
	providerName := strings.Title(m.apiKeyProvider)
	envVarName := strings.ToUpper(m.apiKeyProvider) + "_API_KEY"

	// Header
	borderWidth := 70
	titleText := fmt.Sprintf(" 🔑 Enter API Key for %s ", providerName)
	titleLen := len(titleText)
	remainingDashes := borderWidth - titleLen - 2

	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true).
		Render("  ╭─" + titleText + strings.Repeat("─", remainingDashes) + "╮"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Info message
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(fmt.Sprintf("This provider requires an API key to use. Your key will be")) + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("saved to your workspace config for future sessions.") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Environment variable hint
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Render(fmt.Sprintf("Environment variable: %s", envVarName)) + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Input field
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true).
			Render("API Key: ") +
		m.apiKeyInput.View() + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Instructions
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ ") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true).
			Render("Enter") +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render(" Save & Continue  •  ") +
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

	// Footer
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  ╰" + strings.Repeat("─", borderWidth) + "╯"))
	sb.WriteString("\n")

	return sb.String()
}
