package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// record is a shorthand for recording a tool call with the given name and a
// simple single-key args map, so tests stay readable.
func record(ld *LoopDetector, name string, arg string) {
	ld.Record(name, map[string]interface{}{"input": arg})
}

// recordN records the same (name, arg) pair n times.
func recordN(ld *LoopDetector, name, arg string, n int) {
	for i := 0; i < n; i++ {
		record(ld, name, arg)
	}
}

// ---------------------------------------------------------------------------
// Basic sanity: no loop when calls are varied
// ---------------------------------------------------------------------------

func TestNoLoop(t *testing.T) {
	ld := NewLoopDetector()

	record(ld, "files_read", "a.txt")
	record(ld, "files_write", "b.txt")
	record(ld, "exec_command", "ls")
	record(ld, "net_request", "https://example.com")
	record(ld, "memory_get", "key1")

	result := ld.Check()
	assert.Equal(t, "none", result.Level)
	assert.False(t, result.Detected)
}

func TestSingleCallNoLoop(t *testing.T) {
	ld := NewLoopDetector()
	record(ld, "files_read", "a.txt")

	result := ld.Check()
	assert.Equal(t, "none", result.Level)
}

// ---------------------------------------------------------------------------
// Generic repeat: identical tool + identical args
// ---------------------------------------------------------------------------

func TestGenericRepeatWarning(t *testing.T) {
	ld := NewLoopDetector() // WarningThreshold=3, CriticalThreshold=5

	recordN(ld, "files_read", "same.txt", 3)

	result := ld.Check()
	assert.True(t, result.Detected)
	assert.Equal(t, "warning", result.Level)
	assert.Equal(t, 3, result.RepeatCount)
	assert.Contains(t, result.Pattern, "files_read")
}

func TestGenericRepeat(t *testing.T) {
	ld := NewLoopDetector()

	// Record 5 identical calls — should reach "critical".
	recordN(ld, "files_read", "same.txt", 5)

	result := ld.Check()
	assert.True(t, result.Detected)
	assert.Equal(t, "critical", result.Level)
	assert.Equal(t, 5, result.RepeatCount)
	assert.NotEmpty(t, result.Suggestion)
}

func TestGenericRepeatDifferentArgs(t *testing.T) {
	ld := NewLoopDetector()

	// Same tool but different arguments should NOT trigger a repeat.
	record(ld, "files_read", "a.txt")
	record(ld, "files_read", "b.txt")
	record(ld, "files_read", "c.txt")
	record(ld, "files_read", "d.txt")
	record(ld, "files_read", "e.txt")

	result := ld.Check()
	// Each call has a different arg → the consecutive-identical count is 1.
	assert.Equal(t, "none", result.Level)
}

func TestGenericRepeatInterruptedByDifferentCall(t *testing.T) {
	ld := NewLoopDetector()

	// Build up near-critical, then break the streak.
	recordN(ld, "files_read", "x.txt", 4)
	record(ld, "exec_command", "ls") // breaks the streak
	record(ld, "files_read", "x.txt")

	result := ld.Check()
	// Streak is now 1 (only the last files_read counts).
	assert.Equal(t, "none", result.Level)
}

// ---------------------------------------------------------------------------
// Ping-pong: A-B-A-B alternating
// ---------------------------------------------------------------------------

func TestPingPong(t *testing.T) {
	ld := NewLoopDetector() // WarningThreshold=3

	// 3 full AB cycles = 6 entries: A B A B A B
	for i := 0; i < 3; i++ {
		record(ld, "files_read", "f.txt")
		record(ld, "exec_command", "check")
	}

	result := ld.Check()
	assert.True(t, result.Detected)
	assert.Equal(t, "warning", result.Level)
	assert.GreaterOrEqual(t, result.RepeatCount, 3)
	assert.NotEmpty(t, result.Pattern)
}

func TestPingPongCritical(t *testing.T) {
	ld := NewLoopDetector() // CriticalThreshold=5

	// 5 full AB cycles = 10 entries.
	for i := 0; i < 5; i++ {
		record(ld, "files_read", "f.txt")
		record(ld, "exec_command", "check")
	}

	result := ld.Check()
	assert.True(t, result.Detected)
	assert.Equal(t, "critical", result.Level)
}

func TestPingPongNotTriggeredForTwoAB(t *testing.T) {
	ld := NewLoopDetector() // WarningThreshold=3

	// Only 2 AB cycles — below the warning threshold.
	record(ld, "files_read", "f.txt")
	record(ld, "exec_command", "check")
	record(ld, "files_read", "f.txt")
	record(ld, "exec_command", "check")

	result := ld.Check()
	assert.Equal(t, "none", result.Level)
}

// ---------------------------------------------------------------------------
// Short cycle: A-B-C-A-B-C
// ---------------------------------------------------------------------------

func TestShortCycleDetected(t *testing.T) {
	ld := NewLoopDetector()

	// 3 complete ABC cycles.
	for i := 0; i < 3; i++ {
		record(ld, "files_read", "f.txt")
		record(ld, "exec_command", "build")
		record(ld, "net_request", "http://api")
	}

	result := ld.Check()
	assert.True(t, result.Detected, "3-tool cycle should be detected")
	assert.NotEqual(t, "none", result.Level)
	assert.GreaterOrEqual(t, result.RepeatCount, 2)
}

func TestShortCycleNotTriggeredOnSingleCycle(t *testing.T) {
	ld := NewLoopDetector()

	// Only one ABC sequence — not a loop.
	record(ld, "files_read", "f.txt")
	record(ld, "exec_command", "build")
	record(ld, "net_request", "http://api")

	result := ld.Check()
	assert.Equal(t, "none", result.Level)
}

// ---------------------------------------------------------------------------
// Warning → Critical escalation
// ---------------------------------------------------------------------------

func TestWarningThenCritical(t *testing.T) {
	ld := NewLoopDetector() // Warning=3, Critical=5

	// At 3 repeats: warning.
	recordN(ld, "files_read", "z.txt", 3)
	r1 := ld.Check()
	assert.Equal(t, "warning", r1.Level)
	assert.Equal(t, 3, r1.RepeatCount)

	// At 5 repeats: critical.
	recordN(ld, "files_read", "z.txt", 2) // total = 5
	r2 := ld.Check()
	assert.Equal(t, "critical", r2.Level)
	assert.Equal(t, 5, r2.RepeatCount)
}

// ---------------------------------------------------------------------------
// Reset
// ---------------------------------------------------------------------------

func TestReset(t *testing.T) {
	ld := NewLoopDetector()

	// Trigger a critical loop.
	recordN(ld, "files_read", "same.txt", 5)
	before := ld.Check()
	assert.Equal(t, "critical", before.Level)

	// After reset, detection should be clear.
	ld.Reset()
	after := ld.Check()
	assert.Equal(t, "none", after.Level)
	assert.False(t, after.Detected)
}

func TestResetAllowsReuseAfterLoop(t *testing.T) {
	ld := NewLoopDetector()

	// First loop.
	recordN(ld, "exec_command", "broken", 5)
	assert.Equal(t, "critical", ld.Check().Level)

	// Reset and record a fresh, varied sequence.
	ld.Reset()
	record(ld, "files_read", "a.txt")
	record(ld, "files_write", "b.txt")
	assert.Equal(t, "none", ld.Check().Level)
}

// ---------------------------------------------------------------------------
// Custom thresholds
// ---------------------------------------------------------------------------

func TestCustomThresholds(t *testing.T) {
	ld := &LoopDetector{
		historySize:       20,
		WarningThreshold:  2,
		CriticalThreshold: 3,
	}

	// 2 identical calls → warning with custom threshold.
	recordN(ld, "files_read", "t.txt", 2)
	r1 := ld.Check()
	assert.Equal(t, "warning", r1.Level)

	// 3 identical calls → critical with custom threshold.
	record(ld, "files_read", "t.txt")
	r2 := ld.Check()
	assert.Equal(t, "critical", r2.Level)
}

// ---------------------------------------------------------------------------
// History window eviction
// ---------------------------------------------------------------------------

func TestHistoryWindowBounded(t *testing.T) {
	ld := NewLoopDetector() // historySize=20

	// Record 25 varied calls to overflow the window.
	tools := []string{"files_read", "exec_command", "net_request", "memory_get", "files_list"}
	for i := 0; i < 25; i++ {
		record(ld, tools[i%len(tools)], "x")
	}

	ld.mu.Lock()
	histLen := len(ld.history)
	ld.mu.Unlock()

	assert.LessOrEqual(t, histLen, 20, "history must not exceed historySize")
}

// ---------------------------------------------------------------------------
// argsHash helper
// ---------------------------------------------------------------------------

func TestArgsHashNilArgs(t *testing.T) {
	h := argsHash(nil)
	assert.Equal(t, "{}", h)
}

func TestArgsHashConsistency(t *testing.T) {
	args := map[string]interface{}{"path": "foo.txt", "mode": "r"}
	h1 := argsHash(args)
	h2 := argsHash(args)
	assert.Equal(t, h1, h2, "hash must be deterministic for same input")
}

func TestArgsHashDistinctForDifferentArgs(t *testing.T) {
	h1 := argsHash(map[string]interface{}{"path": "a.txt"})
	h2 := argsHash(map[string]interface{}{"path": "b.txt"})
	assert.NotEqual(t, h1, h2)
}
