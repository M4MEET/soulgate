// Package browser provides a headless Chrome browser manager for the SoulGate
// tool system. It uses the chromedp library (Chrome DevTools Protocol) to
// automate a real browser — no CGO required.
//
// The Manager follows the lazy-initialization pattern used throughout
// SoulGate: Chrome is only launched the first time a browser tool is actually
// called. Callers must call Close() when the session ends to release the
// Chrome process and its context.
package browser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	// operationTimeout is the per-operation deadline applied when executing
	// any browser action. This prevents a single hanging tab from blocking the
	// entire agentic loop indefinitely.
	operationTimeout = 30 * time.Second

	// maxPageTextBytes caps the text content returned from browser_open and
	// browser_html to avoid token explosion in the model context window.
	maxPageTextBytes = 32 * 1024 // 32 KB
)

// Manager holds a long-lived headless Chrome instance that is shared across
// all browser tool calls within a single Orchestrator session.
//
// Concurrency: all exported methods are safe to call concurrently. The
// internal mu lock serialises access to the browser context so that
// simultaneous tool calls do not corrupt the browser state.
type Manager struct {
	mu     sync.Mutex
	ctx    context.Context    // chromedp browser context (non-nil after first use)
	cancel context.CancelFunc // cancels the browser context on Close
}

// NewManager creates a Manager. Chrome is NOT started yet — it will be
// launched lazily on the first tool call.
func NewManager() *Manager {
	return &Manager{}
}

// Close tears down the headless Chrome instance (if it was started) and
// releases all associated resources. It is safe to call Close more than once.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
		m.ctx = nil
	}
}

// browserCtx returns the shared browser context, starting Chrome on first
// call. The caller must hold m.mu when calling this method.
func (m *Manager) ensureBrowser() error {
	if m.ctx != nil {
		// Already running — verify it is still alive.
		select {
		case <-m.ctx.Done():
			// Browser context was cancelled externally; restart it.
			m.cancel = nil
			m.ctx = nil
		default:
			return nil
		}
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("mute-audio", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	// Warm up: run a no-op action to ensure Chrome starts and is reachable
	// before we return to the caller. Without this, the first real action
	// sometimes times out if Chrome takes longer than operationTimeout to start.
	warmCtx, warmCancel := context.WithTimeout(browserCtx, operationTimeout)
	defer warmCancel()

	if err := chromedp.Run(warmCtx); err != nil {
		browserCancel()
		allocCancel()
		return fmt.Errorf("browser: failed to start Chrome: %w", err)
	}

	// Store a combined cancel that cleans up both the browser and the allocator.
	m.cancel = func() {
		browserCancel()
		allocCancel()
	}
	m.ctx = browserCtx

	return nil
}

// run acquires the lock, ensures Chrome is running, builds an operation
// context with a deadline, and executes the provided chromedp actions.
func (m *Manager) run(actions ...chromedp.Action) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureBrowser(); err != nil {
		return err
	}

	opCtx, cancel := context.WithTimeout(m.ctx, operationTimeout)
	defer cancel()

	return chromedp.Run(opCtx, actions...)
}

// truncate returns s unchanged if it is within maxPageTextBytes, otherwise it
// appends a truncation notice and returns the capped prefix.
func truncate(s string) string {
	if len(s) <= maxPageTextBytes {
		return s
	}
	return s[:maxPageTextBytes] + "\n\n[truncated — content exceeds 32 KB display limit]"
}
