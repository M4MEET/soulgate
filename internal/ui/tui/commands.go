package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/protocol"
	tea "github.com/charmbracelet/bubbletea"
)

// Command execution
// These functions handle user commands and shell execution

// handleCommand handles slash commands like /status, /help, etc.
func (m InteractiveChatModel) handleCommand(cmd string) (InteractiveChatModel, tea.Cmd) {
	// Parse command and args
	parts := strings.Fields(strings.TrimSpace(cmd))
	if len(parts) == 0 {
		return m, nil
	}

	cmdName := strings.ToLower(parts[0])
	cmdArgs := parts[1:]

	// Check if this is a sensitive command that requires confirmation
	if isSensitiveCommand(cmd) {
		m.confirmation.Active = true
		m.confirmation.Message = getSensitiveMessage(cmd)
		m.confirmation.Command = cmd
		m.confirmation.PendingAction = func() tea.Cmd {
			return m.executeCommand(cmdName, cmdArgs)
		}
		return m, nil
	}

	// Execute non-sensitive command directly
	return m, m.executeCommand(cmdName, cmdArgs)
}

// executeCommand executes a parsed command
func (m *InteractiveChatModel) executeCommand(cmdName string, args []string) tea.Cmd {
	switch cmdName {
	case "/exit", "/quit":
		return tea.Quit

	case "/clear":
		m.messages = []string{welcomeMessage()}
		m.refreshOutput(true)
		return nil

	case "/history":
		if len(m.history) == 0 {
			m.addMessage(colorMuted("  (no history)"))
		} else {
			var sb strings.Builder
			sb.WriteString(colorAccentBright("Command History:\n"))
			for i := len(m.history) - 1; i >= 0 && i >= len(m.history)-10; i-- {
				sb.WriteString(colorMuted(fmt.Sprintf("  %d. %s\n", len(m.history)-i, m.history[i])))
			}
			m.addMessage(sb.String())
		}
		return nil

	case "/status":
		m.addMessage(m.renderStatus())
		return nil

	case "/tools":
		m.addMessage(renderToolsList(m.orch.GetAvailableToolNames()))
		return nil

	case "/help":
		m.addMessage(renderHelp())
		return nil

	case "/context":
		m.addMessage(renderContext())
		return nil

	case "/model":
		// If no args, show interactive provider selector (step 1)
		if len(args) == 0 {
			m.showModelSelector = true
			m.modelSelectionStep = 1 // Start with provider selection
			m.selectedProvider = ""
			m.selectedModelIndex = 0
			m.addMessage(colorAccent("Select an LLM provider..."))
			return nil
		}

		// If arg is a number, it might be from model selection
		if len(args) == 1 && len(args[0]) == 1 && args[0][0] >= '1' && args[0][0] <= '9' {
			number := int(args[0][0] - '0')
			for _, opt := range m.modelOptions {
				if opt.number == number {
					if err := m.orch.SetProvider(opt.provider, opt.model); err != nil {
						m.addMessage(colorError(fmt.Sprintf("✗ Failed to switch model: %s", err.Error())))
					} else {
						m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
						// Save config to persist the change
						if err := m.orch.GetWorkspace().SaveConfig(); err != nil {
							m.addMessage(colorWarn(fmt.Sprintf("⚠ Model switched but config save failed: %s", err.Error())))
						}
						m.addMessage(colorSuccess(fmt.Sprintf("✓ Switched to %s - %s", opt.name, opt.description)))
					}
					return nil
				}
			}
		}

		// Dynamic model switching: /model <provider> [model-name]
		provider := strings.ToLower(args[0])
		modelName := ""
		if len(args) > 1 {
			modelName = args[1]
		}

		// Switch model
		if err := m.orch.SetProvider(provider, modelName); err != nil {
			m.addMessage(colorError(fmt.Sprintf("✗ Failed to switch model: %s", err.Error())))
			return nil
		}

		// Update current provider/model
		m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()

		// Save config to persist the change
		if err := m.orch.GetWorkspace().SaveConfig(); err != nil {
			m.addMessage(colorWarn(fmt.Sprintf("⚠ Model switched but config save failed: %s", err.Error())))
		}

		m.addMessage(colorSuccess(fmt.Sprintf("✓ Switched to %s (%s)", provider, m.currentModel)))
		return nil

	case "/skills":
		m.addMessage(m.renderSkillsList())
		return nil

	case "/memory":
		if len(args) == 0 {
			m.addMessage(m.renderMemoryList())
		} else if args[0] == "clear" {
			m.addMessage(colorWarn("Memory clear not implemented in TUI. Use CLI: soulgate memory clear"))
		} else {
			m.addMessage(colorError("Usage: /memory [clear]"))
		}
		return nil

	case "/mcp":
		m.addMessage(m.renderMCPStatus())
		return nil

	case "/debug":
		m.addMessage(m.renderDebugInfo())
		return nil

	case "/stream":
		m.streamingEnabled = !m.streamingEnabled
		if m.streamingEnabled {
			m.addMessage(colorSuccess("Streaming enabled - responses will appear token by token"))
		} else {
			m.addMessage(colorMuted("Streaming disabled - responses will appear when complete"))
		}
		return nil

	case "/hub":
		// Hub browser
		if len(args) == 0 {
			m.addMessage(renderHubOverview())
		} else {
			subCmd := args[0]
			switch subCmd {
			case "plugins":
				m.addMessage(renderHubPlugins())
			case "skills":
				m.addMessage(renderHubSkills())
			case "installed":
				m.addMessage(renderHubInstalled())
			case "install":
				if len(args) > 1 {
					m.addMessage(colorAccent(fmt.Sprintf("Installing %s... (use CLI for actual installation)", args[1])))
					m.addMessage(colorMuted(fmt.Sprintf("Run: soulgate hub install %s", args[1])))
				} else {
					m.addMessage(colorError("Usage: /hub install <name>"))
				}
			case "search":
				if len(args) > 1 {
					query := strings.Join(args[1:], " ")
					m.addMessage(colorAccent(fmt.Sprintf("Searching for: %s... (use CLI for full results)", query)))
					m.addMessage(colorMuted(fmt.Sprintf("Run: soulgate hub search %s", query)))
				} else {
					m.addMessage(colorError("Usage: /hub search <query>"))
				}
			default:
				m.addMessage(colorError("Unknown hub command: " + subCmd))
			}
		}
		return nil

	case "/setup":
		// Interactive setup wizard
		if len(args) == 0 {
			// Show all integrations
			m.showSetupWizard = true
			m.setupStep = 0 // Integration selection
			m.setupFieldValues = make(map[string]string)
			m.addMessage(colorAccent("Starting setup wizard..."))
			return nil
		} else {
			// Setup specific integration
			integrationID := args[0]
			m.showSetupWizard = true
			m.setupStep = 1 // Field input
			m.setupIntegrationID = integrationID
			m.setupCurrentField = 0
			m.setupFieldValues = make(map[string]string)
			m.addMessage(colorAccent(fmt.Sprintf("Setting up %s integration...", integrationID)))
			return nil
		}

	case "/onboarding":
		// Start interactive onboarding wizard
		m.StartOnboardingWizard()
		m.addMessage(colorAccent("Starting onboarding wizard..."))
		return nil

	case "/soul":
		if len(args) == 0 {
			m.addMessage(m.renderSoulInfo())
		} else if args[0] == "init" {
			m.addMessage(m.initSoul())
		} else if args[0] == "reset" {
			m.addMessage(m.resetSoul())
		} else {
			m.addMessage(colorError("Usage: /soul [init|reset]"))
		}
		return nil

	case "/schedule":
		if len(args) == 0 {
			m.addMessage(m.renderScheduleInfo())
		} else {
			m.addMessage(colorMuted("Schedule management: use CLI 'soulgate schedule add'"))
		}
		return nil

	case "/think":
		if len(args) == 0 {
			m.addMessage(colorAccent(fmt.Sprintf("Thinking level: %s", m.orch.GetDirectives().ThinkingLevel)))
			m.addMessage(colorMuted("  Usage: /think <off|minimal|low|medium|high|xhigh|adaptive>"))
		} else {
			_, applied := core.ParseDirectives("/think "+args[0], m.orch.GetDirectives())
			if len(applied) > 0 {
				m.addMessage(colorSuccess(fmt.Sprintf("Set: %s", applied[0])))
			} else {
				m.addMessage(colorError("Invalid level. Use: off, minimal, low, medium, high, xhigh, adaptive"))
			}
		}
		return nil

	case "/fast":
		toggle := "on"
		if len(args) > 0 {
			toggle = args[0]
		}
		_, applied := core.ParseDirectives("/fast "+toggle, m.orch.GetDirectives())
		if len(applied) > 0 {
			m.addMessage(colorSuccess(fmt.Sprintf("Set: %s", applied[0])))
		}
		return nil

	case "/verbose":
		if len(args) == 0 {
			m.addMessage(colorAccent(fmt.Sprintf("Verbose: %s", m.orch.GetDirectives().VerboseMode)))
		} else {
			_, applied := core.ParseDirectives("/verbose "+args[0], m.orch.GetDirectives())
			if len(applied) > 0 {
				m.addMessage(colorSuccess(fmt.Sprintf("Set: %s", applied[0])))
			}
		}
		return nil

	case "/processes":
		procs := m.orch.GetProcessManager().List()
		if len(procs) == 0 {
			m.addMessage(colorMuted("  No background processes running."))
		} else {
			var sb strings.Builder
			sb.WriteString(colorAccentBright("Background Processes:\n"))
			for _, p := range procs {
				sb.WriteString(fmt.Sprintf("  %s  %-10s  pid:%d  %s\n", p.ID, p.Status, p.PID, p.Command))
			}
			m.addMessage(sb.String())
		}
		return nil

	case "/agent", "/agents":
		if len(args) == 0 {
			m.addMessage(m.renderAgentList())
		} else {
			agentID := args[0]
			// Support bare number: /agent 1 → agent_1
			if len(agentID) > 0 && agentID[0] >= '0' && agentID[0] <= '9' {
				agentID = "agent_" + agentID
			}
			m.addMessage(m.renderAgentDetail(agentID))

			// If agent is running, start polling for live updates
			agent, ok := m.orch.GetAgentManager().Get(agentID)
			if ok && agent.Status == core.AgentRunning {
				m.watchingAgentID = agentID
				return agentPollCmd(agentID)
			}
		}
		return nil

	case "/trust":
		m.orch.SetTrustMode(!m.orch.IsTrustMode())
		if m.orch.IsTrustMode() {
			m.addMessage(colorSuccess("Trust mode ON — all permissions auto-approved"))
		} else {
			m.addMessage(colorMuted("Trust mode OFF — policy enforcement restored"))
		}
		return nil

	case "/cron":
		jobs := m.orch.GetCronScheduler().List()
		if len(jobs) == 0 {
			m.addMessage(colorMuted("  No scheduled jobs. AI can create them with cron_add tool."))
		} else {
			var sb strings.Builder
			sb.WriteString(colorAccentBright("Scheduled Jobs:\n"))
			for _, j := range jobs {
				next := "-"
				if j.NextRun != nil {
					next = j.NextRun.Format("15:04:05")
				}
				sb.WriteString(fmt.Sprintf("  %s  %-15s  %-8s  %-10s  next:%s\n", j.ID, j.Name, j.Kind, j.Status, next))
			}
			m.addMessage(sb.String())
		}
		return nil

	case "/fork":
		return m.handleFork(args)

	case "/branches":
		m.addMessage(m.renderBranches())
		return nil

	case "/switch":
		if len(args) == 0 {
			m.addMessage(colorError("Usage: /switch <branch-id>"))
			return nil
		}
		return m.handleSwitch(args[0])

	case "/merge":
		if len(args) == 0 {
			m.addMessage(colorError("Usage: /merge <branch-id>"))
			return nil
		}
		return m.handleMerge(args[0])

	case "/new", "/reset":
		// Clear conversation history and restart fresh (context reset)
		m.messages = []string{welcomeMessage()}
		m.refreshOutput(true)
		m.orch.SetConversationHistory(nil)
		m.sessionTokensUsed = 0
		m.addMessage(colorSuccess("  Conversation reset. AI context cleared."))
		return nil

	case "/usage":
		session := m.orch.GetSession()
		var totalTokens int
		for _, run := range session.Runs {
			for _, call := range run.ModelCalls {
				totalTokens += call.TokensUsed
			}
		}
		// Also include the running session tally tracked by the TUI
		if m.sessionTokensUsed > totalTokens {
			totalTokens = m.sessionTokensUsed
		}
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(colorAccentBright("  Token & Cost Usage\n\n"))
		sb.WriteString(colorMuted(fmt.Sprintf("  Session ID   %s\n", session.ID)))
		sb.WriteString(colorMuted(fmt.Sprintf("  Runs         %d\n", len(session.Runs))))
		sb.WriteString(colorMuted(fmt.Sprintf("  Total tokens %d\n", totalTokens)))
		provider, modelName := m.orch.GetCurrentProvider()
		sb.WriteString(colorMuted(fmt.Sprintf("  Provider     %s\n", provider)))
		sb.WriteString(colorMuted(fmt.Sprintf("  Model        %s\n", modelName)))

		// Cost section
		if ct := m.orch.GetCostTracker(); ct != nil {
			summary := ct.Summary()
			sb.WriteString("\n")
			sb.WriteString(colorAccentBright("  Cost\n\n"))
			sb.WriteString(colorMuted(fmt.Sprintf("  Session      %s  (%d calls)\n",
				core.FormatCost(summary.SessionCost), summary.SessionCalls)))
			sb.WriteString(colorMuted(fmt.Sprintf("  Today        %s\n",
				core.FormatCost(summary.TodayCost))))
			sb.WriteString(colorMuted(fmt.Sprintf("  All time     %s  (%d calls)\n",
				core.FormatCost(summary.TotalCost), summary.TotalCalls)))
			if len(summary.ByProvider) > 0 {
				sb.WriteString("\n")
				sb.WriteString(colorAccentBright("  By Provider\n\n"))
				for p, c := range summary.ByProvider {
					sb.WriteString(colorMuted(fmt.Sprintf("  %-14s %s\n", p, core.FormatCost(c))))
				}
			}
			if len(summary.Last7Days) > 0 {
				sb.WriteString("\n")
				sb.WriteString(colorAccentBright("  Last 7 Days\n\n"))
				for _, dc := range summary.Last7Days {
					sb.WriteString(colorMuted(fmt.Sprintf("  %s   %s\n", dc.Date, core.FormatCost(dc.Cost))))
				}
			}
		}
		m.addMessage(sb.String())
		return nil

	case "/abort":
		if !m.thinking {
			m.addMessage(colorMuted("  No active generation to abort."))
			return nil
		}
		triggerAbort(m.cancelHandle)
		m.addMessage(colorWarn("  Generation aborted."))
		return nil

	case "/sessions":
		session := m.orch.GetSession()
		var sb strings.Builder
		sb.WriteString("\n")
		sb.WriteString(colorAccentBright("  Recent Sessions\n\n"))
		sb.WriteString(colorMuted(fmt.Sprintf("  Current  %s  started %s\n",
			session.ID,
			session.CreatedAt.Format("15:04:05"))))
		sb.WriteString(colorMuted(fmt.Sprintf("  Runs     %d\n", len(session.Runs))))
		sb.WriteString(colorMuted("\n  Past sessions are stored in the audit log.\n"))
		sb.WriteString(colorMuted("  Run: soulgate audit tail --last 20\n"))
		m.addMessage(sb.String())
		return nil

	case "/export":
		format := "md"
		if len(args) > 0 {
			format = strings.ToLower(args[0])
		}
		return m.exportConversation(format)

	case "/doctor":
		m.addMessage(renderDiagnostics(m.orch))
		return nil

	default:
		m.addMessage(colorError("Unknown command: " + cmdName))
		return nil
	}
}

// handleShellCommand handles shell commands prefixed with !
func (m InteractiveChatModel) handleShellCommand(cmd string) (InteractiveChatModel, tea.Cmd) {
	shellCmd := strings.TrimPrefix(cmd, "!")

	// Check if this is a sensitive shell command that requires confirmation
	if isSensitiveCommand(cmd) || isSensitiveCommand(shellCmd) {
		m.confirmation.Active = true
		m.confirmation.Message = getSensitiveMessage(cmd)
		m.confirmation.Command = cmd
		m.confirmation.PendingAction = func() tea.Cmd {
			return m.executeShellCommand(shellCmd)
		}
		return m, nil
	}

	// Execute non-sensitive command directly
	return m, m.executeShellCommand(shellCmd)
}

// executeShellCommand executes a shell command via the orchestrator
func (m *InteractiveChatModel) executeShellCommand(shellCmd string) tea.Cmd {
	m.addMessage(colorMuted("Executing: " + shellCmd))

	// Execute shell command via orchestrator's exec_command tool
	return func() tea.Msg {
		result, err := m.orch.Run(context.Background(), fmt.Sprintf("Execute this shell command: %s", shellCmd))
		if err != nil {
			return responseMsg{
				text: colorError("✗ Command failed: " + err.Error()),
			}
		}
		return responseMsg{
			text: colorSuccess("✓ Command executed\n") + result.Response,
		}
	}
}

// sendToAI sends a prompt to the AI and returns the response.
// When streaming is enabled, it sets up a stream callback that sends
// chunks to the TUI via the Bubble Tea program.
func (m *InteractiveChatModel) sendToAI(prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		cancel := func() {}
		if totalSec := m.orch.GetWorkspace().Config.Execution.TotalTimeoutSec; totalSec > 0 {
			ctx, cancel = context.WithTimeout(ctx, time.Duration(totalSec)*time.Second)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		// Register cancel so /abort and Esc can kill the in-flight run.
		// The double pointer survives Bubble Tea's model-copy semantics.
		noop := func() {}
		if m.cancelHandle != nil {
			*m.cancelHandle = cancel
		}
		defer func() {
			cancel()
			// Clear the handle after the run finishes so /abort knows there's nothing to abort.
			if m.cancelHandle != nil {
				*m.cancelHandle = noop
			}
		}()

		if m.teaProgram != nil && *m.teaProgram != nil {
			prog := *m.teaProgram

			// Always set up thinking callback for live thinking output
			m.orch.SetThinkingCallback(func(event core.ThinkingEvent) {
				prog.Send(thinkingEventMsg{event: event})
			})
			defer m.orch.SetThinkingCallback(nil)

			if m.streamingEnabled {
				// Enable streaming on the orchestrator with a callback that sends chunks to the TUI
				m.orch.SetStreaming(true, func(chunk string) {
					prog.Send(streamChunkMsg{chunk: chunk})
				})
				defer m.orch.SetStreaming(false, nil)
			}
		}

		result, err := m.orch.Run(ctx, prompt)
		if err != nil {
			return responseMsg{
				text: "",
				err:  fmt.Errorf("AI request failed: %w", err),
			}
		}

		if result == nil {
			return responseMsg{
				text: "",
				err:  fmt.Errorf("no response from AI (result is nil)"),
			}
		}

		return responseMsg{text: result.Response}
	}
}

// processGatewayMessage runs the orchestrator for an incoming Gateway message
// and returns a gatewayResponseMsg with the result
func (m *InteractiveChatModel) processGatewayMessage(frame *protocol.EventMessageFrame) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		cancel := func() {}
		if totalSec := m.orch.GetWorkspace().Config.Execution.TotalTimeoutSec; totalSec > 0 {
			ctx, cancel = context.WithTimeout(ctx, time.Duration(totalSec)*time.Second)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		defer cancel()

		if m.teaProgram != nil && *m.teaProgram != nil {
			prog := *m.teaProgram

			m.orch.SetThinkingCallback(func(event core.ThinkingEvent) {
				prog.Send(thinkingEventMsg{event: event})
			})
			defer m.orch.SetThinkingCallback(nil)

			if m.streamingEnabled {
				m.orch.SetStreaming(true, func(chunk string) {
					prog.Send(streamChunkMsg{chunk: chunk})
				})
				defer m.orch.SetStreaming(false, nil)
			}
		}

		result, err := m.orch.Run(ctx, frame.Text)
		if err != nil {
			return gatewayResponseMsg{
				frame: frame,
				err:   fmt.Errorf("AI request failed: %w", err),
			}
		}

		if result == nil {
			return gatewayResponseMsg{
				frame: frame,
				err:   fmt.Errorf("no response from AI"),
			}
		}

		return gatewayResponseMsg{
			frame:    frame,
			response: result.Response,
		}
	}
}

// installDependenciesCmd installs dependencies for configured integrations
func (m *InteractiveChatModel) installDependenciesCmd() tea.Cmd {
	return func() tea.Msg {
		if m.OnboardingState == nil {
			return nil
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Install dependencies
		if err := m.OnboardingState.InstallDependencies(ctx); err != nil {
			// Log error but don't fail onboarding
			m.OnboardingState.DependencyErrors = append(
				m.OnboardingState.DependencyErrors,
				fmt.Sprintf("Installation error: %v", err),
			)
		}

		// Return a message to trigger re-render
		return dependencyInstallCompleteMsg{}
	}
}

// triggerAbort calls the active context cancel func via the handle pointer.
// Safe to call when handle is nil or when no run is active (noop).
func triggerAbort(handle *func()) {
	if handle == nil || *handle == nil {
		return
	}
	(*handle)()
}

// exportConversation writes the current conversation to a file in the workspace.
func (m *InteractiveChatModel) exportConversation(format string) tea.Cmd {
	switch format {
	case "json", "md", "html":
		// supported
	default:
		m.addMessage(colorError(fmt.Sprintf("  Unknown export format %q. Use: json, md, html", format)))
		return nil
	}

	workspace := m.orch.GetWorkspace()
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("conversation-%s.%s", ts, format)
	outPath := filepath.Join(workspace.Root, filename)

	history := m.orch.GetConversationHistory()

	var content string
	switch format {
	case "json":
		data, err := json.MarshalIndent(history, "", "  ")
		if err != nil {
			m.addMessage(colorError("  Export failed: " + err.Error()))
			return nil
		}
		content = string(data)

	case "md":
		var sb strings.Builder
		sb.WriteString("# SoulGate Conversation Export\n\n")
		sb.WriteString(fmt.Sprintf("Exported: %s\n", time.Now().Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Session: %s\n\n", m.orch.GetSession().ID))
		sb.WriteString("---\n\n")
		for _, msg := range history {
			role := strings.Title(msg.Role) //nolint:staticcheck
			sb.WriteString(fmt.Sprintf("**%s**\n\n%s\n\n---\n\n", role, msg.Content))
		}
		content = sb.String()

	case "html":
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
		sb.WriteString("<meta charset=\"utf-8\">\n")
		sb.WriteString("<title>SoulGate Conversation</title>\n")
		sb.WriteString("<style>body{font-family:sans-serif;max-width:800px;margin:2rem auto}blockquote{border-left:3px solid #ccc;margin:0;padding-left:1rem}h3{color:#666}</style>\n")
		sb.WriteString("</head>\n<body>\n")
		sb.WriteString(fmt.Sprintf("<h1>Conversation Export</h1>\n<p>%s — session %s</p>\n<hr>\n",
			time.Now().Format(time.RFC3339), m.orch.GetSession().ID))
		for _, msg := range history {
			tag := "blockquote"
			label := "User"
			if msg.Role == "assistant" {
				tag = "div"
				label = "Assistant"
			}
			sb.WriteString(fmt.Sprintf("<h3>%s</h3>\n<%s>%s</%s>\n<hr>\n",
				label, tag, htmlEscape(msg.Content), tag))
		}
		sb.WriteString("</body>\n</html>\n")
		content = sb.String()
	}

	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		m.addMessage(colorError("  Export failed: " + err.Error()))
		return nil
	}

	m.addMessage(colorSuccess(fmt.Sprintf("  Exported %d messages to %s", len(history), filename)))
	return nil
}

// htmlEscape performs minimal HTML escaping for the export feature.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return strings.ReplaceAll(s, "\n", "<br>\n")
}

// renderDiagnostics renders a brief system diagnostics report inline.
func renderDiagnostics(orch *core.Orchestrator) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(colorAccentBright("  Diagnostics\n\n"))

	// Provider check
	provider, modelName := orch.GetCurrentProvider()
	sb.WriteString(colorMuted(fmt.Sprintf("  Provider    %s\n", provider)))
	sb.WriteString(colorMuted(fmt.Sprintf("  Model       %s\n", modelName)))

	// API key check
	apiKeyEnvs := map[string]string{
		"openai":    "OPENAI_API_KEY",
		"anthropic": "ANTHROPIC_API_KEY",
	}
	for p, envVar := range apiKeyEnvs {
		val := os.Getenv(envVar)
		if val != "" {
			if p == provider {
				sb.WriteString(colorSuccess(fmt.Sprintf("  %-11s %s (active)\n", envVar, maskKey(val))))
			} else {
				sb.WriteString(colorMuted(fmt.Sprintf("  %-11s %s\n", envVar, maskKey(val))))
			}
		} else {
			if p == provider {
				sb.WriteString(colorError(fmt.Sprintf("  %-11s not set — provider will fail\n", envVar)))
			} else {
				sb.WriteString(colorMuted(fmt.Sprintf("  %-11s not set\n", envVar)))
			}
		}
	}

	// Workspace
	ws := orch.GetWorkspace()
	sb.WriteString(colorMuted(fmt.Sprintf("\n  Workspace   %s\n", ws.Root)))
	sb.WriteString(colorMuted(fmt.Sprintf("  Config dir  %s\n", ws.ConfigDir)))

	// Session
	session := orch.GetSession()
	sb.WriteString(colorMuted(fmt.Sprintf("\n  Session     %s\n", session.ID)))
	sb.WriteString(colorMuted(fmt.Sprintf("  Runs        %d\n", len(session.Runs))))

	// MCP
	mcpMgr := orch.GetMCPManager()
	if mcpMgr != nil {
		servers := mcpMgr.ListServers()
		running := 0
		for _, s := range servers {
			if s.Running {
				running++
			}
		}
		sb.WriteString(colorMuted(fmt.Sprintf("\n  MCP servers %d configured, %d running\n", len(servers), running)))
	}

	// Trust mode
	if orch.IsTrustMode() {
		sb.WriteString(colorWarn("  Trust mode  ON\n"))
	} else {
		sb.WriteString(colorMuted("  Trust mode  off\n"))
	}

	sb.WriteString(colorSuccess("\n  All checks done.\n"))
	return sb.String()
}

// maskKey shows only the first 8 chars of an API key.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:8] + "..."
}

// ---------------------------------------------------------------------------
// Branch management slash commands
// ---------------------------------------------------------------------------

// handleFork creates a new conversation branch at the end of the current history.
// Usage: /fork [label]
func (m *InteractiveChatModel) handleFork(args []string) tea.Cmd {
	label := strings.Join(args, " ")

	// Fork at the end of the current history — capture the full current context.
	history := m.orch.GetConversationHistory()
	forkPoint := len(history)

	newID, err := m.orch.ForkConversation(label, forkPoint)
	if err != nil {
		m.addMessage(colorError(fmt.Sprintf("  Fork failed: %s", err.Error())))
		return nil
	}

	bm := m.orch.GetBranchManager()
	b, ok := bm.GetBranch(newID)
	if ok && b.Label != "" {
		m.addMessage(colorSuccess(fmt.Sprintf("  Forked into branch %q (%s)", b.Label, shortID(newID))))
	} else {
		m.addMessage(colorSuccess(fmt.Sprintf("  Forked into branch %s", shortID(newID))))
	}
	m.addMessage(colorMuted("  Conversation continues on the new branch. Use /branches to list all."))
	return nil
}

// handleSwitch switches the active conversation branch.
func (m *InteractiveChatModel) handleSwitch(branchID string) tea.Cmd {
	// Allow partial IDs: find the first branch whose ID contains branchID.
	bm := m.orch.GetBranchManager()
	resolved := resolveBranchID(bm, branchID)
	if resolved == "" {
		m.addMessage(colorError(fmt.Sprintf("  No branch matching %q. Use /branches to list.", branchID)))
		return nil
	}

	if err := m.orch.SwitchBranch(resolved); err != nil {
		m.addMessage(colorError(fmt.Sprintf("  Switch failed: %s", err.Error())))
		return nil
	}

	b, ok := bm.GetBranch(resolved)
	label := resolved
	if ok && b.Label != "" {
		label = b.Label
	}
	msgs := bm.GetCurrentMessages()
	m.addMessage(colorSuccess(fmt.Sprintf("  Switched to branch %q (%d messages in context)", label, len(msgs))))
	return nil
}

// handleMerge merges a branch's unique messages into the current branch.
func (m *InteractiveChatModel) handleMerge(branchID string) tea.Cmd {
	bm := m.orch.GetBranchManager()
	resolved := resolveBranchID(bm, branchID)
	if resolved == "" {
		m.addMessage(colorError(fmt.Sprintf("  No branch matching %q. Use /branches to list.", branchID)))
		return nil
	}

	if err := m.orch.MergeBranch(resolved); err != nil {
		m.addMessage(colorError(fmt.Sprintf("  Merge failed: %s", err.Error())))
		return nil
	}

	b, ok := bm.GetBranch(resolved)
	label := resolved
	if ok && b.Label != "" {
		label = b.Label
	}
	msgs := bm.GetCurrentMessages()
	m.addMessage(colorSuccess(fmt.Sprintf("  Merged branch %q into current branch (%d messages now)", label, len(msgs))))
	return nil
}

// renderBranches renders the list of all branches in a compact table.
func (m *InteractiveChatModel) renderBranches() string {
	bm := m.orch.GetBranchManager()
	branches := bm.List()

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(colorAccentBright("  Conversation Branches\n\n"))

	if len(branches) == 0 {
		sb.WriteString(colorMuted("  No branches found.\n"))
		return sb.String()
	}

	for _, b := range branches {
		marker := "  "
		if b.IsCurrent {
			marker = colorSuccess("* ")
		} else {
			marker = colorMuted("  ")
		}

		label := b.Label
		if label == "" {
			label = "(unlabelled)"
		}

		parent := ""
		if b.ParentID != "" {
			parent = fmt.Sprintf("  parent:%s", shortID(b.ParentID))
		}

		line := fmt.Sprintf("%s%-20s  %s  %2d msgs  %s%s\n",
			marker,
			label,
			shortID(b.ID),
			b.MessageCount,
			b.CreatedAt.Format("15:04:05"),
			parent,
		)

		if b.IsCurrent {
			sb.WriteString(colorSuccess(line))
		} else {
			sb.WriteString(colorMuted(line))
		}
	}

	sb.WriteString("\n")
	sb.WriteString(colorMuted("  /fork [label]     fork at current message\n"))
	sb.WriteString(colorMuted("  /switch <id>      switch to branch\n"))
	sb.WriteString(colorMuted("  /merge <id>       merge branch into current\n"))

	return sb.String()
}

// resolveBranchID finds the full branch ID given a partial string.
// Exact matches win; otherwise the first branch whose ID or label contains
// the query (case-insensitive) is returned. Returns "" if nothing matches.
func resolveBranchID(bm *core.BranchManager, query string) string {
	list := bm.List()
	query = strings.ToLower(query)
	for _, b := range list {
		if b.ID == query {
			return b.ID
		}
	}
	for _, b := range list {
		if strings.Contains(strings.ToLower(b.ID), query) ||
			strings.Contains(strings.ToLower(b.Label), query) {
			return b.ID
		}
	}
	return ""
}

// shortID returns the last 8 characters of a branch ID for compact display.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[len(id)-8:]
}
