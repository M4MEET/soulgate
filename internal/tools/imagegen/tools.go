package imagegen

import (
	"context"
	"encoding/json"
	"fmt"
)

// Schema mirrors the voice.Schema type: a tool definition that keeps this
// package free of internal SoulGate dependencies so it can be registered by
// the orchestrator without a circular import.
type Schema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolSchemas returns the JSON tool schema definitions for image_generate and
// image_edit. The caller converts these into model.ToolSchema values and
// appends them to the list returned by getAllToolSchemas().
func ToolSchemas() []Schema {
	return []Schema{
		{
			Name:        "image_generate",
			Description: "Generate an image from a text prompt using DALL-E 3 (OpenAI) or FAL.ai (Flux/Stable Diffusion). Downloads the result and saves it to the workspace. Requires OPENAI_API_KEY (or FAL_KEY when provider is 'fal').",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"prompt": {
						"type": "string",
						"description": "Text description of the image to generate."
					},
					"size": {
						"type": "string",
						"description": "Output image dimensions. Defaults to '1024x1024'.",
						"enum": ["1024x1024", "1024x1792", "1792x1024"]
					},
					"output": {
						"type": "string",
						"description": "Workspace-relative output path for the PNG file, e.g. 'images/cat.png'. Defaults to 'generated-<timestamp>.png'."
					},
					"provider": {
						"type": "string",
						"description": "Image generation backend. 'openai' uses DALL-E 3, 'fal' uses FAL.ai (Flux/Stable Diffusion). Defaults to 'openai'.",
						"enum": ["openai", "fal"]
					},
					"fal_model": {
						"type": "string",
						"description": "FAL.ai model endpoint path, e.g. 'fal-ai/flux/schnell' or 'fal-ai/stable-diffusion-v3-medium'. Only used when provider is 'fal'. Defaults to 'fal-ai/flux/schnell'."
					}
				},
				"required": ["prompt"]
			}`),
		},
		{
			Name:        "image_edit",
			Description: "Edit an existing image in the workspace using a text prompt via OpenAI's image edit endpoint (DALL-E). The source image must be a PNG. Requires OPENAI_API_KEY.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Workspace-relative path to the source PNG image, e.g. 'images/photo.png'."
					},
					"prompt": {
						"type": "string",
						"description": "Text description of the edit to apply, e.g. 'remove the background' or 'add a sunset sky'."
					},
					"output": {
						"type": "string",
						"description": "Workspace-relative output path for the edited PNG. Defaults to '<original-name>-edited-<timestamp>.png'."
					},
					"size": {
						"type": "string",
						"description": "Output image dimensions. Defaults to '1024x1024'.",
						"enum": ["1024x1024", "1024x1792", "1792x1024"]
					}
				},
				"required": ["path", "prompt"]
			}`),
		},
	}
}

// ExecuteTool dispatches a named image tool call with the supplied arguments.
// workspaceRoot is the absolute path to the workspace root directory.
// apiKey is the primary API key from config (OpenAI key for openai/default;
// may be empty — environment variable fallback applies inside each function).
func ExecuteTool(ctx context.Context, workspaceRoot, apiKey, name string, rawArgs json.RawMessage) (string, error) {
	switch name {
	case "image_generate":
		return executeGenerate(ctx, workspaceRoot, apiKey, rawArgs)
	case "image_edit":
		return executeEdit(ctx, workspaceRoot, apiKey, rawArgs)
	default:
		return "", fmt.Errorf("imagegen: unknown tool %q", name)
	}
}

// executeGenerate parses the arguments for image_generate, delegates to
// Generate, and serialises the result as a JSON string.
func executeGenerate(ctx context.Context, workspaceRoot, apiKey string, rawArgs json.RawMessage) (string, error) {
	var opts GenerateOptions
	if err := json.Unmarshal(rawArgs, &opts); err != nil {
		return "", fmt.Errorf("image_generate: invalid arguments: %w", err)
	}

	result, err := Generate(ctx, workspaceRoot, apiKey, opts)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("image_generate: failed to serialise result: %w", err)
	}
	return string(out), nil
}

// executeEdit parses the arguments for image_edit, delegates to Edit, and
// serialises the result as a JSON string.
func executeEdit(ctx context.Context, workspaceRoot, apiKey string, rawArgs json.RawMessage) (string, error) {
	var opts EditOptions
	if err := json.Unmarshal(rawArgs, &opts); err != nil {
		return "", fmt.Errorf("image_edit: invalid arguments: %w", err)
	}

	result, err := Edit(ctx, workspaceRoot, apiKey, opts)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("image_edit: failed to serialise result: %w", err)
	}
	return string(out), nil
}
