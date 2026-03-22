package components

import (
	"strings"

	"github.com/M4MEET/soulgate/internal/ui/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// ConfirmationDialog is a self-contained component for asking the user to
// confirm a potentially destructive action before it executes. It renders an
// overlay and handles the two response keys: yes (y) and no (n/esc).
type ConfirmationDialog struct {
	// Active reports whether the dialog is currently visible. When false,
	// View returns an empty string and HandleKey reports handled=false.
	Active bool

	// Message is the human-readable description shown to the user.
	Message string

	// Command is the specific command or identifier associated with the action.
	Command string

	// PendingAction is the Bubble Tea command to execute when the user confirms.
	// It is cleared after HandleKey processes a decision.
	PendingAction func() tea.Cmd
}

// HandleKey processes a single keypress while the dialog is active.
// It returns:
//   - handled=true   if the key was consumed (caller should not process further)
//   - confirmed=true if the user pressed yes (caller should execute PendingAction)
//
// After HandleKey returns, Active is false and PendingAction is nil regardless
// of whether the user confirmed or cancelled.
func (d *ConfirmationDialog) HandleKey(key string) (handled bool, confirmed bool) {
	if !d.Active {
		return false, false
	}

	switch key {
	case "y", "Y":
		d.Active = false
		d.PendingAction = nil // cleared by caller after extracting it
		return true, true

	case "n", "N", "esc":
		d.Active = false
		d.PendingAction = nil
		return true, false
	}

	// Unrecognized key: consume it so the rest of the TUI does not act on it.
	return true, false
}

// View renders the confirmation dialog overlay. Returns an empty string when
// Active is false.
func (d *ConfirmationDialog) View() string {
	if !d.Active {
		return ""
	}

	var sb strings.Builder
	t := theme.T

	sb.WriteString("  " + t.Warning.Bold(true).Render("Confirm") + "\n\n")
	sb.WriteString("  " + d.Message + "\n")
	sb.WriteString("  " + t.Muted.Render("Command: ") + t.Body.Render(d.Command) + "\n\n")
	sb.WriteString("  " + t.Key.Render("y") + t.Muted.Render(" yes   ") + t.Key.Render("n") + t.Muted.Render(" no") + "\n")

	return sb.String()
}
