package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	"github.com/M4MEET/soulgate/internal/ui/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Onboarding-specific styles
// ---------------------------------------------------------------------------

var (
	// onbCyan is the primary accent for onboarding — cyan #00BFFF (term 81).
	onbCyan = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	// onbCyanDim is muted cyan for borders and secondary accent.
	onbCyanDim = lipgloss.NewStyle().Foreground(lipgloss.Color("38"))
	// onbTitle is bold white for the current step heading.
	onbTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	// onbDim is gray for hints and secondary descriptions.
	onbDim = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	// onbKey is cyan-tinted for keyboard shortcut labels.
	onbKey = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	// onbOk is green for checkmarks and completed items.
	onbOk = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	// onbWarn is yellow for security notices and warnings.
	onbWarn = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	// onbBad is red for errors.
	onbBad = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	// onbSelected is bold cyan for the currently highlighted list item.
	onbSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	// onbBody is off-white for regular prose.
	onbBody = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	// onbBox draws the ASCII art frame border.
	onbBox = lipgloss.NewStyle().Foreground(lipgloss.Color("38"))
	// onbBar draws section sub-headers inside provider lists.
	onbBar = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

// ---------------------------------------------------------------------------
// StartOnboardingWizard / helpers
// ---------------------------------------------------------------------------

// StartOnboardingWizard initializes the onboarding flow with sensible defaults.
func (m *InteractiveChatModel) StartOnboardingWizard() {
	m.ShowOnboarding = true
	m.OnboardingState = onboarding.NewOnboardingState(m.orch.GetWorkspace())
	m.onboardingInput = ""
	m.onboardingSpinnerFrame = 0
	m.onboardingSelection = 0
	if m.OnboardingState == nil {
		return
	}
	if idx, ok := m.OnboardingState.ApplyRecommendedModel(); ok && idx >= 0 {
		m.onboardingSelection = idx
	}
}

func (m *InteractiveChatModel) applyOnboardingQuickStart() {
	if m.OnboardingState == nil {
		return
	}
	m.OnboardingState.QuickMode = true
	if idx, ok := m.OnboardingState.ApplyRecommendedModel(); ok && idx >= 0 {
		m.onboardingSelection = idx
	}
	m.onboardingInput = ""
	m.OnboardingState.APIKeyError = ""
	_ = m.OnboardingState.SetStepByName("api_keys")
}

func moveOnboardingSelection(current, delta, total int) int {
	if total <= 0 {
		return 0
	}
	next := current + delta
	for next < 0 {
		next += total
	}
	for next >= total {
		next -= total
	}
	return next
}

func (m *InteractiveChatModel) toggleIntegrationSelection(id string) {
	if m.OnboardingState == nil || strings.TrimSpace(id) == "" {
		return
	}
	for i, existing := range m.OnboardingState.IntegrationsToSetup {
		if existing == id {
			m.OnboardingState.IntegrationsToSetup = append(
				m.OnboardingState.IntegrationsToSetup[:i],
				m.OnboardingState.IntegrationsToSetup[i+1:]...,
			)
			return
		}
	}
	m.OnboardingState.IntegrationsToSetup = append(m.OnboardingState.IntegrationsToSetup, id)
}

func (m *InteractiveChatModel) togglePopularIntegrations() {
	if m.OnboardingState == nil {
		return
	}
	recommendations := onboarding.GetIntegrationRecommendations()
	popular := make([]string, 0, len(recommendations))
	selected := make(map[string]bool, len(m.OnboardingState.IntegrationsToSetup))
	for _, id := range m.OnboardingState.IntegrationsToSetup {
		selected[id] = true
	}
	for _, rec := range recommendations {
		if rec.Popular {
			popular = append(popular, rec.ID)
		}
	}
	if len(popular) == 0 {
		return
	}

	allPopularSelected := true
	for _, id := range popular {
		if !selected[id] {
			allPopularSelected = false
			break
		}
	}

	if allPopularSelected {
		for _, id := range popular {
			m.toggleIntegrationSelection(id)
		}
		return
	}

	for _, id := range popular {
		if !selected[id] {
			m.OnboardingState.IntegrationsToSetup = append(m.OnboardingState.IntegrationsToSetup, id)
		}
	}
}

func (m *InteractiveChatModel) proceedFromIntegrationsStep() tea.Cmd {
	if m.OnboardingState == nil {
		return nil
	}
	if len(m.OnboardingState.IntegrationsToSetup) > 0 {
		m.OnboardingState.SetStepByName("dependencies")
		m.OnboardingState.InstallingDependencies = true
		m.onboardingSpinnerFrame = 0
		return tea.Batch(m.installDependenciesCmd(), dependencyTickCmd())
	}

	if m.OnboardingState.QuickMode {
		m.OnboardingState.SetStepByName("complete")
	} else {
		m.OnboardingState.SetStepByName("tutorial")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Input handling
// ---------------------------------------------------------------------------

// handleOnboardingInput handles input during onboarding.
func (m *InteractiveChatModel) handleOnboardingInput(msg tea.KeyMsg) (InteractiveChatModel, tea.Cmd) {
	if m.OnboardingState == nil {
		m.ShowOnboarding = false
		return *m, nil
	}

	key := msg.String()
	currentStep := m.OnboardingState.GetCurrentStep()

	// Global controls.
	if key == "q" || key == "ctrl+c" {
		_ = m.OnboardingState.MarkComplete()
		m.ShowOnboarding = false
		m.ShowWelcome()
		m.addMessage(onbWarn.Render("  Onboarding skipped. Run /onboarding anytime."))
		return *m, nil
	}

	if currentStep.Name != "api_keys" && (key == "b" || key == "left" || key == "esc") {
		m.OnboardingState.PreviousStep()
		m.OnboardingState.APIKeyError = ""
		return *m, nil
	}

	switch currentStep.Name {
	case "welcome":
		if key == "enter" || key == " " {
			m.OnboardingState.NextStep()
			return *m, nil
		}
		if key == "f" || key == "F" {
			m.applyOnboardingQuickStart()
			return *m, nil
		}

	case "security":
		if key == "a" || key == "A" || key == "enter" {
			m.OnboardingState.RiskAccepted = true
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "flow_select":
		if key == "1" || key == "q" || key == "Q" || key == "enter" {
			m.OnboardingState.Flow = "quickstart"
			m.OnboardingState.QuickMode = true
			m.OnboardingState.NextStep()
			return *m, nil
		}
		if key == "2" || key == "a" || key == "A" {
			m.OnboardingState.Flow = "advanced"
			m.OnboardingState.QuickMode = false
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "existing_config":
		if key == "1" || key == "u" || key == "enter" {
			m.OnboardingState.ConfigAction = "use"
			m.OnboardingState.NextStep()
			return *m, nil
		}
		if key == "2" {
			m.OnboardingState.ConfigAction = "update"
			m.OnboardingState.NextStep()
			return *m, nil
		}
		if key == "3" || key == "r" {
			m.OnboardingState.ConfigAction = "reset"
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "channels":
		channels := onboarding.GetAvailableChannels()
		if key == "up" || key == "k" {
			m.onboardingSelection = moveOnboardingSelection(m.onboardingSelection, -1, len(channels))
			return *m, nil
		}
		if key == "down" || key == "j" {
			m.onboardingSelection = moveOnboardingSelection(m.onboardingSelection, 1, len(channels))
			return *m, nil
		}
		if key == " " {
			if m.onboardingSelection >= 0 && m.onboardingSelection < len(channels) {
				m.OnboardingState.ToggleChannel(channels[m.onboardingSelection].ID)
			}
			return *m, nil
		}
		if key == "enter" {
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "gateway":
		if key == "enter" {
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "summary":
		if key == "enter" {
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "model_selection":
		options := m.OnboardingState.ModelOptions()
		if len(options) == 0 {
			m.OnboardingState.NextStep()
			return *m, nil
		}
		if m.onboardingSelection < 0 || m.onboardingSelection >= len(options) {
			if idx, ok := m.OnboardingState.ApplyRecommendedModel(); ok && idx >= 0 {
				m.onboardingSelection = idx
			} else {
				m.onboardingSelection = 0
			}
		}

		if key == "up" || key == "k" || key == "shift+tab" {
			m.onboardingSelection = moveOnboardingSelection(m.onboardingSelection, -1, len(options))
			return *m, nil
		}
		if key == "down" || key == "j" || key == "tab" {
			m.onboardingSelection = moveOnboardingSelection(m.onboardingSelection, 1, len(options))
			return *m, nil
		}
		if key == "r" || key == "R" {
			if idx, ok := m.OnboardingState.ApplyRecommendedModel(); ok && idx >= 0 {
				m.onboardingSelection = idx
			}
			return *m, nil
		}
		if key == "enter" || key == " " || key == "f" || key == "F" {
			if key == "f" || key == "F" {
				m.OnboardingState.QuickMode = true
			}
			selected := options[m.onboardingSelection]
			m.OnboardingState.SelectedModel = selected.ID
			m.OnboardingState.SelectedProvider = selected.Provider
			m.onboardingInput = ""
			m.OnboardingState.APIKeyError = ""
			m.OnboardingState.NextStep()
			return *m, nil
		}

		if key >= "1" && key <= "9" {
			idx := int(key[0] - '1')
			if idx < len(options) {
				m.OnboardingState.SelectedModel = options[idx].ID
				m.OnboardingState.SelectedProvider = options[idx].Provider
				m.onboardingSelection = idx
				m.onboardingInput = ""
				m.OnboardingState.APIKeyError = ""
				m.OnboardingState.NextStep()
				return *m, nil
			}
		}

	case "api_keys":
		if key == "esc" {
			m.OnboardingState.PreviousStep()
			m.OnboardingState.APIKeyError = ""
			m.onboardingInput = ""
			return *m, nil
		}

		provider := strings.ToLower(strings.TrimSpace(m.OnboardingState.SelectedProvider))
		if !providerNeedsKey(provider) {
			m.OnboardingState.APIKeyError = ""
			m.OnboardingState.NextStep()
			return *m, nil
		}

		envVar := onboardingEnvVar(provider)
		if key == "enter" {
			rawKey := strings.TrimSpace(m.onboardingInput)
			if rawKey != "" {
				if err := config.ValidateAPIKey(provider, rawKey); err != nil {
					m.OnboardingState.APIKeyError = err.Error()
					return *m, nil
				}
				m.OnboardingState.SetAPIKey(provider, rawKey)
				m.onboardingInput = ""
				m.OnboardingState.APIKeyError = ""
				m.OnboardingState.NextStep()
				return *m, nil
			}

			if envVar != "" && strings.TrimSpace(os.Getenv(envVar)) != "" {
				m.OnboardingState.APIKeyError = ""
				m.OnboardingState.NextStep()
				return *m, nil
			}

			m.OnboardingState.APIKeyError = fmt.Sprintf("Enter %s or press s to skip.", envVar)
			return *m, nil
		} else if key == "e" || key == "E" {
			if envVar != "" && strings.TrimSpace(os.Getenv(envVar)) != "" {
				m.onboardingInput = ""
				m.OnboardingState.APIKeyError = ""
				m.OnboardingState.NextStep()
				return *m, nil
			}
			m.OnboardingState.APIKeyError = fmt.Sprintf("%s is not set in your environment.", envVar)
			return *m, nil
		} else if key == "s" || key == "S" {
			m.onboardingInput = ""
			m.OnboardingState.APIKeyError = ""
			m.OnboardingState.NextStep()
			return *m, nil
		} else if key == "ctrl+u" {
			m.onboardingInput = ""
			m.OnboardingState.APIKeyError = ""
			return *m, nil
		} else if key == "backspace" || key == "delete" {
			runes := []rune(m.onboardingInput)
			if len(runes) > 0 {
				m.onboardingInput = string(runes[:len(runes)-1])
				m.OnboardingState.APIKeyError = ""
			}
			return *m, nil
		} else if msg.Type == tea.KeyRunes {
			m.onboardingInput += string(msg.Runes)
			m.OnboardingState.APIKeyError = ""
			return *m, nil
		}

	case "test_connection":
		if key == "enter" || key == " " {
			if m.OnboardingState.QuickMode {
				_ = m.OnboardingState.SetStepByName("complete")
			} else {
				m.OnboardingState.NextStep()
			}
			return *m, nil
		}

	case "integrations":
		recommendations := onboarding.GetIntegrationRecommendations()
		if m.onboardingSelection < 0 || m.onboardingSelection >= len(recommendations) {
			m.onboardingSelection = 0
		}

		if key == "up" || key == "k" || key == "shift+tab" {
			m.onboardingSelection = moveOnboardingSelection(m.onboardingSelection, -1, len(recommendations))
			return *m, nil
		}
		if key == "down" || key == "j" || key == "tab" {
			m.onboardingSelection = moveOnboardingSelection(m.onboardingSelection, 1, len(recommendations))
			return *m, nil
		}
		if key == " " {
			if len(recommendations) > 0 {
				m.toggleIntegrationSelection(recommendations[m.onboardingSelection].ID)
			}
			return *m, nil
		}
		if key == "a" || key == "A" {
			m.togglePopularIntegrations()
			return *m, nil
		}
		if key == "s" || key == "S" {
			if m.OnboardingState.QuickMode {
				_ = m.OnboardingState.SetStepByName("complete")
				return *m, nil
			}
			_ = m.OnboardingState.SetStepByName("tutorial")
			return *m, nil
		}
		if key == "enter" {
			return *m, m.proceedFromIntegrationsStep()
		}
		if key >= "1" && key <= "9" {
			idx := int(key[0] - '1')
			if idx < len(recommendations) {
				m.onboardingSelection = idx
				m.toggleIntegrationSelection(recommendations[idx].ID)
				return *m, nil
			}
		}

	case "dependencies":
		if m.OnboardingState.InstallingDependencies {
			return *m, nil
		}
		if key == "enter" || key == " " {
			m.OnboardingState.NextStep()
		}

	case "tutorial":
		if key == "enter" || key == " " || key == "s" || key == "S" {
			m.OnboardingState.NextStep()
			return *m, nil
		}

	case "complete":
		if key == "enter" || key == " " {
			if err := m.OnboardingState.SaveAPIKeys(); err != nil {
				m.addMessage(onbBad.Render("  Failed to save onboarding config: " + err.Error()))
				return *m, nil
			}
			if selectedModelID := m.OnboardingState.SelectedModelID(); selectedModelID != "" {
				_ = m.orch.SetProvider(m.OnboardingState.SelectedProvider, selectedModelID)
				m.currentProvider, m.currentModel = m.orch.GetCurrentProvider()
			}
			_ = m.OnboardingState.MarkComplete()
			m.ShowOnboarding = false
			m.ShowWelcome()
			m.addMessage(onbOk.Render("  Setup complete. Ask a task, or try /help and /tools."))
		}
	}

	return *m, nil
}

// ---------------------------------------------------------------------------
// Top-level renderer
// ---------------------------------------------------------------------------

// renderOnboarding renders the full onboarding wizard overlay.
func (m *InteractiveChatModel) renderOnboarding() string {
	if m.OnboardingState == nil {
		return ""
	}

	var sb strings.Builder
	currentStep := m.OnboardingState.GetCurrentStep()
	steps := onboarding.GetOnboardingSteps()

	// Auto-skip steps that don't apply
	switch currentStep.Name {
	case "existing_config":
		if !m.OnboardingState.HasExistingConfig {
			m.OnboardingState.NextStep()
			return m.renderOnboarding()
		}
	case "gateway":
		if m.OnboardingState.Flow == "quickstart" || m.OnboardingState.QuickMode {
			m.OnboardingState.NextStep()
			return m.renderOnboarding()
		}
	}

	// Progress bar
	sb.WriteString(renderOnboardingProgressBar(m.OnboardingState.Step, len(steps), currentStep.Title))
	sb.WriteString("\n\n")

	// Step content
	switch currentStep.Name {
	case "welcome":
		sb.WriteString(m.renderWelcomeStep())
	case "security":
		sb.WriteString(m.renderSecurityStep())
	case "flow_select":
		sb.WriteString(m.renderFlowSelectStep())
	case "existing_config":
		sb.WriteString(m.renderExistingConfigStep())
	case "channels":
		sb.WriteString(m.renderChannelsStep())
	case "gateway":
		sb.WriteString(m.renderGatewayStep())
	case "summary":
		sb.WriteString(m.renderSummaryStep())
	case "model_selection":
		sb.WriteString(m.renderModelSelectionStep())
	case "api_keys":
		sb.WriteString(m.renderAPIKeysStep())
	case "test_connection":
		sb.WriteString(m.renderTestConnectionStep())
	case "integrations":
		sb.WriteString(m.renderIntegrationsStep())
	case "dependencies":
		sb.WriteString(m.renderDependenciesStep())
	case "tutorial":
		sb.WriteString(m.renderTutorialStep())
	case "complete":
		sb.WriteString(m.renderCompleteStep())
	}

	sb.WriteString("\n")
	sb.WriteString(renderOnboardingHints(currentStep.Name))

	return sb.String()
}

// renderOnboardingProgressBar renders the "Step N of M ━━━━░░░ Label" bar.
func renderOnboardingProgressBar(step, total int, label string) string {
	barWidth := 24

	filled := 0
	if total > 1 {
		filled = step * barWidth / (total - 1)
	}
	if filled > barWidth {
		filled = barWidth
	}
	remaining := barWidth - filled

	filledStr := strings.Repeat("━", filled)
	remainingStr := strings.Repeat("░", remaining)

	stepLabel := onbDim.Render(fmt.Sprintf("  Step %d of %d  ", step+1, total))
	bar := onbCyan.Render(filledStr) + onbDim.Render(remainingStr)
	stepName := "  " + onbDim.Render(label)

	return stepLabel + bar + stepName
}

// renderOnboardingHints renders the keyboard shortcut line at the bottom.
func renderOnboardingHints(stepName string) string {
	k := func(key string) string { return onbKey.Render(key) }
	d := func(s string) string { return onbDim.Render(s) }

	switch stepName {
	case "welcome":
		return "  " + d("shortcuts: ") + k("enter") + d(" next  ") + k("f") + d(" fast lane  ") + k("q") + d(" skip\n")
	case "model_selection":
		return "  " + d("shortcuts: ") + k("↑/↓") + d(" move  ") + k("enter") + d(" select  ") + k("r") + d(" recommended  ") + k("f") + d(" fast lane  ") + k("b") + d(" back  ") + k("q") + d(" skip\n")
	case "api_keys":
		return "  " + d("shortcuts: ") + k("enter") + d(" save  ") + k("e") + d(" use env  ") + k("ctrl+u") + d(" clear  ") + k("s") + d(" skip  ") + k("esc") + d(" back\n")
	case "integrations":
		return "  " + d("shortcuts: ") + k("↑/↓") + d(" move  ") + k("space") + d(" toggle  ") + k("a") + d(" popular  ") + k("enter") + d(" continue  ") + k("s") + d(" skip  ") + k("b") + d(" back\n")
	default:
		return "  " + d("shortcuts: ") + k("enter") + d(" continue  ") + k("b") + d(" back  ") + k("q") + d(" skip\n")
	}
}

// ---------------------------------------------------------------------------
// Step renderers
// ---------------------------------------------------------------------------

func (m *InteractiveChatModel) renderSecurityStep() string {
	var sb strings.Builder
	sb.WriteString("  " + onbWarn.Render("⚠  Security Notice") + "\n\n")
	sb.WriteString("  " + onbBody.Render("SoulGate has full system access:") + "\n")
	sb.WriteString("  " + onbDim.Render("  • Read and write any file") + "\n")
	sb.WriteString("  " + onbDim.Render("  • Execute shell commands") + "\n")
	sb.WriteString("  " + onbDim.Render("  • Make network requests") + "\n")
	sb.WriteString("  " + onbDim.Render("  • Control a browser") + "\n\n")
	sb.WriteString("  " + onbBody.Render("Only run on systems you trust.") + "\n\n")
	sb.WriteString("  " + onbKey.Render("[a]") + onbDim.Render(" acknowledge and continue   ") + onbKey.Render("[q]") + onbDim.Render(" quit") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderFlowSelectStep() string {
	var sb strings.Builder
	sb.WriteString("  " + onbCyan.Render("Setup Mode") + "\n\n")
	sb.WriteString("  " + onbKey.Render("[1]") + " " + onbSelected.Render("QuickStart") + "  " + onbDim.Render("Auto-configure with sensible defaults") + "\n")
	sb.WriteString("  " + onbKey.Render("[2]") + " " + onbBody.Render("Advanced") + "    " + onbDim.Render("Full control over every setting") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderExistingConfigStep() string {
	var sb strings.Builder
	sb.WriteString("  " + onbCyan.Render("Existing Configuration Found") + "\n\n")
	sb.WriteString("  " + onbDim.Render("A previous SoulGate configuration was detected.") + "\n\n")
	sb.WriteString("  " + onbKey.Render("[1]") + " " + onbSelected.Render("Use existing") + "  " + onbDim.Render("Keep current settings") + "\n")
	sb.WriteString("  " + onbKey.Render("[2]") + " " + onbBody.Render("Update") + "        " + onbDim.Render("Modify settings") + "\n")
	sb.WriteString("  " + onbKey.Render("[3]") + " " + onbBody.Render("Reset") + "         " + onbDim.Render("Start fresh") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderChannelsStep() string {
	var sb strings.Builder
	channels := onboarding.GetAvailableChannels()
	sb.WriteString("  " + onbCyan.Render("Connect Messaging Platforms") + "\n")
	sb.WriteString("  " + onbDim.Render("Space to toggle, Enter to confirm") + "\n\n")

	for i, ch := range channels {
		cursor := "  "
		if i == m.onboardingSelection {
			cursor = onbCyan.Render("▸ ")
		}
		checked := "[ ]"
		for _, sel := range m.OnboardingState.ChannelsToSetup {
			if sel == ch.ID {
				checked = onbOk.Render("[✓]")
				break
			}
		}
		name := onbBody.Render(padRight(ch.Name, 14))
		if i == m.onboardingSelection {
			name = onbSelected.Render(padRight(ch.Name, 14))
		}
		sb.WriteString("  " + cursor + checked + " " + name + onbDim.Render(ch.Description) + "\n")
	}
	return sb.String()
}

func (m *InteractiveChatModel) renderGatewayStep() string {
	var sb strings.Builder
	sb.WriteString("  " + onbCyan.Render("Gateway Configuration") + "\n\n")
	sb.WriteString("  " + onbDim.Render("Port:    ") + onbBody.Render(fmt.Sprintf("%d", m.OnboardingState.GatewayPort)) + "\n")
	sb.WriteString("  " + onbDim.Render("Bind:    ") + onbBody.Render(m.OnboardingState.GatewayBind) + "\n")
	sb.WriteString("  " + onbDim.Render("Web UI:  ") + onbBody.Render(fmt.Sprintf("http://localhost:%d", m.OnboardingState.GatewayPort)) + "\n\n")
	sb.WriteString("  " + onbDim.Render("Press Enter to continue with these defaults") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderSummaryStep() string {
	var sb strings.Builder
	sb.WriteString("  " + onbCyan.Render("Configuration Summary") + "\n\n")
	sb.WriteString("  " + onbDim.Render("Provider:  ") + onbBody.Render(m.OnboardingState.SelectedProvider) + "\n")
	sb.WriteString("  " + onbDim.Render("Model:     ") + onbBody.Render(m.OnboardingState.SelectedModel) + "\n")
	sb.WriteString("  " + onbDim.Render("Gateway:   ") + onbBody.Render(fmt.Sprintf("http://localhost:%d", m.OnboardingState.GatewayPort)) + "\n")
	if len(m.OnboardingState.ChannelsToSetup) > 0 {
		sb.WriteString("  " + onbDim.Render("Channels:  ") + onbBody.Render(strings.Join(m.OnboardingState.ChannelsToSetup, ", ")) + "\n")
	}
	sb.WriteString("\n  " + onbDim.Render("Press Enter to save and finish") + "\n")
	return sb.String()
}

func (m *InteractiveChatModel) renderWelcomeStep() string {
	var sb strings.Builder

	// ASCII art banner
	sb.WriteString(renderSoulGateBanner())
	sb.WriteString("\n")

	// Setup style choice
	sb.WriteString("  " + onbDim.Render("Choose your setup style:") + "\n\n")

	guided := onbCyan.Render("  enter") + onbDim.Render("  Guided setup  ") + onbDim.Render("pick model, configure keys, add integrations")
	fast := onbKey.Render("  f    ") + onbDim.Render("  Fast lane     ") + onbDim.Render("recommended defaults, skip straight to chat")
	sb.WriteString(guided + "\n")
	sb.WriteString(fast + "\n\n")

	// What guided setup covers
	sb.WriteString("  " + onbDim.Render("Guided setup covers:") + "\n\n")
	items := []string{
		"Choose your AI provider and model",
		"Configure API keys",
		"Connect integrations (optional)",
		"Quick-start tutorial",
	}
	for i, item := range items {
		num := onbDim.Render(fmt.Sprintf("  %d. ", i+1))
		sb.WriteString(num + onbBody.Render(item) + "\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderModelSelectionStep() string {
	var sb strings.Builder
	options := m.OnboardingState.ModelOptions()
	if len(options) == 0 {
		sb.WriteString("  " + onbBad.Render("No model options available.") + "\n")
		return sb.String()
	}
	if m.onboardingSelection < 0 || m.onboardingSelection >= len(options) {
		m.onboardingSelection = 0
	}

	// Group into Cloud / Local / Custom sections
	type group struct {
		header  string
		options []onboarding.ModelOption
		indices []int
	}

	cloudGroup := group{header: "Cloud AI"}
	localGroup := group{header: "Local"}
	customGroup := group{header: "Custom"}

	for idx, opt := range options {
		switch strings.ToLower(opt.Provider) {
		case "ollama":
			localGroup.options = append(localGroup.options, opt)
			localGroup.indices = append(localGroup.indices, idx)
		case "custom":
			customGroup.options = append(customGroup.options, opt)
			customGroup.indices = append(customGroup.indices, idx)
		default:
			cloudGroup.options = append(cloudGroup.options, opt)
			cloudGroup.indices = append(cloudGroup.indices, idx)
		}
	}

	groups := []group{cloudGroup, localGroup, customGroup}

	// Calculate column widths for alignment
	const nameColW = 14
	const descColW = 36

	boxTop := "  " + onbCyanDim.Render("┌") + onbCyanDim.Render(strings.Repeat("─", 50)) + onbCyanDim.Render("┐")
	boxBot := "  " + onbCyanDim.Render("└") + onbCyanDim.Render(strings.Repeat("─", 50)) + onbCyanDim.Render("┘")

	sb.WriteString(boxTop + "\n")

	for gi, grp := range groups {
		if len(grp.options) == 0 {
			continue
		}

		// Group separator header (not on the very first group)
		if gi == 0 {
			hdr := onbCyanDim.Render("│") + onbBar.Render(fmt.Sprintf(" %-13s", "── "+grp.header+" ")) + onbBar.Render(strings.Repeat("─", 36)) + onbCyanDim.Render("│")
			sb.WriteString("  " + hdr + "\n")
		} else {
			hdr := onbCyanDim.Render("├") + onbBar.Render(fmt.Sprintf(" %-13s", "── "+grp.header+" ")) + onbBar.Render(strings.Repeat("─", 36)) + onbCyanDim.Render("┤")
			sb.WriteString("  " + hdr + "\n")
		}

		for li, opt := range grp.options {
			globalIdx := grp.indices[li]
			isCurrent := globalIdx == m.onboardingSelection

			// Cursor arrow
			arrow := "  "
			if isCurrent {
				arrow = onbCyan.Render("▸ ")
			}

			// Name field (fixed width)
			name := opt.Name
			if isCurrent {
				name = onbSelected.Render(padRight(name, nameColW))
			} else {
				name = onbBody.Render(padRight(name, nameColW))
			}

			// Description field (fixed width, truncated)
			desc := opt.Description
			if len(desc) > descColW {
				desc = desc[:descColW-1]
			}
			descStr := onbDim.Render(padRight(desc, descColW))

			// Recommended badge
			badge := "  "
			if opt.Recommended {
				badge = onbOk.Render("* ")
			}

			_ = badge
			row := onbCyanDim.Render("│") + arrow + name + "  " + descStr + onbCyanDim.Render("│")
			sb.WriteString("  " + row + "\n")
		}
	}

	sb.WriteString(boxBot + "\n")
	sb.WriteString("\n")

	// Selected model info
	if m.onboardingSelection >= 0 && m.onboardingSelection < len(options) {
		sel := options[m.onboardingSelection]
		sb.WriteString("  " + onbDim.Render("Selected: ") + onbCyan.Render(sel.Name))
		if sel.Recommended {
			sb.WriteString("  " + onbOk.Render("recommended"))
		}
		sb.WriteString("\n")
		sb.WriteString("  " + onbDim.Render(sel.Description) + "\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderAPIKeysStep() string {
	var sb strings.Builder
	t := theme.T
	provider := strings.ToLower(strings.TrimSpace(m.OnboardingState.SelectedProvider))
	if provider == "" {
		sb.WriteString("  " + onbBad.Render("No provider selected.") + "\n")
		return sb.String()
	}

	if !providerNeedsKey(provider) {
		sb.WriteString("  " + onbOk.Render("No API key required for this provider.") + "\n\n")
		sb.WriteString("  " + onbDim.Render("press ") + onbKey.Render("enter") + onbDim.Render(" to continue") + "\n")
		return sb.String()
	}

	envVar := onboardingEnvVar(provider)
	envVal := ""
	if envVar != "" {
		envVal = strings.TrimSpace(os.Getenv(envVar))
	}
	hasSaved := m.OnboardingState.HasSavedAPIKey(provider)

	// Status row
	if envVal != "" {
		preview := envVal
		if len(preview) > 8 {
			preview = preview[:8] + "..."
		}
		sb.WriteString("  " + onbOk.Render("found  ") + onbDim.Render(envVar+" is set ("+preview+")") + "\n")
	} else if hasSaved {
		sb.WriteString("  " + onbOk.Render("saved  ") + onbDim.Render("key captured") + "\n")
	} else {
		sb.WriteString("  " + onbWarn.Render("missing") + onbDim.Render("  no key detected yet") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render(config.GetProviderAPIKeyInstructions(provider)) + "\n")
	if envVar != "" {
		sb.WriteString("  " + onbDim.Render("Env var: ") + t.Key.Render(envVar) + "\n")
	}
	sb.WriteString("\n")

	// Key input box
	masked := m.maskedOnboardingKey()
	inputDisplay := ""
	if masked == "" {
		inputDisplay = onbDim.Render("(type or paste your key here)")
	} else {
		inputDisplay = onbKey.Render(masked)
	}
	sb.WriteString("  " + onbDim.Render("Key  ") + inputDisplay + "\n")

	if m.OnboardingState.APIKeyError != "" {
		sb.WriteString("\n")
		sb.WriteString("  " + onbBad.Render("! "+m.OnboardingState.APIKeyError) + "\n")
	}

	sb.WriteString("\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderTestConnectionStep() string {
	var sb strings.Builder

	provider := strings.ToLower(strings.TrimSpace(m.OnboardingState.SelectedProvider))
	envVar := onboardingEnvVar(provider)
	envSet := envVar != "" && strings.TrimSpace(os.Getenv(envVar)) != ""
	hasSaved := m.OnboardingState.HasSavedAPIKey(provider)

	check := func(ok bool, label, detail string) string {
		if ok {
			return "  " + onbOk.Render("ok    ") + onbDim.Render(label+": ") + onbBody.Render(detail) + "\n"
		}
		return "  " + onbWarn.Render("warn  ") + onbDim.Render(label+": ") + onbBody.Render(detail) + "\n"
	}

	sb.WriteString(check(true, "Provider", provider))

	if providerNeedsKey(provider) {
		if envSet || hasSaved {
			sb.WriteString(check(true, "API Key", "available"))
		} else {
			sb.WriteString(check(false, "API Key", "not detected (can be added later)"))
		}
	} else {
		sb.WriteString(check(true, "API Key", "not required"))
	}

	sb.WriteString(check(true, "Config", "ready"))

	if m.OnboardingState.QuickMode {
		sb.WriteString("\n  " + onbWarn.Render("fast lane") + onbDim.Render("  optional screens will be skipped") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render("press ") + onbKey.Render("enter") + onbDim.Render(" to continue") + "\n")

	return sb.String()
}

func (m *InteractiveChatModel) renderIntegrationsStep() string {
	var sb strings.Builder
	recommendations := onboarding.GetIntegrationRecommendations()
	if len(recommendations) == 0 {
		sb.WriteString("  " + onbDim.Render("No integrations available.") + "\n")
		return sb.String()
	}
	if m.onboardingSelection < 0 || m.onboardingSelection >= len(recommendations) {
		m.onboardingSelection = 0
	}

	selected := make(map[string]bool)
	for _, id := range m.OnboardingState.IntegrationsToSetup {
		selected[id] = true
	}

	sb.WriteString("  " + onbDim.Render("Select integrations to connect") + onbDim.Render("  (space to toggle, enter to confirm):") + "\n\n")

	for i, rec := range recommendations {
		isCurrent := i == m.onboardingSelection

		// Checkbox
		var checkBox string
		if selected[rec.ID] {
			checkBox = onbOk.Render("[✓]")
		} else {
			checkBox = onbDim.Render("[ ]")
		}

		// Name
		nameStr := ""
		if isCurrent {
			nameStr = onbSelected.Render(padRight(rec.Name, 14))
		} else {
			nameStr = onbBody.Render(padRight(rec.Name, 14))
		}

		// Popular badge
		badge := ""
		if rec.Popular {
			badge = " " + onbOk.Render("popular")
		}

		// Cursor
		cursor := "  "
		if isCurrent {
			cursor = onbCyan.Render("▸ ")
		}

		sb.WriteString(cursor + checkBox + " " + nameStr + "  " + onbDim.Render(rec.Description) + badge + "\n")
	}

	sb.WriteString("\n")
	if len(m.OnboardingState.IntegrationsToSetup) > 0 {
		sb.WriteString("  " + onbDim.Render("Selected: ") + onbCyan.Render(strings.Join(m.OnboardingState.IntegrationsToSetup, ", ")) + "\n\n")
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderDependenciesStep() string {
	var sb strings.Builder

	if len(m.OnboardingState.IntegrationsToSetup) == 0 {
		sb.WriteString("  " + onbDim.Render("No integrations selected.") + "\n\n")
		sb.WriteString("  " + onbDim.Render("press ") + onbKey.Render("enter") + onbDim.Render(" to continue") + "\n")
		return sb.String()
	}

	sb.WriteString("  " + onbDim.Render("Installing dependencies for selected integrations...") + "\n\n")

	for _, id := range m.OnboardingState.IntegrationsToSetup {
		status, ok := m.OnboardingState.DependencyStatus[id]
		if !ok {
			status = "pending"
		}

		var statusStyled string
		switch {
		case strings.HasPrefix(status, "✓") || strings.HasPrefix(status, "ok"):
			statusStyled = onbOk.Render(status)
		case strings.HasPrefix(status, "✗") || strings.HasPrefix(status, "error"):
			statusStyled = onbBad.Render(status)
		default:
			statusStyled = onbDim.Render(status)
		}

		sb.WriteString("  " + onbBody.Render(padRight(id, 16)) + "  " + statusStyled + "\n")
	}

	if len(m.OnboardingState.DependencyErrors) > 0 {
		sb.WriteString("\n")
		for _, err := range m.OnboardingState.DependencyErrors {
			sb.WriteString("  " + onbBad.Render("! "+err) + "\n")
		}
	}

	sb.WriteString("\n")
	if m.OnboardingState.InstallingDependencies {
		spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		spin := onbCyan.Render(spinners[m.onboardingSpinnerFrame%len(spinners)])
		sb.WriteString("  " + spin + " " + onbDim.Render("installing...") + "\n")
	} else {
		sb.WriteString("  " + onbDim.Render("press ") + onbKey.Render("enter") + onbDim.Render(" to continue") + "\n")
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderTutorialStep() string {
	var sb strings.Builder
	steps := onboarding.GetTutorialSteps()

	sb.WriteString("  " + onbDim.Render("A few things to try:") + "\n\n")

	for _, step := range steps {
		title := onbBody.Render(step.Title)
		command := onbKey.Render(step.Command)
		desc := onbDim.Render(step.Desc)

		sb.WriteString("  " + title + "\n")
		sb.WriteString("    " + command + "  " + desc + "\n\n")
	}

	return sb.String()
}

func (m *InteractiveChatModel) renderCompleteStep() string {
	var sb strings.Builder

	// Success header
	sb.WriteString("  " + onbOk.Render("✓ SoulGate is ready!") + "\n\n")

	// Provider summary
	providerDisplay := m.OnboardingState.SelectedProvider
	modelDisplay := m.OnboardingState.SelectedModel
	if preset, ok := m.OnboardingState.SelectedPreset(); ok {
		providerDisplay = preset.Provider
		modelDisplay = preset.Model
	}
	if providerDisplay != "" {
		sb.WriteString("  " + onbDim.Render(padRight("Provider:", 12)) + onbBody.Render(providerDisplay+" ("+modelDisplay+")") + "\n")
	}

	// API key status
	keyStatus := "configured"
	if providerNeedsKey(m.OnboardingState.SelectedProvider) &&
		!m.OnboardingState.HasSavedAPIKey(m.OnboardingState.SelectedProvider) &&
		strings.TrimSpace(os.Getenv(onboardingEnvVar(m.OnboardingState.SelectedProvider))) == "" {
		keyStatus = "not set (add later with /setup)"
	}
	sb.WriteString("  " + onbDim.Render(padRight("API Key:", 12)) + onbBody.Render(keyStatus) + "\n")

	// Integrations
	if len(m.OnboardingState.IntegrationsToSetup) > 0 {
		sb.WriteString("  " + onbDim.Render(padRight("Integrations:", 12)) + onbBody.Render(strings.Join(m.OnboardingState.IntegrationsToSetup, ", ")) + "\n")
	}

	if m.OnboardingState.QuickMode {
		sb.WriteString("  " + onbDim.Render(padRight("Mode:", 12)) + onbWarn.Render("fast lane") + "\n")
	}

	// Next steps
	sb.WriteString("\n")
	sb.WriteString("  " + onbBody.Render("Next steps:") + "\n\n")

	nextSteps := []struct{ step, cmd string }{
		{"1. Start chatting", "Just type a message below"},
		{"2. View available tools", "/tools"},
		{"3. Configure further", "/setup or /model"},
	}
	for _, ns := range nextSteps {
		sb.WriteString("  " + onbDim.Render(padRight(ns.step, 24)) + onbKey.Render(ns.cmd) + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString("  " + onbDim.Render("press ") + onbKey.Render("enter") + onbDim.Render(" to start chatting") + "\n")

	return sb.String()
}

// ---------------------------------------------------------------------------
// ASCII art banner
// ---------------------------------------------------------------------------

// renderSoulGateBanner returns the framed ASCII art logo for the welcome step.
func renderSoulGateBanner() string {
	border := onbBox.Render
	cyan := onbCyan.Render
	dim := onbDim.Render

	// Inner width of the box (between the │ characters) is 51 chars.
	const innerW = 51

	runeWidth := func(s string) int {
		return len([]rune(s))
	}

	pad := func(s string, w int) string {
		rw := runeWidth(s)
		left := (w - rw) / 2
		if left < 0 {
			left = 0
		}
		right := w - rw - left
		if right < 0 {
			right = 0
		}
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
	}

	top := "  " + border("╔") + border(strings.Repeat("═", innerW)) + border("╗")
	bot := "  " + border("╚") + border(strings.Repeat("═", innerW)) + border("╝")
	blank := "  " + border("║") + strings.Repeat(" ", innerW) + border("║")

	logoLines := []string{
		`███████╗ ██████╗ ██╗   ██╗██╗      ██████╗ `,
		`██╔════╝██╔═══██╗██║   ██║██║     ██╔════╝ `,
		`███████╗██║   ██║██║   ██║██║     ██║  ███╗`,
		`╚════██║██║   ██║██║   ██║██║     ██║   ██║`,
		`███████║╚██████╔╝╚██████╔╝███████╗╚██████╔╝`,
		`╚══════╝ ╚═════╝  ╚═════╝ ╚══════╝ ╚═════╝`,
	}

	tagline := "Your AI, everywhere."

	var sb strings.Builder
	sb.WriteString(top + "\n")
	sb.WriteString(blank + "\n")

	for _, line := range logoLines {
		centered := pad(line, innerW)
		// Split: prefix spaces | logo text | suffix spaces
		// We apply cyan only to the non-space portion to avoid coloring padding.
		trimmed := strings.TrimSpace(centered)
		trimW := runeWidth(trimmed)
		lp := (innerW - trimW) / 2
		if lp < 0 {
			lp = 0
		}
		rp := innerW - lp - trimW
		if rp < 0 {
			rp = 0
		}
		leftPad := strings.Repeat(" ", lp)
		rightPad := strings.Repeat(" ", rp)
		rendered := "  " + border("║") + leftPad + cyan(trimmed) + rightPad + border("║")
		sb.WriteString(rendered + "\n")
	}

	sb.WriteString(blank + "\n")

	// Tagline row
	tagCentered := pad(tagline, innerW)
	tagTrimmed := strings.TrimSpace(tagCentered)
	tagLeft := strings.Repeat(" ", (innerW-len(tagTrimmed))/2)
	tagRight := strings.Repeat(" ", innerW-len(tagLeft)-len(tagTrimmed))
	tagRow := "  " + border("║") + tagLeft + dim(tagTrimmed) + tagRight + border("║")
	sb.WriteString(tagRow + "\n")

	sb.WriteString(bot + "\n")

	return sb.String()
}

// ---------------------------------------------------------------------------
// Utility helpers
// ---------------------------------------------------------------------------

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
