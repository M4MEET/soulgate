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

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/gateway"
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
)

func init() {
	rootCmd.AddCommand(gatewayCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)

	gatewayStartCmd.Flags().StringVar(&gatewayAddress, "address", "0.0.0.0", "Gateway bind address")
	gatewayStartCmd.Flags().IntVar(&gatewayPort, "port", 8080, "Gateway port")
	gatewayStartCmd.Flags().BoolVar(&gatewayFreshSession, "fresh", false, "Clear conversation history before starting")
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
