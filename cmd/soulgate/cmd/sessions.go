package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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

var sessionsExportCmd = &cobra.Command{
	Use:   "export <session-id>",
	Short: "Export a session to a file",
	Long: `Export a session transcript to a file.

Supported formats:
  json  - Raw session data as a JSON array
  md    - Formatted Markdown conversation transcript
  html  - Styled standalone HTML file with dark theme

The output file is written to <session-id>.<ext> in the current directory
unless --output is specified.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsExport,
}

var sessionsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all sessions",
	Long: `Search all session files for entries containing the given query text.

The search is case-insensitive and matches against message text, tool names,
tool results, and other human-readable fields stored in each session entry.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsSearch,
}

var sessionsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show aggregate statistics across all sessions",
	RunE:  runSessionsStats,
}

var (
	sessionsDir    string
	sessionsFormat string

	// export flags
	exportFormat string
	exportOutput string
)

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsShowCmd)
	sessionsCmd.AddCommand(sessionsInfoCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsExportCmd)
	sessionsCmd.AddCommand(sessionsSearchCmd)
	sessionsCmd.AddCommand(sessionsStatsCmd)

	sessionsCmd.PersistentFlags().StringVar(&sessionsDir, "dir", "sessions", "Sessions directory")
	sessionsShowCmd.Flags().StringVar(&sessionsFormat, "format", "pretty", "Output format: pretty, json, raw")

	sessionsExportCmd.Flags().StringVar(&exportFormat, "format", "md", "Export format: json, md, html")
	sessionsExportCmd.Flags().StringVar(&exportOutput, "output", "", "Output file path (default: <session-id>.<ext>)")
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
			fmt.Printf("  - %s (error: %v)\n", sessionID, err)
			continue
		}

		fmt.Printf("  - %s\n", sessionID)
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

	fmt.Println("Session deleted")
	return nil
}

// runSessionsExport exports a single session in the requested format.
func runSessionsExport(cmd *cobra.Command, args []string) error {
	sessionID := args[0]

	// Validate format flag.
	switch exportFormat {
	case "json", "md", "html":
	default:
		return fmt.Errorf("unsupported format %q: must be json, md, or html", exportFormat)
	}

	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	entries, err := storage.ReadSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to read session: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("session %q is empty or does not exist", sessionID)
	}

	// Determine output file path.
	outPath := exportOutput
	if outPath == "" {
		ext := exportFormat
		if ext == "md" {
			ext = "md"
		}
		outPath = fmt.Sprintf("%s.%s", sessionID, ext)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	switch exportFormat {
	case "json":
		err = session.ExportJSON(entries, f)
	case "md":
		err = session.ExportMarkdown(entries, f)
	case "html":
		err = session.ExportHTML(entries, f)
	}
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	fmt.Printf("Session exported to %s\n", outPath)
	return nil
}

// runSessionsSearch searches all sessions for entries matching the query.
func runSessionsSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	results, err := session.Search(storage, query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Printf("No results found for %q\n", query)
		return nil
	}

	fmt.Printf("Found %d result(s) for %q:\n\n", len(results), query)
	for _, r := range results {
		fmt.Printf("  Session:   %s\n", r.SessionID)
		fmt.Printf("  Time:      %s\n", r.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Type:      %s\n", r.EntryType)
		fmt.Printf("  Context:   %s\n", r.Context)
		fmt.Println()
	}

	return nil
}

// runSessionsStats computes and displays aggregate statistics.
func runSessionsStats(cmd *cobra.Command, args []string) error {
	storage, err := session.NewStorage(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	sessionIDs, err := storage.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	var (
		totalMessages  int
		totalToolCalls int
		totalResponses int
		totalEntries   int
		totalBytes     int64
	)

	// day -> number of sessions active on that day
	activeDays := make(map[string]int)

	for _, sid := range sessionIDs {
		info, err := storage.GetSessionInfo(sid)
		if err != nil {
			continue
		}

		totalEntries += info.EntryCount
		totalMessages += info.MessageCount
		totalToolCalls += info.ToolCallCount
		totalResponses += info.ResponseCount
		totalBytes += info.FileSize

		day := info.CreatedAt.Format("2006-01-02")
		activeDays[day]++
	}

	// Find most active day.
	type dayCount struct {
		day   string
		count int
	}
	var days []dayCount
	for d, c := range activeDays {
		days = append(days, dayCount{d, c})
	}
	sort.Slice(days, func(i, j int) bool {
		if days[i].count != days[j].count {
			return days[i].count > days[j].count
		}
		return days[i].day > days[j].day
	})

	fmt.Println("Session Statistics")
	fmt.Println(strings.Repeat("-", 36))
	fmt.Printf("Total sessions:    %d\n", len(sessionIDs))
	fmt.Printf("Total entries:     %d\n", totalEntries)
	fmt.Printf("  Messages:        %d\n", totalMessages)
	fmt.Printf("  Tool calls:      %d\n", totalToolCalls)
	fmt.Printf("  Responses:       %d\n", totalResponses)
	fmt.Printf("Total data:        %s\n", formatBytes(totalBytes))
	fmt.Printf("Active days:       %d\n", len(activeDays))

	if len(days) > 0 {
		fmt.Printf("Most active day:   %s (%d sessions)\n", days[0].day, days[0].count)
	}

	if len(days) > 0 {
		fmt.Printf("\nActivity by day:\n")
		// Show up to 10 most active days.
		limit := len(days)
		if limit > 10 {
			limit = 10
		}
		for _, d := range days[:limit] {
			bar := strings.Repeat("#", d.count)
			ts, _ := time.Parse("2006-01-02", d.day)
			fmt.Printf("  %-12s  %s %d\n", ts.Format("Mon Jan 02"), bar, d.count)
		}
	}

	return nil
}

func formatTimestamp(ts int64) string {
	return fmt.Sprintf("%02d:%02d:%02d", (ts%86400)/3600, (ts%3600)/60, ts%60)
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
