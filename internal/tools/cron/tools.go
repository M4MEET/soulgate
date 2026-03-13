package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolSchemas returns the JSON tool schemas understood by ExecuteTool.
// Each schema follows the OpenAI/Anthropic function-calling convention so
// the caller can pass them directly to a model provider.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "cron_add",
			"description": "Schedule a new recurring or one-shot job. The job will call the AI with the given task prompt on the configured schedule.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable label for the job.",
					},
					"schedule": map[string]interface{}{
						"type":        "string",
						"description": "Schedule expression. For kind=at: RFC 3339 timestamp (e.g. \"2026-03-15T10:00:00Z\"). For kind=every: duration string (e.g. \"30m\", \"1h\", \"1d\"). For kind=cron: 5-field cron expression (e.g. \"0 9 * * 1-5\").",
					},
					"task": map[string]interface{}{
						"type":        "string",
						"description": "Prompt sent to the AI on each execution.",
					},
					"kind": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"at", "every", "cron"},
						"description": "Schedule kind: \"at\" fires once, \"every\" repeats at a fixed interval, \"cron\" follows a cron expression.",
					},
					"max_runs": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of times the job may run. 0 (default) means unlimited.",
					},
				},
				"required": []string{"name", "schedule", "task"},
			},
		},
		{
			"name":        "cron_list",
			"description": "List all scheduled jobs and their current status.",
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "cron_remove",
			"description": "Permanently delete a scheduled job.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Job ID (e.g. \"cron_1\").",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "cron_pause",
			"description": "Pause a job so it will not fire until resumed.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Job ID to pause.",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "cron_resume",
			"description": "Resume a previously paused job.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Job ID to resume.",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

// ExecuteTool dispatches a tool call to the scheduler and returns a
// human-readable result string. Unknown tool names return an error.
func ExecuteTool(ctx context.Context, sched *Scheduler, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "cron_add":
		return execAdd(sched, args)
	case "cron_list":
		return execList(sched)
	case "cron_remove":
		return execRemove(sched, args)
	case "cron_pause":
		return execPause(sched, args)
	case "cron_resume":
		return execResume(sched, args)
	default:
		return "", fmt.Errorf("cron: unknown tool %q", name)
	}
}

// --------------------------------------------------------------------------
// Individual tool handlers
// --------------------------------------------------------------------------

func execAdd(sched *Scheduler, args map[string]interface{}) (string, error) {
	jobName, err := stringArg(args, "name", true)
	if err != nil {
		return "", err
	}
	schedule, err := stringArg(args, "schedule", true)
	if err != nil {
		return "", err
	}
	task, err := stringArg(args, "task", true)
	if err != nil {
		return "", err
	}

	// "kind" defaults to "every" when omitted.
	kindStr, _ := stringArg(args, "kind", false)
	if kindStr == "" {
		kindStr = "every"
	}
	kind := ScheduleKind(kindStr)
	switch kind {
	case KindAt, KindEvery, KindCron:
		// valid
	default:
		return "", fmt.Errorf("cron_add: kind must be \"at\", \"every\", or \"cron\", got %q", kindStr)
	}

	maxRuns := 0
	if v, ok := args["max_runs"]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			maxRuns = int(n)
		case int:
			maxRuns = n
		case json.Number:
			parsed, err := n.Int64()
			if err != nil {
				return "", fmt.Errorf("cron_add: invalid max_runs value")
			}
			maxRuns = int(parsed)
		}
	}

	job, err := sched.Add(jobName, schedule, task, kind, maxRuns)
	if err != nil {
		return "", err
	}

	nextStr := "(none)"
	if job.NextRun != nil {
		nextStr = job.NextRun.UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("Scheduled job %s (%q, kind=%s, schedule=%q). First run: %s.", job.ID, job.Name, job.Kind, job.Schedule, nextStr), nil
}

func execList(sched *Scheduler) (string, error) {
	jobs := sched.List()
	if len(jobs) == 0 {
		return "No scheduled jobs.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-10s %-20s %-10s %-10s %-8s %-8s %s\n",
		"ID", "Name", "Kind", "Status", "Runs", "MaxRuns", "NextRun"))
	sb.WriteString(strings.Repeat("-", 85) + "\n")

	for _, j := range jobs {
		nextStr := "-"
		if j.NextRun != nil {
			nextStr = j.NextRun.UTC().Format(time.RFC3339)
		}
		maxStr := "∞"
		if j.MaxRuns > 0 {
			maxStr = fmt.Sprintf("%d", j.MaxRuns)
		}
		sb.WriteString(fmt.Sprintf("%-10s %-20s %-10s %-10s %-8d %-8s %s\n",
			j.ID, truncate(j.Name, 20), j.Kind, j.Status, j.RunCount, maxStr, nextStr))
	}
	return sb.String(), nil
}

func execRemove(sched *Scheduler, args map[string]interface{}) (string, error) {
	id, err := stringArg(args, "id", true)
	if err != nil {
		return "", err
	}
	if err := sched.Remove(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("Job %s removed.", id), nil
}

func execPause(sched *Scheduler, args map[string]interface{}) (string, error) {
	id, err := stringArg(args, "id", true)
	if err != nil {
		return "", err
	}
	if err := sched.Pause(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("Job %s paused.", id), nil
}

func execResume(sched *Scheduler, args map[string]interface{}) (string, error) {
	id, err := stringArg(args, "id", true)
	if err != nil {
		return "", err
	}
	if err := sched.Resume(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("Job %s resumed.", id), nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func stringArg(args map[string]interface{}, key string, required bool) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		if required {
			return "", fmt.Errorf("cron tool: missing required argument %q", key)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("cron tool: argument %q must be a string, got %T", key, v)
	}
	return s, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
