package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// OpenRouterModel represents a model from the OpenRouter catalog.
type OpenRouterModel struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	ContextLength int     `json:"context_length,omitempty"`
	Provider      string  `json:"provider"` // extracted from ID (e.g. "anthropic" from "anthropic/claude-3.5-sonnet")
	InputCost     float64 `json:"input_cost,omitempty"`
	OutputCost    float64 `json:"output_cost,omitempty"`
}

// OpenRouterProvider is a provider extracted from the catalog.
type OpenRouterProvider struct {
	ID     string `json:"id"`     // e.g. "anthropic", "openai"
	Name   string `json:"name"`   // display name
	Models int    `json:"models"` // number of models
}

// OpenRouterCatalog fetches and caches the full OpenRouter model catalog.
type OpenRouterCatalog struct {
	cacheDir   string
	cacheTTL   time.Duration
	mu         sync.RWMutex
	models     []OpenRouterModel
	providers  []OpenRouterProvider
	lastFetch  time.Time
	httpClient *http.Client
}

// NewOpenRouterCatalog creates a catalog with disk caching in the given directory.
func NewOpenRouterCatalog(cacheDir string) *OpenRouterCatalog {
	return &OpenRouterCatalog{
		cacheDir: cacheDir,
		cacheTTL: 24 * time.Hour,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// cacheFile returns the path to the on-disk cache.
func (c *OpenRouterCatalog) cacheFile() string {
	return filepath.Join(c.cacheDir, "openrouter-models.json")
}

// GetProviders returns all providers from the catalog.
func (c *OpenRouterCatalog) GetProviders(ctx context.Context) ([]OpenRouterProvider, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]OpenRouterProvider, len(c.providers))
	copy(out, c.providers)
	return out, nil
}

// GetModels returns models for a specific provider, or all if provider is empty.
func (c *OpenRouterCatalog) GetModels(ctx context.Context, provider string) ([]OpenRouterModel, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if provider == "" {
		out := make([]OpenRouterModel, len(c.models))
		copy(out, c.models)
		return out, nil
	}

	var filtered []OpenRouterModel
	provLower := strings.ToLower(provider)
	for _, m := range c.models {
		if m.Provider == provLower {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// ensureLoaded makes sure the catalog is loaded, from cache or API.
func (c *OpenRouterCatalog) ensureLoaded(ctx context.Context) error {
	c.mu.RLock()
	if len(c.models) > 0 && time.Since(c.lastFetch) < c.cacheTTL {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	// Try disk cache first
	if c.loadFromDisk() {
		return nil
	}

	// Fetch from API
	return c.fetchFromAPI(ctx)
}

// loadFromDisk reads the cached catalog from disk.
func (c *OpenRouterCatalog) loadFromDisk() bool {
	data, err := os.ReadFile(c.cacheFile())
	if err != nil {
		return false
	}

	var cache struct {
		FetchedAt time.Time            `json:"fetched_at"`
		Models    []OpenRouterModel    `json:"models"`
		Providers []OpenRouterProvider `json:"providers"`
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return false
	}

	// Check TTL
	if time.Since(cache.FetchedAt) > c.cacheTTL {
		return false
	}

	c.mu.Lock()
	c.models = cache.Models
	c.providers = cache.Providers
	c.lastFetch = cache.FetchedAt
	c.mu.Unlock()
	return true
}

// saveToDisk persists the catalog to disk.
func (c *OpenRouterCatalog) saveToDisk() {
	c.mu.RLock()
	cache := struct {
		FetchedAt time.Time            `json:"fetched_at"`
		Models    []OpenRouterModel    `json:"models"`
		Providers []OpenRouterProvider `json:"providers"`
	}{
		FetchedAt: c.lastFetch,
		Models:    c.models,
		Providers: c.providers,
	}
	c.mu.RUnlock()

	data, err := json.Marshal(cache)
	if err != nil {
		return
	}

	_ = os.MkdirAll(c.cacheDir, 0700)
	_ = os.WriteFile(c.cacheFile(), data, 0600)
}

// fetchFromAPI fetches the model catalog from OpenRouter's public API.
func (c *OpenRouterCatalog) fetchFromAPI(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// If API fails, try loading stale disk cache
		c.loadStaleDiskCache()
		if len(c.models) > 0 {
			return nil
		}
		return fmt.Errorf("openrouter API unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.loadStaleDiskCache()
		if len(c.models) > 0 {
			return nil
		}
		return fmt.Errorf("openrouter API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			ContextLength int    `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse openrouter response: %w", err)
	}

	// Build models and extract providers
	providerMap := make(map[string]int) // provider -> model count
	var models []OpenRouterModel

	for _, m := range result.Data {
		// Extract provider from model ID: "anthropic/claude-3.5-sonnet" -> "anthropic"
		provider := "unknown"
		if idx := strings.Index(m.ID, "/"); idx > 0 {
			provider = strings.ToLower(m.ID[:idx])
		}

		models = append(models, OpenRouterModel{
			ID:            m.ID,
			Name:          m.Name,
			Description:   m.Description,
			ContextLength: m.ContextLength,
			Provider:      provider,
		})

		providerMap[provider]++
	}

	// Build sorted provider list
	var providers []OpenRouterProvider
	for name, count := range providerMap {
		displayName := name
		// Capitalize first letter
		if len(displayName) > 0 {
			displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
		}
		providers = append(providers, OpenRouterProvider{
			ID:     name,
			Name:   displayName,
			Models: count,
		})
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Models > providers[j].Models // most models first
	})

	c.mu.Lock()
	c.models = models
	c.providers = providers
	c.lastFetch = time.Now()
	c.mu.Unlock()

	// Persist to disk
	go c.saveToDisk()

	return nil
}

// loadStaleDiskCache loads the disk cache ignoring TTL (for fallback).
func (c *OpenRouterCatalog) loadStaleDiskCache() {
	data, err := os.ReadFile(c.cacheFile())
	if err != nil {
		return
	}
	var cache struct {
		FetchedAt time.Time            `json:"fetched_at"`
		Models    []OpenRouterModel    `json:"models"`
		Providers []OpenRouterProvider `json:"providers"`
	}
	if err := json.Unmarshal(data, &cache); err != nil {
		return
	}
	c.mu.Lock()
	c.models = cache.Models
	c.providers = cache.Providers
	c.lastFetch = cache.FetchedAt
	c.mu.Unlock()
}

// Refresh forces a fresh fetch from the API, ignoring cache.
func (c *OpenRouterCatalog) Refresh(ctx context.Context) error {
	c.mu.Lock()
	c.lastFetch = time.Time{} // invalidate
	c.mu.Unlock()
	return c.fetchFromAPI(ctx)
}
