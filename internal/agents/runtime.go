package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/M4MEET/soulgate/internal/skills"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Runtime is the agent runtime that connects to the Gateway
type Runtime struct {
	config     *Config
	gatewayURL string
	clientID   string
	conn       *websocket.Conn
	done       chan struct{}
	workspace  *config.Workspace
	provider   model.Provider
	tools      []model.ToolSchema
	fileBroker *files.Broker
	skills     []skills.Skill
	skillsDir  string
}

// Config holds agent runtime configuration
type Config struct {
	GatewayURL string
	ClientID   string
	Workspace  *config.Workspace
	Skills     []string // Skill IDs to load
}

// NewRuntime creates a new agent runtime
func NewRuntime(cfg *Config) (*Runtime, error) {
	if cfg.ClientID == "" {
		cfg.ClientID = fmt.Sprintf("agent-%s", uuid.New().String()[:8])
	}

	// Initialize model provider
	provider, err := initializeProvider(cfg.Workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	// Initialize file broker (simplified - no policy/audit for now)
	fileBroker, err := files.NewBroker(cfg.Workspace.Root, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize file broker: %w", err)
	}

	// Get tool schemas
	tools := getToolSchemas()

	// Load skills if specified
	skillsDir := filepath.Join(cfg.Workspace.Root, "skills")
	var loadedSkills []skills.Skill

	if len(cfg.Skills) > 0 {
		loader := skills.NewLoader(skillsDir)
		loadedSkills, err = loader.LoadByIDs(cfg.Skills)
		if err != nil {
			// Warning only - don't fail if skills can't be loaded
			fmt.Printf("Warning: failed to load skills: %v\n", err)
		} else {
			fmt.Printf("✓ Loaded %d skills: %v\n", len(loadedSkills), cfg.Skills)
		}
	}

	return &Runtime{
		config:     cfg,
		gatewayURL: cfg.GatewayURL,
		clientID:   cfg.ClientID,
		done:       make(chan struct{}),
		workspace:  cfg.Workspace,
		provider:   provider,
		tools:      tools,
		fileBroker: fileBroker,
		skills:     loadedSkills,
		skillsDir:  skillsDir,
	}, nil
}

// Connect connects to the Gateway
func (r *Runtime) Connect(ctx context.Context) error {
	u, err := url.Parse(r.gatewayURL)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}

	fmt.Printf("🔌 Connecting to Gateway at %s...\n", r.gatewayURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	r.conn = conn

	// Send connect frame
	connectFrame := &protocol.ConnectFrame{
		Type:     protocol.FrameConnect,
		Role:     protocol.RoleAgent,
		ClientID: r.clientID,
		Version:  protocol.ProtocolVersion,
		Metadata: protocol.Metadata{
			"provider": r.provider.Name(),
			"tools":    len(r.tools),
		},
		Timestamp: time.Now().Unix(),
	}

	if err := r.sendFrame(connectFrame); err != nil {
		return fmt.Errorf("failed to send connect frame: %w", err)
	}

	// Wait for connect.ack
	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read connect.ack: %w", err)
	}

	frame, err := protocol.ParseFrame(message)
	if err != nil {
		return fmt.Errorf("failed to parse connect.ack: %w", err)
	}

	ackFrame, ok := frame.(*protocol.ConnectAckFrame)
	if !ok {
		return fmt.Errorf("expected connect.ack, got %T", frame)
	}

	fmt.Printf("✓ Connected as %s\n", ackFrame.ClientID)
	fmt.Printf("✓ Provider: %s\n", r.provider.Name())
	fmt.Printf("✓ Tools: %d available\n", len(r.tools))
	fmt.Println("🤖 Agent ready to process messages...")

	return nil
}

// Start starts the agent runtime
func (r *Runtime) Start(ctx context.Context) error {
	if r.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Start read loop
	go r.readLoop(ctx)

	// Wait for context cancellation or done signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.done:
		return nil
	}
}

// readLoop reads and processes frames from the Gateway
func (r *Runtime) readLoop(ctx context.Context) {
	defer close(r.done)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := r.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("\n❌ Connection error: %v\n", err)
			}
			return
		}

		// Parse frame
		frame, err := protocol.ParseFrame(message)
		if err != nil {
			fmt.Printf("❌ Failed to parse frame: %v\n", err)
			continue
		}

		// Handle frame
		if err := r.handleFrame(ctx, frame); err != nil {
			fmt.Printf("❌ Failed to handle frame: %v\n", err)
		}
	}
}

// handleFrame handles different frame types
func (r *Runtime) handleFrame(ctx context.Context, frame interface{}) error {
	switch f := frame.(type) {
	case *protocol.EventMessageFrame:
		return r.handleEventMessage(ctx, f)

	case *protocol.CmdApproveFrame:
		// TODO: Handle approval
		return nil

	case *protocol.CmdRejectFrame:
		// TODO: Handle rejection
		return nil

	case *protocol.Frame:
		// Handle ping/pong
		if f.Type == protocol.FramePing {
			pongFrame := &protocol.Frame{
				Type:      protocol.FramePong,
				Timestamp: time.Now().Unix(),
			}
			return r.sendFrame(pongFrame)
		}
		return nil

	default:
		return fmt.Errorf("unsupported frame type: %T", frame)
	}
}

// handleEventMessage processes an incoming message
func (r *Runtime) handleEventMessage(ctx context.Context, frame *protocol.EventMessageFrame) error {
	fmt.Printf("📨 Processing message from %s: %s\n", frame.Sender.Username, frame.Text)

	// Run agent loop
	response, err := r.runAgentLoop(ctx, frame)
	if err != nil {
		// Send error back
		errorFrame := &protocol.EventErrorFrame{
			Type:      protocol.FrameEventError,
			SessionID: frame.SessionID,
			Error:     fmt.Sprintf("Agent error: %v", err),
			Timestamp: time.Now().Unix(),
		}
		r.sendFrame(errorFrame)
		return err
	}

	// Send response back to channel
	responseFrame := &protocol.CmdChannelSendFrame{
		Type:           protocol.FrameCmdChannelSend,
		Channel:        frame.Channel,
		ConversationID: frame.ConversationID,
		Text:           response,
		SessionID:      frame.SessionID,
		Timestamp:      time.Now().Unix(),
	}

	return r.sendFrame(responseFrame)
}

// runAgentLoop executes the agent loop: model call → tool execution → repeat
func (r *Runtime) runAgentLoop(ctx context.Context, messageFrame *protocol.EventMessageFrame) (string, error) {
	const maxIterations = 20

	// Initialize conversation with user message
	messages := []model.Message{
		{
			Role:    model.RoleUser,
			Content: messageFrame.Text,
		},
	}

	// Build system prompt
	systemPrompt := r.buildSystemPrompt()

	// Agent loop
	for i := 0; i < maxIterations; i++ {
		// Create completion request
		req := model.CompletionRequest{
			Messages:    messages,
			Tools:       r.tools,
			MaxTokens:   r.workspace.Config.Model.OpenAI.MaxTokens,
			Temperature: r.workspace.Config.Model.OpenAI.Temperature,
			System:      systemPrompt,
		}

		// Call model provider
		resp, err := r.provider.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("model provider error: %w", err)
		}

		// Add assistant message to conversation
		assistantMsg := resp.Message
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		messages = append(messages, assistantMsg)

		// Check stop reason
		if resp.StopReason == model.StopReasonEndTurn || resp.StopReason == model.StopReasonMaxTokens {
			// Model finished - return response
			fmt.Printf("🤖 Generated response: %s\n\n", resp.Message.Content)
			return resp.Message.Content, nil
		}

		// Handle tool calls
		if resp.StopReason == model.StopReasonToolUse && len(resp.ToolCalls) > 0 {
			// Execute tool calls
			for _, toolCall := range resp.ToolCalls {
				result, err := r.executeToolCall(ctx, messageFrame.SessionID, toolCall)
				if err != nil {
					result = fmt.Sprintf("Error: %v", err)
				}

				// Add tool result to conversation
				messages = append(messages, model.Message{
					Role:       model.RoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
					Name:       toolCall.Name,
				})
			}

			// Continue loop to get next model response
			continue
		}

		// Unknown stop reason
		return "", fmt.Errorf("unexpected stop reason: %s", resp.StopReason)
	}

	return "", fmt.Errorf("max iterations reached (%d)", maxIterations)
}

// executeToolCall executes a single tool call
func (r *Runtime) executeToolCall(ctx context.Context, sessionID string, toolCall model.ToolCall) (string, error) {
	toolID := uuid.New().String()

	// Parse tool input
	var args map[string]interface{}
	if err := json.Unmarshal(toolCall.Input, &args); err != nil {
		args = make(map[string]interface{})
	}

	// Emit tool.start event
	startFrame := &protocol.EventToolStartFrame{
		Type:      protocol.FrameEventToolStart,
		SessionID: sessionID,
		ToolName:  toolCall.Name,
		ToolID:    toolID,
		Args:      args,
		Timestamp: time.Now().Unix(),
	}
	r.sendFrame(startFrame)

	fmt.Printf("🔧 Executing tool: %s\n", toolCall.Name)

	startTime := time.Now()

	// Execute tool
	result, err := r.executeTool(ctx, sessionID, toolID, toolCall)

	duration := time.Since(startTime).Milliseconds()

	// Emit tool.end event
	endFrame := &protocol.EventToolEndFrame{
		Type:      protocol.FrameEventToolEnd,
		SessionID: sessionID,
		ToolName:  toolCall.Name,
		ToolID:    toolID,
		Result:    result,
		Duration:  duration,
		Timestamp: time.Now().Unix(),
	}

	// Add metadata based on tool type
	if toolCall.Name == "read_file" && err == nil {
		endFrame.BytesRead = int64(len(result))
	}

	if err != nil {
		endFrame.Error = err.Error()
		// Try to extract error code from error type
		if strings.Contains(err.Error(), "permission denied") {
			endFrame.ErrorCode = "PERMISSION_DENIED"
		} else if strings.Contains(err.Error(), "not found") {
			endFrame.ErrorCode = "NOT_FOUND"
		}
		fmt.Printf("❌ Tool failed: %v\n", err)
	} else {
		fmt.Printf("✅ Tool completed in %dms\n", duration)
	}

	r.sendFrame(endFrame)

	return result, err
}

// executeTool routes tool execution to appropriate broker
func (r *Runtime) executeTool(ctx context.Context, sessionID, toolID string, toolCall model.ToolCall) (string, error) {
	// Parse input JSON
	var input map[string]interface{}
	if err := json.Unmarshal(toolCall.Input, &input); err != nil {
		return "", fmt.Errorf("invalid tool input: %w", err)
	}

	brokerCtx := brokers.BrokerContext{
		WorkspaceRoot: r.workspace.Root,
		PluginID:      "builtin",
	}

	switch toolCall.Name {
	case "read_file":
		// Extract file path from input
		path, ok := input["path"].(string)
		if !ok {
			return "", fmt.Errorf("invalid path argument")
		}

		return r.executeReadFile(ctx, brokerCtx, path, toolCall)

	case "list_files":
		// Extract directory path from input
		path, ok := input["path"].(string)
		if !ok {
			path = "."
		}

		fileList, err := r.fileBroker.ListDir(ctx, brokerCtx, path)
		if err != nil {
			return "", err
		}

		// Format file list
		var result string
		for _, file := range fileList {
			result += fmt.Sprintf("%s\n", file.Name)
		}

		return result, nil

	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Name)
	}
}

// executeReadFile executes the read_file tool with enhanced event streaming
func (r *Runtime) executeReadFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string, toolCall model.ToolCall) (string, error) {
	// Get session ID and tool ID from context (we need to extract these)
	// For now, we'll emit output events as we read the file
	content, err := r.fileBroker.ReadFile(ctx, brokerCtx, path)
	if err != nil {
		return "", err
	}

	// Convert to string
	contentStr := string(content)

	// Emit output info if content is large (> 1000 chars)
	if len(contentStr) > 1000 {
		// Split into lines to get line count
		lines := strings.Split(contentStr, "\n")
		fmt.Printf("   📄 Reading file: %s (%d lines, %d bytes)\n", path, len(lines), len(content))
	}

	return contentStr, nil
}

// buildSystemPrompt builds the system prompt for the agent
func (r *Runtime) buildSystemPrompt() string {
	prompt := fmt.Sprintf(`You are an AI assistant running in SoulGate.

You have access to the following tools:
- read_file: Read file contents
- list_files: List files in a directory

Current workspace: %s

Be helpful and concise in your responses.`, r.workspace.Root)

	// Inject skills context if available
	if len(r.skills) > 0 {
		skillsContext := skills.BuildSkillContext(r.skills)
		prompt += "\n\n" + skillsContext
	}

	return prompt
}

// sendFrame sends a frame to the Gateway
func (r *Runtime) sendFrame(frame interface{}) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	return r.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the agent runtime connection
func (r *Runtime) Close() error {
	if r.conn != nil {
		err := r.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			return err
		}
		return r.conn.Close()
	}
	return nil
}

// Helper functions

func initializeProvider(workspace *config.Workspace) (model.Provider, error) {
	cfg := workspace.Config

	switch cfg.Model.DefaultProvider {
	case "openai":
		if cfg.Model.OpenAI.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}
		return openai.NewProvider(cfg.Model.OpenAI.APIKey, cfg.Model.OpenAI.Model, cfg.Model.OpenAI.BaseURL), nil

	case "anthropic":
		if cfg.Model.Anthropic.APIKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured")
		}
		return anthropic.NewProvider(cfg.Model.Anthropic.APIKey, cfg.Model.Anthropic.Model, ""), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", cfg.Model.DefaultProvider)
	}
}

func getToolSchemas() []model.ToolSchema {
	readFileSchema, _ := json.Marshal(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	})

	listFilesSchema, _ := json.Marshal(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the directory to list (default: current directory)",
			},
		},
	})

	return []model.ToolSchema{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: readFileSchema,
		},
		{
			Name:        "list_files",
			Description: "List files in a directory",
			InputSchema: listFilesSchema,
		},
	}
}
