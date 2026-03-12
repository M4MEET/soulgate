package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/M4MEET/soulgate/internal/config"
)

type setupConfig struct {
	provider             string
	apiKey               string
	baseURL              string
	modelName            string
	policyMode           string
	allowedPaths         []string
	enableNetworkAccess  bool
	enableExecution      bool
	enableTestAgent      bool
	enableDocsAgent      bool
	enablePMAgent        bool
	coverageTarget       int
	assignmentStrategy   string
	enableAudit          bool
	notificationChannels []string
}

func createSetupConfig(configDir string, cfg setupConfig) error {
	// Load or create config
	configPath := fmt.Sprintf("%s/config.yml", configDir)

	var conf *config.Config
	if _, err := os.Stat(configPath); err == nil {
		// Load existing config
		conf, err = config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load existing config: %w", err)
		}
	} else {
		// Create new config
		conf = config.DefaultConfig()
	}

	// Configure model provider
	if cfg.provider != "skip" {
		conf.Model.DefaultProvider = cfg.provider

		switch cfg.provider {
		case "openai":
			conf.Model.OpenAI.APIKey = cfg.apiKey
			if cfg.modelName != "" {
				conf.Model.OpenAI.Model = cfg.modelName
			}
			if cfg.baseURL != "" {
				conf.Model.OpenAI.BaseURL = cfg.baseURL
			}

		case "anthropic":
			conf.Model.Anthropic.APIKey = cfg.apiKey
			if cfg.modelName != "" {
				conf.Model.Anthropic.Model = cfg.modelName
			}
			if cfg.baseURL != "" {
				conf.Model.Anthropic.BaseURL = cfg.baseURL
			}

		case "ollama":
			// Ollama configuration would go here when implemented
			if cfg.baseURL != "" {
				conf.Model.OpenAI.BaseURL = cfg.baseURL // Use OpenAI-compatible endpoint
			}
			if cfg.modelName != "" {
				conf.Model.OpenAI.Model = cfg.modelName
			}
		}
	}

	// Configure audit
	conf.Audit.Enabled = cfg.enableAudit

	// Save config
	if err := conf.Save(configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create policy file based on security mode
	policyPath := fmt.Sprintf("%s/policy.yml", configDir)
	if err := createPolicyFile(policyPath, cfg); err != nil {
		return fmt.Errorf("failed to create policy file: %w", err)
	}

	// Create agents configuration if any agents are enabled
	if cfg.enableTestAgent || cfg.enableDocsAgent || cfg.enablePMAgent {
		agentsPath := fmt.Sprintf("%s/agents.yaml", configDir)
		if err := createAgentsFile(agentsPath, cfg); err != nil {
			return fmt.Errorf("failed to create agents file: %w", err)
		}
	}

	return nil
}

func createPolicyFile(path string, cfg setupConfig) error {
	var policyContent string

	switch cfg.policyMode {
	case "strict":
		policyContent = `version: "1"

# Strict Security Mode
# - Only workspace files allowed
# - No network access
# - No command execution

policies:
  - name: "allow-workspace-reads"
    description: "Allow reading files within workspace only"
    action: "files.read"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "allow-workspace-list"
    description: "Allow listing directories within workspace"
    action: "files.list"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "allow-workspace-stat"
    description: "Allow stat operations within workspace"
    action: "files.stat"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "deny-parent-access"
    description: "Deny access to parent directories"
    action: "files.*"
    resource: "../**"
    decision: deny
    priority: 100

  - name: "deny-absolute-paths"
    description: "Deny absolute paths outside workspace"
    action: "files.*"
    resource: "/**"
    decision: deny
    priority: 90

  - name: "deny-network"
    description: "Deny all network access"
    action: "net.*"
    resource: "**"
    decision: deny
    priority: 50

  - name: "deny-execution"
    description: "Deny all command execution"
    action: "exec.*"
    resource: "**"
    decision: deny
    priority: 50
`

	case "moderate":
		networkPolicy := "deny"
		if cfg.enableNetworkAccess {
			networkPolicy = "allow"
		}

		policyContent = fmt.Sprintf(`version: "1"

# Moderate Security Mode
# - Workspace and project files allowed
# - Limited network access: %v
# - No command execution

policies:
  - name: "allow-workspace-operations"
    description: "Allow all file operations within workspace"
    action: "files.*"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "deny-parent-access"
    description: "Deny access to parent directories"
    action: "files.*"
    resource: "../**"
    decision: deny
    priority: 100

  - name: "network-policy"
    description: "Network access policy"
    action: "net.*"
    resource: "**"
    decision: %s
    priority: 50

  - name: "deny-execution"
    description: "Deny command execution"
    action: "exec.*"
    resource: "**"
    decision: deny
    priority: 50
`, cfg.enableNetworkAccess, networkPolicy)

	case "permissive":
		execPolicy := "deny"
		if cfg.enableExecution {
			execPolicy = "allow"
		}

		policyContent = fmt.Sprintf(`version: "1"

# Permissive Security Mode
# - Home directory access allowed
# - Network access enabled
# - Command execution: %v

policies:
  - name: "allow-workspace-operations"
    description: "Allow all file operations within workspace"
    action: "files.*"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "allow-home-read"
    description: "Allow reading from home directory"
    action: "files.read"
    resource: "$HOME/**"
    decision: allow
    priority: 5

  - name: "allow-network"
    description: "Allow network access"
    action: "net.*"
    resource: "**"
    decision: allow
    priority: 50

  - name: "execution-policy"
    description: "Command execution policy"
    action: "exec.*"
    resource: "**"
    decision: %s
    priority: 50
`, cfg.enableExecution, execPolicy)

	case "custom":
		// Build custom policy based on user selections
		var policies []string

		// Add file access for allowed paths
		for i, path := range cfg.allowedPaths {
			policies = append(policies, fmt.Sprintf(`  - name: "allow-path-%d"
    description: "Allow access to %s"
    action: "files.*"
    resource: "%s/**"
    decision: allow
    priority: 10`, i+1, path, path))
		}

		// Network policy
		networkDecision := "deny"
		if cfg.enableNetworkAccess {
			networkDecision = "allow"
		}
		policies = append(policies, fmt.Sprintf(`  - name: "network-policy"
    description: "Network access policy"
    action: "net.*"
    resource: "**"
    decision: %s
    priority: 50`, networkDecision))

		// Execution policy
		execDecision := "deny"
		if cfg.enableExecution {
			execDecision = "allow"
		}
		policies = append(policies, fmt.Sprintf(`  - name: "execution-policy"
    description: "Command execution policy"
    action: "exec.*"
    resource: "**"
    decision: %s
    priority: 50`, execDecision))

		policyContent = fmt.Sprintf(`version: "1"

# Custom Security Mode

policies:
%s
`, strings.Join(policies, "\n\n"))
	}

	return os.WriteFile(path, []byte(policyContent), 0644)
}

func createAgentsFile(path string, cfg setupConfig) error {
	agentsContent := `# SoulGate Consolidated Agents Configuration
version: "1"

agents:
`

	if cfg.enableTestAgent {
		agentsContent += fmt.Sprintf(`
  test-quality:
    enabled: true
    config:
      coverage_target: %d
      mode: "auto"
`, cfg.coverageTarget)
	}

	if cfg.enableDocsAgent {
		agentsContent += `
  docs-api:
    enabled: true
    config:
      api_spec_format: "openapi"
      docs_coverage_target: 90
`
	}

	if cfg.enablePMAgent {
		agentsContent += fmt.Sprintf(`
  project-management:
    enabled: true
    config:
      assignment_strategy: "%s"
      max_tasks_per_dev: 3
      sprint_duration_days: 14
`, cfg.assignmentStrategy)
	}

	return os.WriteFile(path, []byte(agentsContent), 0644)
}
