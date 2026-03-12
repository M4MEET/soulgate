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
				m.addMessage(colorSuccess(fmt.Sprintf("Selected: %s", integrations[number-1].Name)))
			}
			return *m, nil

		case "esc", "q", "Q":
			m.showSetupWizard = false
			m.addMessage(colorMuted("Setup cancelled"))
			return *m, nil
		}

	case 1: // Field input (handled by text input)
		return *m, nil
	}

	return *m, nil
}

// renderSetupWizard renders the setup wizard UI
func (m *InteractiveChatModel) renderSetupWizard() string {
	wizard := setup.NewWizard(m.orch.GetWorkspace())
	integrations := wizard.GetAvailableIntegrations()

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	name := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cat := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)

	var sb strings.Builder

	if m.setupStep == 0 {
		// Step 1: Select integration
		sb.WriteString("\n")
		sb.WriteString("  " + title.Render("Setup Wizard") + dim.Render(" - Select Integration") + "\n\n")

		// Group by category
		categories := make(map[string][]setup.IntegrationSetup)
		for _, integ := range integrations {
			categories[integ.Category] = append(categories[integ.Category], integ)
		}

		num := 1
		for _, category := range []string{"Communication", "Development", "Productivity", "Project Management", "Cloud"} {
			if integrationsInCat, ok := categories[category]; ok && len(integrationsInCat) > 0 {
				sb.WriteString("  " + cat.Render(category) + "\n")

				for _, integ := range integrationsInCat {
					sb.WriteString("  " + dim.Render(fmt.Sprintf("  %d ", num)) + name.Render(integ.Name) + "\n")
					sb.WriteString("  " + dim.Render("    "+integ.Description) + "\n")
					num++
				}
				sb.WriteString("\n")
			}
		}

		sb.WriteString("  " + dim.Render("press ") + cmd.Render("1-"+fmt.Sprintf("%d", num-1)) + dim.Render(" to select, ") + cmd.Render("esc") + dim.Render(" to cancel") + "\n")

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
			sb.WriteString("  " + title.Render("Setup") + dim.Render(" - "+selectedIntegration.Name) + "\n\n")

			// Show current field
			if m.setupCurrentField < len(selectedIntegration.Fields) {
				field := selectedIntegration.Fields[m.setupCurrentField]

				req := ""
				if field.Required {
					req = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(" *")
				}

				sb.WriteString("  " + name.Render(field.Label) + req + "\n")
				sb.WriteString("  " + dim.Render(field.Description) + "\n\n")
				sb.WriteString("  " + dim.Render("Type your value and press Enter") + "\n")

				if field.Default != "" {
					sb.WriteString("  " + dim.Render(fmt.Sprintf("(Default: %s)", field.Default)) + "\n")
				}
			} else {
				// All fields collected
				ok := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
				sb.WriteString("  " + ok.Render("ok") + dim.Render("  All fields collected") + "\n\n")
				sb.WriteString("  " + dim.Render("Testing connection...") + "\n")
			}
		}
	}

	return sb.String()
}
