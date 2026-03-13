package process

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolSchemas returns the JSON tool schemas that expose the process manager to
// the model. The schema format mirrors the rest of the SoulGate tool catalogue
// so that the orchestrator can pass them directly to any provider adapter.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "process_start",
			"description": "Start a long-running background process. Returns a process ID that can be used with other process tools to monitor output or send input. Useful for servers, build watchers, or any command that should not block the current conversation.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Shell command to execute (passed to sh -c). Example: \"npm run dev\"",
					},
					"workdir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for the process. Defaults to the current directory if omitted.",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of seconds the process may run before it is killed automatically. Defaults to 300 (5 minutes).",
					},
					"env": map[string]interface{}{
						"type":        "array",
						"description": "Additional environment variables in KEY=VALUE format appended to the inherited environment.",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"command"},
			},
		},
		{
			"name":        "process_list",
			"description": "List all managed background processes and their current status (running, exited, killed, failed).",
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "process_poll",
			"description": "Get the most recent output (up to 4 KB) from a background process together with its current status. Use this to check progress without fetching the full log.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Process ID returned by process_start.",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "process_log",
			"description": "Retrieve the last N lines of combined stdout+stderr output from a background process.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Process ID returned by process_start.",
					},
					"lines": map[string]interface{}{
						"type":        "number",
						"description": "Number of lines to return from the end of the log. Defaults to 50.",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "process_write",
			"description": "Send text to the standard input of a running background process. Useful for interactive CLIs or REPL-style tools.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Process ID returned by process_start.",
					},
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Text to write to the process stdin. Append \\n if the process expects a newline.",
					},
				},
				"required": []string{"id", "input"},
			},
		},
		{
			"name":        "process_kill",
			"description": "Terminate a running background process. Sends SIGTERM first and SIGKILL after a short grace period.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Process ID returned by process_start.",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

// ExecuteTool dispatches a tool call by name to the appropriate Manager method
// and returns a human-readable result string. args is the parsed JSON input
// object from the model.
func ExecuteTool(ctx context.Context, mgr *Manager, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "process_start":
		return executeStart(ctx, mgr, args)
	case "process_list":
		return executeList(mgr)
	case "process_poll":
		return executePoll(mgr, args)
	case "process_log":
		return executeLog(mgr, args)
	case "process_write":
		return executeWrite(mgr, args)
	case "process_kill":
		return executeKill(mgr, args)
	default:
		return "", fmt.Errorf("process: unknown tool %q", name)
	}
}

// executeStart handles the process_start tool call.
func executeStart(ctx context.Context, mgr *Manager, args map[string]interface{}) (string, error) {
	command, ok := stringArg(args, "command")
	if !ok || command == "" {
		return "", fmt.Errorf("process_start: missing required argument \"command\"")
	}

	workdir, _ := stringArg(args, "workdir")

	var timeout time.Duration
	if v, ok := args["timeout_seconds"]; ok {
		switch n := v.(type) {
		case float64:
			timeout = time.Duration(n) * time.Second
		case int:
			timeout = time.Duration(n) * time.Second
		}
	}

	var env []string
	if v, ok := args["env"]; ok {
		if rawEnv, ok := v.([]interface{}); ok {
			for _, item := range rawEnv {
				if s, ok := item.(string); ok {
					env = append(env, s)
				}
			}
		}
	}

	proc, err := mgr.Start(ctx, command, workdir, env, timeout)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Process started: id=%s pid=%d command=%q. Use process_poll or process_log to check output.",
		proc.ID, proc.PID, proc.Command,
	), nil
}

// executeList handles the process_list tool call.
func executeList(mgr *Manager) (string, error) {
	procs := mgr.List()
	if len(procs) == 0 {
		return "No managed processes.", nil
	}

	type row struct {
		ID        string `json:"id"`
		Command   string `json:"command"`
		Status    string `json:"status"`
		PID       int    `json:"pid"`
		ExitCode  int    `json:"exit_code,omitempty"`
		StartedAt string `json:"started_at"`
		EndedAt   string `json:"ended_at,omitempty"`
	}

	rows := make([]row, 0, len(procs))
	for _, p := range procs {
		p.mu.Lock()
		r := row{
			ID:        p.ID,
			Command:   p.Command,
			Status:    string(p.Status),
			PID:       p.PID,
			ExitCode:  p.ExitCode,
			StartedAt: p.StartedAt.Format(time.RFC3339),
		}
		if p.EndedAt != nil {
			r.EndedAt = p.EndedAt.Format(time.RFC3339)
		}
		p.mu.Unlock()
		rows = append(rows, r)
	}

	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", fmt.Errorf("process_list: failed to marshal result: %w", err)
	}
	return string(out), nil
}

// executePoll handles the process_poll tool call.
func executePoll(mgr *Manager, args map[string]interface{}) (string, error) {
	id, ok := stringArg(args, "id")
	if !ok || id == "" {
		return "", fmt.Errorf("process_poll: missing required argument \"id\"")
	}
	return mgr.Poll(id)
}

// executeLog handles the process_log tool call.
func executeLog(mgr *Manager, args map[string]interface{}) (string, error) {
	id, ok := stringArg(args, "id")
	if !ok || id == "" {
		return "", fmt.Errorf("process_log: missing required argument \"id\"")
	}

	lines := 0
	if v, ok := args["lines"]; ok {
		switch n := v.(type) {
		case float64:
			lines = int(n)
		case int:
			lines = n
		}
	}

	output, err := mgr.Log(id, lines)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(output) == "" {
		return fmt.Sprintf("Process %s has produced no output yet.", id), nil
	}
	return output, nil
}

// executeWrite handles the process_write tool call.
func executeWrite(mgr *Manager, args map[string]interface{}) (string, error) {
	id, ok := stringArg(args, "id")
	if !ok || id == "" {
		return "", fmt.Errorf("process_write: missing required argument \"id\"")
	}
	input, ok := stringArg(args, "input")
	if !ok {
		return "", fmt.Errorf("process_write: missing required argument \"input\"")
	}
	if err := mgr.Write(id, input); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %d byte(s) to stdin of process %s.", len(input), id), nil
}

// executeKill handles the process_kill tool call.
func executeKill(mgr *Manager, args map[string]interface{}) (string, error) {
	id, ok := stringArg(args, "id")
	if !ok || id == "" {
		return "", fmt.Errorf("process_kill: missing required argument \"id\"")
	}
	if err := mgr.Kill(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("Process %s has been terminated.", id), nil
}

// stringArg extracts a string value from an args map.
func stringArg(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
