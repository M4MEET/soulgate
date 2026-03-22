package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// JSONLLogger implements Logger using append-only JSONL files with date-based rotation.
// Each line is a self-contained JSON object representing one audit event.
// Active log files are named <baseName>-YYYY-MM-DD.jsonl. A legacy file named
// <baseName>.jsonl (without a date suffix) is included in queries for backward
// compatibility but is never written to.
type JSONLLogger struct {
	dir      string     // directory containing log files
	baseName string     // e.g., "audit" (without extension)
	file     *os.File   // current day's open file handle (nil after Close)
	fileDate string     // "YYYY-MM-DD" of the currently open file
	mu       sync.Mutex // protects file, fileDate
}

// NewJSONLLogger creates a new JSONL audit logger rooted at path.
// path is the base path used to derive the directory and date-stamped filenames,
// e.g., "/path/to/audit.jsonl" → dir="/path/to/", baseName="audit".
// Today's date file is opened immediately.
func NewJSONLLogger(path string) (*JSONLLogger, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Strip the extension to get the base name ("audit.jsonl" → "audit").
	ext := filepath.Ext(base)
	baseName := strings.TrimSuffix(base, ext)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	l := &JSONLLogger{
		dir:      dir,
		baseName: baseName,
	}

	today := time.Now().UTC().Format("2006-01-02")
	if err := l.rotate(today); err != nil {
		return nil, err
	}

	return l, nil
}

// filenameForDate returns the full path of the log file for a given date string.
func (l *JSONLLogger) filenameForDate(date string) string {
	return filepath.Join(l.dir, l.baseName+"-"+date+".jsonl")
}

// legacyPath returns the path of the legacy (undated) log file.
func (l *JSONLLogger) legacyPath() string {
	return filepath.Join(l.dir, l.baseName+".jsonl")
}

// rotate closes the current file (if any) and opens the file for date.
// Must be called with l.mu held.
func (l *JSONLLogger) rotate(date string) error {
	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close previous audit log: %w", err)
		}
		l.file = nil
	}

	name := l.filenameForDate(date)
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}

	l.file = f
	l.fileDate = date
	return nil
}

// Log appends an audit event as a single JSON line.
// If the date has changed since the last write, the logger rotates to a new file.
func (l *JSONLLogger) Log(_ context.Context, event *Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	today := time.Now().UTC().Format("2006-01-02")

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("logger is closed")
	}

	if l.fileDate != today {
		if err := l.rotate(today); err != nil {
			return err
		}
	}

	if _, err := l.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// Query reads events from all date-stamped log files that fall within the filter's
// date range. If no date range is specified, all audit-*.jsonl files are scanned.
// The legacy undated file (if present) is always included.
// Events are returned in reverse-chronological order (newest first).
func (l *JSONLLogger) Query(_ context.Context, filter QueryFilter) ([]*Event, error) {
	l.mu.Lock()
	files, err := l.filesForFilter(&filter)
	l.mu.Unlock()
	if err != nil {
		return nil, err
	}

	var all []*Event
	for _, name := range files {
		events, err := l.readFile(name, &filter)
		if err != nil {
			// Skip files that cannot be read (e.g., already rotated away).
			continue
		}
		all = append(all, events...)
	}

	// Sort all collected events by timestamp ascending so we can reverse below.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.Before(all[j].Timestamp)
	})

	// Reverse to newest-first.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	// Apply offset.
	if filter.Offset > 0 {
		if filter.Offset >= len(all) {
			return nil, nil
		}
		all = all[filter.Offset:]
	}

	// Apply limit.
	if filter.Limit > 0 && filter.Limit < len(all) {
		all = all[:filter.Limit]
	}

	return all, nil
}

// filesForFilter returns the list of log file paths that should be scanned for
// the given filter. Must be called with l.mu held (reads l.dir, l.baseName).
func (l *JSONLLogger) filesForFilter(filter *QueryFilter) ([]string, error) {
	pattern := filepath.Join(l.dir, l.baseName+"-*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list audit log files: %w", err)
	}

	prefix := l.baseName + "-"
	suffix := ".jsonl"

	var files []string
	for _, m := range matches {
		base := filepath.Base(m)
		// Extract the date portion from the filename.
		if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, suffix) {
			continue
		}
		dateStr := base[len(prefix) : len(base)-len(suffix)]

		// If a date range filter is set, skip files whose dates are outside it.
		if filter.StartTime != nil || filter.EndTime != nil {
			fileDate, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
			if err != nil {
				// Cannot parse date; include the file to be safe.
				files = append(files, m)
				continue
			}
			// The file covers the whole day [fileDate, fileDate+24h).
			fileDayEnd := fileDate.Add(24 * time.Hour)

			if filter.EndTime != nil && !fileDate.Before(*filter.EndTime) {
				continue // file starts on or after EndTime
			}
			if filter.StartTime != nil && !fileDayEnd.After(*filter.StartTime) {
				continue // file ends on or before StartTime
			}
		}

		files = append(files, m)
	}

	// Always include the legacy undated file if it exists.
	legacy := l.legacyPath()
	if _, err := os.Stat(legacy); err == nil {
		files = append(files, legacy)
	}

	return files, nil
}

// readFile opens a single log file and returns all events matching the filter.
func (l *JSONLLogger) readFile(name string, filter *QueryFilter) ([]*Event, error) {
	f, err := os.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open %s: %w", name, err)
	}
	defer f.Close()

	var events []*Event
	scanner := bufio.NewScanner(f)
	// Allow lines up to 1 MB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			// Skip malformed lines rather than failing the whole query.
			continue
		}

		if !matchesFilter(&ev, filter) {
			continue
		}

		events = append(events, &ev)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", name, err)
	}

	return events, nil
}

// Close flushes and closes the underlying file.
func (l *JSONLLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

// CurrentFilePath returns the path of the currently active log file.
// Exposed for testing and diagnostics.
func (l *JSONLLogger) CurrentFilePath() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.filenameForDate(l.fileDate)
}

// matchesFilter checks whether an event matches the given query filter.
func matchesFilter(ev *Event, f *QueryFilter) bool {
	if f.SessionID != "" && ev.SessionID != f.SessionID {
		return false
	}
	if f.RunID != "" && ev.RunID != f.RunID {
		return false
	}
	if f.PluginID != "" && ev.PluginID != f.PluginID {
		return false
	}
	if f.Type != "" && ev.Type != f.Type {
		return false
	}
	if f.Category != "" && ev.Category != f.Category {
		return false
	}
	if f.Status != "" && ev.Status != f.Status {
		return false
	}
	if f.StartTime != nil && ev.Timestamp.Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && ev.Timestamp.After(*f.EndTime) {
		return false
	}
	return true
}
