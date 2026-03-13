package cron

import (
	"context"
	"os"
	"testing"
	"time"
)

// --------------------------------------------------------------------------
// Scheduler tests
// --------------------------------------------------------------------------

func TestAddAndList(t *testing.T) {
	sched := newTestScheduler(t)

	job, err := sched.Add("daily-report", "1h", "Summarise today's activity", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add: unexpected error: %v", err)
	}
	if job.ID == "" {
		t.Fatal("Add: expected non-empty job ID")
	}
	if job.Name != "daily-report" {
		t.Errorf("Add: name = %q, want %q", job.Name, "daily-report")
	}
	if job.Status != JobActive {
		t.Errorf("Add: status = %q, want %q", job.Status, JobActive)
	}
	if job.NextRun == nil {
		t.Fatal("Add: NextRun should not be nil")
	}

	jobs := sched.List()
	if len(jobs) != 1 {
		t.Fatalf("List: got %d jobs, want 1", len(jobs))
	}
	if jobs[0].ID != job.ID {
		t.Errorf("List: job ID mismatch: got %q, want %q", jobs[0].ID, job.ID)
	}
}

func TestAutoIncrementIDs(t *testing.T) {
	sched := newTestScheduler(t)

	j1, err := sched.Add("job-1", "5m", "task 1", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add job-1: %v", err)
	}
	j2, err := sched.Add("job-2", "10m", "task 2", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add job-2: %v", err)
	}
	if j1.ID == j2.ID {
		t.Errorf("expected distinct IDs, both are %q", j1.ID)
	}
	// IDs should follow the "cron_N" pattern.
	if j1.ID != "cron_1" {
		t.Errorf("first job ID = %q, want %q", j1.ID, "cron_1")
	}
	if j2.ID != "cron_2" {
		t.Errorf("second job ID = %q, want %q", j2.ID, "cron_2")
	}
}

func TestRemoveJob(t *testing.T) {
	sched := newTestScheduler(t)

	job, err := sched.Add("to-remove", "1h", "some task", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Remove(job.ID); err != nil {
		t.Fatalf("Remove: unexpected error: %v", err)
	}

	jobs := sched.List()
	if len(jobs) != 0 {
		t.Errorf("List after Remove: got %d jobs, want 0", len(jobs))
	}

	// A second Remove should return an error.
	if err := sched.Remove(job.ID); err == nil {
		t.Error("Remove: expected error for unknown ID, got nil")
	}
}

func TestPauseResume(t *testing.T) {
	sched := newTestScheduler(t)

	job, err := sched.Add("pausable", "1h", "some task", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := sched.Pause(job.ID); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	got, _ := sched.Get(job.ID)
	if got.Status != JobPaused {
		t.Errorf("after Pause: status = %q, want %q", got.Status, JobPaused)
	}

	// Pausing again should still work (idempotent is acceptable; here we just
	// verify it does not panic – the scheduler does return an error for an
	// already-paused job, which is also acceptable).
	_ = sched.Pause(job.ID)

	if err := sched.Resume(job.ID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	got, _ = sched.Get(job.ID)
	if got.Status != JobActive {
		t.Errorf("after Resume: status = %q, want %q", got.Status, JobActive)
	}

	// Resuming an active job should return an error.
	if err := sched.Resume(job.ID); err == nil {
		t.Error("Resume active job: expected error, got nil")
	}
}

func TestGetUnknownID(t *testing.T) {
	sched := newTestScheduler(t)
	if _, err := sched.Get("cron_999"); err == nil {
		t.Error("Get: expected error for unknown ID, got nil")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	s1 := NewScheduler(dir)
	job, err := s1.Add("persisted", "30m", "hello", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Create a second scheduler pointing to the same directory.
	s2 := NewScheduler(dir)
	loaded, err := s2.Get(job.ID)
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if loaded.Name != job.Name {
		t.Errorf("reloaded name = %q, want %q", loaded.Name, job.Name)
	}
}

func TestOneShotExecution(t *testing.T) {
	sched := newTestScheduler(t)

	executed := make(chan string, 1)
	sched.SetExecutor(func(ctx context.Context, task string) (string, error) {
		executed <- task
		return "done", nil
	})

	// Schedule a one-shot job with a time in the past so it fires immediately.
	past := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)
	job, err := sched.Add("one-shot", past, "my task", KindAt, 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Manually trigger a tick (avoids the 30-second wait in tests).
	sched.tick(context.Background())

	select {
	case got := <-executed:
		if got != "my task" {
			t.Errorf("executor received %q, want %q", got, "my task")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("executor was not called within timeout")
	}

	// After execution the job should be completed.
	updated, err := sched.Get(job.ID)
	if err != nil {
		t.Fatalf("Get after execution: %v", err)
	}
	if updated.Status != JobCompleted {
		t.Errorf("status = %q, want %q", updated.Status, JobCompleted)
	}
	if updated.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", updated.RunCount)
	}
}

func TestEveryJobNextRun(t *testing.T) {
	sched := newTestScheduler(t)
	sched.SetExecutor(func(ctx context.Context, task string) (string, error) {
		return "ok", nil
	})

	job, err := sched.Add("interval", "1s", "ping", KindEvery, 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Force NextRun into the past so tick picks it up.
	past := time.Now().Add(-2 * time.Second)
	sched.mu.Lock()
	sched.jobs[job.ID].NextRun = &past
	sched.mu.Unlock()

	sched.tick(context.Background())

	updated, _ := sched.Get(job.ID)
	if updated.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", updated.RunCount)
	}
	// NextRun should be refreshed after a successful "every" execution.
	if updated.NextRun == nil {
		t.Error("NextRun should not be nil after successful every-job execution")
	}
}

func TestMaxRunsCompletes(t *testing.T) {
	sched := newTestScheduler(t)
	sched.SetExecutor(func(ctx context.Context, task string) (string, error) {
		return "done", nil
	})

	job, err := sched.Add("limited", "1s", "task", KindEvery, 1)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	past := time.Now().Add(-2 * time.Second)
	sched.mu.Lock()
	sched.jobs[job.ID].NextRun = &past
	sched.mu.Unlock()

	sched.tick(context.Background())

	updated, _ := sched.Get(job.ID)
	if updated.Status != JobCompleted {
		t.Errorf("status = %q, want %q", updated.Status, JobCompleted)
	}
}

// --------------------------------------------------------------------------
// Parser tests — ParseDuration
// --------------------------------------------------------------------------

func TestParseEveryDuration(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
		{"1d", 24 * time.Hour},
		{"2d", 48 * time.Hour},
		{"1d12h", 36 * time.Hour},
		{"2h30m", 2*time.Hour + 30*time.Minute},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseDuration(tc.input)
			if err != nil {
				t.Fatalf("ParseDuration(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseDurationErrors(t *testing.T) {
	bad := []string{"", "0s", "abc", "d", "1x"}
	for _, s := range bad {
		t.Run(s, func(t *testing.T) {
			if _, err := ParseDuration(s); err == nil {
				t.Errorf("ParseDuration(%q): expected error, got nil", s)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Parser tests — ParseCronSchedule
// --------------------------------------------------------------------------

func TestParseCronExpression(t *testing.T) {
	// "0 9 * * 1-5" — 09:00 on weekdays (Monday–Friday).
	// We anchor `after` to a Monday so the very next occurrence is the same day.
	after := mustParseTime(t, "2026-03-09T08:59:00Z") // Monday 08:59 UTC
	next, err := ParseCronSchedule("0 9 * * 1-5", after)
	if err != nil {
		t.Fatalf("ParseCronSchedule: %v", err)
	}
	want := mustParseTime(t, "2026-03-09T09:00:00Z")
	if !next.Equal(want) {
		t.Errorf("next = %v, want %v", next.UTC(), want.UTC())
	}

	// "*/5 * * * *" — every 5 minutes.
	after2 := mustParseTime(t, "2026-03-09T10:01:00Z")
	next2, err := ParseCronSchedule("*/5 * * * *", after2)
	if err != nil {
		t.Fatalf("ParseCronSchedule: %v", err)
	}
	want2 := mustParseTime(t, "2026-03-09T10:05:00Z")
	if !next2.Equal(want2) {
		t.Errorf("next2 = %v, want %v", next2.UTC(), want2.UTC())
	}
}

func TestParseCronWeekdaySkip(t *testing.T) {
	// A Saturday: the next weekday (Mon–Fri) should be Monday.
	after := mustParseTime(t, "2026-03-07T09:00:00Z") // Saturday 09:00 UTC
	next, err := ParseCronSchedule("0 9 * * 1-5", after)
	if err != nil {
		t.Fatalf("ParseCronSchedule: %v", err)
	}
	// Next Monday 09:00.
	want := mustParseTime(t, "2026-03-09T09:00:00Z")
	if !next.Equal(want) {
		t.Errorf("next = %v, want %v", next.UTC(), want.UTC())
	}
}

func TestParseCronList(t *testing.T) {
	// "0 9,17 * * *" — 09:00 and 17:00 every day.
	after := mustParseTime(t, "2026-03-09T09:01:00Z")
	next, err := ParseCronSchedule("0 9,17 * * *", after)
	if err != nil {
		t.Fatalf("ParseCronSchedule: %v", err)
	}
	want := mustParseTime(t, "2026-03-09T17:00:00Z")
	if !next.Equal(want) {
		t.Errorf("next = %v, want %v", next.UTC(), want.UTC())
	}
}

func TestParseCronErrors(t *testing.T) {
	after := time.Now()
	bad := []string{
		"",
		"* * * *",     // too few fields
		"* * * * * *", // too many fields
		"60 * * * *",  // minute out of range
		"* 24 * * *",  // hour out of range
		"* * * 13 *",  // month out of range
		"* * * * 7",   // dow out of range
		"*/0 * * * *", // step zero
	}
	for _, expr := range bad {
		t.Run(expr, func(t *testing.T) {
			if _, err := ParseCronSchedule(expr, after); err == nil {
				t.Errorf("ParseCronSchedule(%q): expected error, got nil", expr)
			}
		})
	}
}

// --------------------------------------------------------------------------
// One-shot "at" tests
// --------------------------------------------------------------------------

func TestOneShot(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)

	sched := newTestScheduler(t)
	job, err := sched.Add("one-shot", future, "do something", KindAt, 0)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if job.NextRun == nil {
		t.Fatal("NextRun should not be nil for at-job")
	}
	// Should not fire yet because NextRun is in the future.
	sched.SetExecutor(func(ctx context.Context, task string) (string, error) {
		t.Error("executor should not have been called")
		return "", nil
	})
	sched.tick(context.Background())

	got, _ := sched.Get(job.ID)
	if got.RunCount != 0 {
		t.Errorf("RunCount = %d, want 0 (job is future)", got.RunCount)
	}
	if got.Status != JobActive {
		t.Errorf("status = %q, want active", got.Status)
	}
}

func TestOneShotPastTime(t *testing.T) {
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	_, err := NewScheduler(t.TempDir()).Add("old", past, "task", KindAt, 0)
	// Adding a past time is allowed — the scheduler will fire it on the next tick.
	if err != nil {
		t.Fatalf("Add past at-job: unexpected error: %v", err)
	}
}

// --------------------------------------------------------------------------
// Tool schema / ExecuteTool tests
// --------------------------------------------------------------------------

func TestToolSchemas(t *testing.T) {
	schemas := ToolSchemas()
	if len(schemas) == 0 {
		t.Fatal("ToolSchemas: expected at least one schema")
	}
	names := map[string]bool{}
	for _, s := range schemas {
		n, ok := s["name"].(string)
		if !ok || n == "" {
			t.Errorf("schema missing name: %v", s)
		}
		names[n] = true
	}
	for _, want := range []string{"cron_add", "cron_list", "cron_remove", "cron_pause", "cron_resume"} {
		if !names[want] {
			t.Errorf("missing tool schema for %q", want)
		}
	}
}

func TestExecuteToolAdd(t *testing.T) {
	sched := newTestScheduler(t)
	result, err := ExecuteTool(context.Background(), sched, "cron_add", map[string]interface{}{
		"name":     "my job",
		"schedule": "30m",
		"task":     "summarise logs",
		"kind":     "every",
	})
	if err != nil {
		t.Fatalf("ExecuteTool cron_add: %v", err)
	}
	if result == "" {
		t.Error("ExecuteTool cron_add: expected non-empty result")
	}
	if len(sched.List()) != 1 {
		t.Errorf("expected 1 job after cron_add, got %d", len(sched.List()))
	}
}

func TestExecuteToolList(t *testing.T) {
	sched := newTestScheduler(t)
	_, _ = sched.Add("j1", "1h", "t1", KindEvery, 0)
	result, err := ExecuteTool(context.Background(), sched, "cron_list", nil)
	if err != nil {
		t.Fatalf("ExecuteTool cron_list: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty list output")
	}
}

func TestExecuteToolUnknown(t *testing.T) {
	sched := newTestScheduler(t)
	if _, err := ExecuteTool(context.Background(), sched, "cron_unknown", nil); err == nil {
		t.Error("expected error for unknown tool name, got nil")
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func newTestScheduler(t *testing.T) *Scheduler {
	t.Helper()
	dir, err := os.MkdirTemp("", "cron-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return NewScheduler(dir)
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("mustParseTime(%q): %v", s, err)
	}
	return tm
}
