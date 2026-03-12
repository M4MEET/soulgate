package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/M4MEET/soulgate/internal/agents"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent runtime commands",
	Long: `Run the AI agent runtime that connects to the Gateway.

The agent:
- Connects to Gateway via WebSocket
- Receives messages from channels
- Processes with AI model
- Executes tools
- Sends responses back

The agent is the "brain" of SoulGate.`,
}

var agentStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the agent runtime",
	Long: `Start the agent runtime and connect to the Gateway.

The agent will:
1. Connect to the Gateway as an 'agent' role client
2. Wait for incoming messages
3. Process messages with the configured AI model
4. Execute tools as needed
5. Send responses back through the Gateway

Example:
  soulgate agent start
  soulgate agent start --gateway ws://localhost:8080/ws
  soulgate agent start --client-id my-agent
  soulgate agent start --skills code_review,debugging`,
	RunE: runAgentStart,
}

var (
	agentGatewayURL string
	agentClientID   string
	agentSkills     []string
)

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentStartCmd)

	agentStartCmd.Flags().StringVar(&agentGatewayURL, "gateway", "ws://localhost:8080/ws", "Gateway WebSocket URL")
	agentStartCmd.Flags().StringVar(&agentClientID, "client-id", "", "Custom client ID (auto-generated if not provided)")
	agentStartCmd.Flags().StringSliceVar(&agentSkills, "skills", []string{}, "Skills to load (comma-separated)")
}

func runAgentStart(cmd *cobra.Command, args []string) error {
	fmt.Println("🤖 Starting SoulGate Agent Runtime...")
	fmt.Println("────────────────────────────────────")

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	fmt.Printf("✓ Workspace: %s\n", workspace.Root)

	// Check API keys
	provider := workspace.Config.Model.DefaultProvider
	switch provider {
	case "openai":
		if workspace.Config.Model.OpenAI.APIKey == "" && os.Getenv("OPENAI_API_KEY") == "" {
			return fmt.Errorf("OpenAI API key not configured. Set OPENAI_API_KEY environment variable")
		}
	case "anthropic":
		if workspace.Config.Model.Anthropic.APIKey == "" && os.Getenv("ANTHROPIC_API_KEY") == "" {
			return fmt.Errorf("Anthropic API key not configured. Set ANTHROPIC_API_KEY environment variable")
		}
	}

	// Create agent runtime
	config := &agents.Config{
		GatewayURL: agentGatewayURL,
		ClientID:   agentClientID,
		Workspace:  workspace,
		Skills:     agentSkills,
	}

	runtime, err := agents.NewRuntime(config)
	if err != nil {
		return fmt.Errorf("failed to create agent runtime: %w", err)
	}
	defer runtime.Close()

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Connect to Gateway
	if err := runtime.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Gateway: %w", err)
	}

	// Start agent runtime
	if err := runtime.Start(ctx); err != nil {
		if err == context.Canceled {
			fmt.Println("\n👋 Agent stopped")
			return nil
		}
		return fmt.Errorf("agent error: %w", err)
	}

	return nil
}
