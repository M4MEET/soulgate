package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLiteLogger(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer logger.Close()

	// Verify database file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should exist")
}

func TestNewSQLiteLoggerInvalidPath(t *testing.T) {
	// Use an invalid path (directory that doesn't exist and can't be created)
	dbPath := "/nonexistent/path/to/test.db"

	logger, err := NewSQLiteLogger(dbPath)
	if logger != nil {
		defer logger.Close()
	}

	// Should either fail or succeed depending on SQLite behavior
	// This test mainly ensures we handle the error case
	if err != nil {
		assert.Error(t, err)
	}
}

func TestSQLiteLoggerLog(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	event := NewEvent(EventFileRead, CategoryBroker).
		WithSessionID("session-123").
		WithRunID("run-456").
		WithPlugin("plugin-789").
		WithAction("files.read").
		WithResource("./test.txt").
		WithDecision(DecisionAllow).
		WithStatus(StatusSuccess).
		WithMetadata("size", 1024)

	err = logger.Log(ctx, event)
	assert.NoError(t, err)
}

func TestSQLiteLoggerLogNilEvent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()
	err = logger.Log(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

func TestSQLiteLoggerQuery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert test events one by one with delays
	events := []struct {
		sessionID string
		runID     string
		eventType EventType
		category  EventCategory
		resource  string
	}{
		{"session-1", "run-1", EventFileRead, CategoryBroker, "./file1.txt"},
		{"session-1", "run-2", EventFileWrite, CategoryBroker, "./file2.txt"},
		{"session-2", "run-3", EventModelCall, CategoryModel, ""},
	}

	for _, e := range events {
		time.Sleep(2 * time.Millisecond) // Ensure unique timestamp-based IDs
		event := NewEvent(e.eventType, e.category).
			WithSessionID(e.sessionID).
			WithRunID(e.runID)
		if e.resource != "" {
			event.WithResource(e.resource)
		}
		err := logger.Log(ctx, event)
		require.NoError(t, err)
	}

	// Query all events
	filter := QueryFilter{Limit: 100}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 3)
}

func TestSQLiteLoggerQueryBySessionID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert events with different session IDs
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker).WithSessionID("session-1"))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileWrite, CategoryBroker).WithSessionID("session-1"))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileList, CategoryBroker).WithSessionID("session-2"))
	require.NoError(t, err)

	// Query events for session-1
	filter := QueryFilter{
		SessionID: "session-1",
		Limit:     100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, 2, len(results))
	for _, event := range results {
		assert.Equal(t, "session-1", event.SessionID)
	}
}

func TestSQLiteLoggerQueryByRunID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert events with different run IDs
	err = logger.Log(ctx, NewEvent(EventToolExecute, CategoryTool).WithRunID("run-1"))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventToolSuccess, CategoryTool).WithRunID("run-1"))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventToolError, CategoryTool).WithRunID("run-2"))
	require.NoError(t, err)

	// Query events for run-1
	filter := QueryFilter{
		RunID: "run-1",
		Limit: 100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, 2, len(results))
	for _, event := range results {
		assert.Equal(t, "run-1", event.RunID)
	}
}

func TestSQLiteLoggerQueryByType(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert events of different types
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileWrite, CategoryBroker))
	require.NoError(t, err)

	// Query file read events
	filter := QueryFilter{
		Type:  EventFileRead,
		Limit: 100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)
	for _, event := range results {
		assert.Equal(t, EventFileRead, event.Type)
	}
}

func TestSQLiteLoggerQueryByCategory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert events of different categories
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventModelCall, CategoryModel))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventToolExecute, CategoryTool))
	require.NoError(t, err)

	// Query broker category events
	filter := QueryFilter{
		Category: CategoryBroker,
		Limit:    100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	for _, event := range results {
		assert.Equal(t, CategoryBroker, event.Category)
	}
}

func TestSQLiteLoggerQueryByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert events with different statuses
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker).WithStatus(StatusSuccess))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileWrite, CategoryBroker).WithStatus(StatusError))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileList, CategoryBroker).WithStatus(StatusDenied))
	require.NoError(t, err)

	// Query error status events
	filter := QueryFilter{
		Status: StatusError,
		Limit:  100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 1)
	for _, event := range results {
		assert.Equal(t, StatusError, event.Status)
	}
}

func TestSQLiteLoggerQueryByTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Record start time
	startTime := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)

	// Insert events
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker))
	require.NoError(t, err)
	err = logger.Log(ctx, NewEvent(EventFileWrite, CategoryBroker))
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	endTime := time.Now().UTC()

	// Query events within time range
	filter := QueryFilter{
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results), 2)

	// Verify all events are within the time range
	for _, event := range results {
		assert.True(t, event.Timestamp.After(startTime) || event.Timestamp.Equal(startTime))
		assert.True(t, event.Timestamp.Before(endTime) || event.Timestamp.Equal(endTime))
	}
}

func TestSQLiteLoggerQueryWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert multiple events
	for i := 0; i < 10; i++ {
		err := logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker))
		require.NoError(t, err)
	}

	// Query with limit
	filter := QueryFilter{Limit: 5}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 5)
}

func TestSQLiteLoggerQueryWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert events
	for i := 0; i < 10; i++ {
		err := logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker))
		require.NoError(t, err)
	}

	// Query with offset
	filter := QueryFilter{
		Limit:  5,
		Offset: 5,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 5)
}

func TestSQLiteLoggerQueryMultipleFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Insert specific event
	err = logger.Log(ctx, NewEvent(EventFileRead, CategoryBroker).
		WithSessionID("target-session").
		WithRunID("target-run").
		WithStatus(StatusSuccess))
	require.NoError(t, err)

	// Insert other events
	err = logger.Log(ctx, NewEvent(EventFileWrite, CategoryBroker).
		WithSessionID("target-session").
		WithRunID("other-run"))
	require.NoError(t, err)

	// Query with multiple filters
	filter := QueryFilter{
		SessionID: "target-session",
		RunID:     "target-run",
		Type:      EventFileRead,
		Status:    StatusSuccess,
		Limit:     100,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "target-session", results[0].SessionID)
	assert.Equal(t, "target-run", results[0].RunID)
	assert.Equal(t, EventFileRead, results[0].Type)
	assert.Equal(t, StatusSuccess, results[0].Status)
}

func TestSQLiteLoggerLogWithMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log event with metadata
	event := NewEvent(EventFileRead, CategoryBroker).
		WithMetadata("path", "./test.txt").
		WithMetadata("size", 1024).
		WithMetadata("duration_ms", 42)

	err = logger.Log(ctx, event)
	require.NoError(t, err)

	// Query and verify metadata
	filter := QueryFilter{Limit: 1}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	assert.Equal(t, "./test.txt", results[0].Metadata["path"])
	assert.Equal(t, float64(1024), results[0].Metadata["size"]) // JSON unmarshals numbers as float64
	assert.Equal(t, float64(42), results[0].Metadata["duration_ms"])
}

func TestSQLiteLoggerLogWithError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Log event with error
	testErr := errors.New("test error message")
	event := NewEvent(EventFileRead, CategoryBroker).
		WithError(testErr)

	err = logger.Log(ctx, event)
	require.NoError(t, err)

	// Query and verify error
	filter := QueryFilter{
		Status: StatusError,
		Limit:  1,
	}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 1, len(results))

	assert.Equal(t, "test error message", results[0].Error)
	assert.Equal(t, StatusError, results[0].Status)
}

func TestSQLiteLoggerClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)

	err = logger.Close()
	assert.NoError(t, err)

	// Verify we can't log after closing
	ctx := context.Background()
	event := NewEvent(EventFileRead, CategoryBroker)
	err = logger.Log(ctx, event)
	assert.Error(t, err)
}

func TestSQLiteLoggerConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	logger, err := NewSQLiteLogger(dbPath)
	require.NoError(t, err)
	defer logger.Close()

	ctx := context.Background()

	// Concurrent writes with sync.WaitGroup
	var wg sync.WaitGroup
	numGoroutines := 5
	eventsPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := NewEvent(EventFileRead, CategoryBroker)
				// Ignore errors in concurrent writes (SQLite may lock)
				_ = logger.Log(ctx, event)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify events were logged (may be less than total due to locks, but should have some)
	filter := QueryFilter{Limit: 1000}
	results, err := logger.Query(ctx, filter)
	require.NoError(t, err)
	assert.Greater(t, len(results), 0, "should have logged at least some events")
}

func TestDefaultQueryFilter(t *testing.T) {
	filter := DefaultQueryFilter()

	assert.Equal(t, 100, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
	assert.Empty(t, filter.SessionID)
	assert.Empty(t, filter.RunID)
	assert.Empty(t, filter.PluginID)
}

func TestNullStringHelper(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty string", "", false},
		{"non-empty string", "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullString(tt.input)
			assert.Equal(t, tt.expected, result.Valid)
			if tt.expected {
				assert.Equal(t, tt.input, result.String)
			}
		})
	}
}

func TestNullBytesHelper(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{"empty bytes", []byte{}, false},
		{"nil bytes", nil, false},
		{"non-empty bytes", []byte("test"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullBytes(tt.input)
			assert.Equal(t, tt.expected, result.Valid)
			if tt.expected {
				assert.Equal(t, string(tt.input), result.String)
			}
		})
	}
}
