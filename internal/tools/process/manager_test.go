package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestStartAndList verifies that Start returns a valid process and that List
// contains that process.
func TestStartAndList(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	ctx := context.Background()

	proc, err := mgr.Start(ctx, "sleep 60", "", nil, 0)
	if err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	if proc.ID == "" {
		t.Fatal("Start: returned process has empty ID")
	}
	if proc.PID <= 0 {
		t.Errorf("Start: expected PID > 0, got %d", proc.PID)
	}
	if proc.Status != StatusRunning {
		t.Errorf("Start: expected status %q, got %q", StatusRunning, proc.Status)
	}

	// List should contain the process.
	all := mgr.List()
	if len(all) != 1 {
		t.Fatalf("List: expected 1 process, got %d", len(all))
	}
	if all[0].ID != proc.ID {
		t.Errorf("List: expected id %q, got %q", proc.ID, all[0].ID)
	}

	// Cleanup – kill the sleep so the test doesn't leak a child process.
	if err := mgr.Kill(proc.ID); err != nil {
		t.Errorf("Kill: unexpected error: %v", err)
	}
}

// TestPollOutput starts a process that prints "hello", waits for it to
// complete, then verifies Poll and Log both surface the expected output.
func TestPollOutput(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	ctx := context.Background()

	proc, err := mgr.Start(ctx, "echo hello", "", nil, 10*time.Second)
	if err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	// Wait until the process exits (it should be fast).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		proc.mu.Lock()
		st := proc.Status
		proc.mu.Unlock()
		if st != StatusRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	pollResult, err := mgr.Poll(proc.ID)
	if err != nil {
		t.Fatalf("Poll: unexpected error: %v", err)
	}
	if !strings.Contains(pollResult, "hello") {
		t.Errorf("Poll: expected output to contain \"hello\", got:\n%s", pollResult)
	}

	logResult, err := mgr.Log(proc.ID, 10)
	if err != nil {
		t.Fatalf("Log: unexpected error: %v", err)
	}
	if !strings.Contains(logResult, "hello") {
		t.Errorf("Log: expected output to contain \"hello\", got:\n%s", logResult)
	}
}

// TestKillProcess starts a long-running sleep, kills it, and confirms the
// status transitions to killed.
func TestKillProcess(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	ctx := context.Background()

	proc, err := mgr.Start(ctx, "sleep 60", "", nil, 0)
	if err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	if err := mgr.Kill(proc.ID); err != nil {
		t.Fatalf("Kill: unexpected error: %v", err)
	}

	// Wait a moment for the status to be updated by the background goroutine.
	deadline := time.Now().Add(5 * time.Second)
	var finalStatus ProcessStatus
	for time.Now().Before(deadline) {
		proc.mu.Lock()
		finalStatus = proc.Status
		proc.mu.Unlock()
		if finalStatus != StatusRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if finalStatus == StatusRunning {
		t.Errorf("Kill: process is still running after Kill")
	}

	// Killing an already-killed process should return an error.
	if err := mgr.Kill(proc.ID); err == nil {
		t.Error("Kill: expected error when killing a non-running process, got nil")
	}
}

// TestRingBuffer validates the ring buffer's wrapping behaviour.
func TestRingBuffer(t *testing.T) {
	t.Parallel()

	t.Run("no wrap", func(t *testing.T) {
		rb := newRingBuffer(10)
		rb.Write([]byte("hello")) //nolint:errcheck
		if got := rb.String(); got != "hello" {
			t.Errorf("expected %q, got %q", "hello", got)
		}
	})

	t.Run("exact fill", func(t *testing.T) {
		rb := newRingBuffer(5)
		rb.Write([]byte("abcde")) //nolint:errcheck
		if got := rb.String(); got != "abcde" {
			t.Errorf("expected %q, got %q", "abcde", got)
		}
	})

	t.Run("wrap discards oldest", func(t *testing.T) {
		rb := newRingBuffer(5)
		rb.Write([]byte("abcde")) //nolint:errcheck // fills buffer
		rb.Write([]byte("fg"))    //nolint:errcheck // should overwrite "ab"
		got := rb.String()
		// After write: buf should end with "cdefg" (oldest two "ab" gone).
		if got != "cdefg" {
			t.Errorf("expected %q, got %q", "cdefg", got)
		}
	})

	t.Run("write larger than buffer", func(t *testing.T) {
		rb := newRingBuffer(5)
		rb.Write([]byte("0123456789")) //nolint:errcheck // 10 bytes > size 5
		got := rb.String()
		if got != "56789" {
			t.Errorf("expected %q, got %q", "56789", got)
		}
	})

	t.Run("length tracking", func(t *testing.T) {
		rb := newRingBuffer(8)
		rb.Write([]byte("abc")) //nolint:errcheck
		if rb.Len() != 3 {
			t.Errorf("Len: expected 3, got %d", rb.Len())
		}
		rb.Write([]byte("defghijk")) //nolint:errcheck // wraps
		if rb.Len() != 8 {
			t.Errorf("Len after fill: expected 8, got %d", rb.Len())
		}
	})

	t.Run("multiple small writes ordering", func(t *testing.T) {
		rb := newRingBuffer(6)
		rb.Write([]byte("ab")) //nolint:errcheck
		rb.Write([]byte("cd")) //nolint:errcheck
		rb.Write([]byte("ef")) //nolint:errcheck // fills buffer
		rb.Write([]byte("gh")) //nolint:errcheck // overwrites "ab"
		got := rb.String()
		if got != "cdefgh" {
			t.Errorf("expected %q, got %q", "cdefgh", got)
		}
	})
}

// TestClear starts multiple processes, waits for them to exit, clears them,
// and asserts they are removed.
func TestClear(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	ctx := context.Background()

	const numProcs = 3
	ids := make([]string, numProcs)
	for i := range ids {
		proc, err := mgr.Start(ctx, "echo done", "", nil, 10*time.Second)
		if err != nil {
			t.Fatalf("Start[%d]: unexpected error: %v", i, err)
		}
		ids[i] = proc.ID
	}

	// Wait until all processes have exited.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		allDone := true
		for _, id := range ids {
			p, _ := mgr.Get(id)
			p.mu.Lock()
			st := p.Status
			p.mu.Unlock()
			if st == StatusRunning {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	removed := mgr.Clear()
	if removed != numProcs {
		t.Errorf("Clear: expected %d removed, got %d", numProcs, removed)
	}
	if remaining := mgr.List(); len(remaining) != 0 {
		t.Errorf("Clear: expected 0 processes remaining, got %d", len(remaining))
	}
}

// TestGetUnknown verifies that Get returns an error for an unknown id.
func TestGetUnknown(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	_, err := mgr.Get("nonexistent_proc")
	if err == nil {
		t.Error("Get: expected error for unknown process id, got nil")
	}
}

// TestWriteToStdin verifies that Write forwards input to a running process.
func TestWriteToStdin(t *testing.T) {
	t.Parallel()

	mgr := NewManager()
	ctx := context.Background()

	// A process that reads a line from stdin and echoes it back.
	proc, err := mgr.Start(ctx, "read line && echo $line", "", nil, 10*time.Second)
	if err != nil {
		t.Fatalf("Start: unexpected error: %v", err)
	}

	// Give the shell a moment to start and block on read.
	time.Sleep(150 * time.Millisecond)

	if err := mgr.Write(proc.ID, "hello world\n"); err != nil {
		t.Fatalf("Write: unexpected error: %v", err)
	}

	// Wait for output.
	deadline := time.Now().Add(5 * time.Second)
	var output string
	for time.Now().Before(deadline) {
		result, _ := mgr.Poll(proc.ID)
		if strings.Contains(result, "hello world") {
			output = result
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !strings.Contains(output, "hello world") {
		t.Errorf("WriteToStdin: expected output to contain \"hello world\", got:\n%s", output)
	}
}
