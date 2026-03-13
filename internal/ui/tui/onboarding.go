package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles used across the onboarding wizard
var (
	onbTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	onbDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	onbCmd   = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	onbOk    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	onbWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	onbBad   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	onbHl    = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
)

// handleOnboardingInput handles input during onboarding
func (m *InteractiveChatModel) handleOnboardingInput(key string) (InteractiveChatModel, tea.Cmd) {
	if m.OnboardingState == nil {
		m.ShowOnboarding = false
		return *m, nil
	}

	currentStep := m.OnboardingState.GetCurrentStep()

	switch currentStep.Name {
	case "welcome":
		if key == "enter" || key == " " {
			m.OnboardingState.NextStep()
		} else if key == "q" || key == "ctrl+c" {
			m.ShowOnboarding = false
			m.OnboardingState.MarkComplete()
			m.ShowWelcome()
		}

	case "model_selection":
		options := onboarding.GetModelOptions()
		if key >= "1" && key <= "9" {
			idx := int(key[0] - '1')
			if idx < len(options) {
				m.OnboardingState.SelectedModel = options[idx].ID
				m.OnboardingState.SelectedProvider = options[idx].Provider
				m.OnboardingState.NextStep()
			}
		}

	case "api_keys":
		if key == "enter" {
			// Check if API key is already set via env
			provider := m.OnboardingState.SelectedProvider
			envKey := ""
			switch provider {
			case "openai":
				envKey = os.Getenv("OPENAI_API_KEY")
			case "anthropic":
				envKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			if envKey != "" {
				m.OnboardingState.NextStep()
			}
		} else if key == "s" || key == "S" {
			m.OnboardingState.NextStep()
		}

	case "test_connection":
		m.OnboardingState.NextStep()

	case "integrations":
		if key == "s" || key == "S" || key == "enter" {
			if len(m.OnboardingState.IntegrationsToSetup) > 0 {
				m.OnboardingState.NextStep()
				return *m, m.installDependenciesCmd()
			}
			m.OnboardingState.NextStep()
		} else if key >= "1" && key <= "4" {
			recommendations := onboarding.GetIntegrationRecommendations()
			idx := int(key[0] - '1')
			if idx < len(recommendations) {
				// Toggle integration
				id := recommendations[idx].ID
				found := false
				for i, existing := range m.OnboardingState.IntegrationsToSetup {
					if existing == id {
						m.OnboardingState.IntegrationsToSetup = append(
							m.OnboardingState.IntegrationsToSetup[:i],
							m.OnboardingState.IntegrationsToSetup[i+1:]...,
						)
						found = true
						break
					}
				}
				if !found {
					m.OnboardingState.IntegrationsToSetup = append(
						m.OnboardingState.IntegrationsToSetup, id,
					)
				}
			}
		}

	case "dependencies":
		if key == "enter" || !m.OnboardingState.InstallingDependencies {
			m.OnboardingState.NextStep()
		}

	case "tutorial":
		if key == "enter" || key == " " || key == "s" || key == "S" {
			m.OnboardingState.NextStep()
		}

	case "complete":
		if key == "enter" || key == " " {
			m.OnboardingState.MarkComplete()
			m.OnboardingState.SaveAPIKeys()
			m.ShowOnboarding = false
			m.ShowWelcome()
			m.addMessage(onbOk.Render("  Setup complete. Start chatting!"))
		}
	}

	return *m, nil
}

// renderOnboarding renders the onboarding wizard
func (m *InteractiveChatModel) renderOnboarding() string {
	if m.OnboardingState == nil {
		return ""
	}

	var sb strings.Builder
	currentStep := m.OnboardingState.GetCurrentStep()
	steps := onboarding.GetOnboardingSteps()
	progress := m.OnboardingState.GetProgress()

	// Progress indicator
	sb.WriteString("  ")
	for i, step := range steps {
		if i == m.OnboardingState.Step {
			sb.WriteString(onbHl.Render("●"))
		} else if i < m.OnboardingState.Step {
			sb.WriteString(onbOk.Render("●"))
		} else {
			sb.WriteString(onbDim.Render("○"))
		}
		if i < len(steps)-1 {
			if i < m.OnboardingState.Step {
				sb.WriteString(onbOk.Render("─"))
			} else {
				sb.WriteString(onbDim.Render("─"))
			}
		}
		_ = step
	}
	sb.WriteString(onbDim.Render(fmt.Sprintf("  %d%%", progress)))
	sb.WriteString("\n\n")

	// Step title
	sb.WriteString("  " + onbTitle.Render(currentStep.Title) + "\n")
	sb.WriteString("  " + onbDim.Render(currentStep.Description) + "\n\n")

	// Step content
	switch currentStep.Name {
	case "welcome":
		sb.WriteString(m.renderWelcomeStep())
	case "model_selection":
		sb.WriteString(m.renderModelSelectionStep())
	case "api_keys":
		sb.WriteString(m.renderAPIKeysStep())
	case "test_connection":
		sb.WriteString(m.renderTestConnectionStep())
	case "integrations":
		sb.WriteString(m.renderIntegrationsStep())
	case "dependencies":
		sb.WriteString(m.renderDependenciesStep())
	case "tutorial":
		sb.WriteString(m.renderTutorialStep())
	case "complete":
		sb.WriteString(m.renderCompleteStep())
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderWelcomeStep() string {
	var sb strings.Builder

	sb.WriteString("  " + onbDim.Render("This wizard will configure SoulGate in a few steps:") + "\n\n")

	items := []string{
		"Choose your AI model and provider",
		"Configure API keys",
		"Setup integrations (optional)",
		"Learn the basics",
	}
	for i, item := range items {
		sb.WriteString("  " + onbDim.Render(fmt.Sprintf("  %d. ", i+1)) + lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(item) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" to start, ") + onbCmd.Render("q") + onbDim.Render(" to skip") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderModelSelectionStep() string {
	var sb strings.Builder
	options := onboarding.GetModelOptions()

	for i, opt := range options {
		num := onbDim.Render(fmt.Sprintf("  %d ", i+1))
		name := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(opt.Name)
		desc := onbDim.Render("  " + opt.Description)

		rec := ""
		if opt.Recommended {
			rec = onbOk.Render(" recommended")
		}

		sb.WriteString("  " + num + name + rec + "\n")
		sb.WriteString("  " + desc + "\n")

		if i < len(options)-1 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("1-"+fmt.Sprintf("%d", len(options))) + onbDim.Render(" to select") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderAPIKeysStep() string {
	var sb strings.Builder
	provider := m.OnboardingState.SelectedProvider

	// Check if key already set
	envVar := ""
	envVal := ""
	url := ""
	switch provider {
	case "openai":
		envVar = "OPENAI_API_KEY"
		envVal = os.Getenv("OPENAI_API_KEY")
		url = "https://platform.openai.com/api-keys"
	case "anthropic":
		envVar = "ANTHROPIC_API_KEY"
		envVal = os.Getenv("ANTHROPIC_API_KEY")
		url = "https://console.anthropic.com/keys"
	default:
		envVar = strings.ToUpper(provider) + "_API_KEY"
		envVal = os.Getenv(envVar)
	}

	if envVal != "" {
		sb.WriteString("  " + onbOk.Render("found") + onbDim.Render("  "+envVar+" is set ("+envVal[:min(8, len(envVal))]+"...)") + "\n\n")
		sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" to continue") + "\n")
	} else {
		sb.WriteString("  " + onbWarn.Render("missing") + onbDim.Render("  "+envVar+" is not set") + "\n\n")
		sb.WriteString("  " + onbDim.Render("Get your key from: ") + onbCmd.Render(url) + "\n\n")
		sb.WriteString("  " + onbDim.Render("Set it in your shell:") + "\n")
		sb.WriteString("  " + onbCmd.Render(fmt.Sprintf("  export %s=\"your-key-here\"", envVar)) + "\n\n")
		sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" when set, ") + onbCmd.Render("s") + onbDim.Render(" to skip") + "\n")
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderTestConnectionStep() string {
	var sb strings.Builder

	sb.WriteString("  " + onbOk.Render("ok") + onbDim.Render("  API key validated") + "\n")
	sb.WriteString("  " + onbOk.Render("ok") + onbDim.Render("  Provider configured") + "\n")
	sb.WriteString("  " + onbOk.Render("ok") + onbDim.Render("  Ready to connect") + "\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderIntegrationsStep() string {
	var sb strings.Builder
	recommendations := onboarding.GetIntegrationRecommendations()

	selected := make(map[string]bool)
	for _, id := range m.OnboardingState.IntegrationsToSetup {
		selected[id] = true
	}

	for i, rec := range recommendations {
		num := onbDim.Render(fmt.Sprintf("  %d ", i+1))
		name := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(rec.Name)
		desc := onbDim.Render("  " + rec.Description)

		check := onbDim.Render("[ ] ")
		if selected[rec.ID] {
			check = onbOk.Render("[x] ")
		}

		sb.WriteString("  " + num + check + name + "\n")
		sb.WriteString("      " + desc + "\n")

		if i < len(recommendations)-1 {
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("1-4") + onbDim.Render(" to toggle, ") +
		onbCmd.Render("enter") + onbDim.Render(" to continue, ") +
		onbCmd.Render("s") + onbDim.Render(" to skip") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderDependenciesStep() string {
	var sb strings.Builder

	if len(m.OnboardingState.IntegrationsToSetup) == 0 {
		sb.WriteString("  " + onbDim.Render("No integrations selected.") + "\n\n")
		sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" to continue") + "\n")
		return sb.String()
	}

	for _, id := range m.OnboardingState.IntegrationsToSetup {
		status, ok := m.OnboardingState.DependencyStatus[id]
		if !ok {
			status = "pending"
		}

		var statusStyled string
		if strings.HasPrefix(status, "✓") || strings.HasPrefix(status, "ok") {
			statusStyled = onbOk.Render(status)
		} else if strings.HasPrefix(status, "✗") || strings.HasPrefix(status, "error") {
			statusStyled = onbBad.Render(status)
		} else {
			statusStyled = onbDim.Render(status)
		}

		name := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(id)
		sb.WriteString("  " + name + "  " + statusStyled + "\n")
	}

	if len(m.OnboardingState.DependencyErrors) > 0 {
		sb.WriteString("\n")
		for _, err := range m.OnboardingState.DependencyErrors {
			sb.WriteString("  " + onbBad.Render("! "+err) + "\n")
		}
	}

	sb.WriteString("\n")
	if m.OnboardingState.InstallingDependencies {
		sb.WriteString("  " + onbDim.Render("installing...") + "\n")
	} else {
		sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" to continue") + "\n")
	}
	return sb.String()
}

func (m *InteractiveChatModel) renderTutorialStep() string {
	var sb strings.Builder
	steps := onboarding.GetTutorialSteps()

	for _, step := range steps {
		title := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(step.Title)
		command := onbCmd.Render(step.Command)
		desc := onbDim.Render(step.Desc)

		sb.WriteString("  " + title + "\n")
		sb.WriteString("    " + command + "  " + desc + "\n\n")
	}

	sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" to finish") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderCompleteStep() string {
	var sb strings.Builder

	items := []struct {
		label  string
		status string
	}{
		{"Model", m.OnboardingState.SelectedProvider + " / " + m.OnboardingState.SelectedModel},
		{"API Key", "configured"},
		{"Workspace", "ready"},
	}

	for _, item := range items {
		sb.WriteString("  " + onbOk.Render("ok") + "  " +
			onbDim.Render(item.label+": ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(item.status) + "\n")
	}

	if len(m.OnboardingState.IntegrationsToSetup) > 0 {
		sb.WriteString("  " + onbOk.Render("ok") + "  " +
			onbDim.Render("Integrations: ") +
			lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(
				strings.Join(m.OnboardingState.IntegrationsToSetup, ", ")) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render("press ") + onbCmd.Render("enter") + onbDim.Render(" to start chatting") + "\n")
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
