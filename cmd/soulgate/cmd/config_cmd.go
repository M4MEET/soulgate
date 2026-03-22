package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage configuration",
	Long: `View and manage SoulGate configuration.

Examples:
  soulgate config show          # Show current config
  soulgate config path          # Show config file path
  soulgate config validate      # Validate config file`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, _ := os.UserHomeDir()
		paths := []string{
			filepath.Join(".soulgate", "config.yml"),
			filepath.Join(homeDir, ".soulgate", "config.yml"),
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				fmt.Println(p)
				return
			}
		}
		fmt.Println("No config file found")
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config file",
	RunE:  runConfigValidate,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configValidateCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, path, err := loadAnyConfig()
	if err != nil {
		return err
	}

	fmt.Printf("  Config: %s\n\n", path)
	fmt.Printf("  %-20s %s\n", "Provider:", cfg.Model.DefaultProvider)

	switch cfg.Model.DefaultProvider {
	case "anthropic":
		fmt.Printf("  %-20s %s\n", "Model:", cfg.Model.Anthropic.Model)
		if cfg.Model.Anthropic.APIKey != "" {
			fmt.Printf("  %-20s %s...%s\n", "API Key:", cfg.Model.Anthropic.APIKey[:8], cfg.Model.Anthropic.APIKey[len(cfg.Model.Anthropic.APIKey)-4:])
		}
	default:
		fmt.Printf("  %-20s %s\n", "Model:", cfg.Model.OpenAI.Model)
		if cfg.Model.OpenAI.BaseURL != "" {
			fmt.Printf("  %-20s %s\n", "Base URL:", cfg.Model.OpenAI.BaseURL)
		}
	}

	fmt.Printf("  %-20s %d\n", "Max Tokens:", cfg.Model.OpenAI.MaxTokens)
	fmt.Printf("  %-20s %.1f\n", "Temperature:", cfg.Model.OpenAI.Temperature)
	fmt.Printf("  %-20s %s\n", "Audit:", cfg.Audit.DatabasePath)
	fmt.Printf("  %-20s %s\n", "Policy:", cfg.Policy.FilePath)

	if cfg.Execution.MaxIterations > 0 {
		fmt.Printf("  %-20s %d\n", "Max Iterations:", cfg.Execution.MaxIterations)
	}

	return nil
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	_, path, err := loadAnyConfig()
	if err != nil {
		fmt.Printf("  ✗ %s: %v\n", path, err)
		return err
	}
	fmt.Printf("  ✓ %s is valid\n", path)
	return nil
}

func loadAnyConfig() (*config.Config, string, error) {
	homeDir, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(".soulgate", "config.yml"),
		filepath.Join(homeDir, ".soulgate", "config.yml"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			cfg, err := config.LoadConfig(p)
			if err != nil {
				return nil, p, err
			}
			return cfg, p, nil
		}
	}
	return nil, strings.Join(paths, ", "), fmt.Errorf("no config file found")
}
