package llmtask

import (
	"context"
	"encoding/json"
	"testing"
)

// mockExecutor is a test double for the Executor interface.
type mockExecutor struct {
	response string
	err      error
}

func (m *mockExecutor) Complete(_ context.Context, _ string, _ bool) (string, error) {
	return m.response, m.err
}

// --------------------------------------------------------------------------
// extractJSON
// --------------------------------------------------------------------------

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string // empty string means no extraction expected
	}{
		{
			name:  "bare object",
			input: `{"key":"value"}`,
			want:  `{"key":"value"}`,
		},
		{
			name:  "object with surrounding prose",
			input: `Here is the result: {"name":"Alice","age":30} – that's it.`,
			want:  `{"name":"Alice","age":30}`,
		},
		{
			name:  "object inside markdown code fence",
			input: "```json\n{\"x\":1}\n```",
			want:  `{"x":1}`,
		},
		{
			name:  "bare array",
			input: `[1,2,3]`,
			want:  `[1,2,3]`,
		},
		{
			name:  "array with surrounding text",
			input: `Results: [{"id":1},{"id":2}] done.`,
			want:  `[{"id":1},{"id":2}]`,
		},
		{
			name:  "nested object",
			input: `prefix {"outer":{"inner":"v"}} suffix`,
			want:  `{"outer":{"inner":"v"}}`,
		},
		{
			name:  "string with braces inside value",
			input: `{"msg":"hello {world}"}`,
			want:  `{"msg":"hello {world}"}`,
		},
		{
			name:  "no JSON present",
			input: `no json here`,
			want:  "",
		},
		{
			name:  "unbalanced brace",
			input: `{"unclosed":true`,
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSON(tc.input)
			if got != tc.want {
				t.Errorf("extractJSON(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// validateAgainstSchema
// --------------------------------------------------------------------------

func TestValidateSchema(t *testing.T) {
	t.Run("valid object with string property", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
				"age":  map[string]interface{}{"type": "number"},
			},
		}
		data := json.RawMessage(`{"name":"Alice","age":30}`)
		if err := validateAgainstSchema(data, schema); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("wrong top-level type", func(t *testing.T) {
		schema := map[string]interface{}{"type": "object"}
		data := json.RawMessage(`["not","an","object"]`)
		if err := validateAgainstSchema(data, schema); err == nil {
			t.Error("expected error for array where object required")
		}
	})

	t.Run("wrong property type", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"count": map[string]interface{}{"type": "number"},
			},
		}
		data := json.RawMessage(`{"count":"not-a-number"}`)
		if err := validateAgainstSchema(data, schema); err == nil {
			t.Error("expected error for string where number required")
		}
	})

	t.Run("valid array type", func(t *testing.T) {
		schema := map[string]interface{}{"type": "array"}
		data := json.RawMessage(`[1,2,3]`)
		if err := validateAgainstSchema(data, schema); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("boolean property", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"active": map[string]interface{}{"type": "boolean"},
			},
		}
		data := json.RawMessage(`{"active":true}`)
		if err := validateAgainstSchema(data, schema); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("integer vs fractional number", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"count": map[string]interface{}{"type": "integer"},
			},
		}
		validData := json.RawMessage(`{"count":5}`)
		if err := validateAgainstSchema(validData, schema); err != nil {
			t.Errorf("unexpected error for integer value: %v", err)
		}
		invalidData := json.RawMessage(`{"count":5.5}`)
		if err := validateAgainstSchema(invalidData, schema); err == nil {
			t.Error("expected error for fractional number where integer required")
		}
	})
}

// --------------------------------------------------------------------------
// Required fields
// --------------------------------------------------------------------------

func TestRequiredFields(t *testing.T) {
	schema := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"id", "name"},
		"properties": map[string]interface{}{
			"id":   map[string]interface{}{"type": "number"},
			"name": map[string]interface{}{"type": "string"},
		},
	}

	t.Run("all required fields present", func(t *testing.T) {
		data := json.RawMessage(`{"id":1,"name":"Alice"}`)
		if err := validateAgainstSchema(data, schema); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing one required field", func(t *testing.T) {
		data := json.RawMessage(`{"id":1}`)
		if err := validateAgainstSchema(data, schema); err == nil {
			t.Error("expected error for missing required field \"name\"")
		}
	})

	t.Run("all required fields missing", func(t *testing.T) {
		data := json.RawMessage(`{}`)
		if err := validateAgainstSchema(data, schema); err == nil {
			t.Error("expected error for empty object missing required fields")
		}
	})
}

// --------------------------------------------------------------------------
// Run – integration-style tests using the mockExecutor
// --------------------------------------------------------------------------

func TestRun_ValidJSON(t *testing.T) {
	exec := &mockExecutor{response: `{"answer":42}`}
	result, err := Run(context.Background(), exec, TaskOptions{
		Prompt: "Give me a number",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, got error: %s", result.Error)
	}
	if string(result.Output) != `{"answer":42}` {
		t.Errorf("unexpected output: %s", result.Output)
	}
}

func TestRun_EmbeddedJSON(t *testing.T) {
	// Model wraps the JSON in prose – extractJSON should recover it.
	exec := &mockExecutor{response: `Sure! Here you go: {"name":"Bob"}`}
	result, err := Run(context.Background(), exec, TaskOptions{
		Prompt: "Give me a name",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, got error: %s", result.Error)
	}
}

func TestRun_SchemaValidationFailure(t *testing.T) {
	exec := &mockExecutor{response: `{"name":123}`}
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "string"},
		},
	}
	result, err := Run(context.Background(), exec, TaskOptions{
		Prompt: "name please",
		Schema: schema,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false when schema validation fails")
	}
	if result.Error == "" {
		t.Error("expected non-empty Error field on validation failure")
	}
}

func TestRun_EmptyPrompt(t *testing.T) {
	exec := &mockExecutor{response: `{}`}
	_, err := Run(context.Background(), exec, TaskOptions{Prompt: ""})
	if err == nil {
		t.Error("expected error for empty prompt")
	}
}

func TestRun_NonJSONResponse(t *testing.T) {
	exec := &mockExecutor{response: "I cannot answer that."}
	result, err := Run(context.Background(), exec, TaskOptions{Prompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for non-JSON response")
	}
}
