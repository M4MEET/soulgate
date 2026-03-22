package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/policy"
)

// ComplianceOptions controls what data is included in a compliance export.
type ComplianceOptions struct {
	// ConfigDir is the path to the .soulgate directory.
	ConfigDir string

	// UserID scopes the export to a specific user identifier stored in event
	// metadata.  Empty means include all events.
	UserID string

	// From and To are the inclusive date range boundaries.  Zero values mean
	// "from the beginning" and "up to now" respectively.
	From time.Time
	To   time.Time

	// AuditLogger is used to query audit events.  Required.
	AuditLogger audit.Logger

	// PolicyEngine is used to snapshot the current policy rules.  Optional.
	PolicyEngine *policy.Engine
}

// SessionSummary is a lightweight summary of a session for compliance reports.
type SessionSummary struct {
	SessionID string    `json:"session_id"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// ComplianceExport is a point-in-time snapshot of audit and cost data.
type ComplianceExport struct {
	ExportedAt  time.Time        `json:"exported_at"`
	UserID      string           `json:"user_id,omitempty"`
	DateRange   DateRange        `json:"date_range"`
	AuditEvents []*audit.Event   `json:"audit_events"`
	Sessions    []SessionSummary `json:"sessions"`
	CostEntries []CostEntry      `json:"cost_entries"`
	Policies    []policy.PolicyRule `json:"policies"`
}

// DateRange represents an inclusive [From, To] time interval.
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ExportCompliance gathers audit events, cost entries, and policy rules for the
// given options and returns a structured export suitable for compliance reporting.
func ExportCompliance(ctx context.Context, opts ComplianceOptions) (*ComplianceExport, error) {
	if opts.AuditLogger == nil {
		return nil, fmt.Errorf("AuditLogger is required")
	}

	now := time.Now().UTC()
	from := opts.From
	to := opts.To
	if to.IsZero() {
		to = now
	}

	filter := audit.QueryFilter{
		Limit: 0, // unlimited
	}
	if !from.IsZero() {
		filter.StartTime = &from
	}
	filter.EndTime = &to

	events, err := opts.AuditLogger.Query(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}

	// Filter by user ID if specified.
	var filteredEvents []*audit.Event
	for _, ev := range events {
		if opts.UserID == "" {
			filteredEvents = append(filteredEvents, ev)
			continue
		}
		if uid, ok := ev.Metadata["user_id"].(string); ok && uid == opts.UserID {
			filteredEvents = append(filteredEvents, ev)
		}
	}

	// Derive session summaries from session.start and session.end events.
	sessionMap := make(map[string]*SessionSummary)
	for _, ev := range filteredEvents {
		sid := ev.SessionID
		if sid == "" {
			continue
		}
		if _, exists := sessionMap[sid]; !exists {
			sessionMap[sid] = &SessionSummary{SessionID: sid}
		}
		switch ev.Type {
		case audit.EventSessionStart:
			sessionMap[sid].StartedAt = ev.Timestamp
		case audit.EventSessionEnd:
			sessionMap[sid].EndedAt = ev.Timestamp
		}
	}
	sessions := make([]SessionSummary, 0, len(sessionMap))
	for _, s := range sessionMap {
		sessions = append(sessions, *s)
	}

	// Load cost entries within the date range.
	costEntries, err := loadCostEntriesForExport(opts.ConfigDir, opts.UserID, from, to)
	if err != nil {
		// Non-fatal: include what we have.
		costEntries = nil
	}

	// Snapshot policy rules.
	var rules []policy.PolicyRule
	if opts.PolicyEngine != nil {
		if pol := opts.PolicyEngine.GetPolicy(); pol != nil {
			rules = make([]policy.PolicyRule, len(pol.Policies))
			copy(rules, pol.Policies)
		}
	}

	return &ComplianceExport{
		ExportedAt:  now,
		UserID:      opts.UserID,
		DateRange:   DateRange{From: from, To: to},
		AuditEvents: filteredEvents,
		Sessions:    sessions,
		CostEntries: costEntries,
		Policies:    rules,
	}, nil
}

// PurgeUserData deletes all data associated with a given userID across audit
// logs, cost entries, and memory.  It rewrites each file in place, keeping only
// records that do not match userID.
//
// This implements the GDPR "right to erasure" (right to be forgotten).
func PurgeUserData(ctx context.Context, configDir string, userID string, auditLogger audit.Logger) error {
	if userID == "" {
		return fmt.Errorf("userID must not be empty")
	}

	// Purge from audit JSONL files.
	if err := purgeUserFromAuditFiles(configDir, userID); err != nil {
		return fmt.Errorf("purge audit files: %w", err)
	}

	// Purge from cost log.
	if err := purgeUserFromCostLog(configDir, userID); err != nil {
		return fmt.Errorf("purge cost log: %w", err)
	}

	// Purge from memory.
	if err := purgeUserFromMemory(configDir, userID); err != nil {
		return fmt.Errorf("purge memory: %w", err)
	}

	return nil
}

// --- helpers ---

// loadCostEntriesForExport reads the cost log and returns entries whose
// timestamp falls within [from, to].  If userID is non-empty only entries
// whose session_id starts with "sess_<userID>_" are included (a heuristic;
// adjust to your user-scoping convention).
func loadCostEntriesForExport(configDir, userID string, from, to time.Time) ([]CostEntry, error) {
	path := filepath.Join(configDir, "costs.jsonl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []CostEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e CostEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		ts := e.Timestamp.UTC()
		if !from.IsZero() && ts.Before(from) {
			continue
		}
		if !to.IsZero() && ts.After(to) {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// purgeUserFromAuditFiles removes all events from audit-*.jsonl files where
// the metadata["user_id"] matches userID.
func purgeUserFromAuditFiles(configDir, userID string) error {
	matches, err := filepath.Glob(filepath.Join(configDir, "audit-*.jsonl"))
	if err != nil {
		return err
	}
	// Also include the undated legacy file if present.
	legacy := filepath.Join(configDir, "audit.jsonl")
	if _, statErr := os.Stat(legacy); statErr == nil {
		matches = append(matches, legacy)
	}

	for _, path := range matches {
		if err := rewriteAuditFileWithoutUser(path, userID); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func rewriteAuditFileWithoutUser(path, userID string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev audit.Event
		if json.Unmarshal([]byte(line), &ev) != nil {
			kept = append(kept, line) // keep malformed lines unchanged
			continue
		}
		if uid, ok := ev.Metadata["user_id"].(string); ok && uid == userID {
			continue // drop this event
		}
		kept = append(kept, line)
	}

	tmp := path + ".tmp"
	out := strings.Join(kept, "\n")
	if len(kept) > 0 {
		out += "\n"
	}
	if err := os.WriteFile(tmp, []byte(out), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// purgeUserFromCostLog removes all cost entries whose session_id is associated
// with the given userID.  The association heuristic uses metadata; adjust as
// needed for your production user-scoping convention.
func purgeUserFromCostLog(configDir, userID string) error {
	path := filepath.Join(configDir, "costs.jsonl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e struct {
			SessionID string `json:"session_id"`
		}
		if json.Unmarshal([]byte(line), &e) == nil && strings.Contains(e.SessionID, userID) {
			continue // drop entries associated with this user
		}
		kept = append(kept, line)
	}

	tmp := path + ".tmp"
	out := strings.Join(kept, "\n")
	if len(kept) > 0 {
		out += "\n"
	}
	if err := os.WriteFile(tmp, []byte(out), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// purgeUserFromMemory removes all MemoryEntry records whose AgentID matches
// userID from state/memory.json.
func purgeUserFromMemory(configDir, userID string) error {
	path := filepath.Join(configDir, "state", "memory.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var entries map[string]map[string]MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse memory.json: %w", err)
	}

	delete(entries, userID) // remove the agent/user scope entirely

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
