package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Integration represents a connection to an external service
type Integration interface {
	// Name returns the integration name (e.g., "github", "slack")
	Name() string

	// Description returns what this integration does
	Description() string

	// RequiredConfig returns the config fields needed (API keys, etc.)
	RequiredConfig() []ConfigField

	// Setup configures the integration with provided credentials
	Setup(ctx context.Context, config map[string]string) error

	// GetTools returns the tools this integration provides
	GetTools() []Tool

	// IsConfigured returns whether this integration is ready to use
	IsConfigured() bool

	// Close cleans up resources
	Close() error
}

// ConfigField describes a configuration parameter
type ConfigField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"` // Whether to hide value (API keys, passwords)
	Default     string `json:"default"`
	Example     string `json:"example"`
}

// Tool represents a capability provided by an integration
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	Handler     ToolHandler     `json:"-"`
}

// ToolHandler executes a tool call
type ToolHandler func(ctx context.Context, input json.RawMessage) (string, error)

// Registry manages available integrations
type Registry struct {
	mu           sync.RWMutex
	integrations map[string]Integration
}

// NewRegistry creates a new integration registry
func NewRegistry() *Registry {
	return &Registry{
		integrations: make(map[string]Integration),
	}
}

// Register adds an integration to the registry
func (r *Registry) Register(integration Integration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := integration.Name()
	if _, exists := r.integrations[name]; exists {
		return fmt.Errorf("integration %s already registered", name)
	}

	r.integrations[name] = integration
	return nil
}

// Get retrieves an integration by name
func (r *Registry) Get(name string) (Integration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	integration, exists := r.integrations[name]
	if !exists {
		return nil, fmt.Errorf("integration %s not found", name)
	}

	return integration, nil
}

// List returns all registered integrations
func (r *Registry) List() []Integration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Integration, 0, len(r.integrations))
	for _, integration := range r.integrations {
		list = append(list, integration)
	}

	return list
}

// GetConfiguredTools returns all tools from configured integrations
func (r *Registry) GetConfiguredTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []Tool
	for _, integration := range r.integrations {
		if integration.IsConfigured() {
			tools = append(tools, integration.GetTools()...)
		}
	}

	return tools
}

// Close closes all integrations
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, integration := range r.integrations {
		if err := integration.Close(); err != nil {
			return err
		}
	}

	return nil
}

// IntegrationInfo provides metadata about an integration
type IntegrationInfo struct {
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Configured     bool          `json:"configured"`
	RequiredConfig []ConfigField `json:"required_config"`
	ToolsCount     int           `json:"tools_count"`
}

// ListInfo returns information about all integrations
func (r *Registry) ListInfo() []IntegrationInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := make([]IntegrationInfo, 0, len(r.integrations))
	for _, integration := range r.integrations {
		tools := integration.GetTools()
		info = append(info, IntegrationInfo{
			Name:           integration.Name(),
			Description:    integration.Description(),
			Configured:     integration.IsConfigured(),
			RequiredConfig: integration.RequiredConfig(),
			ToolsCount:     len(tools),
		})
	}

	return info
}
