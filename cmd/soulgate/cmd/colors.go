package cmd

// Color and formatting helpers for terminal output
// These are shared across multiple commands (setup, chat, etc.)
//
// NOTE: This file is kept for backward compatibility.
// All color functions now use the Turtle palette (palette.go)
//
// OpenClaw uses "Lobster" theme (red-orange) - we use "Turtle" theme (teal-green)
// See: palette.go for the full Turtle color palette

// All color functions are now defined in palette.go
// This file exists only to document the "turtle seam" pattern:
//
// Turtle Seam: Use the Turtle palette consistently across all CLI output
// - Never hardcode ANSI codes
// - Always use palette.go color functions
// - Maintain visual cohesion across all commands
//
// Available colors:
// - colorAccent() - Teal (turtle shell) - primary brand color
// - colorSuccess() - Green - success messages
// - colorWarn() - Amber - warnings
// - colorError() - Coral red - errors
// - colorInfo() - Ocean blue - info messages
// - colorMuted() - Gray - secondary text
// - colorBold() - Bold text
//
// Legacy aliases (mapped to Turtle palette):
// - colorCyan() → colorAccent()
// - colorGreen() → colorSuccess()
// - colorRed() → colorError()
// - colorYellow() → colorWarn()
// - colorBlue() → colorInfo()
// - colorGray() → colorMuted()
