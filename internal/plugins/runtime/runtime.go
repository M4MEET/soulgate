package runtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/M4MEET/soulgate/internal/plugins/loader"
	"github.com/M4MEET/soulgate/internal/plugins/sdk"
)

// Runtime defines the interface for plugin execution
type Runtime interface {
	// LoadPlugin loads a plugin
	LoadPlugin(ctx context.Context, plugin *loader.Plugin) error

	// ExecuteTool executes a tool in a plugin
	ExecuteTool(ctx context.Context, pluginID, toolName string, input json.RawMessage) (json.RawMessage, error)

	// UnloadPlugin unloads a plugin
	UnloadPlugin(ctx context.Context, pluginID string) error

	// Close closes the runtime and releases resources
	Close(ctx context.Context) error
}

// ToolExecuteRequest represents a request to execute a tool
type ToolExecuteRequest struct {
	PluginID string
	ToolName string
	Input    json.RawMessage
}

// ToolExecuteResult represents the result of tool execution
type ToolExecuteResult struct {
	Output json.RawMessage
	Error  error
}

// convertToolDefToSchema converts a ToolDef to a model.ToolSchema
func ConvertToolDefToSchema(pluginID string, tool sdk.ToolDef) map[string]interface{} {
	return map[string]interface{}{
		"name":         fmt.Sprintf("%s.%s", pluginID, tool.Name),
		"description":  tool.Description,
		"input_schema": tool.InputSchema,
	}
}
