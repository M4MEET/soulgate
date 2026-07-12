package cmd

import (
	"errors"
	"fmt"
	"os"

	agentsconfig "github.com/M4MEET/soulgate/internal/agents/config"
	"github.com/spf13/cobra"
)

var agentsManageCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agent configurations",
	Long: `Manage multiple agent configurations.

Agents are defined in .soulgate/agents.yaml and can have:
- Different models (GPT-4, Claude, etc.)
- Different tools and skills
- Custom system prompts
- Routing rules

Example:
  soulgate agents list              # List all agents
  soulgate agents show gpt4-general # Show agent config
  soulgate agents validate          # Validate configuration`,
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured agents",
	RunE:  runAgentsList,
}

var agentsShowCmd = &cobra.Command{
	Use:   "show <agent-id>",
	Short: "Show agent configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentsShow,
}

var agentsValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate agents configuration",
	RunE:  runAgentsValidate,
}

var (
	agentsConfigPath string
)

func init() {
	rootCmd.AddCommand(agentsManageCmd)
	agentsManageCmd.AddCommand(agentsListCmd)
	agentsManageCmd.AddCommand(agentsShowCmd)
	agentsManageCmd.AddCommand(agentsValidateCmd)

	agentsManageCmd.PersistentFlags().StringVar(&agentsConfigPath, "config", ".soulgate/agents.yaml", "Path to agents configuration")
}

func runAgentsList(cmd *cobra.Command, args []string) error {
	config, err := agentsconfig.LoadAgentsConfig(agentsConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No agents configured yet.")
			fmt.Printf("\nTo define agents, create %s (see 'soulgate agents --help' for the format).\n", agentsConfigPath)
			return nil
		}
		return fmt.Errorf("failed to load agents config: %w", err)
	}

	if len(config.Agents) == 0 {
		fmt.Println("No agents configured")
		return nil
	}

	fmt.Printf("Configured agents (%d):\n\n", len(config.Agents))

	for _, agent := range config.Agents {
		status := "✗ disabled"
		if agent.Enabled {
			status = "✓ enabled"
		}

		fmt.Printf("  • %s [%s]\n", agent.ID, status)
		fmt.Printf("    Name: %s\n", agent.Name)
		if agent.Description != "" {
			fmt.Printf("    Description: %s\n", agent.Description)
		}
		fmt.Printf("    Model: %s (%s)\n", agent.Model.Model, agent.Model.Provider)
		if len(agent.Tools) > 0 {
			fmt.Printf("    Tools: %v\n", agent.Tools)
		}
		if len(agent.Skills) > 0 {
			fmt.Printf("    Skills: %v\n", agent.Skills)
		}
		fmt.Println()
	}

	// Show routing strategy
	fmt.Printf("Routing strategy: %s\n", config.Routing.Strategy)
	if len(config.Routing.Rules) > 0 {
		fmt.Printf("Routing rules: %d\n", len(config.Routing.Rules))
	}

	return nil
}

func runAgentsShow(cmd *cobra.Command, args []string) error {
	agentID := args[0]

	config, err := agentsconfig.LoadAgentsConfig(agentsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load agents config: %w", err)
	}

	agent, err := config.GetAgentByID(agentID)
	if err != nil {
		return err
	}

	fmt.Printf("Agent: %s\n", agent.ID)
	fmt.Printf("Name: %s\n", agent.Name)
	if agent.Description != "" {
		fmt.Printf("Description: %s\n", agent.Description)
	}
	fmt.Printf("Enabled: %t\n", agent.Enabled)
	fmt.Println()

	fmt.Println("Model Configuration:")
	fmt.Printf("  Provider: %s\n", agent.Model.Provider)
	fmt.Printf("  Model: %s\n", agent.Model.Model)
	fmt.Printf("  Temperature: %.1f\n", agent.Model.Temperature)
	fmt.Printf("  Max Tokens: %d\n", agent.Model.MaxTokens)
	fmt.Println()

	if len(agent.Tools) > 0 {
		fmt.Printf("Tools (%d):\n", len(agent.Tools))
		for _, tool := range agent.Tools {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Println()
	}

	if len(agent.Skills) > 0 {
		fmt.Printf("Skills (%d):\n", len(agent.Skills))
		for _, skill := range agent.Skills {
			fmt.Printf("  - %s\n", skill)
		}
		fmt.Println()
	}

	fmt.Printf("Max Iterations: %d\n", agent.MaxIterations)

	if agent.SystemPrompt != "" {
		fmt.Println("\nSystem Prompt:")
		fmt.Printf("  %s\n", agent.SystemPrompt)
	}

	if len(agent.Metadata) > 0 {
		fmt.Println("\nMetadata:")
		for key, value := range agent.Metadata {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	return nil
}

func runAgentsValidate(cmd *cobra.Command, args []string) error {
	config, err := agentsconfig.LoadAgentsConfig(agentsConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load agents config: %w", err)
	}

	if err := config.Validate(); err != nil {
		fmt.Printf("❌ Configuration is invalid: %v\n", err)
		return err
	}

	fmt.Println("✓ Configuration is valid")
	fmt.Printf("  Agents: %d\n", len(config.Agents))
	fmt.Printf("  Enabled: %d\n", len(config.GetEnabledAgents()))
	fmt.Printf("  Routing rules: %d\n", len(config.Routing.Rules))

	return nil
}
