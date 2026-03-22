package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostTrackerRecord(t *testing.T) {
	dir := t.TempDir()
	ct := NewCostTracker(dir, "session-1")
	defer ct.Flush()

	// claude-sonnet-4: $3.0/M input, $15.0/M output
	// 1000 input + 500 output → (1000/1M)*3 + (500/1M)*15 = 0.003 + 0.0075 = 0.0105
	ct.Record("anthropic", "claude-sonnet-4-20250514", "session-1", 1000, 500, 0)

	assert.InDelta(t, 0.0105, ct.TotalCost(), 0.0001)
	assert.InDelta(t, 0.0105, ct.CurrentSessionCost(), 0.0001)
	assert.InDelta(t, 0.0105, ct.TodayCost(), 0.0001)
}

func TestCostTrackerCachedTokens(t *testing.T) {
	dir := t.TempDir()
	ct := NewCostTracker(dir, "session-1")
	defer ct.Flush()

	// claude-sonnet-4: $3.0/M input, $15.0/M output, $0.3/M cached
	// 1000 input (200 cached) + 500 output
	// billable input = 800, cached = 200
	// cost = (800/1M)*3 + (200/1M)*0.3 + (500/1M)*15
	//      = 0.0024 + 0.00006 + 0.0075 = 0.00996
	ct.Record("anthropic", "claude-sonnet-4-20250514", "session-1", 1000, 500, 200)

	assert.InDelta(t, 0.00996, ct.TotalCost(), 0.00001)
}

func TestCostTrackerUnknownModel(t *testing.T) {
	dir := t.TempDir()
	ct := NewCostTracker(dir, "session-1")
	defer ct.Flush()

	// Unknown model should record with $0 cost but not panic.
	ct.Record("custom", "some-unknown-model-v9", "session-1", 1000, 500, 0)

	assert.Equal(t, 0.0, ct.TotalCost())
	assert.Equal(t, 1, len(ct.entries))
}

func TestCostTrackerByProvider(t *testing.T) {
	dir := t.TempDir()
	ct := NewCostTracker(dir, "session-1")
	defer ct.Flush()

	ct.Record("anthropic", "claude-sonnet-4", "session-1", 1000, 0, 0)
	ct.Record("openai", "gpt-4.1", "session-1", 1000, 0, 0)

	byProvider := ct.CostByProvider()
	assert.Greater(t, byProvider["anthropic"], 0.0)
	assert.Greater(t, byProvider["openai"], 0.0)
	// Sanity: two distinct providers
	assert.Equal(t, 2, len(byProvider))
}

func TestCostTrackerPersistence(t *testing.T) {
	dir := t.TempDir()

	// First tracker writes a record (async flush to disk).
	ct1 := NewCostTracker(dir, "session-1")
	ct1.Record("anthropic", "claude-sonnet-4", "session-1", 1000, 500, 0)

	// In-memory total is available immediately.
	assert.InDelta(t, 0.0105, ct1.TotalCost(), 0.0001)

	// Wait for background writes to complete.
	ct1.Flush()

	expectedPath := filepath.Join(dir, "logs", "costs.jsonl")
	info, err := os.Stat(expectedPath)
	require.NoError(t, err, "logs/costs.jsonl should exist after Flush")
	require.Greater(t, info.Size(), int64(0), "logs/costs.jsonl should be non-empty")

	// Second tracker reads the file on startup and should see the same total.
	ct2 := NewCostTracker(dir, "session-2")
	assert.InDelta(t, 0.0105, ct2.TotalCost(), 0.001, "second tracker should load historical cost")
}

func TestCostTrackerCostByDay(t *testing.T) {
	dir := t.TempDir()
	ct := NewCostTracker(dir, "session-1")
	defer ct.Flush()
	ct.Record("anthropic", "claude-sonnet-4", "session-1", 1000, 500, 0)

	days := ct.CostByDay(7)
	require.Len(t, days, 1, "should have one day with a cost")
	assert.InDelta(t, 0.0105, days[0].Cost, 0.0001)
}

func TestLookupPricing(t *testing.T) {
	tests := []struct {
		modelID   string
		wantFound bool
		wantInput float64
	}{
		{"claude-sonnet-4-20250514", true, 3.0},
		{"gpt-4.1-mini-preview", true, 0.4},
		{"gpt-4.1", true, 2.0},
		{"deepseek-chat", true, 0.14},
		{"gemini-2.5-flash-latest", true, 0.15},
		{"completely-unknown-model", false, 0},
	}

	for _, tc := range tests {
		p, ok := lookupPricing(tc.modelID)
		assert.Equal(t, tc.wantFound, ok, "model: %s", tc.modelID)
		if ok {
			assert.InDelta(t, tc.wantInput, p.InputPerMillion, 0.001, "model: %s", tc.modelID)
		}
	}
}

func TestFormatCost(t *testing.T) {
	assert.Equal(t, "$0.00", FormatCost(0))
	assert.Equal(t, "$0.0042", FormatCost(0.0042))
	assert.Equal(t, "$0.0105", FormatCost(0.0105))
	assert.Equal(t, "$0.0500", FormatCost(0.05))
	assert.Equal(t, "$1.23", FormatCost(1.23))
	assert.Equal(t, "$0.15", FormatCost(0.15))
}
