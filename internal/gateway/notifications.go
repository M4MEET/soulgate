package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// NotificationConfig describes a single outbound notification destination.
type NotificationConfig struct {
	// Name identifies this notification target.
	Name string `json:"name"`
	// URL is the HTTP endpoint that will receive a POST for each matched event.
	URL string `json:"url"`
	// Events is a list of event names that should trigger this notification.
	// Supported values: "message.received", "agent.completed", "error".
	// Use ["*"] to subscribe to all events.
	Events []string `json:"events"`
	// Enabled allows toggling a target without deleting it.
	Enabled bool `json:"enabled"`
}

// notificationPayload is the JSON body sent to outbound notification URLs.
type notificationPayload struct {
	Event   string      `json:"event"`
	OccurAt time.Time   `json:"occurred_at"`
	Payload interface{} `json:"payload"`
}

// NotificationStore persists the list of outbound notification configs.
type NotificationStore struct {
	mu            sync.RWMutex
	notifications map[string]*NotificationConfig
	path          string
}

// NewNotificationStore creates a NotificationStore backed by the given JSON file
// and loads any existing configs from disk. A missing file is not an error.
func NewNotificationStore(path string) (*NotificationStore, error) {
	s := &NotificationStore{
		notifications: make(map[string]*NotificationConfig),
		path:          path,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func newNotificationStore(path string) *NotificationStore {
	return &NotificationStore{
		notifications: make(map[string]*NotificationConfig),
		path:          path,
	}
}

// load reads notification configs from the JSON file. Missing file is not an error.
func (s *NotificationStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read notifications file: %w", err)
	}

	var list []*NotificationConfig
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse notifications file: %w", err)
	}

	s.notifications = make(map[string]*NotificationConfig, len(list))
	for _, n := range list {
		s.notifications[n.Name] = n
	}
	return nil
}

// save writes the current notification list to disk atomically.
func (s *NotificationStore) save() error {
	list := make([]*NotificationConfig, 0, len(s.notifications))
	for _, n := range s.notifications {
		list = append(list, n)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal notifications: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write notifications tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename notifications file: %w", err)
	}
	return nil
}

// List returns a copy of all configured notification targets.
func (s *NotificationStore) List() []*NotificationConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*NotificationConfig, 0, len(s.notifications))
	for _, n := range s.notifications {
		cp := *n
		out = append(out, &cp)
	}
	return out
}

// Add persists a new notification config. Returns an error if the name is already taken.
func (s *NotificationStore) Add(n *NotificationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.notifications[n.Name]; exists {
		return fmt.Errorf("notification %q already exists", n.Name)
	}

	s.notifications[n.Name] = n
	return s.save()
}

// Remove deletes the notification target with the given name.
func (s *NotificationStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.notifications[name]; !exists {
		return fmt.Errorf("notification %q not found", name)
	}
	delete(s.notifications, name)
	return s.save()
}

// matchesEvent reports whether the notification target subscribes to the given
// event name.  A target with Events==["*"] matches all events.
func (nc *NotificationConfig) matchesEvent(event string) bool {
	for _, e := range nc.Events {
		if e == "*" || e == event {
			return true
		}
	}
	return false
}

// Notify dispatches an outbound notification to every enabled target that
// subscribes to the named event. Delivery is fire-and-forget: failures are
// logged to stdout but do not propagate to callers.
func (g *Gateway) Notify(event string, payload interface{}) {
	if g.notificationStore == nil {
		return
	}

	targets := g.notificationStore.List()
	if len(targets) == 0 {
		return
	}

	env := &notificationPayload{
		Event:   event,
		OccurAt: time.Now().UTC(),
		Payload: payload,
	}

	body, err := json.Marshal(env)
	if err != nil {
		fmt.Printf("[notify] failed to marshal payload for event %q: %v\n", event, err)
		return
	}

	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		if !target.matchesEvent(event) {
			continue
		}

		// Fire each delivery in its own goroutine so one slow endpoint does not
		// block the chat response path.
		go deliverNotification(target.URL, body, event)
	}
}

// deliverNotification POSTs the notification body to a single URL.
func deliverNotification(url string, body []byte, event string) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Printf("[notify] build request for %q failed: %v\n", url, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SoulGate-Event", event)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[notify] deliver to %q failed: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		fmt.Printf("[notify] %q returned HTTP %d for event %q\n", url, resp.StatusCode, event)
	}
}
