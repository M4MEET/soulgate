package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/ui/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// Custom message type for dependency installation completion
type dependencyInstallCompleteMsg struct{}

// Update handles all input events and state changes
func (m InteractiveChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Update time tracking for animations
	m.lastUpdate = time.Now()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle onboarding (absolute highest priority)
		if m.ShowOnboarding {
			return m.handleOnboardingInput(msg)
		}

		// Handle setup wizard (highest priority)
		if m.showSetupWizard {
			return m.handleSetupWizardInput(msg.String())
		}

		// Handle API key prompt (high priority)
		if m.showAPIKeyPrompt {
			return m.handleAPIKeyInput(msg)
		}

		// Handle model selector (highest priority after permission)
		if m.showModelSelector {
			if m.modelSelectionStep == 1 {
				// Step 1: Provider selection
				return m.handleProviderSelection(msg.String())
			} else {
				// Step 2: Model selection
				return m.handleModelSelection(msg.String())
			}
		}

		// Handle permission prompt first (highest priority)
		if handled, result := m.permission.HandleKey(msg.String()); handled {
			if result != nil {
				if result.Approved {
					if result.LearnPattern {
						m.addMessage(colorSuccess("✓ Permission granted and learned!"))
					} else {
						m.addMessage(colorSuccess("✓ Permission granted (this time)"))
					}
				} else {
					m.addMessage(colorError("✗ Permission denied"))
				}
			}
			return m, nil
		}

		// Handle confirmation dialog
		if m.confirmation.Active {
			pendingAction := m.confirmation.PendingAction
			handled, confirmed := m.confirmation.HandleKey(msg.String())
			if handled {
				if confirmed {
					m.addMessage(colorSuccess("✓ Confirmed"))
					if pendingAction != nil {
						return m, pendingAction()
					}
				} else {
					m.addMessage(colorMuted("✗ Cancelled"))
				}
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyCtrlD:
			// Exit only when input is empty (matches shell behaviour)
			if m.input.Value() == "" {
				return m, tea.Quit
			}
			return m, nil

		case tea.KeyCtrlL:
			// Clear screen
			m.messages = []string{welcomeMessage()}
			m.refreshOutput(true)
			return m, nil

		case tea.KeyCtrlH:
			// Toggle the help overlay
			m.showHelpOverlay = !m.showHelpOverlay
			return m, nil

		case tea.KeyCtrlN:
			// New conversation: clear messages and AI context
			m.messages = []string{welcomeMessage()}
			m.refreshOutput(true)
			m.orch.SetConversationHistory(nil)
			m.sessionTokensUsed = 0
			m.addMessage(colorSuccess("  New conversation started. AI context cleared."))
			return m, nil

		case tea.KeyCtrlG:
			// Open agent list
			m.addMessage(m.renderAgentList())
			return m, nil

		case tea.KeyCtrlT:
			// Toggle live thinking display
			m.showThinkingOutput = !m.showThinkingOutput
			if m.showThinkingOutput {
				m.addMessage(colorSuccess("  Live thinking output: on"))
			} else {
				m.addMessage(colorMuted("  Live thinking output: off"))
			}
			return m, nil

		case tea.KeyCtrlO:
			// Toggle verbose tool output
			m.showVerboseTools = !m.showVerboseTools
			if m.showVerboseTools {
				m.addMessage(colorSuccess("  Verbose tool output: on"))
			} else {
				m.addMessage(colorMuted("  Verbose tool output: off"))
			}
			return m, nil

		case tea.KeyUp:
			// If autocomplete is showing, navigate suggestions
			if m.showAutocomplete && len(m.autocomplete) > 0 {
				if m.autocompleteIndex > 0 {
					m.autocompleteIndex--
				} else {
					m.autocompleteIndex = len(m.autocomplete) - 1 // Wrap to bottom
				}
				return m, nil
			}

			// Otherwise, navigate command history (up = older)
			if len(m.history) > 0 {
				if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
					m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
					m.input.CursorEnd() // Move cursor to end
				}
			}
			return m, nil

		case tea.KeyDown:
			// If autocomplete is showing, navigate suggestions
			if m.showAutocomplete && len(m.autocomplete) > 0 {
				if m.autocompleteIndex < len(m.autocomplete)-1 {
					m.autocompleteIndex++
				} else {
					m.autocompleteIndex = 0 // Wrap to top
				}
				return m, nil
			}

			// Otherwise, navigate command history (down = newer)
			if m.historyIndex > 0 {
				m.historyIndex--
				m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
				m.input.CursorEnd()
			} else if m.historyIndex == 0 {
				m.historyIndex = -1
				m.input.SetValue("")
			}
			return m, nil

		case tea.KeyLeft, tea.KeyRight:
			// Let textinput handle left/right navigation within input
			// This moves cursor position in the text
			m.input, cmd = m.input.Update(msg)
			return m, cmd

		case tea.KeyHome:
			// Jump to start of input
			m.input.CursorStart()
			return m, nil

		case tea.KeyEnd:
			// Jump to end of input
			m.input.CursorEnd()
			return m, nil

		case tea.KeyCtrlA:
			// Jump to start (alternative)
			m.input.CursorStart()
			return m, nil

		case tea.KeyCtrlE:
			// Jump to end (alternative)
			m.input.CursorEnd()
			return m, nil

		case tea.KeyCtrlU:
			// Clear entire line
			m.input.SetValue("")
			return m, nil

		case tea.KeyCtrlK:
			// Delete from cursor to end
			pos := m.input.Position()
			current := m.input.Value()
			if pos < len(current) {
				m.input.SetValue(current[:pos])
			}
			return m, nil

		case tea.KeyCtrlW:
			// Delete word backwards
			pos := m.input.Position()
			current := m.input.Value()
			if pos > 0 {
				// Find previous word boundary
				newPos := pos - 1
				for newPos > 0 && current[newPos] == ' ' {
					newPos--
				}
				for newPos > 0 && current[newPos-1] != ' ' {
					newPos--
				}
				m.input.SetValue(current[:newPos] + current[pos:])
				m.input.SetCursor(newPos)
			}
			return m, nil

		case tea.KeyPgUp:
			// Scroll output up
			m.autoScroll = false
			m.output.HalfPageUp()
			return m, nil

		case tea.KeyPgDown:
			// Scroll output down
			m.output.HalfPageDown()
			if m.output.AtBottom() {
				m.autoScroll = true
			}
			return m, nil

		case tea.KeyEsc:
			// Close help overlay if open
			if m.showHelpOverlay {
				m.showHelpOverlay = false
				return m, nil
			}
			// Close autocomplete suggestions
			if m.showAutocomplete {
				m.showAutocomplete = false
				m.autocompleteIndex = 0
				return m, nil
			}
			// Abort current AI generation if thinking
			if m.thinking {
				triggerAbort(m.cancelHandle)
				m.addMessage(colorWarn("  Generation aborted."))
				return m, nil
			}
			return m, nil

		case tea.KeyTab:
			// Autocomplete - use selected suggestion
			if m.showAutocomplete && len(m.autocomplete) > 0 {
				m.input.SetValue(m.autocomplete[m.autocompleteIndex])
				m.input.CursorEnd()
				m.showAutocomplete = false
				m.autocompleteIndex = 0
			}
			return m, nil

		case tea.KeyEnter:
			// If autocomplete is showing, select highlighted suggestion
			if m.showAutocomplete && len(m.autocomplete) > 0 {
				m.input.SetValue(m.autocomplete[m.autocompleteIndex])
				m.input.CursorEnd()
				m.showAutocomplete = false
				m.autocompleteIndex = 0
				return m, nil
			}

			value := m.input.Value()
			if value == "" {
				return m, nil
			}

			// Add to history
			m.history = append(m.history, value)
			m.historyIndex = -1

			// Clear input and stop agent watching
			m.input.SetValue("")
			m.showAutocomplete = false
			m.autocompleteIndex = 0
			m.watchingAgentID = ""
			m.autoScroll = true

			// Add user message to output
			m.addMessage(components.FormatUserMessage(value))

			// Handle commands
			if strings.HasPrefix(value, "/") {
				return m.handleCommand(value)
			}

			// Handle shell commands
			if strings.HasPrefix(value, "!") {
				return m.handleShellCommand(value)
			}

			// Send to AI
			m.thinking = true
			m.status = "Thinking..."
			m.spinnerFrame = 0
			m.streamFlushScheduled = false
			m.streamBuffer = ""
			m.thinkingBuffer = ""
			m.streamPanelIndex = -1
			m.thinkingPanelIndex = -1

			if m.streamingEnabled {
				// In streaming mode, add a placeholder message for live updates
				m.ensureStreamPanel()
			}

			// Start both the AI request and thinking animation
			return m, tea.Batch(
				m.sendToAI(value),
				thinkingTickCmd(),
			)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.output.Width = msg.Width - 4
		// Header(3) + status(1) + separator(1) + input(1) + hints(1) + spacing(3) = 10
		viewportHeight := msg.Height - 10
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		m.output.Height = viewportHeight
		m.input.Width = msg.Width - 8
		if len(m.messages) > 0 {
			m.refreshOutput(false)
		}
		return m, nil

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.autoScroll = false
			m.output.LineUp(3)
			return m, nil
		case tea.MouseWheelDown:
			m.output.LineDown(3)
			if m.output.AtBottom() {
				m.autoScroll = true
			}
			return m, nil
		}

	case responseMsg:
		m.thinking = false
		m.status = "Ready"
		m.thinkingActivity = ""
		m.thinkingLog = nil
		m.streamFlushScheduled = false
		// Refresh model info to show actual model name from API response
		m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
		if msg.err != nil {
			errMsg := components.FormatErrorMessage(msg.err)
			if m.streamingEnabled && m.isValidMessageIndex(m.streamPanelIndex) {
				// Replace the stream placeholder
				m.setMessageAt(m.streamPanelIndex, errMsg, true)
			} else {
				m.addMessage(errMsg)
			}
		} else {
			if m.streamingEnabled && m.isValidMessageIndex(m.streamPanelIndex) {
				// Replace the stream placeholder with the final formatted response
				m.setMessageAt(m.streamPanelIndex, formatAIResponse(msg.text), true)
			} else {
				m.addMessage(formatAIResponse(msg.text))
			}
		}
		m.streamBuffer = ""
		m.thinkingBuffer = ""
		m.streamPanelIndex = -1
		m.thinkingPanelIndex = -1
		return m, nil

	case thinkingMsg:
		if m.thinking {
			// Advance spinner frame
			m.spinnerFrame = (m.spinnerFrame + 1) % 10
			// Advance waiting phrase every ~28 ticks (~2 seconds at 70ms per tick)
			m.waitingPhraseTick++
			if m.waitingPhraseTick >= 28 {
				m.waitingPhraseTick = 0
				m.waitingPhraseIndex = (m.waitingPhraseIndex + 1) % len(waitingPhrases)
			}
			// Update thinking animation
			return m, thinkingTickCmd()
		}
		return m, nil

	case PermissionRequestMsg:
		// Show permission prompt
		m.permission.Active = true
		m.permission.Request = &msg.Request
		m.permission.Response = msg.Response
		return m, nil

	case thinkingEventMsg:
		// Live thinking event from the agentic loop
		evt := msg.event

		// Keep a bounded log of recent events
		m.thinkingLog = append(m.thinkingLog, evt)
		if len(m.thinkingLog) > 50 {
			m.thinkingLog = m.thinkingLog[len(m.thinkingLog)-50:]
		}

		// Update status bar activity
		switch evt.Kind {
		case core.ThinkingIteration:
			m.thinkingActivity = fmt.Sprintf("iteration %d", evt.Iteration)

			// Show iteration marker in the chat output
			line := formatThinkingIteration(evt.Iteration)
			if m.streamingEnabled {
				m.thinkingBuffer += line
				return m, m.scheduleStreamFlush()
			} else {
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += line
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}

		case core.ThinkingModelCall:
			m.thinkingActivity = "calling model..."

			// Show model call in the chat output
			line := formatThinkingModelCall(evt.Provider)
			if m.streamingEnabled {
				m.thinkingBuffer += line
				return m, m.scheduleStreamFlush()
			} else {
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += line
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}

		case core.ThinkingModelDone:
			modelName := evt.Model
			if modelName == "" {
				modelName = evt.Provider
			}
			m.thinkingActivity = fmt.Sprintf("model: %s (%s, %d tok, %s)",
				modelName, evt.StopReason, evt.TokensUsed, evt.Duration.Round(time.Millisecond))
			// Accumulate tokens per model call for session-level tracking
			m.sessionTokensUsed += evt.TokensUsed

			// Show model response info in the chat output
			line := formatThinkingModelDone(modelName, evt.StopReason, evt.TokensUsed, evt.Duration)
			if m.streamingEnabled {
				m.thinkingBuffer += line
				return m, m.scheduleStreamFlush()
			} else {
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += line
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}

		case core.ThinkingToolStart:
			toolName := sanitizeToolNameForDisplay(evt.ToolName)
			if toolName == "" {
				toolName = evt.ToolName
			}
			m.thinkingActivity = fmt.Sprintf("running %s", toolName)

			// Show tool call in the chat output for live view
			toolLine := formatThinkingToolCall(toolName, evt.ToolArgs)
			if m.streamingEnabled {
				m.thinkingBuffer += toolLine
				return m, m.scheduleStreamFlush()
			} else {
				// Non-streaming: update the last thinking placeholder
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += toolLine
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}
		case core.ThinkingToolDone:
			toolName := sanitizeToolNameForDisplay(evt.ToolName)
			if toolName == "" {
				toolName = evt.ToolName
			}
			m.thinkingActivity = fmt.Sprintf("%s done (%s)", toolName, evt.Duration.Round(time.Millisecond))

			// Show abbreviated result in live view
			resultLine := formatThinkingToolResult(toolName, evt.ToolResult, evt.Duration)
			if m.streamingEnabled {
				m.thinkingBuffer += resultLine
				return m, m.scheduleStreamFlush()
			} else {
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += resultLine
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}

		case core.ThinkingTokenUsage:
			m.thinkingActivity = fmt.Sprintf("tokens used %d", evt.TokensUsed)
			// Accumulate session-level token count (status bar display)
			if evt.TokensUsed > m.sessionTokensUsed {
				m.sessionTokensUsed = evt.TokensUsed
			}

			line := formatThinkingTokenUsage(evt.TokensUsed)
			if m.streamingEnabled {
				m.thinkingBuffer += line
				return m, m.scheduleStreamFlush()
			} else {
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += line
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}

		case core.ThinkingStatus:
			if msg := strings.TrimSpace(evt.Message); msg != "" {
				m.thinkingActivity = msg
			}
			line := formatThinkingStatus(evt.Message)
			if m.streamingEnabled {
				m.thinkingBuffer += line
				return m, m.scheduleStreamFlush()
			} else {
				m.ensureThinkingPlaceholder()
				m.thinkingBuffer += line
				m.setMessageAt(m.thinkingPanelIndex, formatThinkingPanel(m.thinkingBuffer), true)
			}
		}
		return m, nil

	case streamChunkMsg:
		// Streaming token arrived - append to buffer and update display
		m.streamBuffer += msg.chunk
		return m, m.scheduleStreamFlush()

	case streamFlushMsg:
		m.flushStreamingPreview()
		return m, nil

	case agentPollMsg:
		// Refresh the watched agent's activity log
		if m.watchingAgentID == "" || m.watchingAgentID != msg.agentID {
			return m, nil
		}

		agent, ok := m.orch.GetAgentManager().Get(msg.agentID)
		if !ok {
			m.watchingAgentID = ""
			return m, nil
		}

		// Update the last message with fresh agent detail
		if len(m.messages) > 0 {
			m.setLastMessage(m.renderAgentDetail(msg.agentID), true)
		}

		// Keep polling if agent is still running
		if agent.Status == core.AgentRunning {
			return m, agentPollCmd(msg.agentID)
		}

		// Agent finished — do one final render and stop watching
		m.watchingAgentID = ""
		return m, nil

	case dependencyInstallCompleteMsg:
		// Dependency installation completed; auto-advance for a smoother flow.
		if m.ShowOnboarding && m.OnboardingState != nil {
			m.OnboardingState.InstallingDependencies = false
			if m.OnboardingState.GetCurrentStep().Name == "dependencies" {
				m.OnboardingState.NextStep()
			}
		}
		return m, nil

	case dependencyProgressMsg:
		if m.ShowOnboarding && m.OnboardingState != nil && m.OnboardingState.InstallingDependencies {
			m.onboardingSpinnerFrame = (m.onboardingSpinnerFrame + 1) % 10
			return m, dependencyTickCmd()
		}
		return m, nil

	case gatewayConnectedMsg:
		m.gatewayConnected = true
		m.addMessage(colorSuccess(fmt.Sprintf("Gateway connected as %s", msg.clientID)))
		return m, nil

	case gatewayDisconnectedMsg:
		m.gatewayConnected = false
		if msg.err != nil {
			m.addMessage(colorError(fmt.Sprintf("Gateway disconnected: %v", msg.err)))
		} else {
			m.addMessage(colorMuted("Gateway disconnected"))
		}
		return m, nil

	case gatewayMessageMsg:
		f := msg.frame
		// Display incoming channel message in TUI
		senderLabel := f.Sender.Username
		if senderLabel == "" {
			senderLabel = f.Sender.Name
		}
		if senderLabel == "" {
			senderLabel = f.Sender.ID
		}
		displayMsg := fmt.Sprintf("\n  %s  %s\n  %s\n",
			colorAccent(fmt.Sprintf("[%s] @%s", f.Channel, senderLabel)),
			colorMuted(time.Unix(f.Timestamp, 0).Format("15:04:05")),
			f.Text,
		)
		m.addMessage(displayMsg)

		// Auto-process with orchestrator
		m.thinking = true
		m.status = fmt.Sprintf("Processing [%s]...", f.Channel)
		m.spinnerFrame = 0
		m.streamFlushScheduled = false
		m.streamBuffer = ""
		m.thinkingBuffer = ""
		m.streamPanelIndex = -1
		m.thinkingPanelIndex = -1

		if m.streamingEnabled {
			m.ensureStreamPanel()
		}

		return m, tea.Batch(
			m.processGatewayMessage(f),
			thinkingTickCmd(),
		)

	case gatewayResponseMsg:
		m.thinking = false
		m.status = "Ready"
		m.thinkingActivity = ""
		m.thinkingLog = nil

		if msg.err != nil {
			errMsg := components.FormatErrorMessage(msg.err)
			if m.streamingEnabled && m.isValidMessageIndex(m.streamPanelIndex) {
				m.setMessageAt(m.streamPanelIndex, errMsg, true)
			} else {
				m.addMessage(errMsg)
			}
		} else {
			// Display response in TUI
			if m.streamingEnabled && m.isValidMessageIndex(m.streamPanelIndex) {
				m.setMessageAt(m.streamPanelIndex, formatAIResponse(msg.response), true)
			} else {
				m.addMessage(formatAIResponse(msg.response))
			}

			// Send response back to Gateway → channel
			gw := m.getGatewayClient()
			if gw != nil && msg.frame != nil {
				if err := gw.SendChannelResponse(
					msg.frame.Channel,
					msg.frame.ConversationID,
					msg.frame.SessionID,
					msg.response,
				); err != nil {
					m.addMessage(colorError(fmt.Sprintf("Failed to send to %s: %v", msg.frame.Channel, err)))
				}
			}
		}
		m.streamBuffer = ""
		m.thinkingBuffer = ""
		m.streamPanelIndex = -1
		m.thinkingPanelIndex = -1
		return m, nil
	}

	// Update input
	if !m.thinking {
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)

		// Update autocomplete
		m.updateAutocomplete()
	}

	// Update viewport
	m.output, cmd = m.output.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
