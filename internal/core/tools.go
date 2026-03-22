package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/model"
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

	// Seed conversation with prior history (if any) plus the new user message
	userMsg := model.Message{
		Role:    model.RoleUser,
		Content: cleanedPrompt,
	}
	prior := o.GetConversationHistory()
	messages := make([]model.Message, 0, len(prior)+1)
	messages = append(messages, prior...)
	messages = append(messages, userMsg)
	o.appendToHistory(userMsg)

	// Initialize tool registry with full catalog (lazy loading — only always-on
	// tools are sent to the model; others are loaded via search_available_tools).
	o.initToolRegistry()

	// Build dynamic system prompt with workspace file injection and skills context
	allToolNames := o.toolRegistry.AllToolNames()

	// Get current provider and model for system prompt
	currentProvider, currentModel := o.GetCurrentProvider()
	systemPrompt := buildSystemPrompt(o.workspace.Root, o.workspace.ConfigDir, allToolNames, currentProvider, currentModel)

	// Inject skills context if any skills are available
	if skillsContext := buildSkillsSection(o.workspace.Root); skillsContext != "" {
		systemPrompt = systemPrompt + "\n\n" + skillsContext
	}

	// Inject MCP context (resources and prompts from connected servers)
	if mcpContext := o.getMCPContext(); mcpContext != "" {
		systemPrompt = systemPrompt + "\n\n" + mcpContext
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

		// Get active tools (refreshed each iteration so search_available_tools
		// activations take effect immediately).
		// Optimization: skip tools on first iteration if the message looks like
		// simple chat (short, no action words). Saves ~400 tokens per casual message.
		var tools []model.ToolSchema
		if tracker.iterations == 1 && isCasualChat(cleanedPrompt) {
			tools = nil
		} else {
			tools = o.getToolSchemas()
		}

		// Create completion request
		req := model.CompletionRequest{
			Messages:    messages,
			Tools:       tools,
			MaxTokens:   o.workspace.Config.Model.OpenAI.MaxTokens,
			Temperature: o.workspace.Config.Model.OpenAI.Temperature,
			System:      systemPrompt,
		}

		// Call model provider
		modelCallStart := time.Now()

		o.emitThinking(ThinkingEvent{
			Kind:     ThinkingModelCall,
			Provider: o.provider.Name(),
			Message:  "calling model...",
		})

		resp, err := o.callModelWithRetry(ctx, tracker, req)
		if err != nil {
			return "", fmt.Errorf("model provider error: %w", err)
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
		o.emitThinking(ThinkingEvent{
			Kind:       ThinkingTokenUsage,
			TokensUsed: tracker.tokensUsed,
			Message:    fmt.Sprintf("total tokens used: %d", tracker.tokensUsed),
		})

		// Log model call
		o.audit.Log(ctx, audit.NewEvent(audit.EventModelCall, audit.CategoryModel).
			WithSessionID(o.session.ID).
			WithRunID(runID).
			WithMetadata("provider", o.provider.Name()).
			WithMetadata("model", resp.Model).
			WithMetadata("stop_reason", resp.StopReason).
			WithMetadata("tokens", fmt.Sprintf("%d", resp.Usage.TotalTokens)))

		// Record cost for this model call.
		// resp.Model is the actual model ID returned by the API (may differ from config).
		// CachedTokens is not tracked in the common TokenUsage struct; pass 0 for providers
		// that do not report caching (the cost formula degenerates gracefully).
		if o.costTracker != nil {
			modelID := resp.Model
			if modelID == "" {
				_, modelID = o.GetCurrentProvider()
			}
			o.costTracker.Record(
				o.provider.Name(),
				modelID,
				o.session.ID,
				resp.Usage.PromptTokens,
				resp.Usage.CompletionTokens,
				0, // cached tokens not yet in common schema
			)
		}

		// Add assistant message to conversation
		assistantMsg := resp.Message
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = resp.ToolCalls
		}
		messages = append(messages, assistantMsg)
		o.appendToHistory(assistantMsg)

		// Check stop reason
		if resp.StopReason == model.StopReasonEndTurn || resp.StopReason == model.StopReasonMaxTokens {
			return resp.Message.Content, nil
		}

		// Handle tool calls
		if resp.StopReason == model.StopReasonToolUse && len(resp.ToolCalls) > 0 {
			o.emitThinking(ThinkingEvent{
				Kind:    ThinkingStatus,
				Message: fmt.Sprintf("executing %d tool call(s) in parallel", len(resp.ToolCalls)),
			})

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

			o.emitThinking(ThinkingEvent{
				Kind:    ThinkingStatus,
				Message: "tool execution complete, continuing",
			})

			continue
		}

		return "", fmt.Errorf("unexpected stop reason: %s", resp.StopReason)
	}
}

// isCasualChat returns true if the message looks like simple conversation
// that doesn't need tools (greetings, questions, chitchat). This lets us
// skip sending tool schemas entirely, saving ~400 tokens.
func isCasualChat(msg string) bool {
	msg = strings.TrimSpace(msg)

	// Short messages are almost always casual
	if len(msg) < 60 {
		lower := strings.ToLower(msg)

		// Quick check: contains action-oriented words → not casual
		actionWords := []string{
			"create", "build", "make", "write", "read", "delete", "run",
			"execute", "install", "deploy", "fix", "search", "find",
			"list", "show me", "open", "edit", "update", "fetch",
			"download", "upload", "send", "start", "stop", "kill",
			"file", "code", "script", "command", "project",
		}
		for _, w := range actionWords {
			if strings.Contains(lower, w) {
				return false
			}
		}
		return true
	}
	return false
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
