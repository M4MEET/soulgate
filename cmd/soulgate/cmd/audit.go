package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Audit log management",
	Long:  `View and query the audit log of all agent operations.`,
}

var auditTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Show recent audit log entries",
	Long:  `Display the most recent entries from the audit log.`,
	RunE:  runAuditTail,
}

var (
	auditLast   int
	auditJSON   bool
	auditRunID  string
	auditType   string
	auditStatus string
)

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditTailCmd)

	auditTailCmd.Flags().IntVar(&auditLast, "last", 10, "Number of entries to show")
	auditTailCmd.Flags().BoolVar(&auditJSON, "json", false, "Output as JSON")
	auditTailCmd.Flags().StringVar(&auditRunID, "run", "", "Filter by run ID")
	auditTailCmd.Flags().StringVar(&auditType, "type", "", "Filter by event type")
	auditTailCmd.Flags().StringVar(&auditStatus, "status", "", "Filter by status")
}

func runAuditTail(cmd *cobra.Command, args []string) error {
	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Open audit logger
	auditLogger, err := audit.NewJSONLLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer auditLogger.Close()

	// Build filter
	filter := audit.QueryFilter{
		Limit: auditLast,
	}

	if auditRunID != "" {
		filter.RunID = auditRunID
	}
	if auditType != "" {
		filter.Type = audit.EventType(auditType)
	}
	if auditStatus != "" {
		filter.Status = audit.EventStatus(auditStatus)
	}

	// Query events
	ctx := context.Background()
	events, err := auditLogger.Query(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to query audit log: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("No audit events found")
		return nil
	}

	// Display events
	if auditJSON {
		return displayEventsJSON(events)
	}
	return displayEventsTable(events)
}

func displayEventsJSON(events []*audit.Event) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(events)
}

func displayEventsTable(events []*audit.Event) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	fmt.Fprintln(w, "TIMESTAMP\tTYPE\tSTATUS\tRESOURCE\tRUN_ID")
	fmt.Fprintln(w, "---------\t----\t------\t--------\t------")

	// Events (reverse order to show most recent last)
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		timestamp := event.Timestamp.Local().Format(time.RFC3339)
		resource := event.Resource
		if len(resource) > 40 {
			resource = resource[:37] + "..."
		}
		runID := event.RunID
		if len(runID) > 20 {
			runID = runID[:17] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			timestamp,
			event.Type,
			event.Status,
			resource,
			runID,
		)
	}

	return nil
}
