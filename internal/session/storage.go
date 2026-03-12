package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
)

// Storage manages JSONL session files
type Storage struct {
	sessionsDir string
}

// NewStorage creates a new session storage
func NewStorage(sessionsDir string) (*Storage, error) {
	// Create sessions directory if it doesn't exist
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Storage{
		sessionsDir: sessionsDir,
	}, nil
}

// Entry represents a single line in the JSONL session file
type Entry struct {
	Timestamp int64       `json:"ts"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
}

// LogMessage logs a message to the session
func (s *Storage) LogMessage(sessionID string, sender string, text string) error {
	entry := Entry{
		Timestamp: time.Now().Unix(),
		Type:      "message",
		Data: map[string]interface{}{
			"sender": sender,
			"text":   text,
		},
	}

	return s.appendEntry(sessionID, entry)
}

// LogToolCall logs a tool call to the session
func (s *Storage) LogToolCall(sessionID string, toolName string, args map[string]interface{}) error {
	entry := Entry{
		Timestamp: time.Now().Unix(),
		Type:      "tool_call",
		Data: map[string]interface{}{
			"tool_name": toolName,
			"args":      args,
		},
	}

	return s.appendEntry(sessionID, entry)
}

// LogToolResult logs a tool result to the session
func (s *Storage) LogToolResult(sessionID string, toolName string, result interface{}, err error) error {
	data := map[string]interface{}{
		"tool_name": toolName,
		"result":    result,
	}

	if err != nil {
		data["error"] = err.Error()
	}

	entry := Entry{
		Timestamp: time.Now().Unix(),
		Type:      "tool_result",
		Data:      data,
	}

	return s.appendEntry(sessionID, entry)
}

// LogResponse logs an agent response to the session
func (s *Storage) LogResponse(sessionID string, text string) error {
	entry := Entry{
		Timestamp: time.Now().Unix(),
		Type:      "response",
		Data: map[string]interface{}{
			"text": text,
		},
	}

	return s.appendEntry(sessionID, entry)
}

// LogEvent logs a generic event to the session
func (s *Storage) LogEvent(sessionID string, eventType string, data interface{}) error {
	entry := Entry{
		Timestamp: time.Now().Unix(),
		Type:      eventType,
		Data:      data,
	}

	return s.appendEntry(sessionID, entry)
}

// LogFrame logs a protocol frame to the session
func (s *Storage) LogFrame(sessionID string, frame interface{}) error {
	var entryType string
	var data interface{}

	switch f := frame.(type) {
	case *protocol.EventMessageFrame:
		entryType = "event.message"
		data = map[string]interface{}{
			"channel":         f.Channel,
			"conversation_id": f.ConversationID,
			"sender":          f.Sender,
			"text":            f.Text,
		}

	case *protocol.EventToolStartFrame:
		entryType = "event.tool.start"
		data = map[string]interface{}{
			"tool_name": f.ToolName,
			"tool_id":   f.ToolID,
			"args":      f.Args,
		}

	case *protocol.EventToolEndFrame:
		entryType = "event.tool.end"
		data = map[string]interface{}{
			"tool_name": f.ToolName,
			"tool_id":   f.ToolID,
			"result":    f.Result,
			"error":     f.Error,
			"duration":  f.Duration,
		}

	case *protocol.CmdChannelSendFrame:
		entryType = "cmd.channel.send"
		data = map[string]interface{}{
			"channel":         f.Channel,
			"conversation_id": f.ConversationID,
			"text":            f.Text,
		}

	default:
		// Generic frame logging
		entryType = "frame"
		data = frame
	}

	entry := Entry{
		Timestamp: time.Now().Unix(),
		Type:      entryType,
		Data:      data,
	}

	return s.appendEntry(sessionID, entry)
}

// appendEntry appends an entry to the session JSONL file
func (s *Storage) appendEntry(sessionID string, entry Entry) error {
	filePath := s.getSessionPath(sessionID)

	// Open file in append mode, create if doesn't exist
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Append with newline
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	return nil
}

// ReadSession reads all entries from a session
func (s *Storage) ReadSession(sessionID string) ([]Entry, error) {
	filePath := s.getSessionPath(sessionID)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []Entry{}, nil // Return empty slice for new sessions
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Parse JSONL
	var entries []Entry
	lines := splitLines(data)

	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("failed to parse line %d: %w", i+1, err)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ListSessions returns all session IDs
func (s *Storage) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(s.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			// Remove .jsonl extension to get session ID
			sessionID := entry.Name()[:len(entry.Name())-6]
			sessions = append(sessions, sessionID)
		}
	}

	return sessions, nil
}

// DeleteSession deletes a session file
func (s *Storage) DeleteSession(sessionID string) error {
	filePath := s.getSessionPath(sessionID)

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// GetSessionInfo returns metadata about a session
func (s *Storage) GetSessionInfo(sessionID string) (*SessionInfo, error) {
	filePath := s.getSessionPath(sessionID)

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to stat session file: %w", err)
	}

	// Read entries to count
	entries, err := s.ReadSession(sessionID)
	if err != nil {
		return nil, err
	}

	// Calculate stats
	var messageCount, toolCallCount, responseCount int
	var firstTimestamp, lastTimestamp int64

	for i, entry := range entries {
		if i == 0 {
			firstTimestamp = entry.Timestamp
		}
		lastTimestamp = entry.Timestamp

		switch entry.Type {
		case "message", "event.message":
			messageCount++
		case "tool_call", "event.tool.start":
			toolCallCount++
		case "response", "cmd.channel.send":
			responseCount++
		}
	}

	return &SessionInfo{
		SessionID:     sessionID,
		FilePath:      filePath,
		FileSize:      info.Size(),
		EntryCount:    len(entries),
		MessageCount:  messageCount,
		ToolCallCount: toolCallCount,
		ResponseCount: responseCount,
		FirstEntry:    time.Unix(firstTimestamp, 0),
		LastEntry:     time.Unix(lastTimestamp, 0),
		CreatedAt:     info.ModTime(),
	}, nil
}

// SessionInfo holds metadata about a session
type SessionInfo struct {
	SessionID     string
	FilePath      string
	FileSize      int64
	EntryCount    int
	MessageCount  int
	ToolCallCount int
	ResponseCount int
	FirstEntry    time.Time
	LastEntry     time.Time
	CreatedAt     time.Time
}

// getSessionPath returns the file path for a session
func (s *Storage) getSessionPath(sessionID string) string {
	return filepath.Join(s.sessionsDir, fmt.Sprintf("%s.jsonl", sessionID))
}

// splitLines splits data by newlines
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	var start int

	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}

	// Add last line if no trailing newline
	if start < len(data) {
		lines = append(lines, data[start:])
	}

	return lines
}
