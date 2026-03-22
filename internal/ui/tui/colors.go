package tui

// Color helpers that delegate to the theme package.
// Retained for backward compatibility with existing call-sites throughout the TUI.

import "github.com/M4MEET/soulgate/internal/ui/tui/theme"

func colorAccent(s string) string       { return theme.Accent(s) }
func colorAccentBright(s string) string { return theme.AccentBright(s) }
func colorSuccess(s string) string      { return theme.Success(s) }
func colorError(s string) string        { return theme.Error(s) }
func colorWarn(s string) string         { return theme.Warning(s) }
func colorMuted(s string) string        { return theme.Muted(s) }
func colorInfo(s string) string         { return theme.Info(s) }
func colorBold(s string) string         { return theme.Bold(s) }
