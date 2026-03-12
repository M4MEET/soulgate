package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var (
	useInteractiveTUI bool
)

var chatCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive Terminal UI with chat",
	Long: `Launch the SoulGate Terminal User Interface (TUI).

This provides a beautiful interactive terminal interface where you can:
- Have multi-turn conversations with AI
- See streaming responses in real-time
- Use chat commands (/exit, /clear, /history)
- Build on previous context
- Enhanced TUI with autocomplete and keyboard navigation

Example:
  soulgate tui
  soulgate tui --interactive   (enhanced TUI - enabled by default)

Chat Commands:
  /exit, /quit    Exit the chat session
  /clear          Clear conversation history
  /history        Show conversation history
  /tools          List available tools
  /help           Show available commands

Keyboard Shortcuts (interactive mode):
  Ctrl+H          Show help
  Ctrl+L          Clear screen
  ↑↓              Navigate history
  Tab             Autocomplete`,
	RunE: runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)
	chatCmd.Flags().BoolVarP(&useInteractiveTUI, "interactive", "i", true, "Use interactive TUI with autocomplete and keyboard navigation")
}

func runChat(cmd *cobra.Command, args []string) error {
	// Get current working directory (where user ran the command)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Use global config directory (like openclaw's ~/.openclaw)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	globalConfigDir := filepath.Join(homeDir, ".soulgate")
	globalConfigFile := filepath.Join(globalConfigDir, "config.yml")

	// Create global config directory if it doesn't exist
	if _, err := os.Stat(globalConfigDir); os.IsNotExist(err) {
		if err := os.MkdirAll(globalConfigDir, 0755); err != nil {
			return fmt.Errorf("failed to create global config directory: %w", err)
		}
	}

	// Load or create global configuration
	var workspace *config.Workspace
	if _, err := os.Stat(globalConfigFile); os.IsNotExist(err) {
		// First time - create default config and prompt for setup
		workspace = &config.Workspace{
			Root:      cwd,              // Work from current directory
			ConfigDir: globalConfigDir,  // But config is in ~/.soulgate
			Config:    config.DefaultConfig(),
		}
		workspace.Config.Workspace.Root = cwd
		workspace.Config.Workspace.ConfigDir = globalConfigDir
		workspace.Config.Audit.DatabasePath = filepath.Join(globalConfigDir, "audit.db")
		workspace.Config.Policy.FilePath = filepath.Join(globalConfigDir, "policy.yml")

		// Prompt for configuration
		if err := promptChatConfiguration(workspace); err != nil {
			return fmt.Errorf("configuration failed: %w", err)
		}
	} else {
		// Load existing config
		cfg, err := config.LoadConfig(globalConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		workspace = &config.Workspace{
			Root:      cwd,              // Work from current directory
			ConfigDir: globalConfigDir,  // But config is in ~/.soulgate
			Config:    cfg,
		}

		// Check if API keys are configured
		needsConfig := false
		if workspace.Config.Model.DefaultProvider == "openai" && workspace.Config.Model.OpenAI.APIKey == "" {
			needsConfig = true
		} else if workspace.Config.Model.DefaultProvider == "anthropic" && workspace.Config.Model.Anthropic.APIKey == "" {
			needsConfig = true
		}

		// If configuration still needed, prompt for it
		if needsConfig {
			if err := promptChatConfiguration(workspace); err != nil {
				return fmt.Errorf("configuration failed: %w", err)
			}
		}
	}

	// Create orchestrator
	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}
	defer orch.Close()

	// Start interactive session
	if useInteractiveTUI {
		// Use enhanced TUI with Bubble Tea
		return RunInteractiveTUI(orch)
	}

	// Use classic text-based chat
	printChatWelcome(workspace.Config.Model.DefaultProvider)
	return runInteractiveChat(cmd.Context(), orch)
}

func printChatWelcome(provider string) {
	// Use enhanced banner and welcome
	printEnhancedWelcome(provider, "default")
}

func promptChatConfiguration(workspace *config.Workspace) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println(colorCyan("╭─────────────────────────────────────────────────────────╮"))
	fmt.Println(colorCyan("│") + "              " + colorBold("Quick Configuration for Chat") + "             " + colorCyan("│"))
	fmt.Println(colorCyan("╰─────────────────────────────────────────────────────────╯"))
	fmt.Println()
	fmt.Println("  Let's configure your AI provider to start chatting.")
	fmt.Println()

	// Select provider
	fmt.Println(colorBold("Select AI Provider:"))
	fmt.Println()
	fmt.Println(colorCyan("  ┌─ Ready to Use ──────────────────────────────────────┐"))
	fmt.Println("  " + colorGreen(" 1") + ".  OpenAI        " + colorGray("GPT-4o, GPT-4o-mini, o1, o3"))
	fmt.Println("  " + colorGreen(" 2") + ".  Anthropic     " + colorGray("Claude 3.5 Sonnet, Opus, Haiku"))
	fmt.Println("  " + colorGreen(" 3") + ".  Groq          " + colorGray("Ultra-fast (Llama, Mixtral, Gemma)"))
	fmt.Println("  " + colorGreen(" 4") + ".  OpenRouter    " + colorGray("100+ models via one API"))
	fmt.Println("  " + colorGreen(" 5") + ".  Together AI   " + colorGray("Open-source models, fine-tuning"))
	fmt.Println("  " + colorGreen(" 6") + ".  Ollama        " + colorGray("Local models (Llama, Mistral, etc.)"))
	fmt.Println(colorCyan("  ├─ Enterprise ────────────────────────────────────────┤"))
	fmt.Println("  " + colorGreen(" 7") + ".  Azure OpenAI  " + colorGray("Microsoft Azure hosted models"))
	fmt.Println(colorCyan("  ├─ Advanced ──────────────────────────────────────────┤"))
	fmt.Println("  " + colorGreen(" 8") + ".  Custom        " + colorGray("Any OpenAI-compatible endpoint"))
	fmt.Println(colorCyan("  ├─ Coming Soon ───────────────────────────────────────┤"))
	fmt.Println("  " + colorGray(" 9") + ".  Google Gemini " + colorGray("(needs custom API implementation)"))
	fmt.Println("  " + colorGray("10") + ".  xAI Grok      " + colorGray("(needs API access)"))
	fmt.Println("  " + colorGray("11") + ".  AWS Bedrock   " + colorGray("(needs AWS SDK integration)"))
	fmt.Println(colorCyan("  └─────────────────────────────────────────────────────┘"))
	fmt.Println()
	fmt.Print(colorBold("Enter choice [1-8]: "))

	choice, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	choice = strings.TrimSpace(choice)

	var provider string
	var modelName string
	var apiKeyEnvVar string
	var baseURL string
	var needsAPIKey bool = true

	switch choice {
	case "1":
		provider = "openai"
		modelName = "gpt-4o-mini"
		apiKeyEnvVar = "OPENAI_API_KEY"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("OpenAI"))
		fmt.Println("  " + colorGray("Default model: gpt-4o-mini"))
		fmt.Println()

	case "2":
		provider = "anthropic"
		modelName = "claude-3-5-sonnet-20241022"
		apiKeyEnvVar = "ANTHROPIC_API_KEY"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("Anthropic"))
		fmt.Println("  " + colorGray("Default model: claude-3-5-sonnet-20241022"))
		fmt.Println()

	case "3":
		provider = "groq"
		modelName = "llama-3.3-70b-versatile"
		apiKeyEnvVar = "GROQ_API_KEY"
		baseURL = "https://api.groq.com/openai/v1"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("Groq"))
		fmt.Println("  " + colorGray("Default model: llama-3.3-70b-versatile"))
		fmt.Println("  " + colorGray("Get free API key: https://console.groq.com"))
		fmt.Println()

	case "4":
		provider = "openrouter"
		modelName = "anthropic/claude-3.5-sonnet"
		apiKeyEnvVar = "OPENROUTER_API_KEY"
		baseURL = "https://openrouter.ai/api/v1"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("OpenRouter"))
		fmt.Println("  " + colorGray("Default model: anthropic/claude-3.5-sonnet"))
		fmt.Println("  " + colorGray("Access 100+ models - get key: https://openrouter.ai"))
		fmt.Println()

	case "5":
		provider = "together"
		modelName = "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"
		apiKeyEnvVar = "TOGETHER_API_KEY"
		baseURL = "https://api.together.xyz/v1"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("Together AI"))
		fmt.Println("  " + colorGray("Default model: Llama 3.1 70B Instruct"))
		fmt.Println("  " + colorGray("Get API key: https://api.together.ai"))
		fmt.Println()

	case "6":
		provider = "ollama"
		modelName = "llama3.2"
		apiKeyEnvVar = ""
		baseURL = "http://localhost:11434/v1"
		needsAPIKey = false
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("Ollama (Local)"))
		fmt.Println("  " + colorGray("Default model: llama3.2"))
		fmt.Println("  " + colorYellow("Note: Requires Ollama running locally"))
		fmt.Println("  " + colorGray("Install: https://ollama.ai"))
		fmt.Println()

	case "7":
		provider = "azure"
		modelName = "gpt-4o"
		apiKeyEnvVar = "AZURE_OPENAI_API_KEY"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("Azure OpenAI"))
		fmt.Println("  " + colorGray("Default model: gpt-4o"))
		fmt.Println()
		fmt.Print(colorBold("Azure endpoint (e.g., https://your-resource.openai.azure.com): "))
		endpoint, _ := reader.ReadString('\n')
		baseURL = strings.TrimSpace(endpoint)

	case "8":
		provider = "custom"
		apiKeyEnvVar = "CUSTOM_API_KEY"
		fmt.Println()
		fmt.Println("  " + colorGreen("✓") + " Selected: " + colorCyan("Custom OpenAI-Compatible"))
		fmt.Println()
		fmt.Print(colorBold("Base URL: "))
		url, _ := reader.ReadString('\n')
		baseURL = strings.TrimSpace(url)
		fmt.Print(colorBold("Model name: "))
		model, _ := reader.ReadString('\n')
		modelName = strings.TrimSpace(model)

	case "9", "10", "11":
		return fmt.Errorf("this provider is coming soon - please choose from options 1-8")

	default:
		return fmt.Errorf("invalid choice: %s (choose 1-8)", choice)
	}

	// Get API key (if needed)
	var apiKey string
	if needsAPIKey {
		fmt.Println(colorBold("Enter API Key:"))
		fmt.Println()
		if apiKeyEnvVar != "" {
			fmt.Println("  " + colorGray("Paste your API key below. It will be saved to .soulgate/config.yml"))
			fmt.Println("  " + colorGray("Or set environment variable: "+apiKeyEnvVar))
		} else {
			fmt.Println("  " + colorGray("Paste your API key below. It will be saved to .soulgate/config.yml"))
		}
		fmt.Println()
		fmt.Print(colorBold("API Key: "))

		keyInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		apiKey = strings.TrimSpace(keyInput)

		if apiKey == "" {
			return fmt.Errorf("API key cannot be empty")
		}
	}

	// Update workspace configuration
	workspace.Config.Model.DefaultProvider = provider

	// Map provider to configuration
	// Most providers use OpenAI-compatible API, so we store them in OpenAI config with custom baseURL
	switch provider {
	case "openai":
		workspace.Config.Model.OpenAI.APIKey = apiKey
		workspace.Config.Model.OpenAI.Model = modelName
		workspace.Config.Model.OpenAI.BaseURL = ""

	case "anthropic":
		workspace.Config.Model.Anthropic.APIKey = apiKey
		workspace.Config.Model.Anthropic.Model = modelName
		workspace.Config.Model.Anthropic.BaseURL = ""

	default:
		// All other providers use OpenAI-compatible API format
		// Store in OpenAI config with custom baseURL
		workspace.Config.Model.OpenAI.APIKey = apiKey
		workspace.Config.Model.OpenAI.Model = modelName
		workspace.Config.Model.OpenAI.BaseURL = baseURL
	}

	// Save configuration to global config directory
	homeDir, _ := os.UserHomeDir()
	globalConfigDir := filepath.Join(homeDir, ".soulgate")
	configPath := filepath.Join(globalConfigDir, "config.yml")

	// Ensure directory exists
	os.MkdirAll(globalConfigDir, 0755)

	if err := workspace.Config.Save(configPath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println()
	fmt.Println("  " + colorGreen("✓") + " Configuration saved!")
	fmt.Println()
	fmt.Println("  " + colorGray("Config location: ~/.soulgate/config.yml"))
	if apiKeyEnvVar != "" {
		fmt.Println("  " + colorGray("Tip: You can also set " + apiKeyEnvVar + " environment variable"))
	}
	fmt.Println()

	return nil
}

func runInteractiveChat(ctx context.Context, orch *core.Orchestrator) error {
	reader := bufio.NewReader(os.Stdin)
	conversationHistory := []chatMessage{}

	// Session stats
	sessionStats := &sessionStats{
		messageCount: 0,
		totalTokens:  0,
		startTime:    time.Now(),
	}

	for {
		// Print prompt with session indicator
		fmt.Print(colorBold(colorGreen(">>> ")) + colorGray(""))

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)

		// Skip empty input
		if input == "" {
			continue
		}

		// Handle shell commands (! prefix)
		if strings.HasPrefix(input, "!") {
			shellCmd := strings.TrimSpace(strings.TrimPrefix(input, "!"))
			if shellCmd != "" {
				executeShellCommand(shellCmd)
			}
			continue
		}

		// Handle chat commands
		if strings.HasPrefix(input, "/") {
			if shouldExit := handleChatCommand(input, &conversationHistory, orch, sessionStats); shouldExit {
				return nil
			}
			continue
		}

		// Add user message to history
		conversationHistory = append(conversationHistory, chatMessage{
			role:    "user",
			content: input,
		})

		// Show enhanced thinking indicator with elapsed time
		fmt.Println()
		cancelThinking := printThinkingIndicator(ctx)

		// Build contextual prompt with conversation history
		contextPrompt := buildConversationContext(conversationHistory)

		// Execute with full context
		result, err := orch.Run(ctx, contextPrompt)

		// Stop thinking indicator
		cancelThinking()

		if err != nil {
			fmt.Println(colorRed("  ✗ Error: " + err.Error()))
			fmt.Println()
			continue
		}

		// Update stats (calculate tokens from result if available)
		sessionStats.messageCount++
		// TODO: Get actual token usage from result when available
		// For now, estimate based on response length
		sessionStats.totalTokens += len(result.Response) / 4

		// Add assistant message to history
		conversationHistory = append(conversationHistory, chatMessage{
			role:    "assistant",
			content: result.Response,
		})

		// Print response
		printChatResponse(result.Response)
		fmt.Println()

		// Show brief stats
		fmt.Println(colorGray(fmt.Sprintf("  [%d msgs • %d tokens]", sessionStats.messageCount, sessionStats.totalTokens)))
		fmt.Println()
	}
}

func handleChatCommand(command string, history *[]chatMessage, orch *core.Orchestrator, stats *sessionStats) bool {
	cmd := strings.ToLower(strings.TrimSpace(command))

	switch {
	case cmd == "/exit" || cmd == "/quit" || cmd == "/q":
		fmt.Println()
		fmt.Println(colorCyan("Goodbye! 👋"))
		fmt.Println()
		if stats.messageCount > 0 {
			duration := time.Since(stats.startTime)
			fmt.Println(colorGray(fmt.Sprintf("  Session: %d messages, %d tokens, %s",
				stats.messageCount, stats.totalTokens, duration.Round(time.Second))))
			fmt.Println()
		}
		return true

	case cmd == "/clear":
		*history = []chatMessage{}
		stats.messageCount = 0
		stats.totalTokens = 0
		stats.startTime = time.Now()
		fmt.Println()
		fmt.Println(colorGreen("  ✓ Conversation cleared"))
		fmt.Println()
		return false

	case cmd == "/history":
		printChatHistory(*history)
		return false

	case cmd == "/status":
		printEnhancedStatus(orch, stats, len(*history))
		return false

	case cmd == "/tools":
		printToolsList(orch)
		return false

	case cmd == "/help":
		printEnhancedHelp()
		return false

	default:
		fmt.Println()
		fmt.Println(colorRed("  ✗ Unknown command: " + command))
		fmt.Println(colorGray("  Available: /help, /status, /history, /clear, /exit"))
		fmt.Println()
		return false
	}
}

func printChatResponse(response string) {
	// Print response with light formatting
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			fmt.Println()
		} else {
			fmt.Println("  " + line)
		}
	}
}

func printChatHistory(history []chatMessage) {
	fmt.Println()
	fmt.Println(colorCyan("─── Conversation History ───"))
	fmt.Println()

	if len(history) == 0 {
		fmt.Println(colorGray("  (empty)"))
		fmt.Println()
		return
	}

	for i, msg := range history {
		if msg.role == "user" {
			fmt.Printf("%s %s\n", colorGreen("User:"), msg.content)
		} else {
			fmt.Printf("%s %s\n", colorCyan("Assistant:"), msg.content)
		}

		// Add separator between exchanges
		if i < len(history)-1 && msg.role == "assistant" {
			fmt.Println(colorGray("  ···"))
		}
	}
	fmt.Println()
}

func printChatHelp() {
	fmt.Println()
	fmt.Println(colorCyan("╭─────────────────────────────────────────────────────────╮"))
	fmt.Println(colorCyan("│") + "                   " + colorBold("Chat Commands") + "                      " + colorCyan("│"))
	fmt.Println(colorCyan("╰─────────────────────────────────────────────────────────╯"))
	fmt.Println()
	fmt.Printf("  %s         Show session status (model, tokens, time)\n", colorBold("/status"))
	fmt.Printf("  %s        Show conversation history\n", colorBold("/history"))
	fmt.Printf("  %s          Clear conversation and start fresh\n", colorBold("/clear"))
	fmt.Printf("  %s           Show this help message\n", colorBold("/help"))
	fmt.Printf("  %s      Exit the chat session\n", colorBold("/exit, /quit"))
	fmt.Println()
	fmt.Println(colorGray("  Tip: Your conversation context persists across messages"))
	fmt.Println()
}

func printSessionStatus(orch *core.Orchestrator, stats *sessionStats, historyLen int) {
	session := orch.GetSession()
	fmt.Println()
	fmt.Println(colorCyan("╭─────────────────────────────────────────────────────────╮"))
	fmt.Println(colorCyan("│") + "                  " + colorBold("Session Status") + "                      " + colorCyan("│"))
	fmt.Println(colorCyan("╰─────────────────────────────────────────────────────────╯"))
	fmt.Println()
	fmt.Printf("  %s  %s\n", colorBold("Session ID:"), colorCyan(session.ID))
	fmt.Printf("  %s     %s\n", colorBold("Started:"), colorGray(stats.startTime.Format("2006-01-02 15:04:05")))
	fmt.Printf("  %s    %s\n", colorBold("Duration:"), colorGray(time.Since(stats.startTime).Round(time.Second).String()))
	fmt.Printf("  %s    %s\n", colorBold("Messages:"), colorGreen(fmt.Sprintf("%d", stats.messageCount)))
	fmt.Printf("  %s %s\n", colorBold("Tokens (est):"), colorGreen(fmt.Sprintf("%d", stats.totalTokens)))
	fmt.Printf("  %s     %s\n", colorBold("History:"), colorGreen(fmt.Sprintf("%d messages", historyLen)))
	fmt.Println()
}

// chatMessage represents a message in the conversation
type chatMessage struct {
	role    string
	content string
}

// sessionStats tracks session statistics
type sessionStats struct {
	messageCount int
	totalTokens  int
	startTime    time.Time
}

// buildConversationContext builds a prompt with full conversation history
func buildConversationContext(history []chatMessage) string {
	if len(history) == 0 {
		return ""
	}

	// If only one message, return it directly
	if len(history) == 1 {
		return history[0].content
	}

	// Build context with recent conversation (last 10 messages for context window)
	startIdx := 0
	if len(history) > 10 {
		startIdx = len(history) - 10
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Previous conversation:\n")

	for i := startIdx; i < len(history)-1; i++ {
		msg := history[i]
		if msg.role == "user" {
			contextBuilder.WriteString(fmt.Sprintf("User: %s\n", msg.content))
		} else if msg.role == "assistant" {
			contextBuilder.WriteString(fmt.Sprintf("Assistant: %s\n", msg.content))
		}
	}

	// Add current user message
	currentMsg := history[len(history)-1]
	contextBuilder.WriteString(fmt.Sprintf("\nCurrent request: %s", currentMsg.content))

	return contextBuilder.String()
}
