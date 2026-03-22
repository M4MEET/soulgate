package core

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/mcp"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/M4MEET/soulgate/internal/tools/browser"
	"github.com/M4MEET/soulgate/internal/tools/canvas"
	"github.com/M4MEET/soulgate/internal/tools/cron"
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

	if err := o.authorizeToolCall(ctx, runID, toolCall); err != nil {
		return "", err
	}

	// Route to appropriate broker
	switch toolCall.Name {
	case "search_available_tools":
		return o.handleSearchAvailableTools(ctx, toolCall.Input)

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

	case "soulgate_introspect":
		return o.handleSoulGateIntrospect(ctx, toolCall.Input)

	case "soulgate_configure":
		return o.handleSoulGateConfigure(ctx, brokerCtx, toolCall.Input)

	case "agent_create":
		return o.handleAgentCreate(ctx, toolCall.Input)

	case "agent_list":
		return o.handleAgentList(ctx, toolCall.Input)

	case "agent_stop":
		return o.handleAgentStop(ctx, toolCall.Input)

	case "agent_delegate":
		return o.handleAgentDelegate(ctx, toolCall.Input)

	case "agent_message":
		return o.handleAgentMessage(ctx, toolCall.Input)

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

	case "voice_speak", "voice_transcribe":
		return voice.ExecuteTool(ctx, o.workspace.Root, o.workspace.Config.Model.OpenAI.APIKey, toolCall.Name, toolCall.Input)

	case "image_generate", "image_edit":
		return imagegen.ExecuteTool(ctx, o.workspace.Root, o.workspace.Config.Model.OpenAI.APIKey, toolCall.Name, toolCall.Input)

	case "memory_index", "memory_recall", "memory_forget":
		if o.vectorStore != nil {
			var args map[string]interface{}
			if err := json.Unmarshal(toolCall.Input, &args); err != nil {
				return "", fmt.Errorf("invalid tool input: %w", err)
			}
			return embeddings.ExecuteTool(ctx, o.vectorStore, toolCall.Name, args)
		}
		return "", fmt.Errorf("semantic memory not available (requires OPENAI_API_KEY for embeddings)")

	case "cron_add", "cron_list", "cron_remove", "cron_pause", "cron_resume":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return cron.ExecuteTool(ctx, o.cronScheduler, toolCall.Name, args)

	case "watch_start", "watch_list", "watch_stop":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return filewatcher.ExecuteTool(ctx, o.watchManager, toolCall.Name, args)

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

	case "code_run", "code_install":
		return sandbox.ExecuteTool(ctx, toolCall.Name, toolCall.Input)

	case "browser_open", "browser_screenshot", "browser_click", "browser_type", "browser_eval", "browser_html":
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return browser.ExecuteTool(ctx, o.browserManager, toolCall.Name, args)

	case "canvas_create", "canvas_update", "canvas_list", "canvas_preview":
		if o.canvasManager == nil {
			return "", fmt.Errorf("canvas manager not available")
		}
		var args map[string]interface{}
		if err := json.Unmarshal(toolCall.Input, &args); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return canvas.ExecuteTool(ctx, o.canvasManager, o.canvasPreviewMgr, toolCall.Name, args)

	case "mcp_read_resource":
		return o.handleMCPReadResource(ctx, runID, toolCall.Input)

	case "mcp_get_prompt":
		return o.handleMCPGetPrompt(ctx, runID, toolCall.Input)

	case "email_send":
		return email.ExecuteTool(ctx, toolCall.Name, toolCall.Input)

	case "git_status", "git_diff", "git_log", "git_commit", "git_branch", "git_stash":
		return gittools.ExecuteTool(ctx, o.workspace.Root, toolCall.Name, toolCall.Input)

	default:
		// Try plugin tools (prefixed with pluginname__)
		if o.pluginManager != nil && o.pluginManager.IsPluginTool(toolCall.Name) {
			return o.pluginManager.ExecuteTool(ctx, toolCall.Name, toolCall.Input)
		}

		// Try MCP tools (prefixed with servername__)
		if o.mcpManager != nil && mcp.IsMCPTool(toolCall.Name) {
			return o.handleMCPToolCall(ctx, runID, toolCall)
		}

		// Try integration tools
		return o.handleIntegrationTool(ctx, toolCall.Name, toolCall.Input)
	}
}

// handleSearchAvailableTools handles the search_available_tools meta-tool.
// It searches the full tool catalog and activates matching tools so they
// become available to the model in the next iteration.
func (o *Orchestrator) handleSearchAvailableTools(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	matched := o.toolRegistry.Search(params.Query)
	if len(matched) == 0 {
		// Return all available tool names as a hint
		allNames := o.toolRegistry.AllToolNames()
		return fmt.Sprintf("No tools matched '%s'. Available tools: %s",
			params.Query, strings.Join(allNames, ", ")), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d tool(s) matching '%s'. These tools are now available:\n\n", len(matched), params.Query))
	for _, t := range matched {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", t.Name, t.Description))
	}
	return sb.String(), nil
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

// getMCPContext builds system prompt context from MCP servers (resources and prompts)
func (o *Orchestrator) getMCPContext() string {
	if o.mcpManager == nil {
		return ""
	}

	resources := o.mcpManager.GetAllResources()
	prompts := o.mcpManager.GetAllPrompts()
	servers := o.mcpManager.ListServers()

	if len(resources) == 0 && len(prompts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## MCP (Model Context Protocol)\n\n")

	// Server status summary
	var running []string
	for _, s := range servers {
		if s.Running {
			running = append(running, fmt.Sprintf("%s (%d tools, %d resources, %d prompts)",
				s.Name, s.Tools, s.Resources, s.Prompts))
		}
	}
	if len(running) > 0 {
		sb.WriteString("Connected MCP servers: " + strings.Join(running, ", ") + "\n\n")
	}

	// List available resources
	if len(resources) > 0 {
		sb.WriteString("### Available Resources\n")
		for _, r := range resources {
			sb.WriteString(fmt.Sprintf("- **%s** (`%s`)", r.Name, r.URI))
			if r.Description != "" {
				sb.WriteString(": " + r.Description)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// List available prompts
	if len(prompts) > 0 {
		sb.WriteString("### Available Prompts\n")
		for _, p := range prompts {
			sb.WriteString(fmt.Sprintf("- **%s**", p.Name))
			if p.Description != "" {
				sb.WriteString(": " + p.Description)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// handleMCPReadResource fetches an MCP resource by URI from the connected servers.
func (o *Orchestrator) handleMCPReadResource(ctx context.Context, runID string, input json.RawMessage) (string, error) {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid mcp_read_resource input: %w", err)
	}
	if params.URI == "" {
		return "", fmt.Errorf("mcp_read_resource: uri is required")
	}

	if o.mcpManager == nil {
		return "", fmt.Errorf("mcp_read_resource: no MCP manager initialised")
	}

	result, err := o.mcpManager.ReadResource(ctx, params.URI)
	if err != nil {
		return "", fmt.Errorf("mcp_read_resource: %w", err)
	}

	// Log the access
	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(runID).
		WithMetadata("tool", "mcp_read_resource").
		WithMetadata("uri", params.URI).
		WithMetadata("source", "mcp").
		WithStatus(audit.StatusSuccess))

	// Concatenate text content blocks; include blob sizes as a note.
	var sb strings.Builder
	for _, c := range result.Contents {
		if c.Text != "" {
			sb.WriteString(c.Text)
			sb.WriteString("\n")
		} else if c.Blob != "" {
			sb.WriteString(fmt.Sprintf("[binary blob: %d base64 chars, mimeType=%s]\n", len(c.Blob), c.MIMEType))
		}
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// handleMCPGetPrompt renders a named MCP prompt template with optional arguments.
func (o *Orchestrator) handleMCPGetPrompt(ctx context.Context, runID string, input json.RawMessage) (string, error) {
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid mcp_get_prompt input: %w", err)
	}
	if params.Name == "" {
		return "", fmt.Errorf("mcp_get_prompt: name is required")
	}

	if o.mcpManager == nil {
		return "", fmt.Errorf("mcp_get_prompt: no MCP manager initialised")
	}

	result, err := o.mcpManager.GetPrompt(ctx, params.Name, params.Arguments)
	if err != nil {
		return "", fmt.Errorf("mcp_get_prompt: %w", err)
	}

	// Log the access
	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(runID).
		WithMetadata("tool", "mcp_get_prompt").
		WithMetadata("prompt", params.Name).
		WithMetadata("source", "mcp").
		WithStatus(audit.StatusSuccess))

	// Format rendered messages for the model.
	var sb strings.Builder
	if result.Description != "" {
		sb.WriteString("Description: ")
		sb.WriteString(result.Description)
		sb.WriteString("\n\n")
	}
	for _, msg := range result.Messages {
		sb.WriteString(fmt.Sprintf("[%s]: ", msg.Role))
		if msg.Content.Type == "text" {
			sb.WriteString(msg.Content.Text)
		} else if msg.Content.Type == "resource" && msg.Content.Resource != nil {
			sb.WriteString(msg.Content.Resource.Text)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

// handleMCPToolCall routes a tool call to the appropriate MCP server
func (o *Orchestrator) handleMCPToolCall(ctx context.Context, runID string, toolCall model.ToolCall) (string, error) {
	// Parse arguments
	var args map[string]interface{}
	if err := json.Unmarshal(toolCall.Input, &args); err != nil {
		return "", fmt.Errorf("invalid MCP tool input: %w", err)
	}

	// Call through MCP manager
	result, err := o.mcpManager.CallTool(ctx, toolCall.Name, args)
	if err != nil {
		return "", fmt.Errorf("MCP tool %s failed: %w", toolCall.Name, err)
	}

	// Log MCP tool call
	o.audit.Log(ctx, audit.NewEvent(audit.EventToolExecute, audit.CategoryTool).
		WithSessionID(o.session.ID).
		WithRunID(runID).
		WithMetadata("tool", toolCall.Name).
		WithMetadata("source", "mcp").
		WithStatus(audit.StatusSuccess))

	// Convert MCP result to string
	if result.IsError {
		var errTexts []string
		for _, c := range result.Content {
			if c.Type == "text" {
				errTexts = append(errTexts, c.Text)
			}
		}
		return "", fmt.Errorf("MCP tool error: %s", strings.Join(errTexts, "; "))
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}
	return strings.Join(texts, "\n"), nil
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

// maxFileReadChars caps file content returned to the model to avoid token explosion.
const maxFileReadChars = 32000 // ~8k tokens

// handleFileRead handles the files_read tool call
func (o *Orchestrator) handleFileRead(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	// Parse input
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	// Call file broker
	content, err := o.fileBroker.ReadFile(ctx, brokerCtx, params.Path)
	if err != nil {
		return "", err
	}

	result := string(content)
	if len(result) > maxFileReadChars {
		result = result[:maxFileReadChars] + "\n\n[truncated — file exceeds 32KB display limit]"
	}
	return result, nil
}

// hiddenEntries are files/dirs that waste tokens in listings.
var hiddenEntries = map[string]bool{
	".DS_Store": true, ".git": true, ".svn": true, ".hg": true,
	"node_modules": true, "__pycache__": true, ".pytest_cache": true,
	".mypy_cache": true, "Thumbs.db": true, ".idea": true, ".vscode": true,
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

	// Call file broker
	entries, err := o.fileBroker.ListDir(ctx, brokerCtx, params.Path)
	if err != nil {
		return "", err
	}

	// Compact text output: "dir/" or "file (size)" — much fewer tokens than JSON
	var sb strings.Builder
	for _, e := range entries {
		if hiddenEntries[e.Name] {
			continue
		}
		if e.IsDir {
			sb.WriteString(e.Name)
			sb.WriteString("/\n")
		} else {
			sb.WriteString(e.Name)
			sb.WriteByte('\n')
		}
	}

	return sb.String(), nil
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

// handleSoulGateIntrospect lets the AI inspect its own state
func (o *Orchestrator) handleSoulGateIntrospect(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Section string `json:"section"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		params.Section = "all"
	}
	if params.Section == "" {
		params.Section = "all"
	}

	var sb strings.Builder

	showAll := params.Section == "all"

	if showAll || params.Section == "status" {
		sb.WriteString("## Status\n")
		provider, modelName := o.GetCurrentProvider()
		sb.WriteString(fmt.Sprintf("Provider: %s\nModel: %s\n", provider, modelName))
		sb.WriteString(fmt.Sprintf("Session: %s\n", o.session.ID))
		sb.WriteString(fmt.Sprintf("Workspace: %s\n", o.workspace.Root))
		sb.WriteString(fmt.Sprintf("Config dir: %s\n", o.workspace.ConfigDir))
		sb.WriteString(fmt.Sprintf("Trust mode: %v\n", o.IsTrustMode()))
		stats := o.GetConversationHistory()
		sb.WriteString(fmt.Sprintf("Conversation history: %d messages\n", len(stats)))
		sb.WriteString("\n")
	}

	if showAll || params.Section == "source" {
		sb.WriteString("## Extension Points\n")
		sb.WriteString(fmt.Sprintf("Workspace: %s\n", o.workspace.Root))
		sb.WriteString("\nSafe to create/modify:\n")
		sb.WriteString("  - skills/<name>/SKILL.md        (teach yourself new behaviors)\n")
		sb.WriteString("  - extensions/<name>.sh|.py|.js  (create reusable scripts/tools)\n")
		sb.WriteString("  - plugins/<name>/manifest.yml   (WASM plugin definitions)\n")
		sb.WriteString("  - .soulgate/config.yml          (runtime configuration)\n")
		sb.WriteString("  - .soulgate/policy.yml          (security rules)\n")
		sb.WriteString("  - .soulgate/SOUL.md             (identity & guidelines)\n")
		sb.WriteString("  - .soulgate/MEMORY.md           (persistent memory)\n")
		sb.WriteString("\nProtected (read-only — report bugs to user):\n")
		sb.WriteString("  - internal/   (core Go source)\n")
		sb.WriteString("  - cmd/        (CLI entry points)\n")
		sb.WriteString("  - go.mod/sum  (dependencies)\n")
		sb.WriteString("\n")
	}

	if showAll || params.Section == "config" {
		sb.WriteString("## Config\n")
		cfg := o.workspace.Config
		sb.WriteString(fmt.Sprintf("Config file: %s/config.yml\n", o.workspace.ConfigDir))
		sb.WriteString(fmt.Sprintf("Default provider: %s\n", cfg.Model.DefaultProvider))
		sb.WriteString(fmt.Sprintf("OpenAI model: %s\n", cfg.Model.OpenAI.Model))
		sb.WriteString(fmt.Sprintf("Anthropic model: %s\n", cfg.Model.Anthropic.Model))
		sb.WriteString(fmt.Sprintf("Max tokens: %d\n", cfg.Model.OpenAI.MaxTokens))
		sb.WriteString(fmt.Sprintf("Temperature: %.1f\n", cfg.Model.OpenAI.Temperature))
		sb.WriteString(fmt.Sprintf("Execution limits: iterations=%d timeout=%ds tokens=%d\n",
			cfg.Execution.MaxIterations, cfg.Execution.TotalTimeoutSec, cfg.Execution.MaxTokens))
		sb.WriteString(fmt.Sprintf("Max tool result: %d KB\n", cfg.Execution.MaxToolResultKB))
		sb.WriteString(fmt.Sprintf("Audit: enabled=%v path=%s\n", cfg.Audit.Enabled, cfg.Audit.DatabasePath))
		sb.WriteString(fmt.Sprintf("Policy: %s\n", cfg.Policy.FilePath))
		sb.WriteString("\n")
	}

	if showAll || params.Section == "tools" {
		sb.WriteString("## Available Tools\n")
		tools := o.GetAvailableToolNames()
		for _, name := range tools {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		sb.WriteString(fmt.Sprintf("\nTotal: %d tools\n", len(tools)))
		sb.WriteString("\n")
	}

	if showAll || params.Section == "providers" {
		sb.WriteString("## Registered Providers\n")
		for _, name := range model.AllProviderNames() {
			def, _ := model.LookupProvider(name)
			sb.WriteString(fmt.Sprintf("  - %s (protocol: %s, default: %s, env: %s)\n",
				name, def.Protocol, def.DefaultModel, def.EnvKey))
		}
		sb.WriteString("\n")
	}

	if showAll || params.Section == "policy" {
		sb.WriteString("## Policy\n")
		if o.policyEngine != nil {
			pol := o.policyEngine.GetPolicy()
			if pol != nil && len(pol.Policies) > 0 {
				for _, rule := range pol.Policies {
					sb.WriteString(fmt.Sprintf("  [%d] %s: %s on %s → %s\n",
						rule.Priority, rule.Name, rule.Action, rule.Resource, rule.Decision))
				}
			} else {
				sb.WriteString("  No policy rules loaded (default-deny)\n")
			}
		} else {
			sb.WriteString("  Policy engine not initialized\n")
		}
		sb.WriteString(fmt.Sprintf("\nPolicy file: %s\n", o.workspace.Config.Policy.FilePath))
		sb.WriteString("You can add/edit/remove rules by editing the policy file or using soulgate_configure.\n")
		sb.WriteString("\n")
	}

	sb.WriteString("## How to Extend\n")
	sb.WriteString("Safe ways to add capabilities:\n")
	sb.WriteString("  - New skill: files_write skills/<name>/SKILL.md (loaded into prompt automatically)\n")
	sb.WriteString("  - New script: files_write extensions/<name>.sh, then exec_command to run it\n")
	sb.WriteString("  - Config change: soulgate_configure or files_write .soulgate/config.yml + reload\n")
	sb.WriteString("  - Policy rule: soulgate_configure action=add_policy_rule\n")
	sb.WriteString("  - Switch model: soulgate_configure action=set_provider or action=set_model\n")
	sb.WriteString("\nDO NOT modify: internal/, cmd/, go.mod, go.sum, Makefile\n")
	sb.WriteString("If you find a core bug, report the file + line + fix to the user.\n")

	return sb.String(), nil
}

// handleSoulGateConfigure lets the AI modify its own config at runtime
func (o *Orchestrator) handleSoulGateConfigure(ctx context.Context, brokerCtx brokers.BrokerContext, input json.RawMessage) (string, error) {
	var params struct {
		Action string `json:"action"`
		Key    string `json:"key"`
		Value  string `json:"value"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	switch params.Action {
	case "set_provider":
		if params.Value == "" {
			return "", fmt.Errorf("value required: provider name")
		}
		if err := o.SetProvider(params.Value, params.Key); err != nil {
			return "", err
		}
		return fmt.Sprintf("Provider set to %s (model: %s)", params.Value, params.Key), nil

	case "set_model":
		if params.Value == "" {
			return "", fmt.Errorf("value required: model name")
		}
		provider, _ := o.GetCurrentProvider()
		if err := o.SetProvider(provider, params.Value); err != nil {
			return "", err
		}
		return fmt.Sprintf("Model set to %s", params.Value), nil

	case "set_execution_limit":
		cfg := &o.workspace.Config.Execution
		switch params.Key {
		case "max_iterations":
			val := 0
			fmt.Sscanf(params.Value, "%d", &val)
			cfg.MaxIterations = val
		case "total_timeout_sec":
			val := 0
			fmt.Sscanf(params.Value, "%d", &val)
			cfg.TotalTimeoutSec = val
		case "max_tokens":
			val := 0
			fmt.Sscanf(params.Value, "%d", &val)
			cfg.MaxTokens = val
		case "max_tool_result_kb":
			val := 0
			fmt.Sscanf(params.Value, "%d", &val)
			cfg.MaxToolResultKB = val
		default:
			return "", fmt.Errorf("unknown execution limit: %s (available: max_iterations, total_timeout_sec, max_tokens, max_tool_result_kb)", params.Key)
		}
		return fmt.Sprintf("Execution limit %s set to %s (takes effect next iteration)", params.Key, params.Value), nil

	case "add_policy_rule":
		if o.policyEngine == nil {
			return "", fmt.Errorf("policy engine not initialized")
		}
		// Value format: "action resource decision priority"
		var action, resource, decision string
		var priority int
		n, _ := fmt.Sscanf(params.Value, "%s %s %s %d", &action, &resource, &decision, &priority)
		if n < 3 {
			return "", fmt.Errorf("value format: 'action resource decision [priority]' (e.g., 'exec.command command:* allow 100')")
		}
		ruleName := params.Key
		if ruleName == "" {
			ruleName = fmt.Sprintf("auto-%s-%s", action, decision)
		}
		o.policyEngine.AddRule(policy.PolicyRule{
			Name:     ruleName,
			Action:   action,
			Resource: resource,
			Decision: policy.Decision(decision),
			Priority: priority,
		})
		return fmt.Sprintf("Policy rule '%s' added: %s on %s → %s (priority %d)", ruleName, action, resource, decision, priority), nil

	case "remove_policy_rule":
		if o.policyEngine == nil {
			return "", fmt.Errorf("policy engine not initialized")
		}
		if params.Key == "" {
			return "", fmt.Errorf("key required: rule name to remove")
		}
		o.policyEngine.RemoveRule(params.Key)
		return fmt.Sprintf("Policy rule '%s' removed", params.Key), nil

	case "reload_config":
		// Reload config from disk
		configPath := filepath.Join(o.workspace.ConfigDir, "config.yml")
		newCfg, err := config.LoadConfig(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to reload config: %w", err)
		}
		o.workspace.Config = newCfg

		// Reload policy
		policyPath := o.workspace.Config.Policy.FilePath
		pol, err := policy.LoadPolicy(policyPath)
		if err == nil && pol != nil {
			o.policyEngine = policy.NewEngine(pol)
			if o.policyEngine != nil {
				o.policyEngine.SetBypassChecker(o.IsTrustMode)
			}
		}

		// Re-initialize provider with new config
		provider, err := initializeProvider(newCfg)
		if err == nil {
			o.provider = provider
		}

		return "Config and policy reloaded from disk", nil

	case "reload_plugins":
		if o.pluginManager != nil {
			if err := o.pluginManager.Reload(); err != nil {
				return "", fmt.Errorf("plugin reload failed: %w", err)
			}
			names := o.pluginManager.ListPlugins()
			return fmt.Sprintf("Plugins reloaded. %d loaded: %v", len(names), names), nil
		}
		return "No plugin manager", nil

	default:
		return "", fmt.Errorf("unknown action: %s (available: set_provider, set_model, add_policy_rule, remove_policy_rule, set_execution_limit, reload_config, reload_plugins)", params.Action)
	}
}
