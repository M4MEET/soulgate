package core

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/M4MEET/soulgate/internal/mcp"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/M4MEET/soulgate/internal/tools/browser"
	"github.com/M4MEET/soulgate/internal/tools/canvas"
	"github.com/M4MEET/soulgate/internal/tools/cron"
	"github.com/M4MEET/soulgate/internal/brokers/secrets"
	"github.com/M4MEET/soulgate/internal/tools/email"
	"github.com/M4MEET/soulgate/internal/tools/embeddings"
	"github.com/M4MEET/soulgate/internal/tools/filewatcher"
	gittools "github.com/M4MEET/soulgate/internal/tools/git"
	"github.com/M4MEET/soulgate/internal/tools/imagegen"
	"github.com/M4MEET/soulgate/internal/tools/llmtask"
	"github.com/M4MEET/soulgate/internal/tools/patch"
	"github.com/M4MEET/soulgate/internal/tools/pdf"
	"github.com/M4MEET/soulgate/internal/tools/process"
	"github.com/M4MEET/soulgate/internal/tools/sandbox"
	"github.com/M4MEET/soulgate/internal/tools/voice"
	"github.com/M4MEET/soulgate/internal/tools/web"
)

// getToolSchemas returns the tool schemas currently exposed to the model.
// Uses lazy loading: only always-on tools + the search_available_tools meta-tool
// are sent by default. Other tools are loaded on demand via search_available_tools.
func (o *Orchestrator) getToolSchemas() []model.ToolSchema {
	if o.toolRegistry == nil {
		// Fallback for tests or callers that don't initialize the registry:
		// return all tools (pre-lazy-loading behavior).
		return o.filterToolSchemasByPolicy(o.getAllToolSchemas())
	}
	return o.filterToolSchemasByPolicy(o.toolRegistry.GetActiveSchemas())
}

// initToolRegistry populates the tool registry with all available tools.
// Call once per run (or when tools change). The registry handles which subset
// is actually sent to the model.
func (o *Orchestrator) initToolRegistry() {
	allTools := o.getAllToolSchemas()
	// Prepend the search meta-tool
	allTools = append([]model.ToolSchema{searchAvailableToolsSchema}, allTools...)
	o.toolRegistry.SetAllTools(allTools)
}

// getAllToolSchemas builds the complete catalog of all tool schemas.
// This is NOT sent directly to the model — it feeds the registry.
func (o *Orchestrator) getAllToolSchemas() []model.ToolSchema {
	// Built-in tools
	tools := []model.ToolSchema{
		{
			Name:        "files_read",
			Description: "Read a file.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "File path"
					}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "files_list",
			Description: "List directory contents.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory path"
					}
				}
			}`),
		},
		{
			Name:        "files_write",
			Description: "Write a file (create or overwrite).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "File path"
					},
					"content": {
						"type": "string",
						"description": "File content"
					}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        "files_delete",
			Description: "Delete a file or directory.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path"
					}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "exec_command",
			Description: "Execute a short shell command that should exit quickly. For servers/watchers/daemons, use process_start.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "Shell command"
					}
				},
				"required": ["command"]
			}`),
		},
		{
			Name:        "net_request",
			Description: "Make an HTTP request.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"method": {
						"type": "string",
						"description": "HTTP method",
						"enum": ["GET", "POST", "PUT", "DELETE"]
					},
					"url": {
						"type": "string",
						"description": "URL"
					},
					"body": {
						"type": "string",
						"description": "Request body"
					},
					"headers": {
						"type": "object",
						"description": "Headers",
						"additionalProperties": {"type": "string"}
					}
				},
				"required": ["method", "url"]
			}`),
		},
		{
			Name:        "memory_write",
			Description: "Save a key-value pair to persistent memory.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {
						"type": "string",
						"description": "Key"
					},
					"value": {
						"type": "string",
						"description": "Value"
					}
				},
				"required": ["key", "value"]
			}`),
		},
		{
			Name:        "memory_get",
			Description: "Get a value from memory.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {
						"type": "string",
						"description": "Key"
					}
				},
				"required": ["key"]
			}`),
		},
		{
			Name:        "memory_search",
			Description: "Search memory entries.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query"
					}
				}
			}`),
		},
		{
			Name:        "switch_model",
			Description: "Switch AI model.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"model": {
						"type": "string",
						"description": "Model name",
						"enum": ["gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "o3", "claude-sonnet", "claude-opus"]
					},
					"reason": {
						"type": "string",
						"description": "Reason"
					}
				},
				"required": ["model", "reason"]
			}`),
		},
	}

	// Self-introspection and self-configuration tools
	tools = append(tools, model.ToolSchema{
		Name:        "soulgate_introspect",
		Description: "Inspect SoulGate's own state: config, tools, policy, source paths, build info.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"section": {
					"type": "string",
					"description": "What to inspect",
					"enum": ["all", "config", "tools", "policy", "source", "providers", "status"]
				}
			}
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "soulgate_configure",
		Description: "Modify SoulGate's own config or policy at runtime. Changes are persisted.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {
					"type": "string",
					"description": "What to configure",
					"enum": ["set_provider", "set_model", "add_policy_rule", "remove_policy_rule", "set_execution_limit", "reload_config"]
				},
				"key": {
					"type": "string",
					"description": "Config key or rule name"
				},
				"value": {
					"type": "string",
					"description": "New value"
				}
			},
			"required": ["action"]
		}`),
	})

	// Agent management tools
	tools = append(tools, model.ToolSchema{
		Name:        "agent_create",
		Description: "Create a background agent for a task.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Agent name"
				},
				"task": {
					"type": "string",
					"description": "Task description"
				},
				"role": {
					"type": "string",
					"description": "Agent role specialisation",
					"enum": ["general", "coder", "research", "ops"]
				}
			},
			"required": ["name", "task"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "agent_list",
		Description: "List background agents.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "agent_stop",
		Description: "Stop a background agent.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "string",
					"description": "Agent ID"
				}
			},
			"required": ["id"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "agent_delegate",
		Description: "Delegate a sub-task to a new specialised sub-agent. If wait=true, blocks until the sub-agent finishes and returns its result; otherwise returns immediately with the sub-agent ID.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"task": {
					"type": "string",
					"description": "Description of the sub-task to delegate"
				},
				"role": {
					"type": "string",
					"description": "Sub-agent role specialisation",
					"enum": ["general", "coder", "research", "ops"]
				},
				"wait": {
					"type": "boolean",
					"description": "If true, block until the sub-agent completes and return its result. Default false."
				}
			},
			"required": ["task"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "agent_message",
		Description: "Send a message to another running agent. The receiving agent will see it in its next iteration.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"agent_id": {
					"type": "string",
					"description": "ID of the target agent (e.g. 'agent_2')"
				},
				"message": {
					"type": "string",
					"description": "Message text to deliver"
				}
			},
			"required": ["agent_id", "message"]
		}`),
	})

	// Web tools (web_search, web_fetch)
	for _, s := range web.ToolSchemas() {
		tools = append(tools, model.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	// Process management tools
	for _, s := range process.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// PDF tool
	for _, s := range pdf.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// Cron scheduler tools
	for _, s := range cron.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// File watcher tools
	for _, s := range filewatcher.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// LLM Task tool
	for _, s := range llmtask.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// Apply Patch tool
	for _, s := range patch.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// Browser automation tools
	for _, s := range browser.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// Canvas artifact tools (html, react, svg, mermaid)
	for _, s := range canvas.ToolSchemas() {
		tools = append(tools, model.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	// Voice tools (TTS and STT via OpenAI audio APIs)
	for _, s := range voice.ToolSchemas() {
		tools = append(tools, model.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	// Image generation and editing tools (DALL-E 3 / FAL.ai)
	for _, s := range imagegen.ToolSchemas() {
		tools = append(tools, model.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	// Semantic memory tools (vector embeddings)
	for _, s := range embeddings.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// Sandbox code execution tools (code_run, code_install)
	for _, s := range sandbox.ToolSchemas() {
		tools = append(tools, model.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	// Email tools (email_send via SMTP)
	for _, s := range email.ToolSchemas() {
		tools = append(tools, model.ToolSchema{
			Name:        s.Name,
			Description: s.Description,
			InputSchema: s.InputSchema,
		})
	}

	// Git tools (git_status, git_diff, git_log, git_commit, git_branch, git_stash)
	for _, s := range gittools.ToolSchemas() {
		schemaJSON, _ := json.Marshal(s["input_schema"])
		tools = append(tools, model.ToolSchema{
			Name:        s["name"].(string),
			Description: s["description"].(string),
			InputSchema: json.RawMessage(schemaJSON),
		})
	}

	// Secret broker tools (secret_set, secret_list, secret_delete, secret_inject)
	if o.secretBroker != nil {
		for _, s := range secrets.ToolSchemas() {
			schemaJSON, _ := json.Marshal(s["input_schema"])
			tools = append(tools, model.ToolSchema{
				Name:        s["name"].(string),
				Description: s["description"].(string),
				InputSchema: json.RawMessage(schemaJSON),
			})
		}
	}

	// Add tools from configured integrations
	integrationTools := o.integrationsReg.GetConfiguredTools()
	for _, intTool := range integrationTools {
		tools = append(tools, model.ToolSchema{
			Name:        intTool.Name,
			Description: intTool.Description,
			InputSchema: intTool.InputSchema,
		})
	}

	// Add tools from MCP servers (callable tools + resource/prompt access tools).
	if o.mcpManager != nil {
		for _, mcpTool := range o.mcpManager.GetAllTools() {
			tools = append(tools, model.ToolSchema{
				Name:        mcpTool.Name,
				Description: mcpTool.Description,
				InputSchema: mcpTool.InputSchema,
			})
		}

		// Add mcp_read_resource and mcp_get_prompt when the manager has at
		// least one running server with the relevant capability.  These tools
		// let the model read MCP resources and render MCP prompts at runtime —
		// capabilities that exist on connected servers but otherwise have no
		// tool surface in SoulGate.
		if len(o.mcpManager.GetAllResources()) > 0 {
			tools = append(tools, model.ToolSchema{
				Name:        "mcp_read_resource",
				Description: "Read the content of an MCP resource by URI. Use this to access resources listed under the MCP section of the system prompt (e.g. file://, git://, database:// URIs exposed by connected MCP servers).",
				InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"uri": {
							"type": "string",
							"description": "The resource URI to read (e.g. 'file:///workspace/README.md')"
						}
					},
					"required": ["uri"]
				}`),
			})
		}
		if len(o.mcpManager.GetAllPrompts()) > 0 {
			tools = append(tools, model.ToolSchema{
				Name:        "mcp_get_prompt",
				Description: "Render an MCP prompt template with the provided arguments. Use this to invoke prompt templates listed under the MCP section of the system prompt.",
				InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"name": {
							"type": "string",
							"description": "The prompt template name"
						},
						"arguments": {
							"type": "object",
							"description": "Key-value arguments for the prompt template",
							"additionalProperties": {"type": "string"}
						}
					},
					"required": ["name"]
				}`),
			})
		}
	}

	// Add tools from plugins (script and WASM)
	if o.pluginManager != nil {
		for _, pt := range o.pluginManager.GetToolSchemas() {
			tools = append(tools, model.ToolSchema{
				Name:        pt.Name,
				Description: pt.Description,
				InputSchema: pt.InputSchema,
			})
		}
	}

	return o.filterToolSchemasByPolicy(tools)
}

// GetAvailableToolNames returns all registered tool names (full catalog, policy-filtered).
func (o *Orchestrator) GetAvailableToolNames() []string {
	allTools := o.filterToolSchemasByPolicy(o.getAllToolSchemas())
	names := make([]string, 0, len(allTools))
	for _, tool := range allTools {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func (o *Orchestrator) filterToolSchemasByPolicy(tools []model.ToolSchema) []model.ToolSchema {
	if len(tools) == 0 || o.IsTrustMode() {
		return tools
	}

	filtered := make([]model.ToolSchema, 0, len(tools))
	for _, tool := range tools {
		if o.shouldExposeToolSchema(tool.Name) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func (o *Orchestrator) shouldExposeToolSchema(toolName string) bool {
	checks := schemaPermissionChecksForTool(toolName)
	if len(checks) == 0 {
		return true
	}
	return o.policyAllowsSchemaChecks(checks)
}

func (o *Orchestrator) policyAllowsSchemaChecks(checks []permissionCheck) bool {
	if o.policyEngine == nil {
		return false
	}
	pol := o.policyEngine.GetPolicy()
	if pol == nil {
		return o.permissionCallback != nil
	}

	matcher := policy.NewMatcher()
	for _, check := range checks {
		if !o.policyAllowsSchemaCheck(check, pol.Policies, matcher) {
			return false
		}
	}
	return true
}

func (o *Orchestrator) policyAllowsSchemaCheck(
	check permissionCheck,
	rules []policy.PolicyRule,
	matcher *policy.Matcher,
) bool {
	candidates := permissionCheckCandidates(check)
	if len(candidates) == 0 {
		return true
	}

	hasExplicitDeny := false
	for _, action := range candidates {
		hasMatch := false
		hasRequireApproval := false
		for _, rule := range rules {
			if !matcher.MatchAction(rule.Action, action) {
				continue
			}
			hasMatch = true
			switch rule.Decision {
			case policy.DecisionAllow:
				return true
			case policy.DecisionRequireApproval:
				hasRequireApproval = true
			case policy.DecisionDeny:
				hasExplicitDeny = true
			}
		}

		// If this action has no matching rules, it may still be approvable at runtime.
		if !hasMatch && o.permissionCallback != nil {
			return true
		}
		if hasRequireApproval && o.permissionCallback != nil {
			return true
		}
	}

	return o.permissionCallback != nil && !hasExplicitDeny
}

func schemaPermissionChecksForTool(toolName string) []permissionCheck {
	// Try the data-driven registry first.
	checks := schemaChecksFromRegistry(toolName)
	if checks != nil {
		return checks
	}

	// Tool is in the registry but needs no checks (no-permission-required).
	if _, ok := toolPermissionDefs[toolName]; ok {
		return nil
	}

	// MCP tools
	if mcp.IsMCPTool(toolName) {
		return []permissionCheck{{
			Action:   "mcp.tool_call",
			Resource: toolName,
		}}
	}

	// Integration tools
	return []permissionCheck{{
		Action:          "integration.call",
		Resource:        "integration:" + toolName,
		FallbackActions: []string{"net.request"},
	}}
}

func permissionCheckCandidates(check permissionCheck) []string {
	candidates := make([]string, 0, 1+len(check.FallbackActions))
	primary := normalizePolicyAction(strings.TrimSpace(check.Action))
	if primary != "" {
		candidates = append(candidates, primary)
	}
	for _, fallback := range check.FallbackActions {
		action := normalizePolicyAction(strings.TrimSpace(fallback))
		if action == "" {
			continue
		}
		seen := false
		for _, existing := range candidates {
			if existing == action {
				seen = true
				break
			}
		}
		if !seen {
			candidates = append(candidates, action)
		}
	}
	return candidates
}
