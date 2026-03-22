package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/gateway"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/spf13/cobra"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Gateway control plane",
	Long: `Start the Gateway server - the central control plane for SoulGate.

The Gateway routes messages between:
- Chat Connectors (Telegram, Slack, etc.)
- Agent Runtime(s)
- UI / CLI observers
- Device Nodes

All components connect via WebSocket. An HTTP API (POST /api/chat) is also
served on the same port for simple integrations like the Telegram bridge.`,
}

var gatewayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Gateway server",
	Long: `Start the Gateway server with WebSocket + HTTP API.

Serves:
  ws://<addr>:<port>/ws       WebSocket for native connectors/agents
  POST /api/chat              HTTP chat endpoint (for Telegram bridge, etc.)
  GET  /api/health            Health check

Example:
  soulgate gateway start
  soulgate gateway start --port 8080
  soulgate gateway start --address 0.0.0.0 --port 9000`,
	RunE: runGatewayStart,
}

var (
	gatewayAddress      string
	gatewayPort         int
	gatewayFreshSession bool
	gatewayAuth         bool
	gatewayDevMode      bool
	gatewayRateLimit    float64
)

func init() {
	rootCmd.AddCommand(gatewayCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)

	gatewayStartCmd.Flags().StringVar(&gatewayAddress, "address", "0.0.0.0", "Gateway bind address")
	gatewayStartCmd.Flags().IntVar(&gatewayPort, "port", 8080, "Gateway port")
	gatewayStartCmd.Flags().BoolVar(&gatewayFreshSession, "fresh", false, "Clear conversation history before starting")
	gatewayStartCmd.Flags().BoolVar(&gatewayAuth, "auth", false, "Enable Bearer-token authentication for /api/* endpoints")
	gatewayStartCmd.Flags().BoolVar(&gatewayDevMode, "dev-mode", true, "Bypass auth for requests from localhost (dev convenience)")
	gatewayStartCmd.Flags().Float64Var(&gatewayRateLimit, "rate-limit", 60, "API requests per minute allowed per token")
}

// ANSI color helpers for gateway live view
const (
	gwDim     = "\033[2m"
	gwBold    = "\033[1m"
	gwCyan    = "\033[36m"
	gwGreen   = "\033[32m"
	gwYellow  = "\033[33m"
	gwBlue    = "\033[34m"
	gwMagenta = "\033[35m"
	gwReset   = "\033[0m"
)

func runGatewayStart(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Starting SoulGate Gateway...")

	// Load workspace and initialize orchestrator for the built-in /api/chat endpoint
	workspace, err := config.LoadWorkspace()
	if err != nil {
		log.Fatalf("Failed to load workspace: %v", err)
	}

	// Clear session history if --fresh
	if gatewayFreshSession {
		core.ClearSessionState(workspace.ConfigDir)
		fmt.Println("   Session: fresh start")
	}

	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		log.Fatalf("Failed to initialize orchestrator: %v", err)
	}
	defer orch.Close()

	provider, modelName := orch.GetCurrentProvider()
	fmt.Printf("   Provider: %s (%s)\n", provider, modelName)

	// Live view state — tracks whether we're mid-stream so thinking events
	// can insert a newline before printing, preventing jumbled output.
	var lv struct {
		mu        sync.Mutex
		streaming bool // true while AI text is being streamed token-by-token
	}

	// endStream closes the current streaming block if one is active.
	endStream := func() {
		if lv.streaming {
			fmt.Printf("%s\n", gwReset)
			lv.streaming = false
		}
	}

	// Enable streaming — shows AI text token-by-token with a "🤖" prefix
	orch.SetStreaming(true, func(chunk string) {
		lv.mu.Lock()
		defer lv.mu.Unlock()

		if !lv.streaming {
			// Start a new streaming block
			lv.streaming = true
			fmt.Printf("  %s🤖 %s", gwMagenta, gwReset)
		}
		fmt.Print(chunk)
	})

	// Enable live thinking output
	orch.SetThinkingCallback(func(event core.ThinkingEvent) {
		lv.mu.Lock()
		defer lv.mu.Unlock()

		switch event.Kind {
		case core.ThinkingIteration:
			endStream()
			fmt.Printf("\n  %s── iteration %d ──%s\n", gwDim, event.Iteration, gwReset)

		case core.ThinkingModelCall:
			endStream()
			fmt.Printf("  %s⟳ calling %s...%s\n", gwDim, event.Provider, gwReset)

		case core.ThinkingModelDone:
			endStream()
			m := event.Model
			if m == "" {
				m = event.Provider
			}
			fmt.Printf("  %s✓ %s%s %s(%s, %d tok, %s)%s\n",
				gwGreen, m, gwReset, gwDim, event.StopReason, event.TokensUsed, event.Duration.Round(1e6), gwReset)

		case core.ThinkingToolStart:
			endStream()
			args := event.ToolArgs
			if len(args) > 80 {
				args = args[:80] + "..."
			}
			fmt.Printf("  %s⚡ %s%s%s %s%s%s\n",
				gwYellow, gwBold, event.ToolName, gwReset,
				gwDim, args, gwReset)

		case core.ThinkingToolDone:
			result := strings.ReplaceAll(event.ToolResult, "\n", " ")
			if len(result) > 120 {
				result = result[:120] + "..."
			}
			fmt.Printf("  %s  ↳ %s %s(%s)%s\n",
				gwGreen, result, gwDim, event.Duration.Round(1e6), gwReset)

		case core.ThinkingStatus:
			endStream()
			fmt.Printf("  %s  %s%s\n", gwDim, event.Message, gwReset)
		}
	})

	// Create Gateway with built-in chat handler
	gwConfig := &gateway.Config{
		Address:           gatewayAddress,
		Port:              gatewayPort,
		SessionsDir:       "sessions",
		WebhooksFile:      filepath.Join(workspace.ConfigDir, "webhooks.json"),
		NotificationsFile: filepath.Join(workspace.ConfigDir, "notifications.json"),
		APIAuthEnabled:    gatewayAuth,
		APIDevMode:        gatewayDevMode,
		APIRateLimit:      gatewayRateLimit,
		APITokensFile:     filepath.Join(workspace.ConfigDir, "api_tokens.json"),
		OnChat: func(ctx context.Context, message string) (string, error) {
			fmt.Printf("\n%s┌─ 📨 %s%s\n", gwCyan, message, gwReset)

			result, err := orch.Run(ctx, message)

			lv.mu.Lock()
			endStream()
			lv.mu.Unlock()

			if err != nil {
				fmt.Printf("%s└─ ✗ %v%s\n\n", "\033[31m", err, gwReset)
				return "", err
			}

			response := ""
			if result != nil {
				response = result.Response
			}
			fmt.Printf("%s└─ ✓ %d chars%s\n\n", gwGreen, len(response), gwReset)

			return response, nil
		},
		API: buildGatewayAPI(orch, workspace),
	}

	gw, err := gateway.NewGateway(gwConfig)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start Gateway
	if err := gw.Start(ctx); err != nil {
		return fmt.Errorf("gateway error: %w", err)
	}

	fmt.Println("👋 Gateway stopped")
	return nil
}

// buildGatewayAPI constructs the GatewayAPI callbacks that wire the gateway's
// rich REST endpoints to the live orchestrator.  All callbacks are designed to
// be safe for concurrent HTTP requests.
func buildGatewayAPI(orch *core.Orchestrator, ws *config.Workspace) *gateway.GatewayAPI {
	return &gateway.GatewayAPI{

		// GetConfig returns a sanitised configuration snapshot.
		// API keys are masked — never exposed over the network.
		GetConfig: func() map[string]interface{} {
			cfg := ws.Config
			provider, model := orch.GetCurrentProvider()

			// Mask helper: show first 10 chars + "…" when key is present.
			mask := func(key string) string {
				if key == "" {
					return ""
				}
				if len(key) > 10 {
					return key[:10] + "..."
				}
				return "***"
			}

			return map[string]interface{}{
				"provider": provider,
				"model":    model,
				"openai": map[string]interface{}{
					"api_key":     mask(cfg.Model.OpenAI.APIKey),
					"model":       cfg.Model.OpenAI.Model,
					"base_url":    cfg.Model.OpenAI.BaseURL,
					"max_tokens":  cfg.Model.OpenAI.MaxTokens,
					"temperature": cfg.Model.OpenAI.Temperature,
				},
				"anthropic": map[string]interface{}{
					"api_key":     mask(cfg.Model.Anthropic.APIKey),
					"model":       cfg.Model.Anthropic.Model,
					"base_url":    cfg.Model.Anthropic.BaseURL,
					"max_tokens":  cfg.Model.Anthropic.MaxTokens,
					"temperature": cfg.Model.Anthropic.Temperature,
				},
				"execution": map[string]interface{}{
					"max_iterations":        cfg.Execution.MaxIterations,
					"total_timeout_sec":     cfg.Execution.TotalTimeoutSec,
					"iteration_timeout_sec": cfg.Execution.IterationTimeoutSec,
					"api_call_timeout_sec":  cfg.Execution.APICallTimeoutSec,
					"max_tokens":            cfg.Execution.MaxTokens,
					"max_tool_result_kb":    cfg.Execution.MaxToolResultKB,
				},
				"policy_rules": func() []map[string]interface{} {
					pol := orch.GetPolicyEngine().GetPolicy()
					if pol == nil {
						return []map[string]interface{}{}
					}
					rules := make([]map[string]interface{}, 0, len(pol.Policies))
					for _, r := range pol.Policies {
						rules = append(rules, map[string]interface{}{
							"name":        r.Name,
							"description": r.Description,
							"action":      r.Action,
							"resource":    r.Resource,
							"decision":    string(r.Decision),
							"priority":    r.Priority,
						})
					}
					return rules
				}(),
				"plugins": map[string]interface{}{
					"dir":        cfg.Plugins.Dir,
					"timeout":    cfg.Plugins.Timeout,
					"max_memory": cfg.Plugins.MaxMemory,
				},
			}
		},

		// SetConfig applies a single key=value update to the live provider/model.
		// Only "provider" and "model" are currently supported; unknown keys return
		// an error so callers receive a clear signal instead of a silent no-op.
		SetConfig: func(key, value string) error {
			switch key {
			case "provider":
				return orch.SetProvider(value, "")
			case "model":
				currentProvider, _ := orch.GetCurrentProvider()
				return orch.SetProvider(currentProvider, value)
			default:
				return fmt.Errorf("unknown config key %q; supported keys: provider, model", key)
			}
		},

		// GetTools returns the full tool catalog (name + description).
		// initToolRegistry is called to ensure the catalog is populated before
		// we read it; it is idempotent so calling it here is safe.
		GetTools: func() []map[string]interface{} {
			schemas := orch.GetAllToolSchemas()
			out := make([]map[string]interface{}, 0, len(schemas))
			for _, s := range schemas {
				out = append(out, map[string]interface{}{
					"name":        s.Name,
					"description": s.Description,
				})
			}
			return out
		},

		// GetAgents returns all background agents with their status.
		GetAgents: func() []map[string]interface{} {
			agents := orch.GetAgentManager().List()
			out := make([]map[string]interface{}, 0, len(agents))
			for _, a := range agents {
				entry := map[string]interface{}{
					"id":           a.ID,
					"name":         a.Name,
					"task":         a.Task,
					"status":       string(a.Status),
					"role":         string(a.Role),
					"capabilities": a.Capabilities,
					"created_at":   a.CreatedAt.Format("2006-01-02T15:04:05Z"),
				}
				if a.CompletedAt != nil {
					entry["completed_at"] = a.CompletedAt.Format("2006-01-02T15:04:05Z")
				}
				if a.Result != "" {
					r := a.Result
					if len(r) > 500 {
						r = r[:500] + "..."
					}
					entry["result"] = r
				}
				if a.Error != "" {
					entry["error"] = a.Error
				}
				if a.ParentID != "" {
					entry["parent_id"] = a.ParentID
				}
				if len(a.ChildIDs) > 0 {
					entry["child_ids"] = a.ChildIDs
				}
				out = append(out, entry)
			}
			return out
		},

		// GetMemory returns all memory entries from the global scope.
		GetMemory: func() []map[string]interface{} {
			entries := orch.GetMemoryStore().List()
			out := make([]map[string]interface{}, 0, len(entries))
			for _, e := range entries {
				m := map[string]interface{}{
					"key":          e.Key,
					"value":        e.Value,
					"created_at":   e.CreatedAt.Format("2006-01-02T15:04:05Z"),
					"updated_at":   e.UpdatedAt.Format("2006-01-02T15:04:05Z"),
					"access_count": e.AccessCount,
				}
				if len(e.Tags) > 0 {
					m["tags"] = e.Tags
				}
				if e.ExpiresAt != nil {
					m["expires_at"] = e.ExpiresAt.Format("2006-01-02T15:04:05Z")
				}
				out = append(out, m)
			}
			return out
		},

		// GetCosts returns the full cost summary from the cost tracker.
		GetCosts: func() map[string]interface{} {
			s := orch.GetCostTracker().Summary()
			return map[string]interface{}{
				"session_cost_usd": s.SessionCost,
				"today_cost_usd":   s.TodayCost,
				"total_cost_usd":   s.TotalCost,
				"by_provider":      s.ByProvider,
				"last_7_days":      s.Last7Days,
				"session_calls":    s.SessionCalls,
				"total_calls":      s.TotalCalls,
			}
		},

		// GetAudit queries the audit log and converts events to plain maps for JSON.
		GetAudit: func(limit int) []map[string]interface{} {
			ctx := context.Background()
			events, err := orch.GetAuditLogger().Query(ctx, audit.QueryFilter{Limit: limit})
			if err != nil {
				return []map[string]interface{}{}
			}
			out := make([]map[string]interface{}, 0, len(events))
			for _, ev := range events {
				m := map[string]interface{}{
					"id":        ev.ID,
					"timestamp": ev.Timestamp.Format("2006-01-02T15:04:05Z"),
					"type":      string(ev.Type),
					"category":  string(ev.Category),
					"status":    string(ev.Status),
				}
				if ev.SessionID != "" {
					m["session_id"] = ev.SessionID
				}
				if ev.RunID != "" {
					m["run_id"] = ev.RunID
				}
				if ev.Action != "" {
					m["action"] = ev.Action
				}
				if ev.Resource != "" {
					m["resource"] = ev.Resource
				}
				if ev.Error != "" {
					m["error"] = ev.Error
				}
				if len(ev.Metadata) > 0 {
					m["metadata"] = ev.Metadata
				}
				out = append(out, m)
			}
			return out
		},

		// CreateAgent spawns a new general-purpose background agent.
		CreateAgent: func(name, task string) (map[string]interface{}, error) {
			agent := orch.GetAgentManager().Create(orch, name, task, core.AgentRoleGeneral, "")
			return map[string]interface{}{
				"id":         agent.ID,
				"name":       agent.Name,
				"task":       agent.Task,
				"status":     string(agent.Status),
				"role":       string(agent.Role),
				"created_at": agent.CreatedAt.Format("2006-01-02T15:04:05Z"),
			}, nil
		},

		// StopAgent cancels a running background agent by ID.
		StopAgent: func(id string) error {
			return orch.GetAgentManager().Stop(id)
		},

		// GetAgentDetail returns full observability data for a single agent.
		GetAgentDetail: func(id string) (map[string]interface{}, error) {
			a, ok := orch.GetAgentManager().Get(id)
			if !ok {
				return nil, fmt.Errorf("agent not found: %s", id)
			}

			metrics := a.GetMetrics()
			cfg := a.GetConfig()

			logEntries := a.GetFullLog()
			logMaps := make([]map[string]interface{}, 0, len(logEntries))
			for _, e := range logEntries {
				logMaps = append(logMaps, map[string]interface{}{
					"time":    e.Time.Format("2006-01-02T15:04:05.000Z"),
					"kind":    e.Kind,
					"message": e.Message,
				})
			}

			detail := map[string]interface{}{
				"id":           a.ID,
				"name":         a.Name,
				"task":         a.Task,
				"status":       string(a.Status),
				"role":         string(a.Role),
				"capabilities": a.Capabilities,
				"created_at":   a.CreatedAt.Format("2006-01-02T15:04:05Z"),
				"metrics": map[string]interface{}{
					"tokens_used":      metrics.TokensUsed,
					"cost_usd":         metrics.CostUSD,
					"tool_call_count":  metrics.ToolCallCount,
					"model_call_count": metrics.ModelCallCount,
					"error_count":      metrics.ErrorCount,
					"avg_response_ms":  metrics.AvgResponseMs,
					"started_at":       metrics.StartedAt.Format("2006-01-02T15:04:05Z"),
					"duration":         metrics.Duration,
				},
				"config": map[string]interface{}{
					"model":           cfg.Model,
					"provider":        cfg.Provider,
					"allowed_tools":   cfg.AllowedTools,
					"max_tokens":      cfg.MaxTokens,
					"max_cost_usd":    cfg.MaxCostUSD,
					"thinking_level":  cfg.ThinkingLevel,
					"temperature":     cfg.Temperature,
					"system_prompt":   cfg.SystemPrompt,
					"timeout_seconds": cfg.TimeoutSeconds,
					"auto_restart":    cfg.AutoRestart,
				},
				"activity_log": logMaps,
				"log_count":    len(logMaps),
			}
			if a.CompletedAt != nil {
				detail["completed_at"] = a.CompletedAt.Format("2006-01-02T15:04:05Z")
			}
			if a.Result != "" {
				detail["result"] = a.Result
			}
			if a.Error != "" {
				detail["error"] = a.Error
			}
			if a.ParentID != "" {
				detail["parent_id"] = a.ParentID
			}
			if len(a.ChildIDs) > 0 {
				detail["child_ids"] = a.ChildIDs
			}
			return detail, nil
		},

		// GetAgentLog returns the last N log entries for an agent.
		GetAgentLog: func(id string, limit int) ([]map[string]interface{}, error) {
			a, ok := orch.GetAgentManager().Get(id)
			if !ok {
				return nil, fmt.Errorf("agent not found: %s", id)
			}
			var entries []core.AgentLogEntry
			if limit <= 0 {
				entries = a.GetFullLog()
			} else {
				entries = a.GetLogTail(limit)
			}
			out := make([]map[string]interface{}, 0, len(entries))
			for _, e := range entries {
				out = append(out, map[string]interface{}{
					"time":    e.Time.Format("2006-01-02T15:04:05.000Z"),
					"kind":    e.Kind,
					"message": e.Message,
				})
			}
			return out, nil
		},

		// SetAgentConfig applies configuration overrides to a running agent.
		// Only the fields present in the map are applied; unknown keys are ignored
		// so partial updates work correctly.
		SetAgentConfig: func(id string, config map[string]interface{}) error {
			a, ok := orch.GetAgentManager().Get(id)
			if !ok {
				return fmt.Errorf("agent not found: %s", id)
			}
			cfg := a.GetConfig()
			if v, ok := config["model"].(string); ok {
				cfg.Model = v
			}
			if v, ok := config["provider"].(string); ok {
				cfg.Provider = v
			}
			if v, ok := config["thinking_level"].(string); ok {
				cfg.ThinkingLevel = v
			}
			if v, ok := config["system_prompt"].(string); ok {
				cfg.SystemPrompt = v
			}
			if v, ok := config["auto_restart"].(bool); ok {
				cfg.AutoRestart = v
			}
			if v, ok := config["max_tokens"].(float64); ok {
				cfg.MaxTokens = int(v)
			}
			if v, ok := config["max_cost_usd"].(float64); ok {
				cfg.MaxCostUSD = v
			}
			if v, ok := config["temperature"].(float64); ok {
				cfg.Temperature = v
			}
			if v, ok := config["timeout_seconds"].(float64); ok {
				cfg.TimeoutSeconds = int(v)
			}
			if raw, ok := config["allowed_tools"]; ok {
				if arr, ok := raw.([]interface{}); ok {
					tools := make([]string, 0, len(arr))
					for _, item := range arr {
						if s, ok := item.(string); ok {
							tools = append(tools, s)
						}
					}
					cfg.AllowedTools = tools
				}
			}
			a.SetConfig(cfg)
			return nil
		},

		// SendAgentMessage delivers a message to the agent's inbox.
		// The sender is identified as "gateway-api" so the agent can distinguish
		// API-originated messages from agent-to-agent communication.
		SendAgentMessage: func(id string, message string) error {
			return orch.GetAgentManager().SendMessage("gateway-api", id, message)
		},

		// ListFiles delegates to the FileBroker to list a workspace directory.
		// The path is relative to the workspace root.
		ListFiles: func(path string) ([]map[string]interface{}, error) {
			ctx := context.Background()
			bCtx := brokers.BrokerContext{
				WorkspaceRoot: ws.Root,
				PluginID:      "gateway-ui",
				RunID:         "ui",
				SessionID:     "ui",
			}
			fb := orch.GetFileBroker()
			entries, err := fb.ListDir(ctx, bCtx, path)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]interface{}, 0, len(entries))
			for _, e := range entries {
				out = append(out, map[string]interface{}{
					"name":   e.Name,
					"is_dir": e.IsDir,
					"size":   e.Size,
				})
			}
			return out, nil
		},

		// ReadFile delegates to the FileBroker to read file content.
		// The path is relative to the workspace root.
		ReadFile: func(path string) (string, error) {
			ctx := context.Background()
			bCtx := brokers.BrokerContext{
				WorkspaceRoot: ws.Root,
				PluginID:      "gateway-ui",
				RunID:         "ui",
				SessionID:     "ui",
			}
			data, err := orch.GetFileBroker().ReadFile(ctx, bCtx, path)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},

		// ExecCommand delegates to the ExecBroker to run a shell command.
		// Policy enforcement and audit logging are applied by the broker.
		ExecCommand: func(command string) (string, int, error) {
			ctx := context.Background()
			bCtx := brokers.BrokerContext{
				WorkspaceRoot: ws.Root,
				PluginID:      "gateway-ui",
				RunID:         "ui",
				SessionID:     "ui",
			}
			result, err := orch.GetExecBroker().Execute(ctx, bCtx, command)
			if err != nil {
				return "", 1, err
			}
			return result.Output, result.ExitCode, nil
		},

		// GetScopedPolicies returns all scoped policy rules as plain maps.
		GetScopedPolicies: func() []map[string]interface{} {
			se := orch.GetScopedPolicyEngine()
			if se == nil {
				return []map[string]interface{}{}
			}
			rules := se.GetAllRules()
			out := make([]map[string]interface{}, 0, len(rules))
			for _, r := range rules {
				entry := map[string]interface{}{
					"name":     r.Name,
					"scope":    string(r.Scope),
					"scope_id": r.ScopeID,
					"action":   r.Action,
					"resource": r.Resource,
					"decision": string(r.Decision),
					"priority": r.Priority,
				}
				if r.Description != "" {
					entry["description"] = r.Description
				}
				if len(r.ModelRestriction) > 0 {
					entry["model_restriction"] = r.ModelRestriction
				}
				if r.PIIAction != "" {
					entry["pii_action"] = r.PIIAction
				}
				if r.TimeRestriction != nil {
					entry["time_restriction"] = map[string]interface{}{
						"allowed_days":  r.TimeRestriction.AllowedDays,
						"allowed_hours": r.TimeRestriction.AllowedHours,
						"timezone":      r.TimeRestriction.Timezone,
					}
				}
				if r.CostLimit != nil {
					entry["cost_limit"] = map[string]interface{}{
						"max_per_day":   r.CostLimit.MaxPerDay,
						"max_per_month": r.CostLimit.MaxPerMonth,
						"action":        r.CostLimit.Action,
					}
				}
				out = append(out, entry)
			}
			return out
		},

		// AddScopedPolicy converts a plain map into a ScopedRule and appends it
		// to the live scoped engine, then persists the change to disk.
		AddScopedPolicy: func(rule map[string]interface{}) error {
			se := orch.GetScopedPolicyEngine()
			if se == nil {
				return fmt.Errorf("scoped policy engine not initialised")
			}

			getString := func(m map[string]interface{}, key string) string {
				if v, ok := m[key].(string); ok {
					return v
				}
				return ""
			}
			getInt := func(m map[string]interface{}, key string) int {
				switch v := m[key].(type) {
				case float64:
					return int(v)
				case int:
					return v
				}
				return 0
			}

			sr := policy.ScopedRule{
				PolicyRule: policy.PolicyRule{
					Name:        getString(rule, "name"),
					Description: getString(rule, "description"),
					Action:      getString(rule, "action"),
					Resource:    getString(rule, "resource"),
					Decision:    policy.Decision(getString(rule, "decision")),
					Priority:    getInt(rule, "priority"),
				},
				Scope:    policy.PolicyScope(getString(rule, "scope")),
				ScopeID:  getString(rule, "scope_id"),
				PIIAction: getString(rule, "pii_action"),
			}

			if mods, ok := rule["model_restriction"].([]interface{}); ok {
				for _, m := range mods {
					if s, ok := m.(string); ok {
						sr.ModelRestriction = append(sr.ModelRestriction, s)
					}
				}
			}

			if tr, ok := rule["time_restriction"].(map[string]interface{}); ok {
				restriction := &policy.TimeRestriction{
					AllowedHours: getString(tr, "allowed_hours"),
					Timezone:     getString(tr, "timezone"),
				}
				if days, ok := tr["allowed_days"].([]interface{}); ok {
					for _, d := range days {
						if s, ok := d.(string); ok {
							restriction.AllowedDays = append(restriction.AllowedDays, s)
						}
					}
				}
				sr.TimeRestriction = restriction
			}

			if cl, ok := rule["cost_limit"].(map[string]interface{}); ok {
				limit := &policy.CostLimit{
					Action: getString(cl, "action"),
				}
				if v, ok := cl["max_per_day"].(float64); ok {
					limit.MaxPerDay = v
				}
				if v, ok := cl["max_per_month"].(float64); ok {
					limit.MaxPerMonth = v
				}
				sr.CostLimit = limit
			}

			if err := se.AddRule(sr); err != nil {
				return err
			}
			return se.Save()
		},

		// DeleteScopedPolicy removes a scoped rule by name and persists the
		// change to disk.
		DeleteScopedPolicy: func(name string) error {
			se := orch.GetScopedPolicyEngine()
			if se == nil {
				return fmt.Errorf("scoped policy engine not initialised")
			}
			if err := se.RemoveRule(name); err != nil {
				return err
			}
			return se.Save()
		},
	}
}
