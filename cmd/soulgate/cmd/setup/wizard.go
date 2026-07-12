package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/spf13/cobra"
)

var SetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard for SoulGate",
	Long: `Interactive setup wizard that guides you through configuring SoulGate.

This wizard will help you:
  - Initialize your workspace
  - Configure model providers (OpenAI, Anthropic, Ollama)
  - Set up security policies
  - Configure consolidated agents
  - Enable audit logging
  - Set up notifications`,
	RunE: runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	// Set up signal handler for graceful cancellation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n" + strings.Repeat("─", 56))
		fmt.Println("Setup cancelled by user. No changes were made.")
		fmt.Println(strings.Repeat("─", 56))
		os.Exit(0)
	}()

	reader := bufio.NewReader(os.Stdin)

	printHeader()

	// Step 1: Workspace Path
	workspacePath := promptString(reader, "Workspace path", ".", "Directory where SoulGate will be initialized")

	// Validate and expand path
	workspacePath = expandPath(workspacePath)
	if err := validateWorkspacePath(workspacePath); err != nil {
		return fmt.Errorf("invalid workspace path: %w\n\nTip: Use an existing directory or create it first:\n  mkdir -p %s", err, workspacePath)
	}

	// Check if already initialized
	if config.IsInitialized(workspacePath) {
		fmt.Printf("\n%s Workspace already initialized at %s\n", colorYellow("⚠"), workspacePath)
		reconfigure := promptYesNo(reader, "Reconfigure existing workspace?", false, "This will overwrite existing configuration")
		if !reconfigure {
			fmt.Println("\nSetup cancelled. Your existing configuration is unchanged.")
			return nil
		}
	}

	printSection("1. MODEL PROVIDER CONFIGURATION")

	// Step 2: Model Provider
	providers := []string{"openai", "anthropic", "ollama", "skip"}
	provider := promptChoice(reader, "Select model provider", providers, "openai")

	var apiKey, baseURL, modelName string

	if provider != "skip" {
		switch provider {
		case "openai":
			printSubsection("OpenAI Configuration")
			fmt.Println("  Get your API key from: " + colorBlue(config.GetProviderAPIKeyInstructions("openai")))
			fmt.Println()

			apiKey = promptString(reader, "OpenAI API Key", os.Getenv("OPENAI_API_KEY"), "Your OpenAI API key (sk-...)")
			if apiKey != "" {
				if err := config.ValidateAPIKey("openai", apiKey); err != nil {
					fmt.Println(colorYellow("  Warning: " + err.Error()))
				}
			}

			modelName = promptString(reader, "Model name", "gpt-5-mini", "GPT-5-mini is fast and economical")

		case "anthropic":
			printSubsection("Anthropic Configuration")
			fmt.Println("  Get your API key from: " + colorBlue(config.GetProviderAPIKeyInstructions("anthropic")))
			fmt.Println()

			apiKey = promptString(reader, "Anthropic API Key", os.Getenv("ANTHROPIC_API_KEY"), "Your Anthropic API key")
			if apiKey != "" {
				if err := config.ValidateAPIKey("anthropic", apiKey); err != nil {
					fmt.Println(colorYellow("  Warning: " + err.Error()))
				}
			}

			modelName = promptString(reader, "Model name", "claude-sonnet-5", "Latest Claude Sonnet model")

		case "ollama":
			printSubsection("Ollama Configuration")
			fmt.Println("  Make sure Ollama is running locally")
			fmt.Println()

			baseURL = promptString(reader, "Ollama URL", "http://localhost:11434", "URL where Ollama is running")
			modelName = promptString(reader, "Model name", "llama2", "Model name (e.g., llama2, mistral, codellama)")
		}
	}

	printSection("2. SECURITY POLICY CONFIGURATION")

	// Step 3: Security Policy
	policyModes := []string{"strict", "moderate", "permissive", "custom"}
	policyMode := promptChoice(reader, "Security policy mode", policyModes, "moderate")

	var allowedPaths []string
	var enableNetworkAccess, enableExecution bool

	switch policyMode {
	case "strict":
		fmt.Println()
		fmt.Println("  " + formatCheckmark(true) + " Strict mode: Only workspace files, no network, no execution")
		allowedPaths = []string{workspacePath}
		enableNetworkAccess = false
		enableExecution = false

	case "moderate":
		fmt.Println()
		fmt.Println("  " + formatCheckmark(true) + " Moderate mode: Workspace + project files, limited network, no execution")
		allowedPaths = []string{workspacePath, "$HOME/projects"}
		enableNetworkAccess = promptYesNo(reader, "Enable network access for specific domains?", false, "Allow HTTP/HTTPS requests to approved domains")
		enableExecution = false

	case "permissive":
		fmt.Println()
		fmt.Println("  " + formatCheckmark(true) + " Permissive mode: Home directory, network enabled, execution allowed")
		allowedPaths = []string{"$HOME"}
		enableNetworkAccess = true
		enableExecution = promptYesNo(reader, "Enable command execution?", false, "Allow running shell commands (use with caution)")

	case "custom":
		fmt.Println()
		fmt.Println("  " + formatCheckmark(true) + " Custom mode: You configure everything")
		numPaths := promptInt(reader, "Number of allowed paths", 1, 1, 10, "How many directories should agents be able to access?")
		for i := 0; i < numPaths; i++ {
			path := promptString(reader, fmt.Sprintf("Allowed path %d", i+1), workspacePath, "Absolute or workspace-relative path")
			path = expandPath(path)
			allowedPaths = append(allowedPaths, path)
		}
		enableNetworkAccess = promptYesNo(reader, "Enable network access?", false, "Allow HTTP/HTTPS requests")
		enableExecution = promptYesNo(reader, "Enable command execution?", false, "Allow running shell commands")
	}

	printSection("3. CONSOLIDATED AGENTS CONFIGURATION")

	// Step 4: Agents Configuration
	enableTestAgent := promptYesNo(reader, "Enable Test & Quality Agent?", true, "Runs tests, checks coverage, suggests improvements")
	enableDocsAgent := promptYesNo(reader, "Enable Docs & API Agent?", true, "Generates documentation, API specs, and examples")
	enablePMAgent := promptYesNo(reader, "Enable Project Management Agent?", true, "Plans tasks, tracks progress, assigns work")

	var coverageTarget int
	var assignmentStrategy string

	if enableTestAgent {
		coverageTarget = promptInt(reader, "Test coverage target (%)", 80, 0, 100, "Desired code coverage percentage")
	}

	if enablePMAgent {
		strategies := []string{"skill_based", "round_robin", "workload_balanced"}
		assignmentStrategy = promptChoice(reader, "Task assignment strategy", strategies, "skill_based")
	}

	printSection("4. AUDIT & NOTIFICATIONS")

	// Step 5: Audit & Notifications
	enableAudit := promptYesNo(reader, "Enable audit logging?", true, "Records all agent actions to SQLite database")
	enableNotifications := promptYesNo(reader, "Enable notifications?", true, "Get notified of important events")

	var notificationChannels []string
	if enableNotifications {
		fmt.Println("\n  Available channels: " + colorCyan("console") + ", " + colorCyan("slack") + ", " + colorCyan("email") + ", " + colorCyan("webhook"))
		channels := promptString(reader, "Notification channels (comma-separated)", "console", "List of notification channels")
		notificationChannels = strings.Split(channels, ",")
		for i := range notificationChannels {
			notificationChannels[i] = strings.TrimSpace(notificationChannels[i])
		}
	}

	printSection("5. REVIEW CONFIGURATION")

	// Display configuration summary
	fmt.Println("Configuration Summary:")
	fmt.Println()
	fmt.Printf("  %-20s %s\n", "Workspace:", workspacePath)
	fmt.Printf("  %-20s %s\n", "Model Provider:", provider)
	if provider != "skip" {
		fmt.Printf("  %-20s %s\n", "Model Name:", modelName)
	}
	fmt.Printf("  %-20s %s\n", "Security Mode:", policyMode)
	fmt.Printf("  %-20s %s\n", "Allowed Paths:", formatPaths(allowedPaths))
	fmt.Printf("  %-20s %s\n", "Network Access:", formatBool(enableNetworkAccess))
	fmt.Printf("  %-20s %s\n", "Command Execution:", formatBool(enableExecution))
	fmt.Printf("  %-20s %s\n", "Test Agent:", formatBool(enableTestAgent))
	if enableTestAgent {
		fmt.Printf("  %-20s %d%%\n", "  Coverage Target:", coverageTarget)
	}
	fmt.Printf("  %-20s %s\n", "Docs Agent:", formatBool(enableDocsAgent))
	fmt.Printf("  %-20s %s\n", "PM Agent:", formatBool(enablePMAgent))
	if enablePMAgent {
		fmt.Printf("  %-20s %s\n", "  Assignment:", assignmentStrategy)
	}
	fmt.Printf("  %-20s %s\n", "Audit Logging:", formatBool(enableAudit))
	fmt.Printf("  %-20s %s\n", "Notifications:", formatChannels(notificationChannels))
	fmt.Println()

	proceed := promptYesNo(reader, "Proceed with this configuration?", true, "Start workspace initialization")
	if !proceed {
		fmt.Println("\nSetup cancelled. No changes were made.")
		return nil
	}

	printSection("6. INITIALIZING WORKSPACE")

	// Show progress during initialization
	steps := []struct {
		message string
		action  func() error
	}{
		{"Creating workspace directory structure", func() error {
			workspace, err := config.InitWorkspace(workspacePath)
			if err != nil {
				return err
			}
			fmt.Printf("  %s Created workspace at %s\n", formatCheckmark(true), workspace.Root)
			return nil
		}},
		{"Generating configuration file", func() error {
			workspace, _ := config.InitWorkspace(workspacePath)
			if err := createSetupConfig(workspace.ConfigDir, setupConfig{
				provider:             provider,
				apiKey:               apiKey,
				baseURL:              baseURL,
				modelName:            modelName,
				policyMode:           policyMode,
				allowedPaths:         allowedPaths,
				enableNetworkAccess:  enableNetworkAccess,
				enableExecution:      enableExecution,
				enableTestAgent:      enableTestAgent,
				enableDocsAgent:      enableDocsAgent,
				enablePMAgent:        enablePMAgent,
				coverageTarget:       coverageTarget,
				assignmentStrategy:   assignmentStrategy,
				enableAudit:          enableAudit,
				notificationChannels: notificationChannels,
			}); err != nil {
				return err
			}
			fmt.Printf("  %s Created config.yml\n", formatCheckmark(true))
			return nil
		}},
		{"Creating security policy", func() error {
			fmt.Printf("  %s Created policy.yml\n", formatCheckmark(true))
			return nil
		}},
		{"Setting up audit log", func() error {
			if enableAudit {
				fmt.Printf("  %s Created audit.jsonl\n", formatCheckmark(true))
			} else {
				fmt.Printf("  %s Audit logging disabled\n", formatCheckmark(false))
			}
			return nil
		}},
		{"Configuring agents", func() error {
			if enableTestAgent || enableDocsAgent || enablePMAgent {
				fmt.Printf("  %s Created agents.yaml\n", formatCheckmark(true))
			} else {
				fmt.Printf("  %s No agents enabled\n", formatCheckmark(false))
			}
			return nil
		}},
	}

	for _, step := range steps {
		if err := step.action(); err != nil {
			return fmt.Errorf("\n%s %s\n\n%w", formatCheckmark(false), step.message, err)
		}
	}

	printSuccess()

	// Show next steps
	workspace, _ := config.InitWorkspace(workspacePath)
	printNextSteps(provider, apiKey, workspace.ConfigDir)

	return nil
}

// ============================================================================
// Visual/UI Helpers
// ============================================================================

func printHeader() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║           SoulGate Interactive Setup Wizard             ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  Welcome! This wizard will help you configure SoulGate.")
	fmt.Println("  Press Ctrl+C at any time to cancel.")
	fmt.Println()
}

func printSection(title string) {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 58))
	fmt.Println(title)
	fmt.Println(strings.Repeat("─", 58))
	fmt.Println()
}

func printSubsection(title string) {
	fmt.Println()
	fmt.Println(colorBold(title))
}

func printSuccess() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║              " + colorGreen("Setup Complete!") + "                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func printNextSteps(provider, apiKey, configDir string) {
	fmt.Println("Next steps:")
	fmt.Println()

	step := 1

	if provider != "skip" && apiKey != "" {
		envVar := ""
		switch provider {
		case "openai":
			envVar = "OPENAI_API_KEY"
		case "anthropic":
			envVar = "ANTHROPIC_API_KEY"
		}
		if envVar != "" {
			fmt.Printf("  %s. Set your API key:\n", colorCyan(fmt.Sprintf("%d", step)))
			fmt.Printf("     %s\n", colorGray(fmt.Sprintf("export %s=\"%s\"", envVar, apiKey)))
			fmt.Println()
			step++
		}
	}

	fmt.Printf("  %s. Review your configuration:\n", colorCyan(fmt.Sprintf("%d", step)))
	fmt.Printf("     %s\n", colorGray(fmt.Sprintf("cat %s/config.yml", configDir)))
	fmt.Println()
	step++

	fmt.Printf("  %s. Review your security policy:\n", colorCyan(fmt.Sprintf("%d", step)))
	fmt.Printf("     %s\n", colorGray(fmt.Sprintf("cat %s/policy.yml", configDir)))
	fmt.Println()
	step++

	fmt.Printf("  %s. Start using SoulGate:\n", colorCyan(fmt.Sprintf("%d", step)))
	fmt.Printf("     %s\n", colorGray("soulgate run \"What files are in this directory?\""))
	fmt.Println()
	step++

	fmt.Printf("  %s. View available commands:\n", colorCyan(fmt.Sprintf("%d", step)))
	fmt.Printf("     %s\n", colorGray("soulgate --help"))
	fmt.Println()
}
