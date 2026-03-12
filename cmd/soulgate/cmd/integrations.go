package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/integrations/analytics"
	"github.com/M4MEET/soulgate/internal/integrations/aws"
	"github.com/M4MEET/soulgate/internal/integrations/database"
	"github.com/M4MEET/soulgate/internal/integrations/docker"
	"github.com/M4MEET/soulgate/internal/integrations/github"
	"github.com/M4MEET/soulgate/internal/integrations/google"
	"github.com/M4MEET/soulgate/internal/integrations/notion"
	"github.com/M4MEET/soulgate/internal/integrations/slack"
	"github.com/spf13/cobra"
)

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage external integrations",
	Long:  `Connect SoulGate to external services like GitHub, Slack, databases, and more.`,
}

var integrationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available integrations",
	Long:  `Display all available integrations and their configuration status.`,
	RunE:  runIntegrationsList,
}

var integrationsSetupCmd = &cobra.Command{
	Use:   "setup [integration]",
	Short: "Setup an integration",
	Long:  `Configure an integration by providing required credentials.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runIntegrationsSetup,
}

var integrationsRemoveCmd = &cobra.Command{
	Use:   "remove [integration]",
	Short: "Remove an integration",
	Long:  `Remove an integration and delete its stored credentials.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runIntegrationsRemove,
}

func init() {
	rootCmd.AddCommand(integrationsCmd)
	integrationsCmd.AddCommand(integrationsListCmd)
	integrationsCmd.AddCommand(integrationsSetupCmd)
	integrationsCmd.AddCommand(integrationsRemoveCmd)
}

func runIntegrationsList(cmd *cobra.Command, args []string) error {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".soulgate")

	// Initialize registry
	registry := integrations.NewRegistry()
	registry.Register(github.New())
	registry.Register(slack.New())
	registry.Register(database.NewPostgres())
	registry.Register(google.NewDrive())
	registry.Register(google.NewGmail())
	registry.Register(docker.New())
	registry.Register(aws.NewS3())
	registry.Register(notion.New())
	registry.Register(analytics.New())

	// Load stored configurations
	store, err := integrations.NewStore(configDir)
	if err != nil {
		return fmt.Errorf("failed to load integrations: %w", err)
	}

	// Load configs
	for _, name := range store.List() {
		config, _ := store.Get(name)
		integration, err := registry.Get(name)
		if err == nil {
			integration.Setup(context.Background(), config)
		}
	}

	// Display integrations
	info := registry.ListInfo()

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                 SoulGate Integrations                         ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for _, i := range info {
		status := "❌ Not Configured"
		if i.Configured {
			status = fmt.Sprintf("✅ Configured (%d tools)", i.ToolsCount)
		}

		fmt.Printf("📦 %s\n", i.Name)
		fmt.Printf("   Status:       %s\n", status)
		fmt.Printf("   Description:  %s\n", i.Description)

		if !i.Configured && len(i.RequiredConfig) > 0 {
			fmt.Printf("   Required:     ")
			for idx, field := range i.RequiredConfig {
				if idx > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", field.Name)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("Commands:")
	fmt.Println("  soulgate integrations setup <name>   - Configure an integration")
	fmt.Println("  soulgate integrations remove <name>  - Remove an integration")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  soulgate integrations setup github")
	fmt.Println()

	return nil
}

func runIntegrationsSetup(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".soulgate")

	// Initialize registry
	registry := integrations.NewRegistry()
	registry.Register(github.New())
	registry.Register(slack.New())
	registry.Register(database.NewPostgres())
	registry.Register(google.NewDrive())
	registry.Register(google.NewGmail())
	registry.Register(docker.New())
	registry.Register(aws.NewS3())
	registry.Register(notion.New())
	registry.Register(analytics.New())

	// Get integration
	integration, err := registry.Get(name)
	if err != nil {
		return fmt.Errorf("integration '%s' not found. Run 'soulgate integrations list' to see available integrations", name)
	}

	// Get required config
	requiredConfig := integration.RequiredConfig()

	fmt.Printf("╔═══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ Setup: %-54s ║\n", name)
	fmt.Printf("╚═══════════════════════════════════════════════════════════════╝\n")
	fmt.Println()
	fmt.Printf("%s\n\n", integration.Description())

	config := make(map[string]string)

	// Prompt for each field
	for _, field := range requiredConfig {
		if field.Default != "" {
			fmt.Printf("%s [%s]: ", field.Description, field.Default)
		} else if field.Example != "" {
			fmt.Printf("%s (e.g., %s): ", field.Description, field.Example)
		} else {
			fmt.Printf("%s: ", field.Description)
		}

		var value string
		fmt.Scanln(&value)

		if value == "" && field.Default != "" {
			value = field.Default
		}

		if value == "" && field.Required {
			return fmt.Errorf("%s is required", field.Name)
		}

		config[field.Name] = value
	}

	// Setup integration
	if err := integration.Setup(context.Background(), config); err != nil {
		return fmt.Errorf("failed to setup integration: %w", err)
	}

	// Save config
	store, err := integrations.NewStore(configDir)
	if err != nil {
		return fmt.Errorf("failed to save integration: %w", err)
	}

	if err := store.Save(name, config); err != nil {
		return fmt.Errorf("failed to save integration: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Integration '%s' configured successfully!\n", name)
	fmt.Printf("   %d tools are now available in chat.\n", len(integration.GetTools()))
	fmt.Println()

	return nil
}

func runIntegrationsRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".soulgate")

	// Load store
	store, err := integrations.NewStore(configDir)
	if err != nil {
		return fmt.Errorf("failed to load integrations: %w", err)
	}

	// Check if exists
	if _, exists := store.Get(name); !exists {
		return fmt.Errorf("integration '%s' is not configured", name)
	}

	// Remove
	if err := store.Delete(name); err != nil {
		return fmt.Errorf("failed to remove integration: %w", err)
	}

	fmt.Printf("✅ Integration '%s' removed successfully!\n", name)
	return nil
}
