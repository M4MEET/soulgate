// Package process provides a background process manager for use inside the
// SoulGate agentic loop. It allows long-running shell commands to be started,
// polled, inspected, and killed without blocking the main goroutine.
package process

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ProcessStatus describes the current lifecycle state of a managed process.
type ProcessStatus string

const (
	// StatusRunning means the process is still executing.
	StatusRunning ProcessStatus = "running"
	// StatusExited means the process terminated on its own with exit code 0.
	StatusExited ProcessStatus = "exited"
	// StatusKilled means the process was forcibly terminated by Kill().
	StatusKilled ProcessStatus = "killed"
	// StatusFailed means the process terminated with a non-zero exit code.
	StatusFailed ProcessStatus = "failed"
)

const (
	// ringBufferSize is the maximum number of bytes kept in the rolling output
	// ring buffer per process (64 KiB).
	ringBufferSize = 64 * 1024

	// pollWindowSize is how many bytes Poll returns from the ring buffer (4 KiB).
	pollWindowSize = 4 * 1024

	// defaultTimeout is applied when the caller passes 0 for timeout.
	defaultTimeout = 5 * time.Minute

	// defaultLogLines is the number of lines Log returns when lines == 0.
	defaultLogLines = 50

	// maxProcessEnvVars bounds user-provided env entries per process.
	maxProcessEnvVars = 64

	// maxProcessEnvEntryLen bounds a single KEY=VALUE env entry length.
	maxProcessEnvEntryLen = 4096
)

var blockedSandboxEnvKeys = map[string]struct{}{
	"LD_PRELOAD":            {},
	"LD_LIBRARY_PATH":       {},
	"DYLD_INSERT_LIBRARIES": {},
	"DYLD_LIBRARY_PATH":     {},
	"BASH_ENV":              {},
	"ENV":                   {},
}

// ManagedProcess holds all state for a single supervised process.
type ManagedProcess struct {
	// Public, JSON-serialisable fields.
	ID        string        `json:"id"`
	Command   string        `json:"command"`
	WorkDir   string        `json:"workdir,omitempty"`
	Status    ProcessStatus `json:"status"`
	PID       int           `json:"pid"`
	ExitCode  int           `json:"exit_code"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   *time.Time    `json:"ended_at,omitempty"`

	// Private runtime state.
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bytes.Buffer // captures the full stdout stream
	stderr *bytes.Buffer // captures the full stderr stream
	output *ringBuffer   // combined stdout+stderr ring (last 64 KiB)
	cancel context.CancelFunc
	mu     sync.Mutex

	// stdoutDone / stderrDone are closed when the respective copy goroutines finish.
	stdoutDone chan struct{}
	stderrDone chan struct{}

	// done is closed once the wait goroutine has updated Status/ExitCode/EndedAt.
	done chan struct{}
}

// Manager supervises a set of ManagedProcess instances. It is safe for
// concurrent use.
type Manager struct {
	processes           map[string]*ManagedProcess
	counter             atomic.Int64 // monotonic process counter; avoids map-length races
	workspaceRoot       string
	restrictToWorkspace bool
	baseEnv             []string
	mu                  sync.RWMutex
}

// NewManager returns an initialised Manager ready for use.
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*ManagedProcess),
	}
}

// NewManagerWithWorkspace returns a manager constrained to the given workspace.
// In this mode, workdir is pinned to the workspace boundary and process env is
// built from a minimal allowlist instead of inheriting the full parent env.
func NewManagerWithWorkspace(workspaceRoot string) *Manager {
	mgr := NewManager()

	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return mgr
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = filepath.Clean(root)
	}

	mgr.workspaceRoot = absRoot
	mgr.restrictToWorkspace = true
	mgr.baseEnv = defaultSandboxEnv(absRoot)
	return mgr
}

// Start launches command in a shell subprocess and returns immediately. The
// caller receives a *ManagedProcess whose fields are updated asynchronously as
// the child runs.
//
//   - command  is passed to "sh -c <command>".
//   - workdir  sets the working directory; the current directory is used when empty.
//   - env      is a list of "KEY=VALUE" pairs appended to the inherited environment.
//   - timeout  kills the process automatically after this duration; 0 means 5 minutes.
func (m *Manager) Start(
	ctx context.Context,
	command string,
	workdir string,
	env []string,
	timeout time.Duration,
) (*ManagedProcess, error) {
	if command == "" {
		return nil, fmt.Errorf("process: command must not be empty")
	}

	if timeout <= 0 {
		timeout = defaultTimeout
	}

	resolvedWorkdir, err := m.resolveWorkdir(workdir)
	if err != nil {
		return nil, err
	}

	runtimeEnv, err := m.prepareEnvironment(env)
	if err != nil {
		return nil, err
	}

	// Derive a child context that is cancelled on timeout OR when the parent
	// context is cancelled (e.g. the whole SoulGate session ends).
	childCtx, cancel := context.WithTimeout(ctx, timeout)

	id := fmt.Sprintf("proc_%d", m.counter.Add(1))

	// Build the command.
	cmd := exec.CommandContext(childCtx, "sh", "-c", command) //nolint:gosec // intentional shell execution
	if resolvedWorkdir != "" {
		cmd.Dir = resolvedWorkdir
	}
	if m.restrictToWorkspace {
		cmd.Env = runtimeEnv
	} else if len(runtimeEnv) > 0 {
		// Inherit the parent environment and append caller-provided overrides.
		cmd.Env = append(cmd.Environ(), runtimeEnv...)
	}

	// Set up output capture.
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	ring := newRingBuffer(ringBufferSize)

	// combined is a multi-writer that feeds both the per-stream buffer and the
	// shared ring buffer.
	combined := io.MultiWriter(ring)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("process: failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("process: failed to create stderr pipe: %w", err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("process: failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("process: failed to start %q: %w", command, err)
	}

	proc := &ManagedProcess{
		ID:         id,
		Command:    command,
		WorkDir:    resolvedWorkdir,
		Status:     StatusRunning,
		PID:        cmd.Process.Pid,
		StartedAt:  time.Now().UTC(),
		cmd:        cmd,
		stdin:      stdinPipe,
		stdout:     stdoutBuf,
		stderr:     stderrBuf,
		output:     ring,
		cancel:     cancel,
		stdoutDone: make(chan struct{}),
		stderrDone: make(chan struct{}),
		done:       make(chan struct{}),
	}

	// Drain stdout asynchronously into both the full buffer and the ring.
	go func() {
		defer close(proc.stdoutDone)
		io.Copy(io.MultiWriter(stdoutBuf, combined), stdoutPipe) //nolint:errcheck
	}()

	// Drain stderr asynchronously.
	go func() {
		defer close(proc.stderrDone)
		io.Copy(io.MultiWriter(stderrBuf, combined), stderrPipe) //nolint:errcheck
	}()

	// Wait for the process to exit and update status fields.
	go func() {
		defer close(proc.done)

		// Wait for both pipes to be fully drained before calling cmd.Wait so
		// we don't lose any trailing output.
		<-proc.stdoutDone
		<-proc.stderrDone

		waitErr := cmd.Wait()

		now := time.Now().UTC()

		proc.mu.Lock()
		defer proc.mu.Unlock()

		proc.EndedAt = &now

		if waitErr == nil {
			proc.ExitCode = 0
			if proc.Status == StatusRunning {
				proc.Status = StatusExited
			}
			return
		}

		// Distinguish killed vs failed.
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			proc.ExitCode = exitErr.ExitCode()
			if proc.Status == StatusRunning {
				// ExitCode -1 typically indicates a signal kill.
				if proc.ExitCode == -1 {
					proc.Status = StatusKilled
				} else {
					proc.Status = StatusFailed
				}
			}
			return
		}

		// Context cancellation surfaces as a non-ExitError.
		if proc.Status == StatusRunning {
			proc.Status = StatusKilled
		}
	}()

	m.mu.Lock()
	m.processes[id] = proc
	m.mu.Unlock()

	return proc, nil
}

func (m *Manager) resolveWorkdir(workdir string) (string, error) {
	trimmed := strings.TrimSpace(workdir)
	if !m.restrictToWorkspace {
		return trimmed, nil
	}

	root := m.workspaceRoot
	if root == "" {
		return "", fmt.Errorf("process: workspace root not configured")
	}

	if trimmed == "" || trimmed == "." {
		return root, nil
	}

	cleaned := filepath.Clean(trimmed)
	candidate := cleaned
	if !filepath.IsAbs(cleaned) {
		candidate = filepath.Join(root, cleaned)
	}

	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("process: invalid workdir %q: %w", workdir, err)
	}

	rel, err := filepath.Rel(root, absCandidate)
	if err != nil {
		return "", fmt.Errorf("process: failed to validate workdir %q: %w", workdir, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("process: workdir %q is outside workspace", workdir)
	}

	return absCandidate, nil
}

func (m *Manager) prepareEnvironment(env []string) ([]string, error) {
	if len(env) > maxProcessEnvVars {
		return nil, fmt.Errorf("process: too many environment variables (max %d)", maxProcessEnvVars)
	}

	parsed := make([]string, 0, len(env))
	for _, entry := range env {
		if len(entry) > maxProcessEnvEntryLen {
			return nil, fmt.Errorf("process: environment entry too large")
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("process: invalid environment entry %q", entry)
		}
		if !validProcessEnvKey(key) {
			return nil, fmt.Errorf("process: invalid environment key %q", key)
		}

		upperKey := strings.ToUpper(key)
		if m.restrictToWorkspace {
			if _, blocked := blockedSandboxEnvKeys[upperKey]; blocked {
				return nil, fmt.Errorf("process: environment key %q is not allowed", key)
			}
		}

		parsed = append(parsed, key+"="+value)
	}

	if !m.restrictToWorkspace {
		return parsed, nil
	}

	return mergeEnv(m.baseEnv, parsed), nil
}

func defaultSandboxEnv(workspaceRoot string) []string {
	env := []string{
		"HOME=" + workspaceRoot,
	}
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "TERM", "TMPDIR", "TMP", "TEMP"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func mergeEnv(base []string, overrides []string) []string {
	merged := make([]string, len(base))
	copy(merged, base)
	for _, entry := range overrides {
		key, _, _ := strings.Cut(entry, "=")
		replaced := false
		for i, existing := range merged {
			existingKey, _, _ := strings.Cut(existing, "=")
			if existingKey == key {
				merged[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, entry)
		}
	}
	return merged
}

func validProcessEnvKey(key string) bool {
	for i, ch := range key {
		if i == 0 {
			if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && ch != '_' {
				return false
			}
			continue
		}
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
			return false
		}
	}
	return true
}

// List returns a snapshot of all managed processes. The slice is safe to read
// after return (each element is the original pointer; callers must not mutate
// fields that are guarded by the process mutex).
func (m *Manager) List() []*ManagedProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	procs := make([]*ManagedProcess, 0, len(m.processes))
	for _, p := range m.processes {
		procs = append(procs, p)
	}
	return procs
}

// Get returns the process identified by id or an error if no such process exists.
func (m *Manager) Get(id string) (*ManagedProcess, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.processes[id]
	if !ok {
		return nil, fmt.Errorf("process: no process with id %q", id)
	}
	return p, nil
}

// Poll returns the most recent output (up to 4 KiB) from the ring buffer
// together with a one-line status summary.
func (m *Manager) Poll(id string) (string, error) {
	proc, err := m.Get(id)
	if err != nil {
		return "", err
	}

	proc.mu.Lock()
	status := proc.Status
	pid := proc.PID
	exitCode := proc.ExitCode
	proc.mu.Unlock()

	full := proc.output.String()

	// Return at most pollWindowSize bytes from the tail.
	if len(full) > pollWindowSize {
		full = full[len(full)-pollWindowSize:]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Process %s (pid=%d) status=%s", id, pid, status))
	if status != StatusRunning {
		sb.WriteString(fmt.Sprintf(" exit_code=%d", exitCode))
	}
	sb.WriteString("\n")
	if full != "" {
		sb.WriteString("--- recent output ---\n")
		sb.WriteString(full)
	} else {
		sb.WriteString("(no output yet)")
	}
	return sb.String(), nil
}

// Log returns the last n lines of the combined stdout+stderr output. If n is
// 0 the default of 50 lines is used.
func (m *Manager) Log(id string, lines int) (string, error) {
	proc, err := m.Get(id)
	if err != nil {
		return "", err
	}

	if lines <= 0 {
		lines = defaultLogLines
	}

	// Collect full output from both streams.
	proc.mu.Lock()
	combined := proc.stdout.String() + proc.stderr.String()
	proc.mu.Unlock()

	// Split into lines and take the tail.
	all := strings.Split(combined, "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return strings.Join(all, "\n"), nil
}

// Write sends input to the process's stdin. It returns an error if the process
// is not running or if the write fails.
func (m *Manager) Write(id string, input string) error {
	proc, err := m.Get(id)
	if err != nil {
		return err
	}

	proc.mu.Lock()
	status := proc.Status
	stdinPipe := proc.stdin
	proc.mu.Unlock()

	if status != StatusRunning {
		return fmt.Errorf("process: process %q is not running (status=%s)", id, status)
	}
	if stdinPipe == nil {
		return fmt.Errorf("process: stdin pipe for %q is not available", id)
	}

	if _, err := io.WriteString(stdinPipe, input); err != nil {
		return fmt.Errorf("process: failed to write to stdin of %q: %w", id, err)
	}
	return nil
}

// Kill terminates the process identified by id. It first cancels the process
// context (which sends SIGTERM on Unix systems when used with exec.CommandContext),
// then delivers SIGKILL directly if the process has not exited within 3 seconds.
func (m *Manager) Kill(id string) error {
	proc, err := m.Get(id)
	if err != nil {
		return err
	}

	proc.mu.Lock()
	status := proc.Status
	cancelFn := proc.cancel
	cmd := proc.cmd
	proc.mu.Unlock()

	if status != StatusRunning {
		return fmt.Errorf("process: process %q is not running (status=%s)", id, status)
	}

	// Cancel the context; exec.CommandContext will send SIGKILL once the
	// context is done (Go 1.20+ supports WaitDelay, but for maximum
	// compatibility we also send SIGKILL manually below).
	cancelFn()

	// Give the process a grace period then SIGKILL.
	done := make(chan struct{})
	go func() {
		select {
		case <-proc.done:
		case <-time.After(3 * time.Second):
			if cmd.Process != nil {
				cmd.Process.Signal(syscall.SIGKILL) //nolint:errcheck
			}
		}
		close(done)
	}()
	<-done

	// Mark as killed if the wait goroutine hasn't already updated the status.
	proc.mu.Lock()
	if proc.Status == StatusRunning {
		proc.Status = StatusKilled
	}
	proc.mu.Unlock()

	return nil
}

// Clear removes all processes that have reached a terminal state (exited,
// killed, or failed). It returns the number of entries removed.
func (m *Manager) Clear() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var removed int
	for id, p := range m.processes {
		p.mu.Lock()
		terminal := p.Status != StatusRunning
		p.mu.Unlock()

		if terminal {
			delete(m.processes, id)
			removed++
		}
	}
	return removed
}
