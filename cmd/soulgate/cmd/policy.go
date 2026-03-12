package cmd

import (
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Policy management",
	Long:  `View and manage security policies that control agent access to resources.`,
}

var policyShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show active policy",
	Long:  `Display the currently active security policy.`,
	RunE:  runPolicyShow,
}

func init() {
	rootCmd.AddCommand(policyCmd)
	policyCmd.AddCommand(policyShowCmd)
}

func runPolicyShow(cmd *cobra.Command, args []string) error {
	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Load policy
	pol, err := policy.LoadPolicy(workspace.Config.Policy.FilePath)
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}

	// Display policy
	fmt.Printf("Policy file: %s\n", workspace.Config.Policy.FilePath)
	fmt.Printf("Version: %s\n", pol.Version)
	fmt.Printf("Rules: %d\n\n", len(pol.Policies))

	// Serialize to YAML for display
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	if err := encoder.Encode(pol); err != nil {
		return fmt.Errorf("failed to encode policy: %w", err)
	}

	return nil
}
