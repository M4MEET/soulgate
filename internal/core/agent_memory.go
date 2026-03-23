package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentMemoryEntry is a single key-value pair in an agent's private memory.
type AgentMemoryEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AgentMemory provides per-agent persistent key-value storage.
// Each agent gets its own memory file under .soulgate/state/agents/<id>/memory.json.
type AgentMemory struct {
	mu      sync.RWMutex
	entries map[string]*AgentMemoryEntry
	path    string
}

// NewAgentMemory creates or loads an agent's private memory store.
func NewAgentMemory(agentDir string) *AgentMemory {
	m := &AgentMemory{
		entries: make(map[string]*AgentMemoryEntry),
		path:    filepath.Join(agentDir, "memory.json"),
	}
	m.load()
	return m
}

// Set writes a key-value pair to the agent's memory.
func (m *AgentMemory) Set(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = &AgentMemoryEntry{
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now().UTC(),
	}
	m.save()
}

// Get retrieves a value by key. Returns empty string and false if not found.
func (m *AgentMemory) Get(key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[key]
	if !ok {
		return "", false
	}
	return e.Value, true
}

// Delete removes a key from the agent's memory.
func (m *AgentMemory) Delete(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.entries[key]; !ok {
		return false
	}
	delete(m.entries, key)
	m.save()
	return true
}

// List returns all memory entries.
func (m *AgentMemory) List() []AgentMemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AgentMemoryEntry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, *e)
	}
	return out
}

// Clear removes all entries.
func (m *AgentMemory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*AgentMemoryEntry)
	m.save()
}

func (m *AgentMemory) save() {
	if m.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(m.path), 0700)
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.path, data, 0600)
}

func (m *AgentMemory) load() {
	if m.path == "" {
		return
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	var entries map[string]*AgentMemoryEntry
	if json.Unmarshal(data, &entries) == nil && entries != nil {
		m.entries = entries
	}
}

// agentDataDir returns the per-agent data directory.
func agentDataDir(configDir, agentID string) string {
	if configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "state", "agents", agentID)
}

// GetOrCreateMemory returns the agent's memory store, creating it if needed.
func (a *BackgroundAgent) GetOrCreateMemory(configDir string) *AgentMemory {
	a.memMu.Lock()
	defer a.memMu.Unlock()
	if a.memory == nil {
		dir := agentDataDir(configDir, a.ID)
		if dir != "" {
			a.memory = NewAgentMemory(dir)
		} else {
			a.memory = &AgentMemory{entries: make(map[string]*AgentMemoryEntry)}
		}
	}
	return a.memory
}

// isStandbyTask returns true if the task description suggests a standby/ready agent.
func isStandbyTask(task string, result string) bool {
	standbyKeywords := []string{"standby", "stand by", "ready", "assist", "help with", "be available", "wait for"}
	lower := fmt.Sprintf("%s %s", task, result)
	for _, kw := range standbyKeywords {
		if containsLower(lower, kw) {
			return true
		}
	}
	return false
}

func containsLower(s, substr string) bool {
	ls := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		ls[i] = c
	}
	lsub := make([]byte, len(substr))
	for i := range substr {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		lsub[i] = c
	}
	return bytes_contains(ls, lsub)
}

func bytes_contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
