package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/integrations"
)

// Integration implements Slack integration
type Integration struct {
	token      string
	httpClient *http.Client
	configured bool
}

// New creates a new Slack integration
func New() *Integration {
	return &Integration{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the integration name
func (i *Integration) Name() string {
	return "slack"
}

// Description returns what this integration does
func (i *Integration) Description() string {
	return "Slack integration - send messages, list channels, manage workspace"
}

// RequiredConfig returns required configuration fields
func (i *Integration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "token",
			Description: "Slack Bot Token (starts with xoxb-)",
			Required:    true,
			Secret:      true,
			Example:     "xoxb-your-token-here",
		},
	}
}

// Setup configures the integration
func (i *Integration) Setup(ctx context.Context, config map[string]string) error {
	token, ok := config["token"]
	if !ok || token == "" {
		return fmt.Errorf("Slack token is required")
	}

	i.token = token
	i.configured = true

	return nil
}

// GetTools returns available Slack tools
func (i *Integration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "slack_send_message",
			Description: "Send a message to a Slack channel",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"channel": {
						"type": "string",
						"description": "Channel name or ID (e.g., 'general', 'C1234567890')"
					},
					"text": {
						"type": "string",
						"description": "Message text"
					}
				},
				"required": ["channel", "text"]
			}`),
			Handler: i.handleSendMessage,
		},
		{
			Name:        "slack_list_channels",
			Description: "List all channels in the workspace",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleListChannels,
		},
		{
			Name:        "slack_get_channel_history",
			Description: "Get recent messages from a channel",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"channel": {
						"type": "string",
						"description": "Channel ID"
					},
					"limit": {
						"type": "integer",
						"description": "Number of messages to retrieve (default: 10)",
						"default": 10
					}
				},
				"required": ["channel"]
			}`),
			Handler: i.handleGetChannelHistory,
		},
		{
			Name:        "slack_list_users",
			Description: "List users in the workspace",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleListUsers,
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

func (i *Integration) handleSendMessage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Channel string `json:"channel"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	data := url.Values{}
	data.Set("channel", params.Channel)
	data.Set("text", params.Text)

	body, err := i.makeRequest(ctx, "POST", "https://slack.com/api/chat.postMessage", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleListChannels(ctx context.Context, input json.RawMessage) (string, error) {
	body, err := i.makeRequest(ctx, "GET", "https://slack.com/api/conversations.list", nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleGetChannelHistory(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Channel string `json:"channel"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	if params.Limit == 0 {
		params.Limit = 10
	}

	apiURL := fmt.Sprintf("https://slack.com/api/conversations.history?channel=%s&limit=%d", params.Channel, params.Limit)
	body, err := i.makeRequest(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleListUsers(ctx context.Context, input json.RawMessage) (string, error) {
	body, err := i.makeRequest(ctx, "GET", "https://slack.com/api/users.list", nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// makeRequest makes an authenticated request to Slack API
func (i *Integration) makeRequest(ctx context.Context, method, apiURL string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, apiURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+i.token)
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
		return nil, fmt.Errorf("Slack API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
