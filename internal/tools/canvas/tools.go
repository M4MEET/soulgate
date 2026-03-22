package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Schema is a tool definition compatible with the voice.Schema pattern used
// across SoulGate tool packages — no dependency on internal/model.
type Schema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolSchemas returns the tool definitions for all canvas tools.
func ToolSchemas() []Schema {
	return []Schema{
		{
			Name:        "canvas_create",
			Description: "Create an interactive canvas artifact (HTML page, React app, SVG graphic, or Mermaid diagram) that can be previewed in a browser. Saved to .soulgate/canvas/. Returns the artifact ID and a preview URL.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title": {
						"type": "string",
						"description": "Human-readable title for the artifact (e.g. 'Data Dashboard')."
					},
					"type": {
						"type": "string",
						"description": "Artifact type.",
						"enum": ["html", "react", "svg", "mermaid"]
					},
					"content": {
						"type": "string",
						"description": "Artifact source. For 'html': a full HTML document. For 'react': JSX/JS code (rendered inside a React + Babel CDN harness). For 'svg': an <svg> element. For 'mermaid': a Mermaid diagram definition."
					},
					"description": {
						"type": "string",
						"description": "Optional short description of what the artifact shows."
					}
				},
				"required": ["title", "type", "content"]
			}`),
		},
		{
			Name:        "canvas_update",
			Description: "Update an existing canvas artifact's content. Re-renders the file and returns an updated preview URL.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id": {
						"type": "string",
						"description": "Artifact ID returned by canvas_create."
					},
					"content": {
						"type": "string",
						"description": "New artifact source (same format as canvas_create)."
					}
				},
				"required": ["id", "content"]
			}`),
		},
		{
			Name:        "canvas_list",
			Description: "List all canvas artifacts, including their IDs, titles, types, and creation timestamps.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "canvas_preview",
			Description: "Start a temporary HTTP preview server for a canvas artifact and return its localhost URL. The server shuts down automatically after 30 minutes.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id": {
						"type": "string",
						"description": "Artifact ID to preview."
					}
				},
				"required": ["id"]
			}`),
		},
	}
}

// ExecuteTool dispatches a canvas tool call to the appropriate handler.
// mgr is the shared Manager; pm is the shared PreviewManager.
func ExecuteTool(
	ctx context.Context,
	mgr *Manager,
	pm *PreviewManager,
	toolName string,
	args map[string]interface{},
) (string, error) {
	switch toolName {
	case "canvas_create":
		return executeCreate(mgr, pm, args)
	case "canvas_update":
		return executeUpdate(mgr, pm, args)
	case "canvas_list":
		return executeList(mgr)
	case "canvas_preview":
		return executePreview(mgr, pm, args)
	default:
		return "", fmt.Errorf("canvas: unknown tool %q", toolName)
	}
}

// ---------------------------------------------------------------------------
// Individual handlers
// ---------------------------------------------------------------------------

func executeCreate(mgr *Manager, pm *PreviewManager, args map[string]interface{}) (string, error) {
	title, _ := stringArg(args, "title")
	typStr, _ := stringArg(args, "type")
	content, _ := stringArg(args, "content")
	description, _ := stringArg(args, "description")

	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("canvas_create: 'title' is required")
	}
	if strings.TrimSpace(typStr) == "" {
		return "", fmt.Errorf("canvas_create: 'type' is required")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("canvas_create: 'content' is required")
	}

	a, err := mgr.Create(title, ArtifactType(typStr), content, description)
	if err != nil {
		return "", fmt.Errorf("canvas_create: %w", err)
	}

	// Start a preview server immediately so the user gets a URL right away.
	url, previewErr := pm.StartPreview(a.ID, a.FilePath)
	if previewErr != nil {
		url = "preview unavailable"
	}

	result, _ := json.Marshal(map[string]interface{}{
		"id":          a.ID,
		"title":       a.Title,
		"type":        string(a.Type),
		"description": a.Description,
		"file_path":   a.FilePath,
		"preview_url": url,
		"created_at":  a.CreatedAt.Format(time.RFC3339),
	})
	return string(result), nil
}

func executeUpdate(mgr *Manager, pm *PreviewManager, args map[string]interface{}) (string, error) {
	id, _ := stringArg(args, "id")
	content, _ := stringArg(args, "content")

	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("canvas_update: 'id' is required")
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("canvas_update: 'content' is required")
	}

	a, err := mgr.Update(id, content)
	if err != nil {
		return "", fmt.Errorf("canvas_update: %w", err)
	}

	url, previewErr := pm.StartPreview(a.ID, a.FilePath)
	if previewErr != nil {
		url = "preview unavailable"
	}

	result, _ := json.Marshal(map[string]interface{}{
		"id":          a.ID,
		"title":       a.Title,
		"type":        string(a.Type),
		"file_path":   a.FilePath,
		"preview_url": url,
		"updated_at":  a.UpdatedAt.Format(time.RFC3339),
	})
	return string(result), nil
}

func executeList(mgr *Manager) (string, error) {
	artifacts := mgr.List()

	type row struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Type        string `json:"type"`
		Description string `json:"description,omitempty"`
		FilePath    string `json:"file_path"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	rows := make([]row, 0, len(artifacts))
	for _, a := range artifacts {
		rows = append(rows, row{
			ID:          a.ID,
			Title:       a.Title,
			Type:        string(a.Type),
			Description: a.Description,
			FilePath:    a.FilePath,
			CreatedAt:   a.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
		})
	}

	result, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", fmt.Errorf("canvas_list: failed to serialise result: %w", err)
	}
	return string(result), nil
}

func executePreview(mgr *Manager, pm *PreviewManager, args map[string]interface{}) (string, error) {
	id, _ := stringArg(args, "id")
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("canvas_preview: 'id' is required")
	}

	a, err := mgr.Get(id)
	if err != nil {
		return "", fmt.Errorf("canvas_preview: %w", err)
	}

	url, err := pm.StartPreview(a.ID, a.FilePath)
	if err != nil {
		return "", fmt.Errorf("canvas_preview: %w", err)
	}

	result, _ := json.Marshal(map[string]string{
		"id":          a.ID,
		"title":       a.Title,
		"preview_url": url,
		"message":     "Preview server started. Open the URL in a browser to view the artifact. It will shut down automatically after 30 minutes.",
	})
	return string(result), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func stringArg(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
