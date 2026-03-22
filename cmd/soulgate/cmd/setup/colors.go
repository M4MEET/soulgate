package setup

// Color helpers that delegate to the shared theme package.
// This avoids duplicating ANSI constants across packages.

import "github.com/M4MEET/soulgate/internal/ui/tui/theme"

func colorAccent(s string) string       { return theme.Accent(s) }
func colorAccentBright(s string) string { return theme.AccentBright(s) }
func colorSuccess(s string) string      { return theme.Success(s) }
func colorError(s string) string        { return theme.Error(s) }
func colorWarn(s string) string         { return theme.Warning(s) }
func colorMuted(s string) string        { return theme.Muted(s) }
func colorInfo(s string) string         { return theme.Info(s) }
func colorBold(s string) string         { return theme.Bold(s) }

// Legacy aliases
func colorCyan(s string) string   { return theme.Accent(s) }
func colorGreen(s string) string  { return theme.Success(s) }
func colorRed(s string) string    { return theme.Error(s) }
func colorYellow(s string) string { return theme.Warning(s) }
func colorBlue(s string) string   { return theme.Info(s) }
func colorGray(s string) string   { return theme.Muted(s) }
