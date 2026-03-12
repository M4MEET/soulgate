package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentsConfig holds multiple agent configurations
type AgentsConfig struct {
	Version string              `yaml:"version"`
	Agents  []AgentDefinition   `yaml:"agents"`
	Routing RoutingConfig       `yaml:"routing,omitempty"`
}

// AgentDefinition defines a single agent
type AgentDefinition struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Enabled     bool           `yaml:"enabled"`
	Model       ModelConfig    `yaml:"model"`
	Tools       []string       `yaml:"tools,omitempty"`
	Skills      []string       `yaml:"skills,omitempty"`
	SystemPrompt string        `yaml:"system_prompt,omitempty"`
	MaxIterations int          `yaml:"max_iterations,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// ModelConfig defines model configuration for an agent
type ModelConfig struct {
	Provider    string  `yaml:"provider"`    // openai, anthropic, etc.
	Model       string  `yaml:"model"`       // gpt-4, claude-3-opus, etc.
	Temperature float64 `yaml:"temperature,omitempty"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	APIKey      string  `yaml:"api_key,omitempty"` // Can reference env var
}

// RoutingConfig defines how messages are routed to agents
type RoutingConfig struct {
	Strategy string        `yaml:"strategy"` // round_robin, load_balance, rule_based
	Rules    []RoutingRule `yaml:"rules,omitempty"`
}

// RoutingRule defines a routing rule
type RoutingRule struct {
	Name      string   `yaml:"name"`
	Condition string   `yaml:"condition"` // channel:telegram, sender:admin, etc.
	AgentIDs  []string `yaml:"agent_ids"` // Target agent IDs
	Priority  int      `yaml:"priority,omitempty"`
}

// LoadAgentsConfig loads agents configuration from file
func LoadAgentsConfig(path string) (*AgentsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agents config: %w", err)
	}

	var config AgentsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse agents config: %w", err)
	}

	// Set defaults
	for i := range config.Agents {
		agent := &config.Agents[i]
		if agent.MaxIterations == 0 {
			agent.MaxIterations = 20
		}
		if agent.Model.Temperature == 0 {
			agent.Model.Temperature = 0.7
		}
		if agent.Model.MaxTokens == 0 {
			agent.Model.MaxTokens = 4096
		}
	}

	if config.Routing.Strategy == "" {
		config.Routing.Strategy = "round_robin"
	}

	return &config, nil
}

// SaveAgentsConfig saves agents configuration to file
func (c *AgentsConfig) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetEnabledAgents returns all enabled agents
func (c *AgentsConfig) GetEnabledAgents() []AgentDefinition {
	var enabled []AgentDefinition
	for _, agent := range c.Agents {
		if agent.Enabled {
			enabled = append(enabled, agent)
		}
	}
	return enabled
}

// GetAgentByID returns an agent by ID
func (c *AgentsConfig) GetAgentByID(id string) (*AgentDefinition, error) {
	for _, agent := range c.Agents {
		if agent.ID == id {
			return &agent, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %s", id)
}

// ValidateConfig validates the agents configuration
func (c *AgentsConfig) Validate() error {
	if len(c.Agents) == 0 {
		return fmt.Errorf("no agents defined")
	}

	// Check for duplicate IDs
	ids := make(map[string]bool)
	for _, agent := range c.Agents {
		if agent.ID == "" {
			return fmt.Errorf("agent ID is required")
		}
		if ids[agent.ID] {
			return fmt.Errorf("duplicate agent ID: %s", agent.ID)
		}
		ids[agent.ID] = true

		// Validate model config
		if agent.Model.Provider == "" {
			return fmt.Errorf("agent %s: model provider is required", agent.ID)
		}
		if agent.Model.Model == "" {
			return fmt.Errorf("agent %s: model name is required", agent.ID)
		}
	}

	// Validate routing rules
	for _, rule := range c.Routing.Rules {
		if rule.Name == "" {
			return fmt.Errorf("routing rule name is required")
		}
		if len(rule.AgentIDs) == 0 {
			return fmt.Errorf("routing rule %s: no agent IDs specified", rule.Name)
		}
		// Check that agent IDs exist
		for _, agentID := range rule.AgentIDs {
			if !ids[agentID] {
				return fmt.Errorf("routing rule %s: unknown agent ID: %s", rule.Name, agentID)
			}
		}
	}

	return nil
}
