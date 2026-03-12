package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

All components connect to the Gateway via WebSocket.`,
}

var gatewayStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Gateway server",
	Long: `Start the Gateway WebSocket server.

Example:
  soulgate gateway start
  soulgate gateway start --port 8080
  soulgate gateway start --address 0.0.0.0 --port 9000`,
	RunE: runGatewayStart,
}

var (
	gatewayAddress string
	gatewayPort    int
)

func init() {
	rootCmd.AddCommand(gatewayCmd)
	gatewayCmd.AddCommand(gatewayStartCmd)

	gatewayStartCmd.Flags().StringVar(&gatewayAddress, "address", "0.0.0.0", "Gateway bind address")
	gatewayStartCmd.Flags().IntVar(&gatewayPort, "port", 8080, "Gateway port")
}

func runGatewayStart(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Starting SoulGate Gateway...")

	// Create Gateway
	config := &gateway.Config{
		Address:     gatewayAddress,
		Port:        gatewayPort,
		SessionsDir: "sessions", // Store sessions in ./sessions directory
	}

	gw, err := gateway.NewGateway(config)
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
