package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var retentionCmd = &cobra.Command{
	Use:   "retention",
	Short: "Data retention management",
	Long:  `Manage data retention policies for audit logs, sessions, cost data, and memory.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var retentionRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the retention policy",
	Long:  `Delete data that exceeds the configured retention limits.`,
	Args:  cobra.NoArgs,
	RunE:  runRetentionRun,
}

var retentionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what would be deleted",
	Long:  `Preview what the retention policy would delete without removing anything.`,
	RunE:  runRetentionStatus,
}

var complianceCmd = &cobra.Command{
	Use:   "compliance",
	Short: "Compliance and data governance",
	Long:  `Export audit data for compliance reporting and manage GDPR data erasure requests.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var complianceExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export compliance data",
	Long:  `Export audit events, sessions, cost data, and policy rules for compliance reporting.`,
	Args:  cobra.NoArgs,
	RunE:  runComplianceExport,
}

var compliancePurgeUserCmd = &cobra.Command{
	Use:   "purge-user <user-id>",
	Short: "Purge all data for a user (GDPR right to erasure)",
	Long:  `Permanently delete all stored data associated with the given user ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCompliancePurgeUser,
}

var (
	complianceUserID string
	complianceFrom   string
	complianceTo     string
	complianceOutput string
)

func init() {
	rootCmd.AddCommand(retentionCmd)
	retentionCmd.AddCommand(retentionRunCmd)
	retentionCmd.AddCommand(retentionStatusCmd)

	rootCmd.AddCommand(complianceCmd)
	complianceCmd.AddCommand(complianceExportCmd)
	complianceCmd.AddCommand(compliancePurgeUserCmd)

	complianceExportCmd.Flags().StringVar(&complianceUserID, "user", "", "Scope export to a specific user ID")
	complianceExportCmd.Flags().StringVar(&complianceFrom, "from", "", "Start date (YYYY-MM-DD)")
	complianceExportCmd.Flags().StringVar(&complianceTo, "to", "", "End date (YYYY-MM-DD)")
	complianceExportCmd.Flags().StringVar(&complianceOutput, "output", "", "Write export JSON to this file (default: stdout)")
}

// --- retention run ---

func runRetentionRun(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	policy := toRetentionPolicy(workspace.Config.Retention)
	result, err := core.RunRetention(workspace.ConfigDir, policy)
	if err != nil {
		return fmt.Errorf("retention run failed: %w", err)
	}

	fmt.Printf("Retention completed:\n")
	fmt.Printf("  Audit files deleted:    %d\n", result.AuditFilesDeleted)
	fmt.Printf("  Cost entries purged:    %d\n", result.CostEntriesPurged)
	fmt.Printf("  Memory entries purged:  %d\n", result.MemoryEntriesPurged)
	fmt.Printf("  Bytes freed:            %s\n", formatBytes(result.BytesFreed))
	return nil
}

// --- retention status ---

func runRetentionStatus(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	policy := toRetentionPolicy(workspace.Config.Retention)
	result, err := core.RetentionStatus(workspace.ConfigDir, policy)
	if err != nil {
		return fmt.Errorf("retention status failed: %w", err)
	}

	fmt.Printf("Retention status (would delete):\n")
	fmt.Printf("  Audit files:    %d\n", result.AuditFilesDeleted)
	fmt.Printf("  Cost entries:   %d\n", result.CostEntriesPurged)
	fmt.Printf("  Memory entries: %d\n", result.MemoryEntriesPurged)
	fmt.Printf("  Bytes freed:    %s\n", formatBytes(result.BytesFreed))

	cfg := workspace.Config.Retention
	fmt.Printf("\nConfigured limits:\n")
	fmt.Printf("  audit_log_days: %s\n", formatDays(cfg.AuditLogDays))
	fmt.Printf("  session_days:   %s\n", formatDays(cfg.SessionDays))
	fmt.Printf("  cost_log_days:  %s\n", formatDays(cfg.CostLogDays))
	fmt.Printf("  memory_days:    %s\n", formatDays(cfg.MemoryDays))
	fmt.Printf("  auto_purge:     %v\n", cfg.AutoPurge)
	return nil
}

// --- compliance export ---

func runComplianceExport(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	var from, to time.Time
	if complianceFrom != "" {
		from, err = time.ParseInLocation("2006-01-02", complianceFrom, time.UTC)
		if err != nil {
			return fmt.Errorf("invalid --from date %q (expected YYYY-MM-DD): %w", complianceFrom, err)
		}
	}
	if complianceTo != "" {
		to, err = time.ParseInLocation("2006-01-02", complianceTo, time.UTC)
		if err != nil {
			return fmt.Errorf("invalid --to date %q (expected YYYY-MM-DD): %w", complianceTo, err)
		}
		// Make "to" inclusive by advancing to end of day.
		to = to.Add(24*time.Hour - time.Second)
	}

	auditLogger, err := audit.NewJSONLLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer auditLogger.Close()

	ctx := context.Background()
	export, err := core.ExportCompliance(ctx, core.ComplianceOptions{
		ConfigDir:   workspace.ConfigDir,
		UserID:      complianceUserID,
		From:        from,
		To:          to,
		AuditLogger: auditLogger,
	})
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export: %w", err)
	}

	if complianceOutput != "" {
		if err := os.WriteFile(complianceOutput, data, 0600); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
		fmt.Printf("Compliance export written to %s\n", complianceOutput)
		fmt.Printf("  Audit events:  %d\n", len(export.AuditEvents))
		fmt.Printf("  Sessions:      %d\n", len(export.Sessions))
		fmt.Printf("  Cost entries:  %d\n", len(export.CostEntries))
		fmt.Printf("  Policy rules:  %d\n", len(export.Policies))
	} else {
		fmt.Println(string(data))
	}
	return nil
}

// --- compliance purge-user ---

func runCompliancePurgeUser(cmd *cobra.Command, args []string) error {
	userID := args[0]
	if userID == "" {
		return fmt.Errorf("user ID must not be empty")
	}

	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	auditLogger, err := audit.NewJSONLLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	defer auditLogger.Close()

	ctx := context.Background()
	if err := core.PurgeUserData(ctx, workspace.ConfigDir, userID, auditLogger); err != nil {
		return fmt.Errorf("purge user data failed: %w", err)
	}

	fmt.Printf("All data for user %q has been purged.\n", userID)
	return nil
}

// --- helpers ---

// toRetentionPolicy converts the config struct to the core.RetentionPolicy.
func toRetentionPolicy(cfg config.RetentionConfig) core.RetentionPolicy {
	return core.RetentionPolicy{
		AuditLogDays: cfg.AuditLogDays,
		SessionDays:  cfg.SessionDays,
		CostLogDays:  cfg.CostLogDays,
		MemoryDays:   cfg.MemoryDays,
		AutoPurge:    cfg.AutoPurge,
	}
}

func formatDays(days int) string {
	if days == 0 {
		return "forever"
	}
	return fmt.Sprintf("%d days", days)
}
