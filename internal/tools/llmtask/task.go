// Package llmtask provides a tool for running focused LLM completions that
// return structured JSON output. It is intentionally free of internal SoulGate
// dependencies so that it can be imported by any layer without creating import
// cycles.
package llmtask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// TaskOptions configures a single LLM task call.
type TaskOptions struct {
	Prompt      string                 `json:"prompt"`                // The prompt/question
	Schema      map[string]interface{} `json:"schema,omitempty"`      // JSON Schema for output validation
	Model       string                 `json:"model,omitempty"`       // Override model (passed through to Executor if supported)
	MaxTokens   int                    `json:"max_tokens,omitempty"`  // Max response tokens (default 4096)
	Temperature float64                `json:"temperature,omitempty"` // Sampling temperature (default 0)
}

// TaskResult holds the outcome of a single LLM task call.
type TaskResult struct {
	Output json.RawMessage `json:"output"`           // The structured JSON output
	Valid  bool            `json:"valid"`            // Whether output matched schema
	Error  string          `json:"error,omitempty"`  // Validation error if any
	Model  string          `json:"model"`            // Model that was used
	Tokens int             `json:"tokens,omitempty"` // Tokens used (if reported by Executor)
}

// Executor is the minimal interface required to run an LLM completion.
// The jsonMode flag signals that the caller wants raw JSON back; supporting
// providers may enable their native JSON-mode feature in response.
type Executor interface {
	Complete(ctx context.Context, prompt string, jsonMode bool) (string, error)
}

// Run executes an LLM task and validates the output against the optional schema.
// It returns a non-nil *TaskResult on success even when schema validation fails;
// the caller should inspect TaskResult.Valid and TaskResult.Error in that case.
func Run(ctx context.Context, exec Executor, opts TaskOptions) (*TaskResult, error) {
	if opts.Prompt == "" {
		return nil, fmt.Errorf("llmtask: prompt is required")
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 4096
	}

	// Build a system prompt that constrains the model to JSON-only output.
	systemPrompt := "You are a structured data extraction assistant. " +
		"You MUST respond with valid JSON only. " +
		"No markdown, no explanation, no code fences. Just the JSON object."

	if opts.Schema != nil {
		schemaJSON, err := json.MarshalIndent(opts.Schema, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("llmtask: failed to marshal schema: %w", err)
		}
		systemPrompt += fmt.Sprintf(
			"\n\nYour response MUST conform to this JSON Schema:\n%s",
			string(schemaJSON),
		)
	}

	fullPrompt := systemPrompt + "\n\nUser request: " + opts.Prompt

	// Call the underlying model.
	response, err := exec.Complete(ctx, fullPrompt, true)
	if err != nil {
		return nil, fmt.Errorf("llmtask: LLM call failed: %w", err)
	}

	// Attempt direct JSON parse first.
	var parsed json.RawMessage
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		// Model may have wrapped the JSON in prose or code fences; try extraction.
		extracted := extractJSON(response)
		if extracted == "" {
			return &TaskResult{
				Output: json.RawMessage(`null`),
				Valid:  false,
				Error:  fmt.Sprintf("response is not valid JSON: %s", err.Error()),
			}, nil
		}
		if err := json.Unmarshal([]byte(extracted), &parsed); err != nil {
			return &TaskResult{
				Output: json.RawMessage(`null`),
				Valid:  false,
				Error:  "could not extract valid JSON from response",
			}, nil
		}
	}

	// Optionally validate against the schema.
	valid := true
	validErr := ""
	if opts.Schema != nil {
		if err := validateAgainstSchema(parsed, opts.Schema); err != nil {
			valid = false
			validErr = err.Error()
		}
	}

	return &TaskResult{
		Output: parsed,
		Valid:  valid,
		Error:  validErr,
	}, nil
}

// extractJSON scans s for the first balanced { ... } or [ ... ] block and
// returns it. It picks whichever opening character appears first in s so that
// a top-level array like `[{"id":1}]` is not mistakenly reduced to `{"id":1}`.
// Returns an empty string when no balanced block is found.
func extractJSON(s string) string {
	// Determine which opening bracket appears first.
	objStart := strings.IndexByte(s, '{')
	arrStart := strings.IndexByte(s, '[')

	// Build the candidate list ordered by position.
	type pair struct{ open, close byte }
	candidates := []pair{}
	switch {
	case objStart == -1 && arrStart == -1:
		return ""
	case objStart == -1:
		candidates = []pair{{'[', ']'}}
	case arrStart == -1:
		candidates = []pair{{'{', '}'}}
	case arrStart < objStart:
		candidates = []pair{{'[', ']'}, {'{', '}'}}
	default:
		candidates = []pair{{'{', '}'}, {'[', ']'}}
	}

	for _, p := range candidates {
		start := strings.IndexByte(s, p.open)
		if start == -1 {
			continue
		}

		depth := 0
		inString := false
		escaped := false

		for i := start; i < len(s); i++ {
			ch := s[i]

			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && inString {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}

			if ch == p.open {
				depth++
			} else if ch == p.close {
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	return ""
}

// validateAgainstSchema performs a best-effort validation of data against a
// simple JSON Schema subset:
//   - "type"       – object | array | string | number | boolean | integer
//   - "required"   – list of property names that must exist (objects only)
//   - "properties" – per-property type constraints (objects only)
func validateAgainstSchema(data json.RawMessage, schema map[string]interface{}) error {
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("invalid JSON data: %w", err)
	}

	return validateValue(value, schema, "")
}

// validateValue recursively validates a decoded value against a schema node.
// path is used to build meaningful error messages (e.g. "$.name").
func validateValue(value interface{}, schema map[string]interface{}, path string) error {
	prefix := "$"
	if path != "" {
		prefix = path
	}

	// --- type check ---
	if rawType, ok := schema["type"]; ok {
		typeName, ok := rawType.(string)
		if !ok {
			return fmt.Errorf("%s: schema \"type\" must be a string", prefix)
		}
		if err := checkType(value, typeName, prefix); err != nil {
			return err
		}
	}

	// Further object-specific validation.
	obj, isObj := value.(map[string]interface{})

	// --- required fields ---
	if rawRequired, ok := schema["required"]; ok {
		reqSlice, ok := toStringSlice(rawRequired)
		if !ok {
			return fmt.Errorf("%s: schema \"required\" must be an array of strings", prefix)
		}
		if !isObj {
			return fmt.Errorf("%s: \"required\" constraint applies only to objects", prefix)
		}
		for _, field := range reqSlice {
			if _, exists := obj[field]; !exists {
				return fmt.Errorf("%s: missing required field %q", prefix, field)
			}
		}
	}

	// --- properties ---
	if rawProps, ok := schema["properties"]; ok {
		props, ok := rawProps.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: schema \"properties\" must be an object", prefix)
		}
		if !isObj {
			return fmt.Errorf("%s: \"properties\" constraint applies only to objects", prefix)
		}
		for propName, propSchemaRaw := range props {
			propVal, exists := obj[propName]
			if !exists {
				// Property is optional unless listed in "required"; already checked above.
				continue
			}
			propSchema, ok := propSchemaRaw.(map[string]interface{})
			if !ok {
				continue // Non-object schema node; skip.
			}
			childPath := prefix + "." + propName
			if err := validateValue(propVal, propSchema, childPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkType asserts that value matches the JSON Schema typeName.
func checkType(value interface{}, typeName string, path string) error {
	switch typeName {
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("%s: expected object, got %T", path, value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("%s: expected array, got %T", path, value)
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string, got %T", path, value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("%s: expected number, got %T", path, value)
		}
	case "integer":
		f, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s: expected integer, got %T", path, value)
		}
		if f != float64(int64(f)) {
			return fmt.Errorf("%s: expected integer, got fractional number %v", path, f)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean, got %T", path, value)
		}
	case "null":
		if value != nil {
			return fmt.Errorf("%s: expected null, got %T", path, value)
		}
	default:
		// Unknown type keyword; skip silently to stay forward-compatible.
	}
	return nil
}

// toStringSlice coerces a raw schema value (typically []interface{}) into a
// []string. Returns (nil, false) when the conversion is not possible.
func toStringSlice(raw interface{}) ([]string, bool) {
	slice, ok := raw.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	return result, true
}
