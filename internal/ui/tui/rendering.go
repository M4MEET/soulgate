package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/M4MEET/soulgate/internal/ui/tui/theme"
)

// renderHeader renders a clean, minimal header
func renderHeader() string {
	t := theme.T
	return fmt.Sprintf("  %s  %s",
		t.HeaderName.Render("SoulGate"),
		t.HeaderTagline.Render("secure AI gateway"))
}

// renderHelp renders the help screen as a two-column Commands / Shortcuts layout.
// It is also shown inline when /help is typed in chat.
func renderHelp() string {
	return renderHelpOverlay()
}

// renderHelpOverlay renders the full two-column help reference used both inline
// and in the Ctrl+H overlay.
func renderHelpOverlay() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Help") + "\n\n")

	type row struct {
		cmd     string
		cmdDesc string
		key     string
		keyDesc string
	}

	rows := []row{
		{"/help", "Show this help", "Ctrl+H", "toggle help"},
		{"/clear", "Clear chat", "Ctrl+L", "clear chat"},
		{"/new", "New conversation", "Ctrl+N", "new conversation"},
		{"/model", "Switch model", "Ctrl+T", "toggle thinking"},
		{"/status", "Show status", "Ctrl+O", "toggle tool output"},
		{"/tools", "List tools", "Ctrl+G", "agent list"},
		{"/think", "Set thinking level", "Ctrl+D", "exit (empty input)"},
		{"/agents", "List agents", "Esc", "abort / close"},
		{"/usage", "Token usage", "Tab", "autocomplete"},
		{"/export [format]", "Export chat", "Up/Down", "history / nav"},
		{"/doctor", "Run diagnostics", "PgUp/PgDn", "scroll output"},
		{"/abort", "Abort generation", "Ctrl+C", "force exit"},
		{"/sessions", "Recent sessions", "", ""},
		{"/stream", "Toggle streaming", "", ""},
		{"/trust", "Toggle trust mode", "", ""},
		{"/fork [label]", "Fork conversation", "", ""},
		{"/branches", "List branches", "", ""},
		{"/switch <id>", "Switch branch", "", ""},
		{"/merge <id>", "Merge branch", "", ""},
		{"/exit", "Exit", "", ""},
	}

	// Header row
	sb.WriteString("  " + t.Value.Render("Commands") +
		strings.Repeat(" ", 30) + t.Value.Render("Shortcuts") + "\n")
	sb.WriteString("  " + t.Muted.Render(strings.Repeat("─", 38)) +
		"  " + t.Muted.Render(strings.Repeat("─", 30)) + "\n")

	for _, r := range rows {
		var line strings.Builder
		line.WriteString("  ")
		if r.cmd != "" {
			// Command column: fixed 18 chars for command name, 20 for description
			cmdPad := 18 - len(r.cmd)
			if cmdPad < 1 {
				cmdPad = 1
			}
			line.WriteString(t.Command.Render(r.cmd))
			line.WriteString(strings.Repeat(" ", cmdPad))
			line.WriteString(t.Muted.Render(r.cmdDesc))
			descPad := 20 - len(r.cmdDesc)
			if descPad < 2 {
				descPad = 2
			}
			line.WriteString(strings.Repeat(" ", descPad))
		} else {
			line.WriteString(strings.Repeat(" ", 40))
		}
		if r.key != "" {
			keyPad := 10 - len(r.key)
			if keyPad < 1 {
				keyPad = 1
			}
			line.WriteString(t.Key.Render(r.key))
			line.WriteString(strings.Repeat(" ", keyPad))
			line.WriteString(t.Muted.Render(r.keyDesc))
		}
		sb.WriteString(line.String() + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + t.Muted.Render("Use /context for the full usage guide.  Press Esc or Ctrl+H to close.") + "\n")

	return sb.String()
}

// renderContext renders a complete usage guide for day-to-day operation.
func renderContext() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("SoulGate Context") + "\n\n")

	sb.WriteString("  " + t.Muted.Render("What SoulGate is:") + "\n")
	sb.WriteString("  " + t.Muted.Render("A secure AI gateway. The model is untrusted; policy + brokers enforce access.") + "\n\n")

	sb.WriteString("  " + t.Muted.Render("Recommended flow:") + "\n")
	sb.WriteString("  " + t.Command.Render("1.") + t.Muted.Render(" Run onboarding once: ") + t.Command.Render("soulgate onboarding") + "\n")
	sb.WriteString("  " + t.Command.Render("2.") + t.Muted.Render(" Start chat UI: ") + t.Command.Render("soulgate tui") + "\n")
	sb.WriteString("  " + t.Command.Render("3.") + t.Muted.Render(" Check available tools: ") + t.Command.Render("/tools") + "\n")
	sb.WriteString("  " + t.Command.Render("4.") + t.Muted.Render(" Ask tasks in plain English or run shell commands with ") + t.Command.Render("!command") + "\n")
	sb.WriteString("  " + t.Command.Render("5.") + t.Muted.Render(" Inspect runtime/security status with ") + t.Command.Render("/status") + t.Muted.Render(" and ") + t.Command.Render("soulgate audit tail --last 20") + "\n\n")

	sb.WriteString("  " + t.Muted.Render("Inside TUI (most useful):") + "\n")
	sb.WriteString("  " + t.Command.Render("/context") + t.Muted.Render("   this guide") + "\n")
	sb.WriteString("  " + t.Command.Render("/help") + t.Muted.Render("      quick commands + keys") + "\n")
	sb.WriteString("  " + t.Command.Render("/onboarding") + t.Muted.Render(" rerun guided setup") + "\n")
	sb.WriteString("  " + t.Command.Render("/model") + t.Muted.Render("     switch provider/model") + "\n")
	sb.WriteString("  " + t.Command.Render("/tools") + t.Muted.Render("     list policy-available tools") + "\n")
	sb.WriteString("  " + t.Command.Render("/status") + t.Muted.Render("    view provider, trust mode, session state") + "\n")
	sb.WriteString("  " + t.Command.Render("/stream") + t.Muted.Render("    toggle token streaming") + "\n")
	sb.WriteString("  " + t.Command.Render("/trust") + t.Muted.Render("     temporary full permission bypass (use carefully)") + "\n")
	sb.WriteString("  " + t.Command.Render("/setup") + t.Muted.Render("     integration wizard") + "\n")
	sb.WriteString("  " + t.Command.Render("/exit") + t.Muted.Render("      quit") + "\n\n")

	sb.WriteString("  " + t.Muted.Render("Core CLI commands:") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate init") + t.Muted.Render("            initialize workspace defaults") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate setup") + t.Muted.Render("           full setup wizard") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate onboarding") + t.Muted.Render("      force onboarding in TUI") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate run \"<prompt>\"") + t.Muted.Render(" one-shot execution") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate status") + t.Muted.Render("          workspace/config status") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate policy show") + t.Muted.Render("     active policy") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate audit tail --last 20") + t.Muted.Render(" recent security/audit events") + "\n\n")

	sb.WriteString("  " + t.Muted.Render("Security notes:") + "\n")
	sb.WriteString("  " + t.Muted.Render("- Default behavior is deny unless policy allows.") + "\n")
	sb.WriteString("  " + t.Muted.Render("- Tool access is filtered by policy.") + "\n")
	sb.WriteString("  " + t.Muted.Render("- ") + t.Command.Render("/trust") + t.Muted.Render(" bypasses policy checks temporarily.") + "\n")

	return sb.String()
}

// renderToolsList renders the list of available tools
func renderToolsList(toolNames []string) string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Available Tools") + "\n\n")

	if len(toolNames) == 0 {
		sb.WriteString("  " + t.Muted.Render("No tools exposed in the current policy mode.") + "\n")
		return sb.String()
	}

	sort.Strings(toolNames)

	categories := map[string][]string{
		"Files":    {},
		"System":   {},
		"Memory":   {},
		"Web":      {},
		"Process":  {},
		"Docs":     {},
		"Schedule": {},
		"Agents":   {},
		"Other":    {},
	}
	var integrations []string
	var mcpTools []string

	for _, name := range toolNames {
		switch {
		case strings.Contains(name, "__"):
			mcpTools = append(mcpTools, sanitizeToolNameForDisplay(name))
		case !isBuiltinToolName(name):
			integrations = append(integrations, sanitizeToolNameForDisplay(name))
		case strings.HasPrefix(name, "files_"):
			categories["Files"] = append(categories["Files"], name)
		case strings.HasPrefix(name, "memory_"):
			categories["Memory"] = append(categories["Memory"], name)
		case strings.HasPrefix(name, "web_") || strings.HasPrefix(name, "net_"):
			categories["Web"] = append(categories["Web"], name)
		case strings.HasPrefix(name, "process_") || strings.HasPrefix(name, "exec_"):
			categories["Process"] = append(categories["Process"], name)
		case strings.HasPrefix(name, "pdf_"):
			categories["Docs"] = append(categories["Docs"], name)
		case strings.HasPrefix(name, "cron_"):
			categories["Schedule"] = append(categories["Schedule"], name)
		case strings.HasPrefix(name, "agent_"):
			categories["Agents"] = append(categories["Agents"], name)
		case name == "switch_model":
			categories["System"] = append(categories["System"], name)
		default:
			categories["Other"] = append(categories["Other"], name)
		}
	}

	order := []string{"Files", "System", "Memory", "Web", "Process", "Docs", "Schedule", "Agents", "Other"}
	for _, category := range order {
		names := categories[category]
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		sb.WriteString("  " + t.Muted.Render(category) + "    ")
		for i, tool := range names {
			if i > 0 {
				sb.WriteString(t.Muted.Render(", "))
			}
			sb.WriteString(t.Tool.Render(tool))
		}
		sb.WriteString("\n")
	}

	if len(integrations) > 0 {
		sort.Strings(integrations)
		sb.WriteString("\n  " + t.Muted.Render("Integrations") + "  ")
		for i, tool := range integrations {
			if i > 0 {
				sb.WriteString(t.Muted.Render(", "))
			}
			sb.WriteString(t.Tool.Render(tool))
		}
		sb.WriteString("\n")
	}

	if len(mcpTools) > 0 {
		sort.Strings(mcpTools)
		sb.WriteString("\n  " + t.Muted.Render("MCP") + "           ")
		for i, tool := range mcpTools {
			if i > 0 {
				sb.WriteString(t.Muted.Render(", "))
			}
			sb.WriteString(t.Tool.Render(tool))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderHubOverview renders the SoulHub marketplace overview
func renderHubOverview() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("SoulHub") + t.Muted.Render(" - Community Marketplace") + "\n\n")
	sb.WriteString("  " + t.Command.Render("/hub plugins") + t.Muted.Render("     Browse plugins") + "\n")
	sb.WriteString("  " + t.Command.Render("/hub skills") + t.Muted.Render("      Browse skills") + "\n")
	sb.WriteString("  " + t.Command.Render("/hub installed") + t.Muted.Render("   Show installed") + "\n")
	sb.WriteString("  " + t.Command.Render("/hub install") + t.Muted.Render(" <name>") + "\n")

	return sb.String()
}

// renderHubPlugins renders the list of featured Hub plugins
func renderHubPlugins() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Featured Plugins") + "\n\n")
	sb.WriteString("  " + t.Tool.Render("awesome-github") + t.Muted.Render("    Advanced GitHub integration     1.2k downloads") + "\n")
	sb.WriteString("  " + t.Tool.Render("notion-sync") + t.Muted.Render("       Sync notes to Notion          890 downloads") + "\n")
	sb.WriteString("  " + t.Tool.Render("whatsapp-bot") + t.Muted.Render("      WhatsApp automation            567 downloads") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + t.Muted.Render("Full list: soulgate hub plugins") + "\n")

	return sb.String()
}

// renderHubSkills renders the list of popular Hub skills
func renderHubSkills() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Popular Skills") + "\n\n")
	sb.WriteString("  " + t.Tool.Render("auto-commit") + t.Muted.Render("       Auto-commit and push          2.3k downloads") + "\n")
	sb.WriteString("  " + t.Tool.Render("daily-standup") + t.Muted.Render("     Automated standup reports      1.5k downloads") + "\n")
	sb.WriteString("  " + t.Tool.Render("code-review") + t.Muted.Render("       AI code review                 890 downloads") + "\n")
	sb.WriteString("\n")
	sb.WriteString("  " + t.Muted.Render("Full list: soulgate hub skills") + "\n")

	return sb.String()
}

// renderHubInstalled renders the list of installed Hub items
func renderHubInstalled() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Muted.Render("No items installed yet.") + "\n\n")
	sb.WriteString("  " + t.Muted.Render("Browse with /hub plugins or /hub skills") + "\n")

	return sb.String()
}
