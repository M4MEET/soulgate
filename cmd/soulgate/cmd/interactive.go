package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Start interactive AI terminal",
	Long: `Start an interactive terminal session with SoulGate.

Chat naturally with AI, and SoulGate will handle file operations,
code analysis, documentation, and more - all with security controls.`,
	RunE: runInteractive,
}

var (
	autoSetup bool
)

func init() {
	rootCmd.AddCommand(interactiveCmd)
	interactiveCmd.Flags().BoolVar(&autoSetup, "auto-setup", true, "Automatically run setup if not configured")
}

func runInteractive(cmd *cobra.Command, args []string) error {
	// Print welcome banner
	printWelcomeBanner()

	// Check if initialized
	if !config.IsInitialized(".") {
		if autoSetup {
			fmt.Println("First time? Let me help you set up SoulGate...")
			fmt.Println()

			if err := runQuickSetup(); err != nil {
				return fmt.Errorf("setup failed: %w", err)
			}

			fmt.Println()
			fmt.Println("Setup complete! Let's get started...")
			fmt.Println()
		} else {
			fmt.Println("SoulGate is not set up yet.")
			fmt.Println()
			fmt.Println("Run one of these commands first:")
			fmt.Println("  soulgate setup          (interactive wizard)")
			fmt.Println("  soulgate init           (quick setup)")
			fmt.Println()
			fmt.Println("Or run: soulgate interactive --auto-setup")
			return nil
		}
	}

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Check API key
	provider := workspace.Config.Model.DefaultProvider
	if !isAPIKeyConfigured(provider) {
		fmt.Println("API Key Not Found")
		fmt.Println()
		fmt.Printf("I need an API key for %s to work.\n", provider)
		fmt.Println()

		if err := promptForAPIKey(provider); err != nil {
			return err
		}

		fmt.Println()
	}

	// Create orchestrator for real AI interaction
	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Start interactive REPL with real AI
	return startREPL(orch, workspace)
}

func printWelcomeBanner() {
	fmt.Println("+=======================================================+")
	fmt.Println("|     SoulGate Interactive AI Terminal                   |")
	fmt.Println("+=======================================================+")
	fmt.Println()
	fmt.Println("Welcome! I'm your AI assistant with secure access to your")
	fmt.Println("workspace. I can help you with:")
	fmt.Println()
	fmt.Println("  - File operations (read, search, analyze)")
	fmt.Println("  - Code analysis and generation")
	fmt.Println("  - Documentation")
	fmt.Println("  - Testing and quality checks")
	fmt.Println("  - Project management")
	fmt.Println()
	fmt.Println("All with security controls and audit logging!")
	fmt.Println()
	fmt.Println("--------------------------------------------------------")
	fmt.Println()
}

func startREPL(orch *core.Orchestrator, workspace *config.Workspace) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Type your request below (or 'help' for commands, 'exit' to quit)")
	fmt.Println()

	for {
		// Print prompt
		fmt.Print("You: ")

		// Read input
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)

		// Handle empty input
		if input == "" {
			continue
		}

		// Handle special commands
		switch strings.ToLower(input) {
		case "exit", "quit", "q":
			fmt.Println()
			fmt.Println("Goodbye! Thanks for using SoulGate.")
			return nil

		case "help", "?":
			printHelpMenu()
			continue

		case "status":
			printStatus(workspace)
			continue

		case "clear", "cls":
			clearScreen()
			printWelcomeBanner()
			continue
		}

		// Process AI request using the real orchestrator
		fmt.Println()
		fmt.Print("Assistant: ")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := orch.Run(ctx, input)
		cancel()

		if err != nil {
			fmt.Printf("\nError: %v\n", err)
			fmt.Println()
			continue
		}

		fmt.Println(result.Response)
		fmt.Println()
	}
}

func printHelpMenu() {
	fmt.Println()
	fmt.Println("+=======================================================+")
	fmt.Println("|              SoulGate Commands                         |")
	fmt.Println("+=======================================================+")
	fmt.Println()
	fmt.Println("Natural Language:")
	fmt.Println("   Just type what you want to do!")
	fmt.Println("   Examples:")
	fmt.Println("   - List all files in this directory")
	fmt.Println("   - Run tests and show coverage")
	fmt.Println("   - Generate API documentation")
	fmt.Println()
	fmt.Println("Special Commands:")
	fmt.Println("   status    - Show workspace status")
	fmt.Println("   help, ?   - Show this help")
	fmt.Println("   clear     - Clear screen")
	fmt.Println("   exit, quit - Exit terminal")
	fmt.Println()
}

func printStatus(workspace *config.Workspace) {
	fmt.Println()
	fmt.Println("SoulGate Status")
	fmt.Println("--------------------------------------------------------")
	fmt.Printf("  Workspace:  %s\n", workspace.Root)
	fmt.Printf("  Provider:   %s\n", workspace.Config.Model.DefaultProvider)
	fmt.Printf("  Audit:      %v\n", boolToStatus(workspace.Config.Audit.Enabled))
	fmt.Println()
}

func runQuickSetup() error {
	reader := bufio.NewReader(os.Stdin)

	// Initialize workspace
	fmt.Println("Creating workspace...")
	workspace, err := config.InitWorkspace(".")
	if err != nil {
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	fmt.Println("Workspace created!")
	fmt.Println()

	// Ask for model provider
	fmt.Println("Which AI provider do you want to use?")
	fmt.Println("  1) OpenAI (GPT-4, GPT-3.5)")
	fmt.Println("  2) Anthropic (Claude)")
	fmt.Println("  3) Ollama (Local models)")
	fmt.Print("\nChoice [1-3]: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	var provider string
	switch choice {
	case "1", "":
		provider = "openai"
	case "2":
		provider = "anthropic"
	case "3":
		provider = "ollama"
	default:
		provider = "openai"
	}

	workspace.Config.Model.DefaultProvider = provider

	// Save config
	configPath := workspace.ConfigDir + "/config.yml"
	if err := workspace.Config.Save(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func promptForAPIKey(provider string) error {
	reader := bufio.NewReader(os.Stdin)

	var envVar, exampleKey, getKeyURL string

	switch provider {
	case "openai":
		envVar = "OPENAI_API_KEY"
		exampleKey = "sk-proj-..."
		getKeyURL = "https://platform.openai.com/api-keys"
	case "anthropic":
		envVar = "ANTHROPIC_API_KEY"
		exampleKey = "sk-ant-..."
		getKeyURL = "https://console.anthropic.com/"
	default:
		return nil // Ollama doesn't need a key
	}

	fmt.Println("Options:")
	fmt.Println()
	fmt.Println("1) Set it now:")
	fmt.Printf("   export %s=\"%s\"\n", envVar, exampleKey)
	fmt.Println()
	fmt.Println("2) Get a key:")
	fmt.Printf("   %s\n", getKeyURL)
	fmt.Println()
	fmt.Println("3) Skip for now (you can set it later)")
	fmt.Println()
	fmt.Print("Enter your API key (or press Enter to skip): ")

	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey != "" {
		// Set environment variable for this session
		os.Setenv(envVar, apiKey)
		fmt.Println("API key set for this session!")
		fmt.Println()
		fmt.Println("To make it permanent, add this to your ~/.bashrc or ~/.zshrc:")
		fmt.Printf("   export %s=\"%s\"\n", envVar, apiKey)
	} else {
		fmt.Println("Skipped. Set it later with:")
		fmt.Printf("   export %s=\"your-key-here\"\n", envVar)
	}

	return nil
}

func isAPIKeyConfigured(provider string) bool {
	switch provider {
	case "openai":
		return os.Getenv("OPENAI_API_KEY") != ""
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY") != ""
	case "groq":
		return os.Getenv("GROQ_API_KEY") != ""
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY") != ""
	case "together":
		return os.Getenv("TOGETHER_API_KEY") != ""
	case "ollama":
		return true // No key needed
	default:
		return os.Getenv("OPENAI_API_KEY") != "" // Most use OpenAI-compatible
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func boolToStatus(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}
