package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Role defines the permission level of a user within SoulGate.
type Role string

const (
	// RoleAdmin has full access to all operations and configuration.
	RoleAdmin Role = "admin"
	// RoleDev can run agents and modify most settings but not manage users.
	RoleDev Role = "developer"
	// RoleViewer has read-only access to sessions and audit logs.
	RoleViewer Role = "viewer"
	// RoleOperator can manage agents and sessions but cannot alter config.
	RoleOperator Role = "operator"
)

// validRoles is the set of Role values accepted by CreateUser / UpdateUser.
var validRoles = map[Role]bool{
	RoleAdmin:    true,
	RoleDev:      true,
	RoleViewer:   true,
	RoleOperator: true,
}

// UserSettings holds per-user LLM and UI preferences.
type UserSettings struct {
	DefaultModel     string  `json:"default_model"`
	DefaultProvider  string  `json:"default_provider"`
	ThinkingLevel    string  `json:"thinking_level"`
	Temperature      float64 `json:"temperature"`
	StreamingEnabled bool    `json:"streaming_enabled"`
	Theme            string  `json:"theme"` // "dark" | "light"
}

// UserLimits constrains resource consumption for a user.
// A zero value for any numeric field means "unlimited".
type UserLimits struct {
	MaxTokensPerDay int      `json:"max_tokens_per_day"`
	MaxCostPerDay   float64  `json:"max_cost_per_day"`
	MaxCostPerMonth float64  `json:"max_cost_per_month"`
	MaxAgents       int      `json:"max_agents"`     // concurrent agents; 0 = unlimited
	AllowedModels   []string `json:"allowed_models"` // empty = all
	AllowedTools    []string `json:"allowed_tools"`  // empty = all
}

// User represents an authenticated principal in the system.
type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	DisplayName string       `json:"display_name"`
	Email       string       `json:"email,omitempty"`
	Role        Role         `json:"role"`
	TeamID      string       `json:"team_id,omitempty"`
	APIKey      string       `json:"api_key"` // sg_user_<32 hex chars>
	Settings    UserSettings `json:"settings"`
	Limits      UserLimits   `json:"limits"`
	CreatedAt   time.Time    `json:"created_at"`
	LastActive  time.Time    `json:"last_active"`
	Active      bool         `json:"active"`
}

// Team is an organisational grouping of users with shared limits.
type Team struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Limits      UserLimits `json:"limits"`
	CreatedAt   time.Time  `json:"created_at"`
	MemberCount int        `json:"member_count"`
}

// store is the JSON structure persisted to users.json.
type store struct {
	Users []*User `json:"users"`
	Teams []*Team `json:"teams"`
}

// UserManager manages users and teams with thread-safe in-memory storage backed
// by a JSON file at configDir/users.json.
type UserManager struct {
	mu     sync.RWMutex
	users  map[string]*User // id -> user
	byKey  map[string]*User // api_key -> user
	byName map[string]*User // username -> user
	teams  map[string]*Team // id -> team
	path   string           // persistence path
}

// NewUserManager loads users and teams from configDir/security/users.json.
// On first run (empty or missing file) it creates an admin user, prints the
// API key to stdout, and persists the new state.
func NewUserManager(configDir string) (*UserManager, error) {
	path := filepath.Join(configDir, "security", "users.json")

	m := &UserManager{
		users:  make(map[string]*User),
		byKey:  make(map[string]*User),
		byName: make(map[string]*User),
		teams:  make(map[string]*Team),
		path:   path,
	}

	if err := m.load(); err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}

	// Bootstrap: create the admin user if the store is empty.
	if len(m.users) == 0 {
		admin, err := m.CreateUser("admin", "Administrator", "", RoleAdmin)
		if err != nil {
			return nil, fmt.Errorf("bootstrap admin: %w", err)
		}
		fmt.Printf("\nAdmin user created:\n   Username: %s\n   API Key:  %s\n   Save this key — it won't be shown again.\n\n",
			admin.Username, admin.APIKey)
	}

	return m, nil
}

// load reads users.json; a missing file is not an error.
func (m *UserManager) load() error {
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read users file: %w", err)
	}

	var s store
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("parse users file: %w", err)
	}

	for _, u := range s.Users {
		m.users[u.ID] = u
		m.byKey[u.APIKey] = u
		m.byName[u.Username] = u
	}
	for _, t := range s.Teams {
		m.teams[t.ID] = t
	}
	return nil
}

// Save persists users and teams to disk with 0600 permissions.
// The caller must not hold m.mu; Save acquires a read lock internally.
func (m *UserManager) Save() error {
	m.mu.RLock()
	users := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		cp := *u
		users = append(users, &cp)
	}
	teams := make([]*Team, 0, len(m.teams))
	for _, t := range m.teams {
		cp := *t
		teams = append(teams, &cp)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(store{Users: users, Teams: teams}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal users: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(m.path, data, 0o600); err != nil {
		return fmt.Errorf("write users file: %w", err)
	}
	return nil
}

// GenerateAPIKey generates a sg_user_ prefixed key with 32 random hex chars.
func (m *UserManager) GenerateAPIKey() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		// Fallback: unlikely but handle gracefully.
		panic(fmt.Sprintf("auth: crypto/rand failure: %v", err))
	}
	return "sg_user_" + hex.EncodeToString(raw)
}

// generateID generates a short random hex ID.
func generateID() string {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Sprintf("auth: crypto/rand failure: %v", err))
	}
	return hex.EncodeToString(raw)
}

// CreateUser creates a new active user and persists the store.
// username must be unique and non-empty. role must be one of the defined Role constants.
func (m *UserManager) CreateUser(username, displayName, email string, role Role) (*User, error) {
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if !validRoles[role] {
		return nil, fmt.Errorf("unknown role %q", role)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byName[username]; exists {
		return nil, fmt.Errorf("user %q already exists", username)
	}

	now := time.Now().UTC()
	u := &User{
		ID:          generateID(),
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		Role:        role,
		APIKey:      m.GenerateAPIKey(),
		Settings: UserSettings{
			ThinkingLevel:    "normal",
			Temperature:      1.0,
			StreamingEnabled: true,
			Theme:            "dark",
		},
		CreatedAt:  now,
		LastActive: now,
		Active:     true,
	}

	m.users[u.ID] = u
	m.byKey[u.APIKey] = u
	m.byName[u.Username] = u

	// Persist without the write lock held.
	m.mu.Unlock()
	saveErr := m.Save()
	m.mu.Lock()

	if saveErr != nil {
		// Roll back in-memory state.
		delete(m.users, u.ID)
		delete(m.byKey, u.APIKey)
		delete(m.byName, u.Username)
		return nil, fmt.Errorf("persist user: %w", saveErr)
	}

	return u, nil
}

// GetUser returns the user with the given ID.
func (m *UserManager) GetUser(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("user %q not found", id)
	}
	cp := *u
	return &cp, nil
}

// GetUserByAPIKey looks up a user by their sg_user_ API key.
// This is the primary entry point for auth middleware.
func (m *UserManager) GetUserByAPIKey(key string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.byKey[key]
	if !ok {
		return nil, fmt.Errorf("invalid API key")
	}
	if !u.Active {
		return nil, fmt.Errorf("user %q is inactive", u.Username)
	}
	cp := *u
	return &cp, nil
}

// GetUserByUsername looks up a user by username.
func (m *UserManager) GetUserByUsername(username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.byName[username]
	if !ok {
		return nil, fmt.Errorf("user %q not found", username)
	}
	cp := *u
	return &cp, nil
}

// UpdateUser applies the given key/value pairs to the user identified by id.
// Recognised keys: display_name, email, role, active, team_id,
//
//	settings.default_model, settings.default_provider, settings.thinking_level,
//	settings.temperature, settings.streaming_enabled, settings.theme,
//	limits.max_tokens_per_day, limits.max_cost_per_day, limits.max_cost_per_month,
//	limits.max_agents.
func (m *UserManager) UpdateUser(id string, updates map[string]interface{}) error {
	m.mu.Lock()
	u, ok := m.users[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("user %q not found", id)
	}

	for k, v := range updates {
		switch k {
		case "display_name":
			if s, ok := v.(string); ok {
				u.DisplayName = s
			}
		case "email":
			if s, ok := v.(string); ok {
				u.Email = s
			}
		case "role":
			if s, ok := v.(string); ok {
				r := Role(s)
				if !validRoles[r] {
					m.mu.Unlock()
					return fmt.Errorf("unknown role %q", s)
				}
				u.Role = r
			}
		case "active":
			if b, ok := v.(bool); ok {
				u.Active = b
			}
		case "team_id":
			if s, ok := v.(string); ok {
				if s != "" {
					if _, exists := m.teams[s]; !exists {
						m.mu.Unlock()
						return fmt.Errorf("team %q not found", s)
					}
				}
				u.TeamID = s
			}
		case "settings.default_model":
			if s, ok := v.(string); ok {
				u.Settings.DefaultModel = s
			}
		case "settings.default_provider":
			if s, ok := v.(string); ok {
				u.Settings.DefaultProvider = s
			}
		case "settings.thinking_level":
			if s, ok := v.(string); ok {
				u.Settings.ThinkingLevel = s
			}
		case "settings.temperature":
			if f, ok := toFloat64(v); ok {
				u.Settings.Temperature = f
			}
		case "settings.streaming_enabled":
			if b, ok := v.(bool); ok {
				u.Settings.StreamingEnabled = b
			}
		case "settings.theme":
			if s, ok := v.(string); ok {
				u.Settings.Theme = s
			}
		case "limits.max_tokens_per_day":
			if n, ok := toInt(v); ok {
				u.Limits.MaxTokensPerDay = n
			}
		case "limits.max_cost_per_day":
			if f, ok := toFloat64(v); ok {
				u.Limits.MaxCostPerDay = f
			}
		case "limits.max_cost_per_month":
			if f, ok := toFloat64(v); ok {
				u.Limits.MaxCostPerMonth = f
			}
		case "limits.max_agents":
			if n, ok := toInt(v); ok {
				u.Limits.MaxAgents = n
			}
		}
	}

	m.mu.Unlock()
	return m.Save()
}

// DeleteUser deactivates the user (soft delete). The record is retained in the
// store for audit continuity; use UpdateUser with active=false for the same
// effect, or call DeleteUser which additionally clears the API key index.
func (m *UserManager) DeleteUser(id string) error {
	m.mu.Lock()
	u, ok := m.users[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("user %q not found", id)
	}
	u.Active = false
	// Remove from fast-lookup indexes while keeping the record itself.
	delete(m.byKey, u.APIKey)
	m.mu.Unlock()

	return m.Save()
}

// ListUsers returns a snapshot of all users (active and inactive), sorted by
// no particular order (callers should sort if needed).
func (m *UserManager) ListUsers() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		cp := *u
		out = append(out, &cp)
	}
	return out
}

// TouchLastActive updates the LastActive timestamp for a user. This is
// intended to be called cheaply on every authenticated request without a
// full Save(); callers should periodically flush with Save() on a timer.
func (m *UserManager) TouchLastActive(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u, ok := m.users[id]; ok {
		u.LastActive = time.Now().UTC()
	}
}

// CheckLimits returns an error if the user has exceeded any configured limits.
// tokensUsed and costUSD represent the incremental amounts for the current
// operation; this method compares them against per-day limits only.
// For simplicity limits are checked against the per-request deltas; callers
// must track cumulative usage externally and pass it here.
func (m *UserManager) CheckLimits(userID string, tokensUsed int, costUSD float64) error {
	m.mu.RLock()
	u, ok := m.users[userID]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("user %q not found", userID)
	}
	limits := u.Limits
	m.mu.RUnlock()

	if limits.MaxTokensPerDay > 0 && tokensUsed > limits.MaxTokensPerDay {
		return fmt.Errorf("daily token limit exceeded (%d / %d)", tokensUsed, limits.MaxTokensPerDay)
	}
	if limits.MaxCostPerDay > 0 && costUSD > limits.MaxCostPerDay {
		return fmt.Errorf("daily cost limit exceeded ($%.4f / $%.4f)", costUSD, limits.MaxCostPerDay)
	}
	return nil
}

// --- Team operations ---

// CreateTeam creates a new team and persists the store.
func (m *UserManager) CreateTeam(name, description string) (*Team, error) {
	if name == "" {
		return nil, fmt.Errorf("team name is required")
	}

	m.mu.Lock()
	// Check for duplicate name.
	for _, t := range m.teams {
		if t.Name == name {
			m.mu.Unlock()
			return nil, fmt.Errorf("team %q already exists", name)
		}
	}

	t := &Team{
		ID:          generateID(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	m.teams[t.ID] = t
	m.mu.Unlock()

	if err := m.Save(); err != nil {
		m.mu.Lock()
		delete(m.teams, t.ID)
		m.mu.Unlock()
		return nil, fmt.Errorf("persist team: %w", err)
	}
	return t, nil
}

// GetTeam returns the team with the given ID.
func (m *UserManager) GetTeam(id string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.teams[id]
	if !ok {
		return nil, fmt.Errorf("team %q not found", id)
	}
	cp := *t
	return &cp, nil
}

// ListTeams returns a snapshot of all teams with accurate member counts.
func (m *UserManager) ListTeams() []*Team {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Recompute member counts in case they drifted.
	counts := make(map[string]int, len(m.teams))
	for _, u := range m.users {
		if u.Active && u.TeamID != "" {
			counts[u.TeamID]++
		}
	}

	out := make([]*Team, 0, len(m.teams))
	for _, t := range m.teams {
		cp := *t
		cp.MemberCount = counts[t.ID]
		out = append(out, &cp)
	}
	return out
}

// AddUserToTeam assigns a user to a team, replacing any previous assignment.
func (m *UserManager) AddUserToTeam(userID, teamID string) error {
	m.mu.Lock()
	u, ok := m.users[userID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("user %q not found", userID)
	}
	if _, ok := m.teams[teamID]; !ok {
		m.mu.Unlock()
		return fmt.Errorf("team %q not found", teamID)
	}
	u.TeamID = teamID
	m.mu.Unlock()

	return m.Save()
}

// RemoveUserFromTeam clears the team assignment for a user.
func (m *UserManager) RemoveUserFromTeam(userID string) error {
	m.mu.Lock()
	u, ok := m.users[userID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("user %q not found", userID)
	}
	u.TeamID = ""
	m.mu.Unlock()

	return m.Save()
}

// --- helpers ---

// toFloat64 coerces a JSON-decoded value (float64, int, int64, etc.) to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// toInt coerces a JSON-decoded value to int.
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}
