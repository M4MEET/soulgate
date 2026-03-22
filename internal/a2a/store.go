package a2a

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskStore manages A2A tasks with persistence to disk.
type TaskStore struct {
	mu       sync.RWMutex
	tasks    map[string]*Task
	subs     map[string][]chan StreamResponse // taskID → subscriber channels
	dataDir  string
	onChange func(task *Task) // optional callback for UI updates
}

// NewTaskStore creates a persistent task store.
func NewTaskStore(dataDir string) *TaskStore {
	s := &TaskStore{
		tasks:   make(map[string]*Task),
		subs:    make(map[string][]chan StreamResponse),
		dataDir: dataDir,
	}
	s.load()
	return s
}

// SetOnChange registers a callback invoked on every task state change.
func (s *TaskStore) SetOnChange(fn func(task *Task)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// Create creates a new task from an incoming message.
func (s *TaskStore) Create(msg Message, contextID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.NewString()
	if contextID == "" {
		contextID = uuid.NewString()
	}

	task := &Task{
		ID:        id,
		ContextID: contextID,
		Status: TaskStatus{
			State:     TaskStateSubmitted,
			Timestamp: time.Now().UTC(),
		},
		History: []Message{msg},
	}
	s.tasks[id] = task
	s.save()
	return task
}

// Get returns a task by ID.
func (s *TaskStore) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// List returns all tasks, optionally filtered by context.
func (s *TaskStore) List(contextID string, limit int) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if contextID != "" && t.ContextID != contextID {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}

	// Sort by timestamp descending
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].Status.Timestamp.Before(out[j].Status.Timestamp) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// UpdateStatus transitions a task's state and notifies subscribers.
func (s *TaskStore) UpdateStatus(id string, state TaskState, msg *Message) error {
	s.mu.Lock()
	task, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", id)
	}

	task.Status = TaskStatus{
		State:     state,
		Message:   msg,
		Timestamp: time.Now().UTC(),
	}
	if msg != nil {
		task.History = append(task.History, *msg)
	}
	s.save()

	// Copy subscribers before releasing lock
	subs := make([]chan StreamResponse, len(s.subs[id]))
	copy(subs, s.subs[id])
	onChange := s.onChange
	s.mu.Unlock()

	// Notify subscribers (non-blocking)
	evt := StreamResponse{
		StatusUpdate: &TaskStatusUpdateEvent{
			TaskID:    id,
			ContextID: task.ContextID,
			Status:    task.Status,
		},
	}
	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}

	// Close subscriber channels on terminal state
	if state.IsTerminal() {
		s.mu.Lock()
		for _, ch := range s.subs[id] {
			close(ch)
		}
		delete(s.subs, id)
		s.mu.Unlock()
	}

	if onChange != nil {
		cp := *task
		onChange(&cp)
	}

	return nil
}

// AddArtifact adds an artifact to a task and notifies subscribers.
func (s *TaskStore) AddArtifact(taskID string, artifact Artifact, appendMode bool, lastChunk bool) error {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("task %s not found", taskID)
	}

	if appendMode {
		// Append parts to existing artifact with same ID
		for i := range task.Artifacts {
			if task.Artifacts[i].ArtifactID == artifact.ArtifactID {
				task.Artifacts[i].Parts = append(task.Artifacts[i].Parts, artifact.Parts...)
				break
			}
		}
	} else {
		task.Artifacts = append(task.Artifacts, artifact)
	}
	s.save()

	subs := make([]chan StreamResponse, len(s.subs[taskID]))
	copy(subs, s.subs[taskID])
	s.mu.Unlock()

	evt := StreamResponse{
		ArtifactUpdate: &TaskArtifactUpdateEvent{
			TaskID:    taskID,
			ContextID: task.ContextID,
			Artifact:  artifact,
			Append:    appendMode,
			LastChunk: lastChunk,
		},
	}
	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
	return nil
}

// Cancel marks a task as canceled.
func (s *TaskStore) Cancel(id string) error {
	s.mu.RLock()
	task, ok := s.tasks[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("task %s not found", id)
	}
	if task.Status.State.IsTerminal() {
		return fmt.Errorf("task %s already in terminal state: %s", id, task.Status.State)
	}
	return s.UpdateStatus(id, TaskStateCanceled, nil)
}

// Subscribe returns a channel that receives streaming events for a task.
// The channel is closed when the task reaches a terminal state.
func (s *TaskStore) Subscribe(taskID string) (<-chan StreamResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[taskID]; !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	ch := make(chan StreamResponse, 32)
	s.subs[taskID] = append(s.subs[taskID], ch)
	return ch, nil
}

// --------------------------------------------------------------------------
// Persistence
// --------------------------------------------------------------------------

const tasksFile = "state/a2a_tasks.json"

func (s *TaskStore) save() {
	if s.dataDir == "" {
		return
	}
	path := filepath.Join(s.dataDir, tasksFile)
	_ = os.MkdirAll(filepath.Dir(path), 0700)

	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

func (s *TaskStore) load() {
	if s.dataDir == "" {
		return
	}
	path := filepath.Join(s.dataDir, tasksFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var tasks map[string]*Task
	if json.Unmarshal(data, &tasks) == nil && tasks != nil {
		s.tasks = tasks
	}
}

// Prune removes terminal tasks older than the given age.
func (s *TaskStore) Prune(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	pruned := 0
	for id, t := range s.tasks {
		if t.Status.State.IsTerminal() && t.Status.Timestamp.Before(cutoff) {
			delete(s.tasks, id)
			pruned++
		}
	}
	if pruned > 0 {
		s.save()
	}
	return pruned
}
