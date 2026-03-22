package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// InboxEntry is a persistent notification stored server-side.
type InboxEntry struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"` // success, error, warning, info, agent, connection, activity
	Title     string                 `json:"title"`
	Detail    string                 `json:"detail,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // extra context for detail view
	Timestamp time.Time              `json:"timestamp"`
	Read      bool                   `json:"read"`
	Pinned    bool                   `json:"pinned,omitempty"`
}

// InboxStore persists user-facing notifications to a JSON file.
type InboxStore struct {
	mu      sync.RWMutex
	entries []InboxEntry
	path    string
	nextID  int
	maxSize int // max entries to keep
}

// NewInboxStore creates a store backed by the given JSON file.
func NewInboxStore(path string) *InboxStore {
	s := &InboxStore{
		path:    path,
		maxSize: 200,
	}
	_ = s.load()
	return s
}

func (s *InboxStore) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var entries []InboxEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	s.entries = entries
	// Find max ID
	for _, e := range entries {
		var num int
		if _, scanErr := fmt.Sscanf(e.ID, "notif_%d", &num); scanErr == nil && num >= s.nextID {
			s.nextID = num
		}
	}
	return nil
}

func (s *InboxStore) save() {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0600)
}

// Push adds a notification and persists to disk.
func (s *InboxStore) Push(kind, title, detail string, metadata map[string]interface{}) InboxEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	entry := InboxEntry{
		ID:        fmt.Sprintf("notif_%d", s.nextID),
		Kind:      kind,
		Title:     title,
		Detail:    detail,
		Metadata:  metadata,
		Timestamp: time.Now().UTC(),
		Read:      false,
	}

	// Prepend (newest first)
	s.entries = append([]InboxEntry{entry}, s.entries...)

	// Cap size
	if len(s.entries) > s.maxSize {
		s.entries = s.entries[:s.maxSize]
	}

	s.save()
	return entry
}

// List returns all entries, newest first.
func (s *InboxStore) List() []InboxEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]InboxEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

// Get returns a single entry by ID.
func (s *InboxStore) Get(id string) (InboxEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.ID == id {
			return e, true
		}
	}
	return InboxEntry{}, false
}

// MarkRead marks a notification as read.
func (s *InboxStore) MarkRead(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries[i].Read = true
			s.save()
			return
		}
	}
}

// MarkAllRead marks all notifications as read.
func (s *InboxStore) MarkAllRead() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		s.entries[i].Read = false // will be set below
	}
	for i := range s.entries {
		s.entries[i].Read = true
	}
	s.save()
}

// Delete removes a notification by ID.
func (s *InboxStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			s.save()
			return
		}
	}
}

// UnreadCount returns the number of unread notifications.
func (s *InboxStore) UnreadCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, e := range s.entries {
		if !e.Read {
			count++
		}
	}
	return count
}

// SortedByTime returns entries sorted newest first (they already are, but this ensures it).
func (s *InboxStore) SortedByTime() []InboxEntry {
	entries := s.List()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	return entries
}
