package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHeader renders a clean, minimal header
func renderHeader() string {
	name := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true).
		Render("SoulGate")

	tagline := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Render("secure AI gateway")

	return fmt.Sprintf("  %s  %s", name, tagline)
}

// renderHelp renders the help screen
func renderHelp() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	cmd := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Commands") + "\n\n")
	sb.WriteString("  " + cmd.Render("/status") + dim.Render("              Show status") + "\n")
	sb.WriteString("  " + cmd.Render("/tools") + dim.Render("               List tools") + "\n")
	sb.WriteString("  " + cmd.Render("/skills") + dim.Render("              List skills") + "\n")
	sb.WriteString("  " + cmd.Render("/memory") + dim.Render("              Show memory entries") + "\n")
	sb.WriteString("  " + cmd.Render("/model") + dim.Render(" [provider]    Show/switch AI model") + "\n")
	sb.WriteString("  " + cmd.Render("/soul") + dim.Render("                AI persona config") + "\n")
	sb.WriteString("  " + cmd.Render("/schedule") + dim.Render("            Scheduled tasks") + "\n")
	sb.WriteString("  " + cmd.Render("/hub") + dim.Render("                 Community hub") + "\n")
	sb.WriteString("  " + cmd.Render("/setup") + dim.Render("               Integration wizard") + "\n")
	sb.WriteString("  " + cmd.Render("/onboarding") + dim.Render("          Run onboarding") + "\n")
	sb.WriteString("  " + cmd.Render("/history") + dim.Render("             Command history") + "\n")
	sb.WriteString("  " + cmd.Render("/debug") + dim.Render("               Debug info") + "\n")
	sb.WriteString("  " + cmd.Render("/clear") + dim.Render("               Clear screen") + "\n")
	sb.WriteString("  " + cmd.Render("/exit") + dim.Render("                Exit") + "\n")
	sb.WriteString("  " + cmd.Render("!command") + dim.Render("             Execute shell command") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Shortcuts") + "\n\n")
	sb.WriteString("  " + cmd.Render("Tab") + dim.Render("       Autocomplete    ") + cmd.Render("Ctrl+H") + dim.Render("  Help") + "\n")
	sb.WriteString("  " + cmd.Render("Up/Down") + dim.Render("   History         ") + cmd.Render("Ctrl+L") + dim.Render("  Clear") + "\n")
	sb.WriteString("  " + cmd.Render("PgUp/Dn") + dim.Render("   Scroll          ") + cmd.Render("Ctrl+C") + dim.Render("  Exit") + "\n")

	return sb.String()
}

// renderToolsList renders the list of available tools
func renderToolsList() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	tool := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Available Tools") + "\n\n")
	sb.WriteString("  " + dim.Render("Files") + "     " + tool.Render("files_read") + dim.Render(", ") + tool.Render("files_write") + dim.Render(", ") + tool.Render("files_list") + "\n")
	sb.WriteString("  " + dim.Render("System") + "    " + tool.Render("exec_command") + dim.Render(", ") + tool.Render("net_request") + "\n")
	sb.WriteString("  " + dim.Render("Memory") + "    " + tool.Render("memory_write") + dim.Render(", ") + tool.Render("memory_get") + dim.Render(", ") + tool.Render("memory_search") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + dim.Render("+ 46 integration tools") + "\n")

	return sb.String()
}

// renderHubOverview renders the SoulHub marketplace overview
func renderHubOverview() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	cmd := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("SoulHub") + dim.Render(" - Community Marketplace") + "\n\n")
	sb.WriteString("  " + cmd.Render("/hub plugins") + dim.Render("     Browse plugins") + "\n")
	sb.WriteString("  " + cmd.Render("/hub skills") + dim.Render("      Browse skills") + "\n")
	sb.WriteString("  " + cmd.Render("/hub installed") + dim.Render("   Show installed") + "\n")
	sb.WriteString("  " + cmd.Render("/hub install") + dim.Render(" <name>") + "\n")

	return sb.String()
}

// renderHubPlugins renders the list of featured Hub plugins
func renderHubPlugins() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	name := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Featured Plugins") + "\n\n")
	sb.WriteString("  " + name.Render("awesome-github") + dim.Render("    Advanced GitHub integration     1.2k downloads") + "\n")
	sb.WriteString("  " + name.Render("notion-sync") + dim.Render("       Sync notes to Notion          890 downloads") + "\n")
	sb.WriteString("  " + name.Render("whatsapp-bot") + dim.Render("      WhatsApp automation            567 downloads") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + dim.Render("Full list: soulgate hub plugins") + "\n")

	return sb.String()
}

// renderHubSkills renders the list of popular Hub skills
func renderHubSkills() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Bold(true)

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	name := lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	sb.WriteString("\n")
	sb.WriteString("  " + title.Render("Popular Skills") + "\n\n")
	sb.WriteString("  " + name.Render("auto-commit") + dim.Render("       Auto-commit and push          2.3k downloads") + "\n")
	sb.WriteString("  " + name.Render("daily-standup") + dim.Render("     Automated standup reports      1.5k downloads") + "\n")
	sb.WriteString("  " + name.Render("code-review") + dim.Render("       AI code review                 890 downloads") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + dim.Render("Full list: soulgate hub skills") + "\n")

	return sb.String()
}

// renderHubInstalled renders the list of installed Hub items
func renderHubInstalled() string {
	var sb strings.Builder

	dim := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244"))

	sb.WriteString("\n")
	sb.WriteString("  " + dim.Render("No items installed yet.") + "\n\n")
	sb.WriteString("  " + dim.Render("Browse with /hub plugins or /hub skills") + "\n")

	return sb.String()
}
