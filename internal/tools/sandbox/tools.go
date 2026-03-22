package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Schema mirrors model.ToolSchema without importing the model package,
// keeping this package dependency-free and trivially testable.
type Schema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolSchemas returns the JSON schema definitions for code_run and code_install.
// The caller converts these into model.ToolSchema values and appends them to
// the list returned by getAllToolSchemas().
func ToolSchemas() []Schema {
	supportedLangs := `["python","node","go","bash","ruby"]`

	return []Schema{
		{
			Name: "code_run",
			Description: "Execute a code snippet in an isolated temporary environment. " +
				"Each invocation runs in a fresh directory that is deleted after execution. " +
				"Captures stdout and stderr separately. " +
				"Supported languages: python, node, go, bash, ruby.",
			InputSchema: json.RawMessage(fmt.Sprintf(`{
				"type": "object",
				"properties": {
					"language": {
						"type": "string",
						"description": "Programming language to use.",
						"enum": %s
					},
					"code": {
						"type": "string",
						"description": "Source code to execute."
					},
					"timeout": {
						"type": "integer",
						"description": "Maximum execution time in seconds (1–60). Defaults to 10."
					}
				},
				"required": ["language", "code"]
			}`, supportedLangs)),
		},
		{
			Name: "code_install",
			Description: "Install a package or dependency for subsequent code_run calls. " +
				"Uses the language's native package manager: pip3 (python), npm (node), " +
				"go get (go), gem (ruby). Not supported for bash.",
			InputSchema: json.RawMessage(fmt.Sprintf(`{
				"type": "object",
				"properties": {
					"language": {
						"type": "string",
						"description": "Language whose package manager to use.",
						"enum": %s
					},
					"package": {
						"type": "string",
						"description": "Package name to install (e.g. \"requests\", \"lodash\", \"golang.org/x/text@latest\")."
					}
				},
				"required": ["language", "package"]
			}`, supportedLangs)),
		},
	}
}

// ExecuteTool dispatches a tool call by name and returns a JSON string result.
// It is the single integration point called from the orchestrator's
// executeToolCall switch.
func ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, error) {
	switch name {
	case "code_run":
		return executeRun(ctx, rawArgs)
	case "code_install":
		return executeInstall(ctx, rawArgs)
	default:
		return "", fmt.Errorf("sandbox: unknown tool %q", name)
	}
}

// ---------------------------------------------------------------------------
// Internal dispatch
// ---------------------------------------------------------------------------

func executeRun(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var params struct {
		Language string `json:"language"`
		Code     string `json:"code"`
		Timeout  int    `json:"timeout"`
	}
	if err := json.Unmarshal(rawArgs, &params); err != nil {
		return "", fmt.Errorf("code_run: invalid arguments: %w", err)
	}

	params.Language = strings.TrimSpace(params.Language)
	if params.Language == "" {
		return "", fmt.Errorf("code_run: \"language\" is required")
	}
	if strings.TrimSpace(params.Code) == "" {
		return "", fmt.Errorf("code_run: \"code\" is required")
	}

	timeout := time.Duration(params.Timeout) * time.Second // clampTimeout handles 0

	result, err := Execute(ctx, params.Language, params.Code, timeout)
	if err != nil {
		return "", fmt.Errorf("code_run: %w", err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("code_run: failed to marshal result: %w", err)
	}
	return string(out), nil
}

func executeInstall(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var params struct {
		Language string `json:"language"`
		Package  string `json:"package"`
	}
	if err := json.Unmarshal(rawArgs, &params); err != nil {
		return "", fmt.Errorf("code_install: invalid arguments: %w", err)
	}

	params.Language = strings.TrimSpace(params.Language)
	params.Package = strings.TrimSpace(params.Package)

	if params.Language == "" {
		return "", fmt.Errorf("code_install: \"language\" is required")
	}
	if params.Package == "" {
		return "", fmt.Errorf("code_install: \"package\" is required")
	}

	result, err := Install(ctx, params.Language, params.Package)
	if err != nil {
		return "", fmt.Errorf("code_install: %w", err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("code_install: failed to marshal result: %w", err)
	}
	return string(out), nil
}
