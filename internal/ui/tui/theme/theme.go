// Package theme provides the centralized color and style system for the SoulGate TUI.
// All semantic style roles are defined here so that color values are never scattered
// across individual rendering files.
//
// Usage:
//
//	import "github.com/M4MEET/soulgate/internal/ui/tui/theme"
//
//	// Use the package-level default theme variable
//	theme.T.Success.Render("ok")
//
//	// Use the helper functions for one-off coloring
//	theme.Success("all clear")
//	theme.Error("something went wrong")
package theme

import "github.com/charmbracelet/lipgloss"

// Theme holds lipgloss styles for every semantic role used across the TUI.
// All styles are value types (lipgloss.Style is a struct), so the Theme itself
// is cheap to copy and safe to use from multiple goroutines as long as callers
// do not mutate the styles after construction.
type Theme struct {
	// -------------------------------------------------------------------------
	// Text
	// -------------------------------------------------------------------------

	// Title is used for section headings and major labels (255 bold).
	Title lipgloss.Style
	// Subtitle is used for sub-headings (252).
	Subtitle lipgloss.Style
	// Body is the default prose text color (252).
	Body lipgloss.Style
	// Muted is for secondary / de-emphasized text (244).
	Muted lipgloss.Style
	// Dim is for very subtle text that should barely draw attention (242).
	Dim lipgloss.Style

	// -------------------------------------------------------------------------
	// Status
	// -------------------------------------------------------------------------

	// Success indicates a successful or positive state (42, emerald).
	Success lipgloss.Style
	// Error indicates a failure or dangerous state (203, red).
	Error lipgloss.Style
	// Warning indicates a caution state (214, amber).
	Warning lipgloss.Style
	// Info is for informational callouts (210, coral).
	Info lipgloss.Style
	// Accent is the primary brand color (208, orange).
	Accent lipgloss.Style
	// AccentBright is a brighter variant of the brand color (215).
	AccentBright lipgloss.Style
	// AccentDim is a darker variant of the brand color (202).
	AccentDim lipgloss.Style

	// -------------------------------------------------------------------------
	// UI elements
	// -------------------------------------------------------------------------

	// Key is used for keyboard shortcut labels (117, blue).
	Key lipgloss.Style
	// Command is used for slash command names in help/autocomplete (117).
	Command lipgloss.Style
	// Tool is used for tool names in listings and output (117).
	Tool lipgloss.Style
	// Value is used for config values and identifiers (117).
	Value lipgloss.Style

	// -------------------------------------------------------------------------
	// Code
	// -------------------------------------------------------------------------

	// CodeBlock is the default style applied to lines inside fenced code blocks (117).
	CodeBlock lipgloss.Style
	// CodeKeyword highlights language keywords inside code blocks (176).
	CodeKeyword lipgloss.Style
	// CodeString highlights string literals inside code blocks (179).
	CodeString lipgloss.Style
	// CodeComment highlights comments inside code blocks (242 italic).
	CodeComment lipgloss.Style
	// InlineCode styles backtick-delimited inline code (117 fg, 236 bg).
	InlineCode lipgloss.Style

	// -------------------------------------------------------------------------
	// Thinking / tool activity
	// -------------------------------------------------------------------------

	// Spinner is the color of the activity spinner in the status bar (208).
	Spinner lipgloss.Style
	// ThinkingLabel is the "thinking" section header (208 bold).
	ThinkingLabel lipgloss.Style
	// ThinkingDim is for the separator lines inside the thinking panel (238).
	ThinkingDim lipgloss.Style
	// ToolCall is for the tool-call entry line (117 bold).
	ToolCall lipgloss.Style
	// ToolResult is for the tool-result completion line (42).
	ToolResult lipgloss.Style

	// -------------------------------------------------------------------------
	// Separators / chrome
	// -------------------------------------------------------------------------

	// Separator is used for horizontal rule characters (236).
	Separator lipgloss.Style

	// -------------------------------------------------------------------------
	// Header
	// -------------------------------------------------------------------------

	// HeaderName is the "SoulGate" wordmark in the header (208 bold).
	HeaderName lipgloss.Style
	// HeaderTagline is the subtitle text next to the wordmark (242).
	HeaderTagline lipgloss.Style
}

// Default returns a Theme populated with the "Octopus" color palette that was
// previously scattered as inline lipgloss.NewStyle() calls throughout the TUI.
// The function constructs all styles once; callers should store the result or use
// the package-level T variable rather than calling Default() in hot paths.
func Default() Theme {
	return Theme{
		// Text
		Title:    lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true),
		Subtitle: lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Body:     lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		Muted:    lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		Dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("242")),

		// Status
		Success:      lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		Error:        lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
		Warning:      lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Info:         lipgloss.NewStyle().Foreground(lipgloss.Color("210")),
		Accent:       lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
		AccentBright: lipgloss.NewStyle().Foreground(lipgloss.Color("215")),
		AccentDim:    lipgloss.NewStyle().Foreground(lipgloss.Color("202")),

		// UI elements
		Key:     lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
		Command: lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
		Tool:    lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
		Value:   lipgloss.NewStyle().Foreground(lipgloss.Color("117")),

		// Code
		CodeBlock:   lipgloss.NewStyle().Foreground(lipgloss.Color("117")),
		CodeKeyword: lipgloss.NewStyle().Foreground(lipgloss.Color("176")),
		CodeString:  lipgloss.NewStyle().Foreground(lipgloss.Color("179")),
		CodeComment: lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true),
		InlineCode:  lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Background(lipgloss.Color("236")),

		// Thinking / tool activity
		Spinner:       lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
		ThinkingLabel: lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
		ThinkingDim:   lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		ToolCall:      lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
		ToolResult:    lipgloss.NewStyle().Foreground(lipgloss.Color("42")),

		// Separators / chrome
		Separator: lipgloss.NewStyle().Foreground(lipgloss.Color("236")),

		// Header
		HeaderName:    lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true),
		HeaderTagline: lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
	}
}

// T is the package-level default theme instance. It is initialized once at
// program startup and is safe to read from any goroutine. If you need a
// different theme, create one with Default() (or a custom constructor) and
// substitute it for T before any rendering occurs.
//
//	theme.T.Success.Render("all good")
var T = Default()

// ---------------------------------------------------------------------------
// Helper functions
//
// These are thin wrappers that apply a single semantic style and return the
// rendered string. They replicate the colorXxx() functions in colors.go so
// that callers can migrate one call-site at a time.
// ---------------------------------------------------------------------------

// Accent renders s in the primary brand color (208, orange).
func Accent(s string) string { return T.Accent.Render(s) }

// AccentBright renders s in the bright brand color (215).
func AccentBright(s string) string { return T.AccentBright.Render(s) }

// Success renders s in the success/positive color (42, emerald).
func Success(s string) string { return T.Success.Render(s) }

// Error renders s in the error/danger color (203, red).
func Error(s string) string { return T.Error.Render(s) }

// Warning renders s in the warning/caution color (214, amber).
func Warning(s string) string { return T.Warning.Render(s) }

// Muted renders s in the muted/secondary text color (244).
func Muted(s string) string { return T.Muted.Render(s) }

// Bold renders s with bold weight applied on top of the body color.
// It intentionally does not apply a foreground color so it composes
// cleanly with surrounding text.
func Bold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

// Info renders s in the informational callout color (210, coral).
func Info(s string) string { return T.Info.Render(s) }
