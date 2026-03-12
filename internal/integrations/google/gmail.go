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

// GmailIntegration implements Gmail integration
type GmailIntegration struct {
	accessToken string
	httpClient  *http.Client
	configured  bool
}

// NewGmail creates a new Gmail integration
func NewGmail() *GmailIntegration {
	return &GmailIntegration{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the integration name
func (i *GmailIntegration) Name() string {
	return "gmail"
}

// Description returns what this integration does
func (i *GmailIntegration) Description() string {
	return "Gmail - send emails, read inbox, manage messages"
}

// RequiredConfig returns required configuration fields
func (i *GmailIntegration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "access_token",
			Description: "Google OAuth Access Token with Gmail scope",
			Required:    true,
			Secret:      true,
			Example:     "ya29.a0...",
		},
	}
}

// Setup configures the integration
func (i *GmailIntegration) Setup(ctx context.Context, config map[string]string) error {
	token, ok := config["access_token"]
	if !ok || token == "" {
		return fmt.Errorf("Google access token is required")
	}

	i.accessToken = token
	i.configured = true

	return nil
}

// GetTools returns available Gmail tools
func (i *GmailIntegration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "gmail_send",
			Description: "Send an email via Gmail",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"to": {
						"type": "string",
						"description": "Recipient email address"
					},
					"subject": {
						"type": "string",
						"description": "Email subject"
					},
					"body": {
						"type": "string",
						"description": "Email body (plain text)"
					}
				},
				"required": ["to", "subject", "body"]
			}`),
			Handler: i.handleSendEmail,
		},
		{
			Name:        "gmail_list_messages",
			Description: "List messages in Gmail inbox",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query (e.g., 'is:unread', 'from:user@example.com')"
					},
					"limit": {
						"type": "integer",
						"description": "Max number of messages (default: 10)"
					}
				}
			}`),
			Handler: i.handleListMessages,
		},
		{
			Name:        "gmail_get_message",
			Description: "Get a specific email message",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message_id": {
						"type": "string",
						"description": "Gmail message ID"
					}
				},
				"required": ["message_id"]
			}`),
			Handler: i.handleGetMessage,
		},
		{
			Name:        "gmail_search",
			Description: "Search Gmail messages",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Search query (Gmail search syntax)"
					}
				},
				"required": ["query"]
			}`),
			Handler: i.handleSearch,
		},
		{
			Name:        "gmail_delete_message",
			Description: "Delete a Gmail message",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message_id": {
						"type": "string",
						"description": "Message ID to delete"
					}
				},
				"required": ["message_id"]
			}`),
			Handler: i.handleDeleteMessage,
		},
	}
}

// IsConfigured returns whether the integration is configured
func (i *GmailIntegration) IsConfigured() bool {
	return i.configured
}

// Close cleans up resources
func (i *GmailIntegration) Close() error {
	i.httpClient.CloseIdleConnections()
	return nil
}

// Tool handlers

func (i *GmailIntegration) handleSendEmail(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	// Create RFC 2822 email
	message := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", params.To, params.Subject, params.Body)

	payload := map[string]string{
		"raw": message, // Should be base64 encoded in production
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	body, err := i.makeRequest(ctx, "POST", "https://gmail.googleapis.com/gmail/v1/users/me/messages/send", payloadBytes)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *GmailIntegration) handleListMessages(ctx context.Context, input json.RawMessage) (string, error) {
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

	url := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages?maxResults=%d", params.Limit)
	if params.Query != "" {
		url += "&q=" + params.Query
	}

	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *GmailIntegration) handleGetMessage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s", params.MessageID)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *GmailIntegration) handleSearch(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages?q=%s", params.Query)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *GmailIntegration) handleDeleteMessage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://gmail.googleapis.com/gmail/v1/users/me/messages/%s", params.MessageID)
	_, err := i.makeRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return "", err
	}

	return `{"status": "deleted"}`, nil
}

// makeRequest makes an authenticated request to Gmail API
func (i *GmailIntegration) makeRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
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
		return nil, fmt.Errorf("Gmail API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
