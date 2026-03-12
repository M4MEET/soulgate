package sdk

import "encoding/json"

// Manifest defines the plugin manifest structure
type Manifest struct {
	Name        string                 `yaml:"name"`
	Version     string                 `yaml:"version"`
	Description string                 `yaml:"description,omitempty"`
	Author      string                 `yaml:"author,omitempty"`
	Type        string                 `yaml:"type,omitempty"`        // "channel", "tool", "broker"
	Runtime     string                 `yaml:"runtime,omitempty"`     // "wasm", "go"
	Entrypoint  string                 `yaml:"entrypoint,omitempty"`  // Path to WASM file
	Permissions []string               `yaml:"permissions,omitempty"` // Simplified permission list
	Tools       []ToolDef              `yaml:"tools,omitempty"`
	Provides    map[string]interface{} `yaml:"provides,omitempty"` // What the plugin provides
	Config      map[string]interface{} `yaml:"config,omitempty"`   // Plugin configuration schema
	Commands    []CommandDef           `yaml:"commands,omitempty"` // Commands provided
}

// PermissionsConfig defines the permissions requested by a plugin
type PermissionsConfig struct {
	FilesRead  []string `yaml:"files.read,omitempty"`
	FilesWrite []string `yaml:"files.write,omitempty"`
	FilesList  []string `yaml:"files.list,omitempty"`
	FilesStat  []string `yaml:"files.stat,omitempty"`
	NetRequest []string `yaml:"net.request,omitempty"`
	ExecCommand []string `yaml:"exec.command,omitempty"`
}

// ToolDef defines a tool provided by the plugin
type ToolDef struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	InputSchema json.RawMessage `yaml:"input_schema"` // JSON Schema
}

// CommandDef defines a command provided by the plugin
type CommandDef struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Usage       string        `yaml:"usage,omitempty"`
	Args        []CommandArg  `yaml:"args,omitempty"`
}

// CommandArg defines a command argument
type CommandArg struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required,omitempty"`
}

// GetPermissions returns all permissions as a flat list
func (p *PermissionsConfig) GetPermissions(action string) []string {
	switch action {
	case "files.read":
		return p.FilesRead
	case "files.write":
		return p.FilesWrite
	case "files.list":
		return p.FilesList
	case "files.stat":
		return p.FilesStat
	case "net.request":
		return p.NetRequest
	case "exec.command":
		return p.ExecCommand
	default:
		return nil
	}
}
