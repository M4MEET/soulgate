package sdk

import "encoding/json"

// Manifest defines the plugin manifest structure.
// Plugins can use "script" runtime (default, AI-creatable) or "wasm" (sandboxed).
type Manifest struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description,omitempty"`
	Author      string            `yaml:"author,omitempty"`
	Runtime     string            `yaml:"runtime,omitempty"` // "script" (default) or "wasm"
	Entrypoint  string            `yaml:"entrypoint,omitempty"`
	Permissions []string          `yaml:"permissions,omitempty"`
	Tools       []ToolDef         `yaml:"tools,omitempty"`
	Requires    RequirementsDef   `yaml:"requires,omitempty"`
	Config      map[string]string `yaml:"config,omitempty"` // key -> description
}

// ToolDef defines a tool provided by the plugin.
type ToolDef struct {
	Name           string          `yaml:"name"`
	Description    string          `yaml:"description"`
	Command        string          `yaml:"command,omitempty"` // For script runtime: command to execute
	InputSchemaRaw interface{}     `yaml:"input_schema"`      // Parsed from YAML as generic map
	InputSchema    json.RawMessage `yaml:"-"`                 // Computed: JSON-encoded schema
}

// ResolveInputSchemas converts the YAML-parsed InputSchemaRaw to JSON for each tool.
func ResolveInputSchemas(tools []ToolDef) error {
	for i := range tools {
		if tools[i].InputSchemaRaw != nil {
			data, err := json.Marshal(tools[i].InputSchemaRaw)
			if err != nil {
				return err
			}
			tools[i].InputSchema = data
		}
	}
	return nil
}

// RequirementsDef declares what the plugin needs to run.
type RequirementsDef struct {
	Env  []string `yaml:"env,omitempty"`  // Required environment variables
	Bins []string `yaml:"bins,omitempty"` // Required binaries on PATH
}
