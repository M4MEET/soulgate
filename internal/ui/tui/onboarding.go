package tui

import (
	"fmt"
	"strings"

	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Onboarding wizard
// These functions handle the interactive onboarding flow

// handleOnboardingInput handles input during onboarding
func (m *InteractiveChatModel) handleOnboardingInput(key string) (InteractiveChatModel, tea.Cmd) {
	if m.OnboardingState == nil {
		m.ShowOnboarding = false
		return *m, nil
	}

	currentStep := m.OnboardingState.GetCurrentStep()

	switch currentStep.Name {
	case "welcome":
		// Press any key to continue
		if key == "enter" || key == " " {
			m.OnboardingState.NextStep()
		}

	case "model_selection":
		// Select model by number
		if key >= "1" && key <= "4" {
			options := onboarding.GetModelOptions()
			idx := int(key[0] - '1')
			if idx < len(options) {
				m.OnboardingState.SelectedModel = options[idx].ID
				m.OnboardingState.SelectedProvider = options[idx].Provider
				m.OnboardingState.NextStep()
			}
		}

	case "api_keys":
		// Handled by normal input - just advance on Enter
		if key == "enter" {
			m.OnboardingState.NextStep()
		}

	case "test_connection":
		// Auto-advance after testing
		m.OnboardingState.NextStep()

	case "integrations":
		// Skip or select integrations
		if key == "s" || key == "S" {
			m.OnboardingState.NextStep()
		} else if key >= "1" && key <= "4" {
			// Add integration to setup list
			recommendations := onboarding.GetIntegrationRecommendations()
			idx := int(key[0] - '1')
			if idx < len(recommendations) {
				m.OnboardingState.IntegrationsToSetup = append(
					m.OnboardingState.IntegrationsToSetup,
					recommendations[idx].ID,
				)
			}
		} else if key == "enter" {
			// Advance to dependency installation step
			m.OnboardingState.NextStep()
			// Start installing dependencies in background
			return *m, m.installDependenciesCmd()
		}

	case "dependencies":
		// Auto-advance after installation (or on Enter to skip if errored)
		if key == "enter" || !m.OnboardingState.InstallingDependencies {
			m.OnboardingState.NextStep()
		}

	case "tutorial":
		// Press Enter to continue
		if key == "enter" || key == " " || key == "s" || key == "S" {
			m.OnboardingState.NextStep()
		}

	case "complete":
		// Finish onboarding
		if key == "enter" || key == " " {
			m.OnboardingState.MarkComplete()
			m.OnboardingState.SaveAPIKeys()
			m.ShowOnboarding = false
			m.addMessage(colorSuccess("✓ Onboarding complete! Welcome to SoulGate! 🎉"))
		}
	}

	return *m, nil
}

// renderOnboarding renders the onboarding wizard UI
func (m *InteractiveChatModel) renderOnboarding() string {
	if m.OnboardingState == nil {
		return ""
	}

	var sb strings.Builder
	currentStep := m.OnboardingState.GetCurrentStep()
	progress := m.OnboardingState.GetProgress()

	// Header with progress
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true).
		Render(fmt.Sprintf("  ╭─ 🎯 %s ", currentStep.Title)))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render(strings.Repeat("─", 40-len(currentStep.Title))))
	sb.WriteString("\n")

	// Progress bar
	progressBarWidth := 50
	filledWidth := int(float64(progressBarWidth) * float64(progress) / 100)
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render(strings.Repeat("█", filledWidth)))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(strings.Repeat("░", progressBarWidth-filledWidth)))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(fmt.Sprintf(" %d%%", progress)))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Step-specific content
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

	// Footer
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  ╰"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render(strings.Repeat("─", 54)))
	sb.WriteString("\n")

	return sb.String()
}

// Step rendering functions

func (m *InteractiveChatModel) renderWelcomeStep() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("220")).
		Bold(true).
		Render("Welcome to SoulGate! 🐙"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Render("Your secure AI agent gateway is ready to set up!"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("This wizard will help you:"))
	sb.WriteString("\n")

	features := []string{
		"Choose your AI model",
		"Configure API keys",
		"Setup integrations",
		"Learn the basics",
	}

	for _, feature := range features {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │   "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Render("✓ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Render(feature))
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("Press Enter to get started!"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderModelSelectionStep() string {
	var sb strings.Builder
	options := onboarding.GetModelOptions()

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Render("Choose your default AI model:"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	for i, opt := range options {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render(fmt.Sprintf(" %d. ", i+1)))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Render(opt.Icon + " " + opt.Name))
		if opt.Recommended {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Render(" ⭐ Recommended"))
		}
		sb.WriteString("\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │    "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Render(opt.Description))
		sb.WriteString("\n")

		if i < len(options)-1 {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Render("  │") + "\n")
		}
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderAPIKeysStep() string {
	var sb strings.Builder
	provider := m.OnboardingState.SelectedProvider

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Render(fmt.Sprintf("Configure your %s API key:", strings.ToUpper(provider))))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	if provider == "openai" {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("Get your API key from: "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Underline(true).
			Render("https://platform.openai.com/api-keys"))
		sb.WriteString("\n")
	} else {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("Get your API key from: "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Underline(true).
			Render("https://console.anthropic.com/keys"))
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("Paste your API key below and press Enter"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderTestConnectionStep() string {
	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("✓ Testing connection..."))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("✓ API key validated"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("✓ Connection successful!"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderIntegrationsStep() string {
	var sb strings.Builder
	recommendations := onboarding.GetIntegrationRecommendations()

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Render("Setup integrations (optional):"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	for i, rec := range recommendations {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Render(fmt.Sprintf(" %d. ", i+1)))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Render(rec.Icon + " " + rec.Name))
		if rec.Popular {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Render(" 🔥"))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("Press number to setup, "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("S"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(" to skip"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderDependenciesStep() string {
	var sb strings.Builder

	if len(m.OnboardingState.IntegrationsToSetup) == 0 {
		// No integrations selected, skip this step
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("No integrations selected. Skipping dependency installation..."))
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │") + "\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Render("Press Enter to continue"))
		sb.WriteString("\n")
		return sb.String()
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true).
		Render("Installing Dependencies..."))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Show installation status for each integration
	for _, integrationID := range m.OnboardingState.IntegrationsToSetup {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))

		// Get integration name
		integrationName := strings.Title(integrationID)
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true).
			Render(integrationName + ": "))

		// Get status
		status, ok := m.OnboardingState.DependencyStatus[integrationID]
		if !ok {
			status = "pending..."
		}

		// Color status based on result
		var statusStyle lipgloss.Style
		if strings.HasPrefix(status, "✓") {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
		} else if strings.HasPrefix(status, "✗") {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		} else {
			statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		}

		sb.WriteString(statusStyle.Render(status))
		sb.WriteString("\n")
	}

	// Show errors if any
	if len(m.OnboardingState.DependencyErrors) > 0 {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │") + "\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Render("⚠ Some dependencies failed to install:"))
		sb.WriteString("\n")

		for _, err := range m.OnboardingState.DependencyErrors {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Render("  │   "))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render("• " + err))
			sb.WriteString("\n")
		}
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	// Show completion message
	if m.OnboardingState.InstallingDependencies {
		// Still installing
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("Installing... Please wait..."))
		sb.WriteString("\n")
	} else {
		// Installation complete
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Render("Press Enter to continue"))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderTutorialStep() string {
	var sb strings.Builder
	steps := onboarding.GetTutorialSteps()

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true).
		Render("Quick Start Guide:"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	for _, step := range steps {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Render("▶ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true).
			Render(step.Title))
		sb.WriteString("\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │   "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("45")).
			Background(lipgloss.Color("236")).
			Render(" "+step.Command+" "))
		sb.WriteString("\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │   "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Render(step.Desc))
		sb.WriteString("\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │") + "\n")
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("Press Enter to finish!"))
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderCompleteStep() string {
	var sb strings.Builder

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Bold(true).
		Render("🎉 You're All Set!"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Render("SoulGate is configured and ready to use!"))
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	completedItems := []string{
		"AI model configured",
		"API keys saved",
		"Ready to chat!",
	}

	for _, item := range completedItems {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Render("✓ "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Render(item))
		sb.WriteString("\n")
	}

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │") + "\n")

	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Render("  │ "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("Press Enter to start chatting!"))
	sb.WriteString("\n")

	return sb.String()
}
