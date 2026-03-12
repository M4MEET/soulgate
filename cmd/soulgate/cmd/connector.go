package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/M4MEET/soulgate/internal/connectors/telegram"
	"github.com/spf13/cobra"
)

var connectorCmd = &cobra.Command{
	Use:   "connector",
	Short: "Connector commands",
	Long: `Run connectors that bridge messaging platforms to the Gateway.

Connectors:
- Telegram: Connect Telegram bot to Gateway
- More coming: Slack, Discord, WhatsApp, etc.`,
}

var connectorTelegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Start Telegram connector",
	Long: `Start the Telegram connector that bridges Telegram to the Gateway.

The connector:
1. Connects to Gateway as a 'channel' role client
2. Listens for incoming Telegram messages
3. Forwards messages to Gateway as event.message frames
4. Receives cmd.channel.send frames from Gateway
5. Sends responses back to Telegram users

Requirements:
- TELEGRAM_BOT_TOKEN environment variable
- Running Gateway instance

Example:
  export TELEGRAM_BOT_TOKEN="your-bot-token"
  soulgate connector telegram --gateway ws://localhost:8080/ws`,
	RunE: runConnectorTelegram,
}

var (
	connectorGatewayURL string
	connectorClientID   string
)

func init() {
	rootCmd.AddCommand(connectorCmd)
	connectorCmd.AddCommand(connectorTelegramCmd)

	connectorTelegramCmd.Flags().StringVar(&connectorGatewayURL, "gateway", "ws://localhost:8080/ws", "Gateway WebSocket URL")
	connectorTelegramCmd.Flags().StringVar(&connectorClientID, "client-id", "", "Custom client ID (auto-generated if not provided)")
}

func runConnectorTelegram(cmd *cobra.Command, args []string) error {
	fmt.Println("📱 Starting Telegram Connector...")
	fmt.Println("─────────────────────────────────")

	// Check for bot token
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable not set")
	}

	// Create connector
	config := &telegram.Config{
		GatewayURL: connectorGatewayURL,
		BotToken:   botToken,
		ClientID:   connectorClientID,
	}

	connector, err := telegram.NewConnector(config)
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}
	defer connector.Close()

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Connect to Gateway
	if err := connector.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Gateway: %w", err)
	}

	// Start connector
	if err := connector.Start(ctx); err != nil {
		if err == context.Canceled {
			fmt.Println("\n👋 Connector stopped")
			return nil
		}
		return fmt.Errorf("connector error: %w", err)
	}

	return nil
}
