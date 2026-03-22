package email

import (
	"context"
	"encoding/json"
	"fmt"
)

// Schema mirrors model.ToolSchema without importing the model package,
// keeping this package free of internal dependencies and trivially testable.
type Schema struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// ToolSchemas returns the tool schema definitions for all email tools.
// The caller is expected to convert these into model.ToolSchema values and
// append them to the list returned by core/tools_schemas.go:getAllToolSchemas.
func ToolSchemas() []Schema {
	return []Schema{
		{
			Name:        "email_send",
			Description: "Send an email via SMTP. SMTP credentials are read from environment variables (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"to": {
						"type": "string",
						"description": "Recipient email address (e.g. user@example.com)"
					},
					"subject": {
						"type": "string",
						"description": "Email subject line"
					},
					"body": {
						"type": "string",
						"description": "Plain-text email body"
					}
				},
				"required": ["to", "subject", "body"]
			}`),
		},
	}
}

// ExecuteTool routes the tool call to the appropriate handler and returns the
// result as a plain string. This is the single integration point: add the
// following case to the switch in core/tools_dispatch.go:
//
//	case "email_send":
//	    return email.ExecuteTool(ctx, toolCall.Name, toolCall.Input)
func ExecuteTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, error) {
	switch name {
	case "email_send":
		return executeSend(ctx, rawArgs)
	default:
		return "", fmt.Errorf("email: unknown tool %q", name)
	}
}

func executeSend(ctx context.Context, rawArgs json.RawMessage) (string, error) {
	var params SendParams
	if err := json.Unmarshal(rawArgs, &params); err != nil {
		return "", fmt.Errorf("email_send: invalid arguments: %w", err)
	}

	if err := Send(ctx, params); err != nil {
		return "", err
	}

	return fmt.Sprintf("Email sent successfully to %s", params.To), nil
}
