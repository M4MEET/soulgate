package git

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolSchemas returns the JSON tool schemas that expose git operations to the
// model. The schema format mirrors the rest of the SoulGate tool catalogue.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "git_status",
			"description": "Show the working tree status (modified, staged, untracked files). Equivalent to `git status --short`.",
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "git_diff",
			"description": "Show changes in the working tree or staging area. Set staged=true to see changes that have already been `git add`-ed.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"staged": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, show staged (index) diff instead of unstaged diff.",
					},
				},
			},
		},
		{
			"name":        "git_log",
			"description": "Show recent commit history as one-line summaries.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"n": map[string]interface{}{
						"type":        "number",
						"description": "Number of commits to show. Defaults to 20.",
					},
				},
			},
		},
		{
			"name":        "git_commit",
			"description": "Stage files and create a git commit. When files is empty, all changes are staged (equivalent to `git add --all`).",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Commit message (required).",
					},
					"files": map[string]interface{}{
						"type":        "array",
						"description": "List of file paths to stage. Leave empty to stage all changes.",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"message"},
			},
		},
		{
			"name":        "git_branch",
			"description": "List, create, or switch git branches.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Operation: list, create, or switch.",
						"enum":        []string{"list", "create", "switch"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Branch name (required for create and switch).",
					},
					"remote": map[string]interface{}{
						"type":        "boolean",
						"description": "When action=list, show remote branches instead of local ones.",
					},
				},
				"required": []string{"action"},
			},
		},
		{
			"name":        "git_stash",
			"description": "Save or restore stashed changes. Use save to stash, pop to restore the most recent stash, and list to see all stash entries.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Operation: save, pop, or list.",
						"enum":        []string{"save", "pop", "list"},
					},
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Optional description attached to the stash entry (only used with save).",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

// ExecuteTool dispatches a tool call by name and returns a string result.
// workDir is the workspace root; all git commands run there.
func ExecuteTool(ctx context.Context, workDir, name string, rawInput json.RawMessage) (string, error) {
	switch name {
	case "git_status":
		return Status(ctx, workDir)

	case "git_diff":
		var p struct {
			Staged bool `json:"staged"`
		}
		if err := json.Unmarshal(rawInput, &p); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return Diff(ctx, workDir, p.Staged)

	case "git_log":
		var p struct {
			N int `json:"n"`
		}
		if err := json.Unmarshal(rawInput, &p); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return Log(ctx, workDir, p.N)

	case "git_commit":
		var p struct {
			Message string   `json:"message"`
			Files   []string `json:"files"`
		}
		if err := json.Unmarshal(rawInput, &p); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return Commit(ctx, workDir, p.Files, p.Message)

	case "git_branch":
		var p struct {
			Action string `json:"action"`
			Name   string `json:"name"`
			Remote bool   `json:"remote"`
		}
		if err := json.Unmarshal(rawInput, &p); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return Branch(ctx, workDir, p.Action, p.Name, p.Remote)

	case "git_stash":
		var p struct {
			Action  string `json:"action"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rawInput, &p); err != nil {
			return "", fmt.Errorf("invalid tool input: %w", err)
		}
		return Stash(ctx, workDir, p.Action, p.Message)

	default:
		return "", fmt.Errorf("git: unknown tool %q", name)
	}
}
