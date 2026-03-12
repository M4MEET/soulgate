package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStorage(t *testing.T) {
	// Create temp directory for tests
	tmpDir, err := os.MkdirTemp("", "session-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage, err := NewStorage(tmpDir)
	require.NoError(t, err)

	sessionID := "test-session-123"

	// Test logging message
	err = storage.LogMessage(sessionID, "user1", "Hello world")
	require.NoError(t, err)

	// Test logging tool call
	err = storage.LogToolCall(sessionID, "read_file", map[string]interface{}{
		"path": "test.txt",
	})
	require.NoError(t, err)

	// Test logging tool result
	err = storage.LogToolResult(sessionID, "read_file", "file contents", nil)
	require.NoError(t, err)

	// Test logging response
	err = storage.LogResponse(sessionID, "Here is the file content")
	require.NoError(t, err)

	// Read session back
	entries, err := storage.ReadSession(sessionID)
	require.NoError(t, err)
	assert.Len(t, entries, 4)

	// Check entry types
	assert.Equal(t, "message", entries[0].Type)
	assert.Equal(t, "tool_call", entries[1].Type)
	assert.Equal(t, "tool_result", entries[2].Type)
	assert.Equal(t, "response", entries[3].Type)

	// Check message content
	messageData := entries[0].Data.(map[string]interface{})
	assert.Equal(t, "user1", messageData["sender"])
	assert.Equal(t, "Hello world", messageData["text"])

	// Test session info
	info, err := storage.GetSessionInfo(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, info.SessionID)
	assert.Equal(t, 4, info.EntryCount)
	assert.Equal(t, 1, info.MessageCount)
	assert.Equal(t, 1, info.ToolCallCount)
	assert.Equal(t, 1, info.ResponseCount)

	// Test list sessions
	sessions, err := storage.ListSessions()
	require.NoError(t, err)
	assert.Contains(t, sessions, sessionID)

	// Verify file exists
	sessionPath := filepath.Join(tmpDir, sessionID+".jsonl")
	_, err = os.Stat(sessionPath)
	assert.NoError(t, err)

	// Test delete session
	err = storage.DeleteSession(sessionID)
	require.NoError(t, err)

	// Verify file deleted
	_, err = os.Stat(sessionPath)
	assert.True(t, os.IsNotExist(err))
}

func TestReadNonExistentSession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewStorage(tmpDir)
	require.NoError(t, err)

	// Reading non-existent session should return empty slice
	entries, err := storage.ReadSession("nonexistent")
	require.NoError(t, err)
	assert.Len(t, entries, 0)
}

func TestAppendMultipleEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "session-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewStorage(tmpDir)
	require.NoError(t, err)

	sessionID := "append-test"

	// Append multiple messages
	for i := 0; i < 10; i++ {
		err := storage.LogMessage(sessionID, "user", "Message "+string(rune('0'+i)))
		require.NoError(t, err)
	}

	// Read back
	entries, err := storage.ReadSession(sessionID)
	require.NoError(t, err)
	assert.Len(t, entries, 10)

	// Verify order
	for i, entry := range entries {
		data := entry.Data.(map[string]interface{})
		expectedText := "Message " + string(rune('0'+i))
		assert.Equal(t, expectedText, data["text"])
	}
}
