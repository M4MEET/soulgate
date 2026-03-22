package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers/messaging"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/M4MEET/soulgate/internal/plugins/loader"
	"github.com/M4MEET/soulgate/internal/policy"

	// Import plugins to register them
	_ "github.com/M4MEET/soulgate/plugins/telegram"

	"github.com/spf13/cobra"
)

var messagingCmd = &cobra.Command{
	Use:   "messaging",
	Short: "Messaging platform integrations",
	Long: `Connect SoulGate to messaging platforms via plugins.

Messaging channels are provided by plugins. Install a messaging plugin
(e.g., telegram, whatsapp, discord) to enable that platform.

Example:
  # Start all registered messaging channels
  soulgate messaging start

  # Send a message through a specific channel
  soulgate messaging send telegram <chat-id> <message>

  # List available messaging channels
  soulgate messaging list`,
}

var messagingStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start all messaging channels",
	Long:  "Start listening for messages on all registered messaging channel plugins",
	RunE:  runMessagingStart,
}

var messagingSendCmd = &cobra.Command{
	Use:   "send <channel> <recipient> <message>",
	Short: "Send a message via a specific channel",
	Args:  cobra.ExactArgs(3),
	RunE:  runMessagingSend,
}

var messagingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available messaging channels",
	RunE:  runMessagingList,
}

func init() {
	rootCmd.AddCommand(messagingCmd)
	messagingCmd.AddCommand(messagingStartCmd)
	messagingCmd.AddCommand(messagingSendCmd)
	messagingCmd.AddCommand(messagingListCmd)
}

func runMessagingStart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Initialize policy engine
	policyData, err := policy.LoadPolicy(workspace.Config.Policy.FilePath)
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}
	policyEngine := policy.NewEngine(policyData)

	// Initialize audit logger
	auditLogger, err := audit.NewJSONLLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
	}
	defer auditLogger.Close()

	// Create orchestrator
	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Create message broker
	msgBroker := messaging.NewMessageBroker(policyEngine, auditLogger)

	// Load messaging channel plugins
	pluginLoader := loader.NewLoader(workspace.Config.Plugins.Dir)
	channels, err := loadMessagingChannels(ctx, pluginLoader)
	if err != nil {
		return fmt.Errorf("failed to load messaging channels: %w", err)
	}

	if len(channels) == 0 {
		return fmt.Errorf("no messaging channel plugins found. Install a plugin (e.g., telegram)")
	}

	// Register all channels
	for _, channel := range channels {
		if err := msgBroker.RegisterChannel(channel); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to register channel %s: %v\n", channel.GetName(), err)
			continue
		}
		fmt.Printf("✓ Registered %s channel\n", channel.GetName())
	}

	fmt.Printf("✓ Listening for messages on %d channel(s)...\n\n", len(channels))

	// Start listening for messages
	err = msgBroker.StartListening(ctx, func(msg messaging.IncomingMessage) error {
		fmt.Printf("📨 [%s] Message from @%s: %s\n", msg.Channel, msg.Sender, msg.Text)

		// Process message with AI
		result, err := orch.Run(ctx, msg.Text)
		if err != nil {
			fmt.Printf("❌ Error processing message: %v\n", err)
			return err
		}

		// Send response back
		response := result.Response
		if response == "" {
			response = "I processed your message but have no response."
		}

		fmt.Printf("🤖 Responding: %s\n\n", response)

		return msgBroker.SendMessage(ctx, messaging.OutgoingMessage{
			Channel:   msg.Channel,
			Recipient: msg.ChatID,
			Text:      response,
		})
	})

	if err != nil {
		return fmt.Errorf("failed to start listening: %w", err)
	}

	return nil
}

func runMessagingSend(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	channelName := args[0]
	recipient := args[1]
	message := args[2]

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Initialize policy engine
	policyData, err := policy.LoadPolicy(workspace.Config.Policy.FilePath)
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}
	policyEngine := policy.NewEngine(policyData)

	// Initialize audit logger
	auditLogger, err := audit.NewJSONLLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
	}
	defer auditLogger.Close()

	// Create message broker
	msgBroker := messaging.NewMessageBroker(policyEngine, auditLogger)

	// Load messaging channel plugins
	pluginLoader := loader.NewLoader(workspace.Config.Plugins.Dir)
	channels, err := loadMessagingChannels(ctx, pluginLoader)
	if err != nil {
		return fmt.Errorf("failed to load messaging channels: %w", err)
	}

	// Find and register the requested channel
	var targetChannel messaging.Channel
	for _, channel := range channels {
		if channel.GetName() == channelName {
			targetChannel = channel
			break
		}
	}

	if targetChannel == nil {
		return fmt.Errorf("channel %s not found. Available channels: use 'soulgate messaging list'", channelName)
	}

	if err := msgBroker.RegisterChannel(targetChannel); err != nil {
		return fmt.Errorf("failed to register channel: %w", err)
	}

	// Send message
	err = msgBroker.SendMessage(ctx, messaging.OutgoingMessage{
		Channel:   channelName,
		Recipient: recipient,
		Text:      message,
	})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	fmt.Printf("✓ Message sent to %s on %s\n", recipient, channelName)
	return nil
}

func runMessagingList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Load messaging channel plugins
	pluginLoader := loader.NewLoader(workspace.Config.Plugins.Dir)
	channels, err := loadMessagingChannels(ctx, pluginLoader)
	if err != nil {
		return fmt.Errorf("failed to load messaging channels: %w", err)
	}

	if len(channels) == 0 {
		fmt.Println("No messaging channel plugins installed.")
		fmt.Println("\nTo install a channel plugin, place it in:", workspace.Config.Plugins.Dir)
		return nil
	}

	fmt.Printf("Available messaging channels (%d):\n\n", len(channels))
	for _, channel := range channels {
		fmt.Printf("  • %s\n", channel.GetName())
	}

	return nil
}

// loadMessagingChannels loads all messaging channel plugins
func loadMessagingChannels(ctx context.Context, pluginLoader *loader.Loader) ([]messaging.Channel, error) {
	channels := make([]messaging.Channel, 0)

	// Get all registered channel plugins from the registry
	pluginNames := loader.ListChannelPlugins()

	for _, name := range pluginNames {
		// Create channel instance
		// TODO: Load config from plugin manifest or workspace config
		config := make(map[string]interface{})

		channel, err := loader.CreateChannel(ctx, name, config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create %s channel: %v\n", name, err)
			continue
		}

		channels = append(channels, channel)
	}

	return channels, nil
}
