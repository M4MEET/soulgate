package cmd

import (
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/cmd/soulgate/cmd/setup"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "soulgate",
	Short: "SoulGate - Powerful AI Terminal with Security",
	Long: `SoulGate - Your powerful AI assistant in the terminal.

A single, capable AI agent that can read, write, and manage your files
with full security controls and audit logging.

Features:
  - Interactive AI chat with TUI
  - File operations (read, write, list)
  - Skills system for customizable behavior
  - Per-agent persistent memory
  - Policy-based security with roles
  - Complete audit logging
  - Parallel tool execution
  - Multi-provider support (OpenAI, Anthropic, Groq, Ollama, etc.)`,
	Version: "0.2.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand provided, launch the TUI
		return runChat(cmd, args)
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here

	// Register setup command from setup package
	rootCmd.AddCommand(setup.SetupCmd)
}
