package cmd

import (
	"os"
	"path/filepath"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	"github.com/M4MEET/soulgate/internal/ui/tui"
	tea "github.com/charmbracelet/bubbletea"
)

// RunInteractiveTUI starts the interactive TUI
// This is a thin wrapper that delegates to the tui package
func RunInteractiveTUI(orch *core.Orchestrator) error {
	workspace := orch.GetWorkspace()

	// Check if onboarding is needed
	needsOnboarding := false
	markerPath := filepath.Join(workspace.ConfigDir, ".onboarding_complete")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		needsOnboarding = true
	}

	// Create the TUI model
	m := tui.NewInteractiveChatModel(orch)

	// Auto-trigger onboarding if needed
	if needsOnboarding {
		m.ShowOnboarding = true
		m.OnboardingState = onboarding.NewOnboardingState(workspace)
	} else {
		// Show welcome message only if not entering onboarding
		m.ShowWelcome()
	}

	// Create program first
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Set up permission callback
	orch.SetPermissionCallback(func(req core.PermissionRequest) core.PermissionResponse {
		// Create response channel
		responseChan := make(chan core.PermissionResponse, 1)

		// Send permission request message to TUI
		p.Send(tui.PermissionRequestMsg{
			Request:  req,
			Response: responseChan,
		})

		// Wait for user response
		response := <-responseChan

		return response
	})

	_, err := p.Run()
	return err
}
