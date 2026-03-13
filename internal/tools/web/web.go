// Package web provides web_search and web_fetch tools for the SoulGate
// orchestrator. Both tools are designed to be routed through the standard
// executeToolCall dispatch, exactly like the built-in file and network tools.
//
// Usage (orchestrator integration):
//
//	// In getToolSchemas():
//	for _, s := range web.ToolSchemas() {
//	    tools = append(tools, model.ToolSchema{
//	        Name:        s.Name,
//	        Description: s.Description,
//	        InputSchema: s.InputSchema,
//	    })
//	}
//
//	// In executeToolCall() default case (or explicit cases):
//	case "web_search", "web_fetch":
//	    return web.ExecuteTool(ctx, toolCall.Name, toolCall.Input)
package web

import (
	"context"
	"encoding/json"
	"fmt"
)

// Schema mirrors model.ToolSchema but without importing the model package,
// keeping this package free of internal dependencies and therefore trivially
// testable in isolation.
type Schema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolSchemas returns the JSON tool schema definitions for web_search and
// web_fetch. The caller is expected to convert these into model.ToolSchema
// values and append them to the list returned by getToolSchemas().
func ToolSchemas() []Schema {
	return []Schema{
		{
			Name:        "web_search",
			Description: "Search the web for information. Returns titles, URLs, and descriptions of matching results.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query"
					},
					"max_results": {
						"type": "integer",
						"description": "Maximum number of results to return (default 10)"
					},
					"freshness": {
						"type": "string",
						"enum": ["day", "week", "month"],
						"description": "Filter results by recency: 'day' (last 24h), 'week', or 'month'"
					},
					"country": {
						"type": "string",
						"description": "ISO 3166-1 alpha-2 country code to bias results (e.g. 'US', 'GB')"
					}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "web_fetch",
			Description: "Fetch a URL and return its readable text content. Strips HTML tags and extracts the main page text. Blocks access to private/internal network addresses.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {
						"type": "string",
						"description": "The URL to fetch (must be http or https)"
					},
					"max_chars": {
						"type": "integer",
						"description": "Maximum characters to return from the page content (default 50000)"
					},
					"raw": {
						"type": "boolean",
						"description": "Return raw HTML instead of extracted plain text (default false)"
					}
				},
				"required": ["url"]
			}`),
		},
	}
}

// ExecuteTool routes a tool call by name to the appropriate handler and returns
// the result serialised as a JSON string.
//
// This function is the single integration point: add the following case to the
// switch statement in core/tools.go executeToolCall:
//
//	case "web_search", "web_fetch":
//	    return web.ExecuteTool(ctx, toolCall.Name, toolCall.Input)
func ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, error) {
	switch name {
	case "web_search":
		return executeSearch(ctx, rawArgs)
	case "web_fetch":
		return executeFetch(ctx, rawArgs)
	default:
		return "", fmt.Errorf("web: unknown tool %q", name)
	}
}

// --------------------------------------------------------------------------
// Internal dispatch helpers
// --------------------------------------------------------------------------

func executeSearch(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var opts SearchOptions
	if err := json.Unmarshal(rawArgs, &opts); err != nil {
		return "", fmt.Errorf("web_search: invalid arguments: %w", err)
	}

	results, err := Search(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("web_search: %w", err)
	}

	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("web_search: failed to marshal results: %w", err)
	}

	return string(out), nil
}

func executeFetch(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var opts FetchOptions
	if err := json.Unmarshal(rawArgs, &opts); err != nil {
		return "", fmt.Errorf("web_fetch: invalid arguments: %w", err)
	}

	result, err := Fetch(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("web_fetch: %w", err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("web_fetch: failed to marshal result: %w", err)
	}

	return string(out), nil
}
