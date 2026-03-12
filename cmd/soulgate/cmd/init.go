package cmd

import (
	"fmt"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a new SoulGate workspace with defaults",
	Long: `Initialize a new SoulGate workspace with default configuration and policies.

This command creates a basic workspace with default settings.
For an interactive setup with custom configuration, use: soulgate setup

This command creates:
  - .soulgate/ directory with configuration and audit database
  - plugins/ directory for plugin installation
  - Default policy file with workspace access rules

Tip: Use 'soulgate setup' for a guided configuration experience.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine workspace path
	workspacePath := "."
	if len(args) > 0 {
		workspacePath = args[0]
	}

	// Check if already initialized
	if config.IsInitialized(workspacePath) {
		return fmt.Errorf("workspace already initialized at %s", workspacePath)
	}

	// Initialize workspace
	workspace, err := config.InitWorkspace(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	fmt.Printf("✓ Initialized SoulGate workspace at %s\n", workspace.Root)
	fmt.Println()
	fmt.Println("Created:")
	fmt.Printf("  - %s/ (configuration directory)\n", workspace.ConfigDir)
	fmt.Printf("  - %s/config.yml (workspace configuration)\n", workspace.ConfigDir)
	fmt.Printf("  - %s/policy.yml (default policy)\n", workspace.ConfigDir)
	fmt.Printf("  - %s/audit.db (audit log database)\n", workspace.ConfigDir)
	fmt.Printf("  - %s/ (plugins directory)\n", workspace.Config.Plugins.Dir)
	fmt.Println()
	fmt.Println("⚠️  Using default configuration. For custom setup, run:")
	fmt.Println("     soulgate setup")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Set your API key: export OPENAI_API_KEY=sk-...")
	fmt.Println("  2. Review policy: cat .soulgate/policy.yml")
	fmt.Println("  3. Run: soulgate run \"<your prompt>\"")
	fmt.Println()
	fmt.Println("Or reconfigure everything with: soulgate setup")

	return nil
}
