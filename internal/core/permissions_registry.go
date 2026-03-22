package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/M4MEET/soulgate/internal/model"
)

// ToolPermissionDef describes how a tool's permission checks are derived.
type ToolPermissionDef struct {
	// Action is the primary policy action (e.g., "web.search").
	Action string

	// FallbackActions are tried if the primary action has no matching rule.
	FallbackActions []string

	// ResourceFromInput extracts the resource string from the tool's JSON input.
	// If nil, a static Resource is used instead.
	ResourceFromInput func(input json.RawMessage) (string, error)

	// Resource is used when ResourceFromInput is nil. For schema checks this is
	// the wildcard pattern; for runtime checks it's the static resource string.
	Resource string

	// SchemaResource is the wildcard resource used for schema visibility checks.
	// If empty, Resource is used.
	SchemaResource string

	// BrokerManaged means the broker enforces policy internally so no
	// orchestrator-level permission check is needed at runtime.
	BrokerManaged bool

	// NoPermissionRequired means the tool never needs permission checks.
	NoPermissionRequired bool
}

// toolPermissionDefs maps tool names to their permission definitions.
// This single map replaces both permissionChecksForToolCall and
// schemaPermissionChecksForTool switch statements.
var toolPermissionDefs = map[string]ToolPermissionDef{
	// Broker-managed tools: policy enforced inside the broker
	"files_read":   {Action: "files.read", SchemaResource: "./**", BrokerManaged: true},
	"files_list":   {Action: "files.list", SchemaResource: "./**", BrokerManaged: true},
	"files_write":  {Action: "files.write", SchemaResource: "./**", BrokerManaged: true},
	"files_delete": {Action: "files.delete", SchemaResource: "./**", BrokerManaged: true},
	"exec_command": {Action: "exec.command", SchemaResource: "command:*", BrokerManaged: true},
	"net_request":  {Action: "net.request", SchemaResource: "https://*", BrokerManaged: true},

	// Web tools
	"web_search": {
		Action:          "web.search",
		FallbackActions: []string{"net.request"},
		SchemaResource:  "query:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ Query string `json:"query"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			q := strings.TrimSpace(p.Query)
			if q == "" {
				return "", fmt.Errorf("web_search requires non-empty query")
			}
			return "query:" + q, nil
		},
	},
	"web_fetch": {
		Action:          "web.fetch",
		FallbackActions: []string{"net.request"},
		SchemaResource:  "https://*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ URL string `json:"url"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			if strings.TrimSpace(p.URL) == "" {
				return "", fmt.Errorf("web_fetch requires non-empty url")
			}
			return p.URL, nil
		},
	},

	// Process tools
	"process_start": {
		Action:          "process.start",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "process:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ Command string `json:"command"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			if strings.TrimSpace(p.Command) == "" {
				return "", fmt.Errorf("process_start requires non-empty command")
			}
			return p.Command, nil
		},
	},
	"process_list": {
		Action:          "process.list",
		FallbackActions: []string{"exec.command"},
		Resource:        "processes",
		SchemaResource:  "process:*",
	},
	"process_poll": {
		Action:          "process.poll",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "process:*",
		ResourceFromInput: processIDResource("process_poll"),
	},
	"process_log": {
		Action:          "process.log",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "process:*",
		ResourceFromInput: processIDResource("process_log"),
	},
	"process_write": {
		Action:          "process.write",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "process:*",
		ResourceFromInput: processIDResource("process_write"),
	},
	"process_kill": {
		Action:          "process.kill",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "process:*",
		ResourceFromInput: processIDResource("process_kill"),
	},

	// PDF tool
	"pdf_read": {
		Action:          "pdf.read",
		FallbackActions: []string{"files.read", "pdf.fetch", "net.request"},
		SchemaResource:  "./**",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ Path string `json:"path"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			path := strings.TrimSpace(p.Path)
			if path == "" {
				return "", fmt.Errorf("pdf_read requires non-empty path")
			}
			return normalizePolicyFileResource(path), nil
		},
	},

	// Cron tools
	"cron_add":    cronDef(),
	"cron_list":   cronDef(),
	"cron_remove": cronDef(),
	"cron_pause":  cronDef(),
	"cron_resume": cronDef(),

	// File watcher tools
	"watch_start": {
		Action:          "filewatcher.start",
		FallbackActions: []string{"files.read"},
		SchemaResource:  "./**",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ Path string `json:"path"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			path := strings.TrimSpace(p.Path)
			if path == "" {
				return "", fmt.Errorf("watch_start requires non-empty path")
			}
			return normalizePolicyFileResource(path), nil
		},
	},
	"watch_list": {
		Action:         "filewatcher.list",
		Resource:       "filewatcher:*",
		SchemaResource: "filewatcher:*",
	},
	"watch_stop": {
		Action:         "filewatcher.stop",
		SchemaResource: "filewatcher:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ ID string `json:"id"` }
			_ = json.Unmarshal(input, &p)
			id := strings.TrimSpace(p.ID)
			if id == "" {
				return "filewatcher:*", nil
			}
			return "filewatcher:" + id, nil
		},
	},

	// Apply patch — uses special multi-file handling
	"apply_patch": {
		Action:          "patch.apply",
		FallbackActions: []string{"files.write", "files.delete"},
		SchemaResource:  "./**",
		// ResourceFromInput is nil; apply_patch uses extractApplyPatchPermissionChecks directly
	},

	// Voice tools — non-destructive API calls; no workspace-level permission required
	"voice_speak":      {NoPermissionRequired: true},
	"voice_transcribe": {NoPermissionRequired: true},

	// Image generation tools — external API calls that write files to the
	// workspace; the path traversal guard lives inside the package itself.
	"image_generate": {NoPermissionRequired: true},
	"image_edit":     {NoPermissionRequired: true},

	// Browser automation tools — primary action is browser.*, fallback to net.request
	// for policy files that only allow net access but not a dedicated browser rule.
	"browser_open": {
		Action:          "browser.open",
		FallbackActions: []string{"net.request"},
		SchemaResource:  "https://*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ URL string `json:"url"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			if strings.TrimSpace(p.URL) == "" {
				return "", fmt.Errorf("browser_open requires non-empty url")
			}
			return p.URL, nil
		},
	},
	"browser_screenshot": {
		Action:          "browser.screenshot",
		FallbackActions: []string{"net.request"},
		Resource:        "browser:screenshot",
		SchemaResource:  "browser:screenshot",
	},
	"browser_click": {
		Action:          "browser.click",
		FallbackActions: []string{"net.request"},
		Resource:        "browser:click",
		SchemaResource:  "browser:click",
	},
	"browser_type": {
		Action:          "browser.type",
		FallbackActions: []string{"net.request"},
		Resource:        "browser:type",
		SchemaResource:  "browser:type",
	},
	"browser_eval": {
		Action:          "browser.eval",
		FallbackActions: []string{"net.request"},
		Resource:        "browser:eval",
		SchemaResource:  "browser:eval",
	},
	"browser_html": {
		Action:          "browser.html",
		FallbackActions: []string{"net.request"},
		Resource:        "browser:html",
		SchemaResource:  "browser:html",
	},

	// Semantic memory tools — non-destructive, internal storage
	"memory_index":  {NoPermissionRequired: true},
	"memory_recall": {NoPermissionRequired: true},
	"memory_forget": {NoPermissionRequired: true},

	// Canvas artifact tools — write to .soulgate/canvas/ internally, no workspace policy needed
	"canvas_create":  {NoPermissionRequired: true},
	"canvas_update":  {NoPermissionRequired: true},
	"canvas_list":    {NoPermissionRequired: true},
	"canvas_preview": {NoPermissionRequired: true},

	// Sandbox code execution tools — require sandbox.execute permission,
	// falling back to exec.command for policy files that only define the
	// broader exec action.
	"code_run": {
		Action:          "sandbox.execute",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "sandbox:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct {
				Language string `json:"language"`
			}
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			lang := strings.TrimSpace(p.Language)
			if lang == "" {
				return "", fmt.Errorf("code_run requires non-empty language")
			}
			return "sandbox:" + lang, nil
		},
	},
	"code_install": {
		Action:          "sandbox.execute",
		FallbackActions: []string{"exec.command"},
		SchemaResource:  "sandbox:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct {
				Language string `json:"language"`
				Package  string `json:"package"`
			}
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			lang := strings.TrimSpace(p.Language)
			if lang == "" {
				return "", fmt.Errorf("code_install requires non-empty language")
			}
			return "sandbox:" + lang, nil
		},
	},

	// MCP resource and prompt access tools — use the mcp.tool_call action
	// (same as dynamically-prefixed MCP tools) so a single policy rule covers
	// all MCP interactions from any server.
	"mcp_read_resource": {
		Action:          "mcp.tool_call",
		FallbackActions: []string{"net.request"},
		SchemaResource:  "mcp:resource:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ URI string `json:"uri"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			if strings.TrimSpace(p.URI) == "" {
				return "", fmt.Errorf("mcp_read_resource requires non-empty uri")
			}
			return "mcp:resource:" + p.URI, nil
		},
	},
	"mcp_get_prompt": {
		Action:          "mcp.tool_call",
		FallbackActions: []string{"net.request"},
		SchemaResource:  "mcp:prompt:*",
		ResourceFromInput: func(input json.RawMessage) (string, error) {
			var p struct{ Name string `json:"name"` }
			if err := json.Unmarshal(input, &p); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			if strings.TrimSpace(p.Name) == "" {
				return "", fmt.Errorf("mcp_get_prompt requires non-empty name")
			}
			return "mcp:prompt:" + p.Name, nil
		},
	},

	// No-permission tools
	"search_available_tools": {NoPermissionRequired: true},
	"llm_task":               {NoPermissionRequired: true},
	"memory_write":         {NoPermissionRequired: true},
	"memory_get":           {NoPermissionRequired: true},
	"memory_search":        {NoPermissionRequired: true},
	"switch_model":         {NoPermissionRequired: true},
	"agent_create":    {NoPermissionRequired: true},
	"agent_list":      {NoPermissionRequired: true},
	"agent_stop":      {NoPermissionRequired: true},
	"agent_delegate":  {NoPermissionRequired: true},
	"agent_message":   {NoPermissionRequired: true},
	"soulgate_introspect":  {NoPermissionRequired: true},
	"soulgate_configure":   {NoPermissionRequired: true},
}

func cronDef() ToolPermissionDef {
	return ToolPermissionDef{
		Action:          "cron.manage",
		FallbackActions: []string{"exec.command"},
		Resource:        "cron.manage",
		SchemaResource:  "cron.manage",
	}
}

// processIDResource returns a ResourceFromInput func that extracts the process ID
// and builds a "toolName:id" resource string.
func processIDResource(toolName string) func(json.RawMessage) (string, error) {
	return func(input json.RawMessage) (string, error) {
		var p struct{ ID string `json:"id"` }
		_ = json.Unmarshal(input, &p)
		resource := toolName
		if strings.TrimSpace(p.ID) != "" {
			resource = toolName + ":" + p.ID
		}
		return resource, nil
	}
}

// runtimeChecksFromRegistry builds permission checks for a tool call using
// the permission registry. Returns nil checks for broker-managed and
// no-permission-required tools. Returns an error for tools with validation
// requirements (e.g., empty required fields).
func runtimeChecksFromRegistry(toolCall model.ToolCall) ([]permissionCheck, error) {
	def, ok := toolPermissionDefs[toolCall.Name]
	if !ok {
		return nil, nil // unknown tool — handled by fallback in permissionChecksForToolCall
	}

	if def.NoPermissionRequired || def.BrokerManaged {
		return nil, nil
	}

	// Special case: apply_patch needs multi-file checks
	if toolCall.Name == "apply_patch" {
		var params struct{ Patch string `json:"patch"` }
		if err := json.Unmarshal(toolCall.Input, &params); err != nil {
			return nil, fmt.Errorf("invalid tool input: %w", err)
		}
		return extractApplyPatchPermissionChecks(params.Patch)
	}

	// Special case: pdf_read may switch action based on URL vs path
	if toolCall.Name == "pdf_read" {
		var params struct{ Path string `json:"path"` }
		if err := json.Unmarshal(toolCall.Input, &params); err != nil {
			return nil, fmt.Errorf("invalid tool input: %w", err)
		}
		path := strings.TrimSpace(params.Path)
		if path == "" {
			return nil, fmt.Errorf("pdf_read requires non-empty path")
		}
		if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
			return []permissionCheck{{
				Action:          "pdf.fetch",
				Resource:        path,
				FallbackActions: []string{"net.request"},
			}}, nil
		}
		return []permissionCheck{{
			Action:          def.Action,
			Resource:        normalizePolicyFileResource(path),
			FallbackActions: []string{"files.read"},
		}}, nil
	}

	// Resolve resource
	var resource string
	if def.ResourceFromInput != nil {
		r, err := def.ResourceFromInput(toolCall.Input)
		if err != nil {
			return nil, err
		}
		resource = r
	} else {
		resource = def.Resource
	}

	// Resolve action (process_poll etc. derive action from tool name)
	action := def.Action
	if action == "" {
		action = strings.ReplaceAll(toolCall.Name, "_", ".")
	}

	return []permissionCheck{{
		Action:          action,
		Resource:        resource,
		FallbackActions: def.FallbackActions,
	}}, nil
}

// schemaChecksFromRegistry builds permission checks for schema visibility
// using the permission registry.
func schemaChecksFromRegistry(toolName string) []permissionCheck {
	def, ok := toolPermissionDefs[toolName]
	if !ok {
		return nil // unknown tool — handled by fallback in schemaPermissionChecksForTool
	}

	if def.NoPermissionRequired {
		return nil
	}

	resource := def.SchemaResource
	if resource == "" {
		resource = def.Resource
	}

	return []permissionCheck{{
		Action:          def.Action,
		Resource:        resource,
		FallbackActions: def.FallbackActions,
	}}
}
