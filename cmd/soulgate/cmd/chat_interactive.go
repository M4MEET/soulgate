package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/ui/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// RunInteractiveTUI starts the interactive TUI.
// If forceOnboarding is true, onboarding is shown even when previously completed.
// If gatewayURL is non-empty, connects to the Gateway as an agent to receive channel messages.
// fresh=true discards previous session; cont=true restores it; both false prompts the user.
func RunInteractiveTUI(orch *core.Orchestrator, forceOnboarding bool, gatewayURL string, fresh, cont bool) error {
	workspace := orch.GetWorkspace()

	// Check if onboarding is needed
	needsOnboarding := false
	markerPath := filepath.Join(workspace.ConfigDir, ".onboarding_complete")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		needsOnboarding = true
	}

	// Determine session mode
	restoreSession := resolveSessionMode(workspace.ConfigDir, fresh, cont)

	// Create the TUI model
	m := tui.NewInteractiveChatModel(orch)

	if restoreSession {
		m.RestoreSession()
	} else {
		// Clear saved state so next launch doesn't see stale data
		core.ClearSessionState(workspace.ConfigDir)
	}

	// Auto-trigger onboarding if needed (or explicitly requested).
	if forceOnboarding || needsOnboarding {
		m.StartOnboardingWizard()
	} else if len(m.GetMessages()) == 0 {
		// Show welcome message only if no restored messages and not entering onboarding
		m.ShowWelcome()
	}

	// Store gateway URL for status bar display
	if gatewayURL != "" {
		m.SetGatewayURL(gatewayURL)
	}

	// Initialize the double-pointer for tea.Program sharing
	m.SetProgram(nil)

	// Create program first
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Set the program reference (writes through the double pointer, visible to all copies)
	m.SetProgram(p)

	// Set up permission callback
	orch.SetPermissionCallback(func(req core.PermissionRequest) core.PermissionResponse {
		responseChan := make(chan core.PermissionResponse, 1)
		p.Send(tui.PermissionRequestMsg{
			Request:  req,
			Response: responseChan,
		})
		return <-responseChan
	})

	// Connect to Gateway if URL provided
	if gatewayURL != "" {
		gwClient, err := tui.NewGatewayClient(gatewayURL, p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Gateway connection failed: %v\n", err)
		} else {
			m.SetGatewayClient(gwClient)
			defer gwClient.Close()
			go gwClient.Start()
		}
	}

	_, err := p.Run()

	// Save session state on exit
	m.SaveSession()

	return err
}

// resolveSessionMode decides whether to restore a previous session.
// Returns true to restore, false for fresh start.
func resolveSessionMode(configDir string, fresh, cont bool) bool {
	// Explicit flags take priority
	if fresh {
		return false
	}
	if cont {
		return true
	}

	// Check if a saved session exists
	state, err := core.LoadSessionState(configDir)
	if err != nil || state == nil || len(state.ConversationHistory) == 0 {
		// No previous session — start fresh silently
		return false
	}

	// Previous session exists — ask the user
	msgCount := len(state.Messages)
	histCount := len(state.ConversationHistory)
	savedAt := state.SavedAt.Format("Jan 2, 15:04")

	fmt.Println()
	fmt.Printf("  Previous session found (%d messages, %d turns, saved %s)\n", msgCount, histCount, savedAt)
	fmt.Println()
	fmt.Printf("  \033[1mc\033[0m) Continue previous session\n")
	fmt.Printf("  \033[1mf\033[0m) Fresh start\n")
	fmt.Println()
	fmt.Print("  Choice [c/f]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))

	switch choice {
	case "f", "fresh", "n", "new":
		fmt.Println("  Starting fresh.")
		return false
	default:
		// Default to continue (pressing enter = continue)
		fmt.Println("  Restoring session.")
		return true
	}
}
