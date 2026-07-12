package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
)

// HeartbeatStatus is a point-in-time snapshot of the heartbeat subsystem.
type HeartbeatStatus struct {
	Enabled    bool      `json:"enabled"`
	Running    bool      `json:"running"`
	Interval   string    `json:"interval"`
	LastRun    time.Time `json:"last_run,omitempty"`
	NextRun    time.Time `json:"next_run,omitempty"`
	LastResult string    `json:"last_result,omitempty"`
	RunCount   int       `json:"run_count"`
}

// defaultHeartbeatInstructions is used when no HEARTBEAT.md file is found.
const defaultHeartbeatInstructions = `# Heartbeat Check

On each heartbeat, check:
1. Any running background agents that completed or failed
2. Any file watchers that triggered
3. System health (disk space, memory)
4. Any pending approval requests
5. Cron jobs that failed

If everything is fine, respond with just "OK".
If something needs attention, describe it briefly.`

// heartbeatTimeout is the maximum time a single heartbeat run may take.
const heartbeatTimeout = 60 * time.Second

// Heartbeat periodically wakes the AI agent to check for things that need
// attention. It is embedded inside the Orchestrator and shares its Run method
// so it benefits from full tool access and audit logging.
//
// The goroutine lifecycle follows the pattern used elsewhere in the codebase:
// a stopCh channel signals the ticker goroutine and Start/Stop are idempotent.
type Heartbeat struct {
	mu         sync.Mutex
	config     config.HeartbeatConfig
	orch       *Orchestrator
	ticker     *time.Ticker
	stopCh     chan struct{}
	running    bool
	lastRun    time.Time
	lastResult string
	runCount   int
	callback   func(message string) // called when the AI has something to report
}

// NewHeartbeat creates a Heartbeat that drives the given orchestrator.
func NewHeartbeat(orch *Orchestrator, cfg config.HeartbeatConfig) *Heartbeat {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Minute
	}
	if cfg.PromptFile == "" {
		cfg.PromptFile = ".soulgate/HEARTBEAT.md"
	}
	hb := &Heartbeat{
		config: cfg,
		orch:   orch,
		stopCh: make(chan struct{}),
	}
	hb.loadState()
	return hb
}

type heartbeatState struct {
	LastRun    time.Time `json:"last_run"`
	LastResult string    `json:"last_result"`
	RunCount   int       `json:"run_count"`
}

func (h *Heartbeat) statePath() string {
	if h.orch == nil || h.orch.workspace == nil {
		return ""
	}
	return filepath.Join(h.orch.workspace.ConfigDir, "state", "heartbeat.json")
}

func (h *Heartbeat) saveState() {
	path := h.statePath()
	if path == "" {
		return
	}
	data, _ := json.Marshal(heartbeatState{
		LastRun:    h.lastRun,
		LastResult: h.lastResult,
		RunCount:   h.runCount,
	})
	_ = os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, data, 0600)
}

func (h *Heartbeat) loadState() {
	path := h.statePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s heartbeatState
	if json.Unmarshal(data, &s) == nil {
		h.lastRun = s.LastRun
		h.lastResult = s.LastResult
		h.runCount = s.RunCount
	}
}

// SetCallback registers a function that will be called whenever the heartbeat
// produces a non-OK response. The callback may be invoked from a background
// goroutine so it must be concurrent-safe.
func (h *Heartbeat) SetCallback(fn func(message string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callback = fn
}

// Start launches the periodic ticker goroutine. Calling Start on an already
// running Heartbeat is a no-op.
func (h *Heartbeat) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return
	}

	h.ticker = time.NewTicker(h.config.Interval)
	h.stopCh = make(chan struct{})
	h.running = true

	go h.loop()
}

// Stop signals the ticker goroutine to exit and waits for it to acknowledge.
// Calling Stop on a non-running Heartbeat is a no-op.
func (h *Heartbeat) Stop() {
	h.mu.Lock()

	if !h.running {
		h.mu.Unlock()
		return
	}

	ticker := h.ticker
	stop := h.stopCh
	h.running = false
	h.mu.Unlock()

	close(stop)
	ticker.Stop()
}

// RunNow triggers a heartbeat immediately, bypassing the ticker. This is safe
// to call concurrently with the ticker goroutine. The raw AI response is
// returned to the caller.
func (h *Heartbeat) RunNow() (string, error) {
	return h.run()
}

// RecordResponse stores an explicit model response from the heartbeat_respond
// tool. status "attention" forwards the message to the notification callback;
// "ok" just records a clean check.
func (h *Heartbeat) RecordResponse(status, message string) {
	h.mu.Lock()
	if status == "ok" {
		h.lastResult = "OK"
	} else {
		h.lastResult = message
	}
	cb := h.callback
	h.saveState()
	h.mu.Unlock()

	if status == "attention" && message != "" && cb != nil {
		cb(message)
	}
}

// Status returns a point-in-time snapshot of the heartbeat state.
func (h *Heartbeat) Status() HeartbeatStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	s := HeartbeatStatus{
		Enabled:    h.config.Enabled,
		Running:    h.running,
		Interval:   h.config.Interval.String(),
		LastRun:    h.lastRun,
		LastResult: h.lastResult,
		RunCount:   h.runCount,
	}

	if h.running && !h.lastRun.IsZero() {
		s.NextRun = h.lastRun.Add(h.config.Interval)
	} else if h.running {
		s.NextRun = time.Now().Add(h.config.Interval)
	}

	return s
}

// loop is the background goroutine that fires the heartbeat on every tick.
func (h *Heartbeat) loop() {
	for {
		select {
		case <-h.stopCh:
			return
		case <-h.ticker.C:
			if _, err := h.run(); err != nil {
				fmt.Fprintf(os.Stderr, "[heartbeat] run error: %v\n", err)
			}
		}
	}
}

// run executes one heartbeat cycle: reads the prompt file, calls the
// orchestrator, interprets the response, and notifies if needed.
// It does NOT touch h.running so it can be called by both the ticker
// goroutine and RunNow without deadlocking.
func (h *Heartbeat) run() (string, error) {
	instructions := h.loadInstructions()

	prompt := "[Heartbeat check] " + instructions +
		"\nCheck if anything needs the user's attention. " +
		"Prefer the heartbeat_respond tool: status 'ok' if all clear, " +
		"or 'attention' with a brief message. " +
		"Otherwise respond with just 'OK', or describe the issue briefly."

	ctx, cancel := context.WithTimeout(context.Background(), heartbeatTimeout)
	defer cancel()

	result, err := h.orch.runHeartbeat(ctx, prompt)

	h.mu.Lock()
	h.lastRun = time.Now()
	h.runCount++
	if err != nil {
		h.lastResult = fmt.Sprintf("error: %v", err)
	} else {
		h.lastResult = result
	}
	cb := h.callback
	h.saveState()
	h.mu.Unlock()

	// Log to audit regardless of outcome.
	h.logAudit(result, err)

	if err != nil {
		return "", err
	}

	// Only invoke the callback when the model signals something actionable.
	// Treat blank, whitespace-only, and "OK" (case-insensitive) as silent.
	trimmed := strings.TrimSpace(result)
	if trimmed != "" && !strings.EqualFold(trimmed, "ok") && cb != nil {
		cb(trimmed)
	}

	return result, nil
}

// loadInstructions reads the HEARTBEAT.md file from the workspace config
// directory. If the file does not exist or cannot be read, the built-in
// default instructions are returned.
func (h *Heartbeat) loadInstructions() string {
	promptPath := h.config.PromptFile
	if !filepath.IsAbs(promptPath) {
		promptPath = filepath.Join(h.orch.workspace.Root, promptPath)
	}

	data, err := os.ReadFile(promptPath)
	if err != nil {
		return defaultHeartbeatInstructions
	}

	if trimmed := strings.TrimSpace(string(data)); trimmed != "" {
		return trimmed
	}
	return defaultHeartbeatInstructions
}

// logAudit records the heartbeat execution to the audit log.
// Errors here are non-fatal — a failed audit write must not prevent the
// heartbeat from notifying the user.
func (h *Heartbeat) logAudit(result string, runErr error) {
	ctx, cancel := auditContext()
	defer cancel()

	event := audit.NewEvent(audit.EventRunComplete, audit.CategoryRun).
		WithSessionID(h.orch.session.ID).
		WithMetadata("source", "heartbeat").
		WithMetadata("run_count", h.runCount)

	if runErr != nil {
		event = event.WithError(runErr)
	} else {
		short := result
		if len(short) > 200 {
			short = short[:200] + "..."
		}
		event = event.WithMetadata("result", short).
			WithStatus(audit.StatusSuccess)
	}

	if err := h.orch.audit.Log(ctx, event); err != nil {
		fmt.Fprintf(os.Stderr, "[heartbeat] audit log error: %v\n", err)
	}
}
