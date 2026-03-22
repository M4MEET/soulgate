package computer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/model"
)

// ToolSchemas returns the model.ToolSchema definitions for all computer tools.
func ToolSchemas() []model.ToolSchema {
	return []model.ToolSchema{
		{
			Name:        "computer_screenshot",
			Description: "Take a screenshot of the current screen and return the file path. Use this when you need to see what is currently displayed on screen.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "computer_click",
			Description: "Click the mouse at the specified x,y screen coordinates.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"x": {"type": "integer", "description": "Horizontal pixel coordinate (0 = left edge)"},
					"y": {"type": "integer", "description": "Vertical pixel coordinate (0 = top edge)"}
				},
				"required": ["x", "y"]
			}`),
		},
		{
			Name:        "computer_type",
			Description: "Type text at the current cursor position using keystrokes.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {"type": "string", "description": "Text to type"}
				},
				"required": ["text"]
			}`),
		},
		{
			Name:        "computer_move",
			Description: "Move the mouse pointer to the specified x,y screen coordinates without clicking.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"x": {"type": "integer", "description": "Horizontal pixel coordinate"},
					"y": {"type": "integer", "description": "Vertical pixel coordinate"}
				},
				"required": ["x", "y"]
			}`),
		},
		{
			Name:        "computer_look",
			Description: "Take a screenshot and use vision AI to describe what is on screen, including interactive elements and their approximate x,y coordinates. Returns a detailed description from the model.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"question": {
						"type": "string",
						"description": "Optional specific question about what to look for. If omitted, the model gives a general screen description."
					}
				}
			}`),
		},
	}
}

// Looker is implemented by the orchestrator and provides vision model access.
// The orchestrator injects a closure at wire-up time; it is nil when no
// vision-capable provider is configured.
type Looker interface {
	// Describe sends a base64-encoded image and a question to the current model
	// and returns the model's text response.
	Describe(ctx context.Context, imageBase64, mimeType, question string) (string, error)
}

// ExecuteTool dispatches a computer tool call and returns the result string.
// looker may be nil; computer_look returns an error when it is.
func ExecuteTool(ctx context.Context, looker Looker, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "computer_screenshot":
		return execScreenshot()
	case "computer_click":
		return execClick(args)
	case "computer_type":
		return execType(args)
	case "computer_move":
		return execMove(args)
	case "computer_look":
		return execLook(ctx, looker, args)
	default:
		return "", fmt.Errorf("unknown computer tool: %s", name)
	}
}

// execScreenshot takes a screenshot and returns the file path as JSON.
func execScreenshot() (string, error) {
	path, err := Screenshot()
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]string{"path": path})
	return string(out), nil
}

// execClick performs a mouse click and returns status JSON.
func execClick(args map[string]interface{}) (string, error) {
	x, y, err := extractCoords(args)
	if err != nil {
		return "", err
	}
	if err := Click(x, y); err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "clicked", "x": x, "y": y})
	return string(out), nil
}

// execType sends keystrokes for the provided text.
func execType(args map[string]interface{}) (string, error) {
	text, ok := args["text"].(string)
	if !ok || text == "" {
		return "", fmt.Errorf("computer_type: 'text' argument is required")
	}
	if err := Type(text); err != nil {
		return "", err
	}
	return `{"status":"typed"}`, nil
}

// execMove moves the mouse pointer.
func execMove(args map[string]interface{}) (string, error) {
	x, y, err := extractCoords(args)
	if err != nil {
		return "", err
	}
	if err := MoveMouse(x, y); err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]interface{}{"status": "moved", "x": x, "y": y})
	return string(out), nil
}

// execLook takes a screenshot and asks the vision model to describe the screen.
func execLook(ctx context.Context, looker Looker, args map[string]interface{}) (string, error) {
	if looker == nil {
		return "", fmt.Errorf("computer_look: vision model not available (set a vision-capable provider)")
	}

	// Capture the screen.
	imgPath, err := Screenshot()
	if err != nil {
		return "", fmt.Errorf("computer_look: screenshot failed: %w", err)
	}

	// Read and base64-encode the PNG.
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return "", fmt.Errorf("computer_look: read screenshot: %w", err)
	}
	imgBase64 := base64.StdEncoding.EncodeToString(data)

	// Build the question; default to a general description with coordinates.
	question := "Describe what you see on the screen in detail. " +
		"List all visible UI elements — windows, buttons, text fields, menus, icons, and any text content — " +
		"with their approximate x,y pixel coordinates."
	if q, ok := args["question"].(string); ok && q != "" {
		question = q + " Also note the approximate x,y coordinates of any relevant elements."
	}

	// Call the vision model.
	description, err := looker.Describe(ctx, imgBase64, "image/png", question)
	if err != nil {
		return "", fmt.Errorf("computer_look: model call failed: %w", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"screenshot":  imgPath,
		"description": description,
	})
	return string(out), nil
}

// extractCoords extracts integer x,y coordinates from a tool argument map.
func extractCoords(args map[string]interface{}) (x, y int, err error) {
	xRaw, yRaw := args["x"], args["y"]
	if xRaw == nil || yRaw == nil {
		return 0, 0, fmt.Errorf("x and y coordinates are required")
	}
	xF, xOk := toFloat64(xRaw)
	yF, yOk := toFloat64(yRaw)
	if !xOk || !yOk {
		return 0, 0, fmt.Errorf("x and y must be numbers")
	}
	return int(xF), int(yF), nil
}

// toFloat64 normalises the numeric types that come back from JSON
// unmarshalling (json.Number, float64, int, etc.) to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
