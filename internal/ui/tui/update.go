package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
			return m.handleOnboardingInput(msg.String())
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
		if m.showPermissionPrompt && m.permissionResponse != nil {
			switch msg.String() {
			case "a", "A":
				// Allow once
				m.showPermissionPrompt = false
				m.addMessage(colorSuccess("✓ Permission granted (this time)"))
				m.permissionResponse <- core.PermissionResponse{Approved: true, LearnPattern: false}
				m.permissionResponse = nil
				return m, nil

			case "l", "L":
				// Learn and always allow
				m.showPermissionPrompt = false
				m.addMessage(colorSuccess("✓ Permission granted and learned!"))
				m.permissionResponse <- core.PermissionResponse{Approved: true, LearnPattern: true}
				m.permissionResponse = nil
				return m, nil

			case "d", "D", "n", "N", "esc":
				// Deny
				m.showPermissionPrompt = false
				m.addMessage(colorError("✗ Permission denied"))
				m.permissionResponse <- core.PermissionResponse{Approved: false, LearnPattern: false}
				m.permissionResponse = nil
				return m, nil
			}
			// Ignore other keys when permission prompt is showing
			return m, nil
		}

		// Handle confirmation dialog
		if m.showConfirmation {
			switch msg.String() {
			case "y", "Y":
				// Confirm - execute pending action
				m.showConfirmation = false
				m.addMessage(colorSuccess("✓ Confirmed"))
				if m.pendingAction != nil {
					cmd := m.pendingAction()
					m.pendingAction = nil
					return m, cmd
				}
				return m, nil

			case "n", "N", "esc":
				// Cancel
				m.showConfirmation = false
				m.addMessage(colorMuted("✗ Cancelled"))
				m.pendingAction = nil
				return m, nil
			}
			// Ignore other keys when confirmation is showing
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit

		case tea.KeyCtrlL:
			// Clear screen
			m.messages = []string{welcomeMessage()}
			m.output.SetContent(strings.Join(m.messages, "\n\n"))
			return m, nil

		case tea.KeyCtrlH:
			// Show help
			m.addMessage(renderHelp())
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
			m.output.ViewUp()
			return m, nil

		case tea.KeyPgDown:
			// Scroll output down
			m.output.ViewDown()
			return m, nil

		case tea.KeyEsc:
			// Close autocomplete suggestions
			if m.showAutocomplete {
				m.showAutocomplete = false
				m.autocompleteIndex = 0
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

			// Clear input
			m.input.SetValue("")
			m.showAutocomplete = false
			m.autocompleteIndex = 0

			// Add user message to output
			m.addMessage(lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true).
				Render("  you") + "\n  " +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("252")).
					Render(value))

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

			if m.streamingEnabled {
				// In streaming mode, add a placeholder message for live updates
				m.streamBuffer = ""
				m.addMessage(formatAIResponse(""))
			}

			// Start both the AI request and thinking animation
			return m, tea.Batch(
				m.sendToAI(value),
				tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
					return thinkingMsg{}
				}),
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
		return m, nil

	case responseMsg:
		m.thinking = false
		m.status = "Ready"
		m.thinkingActivity = ""
		m.thinkingLog = nil
		// Refresh model info to show actual model name from API response
		m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
		if msg.err != nil {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
			dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
			var sb strings.Builder
			sb.WriteString(errStyle.Render("  error") + "\n")
			sb.WriteString("  " + errStyle.Render(msg.err.Error()) + "\n\n")
			sb.WriteString(dim.Render("  Check: echo $OPENAI_API_KEY") + "\n")
			sb.WriteString(dim.Render("  Check: cat ~/.soulgate/config.yml") + "\n")
			if m.streamingEnabled && len(m.messages) > 0 {
				// Replace the stream placeholder
				m.messages[len(m.messages)-1] = sb.String()
				m.output.SetContent(strings.Join(m.messages, "\n\n"))
			} else {
				m.addMessage(sb.String())
			}
		} else {
			if m.streamingEnabled && len(m.messages) > 0 {
				// Replace the stream placeholder with the final formatted response
				m.messages[len(m.messages)-1] = formatAIResponse(msg.text)
				m.output.SetContent(strings.Join(m.messages, "\n\n"))
				m.output.GotoBottom()
			} else if m.streamBuffer != "" && len(m.messages) > 0 {
				// Non-streaming with thinking output: finalize the thinking panel, then add response
				m.messages[len(m.messages)-1] = formatThinkingPanel(m.streamBuffer)
				m.addMessage(formatAIResponse(msg.text))
			} else {
				m.addMessage(formatAIResponse(msg.text))
			}
		}
		m.streamBuffer = ""
		return m, nil

	case thinkingMsg:
		if m.thinking {
			// Advance spinner frame
			m.spinnerFrame = (m.spinnerFrame + 1) % 10
			// Update thinking animation
			return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
				return thinkingMsg{}
			})
		}
		return m, nil

	case PermissionRequestMsg:
		// Show permission prompt
		m.showPermissionPrompt = true
		m.permissionRequest = &msg.Request
		m.permissionResponse = msg.Response
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
		case core.ThinkingModelCall:
			m.thinkingActivity = "calling model..."
		case core.ThinkingModelDone:
			modelName := evt.Model
			if modelName == "" {
				modelName = evt.Provider
			}
			m.thinkingActivity = fmt.Sprintf("model: %s (%s, %d tok, %s)",
				modelName, evt.StopReason, evt.TokensUsed, evt.Duration.Round(time.Millisecond))
		case core.ThinkingToolStart:
			m.thinkingActivity = fmt.Sprintf("running %s", evt.ToolName)

			// Show tool call in the chat output for live view
			toolLine := formatThinkingToolCall(evt.ToolName, evt.ToolArgs)
			if m.streamingEnabled && len(m.messages) > 0 {
				m.streamBuffer += toolLine
				m.messages[len(m.messages)-1] = formatAIResponse(m.streamBuffer)
			} else {
				// Non-streaming: update the last thinking placeholder
				m.ensureThinkingPlaceholder()
				m.streamBuffer += toolLine
				m.messages[len(m.messages)-1] = formatThinkingPanel(m.streamBuffer)
			}
			m.output.SetContent(strings.Join(m.messages, "\n\n"))
			m.output.GotoBottom()
		case core.ThinkingToolDone:
			m.thinkingActivity = fmt.Sprintf("%s done (%s)", evt.ToolName, evt.Duration.Round(time.Millisecond))

			// Show abbreviated result in live view
			resultLine := formatThinkingToolResult(evt.ToolName, evt.ToolResult, evt.Duration)
			if m.streamingEnabled && len(m.messages) > 0 {
				m.streamBuffer += resultLine
				m.messages[len(m.messages)-1] = formatAIResponse(m.streamBuffer)
			} else {
				m.ensureThinkingPlaceholder()
				m.streamBuffer += resultLine
				m.messages[len(m.messages)-1] = formatThinkingPanel(m.streamBuffer)
			}
			m.output.SetContent(strings.Join(m.messages, "\n\n"))
			m.output.GotoBottom()
		}
		return m, nil

	case streamChunkMsg:
		// Streaming token arrived - append to buffer and update display
		m.streamBuffer += msg.chunk
		// Update the last message in the output with accumulated stream content
		if len(m.messages) > 0 {
			m.messages[len(m.messages)-1] = formatAIResponse(m.streamBuffer)
			m.output.SetContent(strings.Join(m.messages, "\n\n"))
			m.output.GotoBottom()
		}
		return m, nil

	case dependencyInstallCompleteMsg:
		// Dependency installation completed
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
