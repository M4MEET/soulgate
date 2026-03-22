package cmd

import (
	"context"
	"fmt"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run \"<prompt>\"",
	Short: "Execute a prompt with agent capabilities",
	Long: `Execute a prompt with the LLM agent, giving it access to tools through plugins.

The agent will:
  1. Receive your prompt
  2. Call available plugin tools as needed
  3. Access resources through policy-enforced brokers
  4. Return the final response

All operations are logged to the audit database.`,
	Args: cobra.ExactArgs(1),
	RunE: runRun,
}

var (
	runProvider string
)

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runProvider, "provider", "", "Model provider (openai, anthropic)")
}

func runRun(cmd *cobra.Command, args []string) error {
	prompt := args[0]

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w\n\nRun 'soulgate onboarding' for guided configuration, or 'soulgate init' for defaults", err)
	}

	// Override provider if specified
	if runProvider != "" {
		workspace.Config.Model.DefaultProvider = runProvider
	}

	// Validate API key is configured
	if workspace.Config.Model.DefaultProvider == "openai" && workspace.Config.Model.OpenAI.APIKey == "" {
		return fmt.Errorf("OpenAI API key not configured.\n\nSet it with: export OPENAI_API_KEY=sk-...\nOr run 'soulgate onboarding' to configure everything")
	}
	if workspace.Config.Model.DefaultProvider == "anthropic" && workspace.Config.Model.Anthropic.APIKey == "" {
		return fmt.Errorf("Anthropic API key not configured.\n\nSet it with: export ANTHROPIC_API_KEY=sk-ant-...\nOr run 'soulgate onboarding' to configure everything")
	}

	// Create orchestrator
	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	fmt.Printf("Running prompt with %s...\n\n", workspace.Config.Model.DefaultProvider)

	// Execute run
	ctx := context.Background()
	result, err := orch.Run(ctx, prompt)
	if err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	// Display result
	fmt.Println(result.Response)
	fmt.Println()
	fmt.Printf("✓ Run completed: %s\n", result.RunID)
	fmt.Println()
	fmt.Println("View audit log: soulgate audit tail")

	return nil
}
