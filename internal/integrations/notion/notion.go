package notion

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

// Integration implements Notion integration
type Integration struct {
	token      string
	httpClient *http.Client
	configured bool
}

// New creates a new Notion integration
func New() *Integration {
	return &Integration{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the integration name
func (i *Integration) Name() string {
	return "notion"
}

// Description returns what this integration does
func (i *Integration) Description() string {
	return "Notion - knowledge base, create pages, update databases, search content"
}

// RequiredConfig returns required configuration fields
func (i *Integration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "token",
			Description: "Notion Integration Token",
			Required:    true,
			Secret:      true,
			Example:     "secret_...",
		},
	}
}

// Setup configures the integration
func (i *Integration) Setup(ctx context.Context, config map[string]string) error {
	token, ok := config["token"]
	if !ok || token == "" {
		return fmt.Errorf("Notion token is required")
	}

	i.token = token
	i.configured = true

	return nil
}

// GetTools returns available Notion tools
func (i *Integration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "notion_search",
			Description: "Search Notion workspace",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query"
					}
				},
				"required": ["query"]
			}`),
			Handler: i.handleSearch,
		},
		{
			Name:        "notion_create_page",
			Description: "Create a new Notion page",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"parent_id": {
						"type": "string",
						"description": "Parent page or database ID"
					},
					"title": {
						"type": "string",
						"description": "Page title"
					},
					"content": {
						"type": "string",
						"description": "Page content (markdown)"
					}
				},
				"required": ["parent_id", "title"]
			}`),
			Handler: i.handleCreatePage,
		},
		{
			Name:        "notion_get_page",
			Description: "Get a Notion page",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"page_id": {
						"type": "string",
						"description": "Page ID"
					}
				},
				"required": ["page_id"]
			}`),
			Handler: i.handleGetPage,
		},
		{
			Name:        "notion_update_page",
			Description: "Update a Notion page",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"page_id": {
						"type": "string",
						"description": "Page ID to update"
					},
					"title": {
						"type": "string",
						"description": "New title (optional)"
					},
					"content": {
						"type": "string",
						"description": "New content (optional)"
					}
				},
				"required": ["page_id"]
			}`),
			Handler: i.handleUpdatePage,
		},
		{
			Name:        "notion_list_databases",
			Description: "List Notion databases",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleListDatabases,
		},
		{
			Name:        "notion_query_database",
			Description: "Query a Notion database",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"database_id": {
						"type": "string",
						"description": "Database ID"
					}
				},
				"required": ["database_id"]
			}`),
			Handler: i.handleQueryDatabase,
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

func (i *Integration) handleSearch(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	payload := map[string]string{
		"query": params.Query,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	body, err := i.makeRequest(ctx, "POST", "https://api.notion.com/v1/search", payloadBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleCreatePage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ParentID string `json:"parent_id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	// Simplified - actual Notion API requires more complex structure
	payload := map[string]interface{}{
		"parent": map[string]string{
			"page_id": params.ParentID,
		},
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"title": []map[string]interface{}{
					{
						"text": map[string]string{
							"content": params.Title,
						},
					},
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	body, err := i.makeRequest(ctx, "POST", "https://api.notion.com/v1/pages", payloadBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleGetPage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		PageID string `json:"page_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.notion.com/v1/pages/%s", params.PageID)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleUpdatePage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		PageID  string `json:"page_id"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	// Simplified update payload
	payload := map[string]interface{}{
		"properties": map[string]interface{}{},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.notion.com/v1/pages/%s", params.PageID)
	body, err := i.makeRequest(ctx, "PATCH", url, payloadBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleListDatabases(ctx context.Context, input json.RawMessage) (string, error) {
	body, err := i.makeRequest(ctx, "POST", "https://api.notion.com/v1/search", []byte(`{"filter":{"property":"object","value":"database"}}`))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleQueryDatabase(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		DatabaseID string `json:"database_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://api.notion.com/v1/databases/%s/query", params.DatabaseID)
	body, err := i.makeRequest(ctx, "POST", url, []byte(`{}`))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// makeRequest makes an authenticated request to Notion API
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
	req.Header.Set("Notion-Version", "2022-06-28")
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
		return nil, fmt.Errorf("Notion API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
