package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultHubURL = "https://hub.soulgate.dev/api/v1"
	CacheTimeout  = 30 * time.Minute
)

// HubClient handles communication with SoulHub
type HubClient struct {
	baseURL    string
	httpClient *http.Client
	cacheDir   string
}

// NewHubClient creates a new hub client
func NewHubClient(baseURL string, cacheDir string) *HubClient {
	if baseURL == "" {
		baseURL = DefaultHubURL
	}

	return &HubClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir: cacheDir,
	}
}

// ListPlugins lists all available plugins
func (c *HubClient) ListPlugins(ctx context.Context) ([]PluginInfo, error) {
	var plugins []PluginInfo

	// Try cache first
	cached, err := c.readCache("plugins.json")
	if err == nil && cached != nil {
		if err := json.Unmarshal(cached, &plugins); err == nil {
			return plugins, nil
		}
	}

	// Fetch from hub
	data, err := c.fetch(ctx, "/plugins")
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, fmt.Errorf("failed to parse plugins: %w", err)
	}

	// Cache result
	c.writeCache("plugins.json", data)

	return plugins, nil
}

// SearchPlugins searches for plugins
func (c *HubClient) SearchPlugins(ctx context.Context, query string) ([]PluginInfo, error) {
	endpoint := fmt.Sprintf("/plugins/search?q=%s", query)
	data, err := c.fetch(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var plugins []PluginInfo
	if err := json.Unmarshal(data, &plugins); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	return plugins, nil
}

// GetPlugin gets detailed info about a plugin
func (c *HubClient) GetPlugin(ctx context.Context, name string) (*PluginDetails, error) {
	endpoint := fmt.Sprintf("/plugins/%s", name)
	data, err := c.fetch(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var plugin PluginDetails
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to parse plugin details: %w", err)
	}

	return &plugin, nil
}

// DownloadPlugin downloads a plugin binary
func (c *HubClient) DownloadPlugin(ctx context.Context, name string, destPath string) error {
	endpoint := fmt.Sprintf("/plugins/%s/download", name)

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ListSkills lists all available skills
func (c *HubClient) ListSkills(ctx context.Context) ([]SkillInfo, error) {
	var skills []SkillInfo

	// Try cache first
	cached, err := c.readCache("skills.json")
	if err == nil && cached != nil {
		if err := json.Unmarshal(cached, &skills); err == nil {
			return skills, nil
		}
	}

	// Fetch from hub
	data, err := c.fetch(ctx, "/skills")
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("failed to parse skills: %w", err)
	}

	// Cache result
	c.writeCache("skills.json", data)

	return skills, nil
}

// GetPopular gets popular items
func (c *HubClient) GetPopular(ctx context.Context, category string) ([]HubItem, error) {
	endpoint := fmt.Sprintf("/%s/popular", category)
	data, err := c.fetch(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var items []HubItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse popular items: %w", err)
	}

	return items, nil
}

// GetTrending gets trending items
func (c *HubClient) GetTrending(ctx context.Context) ([]HubItem, error) {
	data, err := c.fetch(ctx, "/trending")
	if err != nil {
		return nil, err
	}

	var items []HubItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse trending items: %w", err)
	}

	return items, nil
}

// RateItem submits a rating
func (c *HubClient) RateItem(ctx context.Context, category, name string, rating int) error {
	endpoint := fmt.Sprintf("/%s/%s/rate", category, name)

	payload := map[string]interface{}{
		"rating": rating,
	}

	_, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("rating failed: status %d", resp.StatusCode)
	}

	return nil
}

// fetch performs HTTP GET request
func (c *HubClient) fetch(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// readCache reads from cache
func (c *HubClient) readCache(filename string) ([]byte, error) {
	path := filepath.Join(c.cacheDir, filename)

	// Check if file exists and is recent
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Check cache age
	if time.Since(info.ModTime()) > CacheTimeout {
		return nil, fmt.Errorf("cache expired")
	}

	return os.ReadFile(path)
}

// writeCache writes to cache
func (c *HubClient) writeCache(filename string, data []byte) error {
	path := filepath.Join(c.cacheDir, filename)

	// Create cache directory
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
