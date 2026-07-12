package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var soulCmd = &cobra.Command{
	Use:   "soul",
	Short: "Manage the AI's persona and behavior (SOUL.md)",
	Long: `Manage the SOUL.md file that defines how the AI behaves.

The SOUL.md file customizes the AI's personality, communication style,
behavior rules, and boundaries. It's loaded into the system prompt.

Examples:
  soulgate soul init          # Create default SOUL.md
  soulgate soul show          # Display current soul configuration
  soulgate soul edit          # Open SOUL.md in your editor
  soulgate soul reset         # Reset to default template`,
}

var soulInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default SOUL.md file",
	Args:  cobra.NoArgs,
	RunE:  runSoulInit,
}

var soulShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the current SOUL.md configuration",
	RunE:  runSoulShow,
}

var soulEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open SOUL.md in your default editor",
	Args:  cobra.NoArgs,
	RunE:  runSoulEdit,
}

var soulResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset SOUL.md to default template",
	Args:  cobra.NoArgs,
	RunE:  runSoulReset,
}

func init() {
	rootCmd.AddCommand(soulCmd)
	soulCmd.AddCommand(soulInitCmd)
	soulCmd.AddCommand(soulShowCmd)
	soulCmd.AddCommand(soulEditCmd)
	soulCmd.AddCommand(soulResetCmd)
}

func getSoulConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".soulgate"
	}
	return filepath.Join(homeDir, ".soulgate")
}

func runSoulInit(cmd *cobra.Command, args []string) error {
	configDir := getSoulConfigDir()

	if err := core.CreateSoulFile(configDir); err != nil {
		return err
	}

	soulPath := filepath.Join(configDir, "SOUL.md")
	fmt.Printf("Created SOUL.md at %s\n", soulPath)
	fmt.Println()
	fmt.Println("This file defines your AI's persona and behavior.")
	fmt.Println("Edit it to customize how the AI communicates and acts.")
	fmt.Println()
	fmt.Println("Sections you can customize:")
	fmt.Println("  ## Identity          - Who the AI is")
	fmt.Println("  ## Personality       - Character traits")
	fmt.Println("  ## Communication Style - How it responds")
	fmt.Println("  ## Behavior Rules    - What it should/shouldn't do")
	fmt.Println("  ## Context Awareness - Memory and context behavior")
	fmt.Println("  ## Boundaries        - Security and safety limits")
	fmt.Println()
	fmt.Println("Edit with: soulgate soul edit")
	return nil
}

func runSoulShow(cmd *cobra.Command, args []string) error {
	configDir := getSoulConfigDir()

	soul, err := core.LoadSoulConfig(configDir)
	if err != nil {
		return err
	}

	if soul == nil {
		fmt.Println("No SOUL.md found.")
		fmt.Println()
		fmt.Println("Create one with: soulgate soul init")
		return nil
	}

	fmt.Printf("SOUL.md (%s)\n", soul.Path)
	fmt.Println("========================================================")
	fmt.Println()
	fmt.Println(soul.Content)
	return nil
}

func runSoulEdit(cmd *cobra.Command, args []string) error {
	configDir := getSoulConfigDir()
	soulPath := filepath.Join(configDir, "SOUL.md")

	// Create if doesn't exist
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		if err := core.CreateSoulFile(configDir); err != nil {
			return err
		}
		fmt.Println("Created default SOUL.md")
	}

	// Try to open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		fmt.Printf("SOUL.md is at: %s\n", soulPath)
		fmt.Println()
		fmt.Println("Open it in your preferred editor:")
		fmt.Printf("  vim %s\n", soulPath)
		fmt.Printf("  code %s\n", soulPath)
		fmt.Printf("  nano %s\n", soulPath)
		return nil
	}

	fmt.Printf("Opening %s in %s...\n", soulPath, editor)
	// Note: In a real implementation, we'd exec the editor
	// For now, just show the path
	fmt.Printf("Edit: %s %s\n", editor, soulPath)
	return nil
}

func runSoulReset(cmd *cobra.Command, args []string) error {
	configDir := getSoulConfigDir()

	if err := core.UpdateSoulFile(configDir, core.DefaultSoulTemplate()); err != nil {
		return err
	}

	fmt.Println("SOUL.md reset to default template.")
	return nil
}
