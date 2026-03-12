package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/integrations"
)

// Integration implements Docker integration
type Integration struct {
	socketPath string
	httpClient *http.Client
	configured bool
}

// New creates a new Docker integration
func New() *Integration {
	return &Integration{
		socketPath: "/var/run/docker.sock",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial("unix", "/var/run/docker.sock")
				},
			},
		},
	}
}

// Name returns the integration name
func (i *Integration) Name() string {
	return "docker"
}

// Description returns what this integration does
func (i *Integration) Description() string {
	return "Docker - manage containers, images, networks, and volumes"
}

// RequiredConfig returns required configuration fields
func (i *Integration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "socket_path",
			Description: "Docker socket path",
			Required:    false,
			Default:     "/var/run/docker.sock",
			Example:     "/var/run/docker.sock",
		},
	}
}

// Setup configures the integration
func (i *Integration) Setup(ctx context.Context, config map[string]string) error {
	socketPath := config["socket_path"]
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	i.socketPath = socketPath
	i.configured = true

	return nil
}

// GetTools returns available Docker tools
func (i *Integration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "docker_list_containers",
			Description: "List Docker containers",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"all": {
						"type": "boolean",
						"description": "Show all containers (default: only running)"
					}
				}
			}`),
			Handler: i.handleListContainers,
		},
		{
			Name:        "docker_list_images",
			Description: "List Docker images",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleListImages,
		},
		{
			Name:        "docker_start_container",
			Description: "Start a Docker container",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"container_id": {
						"type": "string",
						"description": "Container ID or name"
					}
				},
				"required": ["container_id"]
			}`),
			Handler: i.handleStartContainer,
		},
		{
			Name:        "docker_stop_container",
			Description: "Stop a Docker container",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"container_id": {
						"type": "string",
						"description": "Container ID or name"
					}
				},
				"required": ["container_id"]
			}`),
			Handler: i.handleStopContainer,
		},
		{
			Name:        "docker_container_logs",
			Description: "Get container logs",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"container_id": {
						"type": "string",
						"description": "Container ID or name"
					},
					"tail": {
						"type": "integer",
						"description": "Number of lines to show (default: 100)"
					}
				},
				"required": ["container_id"]
			}`),
			Handler: i.handleContainerLogs,
		},
		{
			Name:        "docker_inspect_container",
			Description: "Inspect a container",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"container_id": {
						"type": "string",
						"description": "Container ID or name"
					}
				},
				"required": ["container_id"]
			}`),
			Handler: i.handleInspectContainer,
		},
		{
			Name:        "docker_pull_image",
			Description: "Pull a Docker image",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"image": {
						"type": "string",
						"description": "Image name (e.g., 'nginx:latest')"
					}
				},
				"required": ["image"]
			}`),
			Handler: i.handlePullImage,
		},
		{
			Name:        "docker_remove_container",
			Description: "Remove a Docker container",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"container_id": {
						"type": "string",
						"description": "Container ID or name"
					},
					"force": {
						"type": "boolean",
						"description": "Force removal (default: false)"
					}
				},
				"required": ["container_id"]
			}`),
			Handler: i.handleRemoveContainer,
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

func (i *Integration) handleListContainers(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		All bool `json:"all"`
	}
	json.Unmarshal(input, &params)

	url := "http://localhost/containers/json"
	if params.All {
		url += "?all=true"
	}

	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleListImages(ctx context.Context, input json.RawMessage) (string, error) {
	body, err := i.makeRequest(ctx, "GET", "http://localhost/images/json", nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleStartContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://localhost/containers/%s/start", params.ContainerID)
	_, err := i.makeRequest(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}

	return `{"status": "started"}`, nil
}

func (i *Integration) handleStopContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://localhost/containers/%s/stop", params.ContainerID)
	_, err := i.makeRequest(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}

	return `{"status": "stopped"}`, nil
}

func (i *Integration) handleContainerLogs(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ContainerID string `json:"container_id"`
		Tail        int    `json:"tail"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	if params.Tail == 0 {
		params.Tail = 100
	}

	url := fmt.Sprintf("http://localhost/containers/%s/logs?stdout=true&stderr=true&tail=%d", params.ContainerID, params.Tail)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handleInspectContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://localhost/containers/%s/json", params.ContainerID)
	body, err := i.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (i *Integration) handlePullImage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://localhost/images/create?fromImage=%s", params.Image)
	_, err := i.makeRequest(ctx, "POST", url, nil)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"status": "pulled", "image": "%s"}`, params.Image), nil
}

func (i *Integration) handleRemoveContainer(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ContainerID string `json:"container_id"`
		Force       bool   `json:"force"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://localhost/containers/%s", params.ContainerID)
	if params.Force {
		url += "?force=true"
	}

	_, err := i.makeRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return "", err
	}

	return `{"status": "removed"}`, nil
}

// makeRequest makes a request to Docker API
func (i *Integration) makeRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("Docker API error (status %d): %s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
