package components

import (
	"strings"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/ui/tui/theme"
)

// PermissionPrompt is a self-contained component for requesting user approval
// of a policy-denied operation. It renders an overlay and handles the three
// response keys: allow-once (a), learn (l), and deny (d/n/esc).
type PermissionPrompt struct {
	// Active reports whether the prompt is currently visible. When false,
	// View returns an empty string and HandleKey reports handled=false.
	Active bool

	// Request holds the details of the permission being requested. It must
	// be non-nil whenever Active is true.
	Request *core.PermissionRequest

	// Response is the channel that the orchestrator goroutine is blocking on.
	// HandleKey sends a PermissionResponse to this channel and then sets it
	// to nil and Active to false.
	Response chan core.PermissionResponse
}

// HandleKey processes a single keypress while the prompt is active.
// It returns:
//   - handled=true  if the key was consumed (caller should not process it further)
//   - result non-nil if a decision was made (the response has been sent to Response)
//
// When handled=true and result==nil the key was consumed but no decision was
// made yet (e.g. an unrecognized key that should be ignored while the prompt
// is open). The caller must still return early to prevent further key routing.
//
// After HandleKey returns a non-nil result, Active is false and Response is nil.
func (p *PermissionPrompt) HandleKey(key string) (handled bool, result *core.PermissionResponse) {
	if !p.Active || p.Response == nil {
		return false, nil
	}

	switch key {
	case "a", "A":
		resp := core.PermissionResponse{Approved: true, LearnPattern: false}
		p.Response <- resp
		p.Response = nil
		p.Active = false
		return true, &resp

	case "l", "L":
		resp := core.PermissionResponse{Approved: true, LearnPattern: true}
		p.Response <- resp
		p.Response = nil
		p.Active = false
		return true, &resp

	case "d", "D", "n", "N", "esc":
		resp := core.PermissionResponse{Approved: false, LearnPattern: false}
		p.Response <- resp
		p.Response = nil
		p.Active = false
		return true, &resp
	}

	// Unrecognized key: consume it so the rest of the TUI does not act on it.
	return true, nil
}

// View renders the permission prompt overlay. Returns an empty string when
// Active is false or Request is nil.
func (p *PermissionPrompt) View() string {
	if !p.Active || p.Request == nil {
		return ""
	}

	var sb strings.Builder
	t := theme.T

	sb.WriteString("  " + t.Warning.Bold(true).Render("Permission Required") + "\n\n")
	sb.WriteString("  " + p.Request.Description + "\n\n")
	sb.WriteString("  " + t.Muted.Render("Action:   ") + p.Request.Action + "\n")
	sb.WriteString("  " + t.Muted.Render("Resource: ") + p.Request.Resource + "\n")
	if p.Request.Reason != "" {
		sb.WriteString("  " + t.Muted.Render("Reason:   ") + p.Request.Reason + "\n")
	}
	sb.WriteString("\n")

	pattern := core.GenerateSmartPattern(p.Request.Action, p.Request.Resource)
	sb.WriteString("  " + t.Key.Render("a") + t.Muted.Render(" allow once   "))
	sb.WriteString(t.Key.Render("l") + t.Muted.Render(" learn ("+pattern+")   "))
	sb.WriteString(t.Key.Render("d") + t.Muted.Render(" deny") + "\n")

	return sb.String()
}
