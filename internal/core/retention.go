package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RetentionPolicy defines how long each category of data is kept before being
// purged.  A zero value for a Days field means "keep forever".
type RetentionPolicy struct {
	AuditLogDays int  `yaml:"audit_log_days" json:"audit_log_days"` // 0 = forever
	SessionDays  int  `yaml:"session_days"   json:"session_days"`
	CostLogDays  int  `yaml:"cost_log_days"  json:"cost_log_days"`
	MemoryDays   int  `yaml:"memory_days"    json:"memory_days"`
	AutoPurge    bool `yaml:"auto_purge"     json:"auto_purge"`
}

// RetentionResult reports what was deleted by a single retention run.
type RetentionResult struct {
	AuditFilesDeleted   int
	SessionsDeleted     int
	CostEntriesPurged   int
	MemoryEntriesPurged int
	BytesFreed          int64
}

// RunRetention executes the retention policy against the given config directory
// and returns a summary of what was removed.
//
// configDir is the path to the .soulgate directory (e.g. "/workspace/.soulgate").
// The function is deliberately conservative: it only removes files/entries that
// are strictly older than the configured cut-off, never the current day's data.
func RunRetention(configDir string, policy RetentionPolicy) (RetentionResult, error) {
	var result RetentionResult
	now := time.Now().UTC()

	// --- Audit log files ---
	if policy.AuditLogDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.AuditLogDays)
		n, freed, err := purgeAuditFiles(configDir, cutoff)
		if err != nil {
			return result, fmt.Errorf("purge audit logs: %w", err)
		}
		result.AuditFilesDeleted = n
		result.BytesFreed += freed
	}

	// --- Cost log entries ---
	if policy.CostLogDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.CostLogDays)
		n, freed, err := purgeCostEntries(configDir, cutoff)
		if err != nil {
			return result, fmt.Errorf("purge cost log: %w", err)
		}
		result.CostEntriesPurged = n
		result.BytesFreed += freed
	}

	// --- Memory entries ---
	if policy.MemoryDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.MemoryDays)
		n, err := purgeMemoryEntries(configDir, cutoff)
		if err != nil {
			return result, fmt.Errorf("purge memory: %w", err)
		}
		result.MemoryEntriesPurged = n
	}

	return result, nil
}

// RetentionStatus reports what would be removed without actually removing anything.
func RetentionStatus(configDir string, policy RetentionPolicy) (RetentionResult, error) {
	var result RetentionResult
	now := time.Now().UTC()

	if policy.AuditLogDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.AuditLogDays)
		n, freed, err := auditFileStats(configDir, cutoff)
		if err != nil {
			return result, fmt.Errorf("audit log status: %w", err)
		}
		result.AuditFilesDeleted = n
		result.BytesFreed += freed
	}

	if policy.CostLogDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.CostLogDays)
		n, freed, err := costEntryStats(configDir, cutoff)
		if err != nil {
			return result, fmt.Errorf("cost log status: %w", err)
		}
		result.CostEntriesPurged = n
		result.BytesFreed += freed
	}

	if policy.MemoryDays > 0 {
		cutoff := now.AddDate(0, 0, -policy.MemoryDays)
		n, err := memoryEntryStats(configDir, cutoff)
		if err != nil {
			return result, fmt.Errorf("memory status: %w", err)
		}
		result.MemoryEntriesPurged = n
	}

	return result, nil
}

// --- Audit log helpers ---

// auditDateFromFilename extracts the date from an audit log filename of the form
// "audit-YYYY-MM-DD.jsonl".  Returns zero time if the filename does not match.
func auditDateFromFilename(base string) time.Time {
	const prefix = "audit-"
	const suffix = ".jsonl"
	if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, suffix) {
		return time.Time{}
	}
	dateStr := base[len(prefix) : len(base)-len(suffix)]
	t, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return t
}

// auditFileStats counts files older than cutoff without removing them.
func auditFileStats(configDir string, cutoff time.Time) (count int, bytes int64, err error) {
	matches, err := filepath.Glob(filepath.Join(configDir, "audit-*.jsonl"))
	if err != nil {
		return 0, 0, err
	}
	for _, path := range matches {
		fileDate := auditDateFromFilename(filepath.Base(path))
		if fileDate.IsZero() {
			continue
		}
		// A whole-day file: it is "older than cutoff" only when the entire day
		// has passed the cutoff (i.e. fileDate + 24h <= cutoff).
		if !fileDate.Add(24 * time.Hour).After(cutoff) {
			fi, statErr := os.Stat(path)
			if statErr == nil {
				bytes += fi.Size()
			}
			count++
		}
	}
	return count, bytes, nil
}

// purgeAuditFiles deletes audit-YYYY-MM-DD.jsonl files whose entire day is
// before the cutoff.
func purgeAuditFiles(configDir string, cutoff time.Time) (count int, freed int64, err error) {
	matches, err := filepath.Glob(filepath.Join(configDir, "audit-*.jsonl"))
	if err != nil {
		return 0, 0, err
	}
	for _, path := range matches {
		fileDate := auditDateFromFilename(filepath.Base(path))
		if fileDate.IsZero() {
			continue
		}
		if !fileDate.Add(24 * time.Hour).After(cutoff) {
			fi, statErr := os.Stat(path)
			if statErr == nil {
				freed += fi.Size()
			}
			if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
				return count, freed, fmt.Errorf("remove %s: %w", path, rmErr)
			}
			count++
		}
	}
	return count, freed, nil
}

// --- Cost log helpers ---

// costEntryStats counts JSONL cost entries older than cutoff.
func costEntryStats(configDir string, cutoff time.Time) (count int, bytes int64, err error) {
	path := filepath.Join(configDir, "costs.jsonl")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var entry struct {
			Timestamp time.Time `json:"timestamp"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && entry.Timestamp.Before(cutoff) {
			count++
			bytes += int64(len(line) + 1)
		}
	}
	return count, bytes, scanner.Err()
}

// purgeCostEntries rewrites the cost log file keeping only entries whose
// timestamp is at or after cutoff.
func purgeCostEntries(configDir string, cutoff time.Time) (removed int, freed int64, err error) {
	path := filepath.Join(configDir, "costs.jsonl")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}

	var keep []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry struct {
			Timestamp time.Time `json:"timestamp"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || !entry.Timestamp.Before(cutoff) {
			keep = append(keep, line)
		} else {
			removed++
			freed += int64(len(line) + 1)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		f.Close()
		return removed, freed, scanErr
	}
	f.Close()

	if removed == 0 {
		return 0, 0, nil
	}

	// Atomically rewrite the file.
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return removed, freed, fmt.Errorf("create temp file: %w", err)
	}
	w := bufio.NewWriter(out)
	for _, line := range keep {
		if _, wErr := fmt.Fprintln(w, line); wErr != nil {
			out.Close()
			os.Remove(tmp)
			return removed, freed, wErr
		}
	}
	if err := w.Flush(); err != nil {
		out.Close()
		os.Remove(tmp)
		return removed, freed, err
	}
	out.Close()

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return removed, freed, fmt.Errorf("rename temp file: %w", err)
	}
	return removed, freed, nil
}

// --- Memory helpers ---

// memoryEntryStats counts MemoryEntry records older than cutoff.
func memoryEntryStats(configDir string, cutoff time.Time) (int, error) {
	entries, err := loadRawMemory(configDir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, agentEntries := range entries {
		for _, e := range agentEntries {
			if e.CreatedAt.Before(cutoff) {
				count++
			}
		}
	}
	return count, nil
}

// purgeMemoryEntries removes MemoryEntry records older than cutoff from
// memory.json and returns the number of entries removed.
func purgeMemoryEntries(configDir string, cutoff time.Time) (int, error) {
	path := filepath.Join(configDir, "memory.json")
	entries, err := loadRawMemory(configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for agentID, agentEntries := range entries {
		var kept map[string]MemoryEntry = make(map[string]MemoryEntry)
		for key, e := range agentEntries {
			if e.CreatedAt.Before(cutoff) {
				removed++
			} else {
				kept[key] = e
			}
		}
		entries[agentID] = kept
	}

	if removed == 0 {
		return 0, nil
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return removed, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return removed, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return removed, err
	}
	return removed, nil
}

// loadRawMemory reads memory.json into the nested map structure.
func loadRawMemory(configDir string) (map[string]map[string]MemoryEntry, error) {
	path := filepath.Join(configDir, "memory.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries map[string]map[string]MemoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse memory.json: %w", err)
	}
	return entries, nil
}
