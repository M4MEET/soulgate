package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
)

// Enhanced chat features beyond basic OpenClaw functionality

// executeShellCommand executes a shell command (OpenClaw's `!command` feature)
func executeShellCommand(command string) {
	spinner := NewSpinner("Executing: " + command)
	spinner.Start()

	// Execute command
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		spinner.Stop("Command failed", "error")
		fmt.Println()
		fmt.Println(colorError("  Error: " + err.Error()))
	} else {
		spinner.Stop("Command completed", "success")
	}

	fmt.Println()

	// Display output
	if len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		maxLines := 20 // Limit output display
		if len(lines) > maxLines {
			lines = lines[:maxLines]
			lines = append(lines, colorMuted("... (output truncated, "+fmt.Sprintf("%d", len(strings.Split(string(output), "\n"))-maxLines)+" more lines)"))
		}

		for _, line := range lines {
			if line != "" {
				fmt.Println(colorMuted("  " + line))
			}
		}
		fmt.Println()
	}
}

// printToolExecution displays a tool execution with status
func printToolExecution(toolName string, args string, status string, result string) {
	te := &ToolExecution{
		Name:   toolName,
		Args:   args,
		Status: status,
		Result: result,
	}
	fmt.Print(te.Render())
}

// printEnhancedWelcome shows an enhanced welcome screen
func printEnhancedWelcome(provider string, modelName string) {
	// Show banner
	fmt.Print(Banner())

	// Status line
	statusItems := map[string]string{
		"Provider": provider,
		"Model":    modelName,
	}
	fmt.Print(StatusLine(statusItems))

	fmt.Println()
	fmt.Println(colorMuted("  Available commands:"))
	fmt.Println(colorMuted("    /exit, /quit     ") + colorMuted("Exit the chat"))
	fmt.Println(colorMuted("    /clear           ") + colorMuted("Clear conversation history"))
	fmt.Println(colorMuted("    /history         ") + colorMuted("Show conversation history"))
	fmt.Println(colorMuted("    /status          ") + colorMuted("Show session status"))
	fmt.Println(colorMuted("    /help            ") + colorMuted("Show help"))
	fmt.Println(colorMuted("    /tools           ") + colorMuted("Show available tools"))
	fmt.Println(colorMuted("    !<command>       ") + colorMuted("Execute shell command"))
	fmt.Println()
	fmt.Println(colorMuted("  Shortcuts:"))
	fmt.Println(colorMuted("    Ctrl+C           ") + colorMuted("Exit"))
	fmt.Println(colorMuted("    Ctrl+D           ") + colorMuted("Exit"))
	fmt.Println()
}

// printToolsList displays available tools
func printToolsList(orch *core.Orchestrator) {
	fmt.Println()
	fmt.Println(colorAccentBright("╭─ Available Tools ─────────────────────────────────────────╮"))
	fmt.Println()

	// Core tools
	fmt.Println(colorBold("  Core Tools (9):"))
	fmt.Println()
	coreTools := [][]string{
		{"files_read", "Read any file anywhere on the system"},
		{"files_write", "Create or modify files"},
		{"files_list", "List directory contents"},
		{"files_delete", "Delete files or directories"},
		{"exec_command", "Execute shell commands"},
		{"net_request", "Make HTTP requests"},
		{"memory_write", "Save to persistent memory"},
		{"memory_search", "Search memory"},
		{"memory_get", "Retrieve memory by key"},
	}

	table := NewTable([]string{"Tool", "Description"})
	for _, tool := range coreTools {
		table.AddRow(tool)
	}
	fmt.Print(table.Render())

	fmt.Println()
	fmt.Println(colorMuted("  Use these tools to interact with your system securely."))
	fmt.Println()
	fmt.Println(colorAccentBright("╰───────────────────────────────────────────────────────────╯"))
	fmt.Println()
}

// printEnhancedStatus shows enhanced status display
func printEnhancedStatus(orch *core.Orchestrator, stats *sessionStats, historyLen int) {
	session := orch.GetSession()

	fmt.Println()
	fmt.Print(Box("Session Status", fmt.Sprintf(`Session ID:    %s
Started:       %s
Duration:      %s
Messages:      %d
Tokens (est):  %d
History:       %d messages`,
		colorAccent(session.ID),
		colorMuted(stats.startTime.Format("2006-01-02 15:04:05")),
		colorMuted(time.Since(stats.startTime).Round(time.Second).String()),
		stats.messageCount,
		stats.totalTokens,
		historyLen), 60))
	fmt.Println()

	// Memory status
	fmt.Println(colorBold("  Workspace Files:"))
	fmt.Println(colorMuted("    ~/.soulgate/AGENTS.md    ") + checkFileExists("~/.soulgate/AGENTS.md"))
	fmt.Println(colorMuted("    ~/.soulgate/MEMORY.md    ") + checkFileExists("~/.soulgate/MEMORY.md"))
	fmt.Println(colorMuted("    ~/.soulgate/TOOLS.md     ") + checkFileExists("~/.soulgate/TOOLS.md"))
	fmt.Println(colorMuted("    ~/.soulgate/SOUL.md      ") + checkFileExists("~/.soulgate/SOUL.md"))
	fmt.Println()
}

// checkFileExists checks if a file exists and returns a status indicator
func checkFileExists(path string) string {
	// Expand home directory
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = strings.Replace(path, "~", home, 1)
	}

	if _, err := os.Stat(path); err == nil {
		return colorSuccess("✓ exists")
	}
	return colorMuted("○ not found")
}

// AnimatedSpinner shows an animated spinner with changing messages
type AnimatedSpinner struct {
	spinner  *Spinner
	messages []string
	tick     int
	stopChan chan bool
}

// NewAnimatedSpinner creates a new animated spinner
func NewAnimatedSpinner(messages []string) *AnimatedSpinner {
	return &AnimatedSpinner{
		spinner:  NewSpinner(messages[0]),
		messages: messages,
		tick:     0,
		stopChan: make(chan bool),
	}
}

// Start starts the animated spinner
func (as *AnimatedSpinner) Start() {
	as.spinner.Start()

	// Update message periodically
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-as.stopChan:
				return
			case <-ticker.C:
				as.tick++
				as.spinner.Update(as.messages[as.tick%len(as.messages)])
			}
		}
	}()
}

// Stop stops the animated spinner
func (as *AnimatedSpinner) Stop(finalMessage string, status string) {
	as.stopChan <- true
	as.spinner.Stop(finalMessage, status)
}

// StreamingDisplay shows streaming response display
type StreamingDisplay struct {
	content      strings.Builder
	lastUpdate   time.Time
	updateTicker *time.Ticker
}

// NewStreamingDisplay creates a new streaming display
func NewStreamingDisplay() *StreamingDisplay {
	return &StreamingDisplay{
		lastUpdate:   time.Now(),
		updateTicker: time.NewTicker(50 * time.Millisecond),
	}
}

// Append adds content to the streaming display
func (sd *StreamingDisplay) Append(text string) {
	sd.content.WriteString(text)

	// Throttled update
	if time.Since(sd.lastUpdate) > 100*time.Millisecond {
		sd.render()
		sd.lastUpdate = time.Now()
	}
}

// render displays the current content
func (sd *StreamingDisplay) render() {
	// Clear previous line and print updated content
	fmt.Print("\r" + strings.Repeat(" ", 120) + "\r")
	fmt.Print("  " + sd.content.String())
}

// Finish completes the streaming display
func (sd *StreamingDisplay) Finish() {
	sd.updateTicker.Stop()
	sd.render()
	fmt.Println()
}

// printEnhancedHelp shows enhanced help display
func printEnhancedHelp() {
	fmt.Println()
	fmt.Print(Box("SoulGate Chat Help", `Commands:
  /status        Show session status and workspace files
  /tools         List all available tools
  /history       Show conversation history
  /clear         Clear conversation and start fresh
  /help          Show this help message
  /exit, /quit   Exit the chat session

Shell Commands:
  !<command>     Execute local shell command
  Examples:
    !ls -la      List files in current directory
    !git status  Show git repository status
    !whoami      Show current user

Features:
  • Full system access (any file, any command)
  • Persistent memory across sessions
  • 9 integrations (GitHub, Docker, AWS, etc.)
  • Policy-based security and audit logging
  • Workspace file injection (AGENTS.md, MEMORY.md)`, 60))
	fmt.Println()
}

// printThinkingIndicator shows thinking with elapsed time
func printThinkingIndicator(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		start := time.Now()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		frame := 0
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		messageTick := 0

		for {
			select {
			case <-ctx.Done():
				// Clear the line
				fmt.Print("\r" + strings.Repeat(" ", 100) + "\r")
				return
			case <-ticker.C:
				elapsed := time.Since(start)
				message := GetWaitingMessage(messageTick / 4) // Change message every 2 seconds
				fmt.Printf("\r  %s %s %s",
					colorAccent(frames[frame]),
					colorMuted(message+"..."),
					colorMuted(fmt.Sprintf("(%ds)", int(elapsed.Seconds()))))

				frame = (frame + 1) % len(frames)
				messageTick++
			}
		}
	}()

	return cancel
}
