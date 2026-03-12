package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/M4MEET/soulgate/internal/observer"
	"github.com/spf13/cobra"
)

var observeCmd = &cobra.Command{
	Use:   "observe",
	Short: "Observe Gateway events in real-time",
	Long: `Connect to the Gateway as an observer and watch events in real-time.

The observer displays:
- Incoming messages from channels (Telegram, Slack, etc.)
- Tool execution (start, progress, results)
- Agent responses
- Errors and warnings

This is useful for:
- Debugging agent behavior
- Monitoring tool execution
- Understanding message flow
- Troubleshooting issues

Example:
  soulgate observe
  soulgate observe --gateway ws://localhost:8080/ws
  soulgate observe --verbose`,
	RunE: runObserve,
}

var (
	observeGatewayURL string
	observeVerbose    bool
	observeClientID   string
)

func init() {
	rootCmd.AddCommand(observeCmd)

	observeCmd.Flags().StringVar(&observeGatewayURL, "gateway", "ws://localhost:8080/ws", "Gateway WebSocket URL")
	observeCmd.Flags().BoolVar(&observeVerbose, "verbose", false, "Show verbose output (session IDs, timestamps, etc.)")
	observeCmd.Flags().StringVar(&observeClientID, "client-id", "", "Custom client ID (auto-generated if not provided)")
}

func runObserve(cmd *cobra.Command, args []string) error {
	fmt.Println("👀 SoulGate Observer")
	fmt.Println("───────────────────────────────────────")

	// Create observer
	config := &observer.Config{
		GatewayURL: observeGatewayURL,
		ClientID:   observeClientID,
		Verbose:    observeVerbose,
	}

	obs := observer.NewObserver(config)
	defer obs.Close()

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Connect to Gateway
	if err := obs.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Gateway: %w", err)
	}

	// Start observing
	if err := obs.Start(ctx); err != nil {
		if err == context.Canceled {
			fmt.Println("\n👋 Observer stopped")
			return nil
		}
		return fmt.Errorf("observer error: %w", err)
	}

	return nil
}
