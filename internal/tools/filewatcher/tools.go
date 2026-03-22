package filewatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ToolSchemas returns the JSON tool schemas that expose the file watcher to
// the model.  The schema format is identical to the rest of the SoulGate tool
// catalogue so the orchestrator can pass them directly to any provider adapter.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "watch_start",
			"description": "Start watching a file or directory for changes (create, modify, delete). When a matching change is detected the given action text is forwarded to the AI so it can react. Returns a watcher ID for later management.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file or directory to watch. Accepts relative (resolved from workspace root) or absolute paths.",
					},
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Optional glob pattern applied to changed file names (e.g. \"*.go\", \"*.json\"). Omit or pass \"\" to match every file.",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Description of what the AI should do when a change is detected. Example: \"describe what changed in the file\".",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, all sub-directories are also watched. New sub-directories created after the watcher starts are included automatically. Default false.",
					},
				},
				"required": []string{"path", "action"},
			},
		},
		{
			"name":        "watch_list",
			"description": "List all active file watchers including their paths, patterns, event counts, and the action they trigger.",
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "watch_stop",
			"description": "Stop an active file watcher and release its resources. No more callbacks will fire after this returns.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Watcher ID returned by watch_start (e.g. \"watch_1\").",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

// ExecuteTool dispatches a tool call by name to the appropriate Manager method
// and returns a human-readable result string.  args is the parsed JSON input
// object from the model.
func ExecuteTool(ctx context.Context, mgr *Manager, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "watch_start":
		return executeStart(ctx, mgr, args)
	case "watch_list":
		return executeList(mgr)
	case "watch_stop":
		return executeStop(mgr, args)
	default:
		return "", fmt.Errorf("filewatcher: unknown tool %q", name)
	}
}

// --------------------------------------------------------------------------
// Individual handlers
// --------------------------------------------------------------------------

func executeStart(ctx context.Context, mgr *Manager, args map[string]interface{}) (string, error) {
	path, err := stringArg(args, "path", true)
	if err != nil {
		return "", err
	}

	action, err := stringArg(args, "action", true)
	if err != nil {
		return "", err
	}

	pattern, _ := stringArg(args, "pattern", false)

	recursive := false
	if v, ok := args["recursive"]; ok {
		switch b := v.(type) {
		case bool:
			recursive = b
		case float64:
			recursive = b != 0
		}
	}

	id, err := mgr.Start(ctx, path, pattern, action, recursive)
	if err != nil {
		return "", err
	}

	details := fmt.Sprintf("path=%q", path)
	if pattern != "" {
		details += fmt.Sprintf(" pattern=%q", pattern)
	}
	if recursive {
		details += " recursive=true"
	}

	return fmt.Sprintf(
		"File watcher started: id=%s %s. Action on change: %q. Use watch_list to see status or watch_stop to cancel.",
		id, details, action,
	), nil
}

func executeList(mgr *Manager) (string, error) {
	watchers := mgr.List()
	if len(watchers) == 0 {
		return "No active file watchers.", nil
	}

	type row struct {
		ID        string `json:"id"`
		Path      string `json:"path"`
		Pattern   string `json:"pattern,omitempty"`
		Action    string `json:"action"`
		Recursive bool   `json:"recursive"`
		Events    int64  `json:"events"`
		CreatedAt string `json:"created_at"`
	}

	rows := make([]row, 0, len(watchers))
	for _, w := range watchers {
		rows = append(rows, row{
			ID:        w.ID,
			Path:      w.Path,
			Pattern:   w.Pattern,
			Action:    w.Action,
			Recursive: w.Recursive,
			Events:    w.Events,
			CreatedAt: w.CreatedAt.Format(time.RFC3339),
		})
	}

	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", fmt.Errorf("watch_list: failed to marshal result: %w", err)
	}
	return string(out), nil
}

func executeStop(mgr *Manager, args map[string]interface{}) (string, error) {
	id, err := stringArg(args, "id", true)
	if err != nil {
		return "", err
	}
	if err := mgr.Stop(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("Watcher %s stopped.", id), nil
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func stringArg(args map[string]interface{}, key string, required bool) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		if required {
			return "", fmt.Errorf("watch tool: missing required argument %q", key)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("watch tool: argument %q must be a string, got %T", key, v)
	}
	return strings.TrimSpace(s), nil
}
