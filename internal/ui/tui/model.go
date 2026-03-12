package tui

import (
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// InteractiveChatModel is the Bubble Tea model for interactive chat
type InteractiveChatModel struct {
	orch              *core.Orchestrator
	input             textinput.Model
	output            viewport.Model
	messages          []string
	history           []string
	historyIndex      int
	autocomplete      []string
	showAutocomplete  bool
	autocompleteIndex int // Selected suggestion index
	width             int
	height            int
	status            string
	thinking          bool
	err               error
	// Model info
	currentProvider string
	currentModel    string
	modelDiscovery  *model.ModelDiscovery
	// Confirmation dialog
	showConfirmation    bool
	confirmationMessage string
	pendingCommand      string
	pendingAction       func() tea.Cmd
	// Permission prompt
	showPermissionPrompt bool
	permissionRequest    *core.PermissionRequest
	permissionResponse   chan core.PermissionResponse
	// Visual enhancements
	spinnerFrame int
	lastUpdate   time.Time
	// Model selection mode
	showModelSelector    bool
	modelSelectionStep   int    // 1 = provider selection, 2 = model selection
	selectedProvider     string // Provider selected in step 1
	modelOptions         []modelOption
	selectedModelIndex   int // For arrow key navigation
	// Setup wizard mode
	showSetupWizard     bool
	setupStep           int
	setupIntegrationID  string
	setupFieldValues    map[string]string
	setupCurrentField   int
	// Onboarding mode (exported to allow auto-trigger from parent)
	ShowOnboarding   bool
	OnboardingState  *onboarding.OnboardingState
	onboardingInput  string
	// API key prompt
	showAPIKeyPrompt bool
	apiKeyProvider   string
	apiKeyInput      textinput.Model
}

// modelOption represents a selectable model option
type modelOption struct {
	number      int
	name        string
	provider    string
	model       string
	description string
}

// Message types
type (
	responseMsg struct {
		text string
		err  error
	}
	thinkingMsg struct{}
	// PermissionRequestMsg is exported so it can be sent from parent package
	PermissionRequestMsg struct {
		Request  core.PermissionRequest
		Response chan core.PermissionResponse
	}
	// Keep lowercase alias for internal use
	permissionRequestMsg = PermissionRequestMsg
)

// providerInfo holds provider information for selection
type providerInfo struct {
	name        string
	displayName string
	description string
}

// NewInteractiveChatModel creates a new interactive chat model
func NewInteractiveChatModel(orch *core.Orchestrator) InteractiveChatModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 1000
	ti.Width = 100
	ti.Prompt = "  > "
	ti.PromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("246"))
	ti.TextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	ti.PlaceholderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	vp := viewport.New(100, 25)
	vp.SetContent("")

	provider, modelName := orch.GetCurrentProvider()
	discovery := model.NewModelDiscovery()

	return InteractiveChatModel{
		orch:            orch,
		input:           ti,
		output:          vp,
		messages:        []string{},
		history:         []string{},
		historyIndex:    -1,
		currentProvider: provider,
		currentModel:    modelName,
		modelDiscovery:  discovery,
	}
}

// ShowWelcome displays the welcome message
func (m *InteractiveChatModel) ShowWelcome() {
	welcome := welcomeMessage()
	m.messages = []string{welcome}
	m.output.SetContent(welcome)
}

func welcomeMessage() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(dim.Render("  Type a message to chat with the AI.") + "\n")
	sb.WriteString(dim.Render("  Use ") + cmd.Render("/help") + dim.Render(" for commands, ") + cmd.Render("ctrl+h") + dim.Render(" for shortcuts.") + "\n")
	return sb.String()
}

// Init initializes the Bubble Tea model
func (m InteractiveChatModel) Init() tea.Cmd {
	return textinput.Blink
}
