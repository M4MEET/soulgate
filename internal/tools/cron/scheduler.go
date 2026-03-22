// Package cron provides a persistent job scheduler for SoulGate.
//
// Jobs are stored as JSON on disk so they survive process restarts. The
// scheduler loop wakes every 30 seconds and dispatches any job whose NextRun
// is in the past. Execution is delegated to a caller-supplied executor
// function, which typically invokes the AI orchestrator.
//
// Three schedule kinds are supported:
//   - "at"    — one-shot, fires once at a specific RFC 3339 wall-clock time.
//   - "every" — repeating, fires every fixed duration (e.g. "30m", "1d").
//   - "cron"  — repeating, fired according to a 5-field cron expression.
package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ScheduleKind classifies how a job's trigger time is computed.
type ScheduleKind string

const (
	// KindAt fires once at a specific wall-clock time (RFC 3339).
	KindAt ScheduleKind = "at"
	// KindEvery fires repeatedly at a fixed interval ("30m", "1h", "1d", …).
	KindEvery ScheduleKind = "every"
	// KindCron fires according to a 5-field cron expression ("0 9 * * 1-5").
	KindCron ScheduleKind = "cron"
)

// JobStatus describes the current lifecycle state of a job.
type JobStatus string

const (
	// JobActive means the job is eligible to run.
	JobActive JobStatus = "active"
	// JobPaused means the job will not fire until resumed.
	JobPaused JobStatus = "paused"
	// JobCompleted means a one-shot "at" job has been executed.
	JobCompleted JobStatus = "completed"
	// JobFailed means the job's last execution returned an error.
	JobFailed JobStatus = "failed"
)

const (
	jobsFilename = "state/cron.json"
	tickInterval = 30 * time.Second
	maxLookahead = 366 * 24 * time.Hour
)

// Job holds the configuration and runtime state of a scheduled task.
type Job struct {
	ID   string       `json:"id"`
	Name string       `json:"name"`
	Kind ScheduleKind `json:"kind"`
	// Schedule is the raw schedule expression provided by the caller:
	//   - KindAt:    RFC 3339 time string
	//   - KindEvery: duration string ("30m", "1d", …)
	//   - KindCron:  5-field cron expression ("0 9 * * 1-5")
	Schedule  string     `json:"schedule"`
	Task      string     `json:"task"`
	Status    JobStatus  `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	RunCount  int        `json:"run_count"`
	// MaxRuns limits the number of executions. 0 means unlimited.
	MaxRuns    int    `json:"max_runs,omitempty"`
	LastResult string `json:"last_result,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

// Scheduler manages a collection of Jobs, persists them to disk, and
// periodically dispatches overdue jobs to an executor function.
type Scheduler struct {
	jobs     map[string]*Job
	nextID   int
	dataDir  string
	mu       sync.RWMutex
	cancel   context.CancelFunc
	executor func(ctx context.Context, task string) (string, error)
}

// NewScheduler creates a Scheduler whose state is stored under dataDir.
// It loads any previously saved jobs from disk immediately.
func NewScheduler(dataDir string) *Scheduler {
	s := &Scheduler{
		jobs:    make(map[string]*Job),
		dataDir: dataDir,
	}
	// Best-effort load; if the file does not exist yet that is fine.
	_ = s.load()
	return s
}

// SetExecutor installs the callback used to run job tasks.
// The executor receives a context (cancelled when Stop is called) and the
// plain-text task prompt; it returns the result string or an error.
func (s *Scheduler) SetExecutor(fn func(ctx context.Context, task string) (string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.executor = fn
}

// Add creates a new Job and persists it to disk.
//
// Parameters:
//   - name     — human-readable label
//   - schedule — interpretation depends on kind (see ScheduleKind docs)
//   - task     — prompt text handed to the executor on each run
//   - kind     — one of KindAt, KindEvery, KindCron
//   - maxRuns  — maximum executions; 0 for unlimited
func (s *Scheduler) Add(name, schedule, task string, kind ScheduleKind, maxRuns int) (*Job, error) {
	if name == "" {
		return nil, fmt.Errorf("cron: job name is required")
	}
	if schedule == "" {
		return nil, fmt.Errorf("cron: schedule is required")
	}
	if task == "" {
		return nil, fmt.Errorf("cron: task is required")
	}

	nextRun, err := computeNextRun(kind, schedule, time.Now())
	if err != nil {
		return nil, fmt.Errorf("cron: invalid schedule: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	job := &Job{
		ID:        fmt.Sprintf("cron_%d", s.nextID),
		Name:      name,
		Kind:      kind,
		Schedule:  schedule,
		Task:      task,
		Status:    JobActive,
		CreatedAt: time.Now(),
		NextRun:   &nextRun,
		MaxRuns:   maxRuns,
	}
	s.jobs[job.ID] = job

	if err := s.save(); err != nil {
		// Undo in-memory state so the caller gets a clean failure.
		delete(s.jobs, job.ID)
		s.nextID--
		return nil, fmt.Errorf("cron: failed to persist job: %w", err)
	}

	// Return a copy so the caller cannot mutate internal state.
	jobCopy := *job
	return &jobCopy, nil
}

// Remove deletes the job with the given ID from memory and disk.
func (s *Scheduler) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("cron: job %q not found", id)
	}
	delete(s.jobs, id)

	if err := s.save(); err != nil {
		return fmt.Errorf("cron: failed to persist after remove: %w", err)
	}
	return nil
}

// Pause suspends a job so it will not fire until resumed.
func (s *Scheduler) Pause(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("cron: job %q not found", id)
	}
	if job.Status == JobCompleted {
		return fmt.Errorf("cron: cannot pause completed job %q", id)
	}
	job.Status = JobPaused

	if err := s.save(); err != nil {
		return fmt.Errorf("cron: failed to persist after pause: %w", err)
	}
	return nil
}

// Resume re-activates a paused job.
func (s *Scheduler) Resume(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("cron: job %q not found", id)
	}
	if job.Status != JobPaused {
		return fmt.Errorf("cron: job %q is not paused (status: %s)", id, job.Status)
	}
	job.Status = JobActive

	if err := s.save(); err != nil {
		return fmt.Errorf("cron: failed to persist after resume: %w", err)
	}
	return nil
}

// List returns a snapshot of all known jobs in an unspecified order.
func (s *Scheduler) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		copy := *j
		out = append(out, &copy)
	}
	return out
}

// Get returns a single job by ID or an error if the ID is unknown.
func (s *Scheduler) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("cron: job %q not found", id)
	}
	copy := *job
	return &copy, nil
}

// Start launches the scheduler loop in a background goroutine. The loop
// respects the supplied context for shutdown; calling Stop() also cancels it.
//
// Start is idempotent: calling it multiple times is safe (subsequent calls
// replace the previous loop).
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel() // stop the previous loop if any
	}
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	go s.loop(loopCtx)
}

// Stop terminates the scheduler loop started by Start.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// loop is the background goroutine that fires overdue jobs.
func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	// Run immediately on start so short tests do not have to wait 30 s.
	s.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick checks every active job and dispatches those whose NextRun is overdue.
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()

	// Collect overdue job IDs under the read lock to minimise lock contention.
	s.mu.RLock()
	var due []string
	for id, job := range s.jobs {
		if job.Status != JobActive {
			continue
		}
		if job.NextRun != nil && !job.NextRun.After(now) {
			due = append(due, id)
		}
	}
	s.mu.RUnlock()

	for _, id := range due {
		// Re-acquire write lock for each job execution so we do not hold the
		// lock while the executor (potentially slow) runs.
		s.dispatch(ctx, id, now)
	}
}

// dispatch executes a single overdue job and updates its state.
func (s *Scheduler) dispatch(ctx context.Context, id string, now time.Time) {
	s.mu.Lock()
	job, ok := s.jobs[id]
	if !ok || job.Status != JobActive {
		s.mu.Unlock()
		return
	}

	// Snapshot the task before releasing the lock for execution.
	task := job.Task
	executor := s.executor
	s.mu.Unlock()

	var result string
	var execErr error

	if executor != nil {
		result, execErr = executor(ctx, task)
	} else {
		execErr = fmt.Errorf("no executor configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-fetch: another goroutine may have removed the job while we executed.
	job, ok = s.jobs[id]
	if !ok {
		return
	}

	job.LastRun = &now
	job.RunCount++

	if execErr != nil {
		job.Status = JobFailed
		job.LastError = execErr.Error()
		job.LastResult = ""
	} else {
		job.LastError = ""
		job.LastResult = result

		// Determine whether we should mark the job as completed.
		exhausted := job.MaxRuns > 0 && job.RunCount >= job.MaxRuns

		switch job.Kind {
		case KindAt:
			// One-shot: always completed after first run.
			job.Status = JobCompleted
			job.NextRun = nil

		case KindEvery:
			if exhausted {
				job.Status = JobCompleted
				job.NextRun = nil
			} else {
				d, err := ParseDuration(job.Schedule)
				if err == nil {
					next := now.Add(d)
					job.NextRun = &next
				} else {
					job.Status = JobFailed
					job.LastError = fmt.Sprintf("failed to recompute next run: %v", err)
				}
			}

		case KindCron:
			if exhausted {
				job.Status = JobCompleted
				job.NextRun = nil
			} else {
				next, err := ParseCronSchedule(job.Schedule, now)
				if err == nil {
					job.NextRun = &next
				} else {
					job.Status = JobFailed
					job.LastError = fmt.Sprintf("failed to recompute next run: %v", err)
				}
			}
		}
	}

	// Persist state after every execution attempt.
	_ = s.save()

	// Append to the durable execution history journal.
	s.appendHistory(id, job.Name, now, result, execErr)
}

// --------------------------------------------------------------------------
// Persistence
// --------------------------------------------------------------------------

// persistedState is the on-disk schema for the jobs file.
type persistedState struct {
	NextID int             `json:"next_id"`
	Jobs   map[string]*Job `json:"jobs"`
}

// save writes the current scheduler state to dataDir/state/cron.json.
// Caller must hold s.mu (at least for reading when called from dispatch).
// In practice, save is always called under a write lock.
func (s *Scheduler) save() error {
	path := filepath.Join(s.dataDir, jobsFilename)
	// Ensure the full directory path (including subdirs from jobsFilename) exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cron: cannot create data directory: %w", err)
	}

	state := persistedState{
		NextID: s.nextID,
		Jobs:   s.jobs,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cron: failed to marshal jobs: %w", err)
	}
	// Write to a temporary file first and rename for atomic replacement.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("cron: failed to write jobs file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cron: failed to rename jobs file: %w", err)
	}
	return nil
}

// load reads scheduler state from dataDir/cron_jobs.json.
// It is called once during NewScheduler and does not require the mutex since
// no goroutines are running yet at that point.
func (s *Scheduler) load() error {
	path := filepath.Join(s.dataDir, jobsFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no prior state — start fresh
		}
		return fmt.Errorf("cron: failed to read jobs file: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("cron: failed to parse jobs file: %w", err)
	}

	if state.Jobs != nil {
		s.jobs = state.Jobs
	}
	if state.NextID > s.nextID {
		s.nextID = state.NextID
	}
	return nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// computeNextRun calculates the first fire time for a newly created job.
func computeNextRun(kind ScheduleKind, schedule string, from time.Time) (time.Time, error) {
	switch kind {
	case KindAt:
		t, err := time.Parse(time.RFC3339, schedule)
		if err != nil {
			return time.Time{}, fmt.Errorf("'at' schedule must be RFC 3339 (e.g. 2026-03-15T10:00:00Z): %w", err)
		}
		return t, nil

	case KindEvery:
		d, err := ParseDuration(schedule)
		if err != nil {
			return time.Time{}, err
		}
		return from.Add(d), nil

	case KindCron:
		return ParseCronSchedule(schedule, from)

	default:
		return time.Time{}, fmt.Errorf("unknown schedule kind %q", kind)
	}
}

// --------------------------------------------------------------------------
// Execution History (durable JSONL journal)
// --------------------------------------------------------------------------

const historyFilename = "logs/cron_history.jsonl"

// HistoryEntry records a single cron job execution for auditing purposes.
type HistoryEntry struct {
	JobID    string    `json:"job_id"`
	JobName  string    `json:"job_name"`
	RunAt    time.Time `json:"run_at"`
	Duration string    `json:"duration,omitempty"`
	Result   string    `json:"result,omitempty"`
	Error    string    `json:"error,omitempty"`
	Success  bool      `json:"success"`
}

// appendHistory appends a single execution record to the JSONL history file.
// Errors are silently swallowed — history is best-effort.
func (s *Scheduler) appendHistory(jobID, jobName string, runAt time.Time, result string, err error) {
	path := filepath.Join(s.dataDir, historyFilename)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)

	entry := HistoryEntry{
		JobID:   jobID,
		JobName: jobName,
		RunAt:   runAt,
		Success: err == nil,
	}
	if err != nil {
		entry.Error = err.Error()
	} else {
		r := result
		if len(r) > 500 {
			r = r[:500] + "..."
		}
		entry.Result = r
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	data = append(data, '\n')

	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if openErr != nil {
		return
	}
	f.Write(data)
	f.Close()
}

// History returns the last n execution records from the JSONL history file.
// Returns nil if no history exists.
func (s *Scheduler) History(limit int) []HistoryEntry {
	path := filepath.Join(s.dataDir, historyFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entries []HistoryEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e HistoryEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries
}
