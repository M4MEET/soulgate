package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/skills"
	"github.com/M4MEET/soulgate/internal/ui/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// View renders the main TUI view
func (m InteractiveChatModel) View() string {
	var sb strings.Builder
	t := theme.T

	// Header
	sb.WriteString(renderHeader())
	sb.WriteString("\n")

	// Thin separator
	sepWidth := m.width
	if sepWidth < 40 {
		sepWidth = 80
	}
	sb.WriteString(t.Separator.Render(strings.Repeat("─", sepWidth)))
	sb.WriteString("\n")

	// Output viewport
	sb.WriteString(m.output.View())
	sb.WriteString("\n")

	// Onboarding overlay — fullscreen, vertically centered
	if m.ShowOnboarding {
		content := m.renderOnboarding()
		contentLines := strings.Count(content, "\n") + 1
		availableHeight := m.height - 4 // account for header + separator
		topPad := (availableHeight - contentLines) / 2
		if topPad < 1 {
			topPad = 1
		}
		sb.WriteString(strings.Repeat("\n", topPad))
		sb.WriteString(content)
		// Fill remaining space so footer stays at bottom
		bottomPad := availableHeight - topPad - contentLines
		if bottomPad > 0 {
			sb.WriteString(strings.Repeat("\n", bottomPad))
		}
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
	if m.permission.Active && m.permission.Request != nil {
		sb.WriteString("\n")
		sb.WriteString(m.permission.View())
		return sb.String()
	}

	// Confirmation dialog overlay
	if m.confirmation.Active {
		sb.WriteString("\n")
		sb.WriteString(m.confirmation.View())
		return sb.String()
	}

	// Help overlay (Ctrl+H) — rendered in place of the input area
	if m.showHelpOverlay {
		sb.WriteString("\n")
		sb.WriteString(renderHelpOverlay())
		sb.WriteString("\n")
		sep := t.Separator.Render(strings.Repeat("─", sepWidth))
		sb.WriteString(sep + "\n")
		sb.WriteString(m.input.View())
		sb.WriteString("\n")
		sb.WriteString(m.renderHints())
		return sb.String()
	}

	// Status bar
	sb.WriteString(m.renderStatusBar())
	sb.WriteString("\n")

	// Input separator
	sb.WriteString(t.Separator.Render(strings.Repeat("─", sepWidth)))
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

// waitingPhrases cycles through fun status phrases while the AI is working.
var waitingPhrases = []string{
	"pondering",
	"conjuring",
	"noodling",
	"contemplating",
	"brewing",
	"crafting",
	"weaving",
	"channeling",
}

// contextWindowForModel returns the known context window size for a model.
// Falls back to 128k when the model ID is not recognised.
func contextWindowForModel(modelName string) int {
	type entry struct {
		substr string
		tokens int
	}
	known := []entry{
		// Anthropic
		{"claude-opus-4", 200000},
		{"claude-sonnet-4", 200000},
		{"claude-haiku-4", 200000},
		{"claude-3-5-sonnet", 200000},
		{"claude-3-5-haiku", 200000},
		{"claude-3-opus", 200000},
		// OpenAI
		{"gpt-4.1", 1047576},
		{"gpt-4o", 128000},
		{"o3", 200000},
		{"o4-mini", 200000},
		// Google
		{"gemini-3", 1000000},
		{"gemini-2.5-pro", 1000000},
		{"gemini-2.5-flash", 1000000},
		{"gemini-2.0-flash", 1000000},
		// xAI
		{"grok-4-1", 2000000},
		{"grok-4-fast", 2000000},
		{"grok-3", 131072},
		{"grok-code", 262144},
		{"grok-2-vision", 32768},
		// Groq / open
		{"llama-3.3-70b", 128000},
		{"mixtral-8x7b", 32768},
		// Mistral
		{"mistral-large", 128000},
	}
	lower := strings.ToLower(modelName)
	for _, e := range known {
		if strings.Contains(lower, e.substr) {
			return e.tokens
		}
	}
	return 128000
}

// formatTokenCount formats a token count as a compact string (e.g. 128000 → "128k", 1000000 → "1M").
func formatTokenCount(n int) string {
	switch {
	case n >= 1000000:
		return fmt.Sprintf("%dM", n/1000000)
	case n >= 1000:
		return fmt.Sprintf("%dk", n/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// renderStatusBar renders the status bar in the format:
//
//	provider/model | think <level> | tokens used/ctx (pct%) | stream | ready
func (m *InteractiveChatModel) renderStatusBar() string {
	t := theme.T
	pipe := t.Dim.Render(" | ")

	// --- Status / spinner segment ---
	var statusSeg string
	if m.thinking {
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spinner := spinners[m.spinnerFrame%len(spinners)]
		phrase := waitingPhrases[m.waitingPhraseIndex%len(waitingPhrases)]
		statusSeg = t.Spinner.Render(spinner + " " + phrase + "...")
	} else {
		if strings.TrimSpace(m.status) == "" {
			statusSeg = t.Dim.Render("ready")
		} else {
			statusSeg = t.Dim.Render(strings.ToLower(strings.TrimSpace(m.status)))
		}
	}

	// --- Provider/model segment ---
	modelSeg := t.Dim.Render(fmt.Sprintf("%s/%s", m.currentProvider, m.currentModel))

	// --- Thinking level segment ---
	thinkLevel := "off"
	if d := m.orch.GetDirectives(); d != nil {
		thinkLevel = string(d.ThinkingLevel)
	}
	thinkSeg := t.Dim.Render("think " + thinkLevel)

	// --- Token usage segment ---
	contextSize := contextWindowForModel(m.currentModel)
	used := m.sessionTokensUsed
	var pct int
	if contextSize > 0 {
		pct = (used * 100) / contextSize
	}
	tokenSeg := t.Dim.Render(fmt.Sprintf("tokens %s/%s (%d%%)",
		formatTokenCount(used), formatTokenCount(contextSize), pct))

	// --- Stream segment ---
	streamSeg := ""
	if m.streamingEnabled {
		streamSeg = pipe + t.Success.Render("stream")
	}

	// --- Trust mode segment ---
	trustSeg := ""
	if m.orch.IsTrustMode() {
		remaining := m.orch.TrustModeRemaining()
		mins := int(remaining.Minutes())
		trustSeg = pipe + t.Warning.Render(fmt.Sprintf("trust %dm", mins))
	}

	// --- Non-default directives segment ---
	directivesSeg := ""
	if d := m.orch.GetDirectives(); d != nil {
		ds := d.String()
		if ds != "defaults" {
			directivesSeg = pipe + t.Warning.Render(ds)
		}
	}

	// --- Gateway segment ---
	gatewaySeg := ""
	if m.gatewayURL != "" {
		if m.gatewayConnected {
			gatewaySeg = pipe + t.Success.Render("gw:connected")
		} else {
			gatewaySeg = pipe + t.Error.Render("gw:disconnected")
		}
	}

	// --- Cost segment (today's spend, hidden when zero) ---
	costSeg := ""
	if ct := m.orch.GetCostTracker(); ct != nil {
		if today := ct.TodayCost(); today > 0 {
			costSeg = pipe + t.Dim.Render(core.FormatCost(today)+" today")
		}
	}

	return "  " + statusSeg + pipe + modelSeg + pipe + thinkSeg + pipe + tokenSeg +
		streamSeg + trustSeg + directivesSeg + gatewaySeg + costSeg
}

// renderHints renders keyboard shortcut hints
func (m *InteractiveChatModel) renderHints() string {
	t := theme.T
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// When thinking, show abort shortcut prominently.
	if m.thinking {
		return dim.Render("  ") +
			t.Key.Render("esc") + dim.Render(" abort  ") +
			t.Key.Render("ctrl+c") + dim.Render(" force exit")
	}

	return dim.Render("  ") +
		t.Key.Render("tab") + dim.Render(" complete  ") +
		t.Key.Render("↑/↓") + dim.Render(" history  ") +
		t.Key.Render("pgup/pgdn") + dim.Render(" scroll  ") +
		t.Key.Render("ctrl+h") + dim.Render(" help  ") +
		t.Key.Render("ctrl+n") + dim.Render(" new  ") +
		t.Key.Render("ctrl+g") + dim.Render(" agents  ") +
		t.Key.Render("ctrl+c") + dim.Render(" exit")
}

// renderStatus renders the status information
func (m *InteractiveChatModel) renderStatus() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Status") + "\n\n")
	sb.WriteString("  " + t.Muted.Render("Provider  ") + t.Value.Render(m.currentProvider) + "\n")
	sb.WriteString("  " + t.Muted.Render("Model     ") + t.Value.Render(m.currentModel) + "\n")
	sb.WriteString("  " + t.Muted.Render("Status    ") + t.Success.Render("ready") + "\n")

	return sb.String()
}

// renderDebugInfo renders debug information
func (m *InteractiveChatModel) renderDebugInfo() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Debug") + "\n\n")

	sb.WriteString("  " + t.Muted.Render("Model") + "\n")
	sb.WriteString("    " + t.Muted.Render("Provider: ") + t.Value.Render(m.currentProvider) + "\n")
	sb.WriteString("    " + t.Muted.Render("Model:    ") + t.Value.Render(m.currentModel) + "\n\n")

	sb.WriteString("  " + t.Muted.Render("API Keys") + "\n")
	for _, env := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		v := os.Getenv(env)
		if v != "" {
			sb.WriteString("    " + t.Muted.Render(env+": ") + t.Success.Render("set ("+v[:8]+"...)") + "\n")
		} else {
			sb.WriteString("    " + t.Muted.Render(env+": ") + t.Error.Render("not set") + "\n")
		}
	}
	sb.WriteString("\n")

	cwd, _ := os.Getwd()
	sb.WriteString("  " + t.Muted.Render("Environment") + "\n")
	sb.WriteString("    " + t.Muted.Render("Working Dir: ") + t.Value.Render(cwd) + "\n")
	sb.WriteString("    " + t.Muted.Render("Config Dir:  ") + t.Value.Render("~/.soulgate/") + "\n")

	return sb.String()
}

// renderMCPStatus renders MCP server status
func (m *InteractiveChatModel) renderMCPStatus() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("MCP Servers") + "\n\n")

	mcpMgr := m.orch.GetMCPManager()
	if mcpMgr == nil {
		sb.WriteString("  " + t.Muted.Render("MCP not initialized") + "\n")
		return sb.String()
	}

	servers := mcpMgr.ListServers()
	if len(servers) == 0 {
		sb.WriteString("  " + t.Muted.Render("No MCP servers configured.") + "\n\n")
		sb.WriteString("  " + t.Muted.Render("Add servers in .soulgate/config.yml under 'mcp.servers'") + "\n")
		return sb.String()
	}

	for _, srv := range servers {
		status := t.Error.Render("stopped")
		if srv.Running {
			status = t.Success.Render("running")
		}
		sb.WriteString(fmt.Sprintf("  %s  %s\n", t.Tool.Render(srv.Name), status))
		if srv.Running {
			sb.WriteString(fmt.Sprintf("    %s  %s  %s\n",
				t.Muted.Render(fmt.Sprintf("%d tools", srv.Tools)),
				t.Muted.Render(fmt.Sprintf("%d resources", srv.Resources)),
				t.Muted.Render(fmt.Sprintf("%d prompts", srv.Prompts)),
			))
		}
	}

	allTools := mcpMgr.GetAllTools()
	if len(allTools) > 0 {
		sb.WriteString("\n  " + t.Muted.Render("Tools:") + "\n")
		for _, tool := range allTools {
			desc := tool.Description
			if len(desc) > 60 {
				desc = desc[:60] + "..."
			}
			sb.WriteString(fmt.Sprintf("    %s  %s\n", t.Tool.Render(tool.Name), t.Muted.Render(desc)))
		}
	}

	return sb.String()
}

// renderSkillsList renders available skills
func (m *InteractiveChatModel) renderSkillsList() string {
	var sb strings.Builder
	t := theme.T

	workspace := m.orch.GetWorkspace()
	skillsDir := filepath.Join(workspace.Root, workspace.Config.Skills.Dir)
	loader := skills.NewLoader(skillsDir)

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Skills") + "\n\n")

	skillIDs, err := loader.ListSkills()
	if err != nil || len(skillIDs) == 0 {
		sb.WriteString("  " + t.Muted.Render("No skills found.") + "\n\n")
		sb.WriteString("  " + t.Muted.Render("Create one: soulgate skills create <name>") + "\n")
		sb.WriteString("  " + t.Muted.Render("Directory:  "+skillsDir) + "\n")
		return sb.String()
	}

	loadedSkills, _ := loader.LoadByIDs(skillIDs)
	enabled := workspace.Config.Skills.EnabledSkills

	for _, skill := range loadedSkills {
		status := t.Success.Render("active")
		if len(enabled) > 0 {
			isEnabled := false
			for _, e := range enabled {
				if e == skill.ID {
					isEnabled = true
					break
				}
			}
			if !isEnabled {
				status = t.Muted.Render("disabled")
			}
		}

		sb.WriteString("  " + t.Tool.Render(skill.Name) + "  " + status + "\n")
		if skill.Description != "" {
			desc := skill.Description
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			sb.WriteString("  " + t.Muted.Render(desc) + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderMemoryList renders memory entries
func (m *InteractiveChatModel) renderMemoryList() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Memory") + "\n\n")

	memories := m.orch.GetMemoryStore().List()
	if len(memories) == 0 {
		sb.WriteString("  " + t.Muted.Render("No memory entries yet.") + "\n")
		sb.WriteString("  " + t.Muted.Render("The AI will save memories during conversation.") + "\n")
		return sb.String()
	}

	count := 0
	for _, entry := range memories {
		if count >= 15 {
			sb.WriteString("  " + t.Muted.Render(fmt.Sprintf("... and %d more", len(memories)-count)) + "\n")
			break
		}
		value := entry.Value
		if len(value) > 50 {
			value = value[:47] + "..."
		}
		sb.WriteString("  " + t.Key.Render(entry.Key) + "  " + t.Muted.Render(value) + "\n")
		count++
	}

	return sb.String()
}

// renderSoulInfo shows the current soul configuration
func (m *InteractiveChatModel) renderSoulInfo() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("AI Persona") + t.Muted.Render("  SOUL.md") + "\n\n")

	workspace := m.orch.GetWorkspace()
	soul, err := core.LoadSoulConfig(workspace.ConfigDir)
	if err != nil || soul == nil {
		sb.WriteString("  " + t.Muted.Render("No SOUL.md configured.") + "\n\n")
		sb.WriteString("  " + t.Muted.Render("Create one with /soul init") + "\n")
		sb.WriteString("  " + t.Muted.Render("Defines personality, style, and boundaries.") + "\n")
		return sb.String()
	}

	sections := core.ParseSoulSections(soul.Content)
	for name, content := range sections {
		sb.WriteString("  " + t.Value.Render(name) + "\n")
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if i >= 3 {
				sb.WriteString("  " + t.Muted.Render("  ...") + "\n")
				break
			}
			if strings.TrimSpace(line) != "" {
				sb.WriteString("  " + t.Muted.Render("  "+line) + "\n")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  " + t.Muted.Render("Path: "+soul.Path) + "\n")
	return sb.String()
}

// initSoul creates a default SOUL.md
func (m *InteractiveChatModel) initSoul() string {
	workspace := m.orch.GetWorkspace()
	if err := core.CreateSoulFile(workspace.ConfigDir); err != nil {
		return theme.Error("  Error: " + err.Error())
	}
	return theme.Success("  Created SOUL.md at " + workspace.ConfigDir + "/SOUL.md")
}

// resetSoul resets SOUL.md to defaults
func (m *InteractiveChatModel) resetSoul() string {
	workspace := m.orch.GetWorkspace()
	if err := core.UpdateSoulFile(workspace.ConfigDir, core.DefaultSoulTemplate()); err != nil {
		return theme.Error("  Error: " + err.Error())
	}
	return theme.Success("  SOUL.md reset to default template.")
}

// renderScheduleInfo shows scheduled tasks
func (m *InteractiveChatModel) renderScheduleInfo() string {
	var sb strings.Builder
	t := theme.T

	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Scheduled Tasks") + "\n\n")
	sb.WriteString("  " + t.Muted.Render("No active schedules.") + "\n\n")
	sb.WriteString("  " + t.Muted.Render("Add via CLI:") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate schedule add --type skill --target review --interval 1h") + "\n")
	sb.WriteString("  " + t.Command.Render("soulgate schedule add --type prompt --target \"check status\" --interval 30m") + "\n")

	return sb.String()
}

// renderAutocomplete renders autocomplete suggestions with a scrolling window
func (m *InteractiveChatModel) renderAutocomplete() string {
	total := len(m.autocomplete)
	if total == 0 {
		return ""
	}

	var sb strings.Builder
	t := theme.T
	hl := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)

	maxVisible := 12
	if total <= maxVisible {
		for i, suggestion := range m.autocomplete {
			if i == m.autocompleteIndex {
				sb.WriteString(hl.Render("  > " + suggestion))
			} else {
				sb.WriteString(t.Dim.Render("    " + suggestion))
			}
			sb.WriteString("\n")
		}
	} else {
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
			sb.WriteString(t.Dim.Render(fmt.Sprintf("    ... %d above", start)))
			sb.WriteString("\n")
		}

		for i := start; i < end; i++ {
			if i == m.autocompleteIndex {
				sb.WriteString(hl.Render("  > " + m.autocomplete[i]))
			} else {
				sb.WriteString(t.Dim.Render("    " + m.autocomplete[i]))
			}
			sb.WriteString("\n")
		}

		if end < total {
			sb.WriteString(t.Dim.Render(fmt.Sprintf("    ... %d below", total-end)))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderAgentList renders all agents and their status
func (m *InteractiveChatModel) renderAgentList() string {
	var sb strings.Builder
	t := theme.T

	agents := m.orch.GetAgentManager().List()
	sb.WriteString("\n")
	sb.WriteString("  " + t.Title.Render("Agents") + "\n\n")

	if len(agents) == 0 {
		sb.WriteString("  " + t.Muted.Render("No agents. The AI can create them with agent_create.") + "\n")
		return sb.String()
	}

	for _, a := range agents {
		status := t.Muted.Render(string(a.Status))
		switch a.Status {
		case core.AgentRunning:
			status = t.Spinner.Render("running")
		case core.AgentCompleted:
			status = t.Success.Render("completed")
		case core.AgentFailed:
			status = t.Error.Render("failed")
		case core.AgentStopped:
			status = t.Warning.Render("stopped")
		}

		sb.WriteString(fmt.Sprintf("  %s  %s  %s\n", t.Tool.Render(a.ID), t.Value.Render(a.Name), status))

		task := a.Task
		if len(task) > 60 {
			task = task[:57] + "..."
		}
		sb.WriteString("  " + t.Muted.Render("  "+task) + "\n")

		if a.Error != "" {
			errMsg := a.Error
			if len(errMsg) > 80 {
				errMsg = errMsg[:77] + "..."
			}
			sb.WriteString("  " + t.Error.Render("  error: "+errMsg) + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("  " + t.Muted.Render("Use /agent <id> to see live activity log") + "\n")
	return sb.String()
}

// renderAgentDetail renders the live activity log for a specific agent
func (m *InteractiveChatModel) renderAgentDetail(agentID string) string {
	var sb strings.Builder
	t := theme.T

	agent, ok := m.orch.GetAgentManager().Get(agentID)
	if !ok {
		return "  " + t.Error.Render("Agent not found: "+agentID) + "\n"
	}

	sb.WriteString("\n")

	// Header
	status := t.Muted.Render(string(agent.Status))
	switch agent.Status {
	case core.AgentRunning:
		status = t.Spinner.Render("running")
	case core.AgentCompleted:
		status = t.Success.Render("completed")
	case core.AgentFailed:
		status = t.Error.Render("failed")
	case core.AgentStopped:
		status = t.Warning.Render("stopped")
	}

	sb.WriteString("  " + t.Title.Render(agent.Name) + "  " + t.Muted.Render(agent.ID) + "  " + status + "\n")

	task := agent.Task
	if len(task) > 80 {
		task = task[:77] + "..."
	}
	sb.WriteString("  " + t.Muted.Render(task) + "\n")
	sb.WriteString(t.Separator.Render("  "+strings.Repeat("─", 60)) + "\n")

	// Activity log
	entries := agent.GetLogTail(30)
	if len(entries) == 0 {
		sb.WriteString("  " + t.Muted.Render("(no activity yet)") + "\n")
	} else {
		for _, entry := range entries {
			ts := entry.Time.Format("15:04:05")
			switch entry.Kind {
			case "iteration":
				sb.WriteString(fmt.Sprintf("  %s  %s\n",
					t.Dim.Render(ts),
					t.Warning.Render(entry.Message)))
			case "model_call":
				sb.WriteString(fmt.Sprintf("  %s  %s %s\n",
					t.Dim.Render(ts),
					t.Dim.Render("⟶"),
					t.Muted.Render(entry.Message)))
			case "model_done":
				sb.WriteString(fmt.Sprintf("  %s  %s %s\n",
					t.Dim.Render(ts),
					t.Success.Render("⟵"),
					t.Muted.Render(entry.Message)))
			case "tool_start":
				sb.WriteString(fmt.Sprintf("  %s  %s %s\n",
					t.Dim.Render(ts),
					t.Dim.Render("┌─"),
					t.Tool.Render(entry.Message)))
			case "tool_done":
				sb.WriteString(fmt.Sprintf("  %s  %s %s\n",
					t.Dim.Render(ts),
					t.Dim.Render("└─"),
					t.Muted.Render(entry.Message)))
			case "text":
				msg := entry.Message
				if len(msg) > 100 {
					msg = msg[:97] + "..."
				}
				sb.WriteString(fmt.Sprintf("  %s  %s\n",
					t.Dim.Render(ts),
					t.Body.Render(msg)))
			case "error":
				sb.WriteString(fmt.Sprintf("  %s  %s\n",
					t.Dim.Render(ts),
					t.Error.Render(entry.Message)))
			default:
				sb.WriteString(fmt.Sprintf("  %s  %s\n",
					t.Dim.Render(ts),
					t.Muted.Render(entry.Message)))
			}
		}
	}

	// Final result or error
	if agent.Status == core.AgentCompleted && agent.Result != "" {
		result := agent.Result
		if len(result) > 300 {
			result = result[:297] + "..."
		}
		sb.WriteString("\n  " + t.Success.Render("Result:") + "\n")
		sb.WriteString("  " + t.Body.Render(result) + "\n")
	}
	if agent.Status == core.AgentFailed && agent.Error != "" {
		sb.WriteString("\n  " + t.Error.Render("Error: "+agent.Error) + "\n")
	}

	return sb.String()
}
