package cmd

import (
	"fmt"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var heartbeatCmd = &cobra.Command{
	Use:   "heartbeat",
	Short: "Manage the periodic heartbeat check",
	Long: `Control and inspect the SoulGate heartbeat subsystem.

The heartbeat periodically wakes the AI agent to proactively check for things
that need attention — failed agents, pending approvals, cron errors, etc.

Heartbeat instructions are read from .soulgate/HEARTBEAT.md each time. Edit
that file to customise what the AI checks on each tick.`,
}

var heartbeatStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show heartbeat configuration and state",
	RunE:  runHeartbeatStatus,
}

var heartbeatRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Trigger a heartbeat check immediately",
	Long: `Run one heartbeat cycle right now, regardless of the configured interval.

The AI will check for things that need attention and print any findings.
A response of "OK" means nothing requires action.`,
	Args: cobra.NoArgs,
	RunE: runHeartbeatRun,
}

func init() {
	rootCmd.AddCommand(heartbeatCmd)
	heartbeatCmd.AddCommand(heartbeatStatusCmd)
	heartbeatCmd.AddCommand(heartbeatRunCmd)
}

func runHeartbeatStatus(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	cfg := workspace.Config.Heartbeat

	fmt.Println("Heartbeat Configuration")
	fmt.Println("────────────────────────────────────────────")
	fmt.Printf("  Enabled:     %v\n", cfg.Enabled)
	fmt.Printf("  Interval:    %s\n", cfg.Interval)
	fmt.Printf("  Target:      %s\n", cfg.Target)
	fmt.Printf("  Prompt file: %s\n", cfg.PromptFile)

	if !cfg.Enabled {
		fmt.Println()
		fmt.Println("Heartbeat is disabled. To enable it, add to .soulgate/config.yml:")
		fmt.Println()
		fmt.Println("  heartbeat:")
		fmt.Println("    enabled: true")
		fmt.Println("    interval: 30m")
	}

	return nil
}

func runHeartbeatRun(cmd *cobra.Command, args []string) error {
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		return fmt.Errorf("failed to initialize orchestrator: %w", err)
	}
	defer orch.Close()

	fmt.Printf("Running heartbeat check...\n\n")
	start := time.Now()

	response, err := orch.GetHeartbeat().RunNow()
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}

	elapsed := time.Since(start).Round(time.Millisecond)

	if response == "" || response == "OK" || response == "ok" {
		fmt.Printf("OK — nothing needs attention (%s)\n", elapsed)
	} else {
		fmt.Printf("Attention needed (%s):\n\n%s\n", elapsed, response)
	}

	return nil
}
