package gateway

import (
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/google/uuid"
)

// SessionState represents the state of a session
type SessionState string

const (
	SessionStateActive    SessionState = "active"
	SessionStateIdle      SessionState = "idle"
	SessionStatePaused    SessionState = "paused"
	SessionStateCompleted SessionState = "completed"
)

// Session represents a conversation session
type Session struct {
	ID             string
	ConversationID string
	Channel        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastActivityAt time.Time
	State          SessionState

	// Agent routing
	AssignedAgentID      string   // Current agent handling this session
	AgentHistory         []string // All agents that have handled this session
	AgentAffinityEnabled bool     // Whether to keep using the same agent

	// Statistics
	MessageCount   int
	ToolCalls      int
	TotalTokens    int64
	AverageLatency int64 // milliseconds

	// Message history
	messages []interface{}
	mu       sync.RWMutex
}

// NewSession creates a new session
func NewSession(id, conversationID, channel string) *Session {
	if id == "" {
		id = uuid.New().String()
	}

	now := time.Now()
	return &Session{
		ID:                   id,
		ConversationID:       conversationID,
		Channel:              channel,
		CreatedAt:            now,
		UpdatedAt:            now,
		LastActivityAt:       now,
		State:                SessionStateActive,
		AgentAffinityEnabled: true, // Enable by default
		AgentHistory:         make([]string, 0),
		messages:             make([]interface{}, 0),
	}
}

// AddMessage adds a message to the session history
func (s *Session) AddMessage(message interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, message)
	now := time.Now()
	s.UpdatedAt = now
	s.LastActivityAt = now
	s.MessageCount++

	// Activate session if it was idle
	if s.State == SessionStateIdle {
		s.State = SessionStateActive
	}
}

// GetMessages returns all messages in the session
func (s *Session) GetMessages() []interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return copy to avoid race conditions
	messages := make([]interface{}, len(s.messages))
	copy(messages, s.messages)
	return messages
}

// GetHistory returns formatted message history for agent context
func (s *Session) GetHistory() []protocol.EventMessageFrame {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]protocol.EventMessageFrame, 0)
	for _, msg := range s.messages {
		if eventMsg, ok := msg.(*protocol.EventMessageFrame); ok {
			history = append(history, *eventMsg)
		}
	}

	return history
}

// GetMessageCount returns the number of messages in the session
func (s *Session) GetMessageCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.messages)
}

// AssignAgent assigns an agent to this session
func (s *Session) AssignAgent(agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Record in history if this is a new agent
	if s.AssignedAgentID != agentID {
		s.AgentHistory = append(s.AgentHistory, agentID)
	}

	s.AssignedAgentID = agentID
	s.UpdatedAt = time.Now()
}

// GetAssignedAgent returns the currently assigned agent
func (s *Session) GetAssignedAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.AssignedAgentID
}

// UnassignAgent removes the agent assignment
func (s *Session) UnassignAgent() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.AssignedAgentID = ""
	s.UpdatedAt = time.Now()
}

// SetState updates the session state
func (s *Session) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.State = state
	s.UpdatedAt = time.Now()
}

// GetState returns the current session state
func (s *Session) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.State
}

// UpdateActivity updates the last activity timestamp
func (s *Session) UpdateActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastActivityAt = time.Now()
	s.UpdatedAt = time.Now()

	// Activate session if it was idle
	if s.State == SessionStateIdle {
		s.State = SessionStateActive
	}
}

// IsActive returns whether the session is currently active
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.State == SessionStateActive
}

// IsIdle returns whether the session has been idle for the given duration
func (s *Session) IsIdle(idleThreshold time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return time.Since(s.LastActivityAt) > idleThreshold
}

// IncrementMessageCount increments the message counter
func (s *Session) IncrementMessageCount() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.MessageCount++
}

// IncrementToolCalls increments the tool call counter
func (s *Session) IncrementToolCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ToolCalls++
}

// GetStatistics returns session statistics
func (s *Session) GetStatistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"message_count":    s.MessageCount,
		"tool_calls":       s.ToolCalls,
		"total_tokens":     s.TotalTokens,
		"average_latency":  s.AverageLatency,
		"agent_history":    s.AgentHistory,
		"assigned_agent":   s.AssignedAgentID,
		"state":            s.State,
		"created_at":       s.CreatedAt,
		"updated_at":       s.UpdatedAt,
		"last_activity_at": s.LastActivityAt,
	}
}
