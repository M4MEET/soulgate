package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show SoulGate workspace status",
	Long:  `Display the current workspace configuration, agent status, and system health.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	workspacePath := "."

	// Check if initialized
	if !config.IsInitialized(workspacePath) {
		fmt.Println("❌ SoulGate is not initialized in this directory")
		fmt.Println()
		fmt.Println("To get started, run:")
		fmt.Println("  soulgate setup    (interactive wizard)")
		fmt.Println("  soulgate init     (quick initialization)")
		return nil
	}

	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║            SoulGate Workspace Status                 ║")
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load workspace
	workspace, err := config.LoadWorkspaceFromPath(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	cfg := workspace.Config

	// Workspace Info
	fmt.Println("📁 WORKSPACE")
	fmt.Println("────────────────────────────────────────────────────────")
	absPath, _ := filepath.Abs(workspacePath)
	fmt.Printf("  Path:       %s\n", absPath)
	fmt.Printf("  Config Dir: %s\n", cfg.Workspace.ConfigDir)
	fmt.Printf("  Status:     ✓ Initialized\n")
	fmt.Println()

	// Model Configuration
	fmt.Println("🤖 MODEL PROVIDER")
	fmt.Println("────────────────────────────────────────────────────────")
	provider := cfg.Model.DefaultProvider
	fmt.Printf("  Provider:   %s\n", provider)

	var model string
	switch provider {
	case "openai":
		model = cfg.Model.OpenAI.Model
	case "anthropic":
		model = cfg.Model.Anthropic.Model
	default:
		model = "unknown"
	}
	fmt.Printf("  Model:      %s\n", model)

	// Check API keys
	apiKeySet := false
	switch provider {
	case "openai":
		apiKeySet = os.Getenv("OPENAI_API_KEY") != "" || cfg.Model.OpenAI.APIKey != ""
	case "anthropic":
		apiKeySet = os.Getenv("ANTHROPIC_API_KEY") != "" || cfg.Model.Anthropic.APIKey != ""
	case "ollama":
		apiKeySet = true // Ollama doesn't need API key
	}

	if apiKeySet {
		fmt.Printf("  API Key:    ✓ Set\n")
	} else {
		fmt.Printf("  API Key:    ⚠️  Not set\n")
	}
	fmt.Println()

	// Security Policy
	fmt.Println("🔒 SECURITY POLICY")
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Printf("  Policy File: %s\n", cfg.Policy.FilePath)

	// Check if policy file exists
	if _, err := os.Stat(cfg.Policy.FilePath); err == nil {
		fmt.Printf("  Status:      ✓ Configured\n")
	} else {
		fmt.Printf("  Status:      ⚠️  Not found\n")
	}
	fmt.Println()

	// Consolidated Agents
	fmt.Println("⚙️  CONSOLIDATED AGENTS")
	fmt.Println("────────────────────────────────────────────────────────")

	agentsFile := filepath.Join(cfg.Workspace.ConfigDir, "agents.yaml")
	if _, err := os.Stat(agentsFile); err == nil {
		fmt.Printf("  Config:      %s\n", agentsFile)
		fmt.Println("  Agents:")
		fmt.Println("    ✓ Test & Quality Agent")
		fmt.Println("    ✓ Docs & API Agent")
		fmt.Println("    ✓ Project Management Agent")
		fmt.Println("  Service:")
		fmt.Println("    ✓ Notification Service")
	} else {
		fmt.Println("  Status:      ⚠️  Not configured")
		fmt.Println("  Run 'soulgate setup' to configure agents")
	}
	fmt.Println()

	// Audit
	fmt.Println("📊 AUDIT LOGGING")
	fmt.Println("────────────────────────────────────────────────────────")
	if cfg.Audit.Enabled {
		auditDB := cfg.Audit.DatabasePath
		if _, err := os.Stat(auditDB); err == nil {
			stat, _ := os.Stat(auditDB)
			fmt.Printf("  Status:      ✓ Enabled\n")
			fmt.Printf("  Database:    %s\n", auditDB)
			fmt.Printf("  Size:        %d KB\n", stat.Size()/1024)
		} else {
			fmt.Printf("  Status:      ⚠️  Enabled but database not found\n")
		}
	} else {
		fmt.Printf("  Status:      Disabled\n")
	}
	fmt.Println()

	// Plugins
	fmt.Println("🔌 PLUGINS")
	fmt.Println("────────────────────────────────────────────────────────")
	pluginDir := cfg.Plugins.Dir
	if _, err := os.Stat(pluginDir); err == nil {
		files, _ := os.ReadDir(pluginDir)
		pluginCount := 0
		for _, f := range files {
			if filepath.Ext(f.Name()) == ".wasm" {
				pluginCount++
			}
		}
		fmt.Printf("  Directory:   %s\n", pluginDir)
		fmt.Printf("  Installed:   %d plugins\n", pluginCount)
	} else {
		fmt.Printf("  Directory:   %s (not created)\n", pluginDir)
		fmt.Printf("  Installed:   0 plugins\n")
	}
	fmt.Println()

	// Quick Actions
	fmt.Println("🚀 QUICK ACTIONS")
	fmt.Println("────────────────────────────────────────────────────────")
	fmt.Println("  soulgate run \"<prompt>\"      - Run a prompt")
	fmt.Println("  soulgate audit query          - Query audit logs")
	fmt.Println("  soulgate policy check <path>  - Check policy")
	fmt.Println("  soulgate plugin list          - List plugins")
	fmt.Println()

	return nil
}
