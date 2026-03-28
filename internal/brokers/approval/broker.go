// Package approval provides an async, persistent approval broker for SoulGate.
//
// When the policy engine returns require_approval and no interactive TUI callback
// is registered (e.g., in gateway or CLI non-interactive mode), the broker queues
// the request and blocks the operation until a human approves or denies it via
// the HTTP API or another registered ApprovalHandler.
//
// Requests that are not decided within the timeout window (default 5 minutes) are
// automatically denied and cleaned up.
package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultTimeout is how long an approval request remains pending before
	// it is auto-denied.
	DefaultTimeout = 5 * time.Minute

	// StatusPending means the request has not yet been decided.
	StatusPending = "pending"

	// StatusApproved means a human approved the request.
	StatusApproved = "approved"

	// StatusDenied means a human denied the request.
	StatusDenied = "denied"

	// StatusExpired means the request timed out and was auto-denied.
	StatusExpired = "expired"

	// pendingFile is where in-flight requests are persisted so that a gateway
	// restart can surface them in the UI even if the response channel is gone.
	// Path is relative to the configDir passed to NewBroker.
	pendingFile = "security/approvals.json"
)

// ApprovalRequest represents a single pending approval.
type ApprovalRequest struct {
	ID          string     `json:"id"`
	Action      string     `json:"action"`       // e.g. "files.write"
	Resource    string     `json:"resource"`     // e.g. "./config.yml"
	Reason      string     `json:"reason"`       // why the policy flagged it
	RequestedBy string     `json:"requested_by"` // agent / user ID
	RequestedAt time.Time  `json:"requested_at"`
	Status      string     `json:"status"` // pending | approved | denied | expired
	ExpiresAt   time.Time  `json:"expires_at"`
	DecidedBy   string     `json:"decided_by,omitempty"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`

	// responseCh is closed / sent to by Approve/Deny/expiry goroutine.
	// It is intentionally excluded from JSON serialisation.
	responseCh chan bool `json:"-"`
}

// ApprovalHandler is notified when a new approval request arrives.
// Implementations must not block; use goroutines for any long-running work.
type ApprovalHandler interface {
	OnApprovalRequired(req *ApprovalRequest)
}

// Broker manages async approval requests.
type Broker struct {
	mu       sync.Mutex
	pending  map[string]*ApprovalRequest
	handlers []ApprovalHandler
	dataDir  string // directory where pending requests are persisted
	timeout  time.Duration
	stopCh   chan struct{} // closed to stop the background sweeper
}

// NewBroker creates an ApprovalBroker.  configDir is the .soulgate workspace
// directory used for persisting pending requests across restarts.
func NewBroker(configDir string) *Broker {
	b := &Broker{
		pending: make(map[string]*ApprovalRequest),
		dataDir: configDir,
		timeout: DefaultTimeout,
		stopCh:  make(chan struct{}),
	}
	// Best-effort: load any requests that were pending when the process last
	// exited.  Those requests will not have live response channels; the caller
	// will see them via ListPending but they cannot unblock a waiting goroutine
	// from a previous run.  New requests created in this run work normally.
	b.loadFromDisk()

	// Start the background expiry sweeper.
	go b.expirySweeper()

	return b
}

// WithTimeout overrides the default 5-minute approval window.
func (b *Broker) WithTimeout(d time.Duration) *Broker {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.timeout = d
	return b
}

// AddHandler registers a notification callback for new approval requests.
func (b *Broker) AddHandler(h ApprovalHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

// RequestApproval creates a pending request and blocks until it is approved,
// denied, or the deadline (5 min by default) is reached.
//
// Returns (true, nil) when approved, (false, nil) when denied or expired.
// Returns (false, err) only for internal errors such as a cancelled context.
func (b *Broker) RequestApproval(ctx context.Context, action, resource, reason, requestedBy string) (bool, error) {
	req := b.newRequest(action, resource, reason, requestedBy)

	b.mu.Lock()
	b.pending[req.ID] = req
	handlers := make([]ApprovalHandler, len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.Unlock()

	// Persist before notifying so the UI can show it immediately.
	b.persistToDisk()

	// Notify all registered handlers (non-blocking).
	for _, h := range handlers {
		go h.OnApprovalRequired(req)
	}

	// Block until decided, expired, or caller context cancelled.
	select {
	case approved, ok := <-req.responseCh:
		if !ok {
			// Channel closed without a value means expiry.
			return false, nil
		}
		return approved, nil

	case <-ctx.Done():
		// Caller cancelled — mark denied so it doesn't stay pending forever.
		b.mu.Lock()
		if r, exists := b.pending[req.ID]; exists && r.Status == StatusPending {
			now := time.Now().UTC()
			r.Status = StatusDenied
			r.DecidedBy = "context-cancelled"
			r.DecidedAt = &now
		}
		b.mu.Unlock()
		b.persistToDisk()
		return false, ctx.Err()
	}
}

// Approve marks the request as approved and unblocks any waiting caller.
// decidedBy identifies the human operator who made the decision.
// Returns ErrNotFound if the request does not exist or is no longer pending.
func (b *Broker) Approve(requestID, decidedBy string) error {
	return b.decide(requestID, decidedBy, true)
}

// Deny marks the request as denied and unblocks any waiting caller.
// Returns ErrNotFound if the request does not exist or is no longer pending.
func (b *Broker) Deny(requestID, decidedBy string) error {
	return b.decide(requestID, decidedBy, false)
}

// ListPending returns a snapshot of all currently pending requests.
// The returned slice is safe to read without holding the lock.
func (b *Broker) ListPending() []*ApprovalRequest {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]*ApprovalRequest, 0, len(b.pending))
	for _, req := range b.pending {
		if req.Status == StatusPending {
			// Return a copy so callers cannot race on internal fields.
			cp := *req
			out = append(out, &cp)
		}
	}
	return out
}

// --- internal helpers ---

func (b *Broker) newRequest(action, resource, reason, requestedBy string) *ApprovalRequest {
	now := time.Now().UTC()
	return &ApprovalRequest{
		ID:          uuid.NewString(),
		Action:      action,
		Resource:    resource,
		Reason:      reason,
		RequestedBy: requestedBy,
		RequestedAt: now,
		Status:      StatusPending,
		ExpiresAt:   now.Add(b.timeout),
		responseCh:  make(chan bool, 1), // buffered so sender never blocks
	}
}

func (b *Broker) decide(requestID, decidedBy string, approved bool) error {
	b.mu.Lock()
	req, exists := b.pending[requestID]
	if !exists {
		b.mu.Unlock()
		return fmt.Errorf("approval request not found: %s", requestID)
	}
	if req.Status != StatusPending {
		b.mu.Unlock()
		return fmt.Errorf("approval request %s is already %s", requestID, req.Status)
	}

	now := time.Now().UTC()
	if approved {
		req.Status = StatusApproved
	} else {
		req.Status = StatusDenied
	}
	req.DecidedBy = decidedBy
	req.DecidedAt = &now
	b.mu.Unlock()

	// Send response to the blocked RequestApproval goroutine (non-blocking:
	// the channel is buffered and only written once).
	select {
	case req.responseCh <- approved:
	default:
	}

	b.persistToDisk()
	return nil
}

// expirySweeper runs in the background and auto-denies expired requests.
// The sweep interval is min(timeout/2, 15s) so tests with short timeouts
// do not have to wait 15 seconds for expiry.
func (b *Broker) expirySweeper() {
	b.mu.Lock()
	timeout := b.timeout
	b.mu.Unlock()
	interval := timeout / 2
	if interval < time.Second {
		interval = time.Second // never faster than 1s to avoid hot loops
	}
	if interval > 15*time.Second {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
		}

		now := time.Now().UTC()
		var expired []string

		b.mu.Lock()
		for id, req := range b.pending {
			if req.Status == StatusPending && now.After(req.ExpiresAt) {
				decidedAt := now
				req.Status = StatusExpired
				req.DecidedBy = "system-expiry"
				req.DecidedAt = &decidedAt
				expired = append(expired, id)
			}
		}
		// Collect channels to close outside the lock.
		chans := make([]chan bool, 0, len(expired))
		for _, id := range expired {
			if req, ok := b.pending[id]; ok {
				chans = append(chans, req.responseCh)
			}
		}
		b.mu.Unlock()

		// Close channels to unblock waiting callers (closing signals expiry).
		for _, ch := range chans {
			// Guard against double-close; it is a buffered channel so a send
			// before close would not have blocked, but close can only happen once.
			func(c chan bool) {
				defer func() { recover() }() //nolint:errcheck
				close(c)
			}(ch)
		}

		if len(expired) > 0 {
			b.persistToDisk()
			b.pruneDecided()
		}
	}
}

// Close stops the background sweeper goroutine. Safe to call multiple times.
func (b *Broker) Close() {
	select {
	case <-b.stopCh:
		// already closed
	default:
		close(b.stopCh)
	}
}

// pruneDecided removes decided requests older than 24 hours to bound memory use.
func (b *Broker) pruneDecided() {
	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	b.mu.Lock()
	defer b.mu.Unlock()

	for id, req := range b.pending {
		if req.Status != StatusPending {
			if req.DecidedAt != nil && req.DecidedAt.Before(cutoff) {
				delete(b.pending, id)
			}
		}
	}
}

// persistToDisk writes the current state of all requests (pending + recently
// decided) to disk so the web UI can display them after a gateway restart.
// Errors are logged to stderr but do not propagate — persistence is best-effort.
func (b *Broker) persistToDisk() {
	if b.dataDir == "" {
		return
	}

	b.mu.Lock()
	snapshot := make([]*ApprovalRequest, 0, len(b.pending))
	for _, req := range b.pending {
		cp := *req
		snapshot = append(snapshot, &cp)
	}
	b.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "approval broker: marshal error: %v\n", err)
		return
	}

	path := filepath.Join(b.dataDir, pendingFile)
	// Ensure the parent directory (e.g. security/) exists before writing.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "approval broker: mkdir %s: %v\n", filepath.Dir(path), err)
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "approval broker: write %s: %v\n", path, err)
	}
}

// loadFromDisk restores persisted requests on startup.  Requests that were
// still pending when the process exited are surfaced in ListPending but
// cannot unblock a caller from the previous run.
func (b *Broker) loadFromDisk() {
	if b.dataDir == "" {
		return
	}

	path := filepath.Join(b.dataDir, pendingFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "approval broker: load %s: %v\n", path, err)
		}
		return
	}

	var requests []*ApprovalRequest
	if err := json.Unmarshal(data, &requests); err != nil {
		fmt.Fprintf(os.Stderr, "approval broker: unmarshal %s: %v\n", path, err)
		return
	}

	now := time.Now().UTC()
	for _, req := range requests {
		// Re-create a response channel so the struct is valid even though no
		// goroutine is waiting on it.
		req.responseCh = make(chan bool, 1)

		// Any request that was still pending when we crashed is now considered
		// expired because no goroutine is waiting to receive the decision.
		if req.Status == StatusPending && now.After(req.ExpiresAt) {
			decidedAt := now
			req.Status = StatusExpired
			req.DecidedBy = "system-startup-expiry"
			req.DecidedAt = &decidedAt
		}

		b.pending[req.ID] = req
	}
}
