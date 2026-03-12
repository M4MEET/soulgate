package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Workspace settings
	Workspace WorkspaceConfig `yaml:"workspace"`

	// Model provider settings
	Model ModelConfig `yaml:"model"`

	// Plugin settings
	Plugins PluginsConfig `yaml:"plugins"`

	// Audit settings
	Audit AuditConfig `yaml:"audit"`

	// Policy settings
	Policy PolicyConfig `yaml:"policy"`

	// Execution limits
	Execution ExecutionConfig `yaml:"execution"`

	// HTTP client settings
	HTTPClient HTTPClientConfig `yaml:"http_client"`

	// Integrations settings
	Integrations map[string]IntegrationConfig `yaml:"integrations,omitempty"`

	// Skills settings
	Skills SkillsConfig `yaml:"skills"`
}

// WorkspaceConfig defines workspace-specific settings
type WorkspaceConfig struct {
	// Root directory of the workspace
	Root string `yaml:"root"`

	// Path to .soulgate directory
	ConfigDir string `yaml:"config_dir"`
}

// ModelConfig defines model provider settings
type ModelConfig struct {
	// Default provider (openai, anthropic)
	DefaultProvider string `yaml:"default_provider"`

	// Provider-specific settings
	OpenAI    OpenAIConfig    `yaml:"openai"`
	Anthropic AnthropicConfig `yaml:"anthropic"`
}

// OpenAIConfig defines OpenAI-specific settings
type OpenAIConfig struct {
	APIKey      string `yaml:"api_key"`
	Model       string `yaml:"model"`
	BaseURL     string `yaml:"base_url"`
	MaxTokens   int    `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// AnthropicConfig defines Anthropic-specific settings
type AnthropicConfig struct {
	APIKey      string  `yaml:"api_key"`
	Model       string  `yaml:"model"`
	BaseURL     string  `yaml:"base_url"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
}

// PluginsConfig defines plugin system settings
type PluginsConfig struct {
	// Directory containing plugins
	Dir string `yaml:"dir"`

	// Maximum execution timeout for plugins (seconds)
	Timeout int `yaml:"timeout"`

	// Maximum memory for WASM plugins (bytes)
	MaxMemory int64 `yaml:"max_memory"`
}

// AuditConfig defines audit logging settings
type AuditConfig struct {
	// Path to audit database
	DatabasePath string `yaml:"database_path"`

	// Whether to enable audit logging
	Enabled bool `yaml:"enabled"`
}

// PolicyConfig defines policy engine settings
type PolicyConfig struct {
	// Path to policy file
	FilePath string `yaml:"file_path"`
}

// ExecutionConfig defines execution limits
type ExecutionConfig struct {
	MaxIterations       int `yaml:"max_iterations"`
	TotalTimeoutSec     int `yaml:"total_timeout_sec"`
	IterationTimeoutSec int `yaml:"iteration_timeout_sec"`
	APICallTimeoutSec   int `yaml:"api_call_timeout_sec"`
	MaxTokens           int `yaml:"max_tokens"`
	MaxToolResultKB     int `yaml:"max_tool_result_kb"`
}

// HTTPClientConfig defines HTTP client security settings
type HTTPClientConfig struct {
	ConnectTimeoutSec  int  `yaml:"connect_timeout_sec"`
	TLSTimeoutSec      int  `yaml:"tls_timeout_sec"`
	ResponseTimeoutSec int  `yaml:"response_timeout_sec"`
	TotalTimeoutSec    int  `yaml:"total_timeout_sec"`
	MaxRedirects       int  `yaml:"max_redirects"`
	AllowPrivateIPs    bool `yaml:"allow_private_ips"`
	AllowInsecureTLS   bool `yaml:"allow_insecure_tls"`
}

// IntegrationConfig defines settings for an integration
type IntegrationConfig struct {
	Enabled bool              `yaml:"enabled"`
	Config  map[string]string `yaml:"config"`
}

// SkillsConfig defines skills system settings
type SkillsConfig struct {
	// Directory containing skills (relative to workspace root)
	Dir string `yaml:"dir"`

	// Skills that are explicitly enabled; empty means all discovered skills are active
	EnabledSkills []string `yaml:"enabled_skills,omitempty"`

	// Maximum allowed size of a single SKILL.md file in bytes
	MaxSkillSize int `yaml:"max_skill_size"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Workspace: WorkspaceConfig{
			Root:      ".",
			ConfigDir: ".soulgate",
		},
		Model: ModelConfig{
			DefaultProvider: "openai",
			OpenAI: OpenAIConfig{
				Model:       "gpt-4",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
			Anthropic: AnthropicConfig{
				Model:       "claude-3-5-sonnet-20241022",
				MaxTokens:   4096,
				Temperature: 0.7,
			},
		},
		Plugins: PluginsConfig{
			Dir:       "plugins",
			Timeout:   30,
			MaxMemory: 64 * 1024 * 1024, // 64MB
		},
		Audit: AuditConfig{
			DatabasePath: ".soulgate/audit.db",
			Enabled:      true,
		},
		Policy: PolicyConfig{
			FilePath: ".soulgate/policy.yml",
		},
		Execution: ExecutionConfig{
			MaxIterations:       10,
			TotalTimeoutSec:     300,  // 5 minutes
			IterationTimeoutSec: 60,   // 1 minute
			APICallTimeoutSec:   30,   // 30 seconds
			MaxTokens:           100000,
			MaxToolResultKB:     1024, // 1MB
		},
		HTTPClient: HTTPClientConfig{
			ConnectTimeoutSec:  10,
			TLSTimeoutSec:      10,
			ResponseTimeoutSec: 30,
			TotalTimeoutSec:    90,
			MaxRedirects:       3,
			AllowPrivateIPs:    false,
			AllowInsecureTLS:   false,
		},
		Skills: SkillsConfig{
			Dir:          "skills",
			MaxSkillSize: 102400, // 100KB
		},
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables
	config.applyEnvOverrides()

	return config, nil
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		c.Model.OpenAI.APIKey = apiKey
	}

	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		c.Model.Anthropic.APIKey = apiKey
	}

	if provider := os.Getenv("SOULGATE_MODEL_PROVIDER"); provider != "" {
		c.Model.DefaultProvider = provider
	}
}

// Save saves the configuration to a file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Use 0600 for security - only owner can read/write (contains API keys)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Workspace.Root == "" {
		return fmt.Errorf("workspace root cannot be empty")
	}

	if c.Model.DefaultProvider == "" {
		return fmt.Errorf("default model provider must be specified")
	}

	validProviders := map[string]bool{
		"openai": true, "anthropic": true, "groq": true, "google": true,
		"mistral": true, "cohere": true, "deepseek": true, "openrouter": true,
		"together": true, "perplexity": true, "xai": true, "azure": true,
		"ollama": true, "custom": true,
	}
	if !validProviders[c.Model.DefaultProvider] {
		return fmt.Errorf("invalid model provider: %s", c.Model.DefaultProvider)
	}

	return nil
}
