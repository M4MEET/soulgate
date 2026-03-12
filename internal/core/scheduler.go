package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
)

// ScheduleEntry represents a scheduled task (cron-like) for agents or skills
type ScheduleEntry struct {
	ID          string        `json:"id" yaml:"id"`
	Name        string        `json:"name" yaml:"name"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
	Type        ScheduleType  `json:"type" yaml:"type"` // "skill", "agent", "prompt"
	Target      string        `json:"target" yaml:"target"` // skill ID, agent ID, or raw prompt
	Interval    time.Duration `json:"interval" yaml:"interval"` // How often to run
	CronExpr    string        `json:"cron_expr,omitempty" yaml:"cron_expr,omitempty"` // Optional cron expression
	Enabled     bool          `json:"enabled" yaml:"enabled"`
	LastRun     *time.Time    `json:"last_run,omitempty" yaml:"last_run,omitempty"`
	NextRun     *time.Time    `json:"next_run,omitempty" yaml:"next_run,omitempty"`
	RunCount    int           `json:"run_count" yaml:"run_count"`
	MaxRuns     int           `json:"max_runs,omitempty" yaml:"max_runs,omitempty"` // 0 = unlimited
	CreatedAt   time.Time     `json:"created_at" yaml:"created_at"`
}

// ScheduleType defines the type of scheduled task
type ScheduleType string

const (
	ScheduleTypeSkill  ScheduleType = "skill"
	ScheduleTypeAgent  ScheduleType = "agent"
	ScheduleTypePrompt ScheduleType = "prompt"
)

// Scheduler manages periodic task execution for agents and skills
type Scheduler struct {
	entries  map[string]*ScheduleEntry
	mu       sync.RWMutex
	cancel   context.CancelFunc
	running  bool
	orch     *Orchestrator
	audit    audit.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(orch *Orchestrator, auditLogger audit.Logger) *Scheduler {
	return &Scheduler{
		entries: make(map[string]*ScheduleEntry),
		orch:    orch,
		audit:   auditLogger,
	}
}

// AddEntry adds a new scheduled entry
func (s *Scheduler) AddEntry(entry *ScheduleEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("sched_%d", time.Now().UnixNano())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	// Calculate next run
	now := time.Now()
	next := now.Add(entry.Interval)
	entry.NextRun = &next

	s.entries[entry.ID] = entry
	return nil
}

// RemoveEntry removes a scheduled entry
func (s *Scheduler) RemoveEntry(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, id)
}

// ListEntries returns all scheduled entries
func (s *Scheduler) ListEntries() []*ScheduleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*ScheduleEntry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
	}
	return entries
}

// GetEntry returns a specific entry by ID
func (s *Scheduler) GetEntry(id string) (*ScheduleEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[id]
	return entry, ok
}

// Start begins the scheduler loop
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}

	ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.mu.Unlock()

	go s.loop(ctx)
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
}

// IsRunning returns whether the scheduler is active
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// loop is the main scheduler loop
func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.checkAndRun(ctx, now)
		}
	}
}

// checkAndRun checks all entries and runs any that are due
func (s *Scheduler) checkAndRun(ctx context.Context, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range s.entries {
		if !entry.Enabled {
			continue
		}

		// Check max runs
		if entry.MaxRuns > 0 && entry.RunCount >= entry.MaxRuns {
			entry.Enabled = false
			continue
		}

		// Check if it's time to run
		if entry.NextRun != nil && now.After(*entry.NextRun) {
			go s.executeEntry(ctx, entry)

			// Update scheduling
			entry.RunCount++
			lastRun := now
			entry.LastRun = &lastRun
			nextRun := now.Add(entry.Interval)
			entry.NextRun = &nextRun
		}
	}
}

// executeEntry executes a single scheduled entry
func (s *Scheduler) executeEntry(ctx context.Context, entry *ScheduleEntry) {
	// Log the scheduled execution
	if s.audit != nil {
		event := audit.NewEvent("scheduled_run", audit.CategoryRun).
			WithMetadata("schedule_id", entry.ID).
			WithMetadata("schedule_type", string(entry.Type)).
			WithMetadata("target", entry.Target)
		s.audit.Log(ctx, event)
	}

	var prompt string
	switch entry.Type {
	case ScheduleTypePrompt:
		prompt = entry.Target
	case ScheduleTypeSkill:
		prompt = fmt.Sprintf("Execute skill: %s", entry.Target)
	case ScheduleTypeAgent:
		prompt = fmt.Sprintf("Run agent task: %s", entry.Target)
	}

	if s.orch != nil && prompt != "" {
		taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		s.orch.Run(taskCtx, prompt)
	}
}
