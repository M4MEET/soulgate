package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ModelPricing holds per-million-token prices for a model.
type ModelPricing struct {
	InputPerMillion  float64 // $/1M input tokens
	OutputPerMillion float64 // $/1M output tokens
	CachedPerMillion float64 // $/1M cached input tokens (Anthropic prompt caching)
}

// modelPricing is the embedded pricing table. Model IDs are matched by substring
// so partial IDs like "claude-sonnet-4" match "claude-sonnet-4-20250514" etc.
// Entries are ordered most-specific first to ensure correct substring matching.
var modelPricing = map[string]ModelPricing{
	// Anthropic
	"claude-opus-4":     {15.0, 75.0, 1.5},
	"claude-sonnet-4":   {3.0, 15.0, 0.3},
	"claude-haiku":      {0.25, 1.25, 0.025},
	// OpenAI
	"gpt-4.1-mini":      {0.4, 1.6, 0.1},
	"gpt-4.1-nano":      {0.1, 0.4, 0.025},
	"gpt-4.1":           {2.0, 8.0, 0.5},
	"o3":                {2.0, 8.0, 0.5},
	// Groq / open-source
	"llama-3.3-70b":     {0.59, 0.79, 0},
	// Google
	"gemini-2.5-flash":  {0.15, 0.6, 0.0375},
	"gemini-2.5-pro":    {1.25, 10.0, 0},
	"gemini-2.0-flash":  {0.1, 0.4, 0},
	// DeepSeek
	"deepseek-chat":     {0.14, 0.28, 0.014},
	// Mistral
	"mistral-large":     {2.0, 6.0, 0},
	// xAI Grok
	"grok-3":            {3.0, 15.0, 0},
}

// CostEntry is a single persisted record of one model API call's cost.
type CostEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	InputTok  int       `json:"input_tokens"`
	OutputTok int       `json:"output_tokens"`
	CachedTok int       `json:"cached_tokens"`
	Cost      float64   `json:"cost_usd"`
	SessionID string    `json:"session_id"`
}

// DayCost holds total spend for a calendar day.
type DayCost struct {
	Date string  `json:"date"` // "2006-01-02"
	Cost float64 `json:"cost_usd"`
}

// CostSummary is a snapshot suitable for the /usage command output.
type CostSummary struct {
	SessionCost   float64            `json:"session_cost_usd"`
	TodayCost     float64            `json:"today_cost_usd"`
	TotalCost     float64            `json:"total_cost_usd"`
	ByProvider    map[string]float64 `json:"by_provider"`
	ByModel       map[string]float64 `json:"by_model"`
	Last7Days     []DayCost          `json:"last_7_days"`
	SessionCalls  int                `json:"session_calls"`
	TotalCalls    int                `json:"total_calls"`
}

// CostTracker records and persists API cost entries.
// All methods are safe for concurrent use from the agentic loop.
type CostTracker struct {
	mu        sync.Mutex
	wg        sync.WaitGroup
	entries   []CostEntry
	path      string  // absolute path to .soulgate/costs.jsonl
	totalUSD  float64 // cached sum across all entries
	sessionID string  // the current session being tracked
}

// NewCostTracker creates a CostTracker that persists to configDir/costs.jsonl.
// If the file already exists its historical entries are loaded into memory so
// TodayCost and TotalCost reflect accumulated history across sessions.
func NewCostTracker(configDir string, sessionID string) *CostTracker {
	path := configDir + "/costs.jsonl"
	ct := &CostTracker{
		path:      path,
		sessionID: sessionID,
	}
	ct.load() // best-effort; errors are silently ignored
	return ct
}

// load reads all existing JSONL entries from disk into memory.
// Called once at construction; not concurrent-safe on its own (called before exposure).
func (ct *CostTracker) load() {
	f, err := os.Open(ct.path)
	if err != nil {
		return // file may not exist yet; that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e CostEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		ct.entries = append(ct.entries, e)
		ct.totalUSD += e.Cost
	}
}

// Record calculates the dollar cost for a single model call and appends it
// both in memory and to the costs.jsonl file on disk.
// inputTok, outputTok, cachedTok come directly from the provider's usage response.
func (ct *CostTracker) Record(provider, modelID, sessionID string, inputTok, outputTok, cachedTok int) {
	pricing, ok := lookupPricing(modelID)

	var cost float64
	if ok {
		// Cached tokens replace a fraction of billed input tokens (Anthropic model).
		// For providers without caching (cachedTok == 0) the formula degenerates to
		// normal input billing.
		billableInput := inputTok - cachedTok
		if billableInput < 0 {
			billableInput = 0
		}
		cost = float64(billableInput)*pricing.InputPerMillion/1_000_000 +
			float64(cachedTok)*pricing.CachedPerMillion/1_000_000 +
			float64(outputTok)*pricing.OutputPerMillion/1_000_000
	}
	// When the model is unknown we still record the entry with cost=0 so
	// token counts are preserved for future pricing lookups.

	entry := CostEntry{
		Timestamp: time.Now().UTC(),
		Provider:  provider,
		Model:     modelID,
		InputTok:  inputTok,
		OutputTok: outputTok,
		CachedTok: cachedTok,
		Cost:      cost,
		SessionID: sessionID,
	}

	ct.mu.Lock()
	ct.entries = append(ct.entries, entry)
	ct.totalUSD += cost
	ct.mu.Unlock()

	// Persist asynchronously so the hot path (agentic loop) is not blocked by I/O.
	ct.wg.Add(1)
	go func() {
		defer ct.wg.Done()
		ct.appendLine(entry)
	}()
}

// Flush waits for all pending background writes to complete.
func (ct *CostTracker) Flush() {
	ct.wg.Wait()
}

// appendLine serialises a single CostEntry and appends it to the JSONL file.
func (ct *CostTracker) appendLine(entry CostEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// Open with O_APPEND | O_CREATE so concurrent writers from different sessions
	// are safe at the OS level (each write is atomic for small payloads on POSIX).
	f, err := os.OpenFile(ct.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(data, '\n'))
}

// lookupPricing finds the pricing entry for modelID by substring match.
// The table is scanned in iteration order; more-specific keys (longer) are
// placed before shorter aliases in the literal map, which in Go has random
// iteration order. We therefore sort by key length descending to ensure
// "gpt-4.1-mini" is tested before "gpt-4.1".
func lookupPricing(modelID string) (ModelPricing, bool) {
	lower := strings.ToLower(modelID)

	// Build a slice sorted by key length descending for deterministic matching.
	type kv struct {
		key string
		val ModelPricing
	}
	sorted := make([]kv, 0, len(modelPricing))
	for k, v := range modelPricing {
		sorted = append(sorted, kv{k, v})
	}
	// Simple insertion sort (table is tiny).
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j].key) > len(sorted[j-1].key); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	for _, kv := range sorted {
		if strings.Contains(lower, kv.key) {
			return kv.val, true
		}
	}
	return ModelPricing{}, false
}

// SessionCost returns the total spend attributed to a particular session ID.
func (ct *CostTracker) SessionCost(sessionID string) float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	var total float64
	for _, e := range ct.entries {
		if e.SessionID == sessionID {
			total += e.Cost
		}
	}
	return total
}

// CurrentSessionCost is a convenience wrapper for the tracker's own session.
func (ct *CostTracker) CurrentSessionCost() float64 {
	return ct.SessionCost(ct.sessionID)
}

// TodayCost returns the total spend for the current UTC calendar day.
func (ct *CostTracker) TodayCost() float64 {
	today := time.Now().UTC().Format("2006-01-02")
	ct.mu.Lock()
	defer ct.mu.Unlock()
	var total float64
	for _, e := range ct.entries {
		if e.Timestamp.UTC().Format("2006-01-02") == today {
			total += e.Cost
		}
	}
	return total
}

// TotalCost returns the accumulated cost across all recorded entries.
func (ct *CostTracker) TotalCost() float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.totalUSD
}

// CostByProvider returns a map from provider name to cumulative spend.
func (ct *CostTracker) CostByProvider() map[string]float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	m := make(map[string]float64)
	for _, e := range ct.entries {
		m[e.Provider] += e.Cost
	}
	return m
}

// CostByModel returns a map from model name to cumulative spend.
func (ct *CostTracker) CostByModel() map[string]float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	m := make(map[string]float64)
	for _, e := range ct.entries {
		model := e.Model
		if model == "" {
			model = "unknown"
		}
		m[model] += e.Cost
	}
	return m
}

// CostByDay returns the last N calendar days (UTC) with their totals,
// oldest first. Days with zero spend are omitted.
func (ct *CostTracker) CostByDay(days int) []DayCost {
	if days <= 0 {
		days = 7
	}

	now := time.Now().UTC()
	// Build a lookup of date → spend.
	ct.mu.Lock()
	dayMap := make(map[string]float64, days)
	for _, e := range ct.entries {
		d := e.Timestamp.UTC().Format("2006-01-02")
		dayMap[d] += e.Cost
	}
	ct.mu.Unlock()

	result := make([]DayCost, 0, days)
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if cost, ok := dayMap[day]; ok {
			result = append(result, DayCost{Date: day, Cost: cost})
		}
	}
	return result
}

// Summary returns a complete cost snapshot for display.
func (ct *CostTracker) Summary() CostSummary {
	ct.mu.Lock()
	sessionCalls := 0
	for _, e := range ct.entries {
		if e.SessionID == ct.sessionID {
			sessionCalls++
		}
	}
	totalCalls := len(ct.entries)
	totalUSD := ct.totalUSD
	ct.mu.Unlock()

	return CostSummary{
		SessionCost:  ct.CurrentSessionCost(),
		TodayCost:    ct.TodayCost(),
		TotalCost:    totalUSD,
		ByProvider:   ct.CostByProvider(),
		ByModel:      ct.CostByModel(),
		Last7Days:    ct.CostByDay(7),
		SessionCalls: sessionCalls,
		TotalCalls:   totalCalls,
	}
}

// FormatCost returns a compact dollar string, e.g. "$0.0042" or "$1.23".
func FormatCost(usd float64) string {
	if usd == 0 {
		return "$0.00"
	}
	if usd < 0.1 {
		return fmt.Sprintf("$%.4f", usd)
	}
	return fmt.Sprintf("$%.2f", usd)
}
