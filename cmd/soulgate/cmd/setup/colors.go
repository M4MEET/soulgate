package setup

// ANSI color codes for 256-color terminal support
const (
	// Octopus theme ANSI codes (approximating hex colors)
	AnsiAccent       = "\033[38;5;208m" // Vibrant orange
	AnsiAccentBright = "\033[38;5;215m" // Bright orange
	AnsiAccentDim    = "\033[38;5;202m" // Dark orange
	AnsiInfo         = "\033[38;5;210m" // Coral pink
	AnsiSuccess      = "\033[38;5;42m"  // Emerald green
	AnsiWarn         = "\033[38;5;214m" // Golden amber
	AnsiError        = "\033[38;5;203m" // Deep red
	AnsiMuted        = "\033[38;5;244m" // Slate gray
	AnsiBold         = "\033[1m"
	AnsiReset        = "\033[0m"
)

// Color helper functions using Octopus palette

func colorAccent(s string) string {
	return AnsiAccent + s + AnsiReset
}

func colorAccentBright(s string) string {
	return AnsiAccentBright + s + AnsiReset
}

func colorAccentDim(s string) string {
	return AnsiAccentDim + s + AnsiReset
}

func colorInfo(s string) string {
	return AnsiInfo + s + AnsiReset
}

func colorSuccess(s string) string {
	return AnsiSuccess + s + AnsiReset
}

func colorWarn(s string) string {
	return AnsiWarn + s + AnsiReset
}

func colorError(s string) string {
	return AnsiError + s + AnsiReset
}

func colorMuted(s string) string {
	return AnsiMuted + s + AnsiReset
}

func colorBold(s string) string {
	return AnsiBold + s + AnsiReset
}

// Legacy color function aliases (for backward compatibility)
// Map to Octopus palette equivalents

func colorCyan(s string) string {
	return colorAccent(s) // Cyan → Orange (octopus accent)
}

func colorGreen(s string) string {
	return colorSuccess(s) // Green → Success
}

func colorRed(s string) string {
	return colorError(s) // Red → Error
}

func colorYellow(s string) string {
	return colorWarn(s) // Yellow → Warn
}

func colorBlue(s string) string {
	return colorInfo(s) // Blue → Info
}

func colorGray(s string) string {
	return colorMuted(s) // Gray → Muted
}
