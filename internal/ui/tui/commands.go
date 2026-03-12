package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/ui/onboarding"
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
		m.showConfirmation = true
		m.confirmationMessage = getSensitiveMessage(cmd)
		m.pendingCommand = cmd
		m.pendingAction = func() tea.Cmd {
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
		m.output.SetContent(strings.Join(m.messages, "\n\n"))
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
		m.addMessage(renderToolsList())
		return nil

	case "/help":
		m.addMessage(renderHelp())
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

	case "/debug":
		m.addMessage(m.renderDebugInfo())
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
		m.ShowOnboarding = true
		m.OnboardingState = onboarding.NewOnboardingState(m.orch.GetWorkspace())
		m.onboardingInput = ""
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
		m.showConfirmation = true
		m.confirmationMessage = getSensitiveMessage(cmd)
		m.pendingCommand = cmd
		m.pendingAction = func() tea.Cmd {
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

// sendToAI sends a prompt to the AI and returns the response
func (m *InteractiveChatModel) sendToAI(prompt string) tea.Cmd {
	return func() tea.Msg {
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := m.orch.Run(ctx, prompt)
		if err != nil {
			// Return detailed error
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
