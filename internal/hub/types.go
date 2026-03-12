package hub

import "time"

// PluginInfo represents basic plugin information
type PluginInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Rating      float64  `json:"rating"`
	Downloads   int      `json:"downloads"`
	UpdatedAt   string   `json:"updated_at"`
}

// PluginDetails represents detailed plugin information
type PluginDetails struct {
	PluginInfo
	Homepage     string            `json:"homepage"`
	License      string            `json:"license"`
	Requires     PluginRequires    `json:"requires"`
	Permissions  []string          `json:"permissions"`
	Config       map[string]Config `json:"config"`
	Tools        []ToolInfo        `json:"tools"`
	Screenshots  []string          `json:"screenshots"`
	Changelog    string            `json:"changelog"`
	Reviews      []Review          `json:"reviews"`
	Dependencies []string          `json:"dependencies"`
}

// PluginRequires specifies requirements
type PluginRequires struct {
	SoulGate string `json:"soulgate" yaml:"soulgate"`
}

// Config represents configuration field
type Config struct {
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required" yaml:"required"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
}

// ToolInfo represents tool information
type ToolInfo struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// Review represents a user review
type Review struct {
	User      string    `json:"user"`
	Rating    int       `json:"rating"`
	Title     string    `json:"title"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	Helpful   int       `json:"helpful"`
}

// SkillInfo represents basic skill information
type SkillInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Rating      float64  `json:"rating"`
	Downloads   int      `json:"downloads"`
	UpdatedAt   string   `json:"updated_at"`
}

// SkillDetails represents detailed skill information
type SkillDetails struct {
	SkillInfo
	Triggers []Trigger      `json:"triggers"`
	Steps    []WorkflowStep `json:"steps"`
	Examples []string       `json:"examples"`
}

// Trigger represents skill trigger
type Trigger struct {
	Event    string `json:"event,omitempty" yaml:"event,omitempty"`
	Pattern  string `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Schedule string `json:"schedule,omitempty" yaml:"schedule,omitempty"`
}

// WorkflowStep represents automation step
type WorkflowStep struct {
	Name      string                 `json:"name" yaml:"name"`
	Tool      string                 `json:"tool" yaml:"tool"`
	Params    map[string]interface{} `json:"params" yaml:"params"`
	Condition string                 `json:"condition,omitempty" yaml:"condition,omitempty"`
}

// HubItem represents any hub item
type HubItem struct {
	Type        string  `json:"type"` // plugin, skill, integration, recipe
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Rating      float64 `json:"rating"`
	Downloads   int     `json:"downloads"`
	Category    string  `json:"category"`
}

// InstalledItem represents locally installed item
type InstalledItem struct {
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AutoUpdate bool      `json:"auto_update"`
}

// InstallOptions represents installation options
type InstallOptions struct {
	Force      bool     // Force reinstall
	SkipVerify bool     // Skip signature verification
	Version    string   // Specific version to install
	Config     map[string]string // Pre-filled configuration
}
