package audit

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventFileRead, CategoryBroker)

	assert.NotEmpty(t, event.ID)
	assert.Equal(t, EventFileRead, event.Type)
	assert.Equal(t, CategoryBroker, event.Category)
	assert.Equal(t, StatusSuccess, event.Status)
	assert.NotNil(t, event.Metadata)
	assert.WithinDuration(t, time.Now().UTC(), event.Timestamp, time.Second)
}

func TestEventWithSessionID(t *testing.T) {
	event := NewEvent(EventRunStart, CategoryRun).
		WithSessionID("session-123")

	assert.Equal(t, "session-123", event.SessionID)
}

func TestEventWithRunID(t *testing.T) {
	event := NewEvent(EventToolExecute, CategoryTool).
		WithRunID("run-456")

	assert.Equal(t, "run-456", event.RunID)
}

func TestEventWithPlugin(t *testing.T) {
	event := NewEvent(EventPluginLoad, CategoryPlugin).
		WithPlugin("plugin-789")

	assert.Equal(t, "plugin-789", event.PluginID)
}

func TestEventWithAction(t *testing.T) {
	event := NewEvent(EventPolicyEvaluate, CategoryPolicy).
		WithAction("files.read")

	assert.Equal(t, "files.read", event.Action)
}

func TestEventWithResource(t *testing.T) {
	event := NewEvent(EventFileRead, CategoryBroker).
		WithResource("./test.txt")

	assert.Equal(t, "./test.txt", event.Resource)
}

func TestEventWithDecision(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
	}{
		{"allow decision", DecisionAllow},
		{"deny decision", DecisionDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(EventPolicyAllow, CategoryPolicy).
				WithDecision(tt.decision)

			assert.Equal(t, tt.decision, event.Decision)
		})
	}
}

func TestEventWithStatus(t *testing.T) {
	tests := []struct {
		name   string
		status EventStatus
	}{
		{"success status", StatusSuccess},
		{"error status", StatusError},
		{"denied status", StatusDenied},
		{"pending status", StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(EventToolExecute, CategoryTool).
				WithStatus(tt.status)

			assert.Equal(t, tt.status, event.Status)
		})
	}
}

func TestEventWithError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectError  string
		expectStatus EventStatus
	}{
		{
			name:         "with error",
			err:          errors.New("test error"),
			expectError:  "test error",
			expectStatus: StatusError,
		},
		{
			name:         "with nil error",
			err:          nil,
			expectError:  "",
			expectStatus: StatusSuccess, // Default status
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(EventFileRead, CategoryBroker).
				WithError(tt.err)

			assert.Equal(t, tt.expectError, event.Error)
			assert.Equal(t, tt.expectStatus, event.Status)
		})
	}
}

func TestEventWithMetadata(t *testing.T) {
	event := NewEvent(EventFileRead, CategoryBroker).
		WithMetadata("path", "/test.txt").
		WithMetadata("size", 1024).
		WithMetadata("duration_ms", 42)

	assert.Equal(t, "/test.txt", event.Metadata["path"])
	assert.Equal(t, 1024, event.Metadata["size"])
	assert.Equal(t, 42, event.Metadata["duration_ms"])
}

func TestEventWithMetadataNilMap(t *testing.T) {
	// Test that metadata initializes even if nil
	event := &Event{
		ID:       "test-id",
		Type:     EventFileRead,
		Category: CategoryBroker,
		Status:   StatusSuccess,
		Metadata: nil, // Explicitly nil
	}

	event.WithMetadata("key", "value")

	assert.NotNil(t, event.Metadata)
	assert.Equal(t, "value", event.Metadata["key"])
}

func TestEventChaining(t *testing.T) {
	// Test method chaining
	event := NewEvent(EventFileRead, CategoryBroker).
		WithSessionID("session-1").
		WithRunID("run-1").
		WithPlugin("plugin-1").
		WithAction("files.read").
		WithResource("./test.txt").
		WithDecision(DecisionAllow).
		WithStatus(StatusSuccess).
		WithMetadata("size", 100).
		WithMetadata("encoding", "utf-8")

	assert.Equal(t, "session-1", event.SessionID)
	assert.Equal(t, "run-1", event.RunID)
	assert.Equal(t, "plugin-1", event.PluginID)
	assert.Equal(t, "files.read", event.Action)
	assert.Equal(t, "./test.txt", event.Resource)
	assert.Equal(t, DecisionAllow, event.Decision)
	assert.Equal(t, StatusSuccess, event.Status)
	assert.Equal(t, 100, event.Metadata["size"])
	assert.Equal(t, "utf-8", event.Metadata["encoding"])
}

func TestEventToJSON(t *testing.T) {
	event := NewEvent(EventFileRead, CategoryBroker).
		WithSessionID("session-123").
		WithRunID("run-456").
		WithResource("./test.txt").
		WithStatus(StatusSuccess)

	jsonData, err := event.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)
	assert.Contains(t, string(jsonData), "session-123")
	assert.Contains(t, string(jsonData), "run-456")
	assert.Contains(t, string(jsonData), "./test.txt")
}

func TestEventToJSONWithMetadata(t *testing.T) {
	event := NewEvent(EventFileRead, CategoryBroker).
		WithMetadata("key1", "value1").
		WithMetadata("key2", 123)

	jsonData, err := event.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, string(jsonData), "key1")
	assert.Contains(t, string(jsonData), "value1")
	assert.Contains(t, string(jsonData), "key2")
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	time.Sleep(1 * time.Millisecond)
	id2 := generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "IDs should be unique")
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
	}{
		{"run start", EventRunStart},
		{"run complete", EventRunComplete},
		{"run error", EventRunError},
		{"model call", EventModelCall},
		{"model response", EventModelResponse},
		{"model error", EventModelError},
		{"tool execute", EventToolExecute},
		{"tool success", EventToolSuccess},
		{"tool error", EventToolError},
		{"policy evaluate", EventPolicyEvaluate},
		{"policy allow", EventPolicyAllow},
		{"policy deny", EventPolicyDeny},
		{"file read", EventFileRead},
		{"file write", EventFileWrite},
		{"file list", EventFileList},
		{"file stat", EventFileStat},
		{"file delete", EventFileDelete},
		{"net request", EventNetRequest},
		{"exec command", EventExecCommand},
		{"plugin load", EventPluginLoad},
		{"plugin unload", EventPluginUnload},
		{"session start", EventSessionStart},
		{"session end", EventSessionEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.eventType))
		})
	}
}

func TestEventCategories(t *testing.T) {
	tests := []struct {
		name     string
		category EventCategory
	}{
		{"run category", CategoryRun},
		{"model category", CategoryModel},
		{"tool category", CategoryTool},
		{"policy category", CategoryPolicy},
		{"broker category", CategoryBroker},
		{"plugin category", CategoryPlugin},
		{"session category", CategorySession},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.category))
		})
	}
}

func TestEventStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status EventStatus
	}{
		{"success status", StatusSuccess},
		{"error status", StatusError},
		{"denied status", StatusDenied},
		{"pending status", StatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.status))
		})
	}
}

func TestDecisions(t *testing.T) {
	tests := []struct {
		name     string
		decision Decision
	}{
		{"allow decision", DecisionAllow},
		{"deny decision", DecisionDeny},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, string(tt.decision))
		})
	}
}
