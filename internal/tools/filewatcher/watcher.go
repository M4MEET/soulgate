// Package filewatcher provides a file-system event watcher tool for the
// SoulGate agentic loop.  Agents can ask the runtime to monitor one or more
// paths (files or directories) for create/modify/delete events.  When a
// matching change is detected a caller-supplied callback is invoked so that
// the orchestrator can trigger follow-up AI actions.
//
// Design decisions:
//   - Each logical watcher owns its own *fsnotify.Watcher so that watchers
//     can be stopped individually without disturbing others.
//   - Rapid save sequences (e.g., editor atomic writes) are coalesced by a
//     500 ms debounce timer; a fresh timer replaces the previous one on every
//     event so the callback fires once per burst, not once per save.
//   - Pattern matching is done with filepath.Match so agents can pass standard
//     glob patterns ("*.go", "*.json", …).
//   - Recursive watching walks the directory tree at Start time and adds every
//     sub-directory to the underlying fsnotify watcher.  New sub-directories
//     created after Start are added dynamically on the first Create event for
//     that directory.
package filewatcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// debounceDelay is the quiet period after the last event before the callback
// fires.  Editors such as vim/nvim write a temporary file then rename it, which
// produces two events in rapid succession; 500 ms is comfortably above that.
const debounceDelay = 500 * time.Millisecond

// EventType classifies a file-system change.
type EventType string

const (
	// EventCreate is emitted when a new file or directory is created.
	EventCreate EventType = "create"
	// EventModify is emitted when an existing file's content changes.
	EventModify EventType = "modify"
	// EventDelete is emitted when a file or directory is removed.
	EventDelete EventType = "delete"
)

// Callback is the function the Manager calls when a debounced file event
// arrives.  Parameters are the watcher ID, event type, and the affected path.
type Callback func(watchID string, event EventType, path string)

// Watcher holds all state for a single active watch registration.
type Watcher struct {
	// Public fields — safe to read without holding the manager lock once
	// the watcher has been started.
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Pattern   string    `json:"pattern"`
	Action    string    `json:"action"`
	Recursive bool      `json:"recursive"`
	CreatedAt time.Time `json:"created_at"`

	// Events is the total number of debounced callbacks fired so far.
	// Updated atomically so callers can snapshot without locking.
	Events atomic.Int64

	// Private state — guarded internally.
	fsWatcher *fsnotify.Watcher
	cancel    context.CancelFunc
}

// WatcherInfo is a snapshot of a Watcher's state suitable for serialisation.
type WatcherInfo struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Pattern   string    `json:"pattern"`
	Action    string    `json:"action"`
	Recursive bool      `json:"recursive"`
	Events    int64     `json:"events"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager supervises a set of Watcher instances.  It is safe for concurrent
// use.
type Manager struct {
	mu       sync.RWMutex
	watchers map[string]*Watcher
	nextID   atomic.Int64
	callback Callback
}

// NewManager constructs a Manager.  callback is invoked (in its own goroutine)
// each time a debounced file event matches a watcher's pattern.  callback must
// be non-nil.
func NewManager(callback Callback) *Manager {
	if callback == nil {
		panic("filewatcher: callback must not be nil")
	}
	return &Manager{
		watchers: make(map[string]*Watcher),
		callback: callback,
	}
}

// Start registers a new watcher and begins receiving events.
//
//   - path      — file or directory to watch.  Relative paths are accepted;
//     they are resolved against the process working directory.
//   - pattern   — glob pattern applied to the base name of changed files
//     (e.g., "*.go").  Empty string matches every file.
//   - action    — free-text description of what the AI should do on change;
//     this is stored and returned in List/Callback so the orchestrator can
//     forward it as a prompt.
//   - recursive — when true, all sub-directories under path are also watched.
//
// Returns the watcher's ID (e.g., "watch_1") or an error.
func (m *Manager) Start(ctx context.Context, path, pattern, action string, recursive bool) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("filewatcher: path must not be empty")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("filewatcher: cannot resolve path %q: %w", path, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("filewatcher: path %q is not accessible: %w", absPath, err)
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return "", fmt.Errorf("filewatcher: failed to create OS watcher: %w", err)
	}

	if err := addToWatcher(fsw, absPath, recursive); err != nil {
		fsw.Close() //nolint:errcheck
		return "", err
	}

	id := fmt.Sprintf("watch_%d", m.nextID.Add(1))
	watchCtx, cancel := context.WithCancel(ctx)

	w := &Watcher{
		ID:        id,
		Path:      absPath,
		Pattern:   pattern,
		Action:    action,
		Recursive: recursive,
		CreatedAt: time.Now().UTC(),
		fsWatcher: fsw,
		cancel:    cancel,
	}

	m.mu.Lock()
	m.watchers[id] = w
	m.mu.Unlock()

	go m.runWatcher(watchCtx, w)

	return id, nil
}

// List returns a snapshot of all active watchers.
func (m *Manager) List() []WatcherInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]WatcherInfo, 0, len(m.watchers))
	for _, w := range m.watchers {
		infos = append(infos, WatcherInfo{
			ID:        w.ID,
			Path:      w.Path,
			Pattern:   w.Pattern,
			Action:    w.Action,
			Recursive: w.Recursive,
			Events:    w.Events.Load(),
			CreatedAt: w.CreatedAt,
		})
	}
	return infos
}

// Stop terminates the watcher with the given ID and releases its resources.
// Returns an error if no such watcher exists.
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	w, ok := m.watchers[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("filewatcher: no watcher with id %q", id)
	}
	delete(m.watchers, id)
	m.mu.Unlock()

	w.cancel()
	return w.fsWatcher.Close()
}

// ReplaceCallback atomically replaces the callback function.  This is used by
// the orchestrator to swap in a closure that references the fully-constructed
// Orchestrator after NewOrchestrator completes.  cb must not be nil.
func (m *Manager) ReplaceCallback(cb Callback) {
	if cb == nil {
		panic("filewatcher: replacement callback must not be nil")
	}
	m.mu.Lock()
	m.callback = cb
	m.mu.Unlock()
}

// StopAll terminates every active watcher.  It is safe to call concurrently
// and from a defer.
func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.watchers))
	for id := range m.watchers {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		m.Stop(id) //nolint:errcheck
	}
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// runWatcher is the per-Watcher event loop.  It deduplicates rapid events
// with a debounce timer and then invokes the manager callback.
func (m *Manager) runWatcher(ctx context.Context, w *Watcher) {
	var (
		debounceTimer *time.Timer
		pendingEvent  EventType
		pendingPath   string
		mu            sync.Mutex // protects pending* and the timer
	)

	fire := func() {
		mu.Lock()
		ev := pendingEvent
		p := pendingPath
		mu.Unlock()

		if ev == "" {
			return
		}
		w.Events.Add(1)
		m.callback(w.ID, ev, p)
	}

	for {
		select {
		case <-ctx.Done():
			mu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			mu.Unlock()
			return

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// Transient OS errors (permission denied on rename temp files, etc.)
			// are non-fatal; log nothing here because filewatcher has no logger
			// dependency — the caller can wrap the callback for that.
			_ = err

		case evt, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			evtPath := evt.Name
			base := filepath.Base(evtPath)

			// Pattern filter — empty pattern matches all files.
			if w.Pattern != "" {
				matched, err := filepath.Match(w.Pattern, base)
				if err != nil || !matched {
					continue
				}
			}

			evType := fsnotifyOpToEventType(evt.Op)
			if evType == "" {
				continue
			}

			// Dynamically add new sub-directories for recursive watchers.
			if w.Recursive && (evt.Op&fsnotify.Create != 0) {
				if fi, err := os.Stat(evtPath); err == nil && fi.IsDir() {
					_ = w.fsWatcher.Add(evtPath)
				}
			}

			// Debounce: reset the timer on every qualifying event so that a
			// burst of saves results in exactly one callback after the storm
			// subsides.
			mu.Lock()
			pendingEvent = evType
			pendingPath = evtPath
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, fire)
			mu.Unlock()
		}
	}
}

// addToWatcher registers absPath with fsw.  When recursive is true every
// sub-directory is added as well.
func addToWatcher(fsw *fsnotify.Watcher, absPath string, recursive bool) error {
	if err := fsw.Add(absPath); err != nil {
		return fmt.Errorf("filewatcher: cannot watch %q: %w", absPath, err)
	}

	if !recursive {
		return nil
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		return nil // already added; stat failure is non-fatal
	}
	if !fi.IsDir() {
		return nil // single file — nothing more to add
	}

	return filepath.WalkDir(absPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// Permission errors on individual entries are skipped silently.
			return nil
		}
		if p == absPath {
			return nil // already added above
		}
		if d.IsDir() {
			if addErr := fsw.Add(p); addErr != nil {
				// Non-fatal — skip inaccessible directories.
				return nil
			}
		}
		return nil
	})
}

// fsnotifyOpToEventType maps an fsnotify.Op bitmask to the coarsest EventType.
// Returns "" for operations we do not surface (chmod, rename without create).
func fsnotifyOpToEventType(op fsnotify.Op) EventType {
	switch {
	case op&fsnotify.Create != 0:
		return EventCreate
	case op&fsnotify.Write != 0:
		return EventModify
	case op&fsnotify.Remove != 0, op&fsnotify.Rename != 0:
		return EventDelete
	default:
		return ""
	}
}
