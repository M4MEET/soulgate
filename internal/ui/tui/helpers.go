package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/ui/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	maxRenderedMessages    = 0 // 0 = unlimited in-session scrollback
	streamFlushInterval    = 35 * time.Millisecond
	dependencyTickInterval = 120 * time.Millisecond
	agentPollInterval      = 300 * time.Millisecond
	thinkingTickInterval   = 70 * time.Millisecond
)

var availableSlashCommands = []string{
	"/status", "/tools", "/skills", "/memory", "/soul", "/schedule",
	"/history", "/clear", "/help", "/context", "/model", "/mcp", "/debug", "/hub",
	"/setup", "/onboarding", "/stream", "/think", "/fast", "/verbose",
	"/processes", "/cron", "/agent", "/agents", "/trust", "/exit", "/quit",
	// Extended command set
	"/new", "/reset", "/usage", "/abort", "/sessions", "/export", "/doctor",
	// Conversation branching
	"/fork", "/branches", "/switch", "/merge",
}

// addMessage adds a message to the chat history and updates the display.
func (m *InteractiveChatModel) addMessage(text string) {
	m.messages = append(m.messages, text)
	m.trimMessageBuffer()
	m.refreshOutput(true)
}

func (m *InteractiveChatModel) trimMessageBuffer() {
	if maxRenderedMessages <= 0 {
		return
	}

	if len(m.messages) <= maxRenderedMessages {
		return
	}

	dropped := len(m.messages) - (maxRenderedMessages - 1)
	notice := colorMuted(fmt.Sprintf("  ... %d older messages hidden to keep UI responsive ...", dropped))
	tail := append([]string{notice}, m.messages[len(m.messages)-(maxRenderedMessages-1):]...)
	m.messages = tail

	// Keep active panel indices in sync after dropping head messages.
	if m.thinkingPanelIndex >= 0 {
		m.thinkingPanelIndex -= dropped
		if m.thinkingPanelIndex < 0 || m.thinkingPanelIndex >= len(m.messages) {
			m.thinkingPanelIndex = -1
		}
	}
	if m.streamPanelIndex >= 0 {
		m.streamPanelIndex -= dropped
		if m.streamPanelIndex < 0 || m.streamPanelIndex >= len(m.messages) {
			m.streamPanelIndex = -1
		}
	}
}

func (m *InteractiveChatModel) refreshOutput(stickBottom bool) {
	content := strings.Join(m.messages, "\n\n")
	if content != m.lastRenderedContent {
		m.output.SetContent(content)
		m.lastRenderedContent = content
	}
	if stickBottom && m.autoScroll {
		m.output.GotoBottom()
	}
}

func (m *InteractiveChatModel) setLastMessage(text string, stickBottom bool) {
	if len(m.messages) == 0 {
		m.addMessage(text)
		return
	}
	m.messages[len(m.messages)-1] = text
	m.refreshOutput(stickBottom)
}

func (m *InteractiveChatModel) isValidMessageIndex(index int) bool {
	return index >= 0 && index < len(m.messages)
}

func (m *InteractiveChatModel) setMessageAt(index int, text string, stickBottom bool) {
	if !m.isValidMessageIndex(index) {
		m.addMessage(text)
		return
	}
	m.messages[index] = text
	m.refreshOutput(stickBottom)
}

func (m *InteractiveChatModel) ensureStreamPanel() {
	if m.isValidMessageIndex(m.streamPanelIndex) {
		return
	}
	m.messages = append(m.messages, formatAIStreamingResponse(""))
	m.streamPanelIndex = len(m.messages) - 1
	m.trimMessageBuffer()
	m.refreshOutput(true)
}

func streamFlushCmd() tea.Cmd {
	return tea.Tick(streamFlushInterval, func(t time.Time) tea.Msg {
		return streamFlushMsg{}
	})
}

func dependencyTickCmd() tea.Cmd {
	return tea.Tick(dependencyTickInterval, func(t time.Time) tea.Msg {
		return dependencyProgressMsg{}
	})
}

func thinkingTickCmd() tea.Cmd {
	return tea.Tick(thinkingTickInterval, func(t time.Time) tea.Msg {
		return thinkingMsg{}
	})
}

func agentPollCmd(agentID string) tea.Cmd {
	return tea.Tick(agentPollInterval, func(t time.Time) tea.Msg {
		return agentPollMsg{agentID: agentID}
	})
}

func (m *InteractiveChatModel) scheduleStreamFlush() tea.Cmd {
	if m.streamFlushScheduled {
		return nil
	}
	m.streamFlushScheduled = true
	return streamFlushCmd()
}

func (m *InteractiveChatModel) flushStreamingPreview() {
	if !m.streamFlushScheduled || len(m.messages) == 0 {
		m.streamFlushScheduled = false
		return
	}
	m.streamFlushScheduled = false

	if m.thinkingBuffer != "" {
		m.ensureThinkingPlaceholder()
		if m.isValidMessageIndex(m.thinkingPanelIndex) {
			m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
		}
	}

	if m.isValidMessageIndex(m.streamPanelIndex) {
		m.setMessageAt(m.streamPanelIndex, formatAIStreamingResponse(m.streamBuffer), true)
	}
}

func (m *InteractiveChatModel) maskedOnboardingKey() string {
	if m.onboardingInput == "" {
		return ""
	}
	if len(m.onboardingInput) <= 4 {
		return strings.Repeat("*", len(m.onboardingInput))
	}
	return strings.Repeat("*", len(m.onboardingInput)-4) + m.onboardingInput[len(m.onboardingInput)-4:]
}

func onboardingEnvVar(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "google":
		return "GOOGLE_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "mistral":
		return "MISTRAL_API_KEY"
	case "cohere":
		return "COHERE_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "xai":
		return "XAI_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "together":
		return "TOGETHER_API_KEY"
	case "perplexity":
		return "PERPLEXITY_API_KEY"
	case "ollama":
		return ""
	default:
		return strings.ToUpper(provider) + "_API_KEY"
	}
}

func providerNeedsKey(provider string) bool {
	return strings.ToLower(strings.TrimSpace(provider)) != "ollama"
}

func keyNeedsRedraw(k tea.KeyMsg) bool {
	switch k.Type {
	case tea.KeyRunes, tea.KeyBackspace, tea.KeyDelete, tea.KeySpace:
		return true
	default:
		return false
	}
}

func sanitizeToolNameForDisplay(tool string) string {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return tool
	}
	return strings.ReplaceAll(tool, "__", ".")
}

func isBuiltinToolName(name string) bool {
	switch name {
	case "files_read", "files_list", "files_write", "files_delete",
		"exec_command", "net_request",
		"memory_write", "memory_get", "memory_search",
		"switch_model",
		"agent_create", "agent_list", "agent_stop",
		"web_search", "web_fetch",
		"process_start", "process_list", "process_poll", "process_log", "process_write", "process_kill",
		"pdf_read",
		"cron_add", "cron_list", "cron_remove", "cron_pause", "cron_resume",
		"llm_task", "apply_patch":
		return true
	default:
		return false
	}
}

// updateAutocomplete updates the autocomplete suggestions based on current input.
func (m *InteractiveChatModel) updateAutocomplete() {
	value := m.input.Value()
	if strings.HasPrefix(value, "/") {
		results := components.FuzzyFilter(value, availableSlashCommands)

		// Filter out exact matches (don't suggest what's already typed).
		var newAutocomplete []string
		for _, r := range results {
			if r.Text != value {
				newAutocomplete = append(newAutocomplete, r.Text)
			}
		}

		wasShowing := m.showAutocomplete
		m.showAutocomplete = len(newAutocomplete) > 0

		if !wasShowing && m.showAutocomplete {
			m.autocompleteIndex = 0
		}

		m.autocomplete = newAutocomplete
	} else {
		m.showAutocomplete = false
		m.autocomplete = []string{}
		m.autocompleteIndex = 0
	}
}

// isSensitiveCommand checks if a command is potentially dangerous.
func isSensitiveCommand(cmd string) bool {
	cmdLower := strings.ToLower(strings.TrimSpace(cmd))
	cmdLower = strings.TrimPrefix(cmdLower, "!")

	patterns := []string{
		"rm -rf", "rm -r", "rm ", "sudo rm", "delete", "files_delete", "rmdir", "unlink",
		"sudo ", "su ", "doas ",
		"mkfs", "dd if=", "format", "fdisk", "parted",
		"shutdown", "reboot", "halt", "poweroff",
		"systemctl stop", "systemctl disable",
		"kill -9", "pkill", "killall",
		"chmod 777", "chmod -r 777", "chown",
		"curl | sh", "wget | sh", "curl | bash", "wget | bash",
	}

	for _, pattern := range patterns {
		if strings.Contains(cmdLower, pattern) {
			return true
		}
	}
	return false
}

// getSensitiveMessage returns a warning message for sensitive commands.
func getSensitiveMessage(cmd string) string {
	cmdLower := strings.ToLower(cmd)
	switch {
	case strings.Contains(cmdLower, "rm -rf"):
		return "Recursive force delete - files CANNOT be recovered!"
	case strings.Contains(cmdLower, "rm -r") || strings.Contains(cmdLower, "rmdir"):
		return "This will delete directories and their contents."
	case strings.Contains(cmdLower, "rm ") || strings.Contains(cmdLower, "delete"):
		return "This command will DELETE files permanently."
	case strings.Contains(cmdLower, "sudo") || strings.Contains(cmdLower, "su "):
		return "This command requires ELEVATED PRIVILEGES."
	case strings.Contains(cmdLower, "shutdown") || strings.Contains(cmdLower, "reboot") ||
		strings.Contains(cmdLower, "halt") || strings.Contains(cmdLower, "poweroff"):
		return "This will SHUTDOWN or REBOOT your system."
	case strings.Contains(cmdLower, "kill") || strings.Contains(cmdLower, "pkill"):
		return "This will TERMINATE running processes."
	case strings.Contains(cmdLower, "mkfs") || strings.Contains(cmdLower, "format") ||
		strings.Contains(cmdLower, "fdisk") || strings.Contains(cmdLower, "dd if="):
		return "This can DESTROY entire disk partitions!"
	case strings.Contains(cmdLower, "chmod 777"):
		return "This makes files world-writable!"
	case strings.Contains(cmdLower, "curl | sh") || strings.Contains(cmdLower, "wget | bash"):
		return "Executing remote scripts without review!"
	default:
		return "This is a potentially dangerous operation."
	}
}

// formatAIResponse delegates to the components package for assistant message rendering.
// Retained as a package-level function for call-sites that haven't been migrated yet.
func formatAIResponse(text string) string {
	return components.FormatAssistantMessage(text)
}

// formatAIStreamingResponse is a lightweight renderer used during token streaming.
func formatAIStreamingResponse(text string) string {
	return components.FormatAssistantStreamingMessage(text)
}

// formatThinkingToolCall delegates to the components package.
func formatThinkingToolCall(toolName string, args string) string {
	return components.FormatToolCall(toolName, args)
}

// formatThinkingToolResult delegates to the components package.
func formatThinkingToolResult(toolName string, result string, duration time.Duration) string {
	return components.FormatToolResult(toolName, result, duration)
}

// formatThinkingPanel delegates to the components package.
func formatThinkingPanel(content string) string {
	return components.FormatThinkingPanel(content)
}

// formatThinkingIteration delegates to the components package.
func formatThinkingIteration(iteration int) string {
	return components.FormatThinkingIteration(iteration)
}

// formatThinkingModelCall delegates to the components package.
func formatThinkingModelCall(provider string) string {
	return components.FormatThinkingModelCall(provider)
}

// formatThinkingModelDone delegates to the components package.
func formatThinkingModelDone(modelName string, stopReason string, tokens int, duration time.Duration) string {
	return components.FormatThinkingModelDone(modelName, stopReason, tokens, duration)
}

// formatThinkingStatus delegates to the components package.
func formatThinkingStatus(message string) string {
	return components.FormatThinkingStatus(message)
}

// formatThinkingTokenUsage delegates to the components package.
func formatThinkingTokenUsage(total int) string {
	return components.FormatThinkingTokenUsage(total)
}

// ensureThinkingPlaceholder makes sure there's a placeholder message for thinking output.
func (m *InteractiveChatModel) ensureThinkingPlaceholder() {
	if m.isValidMessageIndex(m.thinkingPanelIndex) {
		return
	}

	placeholder := components.FormatThinkingPanel("")

	// In streaming mode, keep "thinking" above the streamed assistant panel.
	if m.isValidMessageIndex(m.streamPanelIndex) {
		insertAt := m.streamPanelIndex
		m.messages = append(m.messages, "")
		copy(m.messages[insertAt+1:], m.messages[insertAt:])
		m.messages[insertAt] = placeholder
		m.thinkingPanelIndex = insertAt
		m.streamPanelIndex++
		m.trimMessageBuffer()
		m.refreshOutput(true)
		return
	}

	m.messages = append(m.messages, placeholder)
	m.thinkingPanelIndex = len(m.messages) - 1
	m.trimMessageBuffer()
	m.refreshOutput(true)
}
