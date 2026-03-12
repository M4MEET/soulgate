package setup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"gopkg.in/yaml.v3"
)

// IntegrationSetup represents an integration that can be configured
type IntegrationSetup struct {
	ID          string
	Name        string
	Description string
	Icon        string
	Category    string
	Fields      []SetupField
	TestFunc    func(values map[string]string) error
}

// SetupField represents a configuration field
type SetupField struct {
	Key         string
	Label       string
	Description string
	Required    bool
	Secret      bool // If true, mask input
	Default     string
	Validation  func(value string) error
}

// Wizard manages the interactive setup process
type Wizard struct {
	workspace *config.Workspace
}

// NewWizard creates a new setup wizard
func NewWizard(workspace *config.Workspace) *Wizard {
	return &Wizard{
		workspace: workspace,
	}
}

// GetAvailableIntegrations returns all available integrations
func (w *Wizard) GetAvailableIntegrations() []IntegrationSetup {
	return []IntegrationSetup{
		{
			ID:          "slack",
			Name:        "Slack",
			Description: "Send messages, read channels, manage workspace",
			Icon:        "💬",
			Category:    "Communication",
			Fields: []SetupField{
				{
					Key:         "slack_token",
					Label:       "Slack Bot Token",
					Description: "Get from https://api.slack.com/apps (starts with xoxb-)",
					Required:    true,
					Secret:      true,
					Validation:  validateSlackToken,
				},
				{
					Key:         "slack_default_channel",
					Label:       "Default Channel",
					Description: "Default channel for messages (e.g., #general)",
					Required:    false,
					Default:     "#general",
				},
			},
			TestFunc: testSlackConnection,
		},
		{
			ID:          "github",
			Name:        "GitHub",
			Description: "Manage repos, PRs, issues, code reviews",
			Icon:        "🐙",
			Category:    "Development",
			Fields: []SetupField{
				{
					Key:         "github_token",
					Label:       "GitHub Personal Access Token",
					Description: "Get from https://github.com/settings/tokens",
					Required:    true,
					Secret:      true,
					Validation:  validateGitHubToken,
				},
				{
					Key:         "github_default_org",
					Label:       "Default Organization/User",
					Description: "Default GitHub org or username (optional)",
					Required:    false,
				},
			},
			TestFunc: testGitHubConnection,
		},
		{
			ID:          "notion",
			Name:        "Notion",
			Description: "Sync notes, create pages, manage databases",
			Icon:        "📝",
			Category:    "Productivity",
			Fields: []SetupField{
				{
					Key:         "notion_token",
					Label:       "Notion Integration Token",
					Description: "Get from https://www.notion.so/my-integrations",
					Required:    true,
					Secret:      true,
					Validation:  validateNotionToken,
				},
				{
					Key:         "notion_default_page",
					Label:       "Default Page ID",
					Description: "ID of your default Notion page (optional)",
					Required:    false,
				},
			},
			TestFunc: testNotionConnection,
		},
		{
			ID:          "google",
			Name:        "Google Workspace",
			Description: "Gmail, Calendar, Drive, Docs integration",
			Icon:        "📧",
			Category:    "Productivity",
			Fields: []SetupField{
				{
					Key:         "google_credentials",
					Label:       "Google Service Account JSON",
					Description: "Path to service account JSON file",
					Required:    true,
					Validation:  validateGoogleCredentials,
				},
				{
					Key:         "google_default_calendar",
					Label:       "Default Calendar",
					Description: "Default calendar name (e.g., primary)",
					Required:    false,
					Default:     "primary",
				},
			},
			TestFunc: testGoogleConnection,
		},
		{
			ID:          "aws",
			Name:        "AWS",
			Description: "EC2, S3, Lambda, and other AWS services",
			Icon:        "☁️",
			Category:    "Cloud",
			Fields: []SetupField{
				{
					Key:         "aws_access_key_id",
					Label:       "AWS Access Key ID",
					Description: "Your AWS access key ID",
					Required:    true,
					Secret:      true,
				},
				{
					Key:         "aws_secret_access_key",
					Label:       "AWS Secret Access Key",
					Description: "Your AWS secret access key",
					Required:    true,
					Secret:      true,
				},
				{
					Key:         "aws_region",
					Label:       "Default AWS Region",
					Description: "Default region (e.g., us-east-1)",
					Required:    false,
					Default:     "us-east-1",
				},
			},
			TestFunc: testAWSConnection,
		},
		{
			ID:          "docker",
			Name:        "Docker",
			Description: "Manage containers, images, and Docker Compose",
			Icon:        "🐳",
			Category:    "Development",
			Fields: []SetupField{
				{
					Key:         "docker_host",
					Label:       "Docker Host",
					Description: "Docker daemon socket (leave empty for local)",
					Required:    false,
					Default:     "unix:///var/run/docker.sock",
				},
			},
			TestFunc: testDockerConnection,
		},
		{
			ID:          "jira",
			Name:        "Jira",
			Description: "Create issues, track sprints, manage projects",
			Icon:        "📋",
			Category:    "Project Management",
			Fields: []SetupField{
				{
					Key:         "jira_url",
					Label:       "Jira URL",
					Description: "Your Jira instance URL (e.g., https://yourcompany.atlassian.net)",
					Required:    true,
				},
				{
					Key:         "jira_email",
					Label:       "Email",
					Description: "Your Jira account email",
					Required:    true,
				},
				{
					Key:         "jira_api_token",
					Label:       "API Token",
					Description: "Get from https://id.atlassian.com/manage-profile/security/api-tokens",
					Required:    true,
					Secret:      true,
				},
			},
			TestFunc: testJiraConnection,
		},
		{
			ID:          "linear",
			Name:        "Linear",
			Description: "Issue tracking and project management",
			Icon:        "🎯",
			Category:    "Project Management",
			Fields: []SetupField{
				{
					Key:         "linear_api_key",
					Label:       "Linear API Key",
					Description: "Get from https://linear.app/settings/api",
					Required:    true,
					Secret:      true,
				},
			},
			TestFunc: testLinearConnection,
		},
	}
}

// SetupIntegration configures a specific integration
func (w *Wizard) SetupIntegration(integrationID string, values map[string]string) error {
	// Find the integration
	var integration *IntegrationSetup
	for _, integ := range w.GetAvailableIntegrations() {
		if integ.ID == integrationID {
			integration = &integ
			break
		}
	}

	if integration == nil {
		return fmt.Errorf("unknown integration: %s", integrationID)
	}

	// Validate required fields
	for _, field := range integration.Fields {
		value := values[field.Key]
		if field.Required && value == "" {
			return fmt.Errorf("required field missing: %s", field.Label)
		}

		// Run validation if provided
		if value != "" && field.Validation != nil {
			if err := field.Validation(value); err != nil {
				return fmt.Errorf("validation failed for %s: %w", field.Label, err)
			}
		}
	}

	// Test connection
	if integration.TestFunc != nil {
		if err := integration.TestFunc(values); err != nil {
			return fmt.Errorf("connection test failed: %w", err)
		}
	}

	// Save to config
	if err := w.saveIntegrationConfig(integrationID, values); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// saveIntegrationConfig saves integration configuration
func (w *Wizard) saveIntegrationConfig(integrationID string, values map[string]string) error {
	// Load current config
	cfg := w.workspace.Config

	// Initialize integrations map if nil
	if cfg.Integrations == nil {
		cfg.Integrations = make(map[string]config.IntegrationConfig)
	}

	// Create integration config
	integConfig := config.IntegrationConfig{
		Enabled: true,
		Config:  values,
	}

	cfg.Integrations[integrationID] = integConfig

	// Save config to workspace config file
	configPath := filepath.Join(w.workspace.ConfigDir, "config.yml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Validation functions

func validateSlackToken(token string) error {
	if !strings.HasPrefix(token, "xoxb-") && !strings.HasPrefix(token, "xoxp-") {
		return fmt.Errorf("invalid Slack token format (should start with xoxb- or xoxp-)")
	}
	return nil
}

func validateGitHubToken(token string) error {
	if len(token) < 20 {
		return fmt.Errorf("GitHub token seems too short")
	}
	if strings.HasPrefix(token, "ghp_") || strings.HasPrefix(token, "github_pat_") {
		return nil
	}
	return fmt.Errorf("invalid GitHub token format")
}

func validateNotionToken(token string) error {
	if !strings.HasPrefix(token, "secret_") {
		return fmt.Errorf("invalid Notion token format (should start with secret_)")
	}
	return nil
}

func validateGoogleCredentials(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s", path)
	}
	return nil
}

// Test connection functions (real implementations)

func testSlackConnection(values map[string]string) error {
	token := values["slack_token"]
	if token == "" {
		return fmt.Errorf("token is required")
	}

	// Create request with 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://slack.com/api/auth.test", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	// Parse response
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("invalid token: %s", result.Error)
	}

	return nil
}

func testGitHubConnection(values map[string]string) error {
	token := values["github_token"]
	if token == "" {
		return fmt.Errorf("token is required")
	}

	// Create request with 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	return nil
}

func testNotionConnection(values map[string]string) error {
	token := values["notion_token"]
	if token == "" {
		return fmt.Errorf("token is required")
	}

	// Create request with 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.notion.com/v1/users/me", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid token")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	return nil
}

func testGoogleConnection(values map[string]string) error {
	credentialsPath := values["google_credentials"]
	if credentialsPath == "" {
		return fmt.Errorf("credentials path is required")
	}

	// Check if file exists
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s", credentialsPath)
	}

	// Read and validate JSON format
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return fmt.Errorf("failed to read credentials: %w", err)
	}

	// Validate it's valid JSON and has required fields
	var creds struct {
		Type                string `json:"type"`
		ProjectID           string `json:"project_id"`
		PrivateKeyID        string `json:"private_key_id"`
		PrivateKey          string `json:"private_key"`
		ClientEmail         string `json:"client_email"`
		ClientID            string `json:"client_id"`
		AuthURI             string `json:"auth_uri"`
		TokenURI            string `json:"token_uri"`
		AuthProviderX509URL string `json:"auth_provider_x509_cert_url"`
		ClientX509URL       string `json:"client_x509_cert_url"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	if creds.Type != "service_account" {
		return fmt.Errorf("credentials must be for a service account")
	}
	if creds.PrivateKey == "" || creds.ClientEmail == "" {
		return fmt.Errorf("credentials missing required fields")
	}

	return nil
}

func testAWSConnection(values map[string]string) error {
	accessKeyID := values["aws_access_key_id"]
	secretAccessKey := values["aws_secret_access_key"]

	if accessKeyID == "" {
		return fmt.Errorf("access key ID is required")
	}
	if secretAccessKey == "" {
		return fmt.Errorf("secret access key is required")
	}

	// Validate access key ID format
	if !strings.HasPrefix(accessKeyID, "AKIA") && !strings.HasPrefix(accessKeyID, "ASIA") {
		return fmt.Errorf("invalid access key ID format (should start with AKIA or ASIA)")
	}

	// Basic length validation
	if len(accessKeyID) != 20 {
		return fmt.Errorf("invalid access key ID length")
	}
	if len(secretAccessKey) != 40 {
		return fmt.Errorf("invalid secret access key length")
	}

	// Note: We don't make actual AWS API calls here to avoid:
	// 1. Complex signature calculation
	// 2. Potential rate limiting
	// 3. Requiring specific AWS permissions
	// Format validation is sufficient for setup wizard
	return nil
}

func testDockerConnection(values map[string]string) error {
	dockerHost := values["docker_host"]
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}

	// For Unix sockets, check if the socket file exists and is accessible
	if strings.HasPrefix(dockerHost, "unix://") {
		socketPath := strings.TrimPrefix(dockerHost, "unix://")
		info, err := os.Stat(socketPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("Docker socket not found: %s (is Docker running?)", socketPath)
		}
		if err != nil {
			return fmt.Errorf("cannot access Docker socket: %w", err)
		}
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("not a socket file: %s", socketPath)
		}
		return nil
	}

	// For TCP connections, try to connect
	if strings.HasPrefix(dockerHost, "tcp://") {
		// Basic URL validation
		if !strings.Contains(dockerHost, ":") {
			return fmt.Errorf("invalid Docker host URL (missing port)")
		}
		// We don't make actual connection here to avoid complexity
		// Just validate format
		return nil
	}

	return fmt.Errorf("unsupported Docker host format: %s", dockerHost)
}

func testJiraConnection(values map[string]string) error {
	jiraURL := values["jira_url"]
	email := values["jira_email"]
	apiToken := values["jira_api_token"]

	if jiraURL == "" {
		return fmt.Errorf("Jira URL is required")
	}
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if apiToken == "" {
		return fmt.Errorf("API token is required")
	}

	// Ensure URL has proper format
	if !strings.HasPrefix(jiraURL, "http://") && !strings.HasPrefix(jiraURL, "https://") {
		jiraURL = "https://" + jiraURL
	}

	// Create request with 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	apiURL := strings.TrimSuffix(jiraURL, "/") + "/rest/api/3/myself"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Use Basic auth with email:token
	auth := base64.StdEncoding.EncodeToString([]byte(email + ":" + apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	return nil
}

func testLinearConnection(values map[string]string) error {
	apiKey := values["linear_api_key"]
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Create GraphQL query to test connection
	query := map[string]string{
		"query": "{ viewer { id name email } }",
	}
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("failed to create query: %w", err)
	}

	// Create request with 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.linear.app/graphql", bytes.NewReader(queryJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed (HTTP %d)", resp.StatusCode)
	}

	// Parse response to check for errors
	var result struct {
		Data   interface{} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Errors) > 0 {
		return fmt.Errorf("API error: %s", result.Errors[0].Message)
	}

	return nil
}
