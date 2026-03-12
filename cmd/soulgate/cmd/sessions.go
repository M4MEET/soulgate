package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/session"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage session recordings",
	Long: `View and manage session JSONL recordings.

Sessions are stored as append-only JSONL files in the sessions/ directory.
Each session contains a complete transcript of:
- User messages
- Tool calls and results
- Agent responses
- All events

This enables:
- Debugging conversations
- Replaying sessions
- Audit trails
- Analysis`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	RunE:  runSessionsList,
}

var sessionsShowCmd = &cobra.Command{
	Use:   "show <session-id>",
	Short: "Show session contents",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsShow,
}

var sessionsInfoCmd = &cobra.Command{
	Use:   "info <session-id>",
	Short: "Show session metadata",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsInfo,
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsDelete,
}

var (
	sessionsDir    string
	sessionsFormat string
)

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsInfoCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)

	sessionsCmd.PersistentFlags().StringVar(&sessionsDir, "dir", "sessions", "Sessions directory")
	sessionsShowCmd.Flags().StringVar(&sessionsFormat, "format", "pretty", "Output format: pretty, json, raw")
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	sessions, err := storage.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	fmt.Printf("Found %d session(s):\n\n", len(sessions))

	for _, sessionID := range sessions {
		info, err := storage.GetSessionInfo(sessionID)
		if err != nil {
			fmt.Printf("  • %s (error: %v)\n", sessionID, err)
			continue
		}

		fmt.Printf("  • %s\n", sessionID)
		fmt.Printf("    Entries: %d (%d messages, %d tool calls, %d responses)\n",
			info.EntryCount, info.MessageCount, info.ToolCallCount, info.ResponseCount)
		fmt.Printf("    Created: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
		if info.EntryCount > 0 {
			fmt.Printf("    Duration: %s to %s\n",
				info.FirstEntry.Format("15:04:05"),
				info.LastEntry.Format("15:04:05"))
		}
		fmt.Println()
	}

	return nil
}

func runSessionsShow(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	entries, err := storage.ReadSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to read session: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("Session is empty or does not exist")
		return nil
	}

	switch sessionsFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)

	case "raw":
		for _, entry := range entries {
			data, _ := json.Marshal(entry)
			fmt.Println(string(data))
		}
		return nil

	case "pretty":
		fallthrough
	default:
		fmt.Printf("Session: %s\n", sessionID)
		fmt.Printf("Entries: %d\n\n", len(entries))

		for i, entry := range entries {
			timestamp := formatTimestamp(entry.Timestamp)
			fmt.Printf("[%d] %s | %s\n", i+1, timestamp, entry.Type)

			switch entry.Type {
			case "event.message":
				data := entry.Data.(map[string]interface{})
				if sender, ok := data["sender"].(map[string]interface{}); ok {
					fmt.Printf("    From: %s\n", sender["username"])
				}
				if text, ok := data["text"].(string); ok {
					fmt.Printf("    Text: %s\n", text)
				}

			case "event.tool.start":
				data := entry.Data.(map[string]interface{})
				if toolName, ok := data["tool_name"].(string); ok {
					fmt.Printf("    Tool: %s\n", toolName)
				}
				if args, ok := data["args"].(map[string]interface{}); ok {
					fmt.Printf("    Args: %v\n", args)
				}

			case "event.tool.end":
				data := entry.Data.(map[string]interface{})
				if toolName, ok := data["tool_name"].(string); ok {
					fmt.Printf("    Tool: %s\n", toolName)
				}
				if result, ok := data["result"]; ok {
					resultStr := fmt.Sprintf("%v", result)
					if len(resultStr) > 200 {
						resultStr = resultStr[:197] + "..."
					}
					fmt.Printf("    Result: %s\n", resultStr)
				}
				if errorMsg, ok := data["error"].(string); ok && errorMsg != "" {
					fmt.Printf("    Error: %s\n", errorMsg)
				}
				if duration, ok := data["duration"].(float64); ok {
					fmt.Printf("    Duration: %.0fms\n", duration)
				}

			case "cmd.channel.send":
				data := entry.Data.(map[string]interface{})
				if text, ok := data["text"].(string); ok {
					fmt.Printf("    Response: %s\n", text)
				}
			}

			fmt.Println()
		}

		return nil
	}
}

func runSessionsInfo(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	info, err := storage.GetSessionInfo(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session info: %w", err)
	}

	fmt.Printf("Session ID:    %s\n", info.SessionID)
	fmt.Printf("File Path:     %s\n", info.FilePath)
	fmt.Printf("File Size:     %d bytes\n", info.FileSize)
	fmt.Printf("Total Entries: %d\n", info.EntryCount)
	fmt.Printf("  Messages:    %d\n", info.MessageCount)
	fmt.Printf("  Tool Calls:  %d\n", info.ToolCallCount)
	fmt.Printf("  Responses:   %d\n", info.ResponseCount)
	fmt.Printf("Created At:    %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
	if info.EntryCount > 0 {
		fmt.Printf("First Entry:   %s\n", info.FirstEntry.Format("2006-01-02 15:04:05"))
		fmt.Printf("Last Entry:    %s\n", info.LastEntry.Format("2006-01-02 15:04:05"))
		duration := info.LastEntry.Sub(info.FirstEntry)
		fmt.Printf("Duration:      %s\n", duration)
	}

	return nil
}

func runSessionsDelete(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Get info before deleting
	info, err := storage.GetSessionInfo(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	fmt.Printf("Delete session %s?\n", sessionID)
	fmt.Printf("  %d entries, %d bytes\n", info.EntryCount, info.FileSize)
	fmt.Printf("  Created: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Print("\nType 'yes' to confirm: ")

	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Cancelled")
		return nil
	}

	if err := storage.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Println("✓ Session deleted")
	return nil
}

func formatTimestamp(ts int64) string {
	return fmt.Sprintf("%02d:%02d:%02d", (ts%86400)/3600, (ts%3600)/60, ts%60)
}
