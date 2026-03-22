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
	"github.com/M4MEET/soulgate/internal/tools/computer"
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

	tools = append(tools, model.ToolSchema{
		Name:        "delegate_task",
		Description: "Delegate a complex sub-task to an isolated sub-agent. The sub-agent runs with its own context window — tool calls and intermediate results stay in its context, only the final result is returned. Use this for tasks that involve many tool calls to avoid filling the main context.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"task": {
					"type": "string",
					"description": "Description of the task to delegate"
				},
				"tools": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Optional list of tool names the sub-agent should use. If omitted, all tools are available."
				}
			},
			"required": ["task"]
		}`),
	})

	// Skill self-modification tools — allow the AI to create, list, update, and
	// learn new skills at runtime. Skills are SKILL.md files that get injected
	// into the system prompt, shaping the agent's behavior persistently.
	tools = append(tools, model.ToolSchema{
		Name:        "skill_create",
		Description: "Create a new skill that shapes your behavior persistently. The skill is written as a SKILL.md file and automatically loaded into your system prompt on future runs. Use this to learn new capabilities, remember user preferences, or build automation workflows.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "string",
					"description": "Skill identifier (lowercase, hyphens allowed, e.g. 'email-drafting')"
				},
				"content": {
					"type": "string",
					"description": "Full SKILL.md content in markdown. Must include: # Skill: <name>, ## Behavior, ## Tools, ## Examples sections."
				}
			},
			"required": ["id", "content"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "skill_list",
		Description: "List all available skills with their names and descriptions. Use this to understand what behavioral skills are currently active.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "skill_update",
		Description: "Update an existing skill's content. Use this to refine a skill based on feedback or new requirements.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "string",
					"description": "Skill identifier to update"
				},
				"content": {
					"type": "string",
					"description": "New SKILL.md content (replaces the entire file)"
				}
			},
			"required": ["id", "content"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "skill_learn",
		Description: "Learn from a user correction or preference by creating or updating a behavioral skill. Call this when the user says 'remember this', 'always do X', 'never do Y', or corrects your approach. The learning is persisted as a skill that shapes future behavior.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"lesson": {
					"type": "string",
					"description": "What you learned (e.g. 'User prefers bullet points over paragraphs')"
				},
				"category": {
					"type": "string",
					"description": "Category for the learning",
					"enum": ["preference", "correction", "workflow", "tone", "format", "integration"]
				}
			},
			"required": ["lesson", "category"]
		}`),
	})

	// Hub tools — allow the AI to search, install, and manage packages
	// (skills, plugins, connectors, MCP servers, agents) from the SoulGate Hub.
	// Plugin creation — allows the AI to create script-based tool plugins at runtime.
	// The plugin becomes a callable tool immediately after creation (no restart needed).
	tools = append(tools, model.ToolSchema{
		Name:        "plugin_create",
		Description: "Create a new script plugin that registers as a callable tool. Write a Python/Node/Bash script that reads JSON from stdin and writes JSON to stdout. The plugin becomes available immediately. Use this to extend your capabilities with custom tools.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Plugin name (lowercase, hyphens, e.g. 'weather-lookup')"
				},
				"description": {
					"type": "string",
					"description": "What this plugin does"
				},
				"language": {
					"type": "string",
					"description": "Script language",
					"enum": ["python3", "node", "bash"]
				},
				"script": {
					"type": "string",
					"description": "The script source code. Must read JSON from stdin and print JSON result to stdout."
				},
				"tool_name": {
					"type": "string",
					"description": "Name for the tool (will be exposed as pluginname__toolname)"
				},
				"tool_description": {
					"type": "string",
					"description": "Description shown to the model for this tool"
				},
				"input_schema": {
					"type": "object",
					"description": "JSON Schema for the tool's input parameters"
				},
				"requires_env": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Environment variables the script needs (e.g. ['WEATHER_API_KEY'])"
				},
				"requires_bins": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Binaries the script needs on PATH (e.g. ['curl', 'jq'])"
				}
			},
			"required": ["name", "description", "language", "script", "tool_name", "tool_description", "input_schema"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "plugin_list",
		Description: "List all installed plugins with their tools.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	})

	// Hub tools — full package lifecycle: search, install, uninstall, update, info, list.
	// Covers all 6 package types: skills, plugins, agents, MCP servers, connectors, extensions.
	tools = append(tools, model.ToolSchema{
		Name:        "hub_search",
		Description: "Search the SoulGate Hub for packages. Returns skills, plugins, connectors, MCP servers, agents, and extensions matching the query.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "Search query (matches name, description, and tags)"
				}
			},
			"required": ["query"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "hub_install",
		Description: "Install a package from the SoulGate Hub. Supports: skill, plugin, agent, mcp, connector, extension. Format: 'type/name' or 'type:name'.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"package": {
					"type": "string",
					"description": "Package identifier (e.g. 'skill/code-review', 'mcp/github', 'connector/discord', 'plugin/weather')"
				}
			},
			"required": ["package"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "hub_uninstall",
		Description: "Uninstall a locally installed Hub package.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"package": {
					"type": "string",
					"description": "Package identifier to remove (e.g. 'skill/code-review', 'mcp/github')"
				}
			},
			"required": ["package"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "hub_update",
		Description: "Update all installed Hub packages to their latest versions from the registry.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "hub_info",
		Description: "Get detailed information about a specific Hub package (version, description, author, files).",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"package": {
					"type": "string",
					"description": "Package identifier (e.g. 'skill/code-review')"
				}
			},
			"required": ["package"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "hub_list",
		Description: "List all locally installed Hub packages with their types, versions, and install dates.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
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

	// Computer / desktop-automation tools (macOS only)
	for _, s := range computer.ToolSchemas() {
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
		// No policy engine → no restrictions → show all tools
		return true
	}
	pol := o.policyEngine.GetPolicy()
	if pol == nil || len(pol.Policies) == 0 {
		// No policy rules → default-allow for tool visibility
		// (actual execution still goes through authorizeToolCall)
		return true
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
