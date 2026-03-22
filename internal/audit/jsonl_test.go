package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// todayFile returns the date-stamped filename that NewJSONLLogger will create
// for the given base path (e.g., "/tmp/x/audit.jsonl" → "/tmp/x/audit-2026-03-21.jsonl").
func todayFile(basePath string) string {
	dir := filepath.Dir(basePath)
	base := filepath.Base(basePath)
	ext := filepath.Ext(base)
	baseName := base[:len(base)-len(ext)]
	today := time.Now().UTC().Format("2006-01-02")
	return filepath.Join(dir, baseName+"-"+today+".jsonl")
}

func TestNewJSONLLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	// The constructor opens today's date-stamped file, not the base path.
	assert.FileExists(t, todayFile(path))
}

func TestNewJSONLLoggerCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	assert.FileExists(t, todayFile(path))
}

func TestJSONLLoggerLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	event := NewEvent(EventRunStart, CategoryRun).
		WithSessionID("sess-1").
		WithRunID("run-1")

	err = logger.Log(context.Background(), event)
	require.NoError(t, err)

	// Content should be in today's date-stamped file.
	data, err := os.ReadFile(todayFile(path))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"type":"run.start"`)
}

func TestJSONLLoggerLogNilEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	err = logger.Log(context.Background(), nil)
	assert.Error(t, err)
}

func TestJSONLLoggerQuery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	// Log multiple events
	for i := 0; i < 5; i++ {
		event := NewEvent(EventRunStart, CategoryRun).
			WithSessionID("sess-1").
			WithRunID(fmt.Sprintf("run-%d", i))
		require.NoError(t, logger.Log(context.Background(), event))
	}

	events, err := logger.Query(context.Background(), DefaultQueryFilter())
	require.NoError(t, err)
	assert.Len(t, events, 5)

	// Should be newest first
	assert.Equal(t, "run-4", events[0].RunID)
	assert.Equal(t, "run-0", events[4].RunID)
}

func TestJSONLLoggerQueryBySessionID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun).WithSessionID("sess-1"))
	logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun).WithSessionID("sess-2"))
	logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun).WithSessionID("sess-1"))

	events, err := logger.Query(context.Background(), QueryFilter{SessionID: "sess-1", Limit: 100})
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestJSONLLoggerQueryByType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun))
	logger.Log(context.Background(), NewEvent(EventToolExecute, CategoryTool))
	logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun))

	events, err := logger.Query(context.Background(), QueryFilter{Type: EventToolExecute, Limit: 100})
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestJSONLLoggerQueryByTimeRange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun))

	events, err := logger.Query(context.Background(), QueryFilter{StartTime: &past, EndTime: &future, Limit: 100})
	require.NoError(t, err)
	assert.Len(t, events, 1)

	// Query with time range that excludes the event
	wayPast := now.Add(-2 * time.Hour)
	events, err = logger.Query(context.Background(), QueryFilter{StartTime: &wayPast, EndTime: &past, Limit: 100})
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestJSONLLoggerQueryWithLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	for i := 0; i < 10; i++ {
		logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun))
	}

	events, err := logger.Query(context.Background(), QueryFilter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, events, 3)
}

func TestJSONLLoggerQueryWithOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	for i := 0; i < 5; i++ {
		logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun).WithRunID(fmt.Sprintf("run-%d", i)))
	}

	events, err := logger.Query(context.Background(), QueryFilter{Offset: 2, Limit: 100})
	require.NoError(t, err)
	assert.Len(t, events, 3)
}

func TestJSONLLoggerLogWithMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	event := NewEvent(EventFileRead, CategoryBroker).
		WithMetadata("path", "/test/file.txt").
		WithMetadata("size", 1024)
	require.NoError(t, logger.Log(context.Background(), event))

	events, err := logger.Query(context.Background(), DefaultQueryFilter())
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "/test/file.txt", events[0].Metadata["path"])
}

func TestJSONLLoggerConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			event := NewEvent(EventRunStart, CategoryRun).
				WithRunID(fmt.Sprintf("run-%d", n))
			logger.Log(context.Background(), event)
		}(i)
	}
	wg.Wait()

	events, err := logger.Query(context.Background(), QueryFilter{Limit: 200})
	require.NoError(t, err)
	assert.Len(t, events, 50)
}

func TestJSONLLoggerClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)

	err = logger.Close()
	assert.NoError(t, err)

	// Writing after close should fail
	err = logger.Log(context.Background(), NewEvent(EventRunStart, CategoryRun))
	assert.Error(t, err)
}

// TestJSONLLoggerRotation verifies that when the logger's fileDate drifts
// behind the current date, Log() detects the mismatch, closes the old file,
// opens a new date-stamped file, and writes the event there.
func TestJSONLLoggerRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")

	// Write a "today" event normally.
	eventToday := NewEvent(EventRunStart, CategoryRun).WithRunID("run-today")
	require.NoError(t, logger.Log(context.Background(), eventToday))

	// Simulate the logger having been open since "yesterday" by directly
	// writing an event into a yesterday-dated file and telling the logger
	// its fileDate is yesterday. The next Log() call will detect the date
	// mismatch and rotate to today's file.
	//
	// We write the "yesterday" event directly to the file to avoid Log()'s
	// auto-rotation interfering with the setup.
	yesterdayFilePath := logger.filenameForDate(yesterday)
	yesterdayEvent := &Event{
		ID:        generateID(),
		Timestamp: time.Now().UTC().AddDate(0, 0, -1),
		Type:      EventRunStart,
		Category:  CategoryRun,
		Status:    StatusSuccess,
		RunID:     "run-yesterday",
		Metadata:  map[string]interface{}{},
	}
	data, err := json.Marshal(yesterdayEvent)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(yesterdayFilePath, append(data, '\n'), 0o600))

	// Both files must exist.
	assert.FileExists(t, logger.filenameForDate(yesterday))
	assert.FileExists(t, logger.filenameForDate(today))

	// The today file must contain run-today but not run-yesterday.
	todayData, err := os.ReadFile(logger.filenameForDate(today))
	require.NoError(t, err)
	assert.Contains(t, string(todayData), "run-today")
	assert.NotContains(t, string(todayData), "run-yesterday")

	// The yesterday file must contain run-yesterday but not run-today.
	yesterdayData, err := os.ReadFile(yesterdayFilePath)
	require.NoError(t, err)
	assert.Contains(t, string(yesterdayData), "run-yesterday")
	assert.NotContains(t, string(yesterdayData), "run-today")

	// Query should return both events.
	events, err := logger.Query(context.Background(), QueryFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, events, 2)

	runIDs := map[string]bool{}
	for _, e := range events {
		runIDs[e.RunID] = true
	}
	assert.True(t, runIDs["run-today"])
	assert.True(t, runIDs["run-yesterday"])
}

// TestJSONLLoggerQueryAcrossFiles verifies that Query() aggregates events from
// multiple date-stamped files and returns them newest-first.
func TestJSONLLoggerQueryAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	twoDaysAgo := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")

	// Manually write events into date-stamped files for past dates.
	writeEventToFile := func(t *testing.T, filePath string, runID string, ts time.Time) {
		t.Helper()
		ev := &Event{
			ID:        generateID(),
			Timestamp: ts,
			Type:      EventRunStart,
			Category:  CategoryRun,
			Status:    StatusSuccess,
			RunID:     runID,
			Metadata:  map[string]interface{}{},
		}
		data, err := json.Marshal(ev)
		require.NoError(t, err)
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		require.NoError(t, err)
		defer f.Close()
		_, err = f.Write(append(data, '\n'))
		require.NoError(t, err)
	}

	twoDaysAgoTime := time.Now().UTC().AddDate(0, 0, -2)
	yesterdayTime := time.Now().UTC().AddDate(0, 0, -1)
	nowTime := time.Now().UTC()

	writeEventToFile(t, logger.filenameForDate(twoDaysAgo), "run-two-days-ago", twoDaysAgoTime)
	writeEventToFile(t, logger.filenameForDate(yesterday), "run-yesterday", yesterdayTime)

	// Write today's event through the logger itself.
	ev := NewEvent(EventRunStart, CategoryRun).WithRunID("run-today")
	ev.Timestamp = nowTime
	require.NoError(t, logger.Log(context.Background(), ev))

	// Close and reopen so the today file is fully flushed and readable.
	require.NoError(t, logger.Close())
	logger, err = NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	// Force logger to know about today's file.
	logger.mu.Lock()
	require.NoError(t, logger.rotate(today))
	logger.mu.Unlock()

	events, err := logger.Query(context.Background(), QueryFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, events, 3)

	// Newest first.
	assert.Equal(t, "run-today", events[0].RunID)
	assert.Equal(t, "run-yesterday", events[1].RunID)
	assert.Equal(t, "run-two-days-ago", events[2].RunID)
}

// TestJSONLLoggerLegacyFileCompat verifies that a pre-existing undated
// audit.jsonl file is included in query results.
func TestJSONLLoggerLegacyFileCompat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	// Create a legacy file with one event before the logger is initialised.
	legacyEvent := &Event{
		ID:        generateID(),
		Timestamp: time.Now().UTC().AddDate(0, 0, -5),
		Type:      EventRunStart,
		Category:  CategoryRun,
		Status:    StatusSuccess,
		RunID:     "run-legacy",
		Metadata:  map[string]interface{}{},
	}
	data, err := json.Marshal(legacyEvent)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(data, '\n'), 0o600))

	// Now create the logger. It must NOT overwrite the legacy file.
	logger, err := NewJSONLLogger(path)
	require.NoError(t, err)
	defer logger.Close()

	// Write a new event through the logger (goes to the date-stamped file).
	newEvent := NewEvent(EventRunStart, CategoryRun).WithRunID("run-new")
	require.NoError(t, logger.Log(context.Background(), newEvent))

	// Query should return both the legacy and the new event.
	events, err := logger.Query(context.Background(), QueryFilter{Limit: 100})
	require.NoError(t, err)
	require.Len(t, events, 2)

	runIDs := map[string]bool{}
	for _, e := range events {
		runIDs[e.RunID] = true
	}
	assert.True(t, runIDs["run-legacy"], "legacy event should be included in query results")
	assert.True(t, runIDs["run-new"], "new event should be included in query results")

	// The legacy file must still exist and be unmodified.
	assert.FileExists(t, path)
	legacyData, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(legacyData), "run-legacy")
	assert.NotContains(t, string(legacyData), "run-new")
}
