package tui

// This file re-exports color functions from the parent package.
// Since tui is a subpackage of cmd, we can access the parent package's
// unexported functions by creating wrapper functions here.
//
// However, a cleaner approach is to move these color functions to a shared location
// or export them. For now, we'll define them using the same ANSI codes.

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
// These match the color functions in the parent cmd package

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
