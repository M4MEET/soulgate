package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	resetScope string
	resetYes   bool
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset SoulGate state",
	Long: `Reset SoulGate configuration, sessions, or everything.

Scopes:
  sessions    Clear conversation history and session state
  config      Reset configuration to defaults (preserves API keys)
  all         Full reset: config + sessions + audit logs

Examples:
  soulgate reset --scope sessions          # Clear chat history
  soulgate reset --scope sessions --yes    # Skip confirmation
  soulgate reset --scope all --yes         # Full reset`,
	Args: cobra.NoArgs,
	RunE: runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().StringVar(&resetScope, "scope", "sessions", "What to reset: sessions, config, all")
	resetCmd.Flags().BoolVarP(&resetYes, "yes", "y", false, "Skip confirmation prompt")
}

func runReset(cmd *cobra.Command, args []string) error {
	homeDir, _ := os.UserHomeDir()
	globalConfigDir := filepath.Join(homeDir, ".soulgate")
	localConfigDir := ".soulgate"

	// Determine which config dir to use
	configDir := localConfigDir
	if _, err := os.Stat(filepath.Join(localConfigDir, "config.yml")); os.IsNotExist(err) {
		configDir = globalConfigDir
	}

	switch resetScope {
	case "sessions":
		return resetSessions(configDir)
	case "config":
		return resetConfig(configDir)
	case "all":
		if err := resetSessions(configDir); err != nil {
			return err
		}
		if err := resetConfig(configDir); err != nil {
			return err
		}
		return resetAudit(configDir)
	default:
		return fmt.Errorf("unknown scope: %s (use: sessions, config, all)", resetScope)
	}
}

func resetSessions(configDir string) error {
	targets := []string{
		filepath.Join(configDir, "session_state.json"),
	}

	// Also clear sessions directory
	sessionsDir := "sessions"

	if !resetYes {
		fmt.Println("  Will delete:")
		for _, t := range targets {
			if _, err := os.Stat(t); err == nil {
				fmt.Printf("    - %s\n", t)
			}
		}
		if _, err := os.Stat(sessionsDir); err == nil {
			fmt.Printf("    - %s/ (gateway sessions)\n", sessionsDir)
		}
		fmt.Print("\n  Continue? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("  Cancelled.")
			return nil
		}
	}

	count := 0
	for _, t := range targets {
		if err := os.Remove(t); err == nil {
			count++
		}
	}

	// Clear sessions directory contents
	if entries, err := os.ReadDir(sessionsDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".jsonl") {
				os.Remove(filepath.Join(sessionsDir, e.Name()))
				count++
			}
		}
	}

	fmt.Printf("  ✓ Sessions cleared (%d files removed)\n", count)
	return nil
}

func resetConfig(configDir string) error {
	targets := []string{
		filepath.Join(configDir, "policy.yml"),
	}

	if !resetYes {
		fmt.Println("  Will reset:")
		for _, t := range targets {
			if _, err := os.Stat(t); err == nil {
				fmt.Printf("    - %s\n", t)
			}
		}
		fmt.Print("\n  Continue? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(strings.TrimSpace(input)) != "y" {
			fmt.Println("  Cancelled.")
			return nil
		}
	}

	count := 0
	for _, t := range targets {
		if err := os.Remove(t); err == nil {
			count++
		}
	}

	fmt.Printf("  ✓ Config reset (%d files removed, API keys preserved)\n", count)
	return nil
}

func resetAudit(configDir string) error {
	// Remove audit log files
	count := 0
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "audit") && strings.HasSuffix(e.Name(), ".jsonl") {
			if err := os.Remove(filepath.Join(configDir, e.Name())); err == nil {
				count++
			}
		}
	}

	// Also check for memory.json
	memPath := filepath.Join(configDir, "memory.json")
	if err := os.Remove(memPath); err == nil {
		count++
	}

	fmt.Printf("  ✓ Audit logs and memory cleared (%d files removed)\n", count)
	return nil
}
