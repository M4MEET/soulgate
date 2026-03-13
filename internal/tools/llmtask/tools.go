package llmtask

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolSchemas returns the JSON tool schema definitions for the llm_task tool.
// The returned slice uses the same map structure expected by the SoulGate
// orchestrator's tool catalogue so it can be appended without conversion.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name": "llm_task",
			"description": "Run a focused LLM task that returns structured JSON output. " +
				"Use this for data extraction, classification, or any task requiring " +
				"structured output. No tools are available to the model during this task.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "The task prompt",
					},
					"schema": map[string]interface{}{
						"type":        "object",
						"description": "JSON Schema the output must conform to",
					},
					"max_tokens": map[string]interface{}{
						"type":        "integer",
						"description": "Max response tokens (default 4096)",
					},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

// ExecuteTool dispatches the llm_task tool call to Run and serialises the
// result as a JSON string. exec is the Executor that will perform the
// underlying model call.
func ExecuteTool(ctx context.Context, exec Executor, name string, args map[string]interface{}) (string, error) {
	if name != "llm_task" {
		return "", fmt.Errorf("llmtask: unknown tool %q", name)
	}

	opts, err := argsToOptions(args)
	if err != nil {
		return "", fmt.Errorf("llmtask: invalid arguments: %w", err)
	}

	result, err := Run(ctx, exec, opts)
	if err != nil {
		return "", err
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("llmtask: failed to marshal result: %w", err)
	}
	return string(out), nil
}

// argsToOptions converts a raw args map (as received from the model) into a
// TaskOptions struct, performing light type assertions along the way.
func argsToOptions(args map[string]interface{}) (TaskOptions, error) {
	var opts TaskOptions

	prompt, ok := args["prompt"].(string)
	if !ok || prompt == "" {
		return opts, fmt.Errorf("missing required argument \"prompt\"")
	}
	opts.Prompt = prompt

	if v, ok := args["schema"]; ok {
		schema, ok := v.(map[string]interface{})
		if !ok {
			return opts, fmt.Errorf("\"schema\" must be an object")
		}
		opts.Schema = schema
	}

	if v, ok := args["max_tokens"]; ok {
		switch n := v.(type) {
		case float64:
			opts.MaxTokens = int(n)
		case int:
			opts.MaxTokens = n
		}
	}

	return opts, nil
}
