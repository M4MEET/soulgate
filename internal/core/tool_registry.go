package core

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/M4MEET/soulgate/internal/model"
)

// ToolRegistry holds all available tool schemas and manages lazy loading.
// Only a small set of "always-on" tools plus a search_available_tools meta-tool
// are sent to the model by default. Other tools are loaded on demand when the
// model calls search_available_tools, saving ~3000 tokens per API call.
type ToolRegistry struct {
	mu          sync.RWMutex
	allTools    []model.ToolSchema          // full catalog
	toolIndex   map[string]model.ToolSchema // name -> schema for fast lookup
	activeTools map[string]struct{}         // tools currently exposed to model
}

// alwaysOnTools are sent on every API call. These are the tools users expect
// to work immediately without the model having to search for them first.
// Less common tools (pdf, voice, canvas, sandbox, etc.) are discovered via
// search_available_tools to save tokens.
var alwaysOnTools = map[string]bool{
	"search_available_tools": true,
	// File operations
	"files_read":  true,
	"files_write": true,
	"files_list":  true,
	// Shell execution — covers app launching, system commands, etc.
	"exec_command":  true,
	"process_start": true,
	// Web access
	"web_search": true,
	"web_fetch":  true,
	// Memory
	"memory_write":  true,
	"memory_get":    true,
	"memory_search": true,
	// Agents
	"agent_create": true,
	"agent_list":   true,
}

// NewToolRegistry creates a new empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		toolIndex:   make(map[string]model.ToolSchema),
		activeTools: make(map[string]struct{}),
	}
}

// SetAllTools replaces the full tool catalog and resets active tools.
func (r *ToolRegistry) SetAllTools(tools []model.ToolSchema) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.allTools = tools
	r.toolIndex = make(map[string]model.ToolSchema, len(tools))
	for _, t := range tools {
		r.toolIndex[t.Name] = t
	}
	// Reset active set — only always-on tools
	r.activeTools = make(map[string]struct{})
	for name := range alwaysOnTools {
		r.activeTools[name] = struct{}{}
	}
}

// GetActiveSchemas returns the tool schemas currently exposed to the model:
// always-on tools + any tools activated via search.
func (r *ToolRegistry) GetActiveSchemas() []model.ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]model.ToolSchema, 0, len(r.activeTools))
	for _, t := range r.allTools {
		if _, ok := r.activeTools[t.Name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// Search finds tools matching a query string by checking tool names and descriptions.
// Matching tools are automatically activated (added to the active set).
// Returns the matched tool schemas.
func (r *ToolRegistry) Search(query string) []model.ToolSchema {
	r.mu.Lock()
	defer r.mu.Unlock()

	query = strings.ToLower(query)
	keywords := strings.Fields(query)

	var matched []model.ToolSchema
	for _, t := range r.allTools {
		if r.toolMatchesQuery(t, keywords) {
			matched = append(matched, t)
			r.activeTools[t.Name] = struct{}{}
		}
	}
	return matched
}

// ActivateTool adds a specific tool to the active set by name.
func (r *ToolRegistry) ActivateTool(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.toolIndex[name]; exists {
		r.activeTools[name] = struct{}{}
		return true
	}
	return false
}

// ActivateAll adds all tools to the active set (disables lazy loading for this run).
func (r *ToolRegistry) ActivateAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.allTools {
		r.activeTools[t.Name] = struct{}{}
	}
}

// Reset clears activated tools back to only always-on tools.
func (r *ToolRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeTools = make(map[string]struct{})
	for name := range alwaysOnTools {
		r.activeTools[name] = struct{}{}
	}
}

// IsActive returns whether a tool is in the active set.
func (r *ToolRegistry) IsActive(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.activeTools[name]
	return ok
}

// AllToolNames returns all registered tool names.
func (r *ToolRegistry) AllToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.allTools))
	for _, t := range r.allTools {
		names = append(names, t.Name)
	}
	return names
}

// toolMatchesQuery checks if a tool matches the given keywords.
func (r *ToolRegistry) toolMatchesQuery(tool model.ToolSchema, keywords []string) bool {
	name := strings.ToLower(tool.Name)
	desc := strings.ToLower(tool.Description)
	// Also check the schema for enum values or property names
	schemaStr := strings.ToLower(string(tool.InputSchema))

	for _, kw := range keywords {
		if strings.Contains(name, kw) || strings.Contains(desc, kw) || strings.Contains(schemaStr, kw) {
			return true
		}
	}
	return false
}

// searchAvailableToolsSchema is the meta-tool that lets the model discover tools on demand.
var searchAvailableToolsSchema = model.ToolSchema{
	Name:        "search_available_tools",
	Description: "Search for available tools by keyword. Returns matching tool names and descriptions. Call this BEFORE using a tool you haven't used yet in this conversation. Example queries: 'web search', 'process management', 'pdf', 'cron schedule', 'network http', 'agent', 'patch'.",
	InputSchema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search keywords (e.g. 'web search', 'pdf read', 'process', 'cron')"
			}
		},
		"required": ["query"]
	}`),
}
