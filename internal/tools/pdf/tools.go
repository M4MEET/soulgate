package pdf

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolSchemas returns the JSON tool schemas for the PDF tool set.
// The schema follows the Anthropic/OpenAI tool-calling convention used
// throughout the SoulGate model layer (see internal/model/schema.go).
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "pdf_read",
			"description": "Read and extract text from a PDF document. Supports local file paths and HTTP/HTTPS URLs. Optionally filters to specific page ranges.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Local file path or HTTP/HTTPS URL pointing to the PDF.",
					},
					"pages": map[string]interface{}{
						"type":        "string",
						"description": "Optional page range to extract, e.g. '1-5', '1,3,7-9'. Omit to extract all pages (subject to max_pages).",
					},
					"max_pages": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of pages to process. Defaults to 20.",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// ExecuteTool dispatches a named PDF tool call with the supplied arguments.
// It is the primary integration point between the SoulGate orchestrator and
// this tool package.
func ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "pdf_read":
		return executePDFRead(ctx, args)
	default:
		return "", fmt.Errorf("pdf: unknown tool %q", name)
	}
}

// executePDFRead parses the arguments for the pdf_read tool, delegates to
// Analyze, and serialises the result as a JSON string.
func executePDFRead(ctx context.Context, args map[string]interface{}) (string, error) {
	var opts PDFOptions

	// path — required
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("pdf_read: 'path' argument is required and must be a non-empty string")
	}
	opts.Path = path

	// pages — optional string
	if pages, ok := args["pages"].(string); ok {
		opts.Pages = pages
	}

	// max_pages — optional integer; JSON numbers unmarshal as float64
	if mp, ok := args["max_pages"]; ok {
		switch v := mp.(type) {
		case float64:
			opts.MaxPages = int(v)
		case int:
			opts.MaxPages = v
		case int64:
			opts.MaxPages = int(v)
		}
	}

	result, err := Analyze(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("pdf_read: %w", err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("pdf_read: failed to serialise result: %w", err)
	}

	return string(out), nil
}
