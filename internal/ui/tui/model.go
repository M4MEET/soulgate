package tui

import (
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	"github.com/M4MEET/soulgate/internal/ui/tui/components"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	confirmation components.ConfirmationDialog
	// Permission prompt
	permission components.PermissionPrompt
	// Visual enhancements
	spinnerFrame int
	lastUpdate   time.Time
	// Model selection mode
	showModelSelector  bool
	modelSelectionStep int    // 1 = provider selection, 2 = model selection
	selectedProvider   string // Provider selected in step 1
	modelOptions       []modelOption
	selectedModelIndex int // For arrow key navigation
	// Setup wizard mode
	showSetupWizard    bool
	setupStep          int
	setupIntegrationID string
	setupFieldValues   map[string]string
	setupCurrentField  int
	// Onboarding mode (exported to allow auto-trigger from parent)
	ShowOnboarding      bool
	OnboardingState     *onboarding.OnboardingState
	onboardingInput     string
	onboardingSelection int
	// API key prompt
	showAPIKeyPrompt bool
	apiKeyProvider   string
	apiKeyInput      textinput.Model
	// Streaming mode
	streamingEnabled     bool
	streamBuffer         string        // Accumulates streamed assistant chunks
	thinkingBuffer       string        // Accumulates live thinking events
	teaProgram           **tea.Program // Double pointer: shared across copies
	lastRenderedContent  string
	streamFlushScheduled bool
	thinkingPanelIndex   int
	streamPanelIndex     int
	autoScroll           bool
	// Live thinking output
	thinkingLog      []core.ThinkingEvent // Recent thinking events
	thinkingActivity string               // Current activity for status bar
	// Token tracking
	sessionTokensUsed int // Cumulative tokens used in this session
	// Waiting phrase cycling (for status bar animation)
	waitingPhraseIndex int // Current index in waitingPhrases
	waitingPhraseTick  int // Tick counter for phrase rotation (changes every ~28 ticks @ 70ms = ~2s)
	// Abort / cancellation: pointer to a func so the goroutine running sendToAI
	// can update the cancel func while the TUI model copies are in flight.
	cancelHandle *func() // Points to the active context cancel func; nil func = no run
	// Display toggles
	showThinkingOutput bool // Whether to show the live thinking panel (default on)
	showVerboseTools   bool // Whether to show verbose tool input/output
	// Help overlay
	showHelpOverlay bool
	// Agent live view
	watchingAgentID        string // Agent ID being live-watched (empty = not watching)
	onboardingSpinnerFrame int
	// Gateway connectivity
	gatewayClient    **GatewayClient // Double pointer for Bubble Tea copy semantics
	gatewayURL       string          // Gateway WebSocket URL (empty = no gateway)
	gatewayConnected bool            // Whether currently connected to Gateway
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
	thinkingMsg           struct{}
	streamFlushMsg        struct{}
	dependencyProgressMsg struct{}
	// streamChunkMsg carries a streamed token from the AI
	streamChunkMsg struct {
		chunk string
	}
	// thinkingEventMsg carries a live thinking event from the agentic loop
	thinkingEventMsg struct {
		event core.ThinkingEvent
	}
	// agentPollMsg triggers a refresh of the watched agent's activity log
	agentPollMsg struct {
		agentID string
	}
	// PermissionRequestMsg is exported so it can be sent from parent package
	PermissionRequestMsg struct {
		Request  core.PermissionRequest
		Response chan core.PermissionResponse
	}
	// Keep lowercase alias for internal use
	permissionRequestMsg = PermissionRequestMsg
	// sessionAutoSaveMsg triggers a periodic session state persist
	sessionAutoSaveMsg struct{}
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

	progPtr := new(*tea.Program)
	noop := func() {}
	cancelPtr := new(func())
	*cancelPtr = noop
	return InteractiveChatModel{
		orch:               orch,
		input:              ti,
		output:             vp,
		messages:           []string{},
		history:            []string{},
		historyIndex:       -1,
		currentProvider:    provider,
		currentModel:       modelName,
		modelDiscovery:     discovery,
		teaProgram:         progPtr,
		cancelHandle:       cancelPtr,
		thinkingPanelIndex: -1,
		streamPanelIndex:   -1,
		autoScroll:         true,
		showThinkingOutput: true, // On by default; toggled with Ctrl+T
	}
}

// ShowWelcome displays the welcome message
func (m *InteractiveChatModel) ShowWelcome() {
	welcome := welcomeMessage()
	m.messages = []string{welcome}
	m.refreshOutput(true)
}

// RestoreSession loads persisted state (messages, history, agents, settings).
// Call this after NewInteractiveChatModel and before the Bubble Tea program starts.
func (m *InteractiveChatModel) RestoreSession() {
	configDir := m.orch.GetWorkspace().ConfigDir
	state, err := core.LoadSessionState(configDir)
	if err != nil || state == nil {
		return
	}

	// Restore messages
	if len(state.Messages) > 0 {
		m.messages = state.Messages
		m.refreshOutput(true)
	}

	// Restore command history
	if len(state.CommandHistory) > 0 {
		m.history = state.CommandHistory
	}

	// Restore agents
	if len(state.Agents) > 0 {
		m.orch.GetAgentManager().RestoreAgents(state.Agents)
	}

	// Restore settings
	if state.TrustMode {
		m.orch.SetTrustMode(true)
	}
	m.streamingEnabled = state.StreamingEnabled

	// Restore conversation history for AI context continuity
	if len(state.ConversationHistory) > 0 {
		m.orch.SetConversationHistory(state.ConversationHistory)
	}
}

// SaveSession persists the current session state to disk.
// Call this before the Bubble Tea program exits.
func (m *InteractiveChatModel) SaveSession() {
	configDir := m.orch.GetWorkspace().ConfigDir
	provider, modelName := m.orch.GetCurrentProvider()

	state := &core.SessionState{
		SessionID:           m.orch.GetSession().ID,
		Messages:            m.messages,
		CommandHistory:      m.history,
		Agents:              m.orch.GetAgentManager().Snapshot(),
		TrustMode:           m.orch.IsTrustMode(),
		StreamingEnabled:    m.streamingEnabled,
		CurrentProvider:     provider,
		CurrentModel:        modelName,
		ConversationHistory: m.orch.GetConversationHistory(),
	}

	// Best-effort save, don't fail the exit
	_ = core.SaveSessionState(configDir, state)
}

func welcomeMessage() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cmd := lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(dim.Render("  Type a message to chat with the AI.") + "\n")
	sb.WriteString(dim.Render("  Use ") + cmd.Render("/help") + dim.Render(" for commands, ") + cmd.Render("/context") + dim.Render(" for full usage, ") + cmd.Render("ctrl+h") + dim.Render(" for shortcuts.") + "\n")
	return sb.String()
}

// SetGatewayURL sets the Gateway URL for display purposes.
func (m *InteractiveChatModel) SetGatewayURL(url string) {
	m.gatewayURL = url
}

// SetGatewayClient sets the Gateway client (double pointer for Bubble Tea copies).
func (m *InteractiveChatModel) SetGatewayClient(client *GatewayClient) {
	if m.gatewayClient == nil {
		m.gatewayClient = new(*GatewayClient)
	}
	*m.gatewayClient = client
}

func (m *InteractiveChatModel) getGatewayClient() *GatewayClient {
	if m.gatewayClient == nil {
		return nil
	}
	return *m.gatewayClient
}

// GetMessages returns the current messages (used for persistence checks).
func (m *InteractiveChatModel) GetMessages() []string {
	return m.messages
}

// SetProgram sets the tea.Program reference for sending messages from goroutines.
// Uses a double pointer so the value persists across Bubble Tea model copies.
func (m *InteractiveChatModel) SetProgram(p *tea.Program) {
	if m.teaProgram == nil {
		m.teaProgram = new(*tea.Program)
	}
	*m.teaProgram = p
}

// Init initializes the Bubble Tea model
func (m InteractiveChatModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, sessionAutoSaveCmd())
}
