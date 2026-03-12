package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/skills"
	"github.com/charmbracelet/lipgloss"
)

// View renders the main TUI view
func (m InteractiveChatModel) View() string {
	var sb strings.Builder

	// Header
	sb.WriteString(renderHeader())
	sb.WriteString("\n")

	// Thin separator
	sepWidth := m.width
	if sepWidth < 40 {
		sepWidth = 80
	}
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("236")).
		Render(strings.Repeat("─", sepWidth)))
	sb.WriteString("\n")

	// Output viewport
	sb.WriteString(m.output.View())
	sb.WriteString("\n")

	// Onboarding overlay
	if m.ShowOnboarding {
		sb.WriteString("\n")
		sb.WriteString(m.renderOnboarding())
		return sb.String()
	}

	// Setup wizard overlay
	if m.showSetupWizard {
		sb.WriteString("\n")
		sb.WriteString(m.renderSetupWizard())
		return sb.String()
	}

	// API key prompt overlay
	if m.showAPIKeyPrompt {
		sb.WriteString(m.renderAPIKeyPrompt())
		return sb.String()
	}

	// Model selector overlay
	if m.showModelSelector {
		sb.WriteString("\n")
		sb.WriteString(m.renderModelSelectorPrompt())
		return sb.String()
	}

	// Permission prompt overlay
	if m.showPermissionPrompt && m.permissionRequest != nil {
		sb.WriteString("\n")
		sb.WriteString(m.renderPermissionPrompt())
		return sb.String()
	}

	// Confirmation dialog overlay
	if m.showConfirmation {
		sb.WriteString("\n")
		sb.WriteString(m.renderConfirmation())
		return sb.String()
	}

	// Status bar
	sb.WriteString(m.renderStatusBar())
	sb.WriteString("\n")

	// Input separator
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("236")).
		Render(strings.Repeat("─", sepWidth)))
	sb.WriteString("\n")

	// Input
	sb.WriteString(m.input.View())

	// Autocomplete suggestions
	if m.showAutocomplete && len(m.autocomplete) > 0 {
		sb.WriteString("\n")
		sb.WriteString(m.renderAutocomplete())
	}

	// Hints
	sb.WriteString("\n")
	sb.WriteString(m.renderHints())

	return sb.String()
}

// renderStatusBar renders the status bar
func (m *InteractiveChatModel) renderStatusBar() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	dot := dim.Render(" · ")

	var status string
	if m.thinking {
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinner := spinners[m.spinnerFrame%len(spinners)]
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Render(spinner+" thinking...")
	} else {
		status = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			Render("ready")
	}

	model := dim.Render(fmt.Sprintf("%s:%s", m.currentProvider, m.currentModel))
	msgs := dim.Render(fmt.Sprintf("%d messages", len(m.history)))

	return "  " + status + dot + model + dot + msgs
}

// renderHints renders keyboard shortcut hints
func (m *InteractiveChatModel) renderHints() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("246"))

	return dim.Render("  ") +
		key.Render("tab") + dim.Render(" complete  ") +
		key.Render("ctrl+h") + dim.Render(" help  ") +
		key.Render("ctrl+l") + dim.Render(" clear  ") +
		key.Render("ctrl+c") + dim.Render(" exit")
}

// renderPermissionPrompt renders the permission request dialog
func (m *InteractiveChatModel) renderPermissionPrompt() string {
	if m.permissionRequest == nil {
		return ""
	}

	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	sb.WriteString("  " + title.Render("Permission Required") + "\n\n")
	sb.WriteString("  " + m.permissionRequest.Description + "\n\n")
	sb.WriteString("  " + dim.Render("Action:   ") + m.permissionRequest.Action + "\n")
	sb.WriteString("  " + dim.Render("Resource: ") + m.permissionRequest.Resource + "\n")
	if m.permissionRequest.Reason != "" {
		sb.WriteString("  " + dim.Render("Reason:   ") + m.permissionRequest.Reason + "\n")
	}
	sb.WriteString("\n")

	pattern := core.GenerateSmartPattern(m.permissionRequest.Action, m.permissionRequest.Resource)
	sb.WriteString("  " + key.Render("a") + dim.Render(" allow once   "))
	sb.WriteString(key.Render("l") + dim.Render(" learn ("+pattern+")   "))
	sb.WriteString(key.Render("d") + dim.Render(" deny") + "\n")

	return sb.String()
}

// renderConfirmation renders the confirmation dialog
func (m *InteractiveChatModel) renderConfirmation() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	sb.WriteString("  " + title.Render("Confirm") + "\n\n")
	sb.WriteString("  " + m.confirmationMessage + "\n")
	sb.WriteString("  " + dim.Render("Command: ") + cmd.Render(m.pendingCommand) + "\n\n")
	sb.WriteString("  " + key.Render("y") + dim.Render(" yes   ") + key.Render("n") + dim.Render(" no") + "\n")

	return sb.String()
}

// renderStatus renders the status information
func (m *InteractiveChatModel) renderStatus() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	val := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Status") + "\n\n")
	sb.WriteString("  " + dim.Render("Provider  ") + val.Render(m.currentProvider) + "\n")
	sb.WriteString("  " + dim.Render("Model     ") + val.Render(m.currentModel) + "\n")
	sb.WriteString("  " + dim.Render("Status    ") + lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("ready") + "\n")

	return sb.String()
}

// renderDebugInfo renders debug information
func (m *InteractiveChatModel) renderDebugInfo() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	val := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	bad := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Debug") + "\n\n")

	sb.WriteString("  " + dim.Render("Model") + "\n")
	sb.WriteString("    " + dim.Render("Provider: ") + val.Render(m.currentProvider) + "\n")
	sb.WriteString("    " + dim.Render("Model:    ") + val.Render(m.currentModel) + "\n\n")

	sb.WriteString("  " + dim.Render("API Keys") + "\n")
	for _, env := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		v := os.Getenv(env)
		if v != "" {
			sb.WriteString("    " + dim.Render(env+": ") + ok.Render("set ("+v[:8]+"...)") + "\n")
		} else {
			sb.WriteString("    " + dim.Render(env+": ") + bad.Render("not set") + "\n")
		}
	}
	sb.WriteString("\n")

	cwd, _ := os.Getwd()
	sb.WriteString("  " + dim.Render("Environment") + "\n")
	sb.WriteString("    " + dim.Render("Working Dir: ") + val.Render(cwd) + "\n")
	sb.WriteString("    " + dim.Render("Config Dir:  ") + val.Render("~/.soulgate/") + "\n")

	return sb.String()
}

// renderSkillsList renders available skills
func (m *InteractiveChatModel) renderSkillsList() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	name := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	workspace := m.orch.GetWorkspace()
	skillsDir := filepath.Join(workspace.Root, workspace.Config.Skills.Dir)
	loader := skills.NewLoader(skillsDir)

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Skills") + "\n\n")

	skillIDs, err := loader.ListSkills()
	if err != nil || len(skillIDs) == 0 {
		sb.WriteString("  " + dim.Render("No skills found.") + "\n\n")
		sb.WriteString("  " + dim.Render("Create one: soulgate skills create <name>") + "\n")
		sb.WriteString("  " + dim.Render("Directory:  "+skillsDir) + "\n")
		return sb.String()
	}

	loadedSkills, _ := loader.LoadByIDs(skillIDs)
	enabled := workspace.Config.Skills.EnabledSkills

	for _, skill := range loadedSkills {
		status := ok.Render("active")
		if len(enabled) > 0 {
			isEnabled := false
			for _, e := range enabled {
				if e == skill.ID {
					isEnabled = true
					break
				}
			}
			if !isEnabled {
				status = dim.Render("disabled")
			}
		}

		sb.WriteString("  " + name.Render(skill.Name) + "  " + status + "\n")
		if skill.Description != "" {
			desc := skill.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			sb.WriteString("  " + dim.Render(desc) + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderMemoryList renders memory entries
func (m *InteractiveChatModel) renderMemoryList() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Memory") + "\n\n")

	memories := m.orch.GetMemoryStore().List()
	if len(memories) == 0 {
		sb.WriteString("  " + dim.Render("No memory entries yet.") + "\n")
		sb.WriteString("  " + dim.Render("The AI will save memories during conversation.") + "\n")
		return sb.String()
	}

	count := 0
	for _, entry := range memories {
		if count >= 15 {
			sb.WriteString("  " + dim.Render(fmt.Sprintf("... and %d more", len(memories)-count)) + "\n")
			break
		}
		value := entry.Value
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		sb.WriteString("  " + key.Render(entry.Key) + "  " + dim.Render(value) + "\n")
		count++
	}

	return sb.String()
}

// renderSoulInfo shows the current soul configuration
func (m *InteractiveChatModel) renderSoulInfo() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	section := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("AI Persona") + dim.Render("  SOUL.md") + "\n\n")

	workspace := m.orch.GetWorkspace()
	soul, err := core.LoadSoulConfig(workspace.ConfigDir)
	if err != nil || soul == nil {
		sb.WriteString("  " + dim.Render("No SOUL.md configured.") + "\n\n")
		sb.WriteString("  " + dim.Render("Create one with /soul init") + "\n")
		sb.WriteString("  " + dim.Render("Defines personality, style, and boundaries.") + "\n")
		return sb.String()
	}

	sections := core.ParseSoulSections(soul.Content)
	for name, content := range sections {
		sb.WriteString("  " + section.Render(name) + "\n")
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if i >= 3 {
				sb.WriteString("  " + dim.Render("  ...") + "\n")
				break
			}
			if strings.TrimSpace(line) != "" {
				sb.WriteString("  " + dim.Render("  "+line) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  " + dim.Render("Path: "+soul.Path) + "\n")
	return sb.String()
}

// initSoul creates a default SOUL.md
func (m *InteractiveChatModel) initSoul() string {
	workspace := m.orch.GetWorkspace()
	if err := core.CreateSoulFile(workspace.ConfigDir); err != nil {
		return colorError("  Error: " + err.Error())
	}
	return colorSuccess("  Created SOUL.md at " + workspace.ConfigDir + "/SOUL.md")
}

// resetSoul resets SOUL.md to defaults
func (m *InteractiveChatModel) resetSoul() string {
	workspace := m.orch.GetWorkspace()
	if err := core.UpdateSoulFile(workspace.ConfigDir, core.DefaultSoulTemplate()); err != nil {
		return colorError("  Error: " + err.Error())
	}
	return colorSuccess("  SOUL.md reset to default template.")
}

// renderScheduleInfo shows scheduled tasks
func (m *InteractiveChatModel) renderScheduleInfo() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Scheduled Tasks") + "\n\n")
	sb.WriteString("  " + dim.Render("No active schedules.") + "\n\n")
	sb.WriteString("  " + dim.Render("Add via CLI:") + "\n")
	sb.WriteString("  " + cmd.Render("soulgate schedule add --type skill --target review --interval 1h") + "\n")
	sb.WriteString("  " + cmd.Render("soulgate schedule add --type prompt --target \"check status\" --interval 30m") + "\n")

	return sb.String()
}

// renderAutocomplete renders autocomplete suggestions with a scrolling window
func (m *InteractiveChatModel) renderAutocomplete() string {
	total := len(m.autocomplete)
	if total == 0 {
		return ""
	}

	var sb strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	hl := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)

	maxVisible := 12
	if total <= maxVisible {
		// Show all items
		for i, suggestion := range m.autocomplete {
			if i == m.autocompleteIndex {
				sb.WriteString(hl.Render("  > " + suggestion))
			} else {
				sb.WriteString(dim.Render("    " + suggestion))
			}
			sb.WriteString("\n")
		}
	} else {
		// Scrolling window that follows the selection
		start := m.autocompleteIndex - maxVisible/2
		if start < 0 {
			start = 0
		}
		end := start + maxVisible
		if end > total {
			end = total
			start = end - maxVisible
		}

		if start > 0 {
			sb.WriteString(dim.Render(fmt.Sprintf("    ... %d above", start)))
			sb.WriteString("\n")
		}

		for i := start; i < end; i++ {
			if i == m.autocompleteIndex {
				sb.WriteString(hl.Render("  > " + m.autocomplete[i]))
			} else {
				sb.WriteString(dim.Render("    " + m.autocomplete[i]))
			}
			sb.WriteString("\n")
		}

		if end < total {
			sb.WriteString(dim.Render(fmt.Sprintf("    ... %d below", total-end)))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
