package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/gateway"
	"github.com/M4MEET/soulgate/internal/hub"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/spf13/cobra"
)

// maxStoredThreads is the cap on how many threads are retained in the JSON
// file.  When exceeded the oldest threads (by updatedAt) are evicted first.
const maxStoredThreads = 100

// threadStore provides thread-safe CRUD for chat threads persisted to a single
// JSON file at .soulgate/state/threads.json.
type threadStore struct {
	path string
	mu   sync.RWMutex
}

func newThreadStore(path string) *threadStore {
	return &threadStore{path: path}
}

// load reads all threads from disk. Returns an empty slice when the file does
// not yet exist.
func (s *threadStore) load() ([]map[string]interface{}, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read threads file: %w", err)
	}
	var threads []map[string]interface{}
	if err := json.Unmarshal(data, &threads); err != nil {
		// File is corrupt — start fresh rather than crashing.
		return []map[string]interface{}{}, nil
	}
	return threads, nil
}

// persist writes the slice to disk atomically using a temp-file rename.
// Caller must hold s.mu (write).
func (s *threadStore) persist(threads []map[string]interface{}) error {
	data, err := json.Marshal(threads)
	if err != nil {
		return fmt.Errorf("marshal threads: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write threads tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp) //nolint:errcheck
		return fmt.Errorf("rename threads file: %w", err)
	}
	return nil
}

// Get returns all threads ordered as stored (callers may sort on their end).
func (s *threadStore) Get() ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.load()
}

// Save inserts or replaces a thread identified by its "id" field.
// If the resulting collection exceeds maxStoredThreads the oldest entry
// (smallest "updatedAt" ISO string) is dropped.
func (s *threadStore) Save(thread map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	threads, err := s.load()
	if err != nil {
		return err
	}

	id, _ := thread["id"].(string)
	found := false
	for i, t := range threads {
		if tid, _ := t["id"].(string); tid == id {
			threads[i] = thread
			found = true
			break
		}
	}
	if !found {
		threads = append(threads, thread)
	}

	// Enforce cap: evict oldest by updatedAt when over limit.
	for len(threads) > maxStoredThreads {
		oldest := 0
		for i := 1; i < len(threads); i++ {
			a, _ := threads[oldest]["updatedAt"].(string)
			b, _ := threads[i]["updatedAt"].(string)
			if b < a {
				oldest = i
			}
		}
		threads = append(threads[:oldest], threads[oldest+1:]...)
	}

	return s.persist(threads)
}

// Delete removes the thread with the given id. Returns nil if not found.
func (s *threadStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	threads, err := s.load()
	if err != nil {
		return err
	}
	filtered := threads[:0]
	for _, t := range threads {
		if tid, _ := t["id"].(string); tid != id {
			filtered = append(filtered, t)
		}
	}
	return s.persist(filtered)
}

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

	// Thread store — persists web UI chat threads across restarts.
	threads := newThreadStore(filepath.Join(workspace.ConfigDir, "state", "threads.json"))

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

	// Wire heartbeat callback so notifications appear inline in the live view.
	hb := orch.GetHeartbeat()
	hb.SetCallback(func(msg string) {
		lv.mu.Lock()
		defer lv.mu.Unlock()
		endStream()
		fmt.Printf("\n%s Heartbeat: %s%s\n", gwCyan, msg, gwReset)
	})

	// Create Gateway with built-in chat handler
	gwConfig := &gateway.Config{
		Address:           gatewayAddress,
		Port:              gatewayPort,
		SessionsDir:       "sessions",
		Provider:          provider,
		Model:             modelName,
		WebhooksFile:      filepath.Join(workspace.ConfigDir, "webhooks.json"),
		NotificationsFile: filepath.Join(workspace.ConfigDir, "notifications.json"),
		APIAuthEnabled:    gatewayAuth,
		APIDevMode:        gatewayDevMode,
		APIRateLimit:      gatewayRateLimit,
		APITokensFile:     filepath.Join(workspace.ConfigDir, "security", "tokens.json"),
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
		OnStreamChat: func(ctx context.Context, message string, events chan<- gateway.ThinkingEvent) (string, error) {
			// Set up temporary thinking callback that pipes to SSE events
			origThinking := orch.GetThinkingCallback()
			origStream := orch.IsStreaming()

			orch.SetThinkingCallback(func(evt core.ThinkingEvent) {
				// Also call original (for console output)
				if origThinking != nil {
					origThinking(evt)
				}
				// Map to gateway ThinkingEvent for SSE
				switch evt.Kind {
				case core.ThinkingIteration:
					events <- gateway.ThinkingEvent{Kind: "iteration", Message: evt.Message}
				case core.ThinkingModelCall:
					events <- gateway.ThinkingEvent{Kind: "model_call", Message: evt.Message, Data: evt.Provider}
				case core.ThinkingModelDone:
					events <- gateway.ThinkingEvent{Kind: "model_done", Message: evt.Message, Data: evt.Model, Tokens: evt.TokensUsed}
				case core.ThinkingToolStart:
					events <- gateway.ThinkingEvent{Kind: "tool_start", Message: evt.ToolName, Data: evt.ToolArgs}
				case core.ThinkingToolDone:
					events <- gateway.ThinkingEvent{Kind: "tool_done", Message: evt.ToolName, Data: evt.ToolResult}
				case core.ThinkingStatus:
					events <- gateway.ThinkingEvent{Kind: "status", Message: evt.Message}
				}
			})

			// Enable streaming and pipe chunks as SSE events
			orch.SetStreaming(true, func(chunk string) {
				events <- gateway.ThinkingEvent{Kind: "stream", Message: chunk}
			})

			result, err := orch.Run(ctx, message)

			// Restore original callbacks
			orch.SetThinkingCallback(origThinking)
			orch.SetStreaming(origStream, nil)

			if err != nil {
				return "", err
			}
			if result == nil {
				return "", nil
			}
			return result.Response, nil
		},
		API: buildGatewayAPI(orch, workspace, threads),
	}

	gw, err := gateway.NewGateway(gwConfig)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	// Auto-spawn saved connectors from config
	if len(workspace.Config.Connectors) > 0 {
		// Spawn after a short delay so the gateway is listening
		go func() {
			// Wait for the gateway to start listening
			for i := 0; i < 20; i++ {
				_, err := http.Get(fmt.Sprintf("http://localhost:%d/api/health", gatewayPort))
				if err == nil {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			for connType, connCfg := range workspace.Config.Connectors {
				msg, err := spawnConnectorProcess(connType, connCfg, gatewayPort)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: auto-start %s connector: %v\n", connType, err)
				} else {
					fmt.Printf("   Auto-started: %s\n", msg)
					// Track the spawned connector
					if pidIdx := strings.Index(msg, "(pid "); pidIdx >= 0 {
						var pid int
						if _, scanErr := fmt.Sscanf(msg[pidIdx:], "(pid %d)", &pid); scanErr == nil && pid > 0 {
							gw.TrackConnector(connType, pid)
						}
					}
				}
			}
		}()
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
func buildGatewayAPI(orch *core.Orchestrator, ws *config.Workspace, ts *threadStore) *gateway.GatewayAPI {
	// Initialize OpenRouter catalog for live provider/model discovery
	catalogDir := filepath.Join(ws.ConfigDir, "cache")
	orCatalog := model.NewOpenRouterCatalog(catalogDir)

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

		// SetConfig applies a single key=value update to the live provider/model
		// and persists the change to disk so it survives restarts.
		SetConfig: func(key, value string) error {
			var err error
			switch key {
			case "provider":
				err = orch.SetProvider(value, "")
			case "model":
				currentProvider, _ := orch.GetCurrentProvider()
				err = orch.SetProvider(currentProvider, value)
			default:
				return fmt.Errorf("unknown config key %q; supported keys: provider, model", key)
			}
			if err != nil {
				return err
			}
			// Persist to disk
			configPath := filepath.Join(ws.ConfigDir, "config.yml")
			if saveErr := ws.Config.Save(configPath); saveErr != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to persist config: %v\n", saveErr)
			}
			return nil
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
					"type":         "string",
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
				"by_model":         s.ByModel,
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
					"timestamp": e.Time.Format("2006-01-02T15:04:05Z07:00"),
					"type":      e.Kind,
					"message":   e.Message,
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
					"timeout_seconds":  cfg.TimeoutSeconds,
					"auto_restart":     cfg.AutoRestart,
					"schedule_enabled": cfg.ScheduleEnabled,
					"schedule_cron":    cfg.ScheduleCron,
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
					"timestamp": e.Time.Format("2006-01-02T15:04:05Z07:00"),
					"type":      e.Kind,
					"message":   e.Message,
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
			if v, ok := config["schedule_enabled"].(bool); ok {
				cfg.ScheduleEnabled = v
			}
			if v, ok := config["schedule_cron"].(string); ok {
				cfg.ScheduleCron = v
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

		// GetAgentMessages returns pending messages in an agent's inbox.
		GetAgentMessages: func(id string) ([]map[string]interface{}, error) {
			msgs, err := orch.GetAgentManager().GetMessages(id)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]interface{}, 0, len(msgs))
			for _, m := range msgs {
				out = append(out, map[string]interface{}{
					"from_id":   m.FromID,
					"from_name": m.FromName,
					"message":   m.Message,
					"timestamp": m.Timestamp,
				})
			}
			return out, nil
		},

		// RestartAgent stops and re-creates an agent with the same config.
		RestartAgent: func(id string) (map[string]interface{}, error) {
			newAgent, err := orch.GetAgentManager().Restart(orch, id)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"status":    "restarted",
				"old_id":    id,
				"new_id":    newAgent.ID,
				"name":      newAgent.Name,
				"task":      newAgent.Task,
				"role":      string(newAgent.Role),
			}, nil
		},

		// ActivateStandby puts an agent into standby listening mode.
		ActivateStandby: func(id string) (map[string]interface{}, error) {
			agent, err := orch.GetAgentManager().ActivateStandby(orch, id)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"status": "standby_activated",
				"id":     agent.ID,
				"name":   agent.Name,
				"task":   agent.Task,
			}, nil
		},

		// DeleteAgent permanently removes an agent.
		DeleteAgent: func(id string) error {
			return orch.GetAgentManager().Delete(id)
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

		// ListApprovals returns all pending approval requests as plain maps
		// suitable for JSON transport to the web UI.
		ListApprovals: func() []map[string]interface{} {
			ab := orch.GetApprovalBroker()
			if ab == nil {
				return []map[string]interface{}{}
			}
			pending := ab.ListPending()
			out := make([]map[string]interface{}, 0, len(pending))
			for _, req := range pending {
				m := map[string]interface{}{
					"id":           req.ID,
					"action":       req.Action,
					"resource":     req.Resource,
					"reason":       req.Reason,
					"requested_by": req.RequestedBy,
					"requested_at": req.RequestedAt.Format("2006-01-02T15:04:05Z"),
					"status":       req.Status,
					"expires_at":   req.ExpiresAt.Format("2006-01-02T15:04:05Z"),
				}
				out = append(out, m)
			}
			return out
		},

		// ApproveRequest approves the pending approval request identified by id.
		ApproveRequest: func(id, decidedBy string) error {
			ab := orch.GetApprovalBroker()
			if ab == nil {
				return fmt.Errorf("approval broker not available")
			}
			return ab.Approve(id, decidedBy)
		},

		// DenyRequest denies the pending approval request identified by id.
		DenyRequest: func(id, decidedBy string) error {
			ab := orch.GetApprovalBroker()
			if ab == nil {
				return fmt.Errorf("approval broker not available")
			}
			return ab.Deny(id, decidedBy)
		},

		// GetHeartbeatStatus returns a snapshot of the heartbeat state for the
		// /api/heartbeat endpoint.
		GetHeartbeatStatus: func() map[string]interface{} {
			hb := orch.GetHeartbeat()
			s := hb.Status()
			m := map[string]interface{}{
				"enabled":    s.Enabled,
				"running":    s.Running,
				"interval":   s.Interval,
				"run_count":  s.RunCount,
			}
			if !s.LastRun.IsZero() {
				m["last_run"] = s.LastRun.Format("2006-01-02T15:04:05Z")
			}
			if !s.NextRun.IsZero() {
				m["next_run"] = s.NextRun.Format("2006-01-02T15:04:05Z")
			}
			if s.LastResult != "" {
				m["last_result"] = s.LastResult
			}
			return m
		},

		// RunHeartbeatNow triggers an immediate heartbeat check.
		RunHeartbeatNow: func() (string, error) {
			return orch.GetHeartbeat().RunNow()
		},

		// ToggleHeartbeat enables or disables the heartbeat.
		ToggleHeartbeat: func(enabled bool) bool {
			hb := orch.GetHeartbeat()
			if enabled {
				hb.Start()
			} else {
				hb.Stop()
			}
			return enabled
		},

		// GetThreads returns all persisted web UI chat threads.
		GetThreads: func() ([]map[string]interface{}, error) {
			return ts.Get()
		},

		// SaveThread inserts or updates a single chat thread.
		SaveThread: func(thread map[string]interface{}) error {
			return ts.Save(thread)
		},

		// DeleteThread removes a chat thread by id.
		DeleteThread: func(id string) error {
			return ts.Delete(id)
		},

		// ListProviders returns all providers from the live OpenRouter catalog,
		// merged with the local registry.
		ListProviders: func() []string {
			providers, err := orCatalog.GetProviders(context.Background())
			if err != nil || len(providers) == 0 {
				// Fallback to local registry
				return model.AllProviderNames()
			}
			// Build a deduplicated list: OpenRouter providers + local-only ones
			seen := make(map[string]bool)
			var result []string
			for _, p := range providers {
				if !seen[p.ID] {
					seen[p.ID] = true
					result = append(result, p.ID)
				}
			}
			// Add local-only providers (e.g. ollama, azure, custom)
			for _, name := range model.AllProviderNames() {
				if !seen[name] {
					seen[name] = true
					result = append(result, name)
				}
			}
			return result
		},

		// ListModels returns models for a provider from the live catalog.
		ListModels: func(provider string) []map[string]interface{} {
			models, err := orCatalog.GetModels(context.Background(), provider)
			if err != nil || len(models) == 0 {
				// Fallback to local registry
				regModels := model.ModelsFromRegistryPublic(provider)
				out := make([]map[string]interface{}, 0, len(regModels))
				for _, m := range regModels {
					out = append(out, map[string]interface{}{
						"id":          m.ID,
						"name":        m.Name,
						"description": m.Description,
						"provider":    m.Provider,
					})
				}
				return out
			}
			out := make([]map[string]interface{}, 0, len(models))
			for _, m := range models {
				entry := map[string]interface{}{
					"id":       m.ID,
					"name":     m.Name,
					"provider": m.Provider,
				}
				if m.Description != "" {
					entry["description"] = m.Description
				}
				if m.ContextLength > 0 {
					entry["context_length"] = m.ContextLength
				}
				out = append(out, entry)
			}
			return out
		},

		// GetAPIKeyStatus returns which providers have API keys configured.
		GetAPIKeyStatus: func() map[string]bool {
			status := make(map[string]bool)
			// Check all known providers
			allProviders, _ := orCatalog.GetProviders(context.Background())
			for _, p := range allProviders {
				status[p.ID] = ws.Config.ResolveAPIKey(p.ID) != ""
			}
			// Also check local registry providers
			for _, name := range model.AllProviderNames() {
				if _, ok := status[name]; !ok {
					status[name] = ws.Config.ResolveAPIKey(name) != ""
				}
			}
			return status
		},

		// SetAPIKey saves an API key for a provider and persists to config.
		SetAPIKey: func(provider, key string) error {
			ws.Config.SetAPIKey(provider, key)
			// Save config to disk
			configPath := filepath.Join(ws.ConfigDir, "config.yml")
			if err := ws.Config.Save(configPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			return nil
		},

		// SpawnConnector starts a connector process in the background.
		// Saves credentials to config so connectors auto-start on next boot.
		SpawnConnector: func(connectorType string, cfg map[string]string) (string, error) {
			// Persist credentials
			if ws.Config.Connectors == nil {
				ws.Config.Connectors = make(map[string]map[string]string)
			}
			ws.Config.Connectors[connectorType] = cfg
			configPath := filepath.Join(ws.ConfigDir, "config.yml")
			_ = ws.Config.Save(configPath)

			return spawnConnectorProcess(connectorType, cfg, gatewayPort)
		},

		// Hub integration — uses hub.NewHub() for all operations.
		HubSearch: func(query string) ([]map[string]interface{}, error) {
			h := hub.NewHub(ws.Root)
			pkgs, err := h.Search(query)
			if err != nil {
				return nil, err
			}
			results := make([]map[string]interface{}, 0, len(pkgs))
			for _, p := range pkgs {
				results = append(results, map[string]interface{}{
					"name":        p.Name,
					"type":        string(p.Type),
					"kind":        string(p.Kind),
					"description": p.Description,
					"version":     p.Version,
					"author":      p.Author,
					"tags":        p.Tags,
				})
			}
			return results, nil
		},

		HubInstall: func(name string) error {
			h := hub.NewHub(ws.Root)
			return h.Install(name)
		},

		HubUninstall: func(name string) error {
			h := hub.NewHub(ws.Root)
			return h.Uninstall(name)
		},

		HubInstalled: func() []map[string]interface{} {
			h := hub.NewHub(ws.Root)
			items, err := h.List()
			if err != nil {
				return nil
			}
			results := make([]map[string]interface{}, 0, len(items))
			for _, item := range items {
				results = append(results, map[string]interface{}{
					"type":         string(item.Type),
					"kind":         string(item.Kind),
					"name":         item.Name,
					"version":      item.Version,
					"installed_at": item.InstalledAt,
				})
			}
			return results
		},
	}
}

// spawnConnectorProcess launches a connector as a background process.
// It locates the soulgate binary and runs `soulgate connector <type>` with
// the appropriate environment variables set.
func spawnConnectorProcess(connectorType string, cfg map[string]string, port int) (string, error) {
	// Find the soulgate binary
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find soulgate binary: %w", err)
	}

	// Telegram uses WebSocket, all other connectors use HTTP.
	gatewayURL := fmt.Sprintf("http://localhost:%d", port)
	if connectorType == "telegram" {
		gatewayURL = fmt.Sprintf("ws://localhost:%d/ws", port)
	}

	// Build command args
	args := []string{"connector", connectorType, "--gateway", gatewayURL}

	// Build environment: inherit current env + add connector-specific vars
	env := os.Environ()

	switch connectorType {
	case "telegram":
		token := cfg["token"]
		if token == "" {
			return "", fmt.Errorf("telegram connector requires 'token' (bot token)")
		}
		env = append(env, "TELEGRAM_BOT_TOKEN="+token)

	case "discord":
		token := cfg["token"]
		if token == "" {
			return "", fmt.Errorf("discord connector requires 'token' (bot token)")
		}
		env = append(env, "DISCORD_BOT_TOKEN="+token)

	case "slack":
		appToken := cfg["app_token"]
		botToken := cfg["bot_token"]
		if appToken == "" || botToken == "" {
			return "", fmt.Errorf("slack connector requires 'app_token' and 'bot_token'")
		}
		env = append(env, "SLACK_APP_TOKEN="+appToken, "SLACK_BOT_TOKEN="+botToken)

	default:
		// For other connectors, pass all config as env vars with
		// SOULGATE_CONNECTOR_ prefix
		for k, v := range cfg {
			envKey := "SOULGATE_CONNECTOR_" + strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
			env = append(env, envKey+"="+v)
		}
	}

	// Start as a background process
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start %s connector: %w", connectorType, err)
	}

	// Detach — don't wait for the process
	go func() {
		_ = cmd.Wait()
	}()

	return fmt.Sprintf("%s connector started (pid %d)", connectorType, cmd.Process.Pid), nil
}
