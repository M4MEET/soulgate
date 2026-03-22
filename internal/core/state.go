package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/M4MEET/soulgate/internal/model"
)

// SessionState represents the persistent state of a TUI session.
// Saved to .soulgate/session_state.json on shutdown and restored on startup.
type SessionState struct {
	SessionID string    `json:"session_id"`
	SavedAt   time.Time `json:"saved_at"`

	// Chat history (rendered messages shown in the viewport)
	Messages []string `json:"messages,omitempty"`

	// Conversation history (structured model messages for AI context continuity)
	ConversationHistory []model.Message `json:"conversation_history,omitempty"`

	// Command history (for up/down arrow navigation)
	CommandHistory []string `json:"command_history,omitempty"`

	// Agent snapshots
	Agents []AgentSnapshot `json:"agents,omitempty"`

	// Settings
	TrustMode        bool   `json:"trust_mode"`
	StreamingEnabled bool   `json:"streaming_enabled"`
	CurrentProvider  string `json:"current_provider,omitempty"`
	CurrentModel     string `json:"current_model,omitempty"`
}

// AgentSnapshot is the serializable form of a BackgroundAgent.
type AgentSnapshot struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Task        string          `json:"task"`
	Status      AgentStatus     `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Result      string          `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	ActivityLog []AgentLogEntry `json:"activity_log,omitempty"`
}

const stateFileName = "state/session.json"

// SaveSessionState writes the session state to disk.
func SaveSessionState(configDir string, state *SessionState) error {
	state.SavedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	path := filepath.Join(configDir, stateFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write session state: %w", err)
	}

	return nil
}

// LoadSessionState reads the session state from disk.
// Returns nil, nil if no state file exists.
func LoadSessionState(configDir string) (*SessionState, error) {
	path := filepath.Join(configDir, stateFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read session state: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse session state: %w", err)
	}

	return &state, nil
}

// ClearSessionState deletes the state file.
func ClearSessionState(configDir string) error {
	path := filepath.Join(configDir, stateFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SnapshotAgents captures the current state of all agents for persistence.
func (am *AgentManager) Snapshot() []AgentSnapshot {
	am.mu.RLock()
	defer am.mu.RUnlock()

	snapshots := make([]AgentSnapshot, 0, len(am.agents))
	for _, a := range am.agents {
		snap := AgentSnapshot{
			ID:          a.ID,
			Name:        a.Name,
			Task:        a.Task,
			Status:      a.Status,
			CreatedAt:   a.CreatedAt,
			CompletedAt: a.CompletedAt,
			Result:      a.Result,
			Error:       a.Error,
			ActivityLog: a.GetLog(),
		}
		// Running agents that are being serialized were interrupted
		if snap.Status == AgentRunning {
			snap.Status = AgentStopped
			now := time.Now().UTC()
			snap.CompletedAt = &now
			snap.Error = "interrupted: session ended while agent was running"
		}
		snapshots = append(snapshots, snap)
	}
	return snapshots
}

// RestoreAgents loads agent snapshots into the manager so they're visible.
func (am *AgentManager) RestoreAgents(snapshots []AgentSnapshot) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for _, snap := range snapshots {
		agent := &BackgroundAgent{
			ID:          snap.ID,
			Name:        snap.Name,
			Task:        snap.Task,
			Status:      snap.Status,
			CreatedAt:   snap.CreatedAt,
			CompletedAt: snap.CompletedAt,
			Result:      snap.Result,
			Error:       snap.Error,
		}
		// Restore activity log
		agent.logMu.Lock()
		agent.activityLog = snap.ActivityLog
		agent.logMu.Unlock()

		am.agents[snap.ID] = agent

		// Track highest ID for nextID
		var num int
		if _, err := fmt.Sscanf(snap.ID, "agent_%d", &num); err == nil {
			if num >= am.nextID {
				am.nextID = num
			}
		}
	}
}
