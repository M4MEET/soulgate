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

// View rendering methods
// These methods render the main view and various UI overlays

// View renders the main TUI view
func (m InteractiveChatModel) View() string {
	var sb strings.Builder

	// Header (fixed height: 3 lines)
	sb.WriteString(renderHeader())
	sb.WriteString("\n\n")

	// Output viewport
	sb.WriteString(m.output.View())
	sb.WriteString("\n\n")

	// Onboarding (absolute highest priority overlay)
	if m.ShowOnboarding {
		sb.WriteString("\n")
		sb.WriteString(m.renderOnboarding())
		return sb.String()
	}

	// Setup wizard (highest priority overlay)
	if m.showSetupWizard {
		sb.WriteString("\n")
		sb.WriteString(m.renderSetupWizard())
		return sb.String()
	}

	// API key prompt (high priority overlay)
	if m.showAPIKeyPrompt {
		sb.WriteString(m.renderAPIKeyPrompt())
		return sb.String()
	}

	// Model selector (highest priority overlay)
	if m.showModelSelector {
		sb.WriteString("\n")
		sb.WriteString(m.renderModelSelectorPrompt())
		return sb.String()
	}

	// Permission prompt (highest priority overlay)
	if m.showPermissionPrompt && m.permissionRequest != nil {
		sb.WriteString("\n")
		sb.WriteString(m.renderPermissionPrompt())
		return sb.String()
	}

	// Confirmation dialog (overlays everything if shown)
	if m.showConfirmation {
		sb.WriteString("\n")
		sb.WriteString(m.renderConfirmation())
		return sb.String()
	}

	// Status bar
	sb.WriteString(m.renderStatusBar())
	sb.WriteString("\n")

	// Input separator for better visibility
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", 100)))
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

// renderStatusBar renders the status bar with current state
func (m *InteractiveChatModel) renderStatusBar() string {
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Padding(0, 1)

	status := "Ready"
	statusColor := "82" // Green

	if m.thinking {
		// Animated spinner frames
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinner := spinners[m.spinnerFrame%len(spinners)]
		status = fmt.Sprintf("%s Thinking", spinner)
		statusColor = "208" // Orange
	}

	// Show current model in status bar with gradient effect
	modelInfo := fmt.Sprintf("%s:%s", m.currentProvider, m.currentModel)

	// Create colored status
	coloredStatus := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor)).
		Bold(true).
		Render(status)

	return statusStyle.Render(fmt.Sprintf("%s  •  Model: %s  •  Messages: %d", coloredStatus, modelInfo, len(m.history)))
}

// renderHints renders keyboard shortcut hints
func (m *InteractiveChatModel) renderHints() string {
	var sb strings.Builder

	// Build hints with color-coded shortcuts
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("  "))

	// History
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("45")).
		Render("↑↓"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(": History  •  "))

	// Navigate
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("45")).
		Render("←→"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(": Navigate  •  "))

	// Complete
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("Tab"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(": Complete  •  "))

	// Help
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Render("Ctrl+H"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(": Help  •  "))

	// Clear
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Render("Ctrl+L"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(": Clear  •  "))

	// Exit
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Render("Ctrl+C"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(": Exit"))

	return sb.String()
}

// renderPermissionPrompt renders the permission request dialog
func (m *InteractiveChatModel) renderPermissionPrompt() string {
	if m.permissionRequest == nil {
		return ""
	}

	var sb strings.Builder

	// Permission box
	sb.WriteString("\n")
	sb.WriteString(colorInfo("  ╭─ 🔐 Permission Required "))
	sb.WriteString(colorInfo(strings.Repeat("─", 27)))
	sb.WriteString("\n")
	sb.WriteString(colorInfo("  │") + "\n")
	sb.WriteString(colorInfo("  │  ") + colorBold("The AI wants to:") + "\n")
	sb.WriteString(colorInfo("  │  ") + "→ " + m.permissionRequest.Description + "\n")
	sb.WriteString(colorInfo("  │") + "\n")
	sb.WriteString(colorInfo("  │  ") + colorMuted("Action: ") + m.permissionRequest.Action + "\n")
	sb.WriteString(colorInfo("  │  ") + colorMuted("Resource: ") + m.permissionRequest.Resource + "\n")
	if m.permissionRequest.Reason != "" {
		sb.WriteString(colorInfo("  │  ") + colorMuted("Reason: ") + m.permissionRequest.Reason + "\n")
	}
	sb.WriteString(colorInfo("  │") + "\n")

	// Show what pattern will be learned
	pattern := core.GenerateSmartPattern(m.permissionRequest.Action, m.permissionRequest.Resource)
	sb.WriteString(colorInfo("  │  ") + colorBold("Grant permission?") + "\n")
	sb.WriteString(colorInfo("  │  ") + colorSuccess("(A)") + " Allow Once\n")
	sb.WriteString(colorInfo("  │  ") + colorAccentBright("(L)") + " Learn & Always Allow " + colorMuted("(saves: "+pattern+")") + "\n")
	sb.WriteString(colorInfo("  │  ") + colorError("(D)") + " Deny\n")
	sb.WriteString(colorInfo("  │") + "\n")
	sb.WriteString(colorInfo("  ╰"))
	sb.WriteString(colorInfo(strings.Repeat("─", 54)))
	sb.WriteString("\n")

	return sb.String()
}

// renderConfirmation renders the confirmation dialog for sensitive commands
func (m *InteractiveChatModel) renderConfirmation() string {
	var sb strings.Builder

	// Warning box
	sb.WriteString("\n")
	sb.WriteString(colorWarn("  ╭─ ⚠  Confirmation Required "))
	sb.WriteString(colorWarn(strings.Repeat("─", 25)))
	sb.WriteString("\n")
	sb.WriteString(colorWarn("  │") + "\n")
	sb.WriteString(colorWarn("  │  ") + m.confirmationMessage + "\n")
	sb.WriteString(colorWarn("  │") + "\n")
	sb.WriteString(colorWarn("  │  ") + colorBold("Command: ") + colorAccent(m.pendingCommand) + "\n")
	sb.WriteString(colorWarn("  │") + "\n")
	sb.WriteString(colorWarn("  │  ") + colorBold("Are you sure? ") + colorSuccess("(Y)es") + " / " + colorError("(N)o") + "\n")
	sb.WriteString(colorWarn("  │") + "\n")
	sb.WriteString(colorWarn("  ╰"))
	sb.WriteString(colorWarn(strings.Repeat("─", 54)))
	sb.WriteString("\n")

	return sb.String()
}

// renderStatus renders the status information screen
func (m *InteractiveChatModel) renderStatus() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ Status ───────────────────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " Provider: " + colorAccent(m.currentProvider) + "\n")
	sb.WriteString(colorAccent("│") + " Model: " + colorMuted(m.currentModel) + "\n")
	sb.WriteString(colorAccent("│") + " Status: " + colorSuccess("Ready") + "\n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderDebugInfo renders debug information about the environment
func (m *InteractiveChatModel) renderDebugInfo() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ Debug Information ────────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " \n")

	// Current model
	sb.WriteString(colorAccent("│") + " " + colorBold("Current Model:") + "\n")
	sb.WriteString(colorAccent("│") + fmt.Sprintf("   Provider: %s\n", m.currentProvider))
	sb.WriteString(colorAccent("│") + fmt.Sprintf("   Model: %s\n", m.currentModel))
	sb.WriteString(colorAccent("│") + " \n")

	// API Keys
	sb.WriteString(colorAccent("│") + " " + colorBold("API Keys:") + "\n")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		sb.WriteString(colorAccent("│") + "   OPENAI_API_KEY: " + colorSuccess("✓ Set ("+openaiKey[:8]+"...)") + "\n")
	} else {
		sb.WriteString(colorAccent("│") + "   OPENAI_API_KEY: " + colorError("✗ Not set") + "\n")
	}

	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey != "" {
		sb.WriteString(colorAccent("│") + "   ANTHROPIC_API_KEY: " + colorSuccess("✓ Set ("+anthropicKey[:8]+"...)") + "\n")
	} else {
		sb.WriteString(colorAccent("│") + "   ANTHROPIC_API_KEY: " + colorError("✗ Not set") + "\n")
	}
	sb.WriteString(colorAccent("│") + " \n")

	// Working directory
	cwd, _ := os.Getwd()
	sb.WriteString(colorAccent("│") + " " + colorBold("Environment:") + "\n")
	sb.WriteString(colorAccent("│") + fmt.Sprintf("   Working Dir: %s\n", cwd))
	sb.WriteString(colorAccent("│") + fmt.Sprintf("   Config Dir: ~/.soulgate/\n"))
	sb.WriteString(colorAccent("│") + " \n")

	// Quick fixes
	sb.WriteString(colorAccent("│") + " " + colorBold("Quick Fixes:") + "\n")
	if openaiKey == "" {
		sb.WriteString(colorAccent("│") + "   " + colorMuted("export OPENAI_API_KEY='sk-...'") + "\n")
	}
	if anthropicKey == "" {
		sb.WriteString(colorAccent("│") + "   " + colorMuted("export ANTHROPIC_API_KEY='sk-ant-...'") + "\n")
	}
	sb.WriteString(colorAccent("│") + " \n")

	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderSkillsList renders available skills
func (m *InteractiveChatModel) renderSkillsList() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("+----- Skills -------------------------------------------+\n"))

	// Load skills from workspace
	workspace := m.orch.GetWorkspace()
	skillsDir := filepath.Join(workspace.Root, workspace.Config.Skills.Dir)
	loader := skills.NewLoader(skillsDir)

	skillIDs, err := loader.ListSkills()
	if err != nil || len(skillIDs) == 0 {
		sb.WriteString(colorAccent("|") + " No skills found.\n")
		sb.WriteString(colorAccent("|") + "\n")
		sb.WriteString(colorAccent("|") + " Create skills:\n")
		sb.WriteString(colorAccent("|") + "   soulgate skills create <name>\n")
		sb.WriteString(colorAccent("|") + "\n")
		sb.WriteString(colorAccent("|") + " Skills directory: " + colorMuted(skillsDir) + "\n")
	} else {
		sb.WriteString(colorAccent("|") + fmt.Sprintf(" Available Skills (%d):\n", len(skillIDs)))
		sb.WriteString(colorAccent("|") + "\n")

		loadedSkills, _ := loader.LoadByIDs(skillIDs)
		enabled := workspace.Config.Skills.EnabledSkills

		for _, skill := range loadedSkills {
			status := colorSuccess("active")
			if len(enabled) > 0 {
				isEnabled := false
				for _, e := range enabled {
					if e == skill.ID {
						isEnabled = true
						break
					}
				}
				if !isEnabled {
					status = colorMuted("disabled")
				}
			}

			sb.WriteString(colorAccent("|") + fmt.Sprintf("   %s [%s]\n", colorBold(skill.Name), status))
			if skill.Description != "" {
				desc := skill.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				sb.WriteString(colorAccent("|") + fmt.Sprintf("     %s\n", colorMuted(desc)))
			}
		}
	}
	sb.WriteString(colorAccentBright("+-------------------------------------------------------+"))
	return sb.String()
}

// renderMemoryList renders memory entries
func (m *InteractiveChatModel) renderMemoryList() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("+----- Memory -------------------------------------------+\n"))

	memories := m.orch.GetMemoryStore().List()
	if len(memories) == 0 {
		sb.WriteString(colorAccent("|") + " No memory entries yet.\n")
		sb.WriteString(colorAccent("|") + "\n")
		sb.WriteString(colorAccent("|") + " The AI will save memories automatically, or you can ask it to\n")
		sb.WriteString(colorAccent("|") + " remember things during conversation.\n")
	} else {
		sb.WriteString(colorAccent("|") + fmt.Sprintf(" Memory Entries (%d):\n", len(memories)))
		sb.WriteString(colorAccent("|") + "\n")
		count := 0
		for _, entry := range memories {
			if count >= 15 {
				sb.WriteString(colorAccent("|") + fmt.Sprintf("   ... and %d more\n", len(memories)-count))
				break
			}
			value := entry.Value
			if len(value) > 50 {
				value = value[:47] + "..."
			}
			sb.WriteString(colorAccent("|") + fmt.Sprintf("   %s = %s\n", colorBold(entry.Key), colorMuted(value)))
			count++
		}
	}
	sb.WriteString(colorAccentBright("+-------------------------------------------------------+"))
	return sb.String()
}

// renderSoulInfo shows the current soul configuration
func (m *InteractiveChatModel) renderSoulInfo() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("+----- AI Persona (SOUL.md) ----------------------------+\n"))

	workspace := m.orch.GetWorkspace()
	soul, err := core.LoadSoulConfig(workspace.ConfigDir)
	if err != nil || soul == nil {
		sb.WriteString(colorAccent("|") + " No SOUL.md configured.\n")
		sb.WriteString(colorAccent("|") + "\n")
		sb.WriteString(colorAccent("|") + " Create one with: /soul init\n")
		sb.WriteString(colorAccent("|") + "\n")
		sb.WriteString(colorAccent("|") + " SOUL.md defines your AI's persona, personality,\n")
		sb.WriteString(colorAccent("|") + " communication style, and behavior boundaries.\n")
	} else {
		sections := core.ParseSoulSections(soul.Content)
		for name, content := range sections {
			sb.WriteString(colorAccent("|") + " " + colorBold(name) + "\n")
			lines := strings.Split(content, "\n")
			for i, line := range lines {
				if i >= 3 {
					sb.WriteString(colorAccent("|") + "   " + colorMuted("...") + "\n")
					break
				}
				if strings.TrimSpace(line) != "" {
					sb.WriteString(colorAccent("|") + "   " + colorMuted(line) + "\n")
				}
			}
		}
		sb.WriteString(colorAccent("|") + "\n")
		sb.WriteString(colorAccent("|") + " Path: " + colorMuted(soul.Path) + "\n")
	}
	sb.WriteString(colorAccentBright("+-------------------------------------------------------+"))
	return sb.String()
}

// initSoul creates a default SOUL.md
func (m *InteractiveChatModel) initSoul() string {
	workspace := m.orch.GetWorkspace()
	if err := core.CreateSoulFile(workspace.ConfigDir); err != nil {
		return colorError("Error: " + err.Error())
	}
	return colorSuccess("Created SOUL.md! Edit it to customize your AI's persona.\nPath: " + workspace.ConfigDir + "/SOUL.md")
}

// resetSoul resets SOUL.md to defaults
func (m *InteractiveChatModel) resetSoul() string {
	workspace := m.orch.GetWorkspace()
	if err := core.UpdateSoulFile(workspace.ConfigDir, core.DefaultSoulTemplate()); err != nil {
		return colorError("Error: " + err.Error())
	}
	return colorSuccess("SOUL.md reset to default template.")
}

// renderScheduleInfo shows scheduled tasks
func (m *InteractiveChatModel) renderScheduleInfo() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("+----- Scheduled Tasks ---------------------------------+\n"))
	sb.WriteString(colorAccent("|") + " No active schedules.\n")
	sb.WriteString(colorAccent("|") + "\n")
	sb.WriteString(colorAccent("|") + " Add schedules via CLI:\n")
	sb.WriteString(colorAccent("|") + "   soulgate schedule add --type skill --target review --interval 1h\n")
	sb.WriteString(colorAccent("|") + "   soulgate schedule add --type prompt --target \"check status\" --interval 30m\n")
	sb.WriteString(colorAccent("|") + "\n")
	sb.WriteString(colorAccent("|") + " Types: skill, agent, prompt\n")
	sb.WriteString(colorAccentBright("+-------------------------------------------------------+"))
	return sb.String()
}

// renderAutocomplete renders autocomplete suggestions dropdown
func (m *InteractiveChatModel) renderAutocomplete() string {
	if len(m.autocomplete) == 0 {
		return ""
	}

	var sb strings.Builder

	// Box top with gradient
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("45")).
		Render("  ┌─ ✨ Suggestions "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Render(strings.Repeat("─", 35)))
	sb.WriteString("\n")

	// Suggestions list (max 6 items)
	maxItems := 6
	displayCount := len(m.autocomplete)
	if displayCount > maxItems {
		displayCount = maxItems
	}

	for i := 0; i < displayCount; i++ {
		suggestion := m.autocomplete[i]

		if i == m.autocompleteIndex {
			// Highlighted item (selected) with gradient
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("45")).
				Render("  │ "))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).
				Bold(true).
				Render("► "))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).
				Bold(true).
				Background(lipgloss.Color("236")).
				Render(" "+suggestion+" "))
		} else {
			// Normal item
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")).
				Render("  │ "))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				Render("  "))
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Render(suggestion))
		}
		sb.WriteString("\n")
	}

	// Show indicator if more items
	if len(m.autocomplete) > maxItems {
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Render("  │   "))
		sb.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Render(fmt.Sprintf("... (%d more)", len(m.autocomplete)-maxItems)))
		sb.WriteString("\n")
	}

	// Box bottom
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Render("  └"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Render(strings.Repeat("─", 54)))

	// Hint with icons
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("  ↑↓ navigate • "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("82")).
		Render("Enter/Tab"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(" select • "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Render("Esc"))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render(" close"))

	return sb.String()
}
