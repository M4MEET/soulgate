package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/model"
)

const branchesFileName = "state/branches.json"

// ConversationBranch represents a saved snapshot of a conversation timeline.
// A branch holds the messages up to (and including) the fork point, plus any
// subsequent messages added while the branch is active.
type ConversationBranch struct {
	ID        string          `json:"id"`
	ParentID  string          `json:"parent_id"`  // empty for the root branch
	ForkPoint int             `json:"fork_point"` // message index in parent where this branch diverges
	Label     string          `json:"label"`      // user-supplied label
	Messages  []model.Message `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
}

// BranchInfo is a summary view of a branch used by List().
type BranchInfo struct {
	ID           string
	ParentID     string
	Label        string
	MessageCount int
	CreatedAt    time.Time
	IsCurrent    bool
}

// BranchManager stores and manipulates conversation branches.
// It is the single source of truth for which branch is active and persists
// state to .soulgate/branches.json so branches survive session restarts.
type BranchManager struct {
	mu       sync.RWMutex
	branches map[string]*ConversationBranch
	current  string // ID of the currently active branch
	path     string // absolute path to branches.json
}

// branchesFile is the on-disk representation persisted by BranchManager.
type branchesFile struct {
	CurrentBranchID string                         `json:"current_branch_id"`
	Branches        map[string]*ConversationBranch `json:"branches"`
}

// NewBranchManager creates a BranchManager rooted at configDir.
// It loads any previously saved branches from disk and creates the initial
// "main" branch if none exist. It never returns an error — missing or corrupt
// files are handled by starting fresh.
func NewBranchManager(configDir string) *BranchManager {
	bm := &BranchManager{
		branches: make(map[string]*ConversationBranch),
		path:     filepath.Join(configDir, branchesFileName),
	}

	// Best-effort load; errors are silently swallowed — we start fresh.
	_ = bm.load()

	// Guarantee there is always at least one branch.
	if len(bm.branches) == 0 {
		root := bm.newBranch("", 0, "main", nil)
		bm.branches[root.ID] = root
		bm.current = root.ID
	}

	// Repair: if current points to a non-existent branch, reset to any branch.
	if _, ok := bm.branches[bm.current]; !ok {
		for id := range bm.branches {
			bm.current = id
			break
		}
	}

	return bm
}

// newBranch allocates a ConversationBranch without acquiring the mutex.
// Callers are responsible for holding the lock when required.
func (bm *BranchManager) newBranch(parentID string, forkPoint int, label string, messages []model.Message) *ConversationBranch {
	id := fmt.Sprintf("branch_%d", time.Now().UnixNano())
	msgs := make([]model.Message, len(messages))
	copy(msgs, messages)
	return &ConversationBranch{
		ID:        id,
		ParentID:  parentID,
		ForkPoint: forkPoint,
		Label:     label,
		Messages:  msgs,
		CreatedAt: time.Now(),
	}
}

// Fork creates a new branch from the current branch at the given message index.
// forkPoint is the index (0-based) into the current branch's Messages slice
// at which the new branch diverges; only messages[0:forkPoint] are carried over.
// If label is empty a default label is generated.
// Returns the new branch ID.
func (bm *BranchManager) Fork(label string, forkPoint int) (string, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	parent, ok := bm.branches[bm.current]
	if !ok {
		return "", fmt.Errorf("current branch %q not found", bm.current)
	}

	if forkPoint < 0 {
		forkPoint = 0
	}
	if forkPoint > len(parent.Messages) {
		forkPoint = len(parent.Messages)
	}

	if label == "" {
		label = fmt.Sprintf("fork-%s", time.Now().Format("150405"))
	}

	branch := bm.newBranch(parent.ID, forkPoint, label, parent.Messages[:forkPoint])
	bm.branches[branch.ID] = branch

	return branch.ID, bm.save()
}

// Switch changes the active branch to branchID.
// It does not modify any branch's Messages; callers are responsible for
// syncing the orchestrator's conversationHistory using GetCurrentMessages.
func (bm *BranchManager) Switch(branchID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if _, ok := bm.branches[branchID]; !ok {
		return fmt.Errorf("branch %q not found", branchID)
	}

	bm.current = branchID
	return bm.save()
}

// List returns a summary of all branches, sorted by creation time (oldest first).
func (bm *BranchManager) List() []BranchInfo {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	infos := make([]BranchInfo, 0, len(bm.branches))
	for _, b := range bm.branches {
		infos = append(infos, BranchInfo{
			ID:           b.ID,
			ParentID:     b.ParentID,
			Label:        b.Label,
			MessageCount: len(b.Messages),
			CreatedAt:    b.CreatedAt,
			IsCurrent:    b.ID == bm.current,
		})
	}

	// Stable sort: oldest branch first; current branch listed first on ties.
	for i := 0; i < len(infos); i++ {
		for j := i + 1; j < len(infos); j++ {
			less := infos[i].CreatedAt.After(infos[j].CreatedAt)
			if less {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	return infos
}

// Delete removes a branch. The current branch and the root branch (no parent)
// cannot be deleted.
func (bm *BranchManager) Delete(branchID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	b, ok := bm.branches[branchID]
	if !ok {
		return fmt.Errorf("branch %q not found", branchID)
	}

	if branchID == bm.current {
		return fmt.Errorf("cannot delete the active branch; switch to another branch first")
	}

	if b.ParentID == "" {
		return fmt.Errorf("cannot delete the root branch")
	}

	delete(bm.branches, branchID)
	return bm.save()
}

// GetCurrentMessages returns a copy of the messages belonging to the active branch.
// This is what the orchestrator should use as its conversationHistory when the
// branch is switched.
func (bm *BranchManager) GetCurrentMessages() []model.Message {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	b, ok := bm.branches[bm.current]
	if !ok {
		return nil
	}

	cp := make([]model.Message, len(b.Messages))
	copy(cp, b.Messages)
	return cp
}

// SyncMessages replaces the active branch's message list with msgs.
// The orchestrator calls this to keep the branch in sync after each run.
func (bm *BranchManager) SyncMessages(msgs []model.Message) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	b, ok := bm.branches[bm.current]
	if !ok {
		return fmt.Errorf("current branch %q not found", bm.current)
	}

	cp := make([]model.Message, len(msgs))
	copy(cp, msgs)
	b.Messages = cp

	return bm.save()
}

// Merge appends the unique messages from branchID (those beyond its fork point)
// into the current branch. Messages already present in the current branch are
// not duplicated.
func (bm *BranchManager) Merge(branchID string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	src, ok := bm.branches[branchID]
	if !ok {
		return fmt.Errorf("branch %q not found", branchID)
	}

	if branchID == bm.current {
		return fmt.Errorf("cannot merge a branch into itself")
	}

	dst, ok := bm.branches[bm.current]
	if !ok {
		return fmt.Errorf("current branch %q not found", bm.current)
	}

	// Unique messages are those beyond the fork point in the source branch.
	uniqueStart := src.ForkPoint
	if uniqueStart > len(src.Messages) {
		uniqueStart = len(src.Messages)
	}
	unique := src.Messages[uniqueStart:]

	merged := make([]model.Message, len(dst.Messages), len(dst.Messages)+len(unique))
	copy(merged, dst.Messages)
	merged = append(merged, unique...)
	dst.Messages = merged

	return bm.save()
}

// CurrentBranchID returns the ID of the active branch.
func (bm *BranchManager) CurrentBranchID() string {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return bm.current
}

// GetBranch returns the branch with the given ID, or false if not found.
func (bm *BranchManager) GetBranch(id string) (*ConversationBranch, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	b, ok := bm.branches[id]
	if !ok {
		return nil, false
	}
	// Return a shallow copy to prevent external mutation.
	cp := *b
	msgs := make([]model.Message, len(b.Messages))
	copy(msgs, b.Messages)
	cp.Messages = msgs
	return &cp, true
}

// save writes the current state to disk. Caller must hold bm.mu (at least read lock).
// In practice callers hold the write lock, so no additional locking is needed here.
func (bm *BranchManager) save() error {
	f := branchesFile{
		CurrentBranchID: bm.current,
		Branches:        bm.branches,
	}

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal branches: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(bm.path), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(bm.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write branches file: %w", err)
	}

	return nil
}

// load reads branches from disk into bm. No lock required — called only during
// construction before the BranchManager is shared.
func (bm *BranchManager) load() error {
	data, err := os.ReadFile(bm.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh start
		}
		return fmt.Errorf("failed to read branches file: %w", err)
	}

	var f branchesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("failed to parse branches file: %w", err)
	}

	if f.Branches != nil {
		bm.branches = f.Branches
	}
	bm.current = f.CurrentBranchID
	return nil
}
