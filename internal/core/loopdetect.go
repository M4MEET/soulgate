package core

import (
	"encoding/json"
	"fmt"
	"sync"
)

// LoopDetector watches for repetitive tool-call patterns within a single
// agentic-loop run. It is not safe to share a single LoopDetector across
// concurrent runs; create one per run.
type LoopDetector struct {
	history     []toolCallRecord
	historySize int

	// WarningThreshold is the number of consecutive identical calls before a
	// warning is raised. Default: 3.
	WarningThreshold int

	// CriticalThreshold is the number of consecutive identical calls before the
	// loop is considered stuck and must be interrupted. Default: 5.
	CriticalThreshold int

	mu sync.Mutex
}

// toolCallRecord stores a normalised fingerprint of one tool invocation.
type toolCallRecord struct {
	ToolName string
	ArgsHash string
}

// LoopDetection is the result returned by LoopDetector.Check.
type LoopDetection struct {
	// Detected is true when any non-"none" level is active.
	Detected bool

	// Level is "none", "warning", or "critical".
	Level string

	// Pattern is a short description of the repetition observed, e.g.
	// "files_read called 5 times with identical arguments".
	Pattern string

	// Suggestion is a hint for the caller / model about how to proceed
	// differently.
	Suggestion string

	// RepeatCount is how many times the offending pattern has been seen.
	RepeatCount int
}

// NewLoopDetector creates a LoopDetector with default thresholds.
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		historySize:       20,
		WarningThreshold:  3,
		CriticalThreshold: 5,
	}
}

// Record appends a tool call to the rolling history window.
// args is serialised to JSON for deterministic comparison; if serialisation
// fails a fmt.Sprintf fallback is used so Record never drops data silently.
func (ld *LoopDetector) Record(toolName string, args map[string]interface{}) {
	hash := argsHash(args)

	ld.mu.Lock()
	defer ld.mu.Unlock()

	ld.history = append(ld.history, toolCallRecord{
		ToolName: toolName,
		ArgsHash: hash,
	})

	// Trim to the configured window size.
	if len(ld.history) > ld.historySize {
		ld.history = ld.history[len(ld.history)-ld.historySize:]
	}
}

// Check analyses the recent history for three loop patterns and returns the
// highest-severity detection found.
//
//   - Generic repeat : the most-recent N calls share identical tool+args.
//   - Ping-pong      : alternating A-B-A-B… pattern of length >= 4.
//   - Short cycle    : repeating A-B-C-A-B-C… cycle of period 3+ and >= 2
//     complete repetitions visible.
func (ld *LoopDetector) Check() LoopDetection {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	if len(ld.history) < 2 {
		return LoopDetection{Level: "none"}
	}

	// Evaluate all three patterns and return the most severe one found.
	candidates := []LoopDetection{
		ld.checkGenericRepeat(),
		ld.checkPingPong(),
		ld.checkShortCycle(),
	}

	best := LoopDetection{Level: "none"}
	for _, c := range candidates {
		if severityOf(c.Level) > severityOf(best.Level) {
			best = c
		}
	}

	return best
}

// Reset clears all recorded history so the detector starts fresh.
func (ld *LoopDetector) Reset() {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.history = ld.history[:0]
}

// ---------------------------------------------------------------------------
// Pattern detectors (called with mu already held)
// ---------------------------------------------------------------------------

// checkGenericRepeat counts how many of the most-recent entries are
// identical (same tool + same args hash) and maps the count to a severity.
func (ld *LoopDetector) checkGenericRepeat() LoopDetection {
	h := ld.history
	last := h[len(h)-1]

	count := 0
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].ToolName == last.ToolName && h[i].ArgsHash == last.ArgsHash {
			count++
		} else {
			break
		}
	}

	if count < ld.WarningThreshold {
		return LoopDetection{Level: "none"}
	}

	level := "warning"
	if count >= ld.CriticalThreshold {
		level = "critical"
	}

	return LoopDetection{
		Detected:    true,
		Level:       level,
		Pattern:     fmt.Sprintf("%s called %d times with identical arguments", last.ToolName, count),
		Suggestion:  "Try a different approach or tool; repeating the same call will not produce a new result.",
		RepeatCount: count,
	}
}

// checkPingPong detects an A-B-A-B… alternating pattern where A != B.
// It requires at least 4 entries and at least WarningThreshold full AB cycles.
func (ld *LoopDetector) checkPingPong() LoopDetection {
	h := ld.history
	if len(h) < 4 {
		return LoopDetection{Level: "none"}
	}

	// The last two entries define the candidate A and B positions.
	// In a ping-pong ...A B A B the last entry is B (period-2 position).
	period := 2
	count := countTrailingCycle(h, period)
	// count is the number of full copies of the 2-element cycle visible at the
	// tail, expressed as total entries (e.g. A B A B → count=4 → 2 complete AB cycles).
	fullCycles := count / period

	if fullCycles < ld.WarningThreshold {
		return LoopDetection{Level: "none"}
	}

	level := "warning"
	if fullCycles >= ld.CriticalThreshold {
		level = "critical"
	}

	a := h[len(h)-2].ToolName
	b := h[len(h)-1].ToolName

	return LoopDetection{
		Detected:    true,
		Level:       level,
		Pattern:     fmt.Sprintf("ping-pong between %s and %s (%d cycles)", a, b, fullCycles),
		Suggestion:  "The model is alternating between two tools without making progress. Consider a different strategy.",
		RepeatCount: fullCycles,
	}
}

// checkShortCycle detects repeating A-B-C-A-B-C… sequences for period 3+.
// It looks for cycles of period 3 to 5 with at least 2 full repetitions.
func (ld *LoopDetector) checkShortCycle() LoopDetection {
	h := ld.history

	best := LoopDetection{Level: "none"}

	for period := 3; period <= 5; period++ {
		if len(h) < period*2 {
			continue
		}

		count := countTrailingCycle(h, period)
		fullCycles := count / period

		if fullCycles < 2 {
			continue
		}

		level := "warning"
		if fullCycles >= ld.CriticalThreshold {
			level = "critical"
		}

		// Build a short description of the cycle pattern.
		cycleTools := make([]string, period)
		for i := 0; i < period; i++ {
			cycleTools[i] = h[len(h)-period+i].ToolName
		}
		pattern := fmt.Sprintf("repeating cycle of %d tools [%s] (%d iterations)",
			period, joinToolNames(cycleTools), fullCycles)

		d := LoopDetection{
			Detected:    true,
			Level:       level,
			Pattern:     pattern,
			Suggestion:  "A repetitive call cycle was detected. Break out of it by re-evaluating the goal.",
			RepeatCount: fullCycles,
		}

		if severityOf(d.Level) > severityOf(best.Level) {
			best = d
		}
	}

	return best
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// countTrailingCycle returns the length of the longest suffix of h that
// consists of an exact repetition of the last `period` entries.
// For example, with h = [A B C A B C A B C] and period=3 it returns 9.
// The return value is always a multiple of period, and at minimum period
// (a single copy is the baseline; the function returns 0 when the period
// pattern doesn't repeat at least twice).
func countTrailingCycle(h []toolCallRecord, period int) int {
	if len(h) < period*2 {
		return 0
	}

	// The reference pattern is the last `period` entries.
	ref := h[len(h)-period:]

	// Walk backwards from len(h)-period-1 to check how far the cycle extends.
	matched := period // we always have at least one copy (the reference itself)
	pos := len(h) - period - 1

	for pos >= 0 {
		// Check if the period entries ending at pos match the reference.
		start := pos - period + 1
		if start < 0 {
			break
		}
		window := h[start : pos+1]
		if !cycleMatch(window, ref) {
			break
		}
		matched += period
		pos -= period
	}

	if matched < period*2 {
		// Only one copy found (the reference); no real repetition.
		return 0
	}
	return matched
}

// cycleMatch returns true if a and b have the same length and contain the
// same tool names AND arguments. Different args means different intent —
// e.g. exec_command("open Chrome") followed by exec_command("Cmd+Shift+N")
// is not a loop.
func cycleMatch(a, b []toolCallRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ToolName != b[i].ToolName || a[i].ArgsHash != b[i].ArgsHash {
			return false
		}
	}
	return true
}

// severityOf maps a level string to a numeric value for comparison.
func severityOf(level string) int {
	switch level {
	case "critical":
		return 2
	case "warning":
		return 1
	default:
		return 0
	}
}

// argsHash produces a stable string fingerprint for a tool-arguments map.
// JSON marshalling is used for deterministic key ordering. If marshalling
// fails (which should be extremely rare for map[string]interface{}) we fall
// back to fmt.Sprintf.
func argsHash(args map[string]interface{}) string {
	if args == nil {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(b)
}

// joinToolNames concatenates tool names with " → " separators for display.
func joinToolNames(names []string) string {
	result := ""
	for i, n := range names {
		if i > 0 {
			result += " -> "
		}
		result += n
	}
	return result
}
