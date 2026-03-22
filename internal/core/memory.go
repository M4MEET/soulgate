package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
)

// globalAgentID is the sentinel agentID used for entries shared across all agents.
const globalAgentID = ""

// MemoryEntry represents a single memory entry with scoping, TTL, and access tracking.
type MemoryEntry struct {
	AgentID        string     `json:"agent_id"`
	Key            string     `json:"key"`
	Value          string     `json:"value"`
	Tags           []string   `json:"tags,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	AccessCount    int        `json:"access_count"`
	LastAccessedAt time.Time  `json:"last_accessed_at"`
}

// isExpired reports whether the entry has passed its TTL.
func (e *MemoryEntry) isExpired() bool {
	return e.ExpiresAt != nil && time.Now().After(*e.ExpiresAt)
}

// MemoryStore manages persistent, per-agent memory across sessions.
// The zero-value agentID ("") represents the global scope shared by all agents.
//
// On-disk layout:
//
//	map[agentID]map[key]MemoryEntry  (JSON)
type MemoryStore struct {
	path    string
	mu      sync.RWMutex
	entries map[string]map[string]MemoryEntry // agentID -> key -> entry
}

// NewMemoryStore creates a new MemoryStore backed by the JSON file at
// <configDir>/state/memory.json. Existing entries are loaded on startup.
func NewMemoryStore(configDir string) (*MemoryStore, error) {
	memoryPath := filepath.Join(configDir, "state", "memory.json")

	store := &MemoryStore{
		path:    memoryPath,
		entries: make(map[string]map[string]MemoryEntry),
	}

	if _, err := os.Stat(memoryPath); err == nil {
		if err := store.load(); err != nil {
			return nil, fmt.Errorf("failed to load memory: %w", err)
		}
	}

	return store, nil
}

// ---- persistence -------------------------------------------------------

// load reads the nested map structure from disk.
// It tolerates old flat-map files by attempting a migration.
func (m *MemoryStore) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}

	// Try to unmarshal into the new nested structure first.
	var nested map[string]map[string]MemoryEntry
	if err := json.Unmarshal(data, &nested); err == nil {
		m.entries = nested
		return nil
	}

	// Fall back: attempt to parse the legacy flat structure and migrate it
	// into the global scope so old data is not lost.
	var flat map[string]MemoryEntry
	if err := json.Unmarshal(data, &flat); err != nil {
		return fmt.Errorf("unable to parse memory file: %w", err)
	}

	m.entries[globalAgentID] = make(map[string]MemoryEntry, len(flat))
	for k, v := range flat {
		v.AgentID = globalAgentID
		m.entries[globalAgentID][k] = v
	}
	return nil
}

// save persists the current state to disk.
// Caller must not hold m.mu (save acquires no lock itself; callers hold it).
func (m *MemoryStore) save() error {
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0600)
}

// ---- write helpers -----------------------------------------------------

// agentScope returns the per-agent bucket, creating it if necessary.
// Caller must hold m.mu (write lock).
func (m *MemoryStore) agentScope(agentID string) map[string]MemoryEntry {
	if _, ok := m.entries[agentID]; !ok {
		m.entries[agentID] = make(map[string]MemoryEntry)
	}
	return m.entries[agentID]
}

// ---- per-agent API -----------------------------------------------------

// WriteForAgent writes a key/value pair into the given agent's memory scope.
// Pass agentID="" to write to the global scope.
// An optional TTL duration d > 0 sets the ExpiresAt field.
// tags may be nil or empty.
func (m *MemoryStore) WriteForAgent(agentID, key, value string, tags []string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	scope := m.agentScope(agentID)

	entry, exists := scope[key]
	if exists {
		entry.Value = value
		entry.UpdatedAt = now
		entry.Tags = tags
	} else {
		entry = MemoryEntry{
			AgentID:        agentID,
			Key:            key,
			Value:          value,
			Tags:           tags,
			CreatedAt:      now,
			UpdatedAt:      now,
			LastAccessedAt: now,
		}
	}

	if ttl > 0 {
		exp := now.Add(ttl)
		entry.ExpiresAt = &exp
	} else {
		entry.ExpiresAt = nil
	}

	scope[key] = entry
	return m.save()
}

// GetForAgent retrieves a memory entry from the given agent's scope.
// It also checks the global scope as a fallback (agent-scoped entries shadow
// global ones with the same key). Returns ("", false) when not found or expired.
func (m *MemoryStore) GetForAgent(agentID, key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check agent scope first.
	if agentID != globalAgentID {
		if scope, ok := m.entries[agentID]; ok {
			if entry, ok := scope[key]; ok {
				if entry.isExpired() {
					// Lazy-delete expired entry.
					delete(scope, key)
					_ = m.save()
					// Fall through to check global scope.
				} else {
					entry.AccessCount++
					entry.LastAccessedAt = time.Now()
					scope[key] = entry
					_ = m.save()
					return entry.Value, true
				}
			}
		}
	}

	// Check global scope.
	if globalScope, ok := m.entries[globalAgentID]; ok {
		if entry, ok := globalScope[key]; ok {
			if entry.isExpired() {
				delete(globalScope, key)
				_ = m.save()
				return "", false
			}
			entry.AccessCount++
			entry.LastAccessedAt = time.Now()
			globalScope[key] = entry
			_ = m.save()
			return entry.Value, true
		}
	}

	return "", false
}

// SearchForAgent returns all non-expired entries in the given agent's scope
// (plus the global scope) whose key, value, or tags contain query (case-insensitive).
// If agentID is "", only the global scope is searched.
func (m *MemoryStore) SearchForAgent(agentID, query string) []MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(query)
	seen := make(map[string]struct{})
	var results []MemoryEntry

	matchEntry := func(entry MemoryEntry) bool {
		if entry.isExpired() {
			return false
		}
		if strings.Contains(strings.ToLower(entry.Key), q) ||
			strings.Contains(strings.ToLower(entry.Value), q) {
			return true
		}
		for _, tag := range entry.Tags {
			if strings.Contains(strings.ToLower(tag), q) {
				return true
			}
		}
		return false
	}

	// Agent-specific scope (shadowing global).
	if agentID != globalAgentID {
		if scope, ok := m.entries[agentID]; ok {
			for k, entry := range scope {
				if matchEntry(entry) {
					results = append(results, entry)
					seen[k] = struct{}{}
				}
			}
		}
	}

	// Global scope (only entries not already returned from agent scope).
	if globalScope, ok := m.entries[globalAgentID]; ok {
		for k, entry := range globalScope {
			if _, shadowed := seen[k]; shadowed {
				continue
			}
			if matchEntry(entry) {
				results = append(results, entry)
			}
		}
	}

	return results
}

// ListForAgent returns all non-expired entries visible to the given agent
// (its own scope merged with the global scope; agent entries take precedence).
func (m *MemoryStore) ListForAgent(agentID string) []MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]struct{})
	var entries []MemoryEntry

	if agentID != globalAgentID {
		if scope, ok := m.entries[agentID]; ok {
			for k, entry := range scope {
				if !entry.isExpired() {
					entries = append(entries, entry)
					seen[k] = struct{}{}
				}
			}
		}
	}

	if globalScope, ok := m.entries[globalAgentID]; ok {
		for k, entry := range globalScope {
			if _, shadowed := seen[k]; shadowed {
				continue
			}
			if !entry.isExpired() {
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

// GetRecentMemories returns up to limit entries from the given agent's visible
// scope (own + global), ordered by LastAccessedAt descending (most recent first).
func (m *MemoryStore) GetRecentMemories(agentID string, limit int) []MemoryEntry {
	all := m.ListForAgent(agentID)

	sort.Slice(all, func(i, j int) bool {
		return all[i].LastAccessedAt.After(all[j].LastAccessedAt)
	})

	if limit > 0 && len(all) > limit {
		return all[:limit]
	}
	return all
}

// ---- CleanExpired ------------------------------------------------------

// CleanExpired removes all entries whose TTL has elapsed from every agent scope.
// It is safe to call concurrently and persists the result to disk.
func (m *MemoryStore) CleanExpired() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for agentID, scope := range m.entries {
		for key, entry := range scope {
			if entry.isExpired() {
				delete(scope, key)
			}
		}
		// Remove the agent bucket entirely if now empty.
		if len(scope) == 0 {
			delete(m.entries, agentID)
		}
	}

	return m.save()
}

// ---- backward-compatible global API ------------------------------------

// Write writes a key/value pair into the global memory scope.
func (m *MemoryStore) Write(key, value string) error {
	return m.WriteForAgent(globalAgentID, key, value, nil, 0)
}

// Get retrieves a value from the global memory scope.
func (m *MemoryStore) Get(key string) (string, bool) {
	return m.GetForAgent(globalAgentID, key)
}

// Search searches the global memory scope for query.
func (m *MemoryStore) Search(query string) []MemoryEntry {
	return m.SearchForAgent(globalAgentID, query)
}

// List returns all non-expired entries in the global memory scope.
func (m *MemoryStore) List() []MemoryEntry {
	return m.ListForAgent(globalAgentID)
}

// Delete removes a key from the global memory scope.
func (m *MemoryStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scope, ok := m.entries[globalAgentID]; ok {
		delete(scope, key)
	}

	return m.save()
}

// ---- Orchestrator tool handlers ----------------------------------------

// handleMemoryWrite handles the memory_write tool call.
// The calling agent's identity is taken from brokerCtx.PluginID.
func (o *Orchestrator) handleMemoryWrite(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Key   string   `json:"key"`
		Value string   `json:"value"`
		Tags  []string `json:"tags"`
		TTL   int      `json:"ttl_seconds"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	if params.Key == "" {
		return "", fmt.Errorf("key is required")
	}
	if params.Value == "" {
		return "", fmt.Errorf("value is required")
	}

	agentID := brokerCtx.PluginID
	var ttl time.Duration
	if params.TTL > 0 {
		ttl = time.Duration(params.TTL) * time.Second
	}

	if err := o.memoryStore.WriteForAgent(agentID, params.Key, params.Value, params.Tags, ttl); err != nil {
		return "", fmt.Errorf("failed to write memory: %w", err)
	}

	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(brokerCtx.RunID).
		WithMetadata("tool", "memory_write").
		WithMetadata("agent_id", agentID).
		WithMetadata("key", params.Key))

	return fmt.Sprintf(`{"status": "success", "message": "Memory saved: %s = %s", "agent_id": "%s"}`,
		params.Key, params.Value, agentID), nil
}

// handleMemoryGet handles the memory_get tool call.
func (o *Orchestrator) handleMemoryGet(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	if params.Key == "" {
		return "", fmt.Errorf("key is required")
	}

	agentID := brokerCtx.PluginID
	value, exists := o.memoryStore.GetForAgent(agentID, params.Key)
	if !exists {
		return fmt.Sprintf(`{"status": "not_found", "message": "No memory found for key: %s", "agent_id": "%s"}`,
			params.Key, agentID), nil
	}

	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(brokerCtx.RunID).
		WithMetadata("tool", "memory_get").
		WithMetadata("agent_id", agentID).
		WithMetadata("key", params.Key))

	result := map[string]string{
		"status":   "success",
		"agent_id": agentID,
		"key":      params.Key,
		"value":    value,
	}

	output, _ := json.Marshal(result)
	return string(output), nil
}

// handleMemorySearch handles the memory_search tool call.
func (o *Orchestrator) handleMemorySearch(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	agentID := brokerCtx.PluginID

	if params.Query == "" {
		entries := o.memoryStore.ListForAgent(agentID)
		output, _ := json.Marshal(map[string]interface{}{
			"status":   "success",
			"agent_id": agentID,
			"count":    len(entries),
			"entries":  entries,
		})
		return string(output), nil
	}

	results := o.memoryStore.SearchForAgent(agentID, params.Query)

	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(brokerCtx.RunID).
		WithMetadata("tool", "memory_search").
		WithMetadata("agent_id", agentID).
		WithMetadata("query", params.Query).
		WithMetadata("results", fmt.Sprintf("%d", len(results))))

	output, _ := json.Marshal(map[string]interface{}{
		"status":   "success",
		"agent_id": agentID,
		"query":    params.Query,
		"count":    len(results),
		"results":  results,
	})

	return string(output), nil
}
