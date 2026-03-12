package sdk

import (
	"context"

	"github.com/M4MEET/soulgate/internal/brokers/messaging"
)

// Plugin is the interface that all SoulGate plugins must implement
type Plugin interface {
	// Initialize sets up the plugin with configuration
	Initialize(ctx context.Context, config map[string]interface{}) error

	// Shutdown cleans up plugin resources
	Shutdown(ctx context.Context) error
}

// ChannelPlugin extends Plugin for messaging channel providers
type ChannelPlugin interface {
	Plugin

	// GetChannel returns the messaging channel provided by this plugin
	GetChannel() messaging.Channel
}

// CommandPlugin extends Plugin for plugins that provide CLI commands
type CommandPlugin interface {
	Plugin

	// ExecuteCommand handles plugin-specific commands
	ExecuteCommand(ctx context.Context, cmd string, args []string) (string, error)
}

// ToolPlugin extends Plugin for plugins that provide tools to the AI
type ToolPlugin interface {
	Plugin

	// GetTools returns the tool schemas provided by this plugin
	GetTools() []ToolSchema

	// ExecuteTool executes a tool call
	ExecuteTool(ctx context.Context, toolName string, input map[string]interface{}) (interface{}, error)
}

// ToolSchema defines a tool's interface
type ToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// RegisterPlugin registers a plugin instance (called by plugin's main())
func RegisterPlugin(plugin Plugin) {
	// This will be implemented by the plugin loader
	// For now, this is a placeholder for the plugin registration mechanism
}
