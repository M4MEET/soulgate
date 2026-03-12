package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Static rendering functions (non-method functions)
// These functions render various UI components

// renderHeader renders the SoulGate header with gradient effect
func renderHeader() string {
	// Gradient text effect for "SoulGate"
	soul := lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")). // Orange
		Bold(true).
		Render("Soul")

	gate := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")). // Lighter orange
		Bold(true).
		Render("Gate")

	tagline := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")). // Gray
		Italic(true).
		Render("Your AI Guardian")

	// Build header with gradient
	header := fmt.Sprintf("  🐙  %s%s  -  %s", soul, gate, tagline)

	// Center it
	headerStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(60)

	return headerStyle.Render(header)
}

// renderHelp renders the help screen with commands and shortcuts
func renderHelp() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ Help ─────────────────────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Commands:") + "\n")
	sb.WriteString(colorAccent("│") + "   /status              Show status\n")
	sb.WriteString(colorAccent("│") + "   /tools               List tools\n")
	sb.WriteString(colorAccent("│") + "   /skills              List skills\n")
	sb.WriteString(colorAccent("│") + "   /memory              Show memory entries\n")
	sb.WriteString(colorAccent("│") + "   /model [provider]    Show/switch AI model\n")
	sb.WriteString(colorAccent("│") + "   /history             Show history\n")
	sb.WriteString(colorAccent("│") + "   /clear               Clear screen\n")
	sb.WriteString(colorAccent("│") + "   /hub                 Browse community hub\n")
	sb.WriteString(colorAccent("│") + "   /setup               Integration wizard\n")
	sb.WriteString(colorAccent("│") + "   /onboarding          Run onboarding\n")
	sb.WriteString(colorAccent("│") + "   /help                Show this help\n")
	sb.WriteString(colorAccent("│") + "   /exit                Exit\n")
	sb.WriteString(colorAccent("│") + "   !command             Execute shell command\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Examples:") + "\n")
	sb.WriteString(colorAccent("│") + "   /model               List all models\n")
	sb.WriteString(colorAccent("│") + "   /model openai        Switch to OpenAI\n")
	sb.WriteString(colorAccent("│") + "   /model anthropic     Switch to Claude\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Navigation:") + "\n")
	sb.WriteString(colorAccent("│") + "   ↑↓         Navigate command history\n")
	sb.WriteString(colorAccent("│") + "   ←→         Move cursor in input\n")
	sb.WriteString(colorAccent("│") + "   Home/End   Jump to start/end of line\n")
	sb.WriteString(colorAccent("│") + "   PgUp/PgDn  Scroll output\n")
	sb.WriteString(colorAccent("│") + "   Tab        Autocomplete command\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Shortcuts:") + "\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+A/E   Jump to start/end\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+U     Clear line\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+K     Delete to end\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+W     Delete word\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+H     Show this help\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+L     Clear screen\n")
	sb.WriteString(colorAccent("│") + "   Ctrl+C/D   Exit\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderToolsList renders the list of available tools
func renderToolsList() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ Available Tools ──────────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " Core Tools (9):\n")
	sb.WriteString(colorAccent("│") + "   • files_read, files_write, files_list\n")
	sb.WriteString(colorAccent("│") + "   • exec_command, net_request\n")
	sb.WriteString(colorAccent("│") + "   • memory_write, memory_search, memory_get\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " Integration Tools: " + colorMuted("46 tools") + "\n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderHubOverview renders the SoulHub marketplace overview
func renderHubOverview() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ 🐙 SoulHub - Community Marketplace ───────────────╮\n"))
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Categories:") + "\n")
	sb.WriteString(colorAccent("│") + "   🔌 Plugins       - WASM extensions\n")
	sb.WriteString(colorAccent("│") + "   ⚡ Skills        - Automation workflows\n")
	sb.WriteString(colorAccent("│") + "   🔗 Integrations - Ready-to-use services\n")
	sb.WriteString(colorAccent("│") + "   📦 Recipes       - Complete solutions\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Browse:") + "\n")
	sb.WriteString(colorAccent("│") + "   /hub plugins     - View all plugins\n")
	sb.WriteString(colorAccent("│") + "   /hub skills      - View all skills\n")
	sb.WriteString(colorAccent("│") + "   /hub installed   - Show installed items\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Install:") + "\n")
	sb.WriteString(colorAccent("│") + "   /hub install <name>\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorMuted("Or use CLI: soulgate hub") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderHubPlugins renders the list of featured Hub plugins
func renderHubPlugins() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ 🔌 Hub Plugins ───────────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Featured Plugins:") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + "   1. ⭐⭐⭐⭐⭐ " + colorSuccess("awesome-github") + "\n")
	sb.WriteString(colorAccent("│") + "      Advanced GitHub integration\n")
	sb.WriteString(colorAccent("│") + "      1.2k downloads\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + "   2. ⭐⭐⭐⭐⭐ " + colorInfo("notion-sync") + "\n")
	sb.WriteString(colorAccent("│") + "      Sync notes to Notion\n")
	sb.WriteString(colorAccent("│") + "      890 downloads\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + "   3. ⭐⭐⭐⭐ " + colorInfo("whatsapp-bot") + "\n")
	sb.WriteString(colorAccent("│") + "      WhatsApp automation\n")
	sb.WriteString(colorAccent("│") + "      567 downloads\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorMuted("For full list: soulgate hub plugins") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderHubSkills renders the list of popular Hub skills
func renderHubSkills() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ ⚡ Hub Skills ─────────────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Popular Skills:") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + "   1. ⭐⭐⭐⭐⭐ " + colorSuccess("auto-commit") + "\n")
	sb.WriteString(colorAccent("│") + "      Auto-commit and push changes\n")
	sb.WriteString(colorAccent("│") + "      2.3k downloads\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + "   2. ⭐⭐⭐⭐⭐ " + colorInfo("daily-standup") + "\n")
	sb.WriteString(colorAccent("│") + "      Automated standup reports\n")
	sb.WriteString(colorAccent("│") + "      1.5k downloads\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + "   3. ⭐⭐⭐⭐ " + colorInfo("code-review") + "\n")
	sb.WriteString(colorAccent("│") + "      AI code review automation\n")
	sb.WriteString(colorAccent("│") + "      890 downloads\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorMuted("For full list: soulgate hub skills") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}

// renderHubInstalled renders the list of installed Hub items
func renderHubInstalled() string {
	var sb strings.Builder
	sb.WriteString(colorAccentBright("╭─ 📦 Installed Items ───────────────────────────────╮\n"))
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorMuted("No items installed yet.") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorBold("Get Started:") + "\n")
	sb.WriteString(colorAccent("│") + "   /hub plugins     - Browse plugins\n")
	sb.WriteString(colorAccent("│") + "   /hub install <name>\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccent("│") + " " + colorMuted("Or: soulgate hub install <name>") + "\n")
	sb.WriteString(colorAccent("│") + " \n")
	sb.WriteString(colorAccentBright("╰────────────────────────────────────────────────────╯"))
	return sb.String()
}
