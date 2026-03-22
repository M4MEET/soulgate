package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// HeartbeatConfig configures the periodic heartbeat that wakes the AI agent
// to proactively check for things that need attention.
type HeartbeatConfig struct {
	// Enabled controls whether the heartbeat timer runs automatically.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Interval is how often the heartbeat fires. Parsed as a Go duration
	// string (e.g. "30m", "1h"). Defaults to 30m when zero.
	Interval time.Duration `yaml:"interval" json:"interval"`

	// Target controls where heartbeat notifications are delivered.
	// Supported values: "none" (silent), "last" (last known channel), or a
	// channel name that the caller recognises (e.g. a Telegram chat ID).
	Target string `yaml:"target" json:"target"`

	// PromptFile is the workspace-relative path to a Markdown file that
	// provides the heartbeat instructions. Defaults to ".soulgate/HEARTBEAT.md".
	PromptFile string `yaml:"prompt_file" json:"prompt_file"`
}

// ContextConfig controls conversation history management and compaction.
type ContextConfig struct {
	// MaxHistoryMessages is the maximum number of messages to retain.
	// Default 50 (~25 user/assistant turns).
	MaxHistoryMessages int `yaml:"max_history_messages" json:"max_history_messages"`

	// MaxHistoryChars is the approximate character budget for history.
	// Default 100000 (~25k tokens).
	MaxHistoryChars int `yaml:"max_history_chars" json:"max_history_chars"`

	// CompactionEnabled controls whether LLM-based history compaction runs
	// when the history approaches its limits.
	CompactionEnabled bool `yaml:"compaction_enabled" json:"compaction_enabled"`

	// CompactionThreshold is the fraction of MaxHistoryChars at which
	// compaction triggers (e.g. 0.8 = 80%). Default 0.8.
	CompactionThreshold float64 `yaml:"compaction_threshold" json:"compaction_threshold"`
}

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

	// Context management settings
	Context ContextConfig `yaml:"context"`

	// HTTP client settings
	HTTPClient HTTPClientConfig `yaml:"http_client"`

	// Integrations settings
	Integrations map[string]IntegrationConfig `yaml:"integrations,omitempty"`

	// Skills settings
	Skills SkillsConfig `yaml:"skills"`

	// MCP (Model Context Protocol) server settings
	MCP MCPConfig `yaml:"mcp"`

	// Retention policy for audit logs, sessions, cost data, and memory.
	Retention RetentionConfig `yaml:"retention"`

	// Heartbeat configures the periodic proactive health-check.
	Heartbeat HeartbeatConfig `yaml:"heartbeat"`
}

// RetentionConfig defines data-retention settings for all stored artefacts.
// A zero Days value means "keep forever" (the safe default).
type RetentionConfig struct {
	// AuditLogDays is the number of days to retain audit JSONL files.
	// Files whose entire day has elapsed beyond this limit are deleted.
	AuditLogDays int `yaml:"audit_log_days"`

	// SessionDays is the number of days to retain session records.
	SessionDays int `yaml:"session_days"`

	// CostLogDays is the number of days to retain cost log entries.
	CostLogDays int `yaml:"cost_log_days"`

	// MemoryDays is the number of days to retain memory entries.
	// 0 means keep forever.
	MemoryDays int `yaml:"memory_days"`

	// AutoPurge, when true, runs retention automatically at startup.
	AutoPurge bool `yaml:"auto_purge"`
}

// MCPConfig defines MCP server settings
type MCPConfig struct {
	// List of MCP servers to connect to
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig defines a single MCP server connection
type MCPServerConfig struct {
	// Name identifies this server (used for tool prefixing)
	Name string `yaml:"name"`

	// Command to spawn the MCP server process
	Command string `yaml:"command"`

	// Arguments for the command
	Args []string `yaml:"args,omitempty"`

	// Environment variables for the server process
	Env map[string]string `yaml:"env,omitempty"`

	// Working directory for the server process
	WorkDir string `yaml:"work_dir,omitempty"`

	// Whether this server is enabled (default true)
	Enabled *bool `yaml:"enabled,omitempty"`
}

// WorkspaceConfig defines workspace-specific settings
type WorkspaceConfig struct {
	// Root directory of the workspace
	Root string `yaml:"root"`

	// Path to .soulgate directory
	ConfigDir string `yaml:"config_dir"`
}

// FallbackProviderConfig defines a single entry in the model fallback chain.
type FallbackProviderConfig struct {
	// Provider is the provider name from the registry (e.g. "groq", "ollama").
	Provider string `yaml:"provider"`

	// Model is the model name to use with this provider.
	// If empty the provider's DefaultModel is used.
	Model string `yaml:"model,omitempty"`

	// Priority controls evaluation order — lower numbers are tried first.
	Priority int `yaml:"priority"`
}

// ModelConfig defines model provider settings
type ModelConfig struct {
	// Default provider (openai, anthropic)
	DefaultProvider string `yaml:"default_provider"`

	// Provider-specific settings
	OpenAI    OpenAIConfig    `yaml:"openai"`
	Anthropic AnthropicConfig `yaml:"anthropic"`

	// FallbackChain is an ordered list of providers to try when the primary
	// provider fails with a retryable error (429, 500, 503, connection loss).
	// Entries are sorted by Priority ascending before use.
	FallbackChain []FallbackProviderConfig `yaml:"fallback_chain,omitempty"`
}

// OpenAIConfig defines OpenAI-specific settings
type OpenAIConfig struct {
	APIKey      string  `yaml:"api_key"`
	Model       string  `yaml:"model"`
	BaseURL     string  `yaml:"base_url"`
	MaxTokens   int     `yaml:"max_tokens"`
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
	// Path to the base policy file
	FilePath string `yaml:"file_path"`

	// ScopedFilePath is the path to the hierarchical (global/team/user/agent)
	// scoped policy file.  When empty the scoped engine starts with no rules.
	ScopedFilePath string `yaml:"scoped_file_path"`
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
				Model:       "gpt-4.1",
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
			DatabasePath: ".soulgate/audit.jsonl",
			Enabled:      true,
		},
		Policy: PolicyConfig{
			FilePath:       ".soulgate/policy.yml",
			ScopedFilePath: ".soulgate/scoped_policy.yml",
		},
		Execution: ExecutionConfig{
			MaxIterations:       0,     // unlimited
			TotalTimeoutSec:     0,     // unlimited
			IterationTimeoutSec: 0,     // unlimited
			APICallTimeoutSec:   0,     // unlimited
			MaxTokens:           0,     // unlimited
			MaxToolResultKB:     10240, // 10MB
		},
		Context: ContextConfig{
			MaxHistoryMessages:  50,
			MaxHistoryChars:     100000,
			CompactionEnabled:   true,
			CompactionThreshold: 0.8,
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
		Retention: RetentionConfig{
			AuditLogDays: 90,  // 3 months
			SessionDays:  30,  // 1 month
			CostLogDays:  365, // 1 year
			MemoryDays:   0,   // forever
			AutoPurge:    false,
		},
		Heartbeat: HeartbeatConfig{
			Enabled:    false,
			Interval:   30 * time.Minute,
			Target:     "none",
			PromptFile: ".soulgate/HEARTBEAT.md",
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
	if err := os.MkdirAll(dir, 0700); err != nil {
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
