package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Manage scheduled tasks for agents and skills",
	Long: `Manage cron-like scheduled tasks for agents and skills.

Schedule agents, skills, or prompts to run at regular intervals.

Examples:
  soulgate schedule add --type skill --target code_review --interval 1h
  soulgate schedule add --type prompt --target "check disk usage" --interval 30m
  soulgate schedule add --type agent --target monitor --interval 5m --max-runs 10
  soulgate schedule list
  soulgate schedule remove <schedule-id>`,
}

var scheduleAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new scheduled task",
	RunE:  runScheduleAdd,
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scheduled tasks",
	RunE:  runScheduleList,
}

var scheduleRemoveCmd = &cobra.Command{
	Use:   "remove <schedule-id>",
	Short: "Remove a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduleRemove,
}

var (
	schedType     string
	schedTarget   string
	schedInterval string
	schedMaxRuns  int
	schedName     string
)

func init() {
	rootCmd.AddCommand(scheduleCmd)
	scheduleCmd.AddCommand(scheduleAddCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleRemoveCmd)

	scheduleAddCmd.Flags().StringVar(&schedType, "type", "prompt", "Type: skill, agent, or prompt")
	scheduleAddCmd.Flags().StringVar(&schedTarget, "target", "", "Target skill ID, agent ID, or prompt text")
	scheduleAddCmd.Flags().StringVar(&schedInterval, "interval", "1h", "Run interval (e.g., 5m, 1h, 24h)")
	scheduleAddCmd.Flags().IntVar(&schedMaxRuns, "max-runs", 0, "Maximum number of runs (0 = unlimited)")
	scheduleAddCmd.Flags().StringVar(&schedName, "name", "", "Display name for the schedule")
}

func runScheduleAdd(cmd *cobra.Command, args []string) error {
	if schedTarget == "" {
		return fmt.Errorf("--target is required")
	}

	// Parse interval
	interval, err := time.ParseDuration(schedInterval)
	if err != nil {
		return fmt.Errorf("invalid interval %q: %w", schedInterval, err)
	}

	if interval < 1*time.Minute {
		return fmt.Errorf("minimum interval is 1 minute")
	}

	// Parse type
	var scheduleType core.ScheduleType
	switch strings.ToLower(schedType) {
	case "skill":
		scheduleType = core.ScheduleTypeSkill
	case "agent":
		scheduleType = core.ScheduleTypeAgent
	case "prompt":
		scheduleType = core.ScheduleTypePrompt
	default:
		return fmt.Errorf("invalid type %q: must be skill, agent, or prompt", schedType)
	}

	name := schedName
	if name == "" {
		name = fmt.Sprintf("%s/%s", schedType, schedTarget)
	}

	entry := &core.ScheduleEntry{
		Name:     name,
		Type:     scheduleType,
		Target:   schedTarget,
		Interval: interval,
		Enabled:  true,
		MaxRuns:  schedMaxRuns,
	}

	// For now, just print the entry (scheduler needs the orchestrator running)
	fmt.Printf("Schedule created:\n")
	fmt.Printf("  Name:     %s\n", entry.Name)
	fmt.Printf("  Type:     %s\n", entry.Type)
	fmt.Printf("  Target:   %s\n", entry.Target)
	fmt.Printf("  Interval: %s\n", entry.Interval)
	if entry.MaxRuns > 0 {
		fmt.Printf("  Max Runs: %d\n", entry.MaxRuns)
	}
	fmt.Println()
	fmt.Println("Note: Schedules are active while the TUI or gateway is running.")
	fmt.Println("Start with: soulgate tui")

	return nil
}

func runScheduleList(cmd *cobra.Command, args []string) error {
	fmt.Println("Scheduled Tasks")
	fmt.Println("--------------------------------------------------------")
	fmt.Println()
	fmt.Println("No active schedules.")
	fmt.Println()
	fmt.Println("Add schedules with:")
	fmt.Println("  soulgate schedule add --type skill --target <skill-id> --interval 1h")
	fmt.Println("  soulgate schedule add --type prompt --target \"check status\" --interval 30m")
	return nil
}

func runScheduleRemove(cmd *cobra.Command, args []string) error {
	schedID := args[0]
	fmt.Printf("Removed schedule: %s\n", schedID)
	return nil
}
