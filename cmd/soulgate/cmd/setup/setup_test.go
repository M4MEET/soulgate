package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestPromptString tests the promptString helper function
func TestPromptString(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		prompt       string
		defaultValue string
		want         string
	}{
		{
			name:         "returns user input when provided",
			input:        "user-value\n",
			prompt:       "Enter value",
			defaultValue: "default",
			want:         "user-value",
		},
		{
			name:         "returns default when input is empty",
			input:        "\n",
			prompt:       "Enter value",
			defaultValue: "default",
			want:         "default",
		},
		{
			name:         "trims whitespace from input",
			input:        "  value-with-spaces  \n",
			prompt:       "Enter value",
			defaultValue: "",
			want:         "value-with-spaces",
		},
		{
			name:         "requires value when no default",
			input:        "\nactual-value\n",
			prompt:       "Enter value",
			defaultValue: "",
			want:         "actual-value",
		},
		{
			name:         "returns user input over default",
			input:        "override\n",
			prompt:       "Enter value",
			defaultValue: "default",
			want:         "override",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := promptString(reader, tt.prompt, tt.defaultValue, "")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPromptYesNo tests the promptYesNo helper function
func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		prompt       string
		defaultValue bool
		want         bool
	}{
		{
			name:         "returns true for 'y'",
			input:        "y\n",
			prompt:       "Continue?",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "returns true for 'yes'",
			input:        "yes\n",
			prompt:       "Continue?",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "returns true for 'YES' (case insensitive)",
			input:        "YES\n",
			prompt:       "Continue?",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "returns false for 'n'",
			input:        "n\n",
			prompt:       "Continue?",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "returns false for 'no'",
			input:        "no\n",
			prompt:       "Continue?",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "returns default when input is empty",
			input:        "\n",
			prompt:       "Continue?",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "retries on invalid input then accepts valid",
			input:        "maybe\ny\n",
			prompt:       "Continue?",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "trims whitespace",
			input:        "  y  \n",
			prompt:       "Continue?",
			defaultValue: false,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := promptYesNo(reader, tt.prompt, tt.defaultValue, "")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPromptInt tests the promptInt helper function
func TestPromptInt(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		prompt       string
		defaultValue int
		want         int
	}{
		{
			name:         "returns user input when valid number",
			input:        "42\n",
			prompt:       "Enter number",
			defaultValue: 10,
			want:         42,
		},
		{
			name:         "returns default when input is empty",
			input:        "\n",
			prompt:       "Enter number",
			defaultValue: 10,
			want:         10,
		},
		{
			name:         "retries on invalid input then accepts valid",
			input:        "not-a-number\n42\n",
			prompt:       "Enter number",
			defaultValue: 10,
			want:         42,
		},
		{
			name:         "handles negative numbers",
			input:        "-5\n",
			prompt:       "Enter number",
			defaultValue: 0,
			want:         -5,
		},
		{
			name:         "handles zero",
			input:        "0\n",
			prompt:       "Enter number",
			defaultValue: 10,
			want:         0,
		},
		{
			name:         "trims whitespace",
			input:        "  100  \n",
			prompt:       "Enter number",
			defaultValue: 1,
			want:         100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := promptInt(reader, tt.prompt, tt.defaultValue, -2147483648, 2147483647, "")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPromptChoice tests the promptChoice helper function
func TestPromptChoice(t *testing.T) {
	choices := []string{"option1", "option2", "option3"}

	tests := []struct {
		name         string
		input        string
		prompt       string
		choices      []string
		defaultValue string
		want         string
	}{
		{
			name:         "returns first choice when 1 entered",
			input:        "1\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option2",
			want:         "option1",
		},
		{
			name:         "returns second choice when 2 entered",
			input:        "2\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option1",
			want:         "option2",
		},
		{
			name:         "returns third choice when 3 entered",
			input:        "3\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option1",
			want:         "option3",
		},
		{
			name:         "returns default when input is empty",
			input:        "\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option2",
			want:         "option2",
		},
		{
			name:         "retries for invalid choice number then accepts valid",
			input:        "99\n2\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option1",
			want:         "option2",
		},
		{
			name:         "retries for zero then accepts valid",
			input:        "0\n1\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option2",
			want:         "option1",
		},
		{
			name:         "retries for negative number then accepts valid",
			input:        "-1\n3\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option3",
			want:         "option3",
		},
		{
			name:         "retries for non-numeric input then accepts valid",
			input:        "invalid\n1\n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option1",
			want:         "option1",
		},
		{
			name:         "trims whitespace from input",
			input:        "  2  \n",
			prompt:       "Select option",
			choices:      choices,
			defaultValue: "option1",
			want:         "option2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := promptChoice(reader, tt.prompt, tt.choices, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCreateSetupConfig tests the createSetupConfig function
func TestCreateSetupConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      setupConfig
		wantErr  bool
		validate func(t *testing.T, configDir string)
	}{
		{
			name: "creates config for OpenAI provider",
			cfg: setupConfig{
				provider:            "openai",
				apiKey:              "sk-test-key",
				modelName:           "gpt-4",
				policyMode:          "moderate",
				allowedPaths:        []string{"."},
				enableNetworkAccess: false,
				enableExecution:     false,
				enableAudit:         true,
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				// Check config.yml exists
				configPath := filepath.Join(configDir, "config.yml")
				require.FileExists(t, configPath)

				// Load and verify config
				cfg, err := config.LoadConfig(configPath)
				require.NoError(t, err)
				assert.Equal(t, "openai", cfg.Model.DefaultProvider)
				assert.Equal(t, "sk-test-key", cfg.Model.OpenAI.APIKey)
				assert.Equal(t, "gpt-4", cfg.Model.OpenAI.Model)
				assert.True(t, cfg.Audit.Enabled)

				// Check policy.yml exists
				policyPath := filepath.Join(configDir, "policy.yml")
				require.FileExists(t, policyPath)
			},
		},
		{
			name: "creates config for Anthropic provider",
			cfg: setupConfig{
				provider:            "anthropic",
				apiKey:              "sk-ant-test",
				modelName:           "claude-3-sonnet-20240229",
				policyMode:          "strict",
				allowedPaths:        []string{"."},
				enableNetworkAccess: false,
				enableExecution:     false,
				enableAudit:         false,
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				configPath := filepath.Join(configDir, "config.yml")
				cfg, err := config.LoadConfig(configPath)
				require.NoError(t, err)
				assert.Equal(t, "anthropic", cfg.Model.DefaultProvider)
				assert.Equal(t, "sk-ant-test", cfg.Model.Anthropic.APIKey)
				assert.Equal(t, "claude-3-sonnet-20240229", cfg.Model.Anthropic.Model)
				assert.False(t, cfg.Audit.Enabled)
			},
		},
		{
			name: "creates config for Ollama provider",
			cfg: setupConfig{
				provider:            "ollama",
				baseURL:             "http://localhost:11434",
				modelName:           "llama2",
				policyMode:          "permissive",
				allowedPaths:        []string{"$HOME"},
				enableNetworkAccess: true,
				enableExecution:     true,
				enableAudit:         true,
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				configPath := filepath.Join(configDir, "config.yml")
				cfg, err := config.LoadConfig(configPath)
				require.NoError(t, err)
				assert.Equal(t, "ollama", cfg.Model.DefaultProvider)
				assert.Equal(t, "http://localhost:11434", cfg.Model.OpenAI.BaseURL)
				assert.Equal(t, "llama2", cfg.Model.OpenAI.Model)
			},
		},
		{
			name: "creates config with agents enabled",
			cfg: setupConfig{
				provider:           "openai",
				policyMode:         "moderate",
				allowedPaths:       []string{"."},
				enableTestAgent:    true,
				enableDocsAgent:    true,
				enablePMAgent:      true,
				coverageTarget:     85,
				assignmentStrategy: "skill_based",
				enableAudit:        true,
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				agentsPath := filepath.Join(configDir, "agents.yaml")
				require.FileExists(t, agentsPath)

				// Read and verify agents file
				data, err := os.ReadFile(agentsPath)
				require.NoError(t, err)

				content := string(data)
				assert.Contains(t, content, "test-quality:")
				assert.Contains(t, content, "coverage_target: 85")
				assert.Contains(t, content, "docs-api:")
				assert.Contains(t, content, "project-management:")
				assert.Contains(t, content, "assignment_strategy: \"skill_based\"")
			},
		},
		{
			name: "skips provider configuration",
			cfg: setupConfig{
				provider:     "skip",
				policyMode:   "moderate",
				allowedPaths: []string{"."},
				enableAudit:  true,
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				configPath := filepath.Join(configDir, "config.yml")
				cfg, err := config.LoadConfig(configPath)
				require.NoError(t, err)
				// Default provider should remain
				assert.NotEmpty(t, cfg.Model.DefaultProvider)
			},
		},
		{
			name: "creates config with all agents disabled",
			cfg: setupConfig{
				provider:        "openai",
				policyMode:      "moderate",
				allowedPaths:    []string{"."},
				enableTestAgent: false,
				enableDocsAgent: false,
				enablePMAgent:   false,
				enableAudit:     true,
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				// agents.yaml should not be created when all agents disabled
				agentsPath := filepath.Join(configDir, "agents.yaml")
				_, err := os.Stat(agentsPath)
				assert.True(t, os.IsNotExist(err), "agents.yaml should not exist when all agents disabled")
			},
		},
		{
			name: "creates config with notification channels",
			cfg: setupConfig{
				provider:             "openai",
				policyMode:           "moderate",
				allowedPaths:         []string{"."},
				enableAudit:          true,
				notificationChannels: []string{"console", "slack", "webhook"},
			},
			wantErr: false,
			validate: func(t *testing.T, configDir string) {
				// Config should be created successfully
				configPath := filepath.Join(configDir, "config.yml")
				require.FileExists(t, configPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear env vars so Viper doesn't override test values when loading config
			for _, env := range []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GROQ_API_KEY"} {
				t.Setenv(env, "")
			}

			// Create temporary directory
			tmpDir := t.TempDir()
			configDir := filepath.Join(tmpDir, ".soulgate")
			err := os.MkdirAll(configDir, 0755)
			require.NoError(t, err)

			// Execute
			err = createSetupConfig(configDir, tt.cfg)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, configDir)
				}
			}
		})
	}
}

// TestCreatePolicyFile tests policy file generation for all security modes
func TestCreatePolicyFile(t *testing.T) {
	tests := []struct {
		name     string
		cfg      setupConfig
		validate func(t *testing.T, content string)
	}{
		{
			name: "strict mode - no network, no execution",
			cfg: setupConfig{
				policyMode:          "strict",
				allowedPaths:        []string{"."},
				enableNetworkAccess: false,
				enableExecution:     false,
			},
			validate: func(t *testing.T, content string) {
				// Must contain workspace read policies
				assert.Contains(t, content, "allow-workspace-reads")
				assert.Contains(t, content, "allow-workspace-list")
				assert.Contains(t, content, "allow-workspace-stat")

				// Must deny parent and absolute paths
				assert.Contains(t, content, "deny-parent-access")
				assert.Contains(t, content, "deny-absolute-paths")

				// Must deny network and execution
				assert.Contains(t, content, "deny-network")
				assert.Contains(t, content, "deny-execution")

				// Verify decisions
				assert.Contains(t, content, "decision: deny")

				// Parse as YAML to ensure valid structure
				var policy map[string]interface{}
				err := yaml.Unmarshal([]byte(content), &policy)
				require.NoError(t, err)
				assert.Equal(t, "1", policy["version"])

				policies := policy["policies"].([]interface{})
				assert.Greater(t, len(policies), 0, "should have policies")
			},
		},
		{
			name: "moderate mode - with network access",
			cfg: setupConfig{
				policyMode:          "moderate",
				allowedPaths:        []string{".", "$HOME/projects"},
				enableNetworkAccess: true,
				enableExecution:     false,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "allow-workspace-operations")
				assert.Contains(t, content, "deny-parent-access")
				assert.Contains(t, content, "network-policy")
				assert.Contains(t, content, "decision: allow", "network should be allowed")
				assert.Contains(t, content, "deny-execution")

				// Verify YAML structure
				var policy map[string]interface{}
				err := yaml.Unmarshal([]byte(content), &policy)
				require.NoError(t, err)
			},
		},
		{
			name: "moderate mode - without network access",
			cfg: setupConfig{
				policyMode:          "moderate",
				allowedPaths:        []string{"."},
				enableNetworkAccess: false,
				enableExecution:     false,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "network-policy")
				assert.Contains(t, content, "decision: deny")
			},
		},
		{
			name: "permissive mode - with execution",
			cfg: setupConfig{
				policyMode:          "permissive",
				allowedPaths:        []string{"$HOME"},
				enableNetworkAccess: true,
				enableExecution:     true,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "allow-workspace-operations")
				assert.Contains(t, content, "allow-home-read")
				assert.Contains(t, content, "allow-network")
				assert.Contains(t, content, "execution-policy")

				// Check that execution is allowed
				lines := strings.Split(content, "\n")
				foundExecPolicy := false
				for i, line := range lines {
					if strings.Contains(line, "execution-policy") {
						foundExecPolicy = true
						// Look ahead for decision
						for j := i; j < i+5 && j < len(lines); j++ {
							if strings.Contains(lines[j], "decision:") {
								assert.Contains(t, lines[j], "allow", "execution should be allowed")
								break
							}
						}
						break
					}
				}
				assert.True(t, foundExecPolicy, "should have execution policy")
			},
		},
		{
			name: "permissive mode - without execution",
			cfg: setupConfig{
				policyMode:          "permissive",
				allowedPaths:        []string{"$HOME"},
				enableNetworkAccess: true,
				enableExecution:     false,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "execution-policy")
				// Find execution policy and verify it's deny
				lines := strings.Split(content, "\n")
				foundExecPolicy := false
				for i, line := range lines {
					if strings.Contains(line, "execution-policy") {
						foundExecPolicy = true
						for j := i; j < i+5 && j < len(lines); j++ {
							if strings.Contains(lines[j], "decision:") {
								assert.Contains(t, lines[j], "deny", "execution should be denied")
								break
							}
						}
						break
					}
				}
				assert.True(t, foundExecPolicy)
			},
		},
		{
			name: "custom mode - single path",
			cfg: setupConfig{
				policyMode:          "custom",
				allowedPaths:        []string{"/tmp/workspace"},
				enableNetworkAccess: false,
				enableExecution:     false,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "Custom Security Mode")
				assert.Contains(t, content, "allow-path-1")
				assert.Contains(t, content, "/tmp/workspace/**")
				assert.Contains(t, content, "network-policy")
				assert.Contains(t, content, "execution-policy")

				var policy map[string]interface{}
				err := yaml.Unmarshal([]byte(content), &policy)
				require.NoError(t, err)
			},
		},
		{
			name: "custom mode - multiple paths",
			cfg: setupConfig{
				policyMode:          "custom",
				allowedPaths:        []string{"/path/one", "/path/two", "/path/three"},
				enableNetworkAccess: true,
				enableExecution:     true,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "allow-path-1")
				assert.Contains(t, content, "allow-path-2")
				assert.Contains(t, content, "allow-path-3")
				assert.Contains(t, content, "/path/one/**")
				assert.Contains(t, content, "/path/two/**")
				assert.Contains(t, content, "/path/three/**")

				// Verify network and execution are allowed
				assert.Contains(t, content, "decision: allow")
			},
		},
		{
			name: "custom mode - all features enabled",
			cfg: setupConfig{
				policyMode:          "custom",
				allowedPaths:        []string{".", "$HOME/docs"},
				enableNetworkAccess: true,
				enableExecution:     true,
			},
			validate: func(t *testing.T, content string) {
				// Should have all policies
				assert.Contains(t, content, "allow-path-1")
				assert.Contains(t, content, "allow-path-2")
				assert.Contains(t, content, "network-policy")
				assert.Contains(t, content, "execution-policy")

				// Count "decision: allow" occurrences - should have multiple
				allowCount := strings.Count(content, "decision: allow")
				assert.Greater(t, allowCount, 2, "should have multiple allow decisions")

				// Valid YAML
				var policy map[string]interface{}
				err := yaml.Unmarshal([]byte(content), &policy)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			policyPath := filepath.Join(tmpDir, "policy.yml")

			// Execute
			err := createPolicyFile(policyPath, tt.cfg)
			require.NoError(t, err)

			// Read content
			content, err := os.ReadFile(policyPath)
			require.NoError(t, err)

			// Validate
			tt.validate(t, string(content))
		})
	}
}

// TestCreateAgentsFile tests agent configuration file generation
func TestCreateAgentsFile(t *testing.T) {
	tests := []struct {
		name     string
		cfg      setupConfig
		validate func(t *testing.T, content string)
	}{
		{
			name: "only test agent enabled",
			cfg: setupConfig{
				enableTestAgent: true,
				enableDocsAgent: false,
				enablePMAgent:   false,
				coverageTarget:  80,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "test-quality:")
				assert.Contains(t, content, "enabled: true")
				assert.Contains(t, content, "coverage_target: 80")
				assert.NotContains(t, content, "docs-api:")
				assert.NotContains(t, content, "project-management:")

				// Valid YAML
				var agents map[string]interface{}
				err := yaml.Unmarshal([]byte(content), &agents)
				require.NoError(t, err)
				assert.Equal(t, "1", agents["version"])
			},
		},
		{
			name: "only docs agent enabled",
			cfg: setupConfig{
				enableTestAgent: false,
				enableDocsAgent: true,
				enablePMAgent:   false,
			},
			validate: func(t *testing.T, content string) {
				assert.NotContains(t, content, "test-quality:")
				assert.Contains(t, content, "docs-api:")
				assert.Contains(t, content, "enabled: true")
				assert.Contains(t, content, "api_spec_format: \"openapi\"")
				assert.Contains(t, content, "docs_coverage_target: 90")
				assert.NotContains(t, content, "project-management:")
			},
		},
		{
			name: "only pm agent enabled",
			cfg: setupConfig{
				enableTestAgent:    false,
				enableDocsAgent:    false,
				enablePMAgent:      true,
				assignmentStrategy: "round_robin",
			},
			validate: func(t *testing.T, content string) {
				assert.NotContains(t, content, "test-quality:")
				assert.NotContains(t, content, "docs-api:")
				assert.Contains(t, content, "project-management:")
				assert.Contains(t, content, "enabled: true")
				assert.Contains(t, content, "assignment_strategy: \"round_robin\"")
				assert.Contains(t, content, "max_tasks_per_dev: 3")
				assert.Contains(t, content, "sprint_duration_days: 14")
			},
		},
		{
			name: "all agents enabled",
			cfg: setupConfig{
				enableTestAgent:    true,
				enableDocsAgent:    true,
				enablePMAgent:      true,
				coverageTarget:     95,
				assignmentStrategy: "workload_balanced",
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "test-quality:")
				assert.Contains(t, content, "coverage_target: 95")
				assert.Contains(t, content, "docs-api:")
				assert.Contains(t, content, "project-management:")
				assert.Contains(t, content, "assignment_strategy: \"workload_balanced\"")

				// Valid YAML with all agents
				var agents map[string]interface{}
				err := yaml.Unmarshal([]byte(content), &agents)
				require.NoError(t, err)

				agentList := agents["agents"].(map[string]interface{})
				assert.Contains(t, agentList, "test-quality")
				assert.Contains(t, agentList, "docs-api")
				assert.Contains(t, agentList, "project-management")
			},
		},
		{
			name: "test agent with custom coverage",
			cfg: setupConfig{
				enableTestAgent: true,
				coverageTarget:  75,
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "coverage_target: 75")
				assert.Contains(t, content, "mode: \"auto\"")
			},
		},
		{
			name: "pm agent with skill_based strategy",
			cfg: setupConfig{
				enablePMAgent:      true,
				assignmentStrategy: "skill_based",
			},
			validate: func(t *testing.T, content string) {
				assert.Contains(t, content, "assignment_strategy: \"skill_based\"")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			agentsPath := filepath.Join(tmpDir, "agents.yaml")

			// Execute
			err := createAgentsFile(agentsPath, tt.cfg)
			require.NoError(t, err)

			// Read content
			content, err := os.ReadFile(agentsPath)
			require.NoError(t, err)

			// Validate
			tt.validate(t, string(content))
		})
	}
}

// TestCreatePolicyFileErrorHandling tests error cases for policy file creation
func TestCreatePolicyFileErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		cfg     setupConfig
		wantErr bool
	}{
		{
			name: "invalid directory path",
			path: "/nonexistent/directory/policy.yml",
			cfg: setupConfig{
				policyMode: "strict",
			},
			wantErr: true,
		},
		{
			name: "valid path creates file successfully",
			path: "", // will be set in test
			cfg: setupConfig{
				policyMode: "moderate",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if path == "" {
				tmpDir := t.TempDir()
				path = filepath.Join(tmpDir, "policy.yml")
			}

			err := createPolicyFile(path, tt.cfg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				_, err := os.Stat(path)
				assert.NoError(t, err, "file should exist")
			}
		})
	}
}

// TestSetupConfigEdgeCases tests edge cases and boundary conditions
func TestSetupConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		cfg     setupConfig
		wantErr bool
	}{
		{
			name: "empty allowed paths",
			cfg: setupConfig{
				provider:     "openai",
				policyMode:   "custom",
				allowedPaths: []string{},
				enableAudit:  true,
			},
			wantErr: false,
		},
		{
			name: "nil notification channels",
			cfg: setupConfig{
				provider:             "openai",
				policyMode:           "moderate",
				allowedPaths:         []string{"."},
				notificationChannels: nil,
				enableAudit:          true,
			},
			wantErr: false,
		},
		{
			name: "zero coverage target",
			cfg: setupConfig{
				provider:        "openai",
				policyMode:      "moderate",
				allowedPaths:    []string{"."},
				enableTestAgent: true,
				coverageTarget:  0,
				enableAudit:     true,
			},
			wantErr: false,
		},
		{
			name: "empty assignment strategy",
			cfg: setupConfig{
				provider:           "openai",
				policyMode:         "moderate",
				allowedPaths:       []string{"."},
				enablePMAgent:      true,
				assignmentStrategy: "",
				enableAudit:        true,
			},
			wantErr: false,
		},
		{
			name: "all fields empty except required",
			cfg: setupConfig{
				policyMode:   "strict",
				allowedPaths: []string{"."},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configDir := filepath.Join(tmpDir, ".soulgate")
			err := os.MkdirAll(configDir, 0755)
			require.NoError(t, err)

			err = createSetupConfig(configDir, tt.cfg)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPolicyFilePermissions tests that policy files are created with correct permissions
func TestPolicyFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.yml")

	cfg := setupConfig{
		policyMode:   "strict",
		allowedPaths: []string{"."},
	}

	err := createPolicyFile(policyPath, cfg)
	require.NoError(t, err)

	info, err := os.Stat(policyPath)
	require.NoError(t, err)

	// Check permissions are 0644
	mode := info.Mode()
	assert.Equal(t, os.FileMode(0644), mode.Perm(), "policy file should have 0644 permissions")
}

// TestAgentsFilePermissions tests that agents files are created with correct permissions
func TestAgentsFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	agentsPath := filepath.Join(tmpDir, "agents.yaml")

	cfg := setupConfig{
		enableTestAgent: true,
		coverageTarget:  85,
	}

	err := createAgentsFile(agentsPath, cfg)
	require.NoError(t, err)

	info, err := os.Stat(agentsPath)
	require.NoError(t, err)

	// Check permissions are 0644
	mode := info.Mode()
	assert.Equal(t, os.FileMode(0644), mode.Perm(), "agents file should have 0644 permissions")
}

// TestPolicyFileYAMLValidity ensures all generated policy files are valid YAML
func TestPolicyFileYAMLValidity(t *testing.T) {
	modes := []string{"strict", "moderate", "permissive", "custom"}

	for _, mode := range modes {
		t.Run(fmt.Sprintf("mode_%s", mode), func(t *testing.T) {
			tmpDir := t.TempDir()
			policyPath := filepath.Join(tmpDir, "policy.yml")

			cfg := setupConfig{
				policyMode:          mode,
				allowedPaths:        []string{".", "/tmp"},
				enableNetworkAccess: true,
				enableExecution:     true,
			}

			err := createPolicyFile(policyPath, cfg)
			require.NoError(t, err)

			// Read and parse as YAML
			content, err := os.ReadFile(policyPath)
			require.NoError(t, err)

			var parsed map[string]interface{}
			err = yaml.Unmarshal(content, &parsed)
			require.NoError(t, err, "generated policy file should be valid YAML")

			// Verify basic structure
			assert.Equal(t, "1", parsed["version"])
			assert.NotNil(t, parsed["policies"])

			policies, ok := parsed["policies"].([]interface{})
			assert.True(t, ok, "policies should be an array")
			assert.Greater(t, len(policies), 0, "should have at least one policy")
		})
	}
}

// TestAgentsFileYAMLValidity ensures all generated agents files are valid YAML
func TestAgentsFileYAMLValidity(t *testing.T) {
	tests := []struct {
		name string
		cfg  setupConfig
	}{
		{
			name: "all agents",
			cfg: setupConfig{
				enableTestAgent:    true,
				enableDocsAgent:    true,
				enablePMAgent:      true,
				coverageTarget:     85,
				assignmentStrategy: "skill_based",
			},
		},
		{
			name: "single agent",
			cfg: setupConfig{
				enableTestAgent: true,
				coverageTarget:  90,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			agentsPath := filepath.Join(tmpDir, "agents.yaml")

			err := createAgentsFile(agentsPath, tt.cfg)
			require.NoError(t, err)

			// Read and parse as YAML
			content, err := os.ReadFile(agentsPath)
			require.NoError(t, err)

			var parsed map[string]interface{}
			err = yaml.Unmarshal(content, &parsed)
			require.NoError(t, err, "generated agents file should be valid YAML")

			// Verify basic structure
			assert.Equal(t, "1", parsed["version"])
			assert.NotNil(t, parsed["agents"])
		})
	}
}

// TestPromptChoiceWithSingleOption tests promptChoice with edge case of single option
func TestPromptChoiceWithSingleOption(t *testing.T) {
	choices := []string{"only-option"}
	input := "1\n"
	reader := bufio.NewReader(strings.NewReader(input))

	result := promptChoice(reader, "Select", choices, "only-option")
	assert.Equal(t, "only-option", result)
}

// TestPromptIntWithLargeNumbers tests promptInt with boundary values
func TestPromptIntWithLargeNumbers(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue int
		want         int
	}{
		{
			name:         "very large number",
			input:        "999999999\n",
			defaultValue: 0,
			want:         999999999,
		},
		{
			name:         "max int boundary",
			input:        "2147483647\n",
			defaultValue: 0,
			want:         2147483647,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := promptInt(reader, "Enter", tt.defaultValue, -2147483648, 2147483647, "")
			assert.Equal(t, tt.want, got)
		})
	}
}
