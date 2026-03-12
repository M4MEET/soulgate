package tui

import (
	"fmt"
	"strings"

	"github.com/M4MEET/soulgate/internal/ui/setup"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Setup wizard
// These functions handle the interactive setup wizard for integrations

// handleSetupWizardInput handles input during setup wizard
func (m *InteractiveChatModel) handleSetupWizardInput(key string) (InteractiveChatModel, tea.Cmd) {
	wizard := setup.NewWizard(m.orch.GetWorkspace())
	integrations := wizard.GetAvailableIntegrations()

	switch m.setupStep {
	case 0: // Integration selection
		switch key {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			number := int(key[0] - '0')
			if number > 0 && number <= len(integrations) {
				m.setupIntegrationID = integrations[number-1].ID
				m.setupStep = 1
				m.setupCurrentField = 0
				m.addMessage(colorSuccess(fmt.Sprintf("Selected: %s %s", integrations[number-1].Icon, integrations[number-1].Name)))
			}
			return *m, nil

		case "esc", "q", "Q":
			m.showSetupWizard = false
			m.addMessage(colorMuted("Setup cancelled"))
			return *m, nil
		}

	case 1: // Field input (handled by text input)
		// Let the normal input handler capture the value
		// When user presses Enter, it will be processed in handleCommand
		return *m, nil
	}

	return *m, nil
}

// renderSetupWizard renders the setup wizard UI
func (m *InteractiveChatModel) renderSetupWizard() string {
	wizard := setup.NewWizard(m.orch.GetWorkspace())
	integrations := wizard.GetAvailableIntegrations()

	var sb strings.Builder

	if m.setupStep == 0 {
		// Step 1: Select integration
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true).
			Render("  ╭─ 🔧 Setup Wizard - Select Integration "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render(strings.Repeat("─", 16)))
		sb.WriteString("\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │") + "\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ ") +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render("Configure integrations with ease!") + "\n")

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │") + "\n")

		// Group by category
		categories := make(map[string][]setup.IntegrationSetup)
		for _, integ := range integrations {
			categories[integ.Category] = append(categories[integ.Category], integ)
		}

		num := 1
		for _, category := range []string{"Communication", "Development", "Productivity", "Project Management", "Cloud"} {
			if integrationsInCat, ok := categories[category]; ok && len(integrationsInCat) > 0 {
				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("45")).
						Bold(true).
						Render(category+":") + "\n")

				for _, integ := range integrationsInCat {
					sb.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("208")).
						Render("  │ ") +
						lipgloss.NewStyle().
							Foreground(lipgloss.Color("214")).
							Bold(true).
							Render(fmt.Sprintf(" %d. ", num)) +
						lipgloss.NewStyle().
							Foreground(lipgloss.Color("220")).
							Render(integ.Icon+" "+integ.Name) + "\n")

					sb.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("208")).
						Render("  │    ") +
						lipgloss.NewStyle().
							Foreground(lipgloss.Color("244")).
							Italic(true).
							Render(integ.Description) + "\n")

					num++
				}
				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │") + "\n")
			}
		}

		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render("  │ ") +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render("Type a number to select, or ") +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Render("Esc") +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render(" to cancel") + "\n")

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

	} else if m.setupStep == 1 {
		// Step 2: Enter field values
		var selectedIntegration *setup.IntegrationSetup
		for _, integ := range integrations {
			if integ.ID == m.setupIntegrationID {
				selectedIntegration = &integ
				break
			}
		}

		if selectedIntegration != nil {
			sb.WriteString("\n")
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Bold(true).
				Render(fmt.Sprintf("  ╭─ 🔧 Setup: %s %s ", selectedIntegration.Icon, selectedIntegration.Name)))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Render(strings.Repeat("─", 20))+ "\n")

			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Render("  │") + "\n")

			// Show current field
			if m.setupCurrentField < len(selectedIntegration.Fields) {
				field := selectedIntegration.Fields[m.setupCurrentField]

				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("220")).
						Bold(true).
						Render(field.Label) +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("196")).
						Render(func() string {
							if field.Required {
								return " *"
							}
							return ""
						}()) + "\n")

				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("244")).
						Italic(true).
						Render(field.Description) + "\n")

				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │") + "\n")

				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("244")).
						Render("Type your value and press Enter") + "\n")

				if field.Default != "" {
					sb.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("208")).
						Render("  │ ") +
						lipgloss.NewStyle().
							Foreground(lipgloss.Color("244")).
							Render(fmt.Sprintf("(Default: %s)", field.Default)) + "\n")
				}
			} else {
				// All fields collected - show summary
				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("82")).
						Bold(true).
						Render("✓ All fields collected!") + "\n")

				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │") + "\n")

				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Render("  │ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("244")).
						Render("Testing connection...") + "\n")
			}

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
		}
	}

	return sb.String()
}
