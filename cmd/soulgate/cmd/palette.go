package cmd

// Octopus palette tokens for CLI/UI theming
// The octopus represents intelligence, adaptability, and multi-tasking
// "octopus seam" == use this palette consistently

// OctopusPalette defines the SoulGate color scheme
type OctopusPalette struct {
	// Primary accent color (vibrant orange)
	Accent string
	// Brighter accent for highlights
	AccentBright string
	// Dimmer accent for backgrounds
	AccentDim string
	// Info messages (coral pink)
	Info string
	// Success messages (emerald green)
	Success string
	// Warnings (golden amber)
	Warn string
	// Errors (deep red)
	Error string
	// Muted text (slate gray)
	Muted string
}

// OCTOPUS_PALETTE is the default SoulGate color palette
var OCTOPUS_PALETTE = OctopusPalette{
	Accent:       "#FF6B35", // Vibrant orange (octopus energy)
	AccentBright: "#FF8C5F", // Bright orange
	AccentDim:    "#E5552E", // Dark orange
	Info:         "#FF8474", // Coral pink (ocean creature)
	Success:      "#00C896", // Emerald green (sea vegetation)
	Warn:         "#FFA500", // Golden amber (warning glow)
	Error:        "#E63946", // Deep red (alert)
	Muted:        "#6C757D", // Slate gray
}

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

// Octopus seam: use the Octopus palette consistently across all CLI output
// This ensures a cohesive visual identity representing intelligence and adaptability
