package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/integrations"
)

// Integration implements GitHub integration
type Integration struct {
	token      string
	httpClient *http.Client
	configured bool
}

// New creates a new GitHub integration
func New() *Integration {
	return &Integration{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the integration name
func (i *Integration) Name() string {
	return "github"
}

// Description returns what this integration does
func (i *Integration) Description() string {
	return "GitHub integration - manage repos, issues, PRs, and more"
}

// RequiredConfig returns required configuration fields
func (i *Integration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "token",
			Description: "GitHub Personal Access Token",
			Required:    true,
			Secret:      true,
			Example:     "ghp_xxxxxxxxxxxxxxxxxxxx",
		},
	}
}

// Setup configures the integration
func (i *Integration) Setup(ctx context.Context, config map[string]string) error {
	token, ok := config["token"]
	if !ok || token == "" {
		return fmt.Errorf("GitHub token is required")
	}

	i.token = token
	i.configured = true

	return nil
}

// GetTools returns available GitHub tools
func (i *Integration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "github_list_repos",
			Description: "List repositories for a user or organization",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Username or organization name"
					}
				},
				"required": ["owner"]
			}`),
			Handler: i.handleListRepos,
		},
		{
			Name:        "github_get_repo",
			Description: "Get information about a repository",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner (user or org)"
					},
					"repo": {
						"type": "string",
						"description": "Repository name"
					}
				},
				"required": ["owner", "repo"]
			}`),
			Handler: i.handleGetRepo,
		},
		{
			Name:        "github_list_issues",
			Description: "List issues for a repository",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner"
					},
					"repo": {
						"type": "string",
						"description": "Repository name"
					},
					"state": {
						"type": "string",
						"description": "Issue state (open, closed, all)",
						"enum": ["open", "closed", "all"]
					}
				},
				"required": ["owner", "repo"]
			}`),
			Handler: i.handleListIssues,
		},
		{
			Name:        "github_create_issue",
			Description: "Create a new issue in a repository",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner"
					},
					"repo": {
						"type": "string",
						"description": "Repository name"
					},
					"title": {
						"type": "string",
						"description": "Issue title"
					},
					"body": {
						"type": "string",
						"description": "Issue body/description"
					}
				},
				"required": ["owner", "repo", "title"]
			}`),
			Handler: i.handleCreateIssue,
		},
		{
			Name:        "github_list_prs",
			Description: "List pull requests for a repository",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"owner": {
						"type": "string",
						"description": "Repository owner"
					},
					"repo": {
						"type": "string",
						"description": "Repository name"
					},
					"state": {
						"type": "string",
						"description": "PR state (open, closed, all)",
						"enum": ["open", "closed", "all"]
					}
				},
				"required": ["owner", "repo"]
			}`),
			Handler: i.handleListPRs,
		},
	}
}

// IsConfigured returns whether the integration is configured
func (i *Integration) IsConfigured() bool {
	return i.configured
}

// Close cleans up resources
func (i *Integration) Close() error {
	i.httpClient.CloseIdleConnections()
	return nil
}

// Tool handlers

func (i *Integration) handleListRepos(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Owner string `json:"owner"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/users/%s/repos", params.Owner)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleGetRepo(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", params.Owner, params.Repo)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleListIssues(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	if params.State == "" {
		params.State = "open"
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=%s", params.Owner, params.Repo, params.State)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleCreateIssue(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", params.Owner, params.Repo)

	payload := map[string]string{
		"title": params.Title,
		"body":  params.Body,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	body, err := i.makeRequest(ctx, "POST", url, payloadBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleListPRs(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	if params.State == "" {
		params.State = "open"
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=%s", params.Owner, params.Repo, params.State)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// makeRequest makes an authenticated request to GitHub API
func (i *Integration) makeRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+i.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
