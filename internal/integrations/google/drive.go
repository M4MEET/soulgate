package google

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

// DriveIntegration implements Google Drive integration
type DriveIntegration struct {
	accessToken string
	httpClient  *http.Client
	configured  bool
}

// NewDrive creates a new Google Drive integration
func NewDrive() *DriveIntegration {
	return &DriveIntegration{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the integration name
func (i *DriveIntegration) Name() string {
	return "google_drive"
}

// Description returns what this integration does
func (i *DriveIntegration) Description() string {
	return "Google Drive - manage documents, sheets, slides, and files"
}

// RequiredConfig returns required configuration fields
func (i *DriveIntegration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "access_token",
			Description: "Google OAuth Access Token",
			Required:    true,
			Secret:      true,
			Example:     "ya29.a0...",
		},
	}
}

// Setup configures the integration
func (i *DriveIntegration) Setup(ctx context.Context, config map[string]string) error {
	token, ok := config["access_token"]
	if !ok || token == "" {
		return fmt.Errorf("Google access token is required")
	}

	i.accessToken = token
	i.configured = true

	return nil
}

// GetTools returns available Google Drive tools
func (i *DriveIntegration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "drive_list_files",
			Description: "List files in Google Drive",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query (optional, e.g., 'type:document')"
					},
					"limit": {
						"type": "integer",
						"description": "Max number of files to return (default: 10)"
					}
				}
			}`),
			Handler: i.handleListFiles,
		},
		{
			Name:        "drive_get_file",
			Description: "Get file metadata from Google Drive",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file_id": {
						"type": "string",
						"description": "Google Drive file ID"
					}
				},
				"required": ["file_id"]
			}`),
			Handler: i.handleGetFile,
		},
		{
			Name:        "drive_download_file",
			Description: "Download a file from Google Drive",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file_id": {
						"type": "string",
						"description": "Google Drive file ID"
					}
				},
				"required": ["file_id"]
			}`),
			Handler: i.handleDownloadFile,
		},
		{
			Name:        "drive_create_doc",
			Description: "Create a new Google Doc",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title": {
						"type": "string",
						"description": "Document title"
					},
					"content": {
						"type": "string",
						"description": "Initial document content (optional)"
					}
				},
				"required": ["title"]
			}`),
			Handler: i.handleCreateDoc,
		},
		{
			Name:        "drive_share_file",
			Description: "Share a Google Drive file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file_id": {
						"type": "string",
						"description": "File ID to share"
					},
					"email": {
						"type": "string",
						"description": "Email address to share with"
					},
					"role": {
						"type": "string",
						"description": "Permission role (reader, writer, commenter)",
						"enum": ["reader", "writer", "commenter"]
					}
				},
				"required": ["file_id", "email", "role"]
			}`),
			Handler: i.handleShareFile,
		},
	}
}

// IsConfigured returns whether the integration is configured
func (i *DriveIntegration) IsConfigured() bool {
	return i.configured
}

// Close cleans up resources
func (i *DriveIntegration) Close() error {
	i.httpClient.CloseIdleConnections()
	return nil
}

// Tool handlers

func (i *DriveIntegration) handleListFiles(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	if params.Limit == 0 {
		params.Limit = 10
	}

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files?pageSize=%d", params.Limit)
	if params.Query != "" {
		url += "&q=" + params.Query
	}

	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *DriveIntegration) handleGetFile(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s", params.FileID)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *DriveIntegration) handleDownloadFile(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s?alt=media", params.FileID)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *DriveIntegration) handleCreateDoc(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	// Create file metadata
	metadata := map[string]interface{}{
		"name":     params.Title,
		"mimeType": "application/vnd.google-apps.document",
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	body, err := i.makeRequest(ctx, "POST", "https://www.googleapis.com/drive/v3/files", metadataBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *DriveIntegration) handleShareFile(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		FileID string `json:"file_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	permission := map[string]string{
		"type":         "user",
		"role":         params.Role,
		"emailAddress": params.Email,
	}

	permissionBytes, err := json.Marshal(permission)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://www.googleapis.com/drive/v3/files/%s/permissions", params.FileID)
	body, err := i.makeRequest(ctx, "POST", url, permissionBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// makeRequest makes an authenticated request to Google Drive API
func (i *DriveIntegration) makeRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+i.accessToken)
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
		return nil, fmt.Errorf("Google Drive API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
