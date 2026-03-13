package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/tools/cron"
	"github.com/M4MEET/soulgate/internal/tools/llmtask"
	"github.com/M4MEET/soulgate/internal/tools/patch"
	"github.com/M4MEET/soulgate/internal/tools/pdf"
	"github.com/M4MEET/soulgate/internal/tools/process"
	"github.com/M4MEET/soulgate/internal/tools/web"
)

// executeAgenticLoop runs the agentic loop: model call -> tool execution -> repeat
func (o *Orchestrator) executeAgenticLoop(ctx context.Context, userPrompt string, runID string) (string, error) {
	// Build execution limits from config, falling back to defaults for any zero values.
	execCfg := o.workspace.Config.Execution
	defaults := DefaultExecutionLimits()

	limits := ExecutionLimits{
		MaxIterations:     defaults.MaxIterations,
		TotalTimeout:      defaults.TotalTimeout,
		IterationTimeout:  defaults.IterationTimeout,
		APICallTimeout:    defaults.APICallTimeout,
		MaxTokens:         defaults.MaxTokens,
		MaxToolResultSize: defaults.MaxToolResultSize,
	}
	if execCfg.MaxIterations > 0 {
		limits.MaxIterations = execCfg.MaxIterations
	}
	if execCfg.TotalTimeoutSec > 0 {
		limits.TotalTimeout = time.Duration(execCfg.TotalTimeoutSec) * time.Second
	}
	if execCfg.IterationTimeoutSec > 0 {
		limits.IterationTimeout = time.Duration(execCfg.IterationTimeoutSec) * time.Second
	}
	if execCfg.APICallTimeoutSec > 0 {
		limits.APICallTimeout = time.Duration(execCfg.APICallTimeoutSec) * time.Second
	}
	if execCfg.MaxTokens > 0 {
		limits.MaxTokens = execCfg.MaxTokens
	}
	if execCfg.MaxToolResultKB > 0 {
		limits.MaxToolResultSize = execCfg.MaxToolResultKB * 1024
	}

	tracker := NewExecutionTracker(limits)

	// Parse directives from user message (e.g. /think high, /fast on)
	cleanedPrompt, appliedDirectives := ParseDirectives(userPrompt, o.directives)
	if len(appliedDirectives) > 0 && o.streamCallback != nil {
		for _, d := range appliedDirectives {
			o.streamCallback(fmt.Sprintf("[directive: %s]\n", d))
		}
	}
	if cleanedPrompt == "" {
		cleanedPrompt = userPrompt // fallback if all content was directives
	}

	// Initialize conversation with user message
	messages := []model.Message{
		{
			Role:    model.RoleUser,
			Content: cleanedPrompt,
		},
	}

	// Get tool schemas
	tools := o.getToolSchemas()

	// Build dynamic system prompt with workspace file injection and skills context
	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	// Get current provider and model for system prompt
	currentProvider, currentModel := o.GetCurrentProvider()
	systemPrompt := buildSystemPrompt(o.workspace.Root, o.workspace.ConfigDir, toolNames, currentProvider, currentModel)

	// Inject skills context if any skills are available
	if skillsContext := o.getSkillsContext(); skillsContext != "" {
		systemPrompt = systemPrompt + "\n\n" + skillsContext
	}

	// Agentic loop
	for {
		// BeginIteration checks max iterations and total timeout
		if err := tracker.BeginIteration(); err != nil {
			return "", err
		}

		o.emitThinking(ThinkingEvent{
			Kind:      ThinkingIteration,
			Iteration: tracker.iterations,
			Message:   fmt.Sprintf("iteration %d", tracker.iterations),
		})

		// Create completion request
		req := model.CompletionRequest{
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   o.workspace.Config.Model.OpenAI.MaxTokens,
			Temperature: o.workspace.Config.Model.OpenAI.Temperature,
			System:      systemPrompt,
		}

		// Call model provider with API call timeout
		apiCtx, apiCancel := tracker.APICallContext(ctx)
		modelCallStart := time.Now()

		o.emitThinking(ThinkingEvent{
			Kind:     ThinkingModelCall,
			Provider: o.provider.Name(),
			Message:  "calling model...",
		})

		var resp *model.CompletionResponse

		if o.streaming && o.streamCallback != nil {
			// Streaming mode: read chunks and forward to callback
			streamCh, err := o.provider.StreamComplete(apiCtx, req)
			apiCancel()
			if err != nil {
				return "", fmt.Errorf("model provider error: %w", err)
			}

			for chunk := range streamCh {
				if chunk.Error != nil {
					return "", fmt.Errorf("streaming error: %w", chunk.Error)
				}
				if chunk.Delta != "" {
					o.streamCallback(chunk.Delta)
				}
				if chunk.Done && chunk.Response != nil {
					resp = chunk.Response
				}
			}

			if resp == nil {
				return "", fmt.Errorf("streaming completed without final response")
			}
		} else {
			// Non-streaming mode
			var err error
			resp, err = o.provider.Complete(apiCtx, req)
			apiCancel()
			if err != nil {
				return "", fmt.Errorf("model provider error: %w", err)
			}
		}

		// Track actual model name from API response
		if resp.Model != "" {
			o.actualModelName = resp.Model
		}

		o.emitThinking(ThinkingEvent{
			Kind:       ThinkingModelDone,
			Provider:   o.provider.Name(),
			Model:      resp.Model,
			Duration:   time.Since(modelCallStart),
			TokensUsed: resp.Usage.TotalTokens,
			StopReason: resp.StopReason,
			Message:    fmt.Sprintf("model responded (%s)", resp.StopReason),
		})

		// Track token usage
		if err := tracker.AddTokens(resp.Usage.TotalTokens); err != nil {
			return "", err
		}

		// Log model call
		o.audit.Log(ctx, audit.NewEvent(audit.EventModelCall, audit.CategoryModel).
			WithSessionID(o.session.ID).
			WithRunID(runID).
			WithMetadata("provider", o.provider.Name()).
			WithMetadata("model", resp.Model).
			WithMetadata("stop_reason", resp.StopReason).
			WithMetadata("tokens", fmt.Sprintf("%d", resp.Usage.TotalTokens)))

		// Add assistant message to conversation
		assistantMsg := resp.Message
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		messages = append(messages, assistantMsg)

		// Check stop reason
		if resp.StopReason == model.StopReasonEndTurn || resp.StopReason == model.StopReasonMaxTokens {
			return resp.Message.Content, nil
		}

		// Handle tool calls
		if resp.StopReason == model.StopReasonToolUse && len(resp.ToolCalls) > 0 {
			// Loop detection: record and check
			for _, tc := range resp.ToolCalls {
				var args map[string]interface{}
				_ = json.Unmarshal(tc.Input, &args)
				o.loopDetector.Record(tc.Name, args)
			}
			detection := o.loopDetector.Check()
			if detection.Level == "critical" {
				return "", fmt.Errorf("loop detected: %s. %s", detection.Pattern, detection.Suggestion)
			}
			if detection.Level == "warning" && o.streaming && o.streamCallback != nil {
				o.streamCallback(fmt.Sprintf("\n[warning: %s]\n", detection.Pattern))
			}

			// Notify stream callback about tool use
			if o.streaming && o.streamCallback != nil {
				for _, tc := range resp.ToolCalls {
					o.streamCallback(fmt.Sprintf("\n[calling %s...]\n", tc.Name))
				}
			}

			toolResults := o.executeToolCallsParallel(ctx, runID, tracker, resp.ToolCalls)
			messages = append(messages, toolResults...)
			continue
		}

		return "", fmt.Errorf("unexpected stop reason: %s", resp.StopReason)
	}
}

// toolResult holds the outcome of a single tool call, keyed by its slice index
// so results can be appended in the original order after parallel execution.
type toolResult struct {
	index   int
	message model.Message
}

// executeToolCallsParallel runs all tool calls concurrently and returns the
// results as model.Messages in the same order as the input slice.
func (o *Orchestrator) executeToolCallsParallel(
	ctx context.Context,
	runID string,
	tracker *ExecutionTracker,
	toolCalls []model.ToolCall,
) []model.Message {
	results := make([]toolResult, len(toolCalls))

	var wg sync.WaitGroup
	var mu sync.Mutex // guards tracker.ValidateToolResultSize (shared mutable state)

	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, toolCall model.ToolCall) {
			defer wg.Done()

			// Abbreviate args for display
			argsSummary := string(toolCall.Input)
			if len(argsSummary) > 120 {
				argsSummary = argsSummary[:120] + "..."
			}

			o.emitThinking(ThinkingEvent{
				Kind:     ThinkingToolStart,
				ToolName: toolCall.Name,
				ToolArgs: argsSummary,
				Message:  fmt.Sprintf("calling %s", toolCall.Name),
			})

			toolStart := time.Now()
			result, err := o.executeToolCall(ctx, runID, toolCall)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// Validate (and potentially truncate) the tool result size.
			mu.Lock()
			sizeErr := tracker.ValidateToolResultSize(result)
			mu.Unlock()

			if sizeErr != nil {
				// Truncate to the limit and append a note so the model knows.
				limit := tracker.limits.MaxToolResultSize
				if len(result) > limit {
					result = result[:limit]
				}
				result = result + fmt.Sprintf(
					"\n\n[Note: tool result truncated to %d bytes; original size exceeded limit]",
					limit,
				)
			}

			// Abbreviate result for display
			resultSummary := result
			if len(resultSummary) > 200 {
				resultSummary = resultSummary[:200] + "..."
			}

			o.emitThinking(ThinkingEvent{
				Kind:       ThinkingToolDone,
				ToolName:   toolCall.Name,
				ToolResult: resultSummary,
				Duration:   time.Since(toolStart),
				Message:    fmt.Sprintf("%s completed", toolCall.Name),
			})

			results[idx] = toolResult{
				index: idx,
				message: model.Message{
					Role:       model.RoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
					Name:       toolCall.Name,
				},
			}
		}(i, tc)
	}

	wg.Wait()

	// Build ordered message slice
	messages := make([]model.Message, len(toolCalls))
	for _, r := range results {
		messages[r.index] = r.message
	}
	return messages
}

// getSkillsContext loads skills from the workspace skills directory and returns
// their content formatted for injection into the system prompt. Each skill is
// expected to live in a subdirectory of <workspaceRoot>/skills/ and contain a
// SKILL.md file. Skills that cannot be read are silently skipped.
func (o *Orchestrator) getSkillsContext() string {
	skillsDir := filepath.Join(o.workspace.Root, "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		// Skills directory doesn't exist or can't be read - this is not an error
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Skills\n\n")
	sb.WriteString("The following skills are available to guide your behaviour:\n\n")

	const maxSkillChars = 4000
	const maxTotalSkillChars = 16000
	totalChars := 0
	loaded := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}

		content := string(data)
		if len(content) > maxSkillChars {
			content = content[:maxSkillChars] + truncationMarker
		}

		if totalChars+len(content) > maxTotalSkillChars {
			sb.WriteString(fmt.Sprintf("[Note: skill '%s' skipped - total skills context too large]\n", entry.Name()))
			continue
		}

		sb.WriteString(fmt.Sprintf("### Skill: %s\n\n", entry.Name()))
		sb.WriteString(content)
		sb.WriteString("\n\n")

		totalChars += len(content)
		loaded++
	}

	if loaded == 0 {
		return ""
	}

	return sb.String()
}

// executeToolCall executes a single tool call
func (o *Orchestrator) executeToolCall(ctx context.Context, runID string, toolCall model.ToolCall) (string, error) {
	// Log tool call
	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(runID).
		WithMetadata("tool", toolCall.Name).
		WithMetadata("tool_call_id", toolCall.ID))

	// Create broker context
	brokerCtx := brokers.BrokerContext{
		WorkspaceRoot: o.workspace.Root,
		PluginID:      "builtin",
		RunID:         runID,
		SessionID:     o.session.ID,
	}

	// Route to appropriate broker
	switch toolCall.Name {
	case "files_read":
		return o.handleFileRead(ctx, brokerCtx, toolCall.Input)

	case "files_list":
		return o.handleFileList(ctx, brokerCtx, toolCall.Input)

	case "files_write":
		return o.handleFileWrite(ctx, brokerCtx, toolCall.Input)

	case "files_delete":
		return o.handleFileDelete(ctx, brokerCtx, toolCall.Input)

	case "exec_command":
		return o.handleExecCommand(ctx, brokerCtx, toolCall.Input)

	case "net_request":
		return o.handleNetRequest(ctx, brokerCtx, toolCall.Input)

	case "memory_write":
		return o.handleMemoryWrite(ctx, brokerCtx, toolCall.Input)

	case "memory_get":
		return o.handleMemoryGet(ctx, brokerCtx, toolCall.Input)

	case "memory_search":
		return o.handleMemorySearch(ctx, brokerCtx, toolCall.Input)

	case "switch_model":
		return o.handleSwitchModel(ctx, brokerCtx, toolCall.Input)

	case "agent_create":
		return o.handleAgentCreate(ctx, toolCall.Input)

	case "agent_list":
		return o.handleAgentList(ctx, toolCall.Input)

	case "agent_stop":
		return o.handleAgentStop(ctx, toolCall.Input)

	case "web_search", "web_fetch":
		return web.ExecuteTool(ctx, toolCall.Name, toolCall.Input)

	case "process_start", "process_list", "process_poll", "process_log", "process_write", "process_kill":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return process.ExecuteTool(ctx, o.processManager, toolCall.Name, args)

	case "pdf_read":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return pdf.ExecuteTool(ctx, toolCall.Name, args)

	case "cron_add", "cron_list", "cron_remove", "cron_pause", "cron_resume":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return cron.ExecuteTool(ctx, o.cronScheduler, toolCall.Name, args)

	case "llm_task":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		executor := &llmTaskExecutor{orch: o}
		return llmtask.ExecuteTool(ctx, executor, toolCall.Name, args)

	case "apply_patch":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return patch.ExecuteTool(ctx, o.workspace.Root, toolCall.Name, args)

	default:
		// Try integration tools
		return o.handleIntegrationTool(ctx, toolCall.Name, toolCall.Input)
	}
}

// getToolSchemas returns the tool schemas available to the model
func (o *Orchestrator) getToolSchemas() []model.ToolSchema {
	// Built-in tools
	tools := []model.ToolSchema{
		{
			Name:        "files_read",
			Description: "Read the contents of a file. All file operations are allowed.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to the file (e.g., 'config.yml', 'README.md')"
					}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "files_list",
			Description: "List files in a directory. All file operations are allowed.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to the directory (default: current directory)"
					}
				}
			}`),
		},
		{
			Name:        "files_write",
			Description: "Write or update a file. Can create new files or overwrite existing ones.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to the file to write"
					},
					"content": {
						"type": "string",
						"description": "Content to write to the file"
					}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        "files_delete",
			Description: "Delete a file or directory. Use with caution!",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to the file or directory to delete"
					}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "exec_command",
			Description: "Execute a shell command in the workspace directory. Returns output and exit code.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "The shell command to execute (e.g., 'ls -la', 'go build', 'npm install')"
					}
				},
				"required": ["command"]
			}`),
		},
		{
			Name:        "net_request",
			Description: "Make an HTTP request to a URL. Supports GET, POST, PUT, DELETE methods.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"method": {
						"type": "string",
						"description": "HTTP method (GET, POST, PUT, DELETE)",
						"enum": ["GET", "POST", "PUT", "DELETE"]
					},
					"url": {
						"type": "string",
						"description": "The URL to request"
					},
					"body": {
						"type": "string",
						"description": "Request body (optional, for POST/PUT)"
					},
					"headers": {
						"type": "object",
						"description": "HTTP headers (optional)",
						"additionalProperties": {"type": "string"}
					}
				},
				"required": ["method", "url"]
			}`),
		},
		{
			Name:        "memory_write",
			Description: "Save information to persistent memory (key-value pairs). Memory persists across sessions.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {
						"type": "string",
						"description": "Memory key (e.g., 'user_language', 'project_type')"
					},
					"value": {
						"type": "string",
						"description": "Value to store"
					}
				},
				"required": ["key", "value"]
			}`),
		},
		{
			Name:        "memory_get",
			Description: "Retrieve a specific value from memory by key.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"key": {
						"type": "string",
						"description": "Memory key to retrieve"
					}
				},
				"required": ["key"]
			}`),
		},
		{
			Name:        "memory_search",
			Description: "Search memory entries. Returns all entries matching the query (searches keys and values).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query (optional - omit to list all memory)"
					}
				}
			}`),
		},
		{
			Name:        "switch_model",
			Description: "Switch to a different AI model based on task requirements. Use this to optimize performance and cost. Available models: 'gpt-4.1' (latest OpenAI flagship), 'gpt-4.1-mini' (fast and economical), 'gpt-4.1-nano' (fastest/cheapest), 'o3' (deep reasoning), 'claude-sonnet' (excellent reasoning), 'claude-opus' (most capable for complex tasks). Always explain to the user why you're switching models.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"model": {
						"type": "string",
						"description": "Model to switch to. Options: 'gpt-4.1', 'gpt-4.1-mini', 'gpt-4.1-nano', 'o3', 'claude-sonnet', 'claude-opus'",
						"enum": ["gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "o3", "claude-sonnet", "claude-opus"]
					},
					"reason": {
						"type": "string",
						"description": "Explain why you're switching to this model (this will be shown to the user)"
					}
				},
				"required": ["model", "reason"]
			}`),
		},
	}

	// Agent management tools
	tools = append(tools, model.ToolSchema{
		Name:        "agent_create",
		Description: "Create a task-specific background agent. The agent runs independently with its own agentic loop and can use all tools (files, exec, net, memory). Use this when the user asks to create an agent, automate a task, or when a task benefits from running in the background.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {
					"type": "string",
					"description": "Short name for the agent (e.g., 'log-monitor', 'code-reviewer', 'test-runner')"
				},
				"task": {
					"type": "string",
					"description": "Detailed description of what the agent should do. Be specific about the task, what files to look at, what to check for, and what output to produce."
				}
			},
			"required": ["name", "task"]
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "agent_list",
		Description: "List all background agents and their status (running, completed, failed, stopped). Shows agent results when completed.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	})

	tools = append(tools, model.ToolSchema{
		Name:        "agent_stop",
		Description: "Stop a running background agent by its ID.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {
					"type": "string",
					"description": "The agent ID to stop (e.g., 'agent_1')"
				}
			},
			"required": ["id"]
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

	// Add tools from configured integrations
	integrationTools := o.integrationsReg.GetConfiguredTools()
	for _, intTool := range integrationTools {
		tools = append(tools, model.ToolSchema{
			Name:        intTool.Name,
			Description: intTool.Description,
			InputSchema: intTool.InputSchema,
		})
	}

	return tools
}

// handleIntegrationTool handles tool calls from integrations
func (o *Orchestrator) handleIntegrationTool(ctx context.Context, toolName string, input json.RawMessage) (string, error) {
	// Find which integration provides this tool
	integrationTools := o.integrationsReg.GetConfiguredTools()
	for _, tool := range integrationTools {
		if tool.Name == toolName {
			return tool.Handler(ctx, input)
		}
	}

	return "", fmt.Errorf("unknown tool: %s", toolName)
}

// llmTaskExecutor adapts the orchestrator to the llmtask.Executor interface
type llmTaskExecutor struct {
	orch *Orchestrator
}

func (e *llmTaskExecutor) Complete(ctx context.Context, prompt string, jsonMode bool) (string, error) {
	req := model.CompletionRequest{
		Messages: []model.Message{
			{Role: model.RoleUser, Content: prompt},
		},
		MaxTokens: 4096,
	}
	resp, err := e.orch.provider.Complete(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

// handleFileRead handles the files_read tool call
func (o *Orchestrator) handleFileRead(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	// Parse input
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Check permission first
	allowed, reason := o.checkOrRequestPermission(ctx, "files_read", params.Path)
	if !allowed {
		return "", fmt.Errorf("permission denied: %s", reason)
	}

	// Call file broker
	content, err := o.fileBroker.ReadFile(ctx, brokerCtx, params.Path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// handleFileList handles the files_list tool call
func (o *Orchestrator) handleFileList(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	// Parse input
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	if params.Path == "" {
		params.Path = "."
	}

	// Check permission first
	allowed, reason := o.checkOrRequestPermission(ctx, "files_list", params.Path)
	if !allowed {
		return "", fmt.Errorf("permission denied: %s", reason)
	}

	// Call file broker
	entries, err := o.fileBroker.ListDir(ctx, brokerCtx, params.Path)
	if err != nil {
		return "", err
	}

	// Format as JSON
	result, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

// handleFileWrite handles the files_write tool call
func (o *Orchestrator) handleFileWrite(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Check permission first
	allowed, reason := o.checkOrRequestPermission(ctx, "files_write", params.Path)
	if !allowed {
		return "", fmt.Errorf("permission denied: %s", reason)
	}

	err := o.fileBroker.WriteFile(ctx, brokerCtx, params.Path, []byte(params.Content))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"status": "success", "message": "File '%s' written (%d bytes)"}`, params.Path, len(params.Content)), nil
}

// handleFileDelete handles the files_delete tool call
func (o *Orchestrator) handleFileDelete(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Check permission first
	allowed, reason := o.checkOrRequestPermission(ctx, "files_delete", params.Path)
	if !allowed {
		return "", fmt.Errorf("permission denied: %s", reason)
	}

	err := o.fileBroker.DeleteFile(ctx, brokerCtx, params.Path)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"status": "success", "message": "Deleted '%s'"}`, params.Path), nil
}

// handleExecCommand handles the exec_command tool call
func (o *Orchestrator) handleExecCommand(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Check permission first
	allowed, reason := o.checkOrRequestPermission(ctx, "exec_command", params.Command)
	if !allowed {
		return "", fmt.Errorf("permission denied: %s", reason)
	}

	result, err := o.execBroker.Execute(ctx, brokerCtx, params.Command)
	if err != nil {
		return "", err
	}

	// Return as JSON
	output, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// handleNetRequest handles the net_request tool call
func (o *Orchestrator) handleNetRequest(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Body    string            `json:"body"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Check permission first
	allowed, reason := o.checkOrRequestPermission(ctx, "net_request", params.URL)
	if !allowed {
		return "", fmt.Errorf("permission denied: %s", reason)
	}

	if params.Headers == nil {
		params.Headers = make(map[string]string)
	}

	result, err := o.netBroker.Request(ctx, brokerCtx, params.Method, params.URL, params.Body, params.Headers)
	if err != nil {
		return "", err
	}

	// Return as JSON
	output, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(output), nil
}

// handleSwitchModel handles autonomous model switching by the AI
func (o *Orchestrator) handleSwitchModel(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	// Parse input
	var params struct {
		Model  string `json:"model"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Map friendly names to provider/model combinations
	modelMap := map[string]struct {
		provider  string
		modelName string
	}{
		"gpt-4.1":       {"openai", "gpt-4.1"},
		"gpt-4.1-mini":  {"openai", "gpt-4.1-mini"},
		"gpt-4.1-nano":  {"openai", "gpt-4.1-nano"},
		"o3":            {"openai", "o3"},
		"claude-sonnet": {"anthropic", "claude-sonnet-4-20250514"},
		"claude-opus":   {"anthropic", "claude-opus-4-20250514"},
	}

	// Get model info
	modelInfo, ok := modelMap[params.Model]
	if !ok {
		return "", fmt.Errorf("unknown model: %s. Available: gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, o3, claude-sonnet, claude-opus", params.Model)
	}

	// Get current provider/model for logging
	currentProvider, currentModel := o.GetCurrentProvider()

	// Check if already using this model
	if currentProvider == modelInfo.provider && currentModel == modelInfo.modelName {
		return fmt.Sprintf(`{"status": "already_active", "message": "Already using %s"}`, params.Model), nil
	}

	// Switch the provider
	if err := o.SetProvider(modelInfo.provider, modelInfo.modelName); err != nil {
		return "", fmt.Errorf("failed to switch model: %w", err)
	}

	// Log the switch
	auditCtx, cancel := auditContext()
	defer cancel()

	event := audit.NewEvent("model_switch", audit.CategoryModel).
		WithSessionID(o.session.ID).
		WithRunID(brokerCtx.RunID).
		WithMetadata("from_provider", currentProvider).
		WithMetadata("from_model", currentModel).
		WithMetadata("to_provider", modelInfo.provider).
		WithMetadata("to_model", modelInfo.modelName).
		WithMetadata("reason", params.Reason).
		WithStatus(audit.StatusSuccess)

	o.audit.Log(auditCtx, event)

	// Return success message
	return fmt.Sprintf(`{"status": "success", "model": "%s", "reason": "%s", "message": "Switched to %s. %s"}`,
		params.Model, params.Reason, params.Model, params.Reason), nil
}
