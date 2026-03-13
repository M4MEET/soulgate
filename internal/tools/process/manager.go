// Package process provides a background process manager for use inside the
// SoulGate agentic loop. It allows long-running shell commands to be started,
// polled, inspected, and killed without blocking the main goroutine.
package process

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
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
)

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
	processes map[string]*ManagedProcess
	counter   atomic.Int64 // monotonic process counter; avoids map-length races
	mu        sync.RWMutex
}

// NewManager returns an initialised Manager ready for use.
func NewManager() *Manager {
	return &Manager{
		processes: make(map[string]*ManagedProcess),
	}
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

	// Derive a child context that is cancelled on timeout OR when the parent
	// context is cancelled (e.g. the whole SoulGate session ends).
	childCtx, cancel := context.WithTimeout(ctx, timeout)

	id := fmt.Sprintf("proc_%d", m.counter.Add(1))

	// Build the command.
	cmd := exec.CommandContext(childCtx, "sh", "-c", command) //nolint:gosec // intentional shell execution
	if workdir != "" {
		cmd.Dir = workdir
	}
	if len(env) > 0 {
		// Inherit the parent environment and append caller-provided overrides.
		cmd.Env = append(cmd.Environ(), env...)
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
		WorkDir:    workdir,
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
