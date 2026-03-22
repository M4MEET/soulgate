package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ModelInfo represents information about an available model
type ModelInfo struct {
	ID          string
	Name        string
	Provider    string
	Description string
	ContextSize int
	Pricing     *ModelPricing
}

// ModelPricing represents model pricing information
type ModelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// ModelDiscovery handles dynamic model discovery from providers
type ModelDiscovery struct {
	cache      map[string][]ModelInfo
	cacheTTL   time.Duration
	cacheTime  map[string]time.Time
	mu         sync.RWMutex
	httpClient *http.Client
}

// NewModelDiscovery creates a new model discovery service
func NewModelDiscovery() *ModelDiscovery {
	return &ModelDiscovery{
		cache:     make(map[string][]ModelInfo),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  1 * time.Hour,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// modelsFromRegistry converts registry ModelOptions to ModelInfo for a provider.
func modelsFromRegistry(providerName string) []ModelInfo {
	def, err := LookupProvider(providerName)
	if err != nil {
		return nil
	}
	models := make([]ModelInfo, len(def.Models))
	for i, m := range def.Models {
		models[i] = ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			Provider:    providerName,
			Description: m.Description,
		}
	}
	return models
}

// DiscoverModels fetches available models for a provider
func (md *ModelDiscovery) DiscoverModels(ctx context.Context, provider string) ([]ModelInfo, error) {
	// Validate provider exists in registry
	if _, err := LookupProvider(provider); err != nil {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	// Check cache first
	md.mu.RLock()
	if cached, ok := md.cache[provider]; ok {
		if time.Since(md.cacheTime[provider]) < md.cacheTTL {
			md.mu.RUnlock()
			return cached, nil
		}
	}
	md.mu.RUnlock()

	// Try dynamic discovery for providers that support it
	var models []ModelInfo
	var err error

	switch provider {
	case "openai":
		models, err = md.fetchOpenAICompatibleModels(ctx, "https://api.openai.com/v1/models", "OPENAI_API_KEY", "openai", func(id string) bool {
			return strings.Contains(id, "gpt-") && !strings.Contains(id, "instruct")
		})
	case "groq":
		models, err = md.fetchGroqModels(ctx)
	case "ollama":
		models, err = md.fetchOllamaModels(ctx)
	case "xai":
		models, err = md.fetchXAIModels(ctx)
	default:
		// All other providers: use registry defaults (no dynamic discovery endpoint)
		models = modelsFromRegistry(provider)
	}

	if err != nil || len(models) == 0 {
		// Fall back to registry defaults
		md.mu.RLock()
		cached, ok := md.cache[provider]
		md.mu.RUnlock()
		if ok {
			return cached, nil
		}
		return modelsFromRegistry(provider), nil
	}

	// Update cache
	md.mu.Lock()
	md.cache[provider] = models
	md.cacheTime[provider] = time.Now()
	md.mu.Unlock()

	return models, nil
}

// fetchOpenAICompatibleModels fetches from an OpenAI-compatible /models endpoint
func (md *ModelDiscovery) fetchOpenAICompatibleModels(ctx context.Context, url, envKey, provider string, filter func(string) bool) ([]ModelInfo, error) {
	apiKey := os.Getenv(envKey)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Data {
		if filter == nil || filter(m.ID) {
			models = append(models, ModelInfo{
				ID:       m.ID,
				Name:     m.ID,
				Provider: provider,
			})
		}
	}

	return models, nil
}

// fetchGroqModels fetches models from Groq API
func (md *ModelDiscovery) fetchGroqModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("no API key")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Data {
		if m.Active {
			models = append(models, ModelInfo{
				ID:       m.ID,
				Name:     m.ID,
				Provider: "groq",
			})
		}
	}

	return models, nil
}

// fetchOllamaModels fetches locally available Ollama models
func (md *ModelDiscovery) fetchOllamaModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Models {
		models = append(models, ModelInfo{
			ID:       m.Name,
			Name:     m.Name,
			Provider: "ollama",
		})
	}

	return models, nil
}

// fetchXAIModels fetches models from xAI API
func (md *ModelDiscovery) fetchXAIModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("no API key")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.x.ai/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, m := range result.Data {
		if strings.Contains(strings.ToLower(m.ID), "grok") {
			models = append(models, ModelInfo{
				ID:       m.ID,
				Name:     m.ID,
				Provider: "xai",
			})
		}
	}

	return models, nil
}

// GetAllModels returns models for all providers with available API keys
func (md *ModelDiscovery) GetAllModels(ctx context.Context) ([]ModelInfo, error) {
	var allModels []ModelInfo
	for _, provider := range AllProviderNames() {
		models, err := md.DiscoverModels(ctx, provider)
		if err != nil {
			continue
		}
		allModels = append(allModels, models...)
	}
	return allModels, nil
}

// ClearCache clears the model cache
func (md *ModelDiscovery) ClearCache() {
	md.mu.Lock()
	defer md.mu.Unlock()
	md.cache = make(map[string][]ModelInfo)
	md.cacheTime = make(map[string]time.Time)
}
