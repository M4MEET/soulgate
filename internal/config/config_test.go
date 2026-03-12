package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, ".", cfg.Workspace.Root)
	assert.Equal(t, ".soulgate", cfg.Workspace.ConfigDir)
	assert.Equal(t, "openai", cfg.Model.DefaultProvider)
	assert.Equal(t, "gpt-4", cfg.Model.OpenAI.Model)
	assert.Equal(t, "claude-3-5-sonnet-20241022", cfg.Model.Anthropic.Model)
	assert.Equal(t, true, cfg.Audit.Enabled)
	assert.Equal(t, false, cfg.HTTPClient.AllowPrivateIPs)
	assert.Equal(t, false, cfg.HTTPClient.AllowInsecureTLS)
}

func TestLoadConfigValidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	// Create valid YAML config
	validYAML := `
workspace:
  root: /test/workspace
  config_dir: .soulgate

model:
  default_provider: anthropic
  openai:
    model: gpt-3.5-turbo
    max_tokens: 2000
  anthropic:
    model: claude-3-opus-20240229
    max_tokens: 4000

plugins:
  dir: plugins
  timeout: 60

audit:
  database_path: .soulgate/audit.db
  enabled: true
`

	err := os.WriteFile(configPath, []byte(validYAML), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "/test/workspace", cfg.Workspace.Root)
	assert.Equal(t, "anthropic", cfg.Model.DefaultProvider)
	assert.Equal(t, "gpt-3.5-turbo", cfg.Model.OpenAI.Model)
	assert.Equal(t, "claude-3-opus-20240229", cfg.Model.Anthropic.Model)
	assert.Equal(t, 60, cfg.Plugins.Timeout)
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	// Create invalid YAML
	invalidYAML := `
model:
  default_provider: openai
  openai:
    model: gpt-4
	invalid_indentation_here
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to parse config")
}

func TestLoadConfigNonExistentFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yml")

	cfg := DefaultConfig()
	cfg.Model.DefaultProvider = "anthropic"
	cfg.Workspace.Root = "/test/root"

	err := cfg.Save(configPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Verify file permissions (0600 for security)
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Verify content can be loaded back
	loadedCfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, "anthropic", loadedCfg.Model.DefaultProvider)
	assert.Equal(t, "/test/root", loadedCfg.Workspace.Root)
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested", "deep", "config.yml")

	cfg := DefaultConfig()
	err := cfg.Save(configPath)
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(filepath.Dir(configPath))
	assert.NoError(t, err)
}

func TestApplyEnvOverrides(t *testing.T) {
	// Save original env vars
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	origAnthropic := os.Getenv("ANTHROPIC_API_KEY")
	origProvider := os.Getenv("SOULGATE_MODEL_PROVIDER")

	// Clean up env vars after test
	defer func() {
		os.Setenv("OPENAI_API_KEY", origOpenAI)
		os.Setenv("ANTHROPIC_API_KEY", origAnthropic)
		os.Setenv("SOULGATE_MODEL_PROVIDER", origProvider)
	}()

	// Set test env vars
	os.Setenv("OPENAI_API_KEY", "test-openai-key")
	os.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	os.Setenv("SOULGATE_MODEL_PROVIDER", "anthropic")

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()

	assert.Equal(t, "test-openai-key", cfg.Model.OpenAI.APIKey)
	assert.Equal(t, "test-anthropic-key", cfg.Model.Anthropic.APIKey)
	assert.Equal(t, "anthropic", cfg.Model.DefaultProvider)
}

func TestApplyEnvOverridesPartial(t *testing.T) {
	// Save original env vars
	origOpenAI := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", origOpenAI)

	// Only set OpenAI key
	os.Setenv("OPENAI_API_KEY", "partial-test-key")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("SOULGATE_MODEL_PROVIDER")

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()

	assert.Equal(t, "partial-test-key", cfg.Model.OpenAI.APIKey)
	assert.Empty(t, cfg.Model.Anthropic.APIKey)
	assert.Equal(t, "openai", cfg.Model.DefaultProvider) // Default unchanged
}

func TestLoadConfigWithEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	// Create config file
	yamlContent := `
model:
  default_provider: openai
  openai:
    api_key: file-key
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Set env var to override
	origKey := os.Getenv("OPENAI_API_KEY")
	defer os.Setenv("OPENAI_API_KEY", origKey)
	os.Setenv("OPENAI_API_KEY", "env-override-key")

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Env var should override file value
	assert.Equal(t, "env-override-key", cfg.Model.OpenAI.APIKey)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Config)
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid config",
			setupFunc: func(c *Config) {},
			wantError: false,
		},
		{
			name: "empty workspace root",
			setupFunc: func(c *Config) {
				c.Workspace.Root = ""
			},
			wantError: true,
			errorMsg:  "workspace root cannot be empty",
		},
		{
			name: "empty provider",
			setupFunc: func(c *Config) {
				c.Model.DefaultProvider = ""
			},
			wantError: true,
			errorMsg:  "default model provider must be specified",
		},
		{
			name: "invalid provider",
			setupFunc: func(c *Config) {
				c.Model.DefaultProvider = "invalid"
			},
			wantError: true,
			errorMsg:  "invalid model provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.setupFunc(cfg)

			err := cfg.Validate()

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// Workspace defaults
	assert.Equal(t, ".", cfg.Workspace.Root)
	assert.Equal(t, ".soulgate", cfg.Workspace.ConfigDir)

	// Model defaults
	assert.Equal(t, "openai", cfg.Model.DefaultProvider)
	assert.Equal(t, 4096, cfg.Model.OpenAI.MaxTokens)
	assert.Equal(t, 0.7, cfg.Model.OpenAI.Temperature)
	assert.Equal(t, 4096, cfg.Model.Anthropic.MaxTokens)

	// Plugin defaults
	assert.Equal(t, "plugins", cfg.Plugins.Dir)
	assert.Equal(t, 30, cfg.Plugins.Timeout)
	assert.Equal(t, int64(64*1024*1024), cfg.Plugins.MaxMemory)

	// Audit defaults
	assert.Equal(t, ".soulgate/audit.db", cfg.Audit.DatabasePath)
	assert.True(t, cfg.Audit.Enabled)

	// Policy defaults
	assert.Equal(t, ".soulgate/policy.yml", cfg.Policy.FilePath)

	// Execution defaults
	assert.Equal(t, 10, cfg.Execution.MaxIterations)
	assert.Equal(t, 300, cfg.Execution.TotalTimeoutSec)
	assert.Equal(t, 60, cfg.Execution.IterationTimeoutSec)

	// HTTP client defaults
	assert.Equal(t, 10, cfg.HTTPClient.ConnectTimeoutSec)
	assert.Equal(t, 3, cfg.HTTPClient.MaxRedirects)
	assert.False(t, cfg.HTTPClient.AllowPrivateIPs)
	assert.False(t, cfg.HTTPClient.AllowInsecureTLS)
}

func TestModelConfigs(t *testing.T) {
	cfg := DefaultConfig()

	// OpenAI config
	assert.Equal(t, "gpt-4", cfg.Model.OpenAI.Model)
	assert.Empty(t, cfg.Model.OpenAI.APIKey) // Should be empty by default
	assert.Empty(t, cfg.Model.OpenAI.BaseURL)

	// Anthropic config
	assert.Equal(t, "claude-3-5-sonnet-20241022", cfg.Model.Anthropic.Model)
	assert.Empty(t, cfg.Model.Anthropic.APIKey)
	assert.Empty(t, cfg.Model.Anthropic.BaseURL)
}

func TestExecutionConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 10, cfg.Execution.MaxIterations)
	assert.Equal(t, 300, cfg.Execution.TotalTimeoutSec)
	assert.Equal(t, 60, cfg.Execution.IterationTimeoutSec)
	assert.Equal(t, 30, cfg.Execution.APICallTimeoutSec)
	assert.Equal(t, 100000, cfg.Execution.MaxTokens)
	assert.Equal(t, 1024, cfg.Execution.MaxToolResultKB)
}

func TestHTTPClientSecurityDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// Security defaults should be restrictive
	assert.False(t, cfg.HTTPClient.AllowPrivateIPs, "should not allow private IPs by default")
	assert.False(t, cfg.HTTPClient.AllowInsecureTLS, "should not allow insecure TLS by default")
	assert.Equal(t, 3, cfg.HTTPClient.MaxRedirects, "should limit redirects")
}

func TestIntegrationConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	yamlContent := `
integrations:
  github:
    enabled: true
    config:
      token: test-token
      org: test-org
  slack:
    enabled: false
    config:
      webhook: test-webhook
`

	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Len(t, cfg.Integrations, 2)
	assert.True(t, cfg.Integrations["github"].Enabled)
	assert.Equal(t, "test-token", cfg.Integrations["github"].Config["token"])
	assert.False(t, cfg.Integrations["slack"].Enabled)
}

func TestConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	// Create a config with custom values
	original := DefaultConfig()
	original.Workspace.Root = "/custom/workspace"
	original.Model.DefaultProvider = "anthropic"
	original.Model.OpenAI.Model = "gpt-4-turbo"
	original.Plugins.Timeout = 120
	original.HTTPClient.MaxRedirects = 5

	// Save it
	err := original.Save(configPath)
	require.NoError(t, err)

	// Load it back
	loaded, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify values match
	assert.Equal(t, original.Workspace.Root, loaded.Workspace.Root)
	assert.Equal(t, original.Model.DefaultProvider, loaded.Model.DefaultProvider)
	assert.Equal(t, original.Model.OpenAI.Model, loaded.Model.OpenAI.Model)
	assert.Equal(t, original.Plugins.Timeout, loaded.Plugins.Timeout)
	assert.Equal(t, original.HTTPClient.MaxRedirects, loaded.HTTPClient.MaxRedirects)
}
