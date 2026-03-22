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
		apiKey := m.apiKeyInput.Value()

		// If empty and a key already exists, proceed with existing key
		existingKey := m.orch.GetWorkspace().Config.ResolveAPIKey(m.apiKeyProvider)
		if apiKey == "" && existingKey != "" {
			m.showAPIKeyPrompt = false
			providerName := strings.Title(m.apiKeyProvider)
			m.addMessage(colorSuccess(fmt.Sprintf("Using saved key for %s", providerName)))
			m.showModelSelector = true
			m.modelSelectionStep = 2
			m.selectedModelIndex = 0
			m.modelOptions = m.buildModelOptionsForProvider(m.apiKeyProvider)
			return *m, nil
		}

		if apiKey == "" {
			m.addMessage(colorError("API key cannot be empty"))
			return *m, nil
		}

		// Save to workspace config
		if err := m.saveAPIKey(m.apiKeyProvider, apiKey); err != nil {
			m.addMessage(colorError(fmt.Sprintf("Failed to save API key: %s", err.Error())))
			m.showAPIKeyPrompt = false
			return *m, nil
		}

		// Close API key prompt
		m.showAPIKeyPrompt = false

		// Show success message
		providerName := strings.Title(m.apiKeyProvider)
		if existingKey != "" {
			m.addMessage(colorSuccess(fmt.Sprintf("API key overwritten for %s", providerName)))
		} else {
			m.addMessage(colorSuccess(fmt.Sprintf("API key saved for %s", providerName)))
		}

		// Proceed to model selection
		m.showModelSelector = true
		m.modelSelectionStep = 2
		m.selectedModelIndex = 0
		m.modelOptions = m.buildModelOptionsForProvider(m.apiKeyProvider)

		return *m, nil

	case "esc":
		// Cancel API key input — if key exists, proceed anyway
		existingKey := m.orch.GetWorkspace().Config.ResolveAPIKey(m.apiKeyProvider)
		m.showAPIKeyPrompt = false
		if existingKey != "" {
			// Key exists, just proceed to model selection
			m.showModelSelector = true
			m.modelSelectionStep = 2
			m.selectedModelIndex = 0
			m.modelOptions = m.buildModelOptionsForProvider(m.apiKeyProvider)
		} else {
			// No key, go back to provider selection
			m.showModelSelector = true
			m.modelSelectionStep = 1
			m.selectedModelIndex = 0
			m.addMessage(colorMuted("API key entry cancelled"))
		}
		return *m, nil

	default:
		// Update text input with the key message
		var cmd tea.Cmd
		m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
		return *m, cmd
	}
}

// saveAPIKey saves an API key to the workspace config using the multi-provider
// key store. Keys are persisted to .soulgate/config.yml under the api_keys map
// so switching providers doesn't lose previously saved keys.
func (m *InteractiveChatModel) saveAPIKey(provider string, apiKey string) error {
	// Save to environment variable for current session
	envVarName := strings.ToUpper(provider) + "_API_KEY"
	os.Setenv(envVarName, apiKey)

	// Use the multi-provider key store (also syncs to legacy fields)
	m.orch.GetWorkspace().Config.SetAPIKey(provider, apiKey)

	// Save config to file
	return m.orch.GetWorkspace().SaveConfig()
}

// renderAPIKeyPrompt renders the API key input prompt with saved key status
func (m *InteractiveChatModel) renderAPIKeyPrompt() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	providerName := strings.Title(m.apiKeyProvider)
	envVarName := strings.ToUpper(m.apiKeyProvider) + "_API_KEY"

	// Check if a key already exists for this provider
	existingKey := m.orch.GetWorkspace().Config.ResolveAPIKey(m.apiKeyProvider)
	hasExisting := existingKey != ""

	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("API Key") + dim.Render(" - "+providerName) + "\n\n")

	if hasExisting {
		// Show that a key exists and offer to replace
		masked := existingKey[:4] + "****" + existingKey[len(existingKey)-4:]
		sb.WriteString("  " + success.Render("Key saved: ") + dim.Render(masked) + "\n\n")
		sb.WriteString("  " + dim.Render("Enter a new key to overwrite, or press ") + cmd.Render("esc") + dim.Render(" to keep current.") + "\n\n")
	} else {
		sb.WriteString("  " + warn.Render("No key configured") + dim.Render(" for this provider.") + "\n")
		sb.WriteString("  " + dim.Render("Your key will be saved to .soulgate/config.yml") + "\n\n")
	}

	sb.WriteString("  " + dim.Render("Environment variable: ") + cmd.Render(envVarName) + "\n\n")

	// Show how many providers have keys
	savedCount := 0
	for _, name := range []string{"openai", "anthropic", "groq", "google", "mistral", "cohere", "deepseek", "openrouter", "together", "perplexity", "xai"} {
		if m.orch.GetWorkspace().Config.ResolveAPIKey(name) != "" {
			savedCount++
		}
	}
	if savedCount > 0 {
		sb.WriteString("  " + dim.Render(fmt.Sprintf("%d provider key(s) saved in config", savedCount)) + "\n\n")
	}

	label := "API Key: "
	if hasExisting {
		label = "New Key: "
	}
	sb.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true).Render(label) + m.apiKeyInput.View() + "\n\n")

	if hasExisting {
		sb.WriteString("  " + dim.Render("press ") + cmd.Render("enter") + dim.Render(" to overwrite, ") + cmd.Render("esc") + dim.Render(" to keep current key") + "\n")
	} else {
		sb.WriteString("  " + dim.Render("press ") + cmd.Render("enter") + dim.Render(" to save, ") + cmd.Render("esc") + dim.Render(" to cancel") + "\n")
	}

	return sb.String()
}
