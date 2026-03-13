package core

import (
	"fmt"
	"strconv"
	"strings"
)

// ThinkingLevel controls the reasoning depth for a model call.
type ThinkingLevel string

const (
	ThinkOff      ThinkingLevel = "off"
	ThinkMinimal  ThinkingLevel = "minimal"
	ThinkLow      ThinkingLevel = "low"
	ThinkMedium   ThinkingLevel = "medium"
	ThinkHigh     ThinkingLevel = "high"
	ThinkMax      ThinkingLevel = "xhigh"
	ThinkAdaptive ThinkingLevel = "adaptive"
)

// validThinkingLevels is the set of recognised level strings.
var validThinkingLevels = map[ThinkingLevel]bool{
	ThinkOff:      true,
	ThinkMinimal:  true,
	ThinkLow:      true,
	ThinkMedium:   true,
	ThinkHigh:     true,
	ThinkMax:      true,
	ThinkAdaptive: true,
}

// Directives holds session-level directive overrides set by the user inline
// inside their messages. They are stripped from the message text before the
// content is forwarded to the model.
type Directives struct {
	ThinkingLevel ThinkingLevel // Current thinking level
	FastMode      bool          // Reduced reasoning, faster responses
	VerboseMode   string        // "off", "on", "full"
	ReasoningShow string        // "off", "on", "stream" - show thinking in output
	MaxTokens     int           // Override max response tokens (0 = use default)
	Temperature   float64       // Override temperature (-1 = use default)
}

// DefaultDirectives returns sensible defaults that leave all optional
// overrides unset (Temperature -1 signals "not overridden").
func DefaultDirectives() *Directives {
	return &Directives{
		ThinkingLevel: ThinkAdaptive,
		VerboseMode:   "off",
		ReasoningShow: "off",
		Temperature:   -1,
	}
}

// ParseDirectives scans every line of message for inline directives (lines
// that start with a recognised /command token), mutates current in place for
// each directive found, and returns the cleaned message text (directives
// stripped) plus a human-readable description of every directive that was
// applied.
//
// Directives are only recognised when they appear at the very beginning of a
// line (after whitespace trimming). A line that fails to parse as a directive
// is left in the message unchanged.
func ParseDirectives(message string, current *Directives) (cleaned string, applied []string) {
	lines := strings.Split(message, "\n")
	var cleanLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if description, ok := tryParseDirective(trimmed, current); ok {
			applied = append(applied, description)
		} else {
			cleanLines = append(cleanLines, line)
		}
	}

	cleaned = strings.TrimSpace(strings.Join(cleanLines, "\n"))
	return
}

// tryParseDirective attempts to interpret line as a single directive. On
// success it mutates d and returns a human-readable description plus true.
// On failure it returns ("", false) without touching d.
//
// Supported directives
//
//	/think <level>            – set thinking level (off/minimal/low/medium/high/xhigh/adaptive)
//	/fast [on|off]            – toggle fast mode; bare /fast toggles current state
//	/verbose [off|on|full]    – set verbose mode (default: "on" when no argument given)
//	/reasoning [off|on|stream]– set reasoning output mode (default: "on")
//	/temperature <0.0-2.0>    – override temperature
//	/maxtokens <n>            – override maximum response tokens
func tryParseDirective(line string, d *Directives) (string, bool) {
	if !strings.HasPrefix(line, "/") {
		return "", false
	}

	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", false
	}

	cmd := strings.ToLower(fields[0])

	switch cmd {
	case "/think":
		if len(fields) < 2 {
			return "", false
		}
		level := ThinkingLevel(strings.ToLower(fields[1]))
		if !validThinkingLevels[level] {
			return "", false
		}
		d.ThinkingLevel = level
		return fmt.Sprintf("thinking level set to %q", level), true

	case "/fast":
		if len(fields) == 1 {
			// Bare /fast toggles current state.
			d.FastMode = !d.FastMode
			return fmt.Sprintf("fast mode toggled to %v", d.FastMode), true
		}
		switch strings.ToLower(fields[1]) {
		case "on":
			d.FastMode = true
			return "fast mode enabled", true
		case "off":
			d.FastMode = false
			return "fast mode disabled", true
		default:
			return "", false
		}

	case "/verbose":
		if len(fields) == 1 {
			// Bare /verbose defaults to "on".
			d.VerboseMode = "on"
			return "verbose mode set to \"on\"", true
		}
		mode := strings.ToLower(fields[1])
		switch mode {
		case "off", "on", "full":
			d.VerboseMode = mode
			return fmt.Sprintf("verbose mode set to %q", mode), true
		default:
			return "", false
		}

	case "/reasoning":
		if len(fields) == 1 {
			// Bare /reasoning defaults to "on".
			d.ReasoningShow = "on"
			return "reasoning output set to \"on\"", true
		}
		mode := strings.ToLower(fields[1])
		switch mode {
		case "off", "on", "stream":
			d.ReasoningShow = mode
			return fmt.Sprintf("reasoning output set to %q", mode), true
		default:
			return "", false
		}

	case "/temperature":
		if len(fields) < 2 {
			return "", false
		}
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil || v < 0.0 || v > 2.0 {
			return "", false
		}
		d.Temperature = v
		return fmt.Sprintf("temperature set to %.2f", v), true

	case "/maxtokens":
		if len(fields) < 2 {
			return "", false
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil || n <= 0 {
			return "", false
		}
		d.MaxTokens = n
		return fmt.Sprintf("max tokens set to %d", n), true

	default:
		return "", false
	}
}

// ThinkingBudget returns the token budget appropriate for the current thinking
// level. A return value of 0 means thinking is disabled; -1 means the model
// should decide adaptively.
func (d *Directives) ThinkingBudget() int {
	switch d.ThinkingLevel {
	case ThinkOff:
		return 0
	case ThinkMinimal:
		return 512
	case ThinkLow:
		return 2048
	case ThinkMedium:
		return 4096
	case ThinkHigh:
		return 8192
	case ThinkMax:
		return 16384
	case ThinkAdaptive:
		return -1 // let the model decide
	default:
		return -1
	}
}

// ApplyToRequest merges the active directives into a generic completion
// request map. Callers are responsible for translating the resulting map into
// their provider-specific request struct.
func (d *Directives) ApplyToRequest(req map[string]interface{}) {
	if d.FastMode {
		req["fast_mode"] = true
	}
	if d.Temperature >= 0 {
		req["temperature"] = d.Temperature
	}
	if d.MaxTokens > 0 {
		req["max_tokens"] = d.MaxTokens
	}
	budget := d.ThinkingBudget()
	if budget >= 0 {
		req["thinking_budget"] = budget
	}
}

// String returns a compact, human-readable summary of the active non-default
// directives. Returns "defaults" when nothing has been overridden.
func (d *Directives) String() string {
	var parts []string

	if d.ThinkingLevel != ThinkAdaptive {
		parts = append(parts, fmt.Sprintf("think:%s", d.ThinkingLevel))
	}
	if d.FastMode {
		parts = append(parts, "fast")
	}
	if d.VerboseMode != "off" {
		parts = append(parts, fmt.Sprintf("verbose:%s", d.VerboseMode))
	}
	if d.ReasoningShow != "off" {
		parts = append(parts, fmt.Sprintf("reasoning:%s", d.ReasoningShow))
	}

	if len(parts) == 0 {
		return "defaults"
	}
	return strings.Join(parts, " ")
}
