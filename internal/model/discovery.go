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
		cacheTTL:  1 * time.Hour, // Cache for 1 hour
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DiscoverModels fetches available models for a provider
func (md *ModelDiscovery) DiscoverModels(ctx context.Context, provider string) ([]ModelInfo, error) {
	// Check cache first
	md.mu.RLock()
	if cached, ok := md.cache[provider]; ok {
		if time.Since(md.cacheTime[provider]) < md.cacheTTL {
			md.mu.RUnlock()
			return cached, nil
		}
	}
	md.mu.RUnlock()

	// Fetch fresh data
	var models []ModelInfo
	var err error

	switch provider {
	case "openai":
		models, err = md.fetchOpenAIModels(ctx)
	case "anthropic":
		models, err = md.fetchAnthropicModels(ctx)
	case "groq":
		models, err = md.fetchGroqModels(ctx)
	case "google":
		models, err = md.fetchGoogleModels(ctx)
	case "mistral":
		models, err = md.fetchMistralModels(ctx)
	case "cohere":
		models, err = md.fetchCohereModels(ctx)
	case "deepseek":
		models, err = md.fetchDeepSeekModels(ctx)
	case "openrouter":
		models, err = md.fetchOpenRouterModels(ctx)
	case "together":
		models, err = md.fetchTogetherModels(ctx)
	case "perplexity":
		models, err = md.fetchPerplexityModels(ctx)
	case "ollama":
		models, err = md.fetchOllamaModels(ctx)
	case "xai":
		models, err = md.fetchXAIModels(ctx)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	if err != nil {
		// Return cached data if available, even if stale
		md.mu.RLock()
		cached, ok := md.cache[provider]
		md.mu.RUnlock()
		if ok {
			return cached, nil
		}
		return nil, err
	}

	// Update cache
	md.mu.Lock()
	md.cache[provider] = models
	md.cacheTime[provider] = time.Now()
	md.mu.Unlock()

	return models, nil
}

// fetchOpenAIModels fetches models from OpenAI API
func (md *ModelDiscovery) fetchOpenAIModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return md.getDefaultOpenAIModels(), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return md.getDefaultOpenAIModels(), nil
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return md.getDefaultOpenAIModels(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return md.getDefaultOpenAIModels(), nil
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return md.getDefaultOpenAIModels(), nil
	}

	var models []ModelInfo
	for _, m := range result.Data {
		// Filter to only chat models
		if strings.Contains(m.ID, "gpt-") && !strings.Contains(m.ID, "instruct") {
			models = append(models, ModelInfo{
				ID:       m.ID,
				Name:     m.ID,
				Provider: "openai",
			})
		}
	}

	// If API returned no models, use defaults
	if len(models) == 0 {
		return md.getDefaultOpenAIModels(), nil
	}

	return models, nil
}

// fetchGroqModels fetches models from Groq API
func (md *ModelDiscovery) fetchGroqModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return md.getDefaultGroqModels(), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return md.getDefaultGroqModels(), nil
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return md.getDefaultGroqModels(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return md.getDefaultGroqModels(), nil
	}

	var result struct {
		Data []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return md.getDefaultGroqModels(), nil
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

	if len(models) == 0 {
		return md.getDefaultGroqModels(), nil
	}

	return models, nil
}

// fetchOllamaModels fetches locally available Ollama models
func (md *ModelDiscovery) fetchOllamaModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	if err != nil {
		return md.getDefaultOllamaModels(), nil
	}

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return md.getDefaultOllamaModels(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return md.getDefaultOllamaModels(), nil
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return md.getDefaultOllamaModels(), nil
	}

	var models []ModelInfo
	for _, m := range result.Models {
		models = append(models, ModelInfo{
			ID:       m.Name,
			Name:     m.Name,
			Provider: "ollama",
		})
	}

	if len(models) == 0 {
		return md.getDefaultOllamaModels(), nil
	}

	return models, nil
}

// Default model lists (fallback when API is unavailable)

func (md *ModelDiscovery) getDefaultOpenAIModels() []ModelInfo {
	return []ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Description: "Most capable - Complex tasks"},
		{ID: "gpt-4o-mini", Name: "GPT-4o-mini", Provider: "openai", Description: "Fast & economical"},
		{ID: "gpt-4-turbo", Name: "GPT-4-Turbo", Provider: "openai", Description: "Previous generation"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5-Turbo", Provider: "openai", Description: "Budget friendly"},
	}
}

func (md *ModelDiscovery) getDefaultGroqModels() []ModelInfo {
	return []ModelInfo{
		{ID: "llama-3.3-70b-versatile", Name: "Llama 3.3 70B", Provider: "groq", Description: "Lightning fast"},
		{ID: "mixtral-8x7b-32768", Name: "Mixtral 8x7B", Provider: "groq", Description: "Large context"},
		{ID: "gemma2-9b-it", Name: "Gemma 2 9B", Provider: "groq", Description: "Efficient"},
	}
}

func (md *ModelDiscovery) getDefaultOllamaModels() []ModelInfo {
	return []ModelInfo{
		{ID: "llama3.2", Name: "Llama 3.2", Provider: "ollama", Description: "Local - Private"},
		{ID: "mistral", Name: "Mistral", Provider: "ollama", Description: "Local - Fast"},
		{ID: "codellama", Name: "CodeLlama", Provider: "ollama", Description: "Local - Code"},
	}
}

func (md *ModelDiscovery) fetchAnthropicModels(ctx context.Context) ([]ModelInfo, error) {
	// Anthropic doesn't have a public models endpoint, return defaults
	return []ModelInfo{
		{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Provider: "anthropic", Description: "Deep reasoning"},
		{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Provider: "anthropic", Description: "Balanced"},
		{ID: "claude-haiku-4-20250501", Name: "Claude Haiku 4", Provider: "anthropic", Description: "Fast"},
	}, nil
}

func (md *ModelDiscovery) fetchGoogleModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		// Gemini 3 Series (Latest - Preview)
		{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro Preview", Provider: "google", Description: "Latest - Complex reasoning (1M context)"},
		{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash Preview", Provider: "google", Description: "Latest - Fast multimodal"},
		{ID: "gemini-3-pro-image-preview", Name: "Gemini 3 Pro Image", Provider: "google", Description: "Image generation"},
		// Gemini 2.5 Series (Current Stable)
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: "google", Description: "Most powerful stable"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "google", Description: "Fast & efficient"},
		{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Provider: "google", Description: "Lightweight"},
		{ID: "gemini-2.5-flash-image", Name: "Gemini 2.5 Flash Image", Provider: "google", Description: "Image generation"},
		// Gemini 2.0 (Deprecated)
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash (Deprecated)", Provider: "google", Description: "Shutting down March 31, 2026"},
	}, nil
}

func (md *ModelDiscovery) fetchMistralModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "mistral-large-latest", Name: "Mistral Large", Provider: "mistral", Description: "Most capable"},
		{ID: "mistral-medium-latest", Name: "Mistral Medium", Provider: "mistral", Description: "Balanced"},
		{ID: "mistral-small-latest", Name: "Mistral Small", Provider: "mistral", Description: "Fast"},
	}, nil
}

func (md *ModelDiscovery) fetchCohereModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "command-r-plus", Name: "Command R+", Provider: "cohere", Description: "RAG & tools"},
		{ID: "command-r", Name: "Command R", Provider: "cohere", Description: "Balanced"},
	}, nil
}

func (md *ModelDiscovery) fetchDeepSeekModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "deepseek-chat", Name: "DeepSeek V3", Provider: "deepseek", Description: "Strong coding"},
		{ID: "deepseek-coder", Name: "DeepSeek Coder", Provider: "deepseek", Description: "Code specialist"},
	}, nil
}

func (md *ModelDiscovery) fetchOpenRouterModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "openrouter/auto", Name: "Auto", Provider: "openrouter", Description: "Best available"},
		{ID: "openai/gpt-4o", Name: "GPT-4o", Provider: "openrouter", Description: "Via OpenRouter"},
		{ID: "anthropic/claude-opus-4", Name: "Claude Opus 4", Provider: "openrouter", Description: "Via OpenRouter"},
	}, nil
}

func (md *ModelDiscovery) fetchTogetherModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo", Name: "Llama 3.1 405B", Provider: "together", Description: "Largest open"},
		{ID: "Qwen/Qwen2.5-72B-Instruct-Turbo", Name: "Qwen 2.5 72B", Provider: "together", Description: "Multilingual"},
	}, nil
}

func (md *ModelDiscovery) fetchPerplexityModels(ctx context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{ID: "sonar", Name: "Sonar", Provider: "perplexity", Description: "Web search"},
		{ID: "sonar-pro", Name: "Sonar Pro", Provider: "perplexity", Description: "Advanced search"},
	}, nil
}

func (md *ModelDiscovery) fetchXAIModels(ctx context.Context) ([]ModelInfo, error) {
	apiKey := os.Getenv("XAI_API_KEY")
	if apiKey == "" {
		return md.getDefaultXAIModels(), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.x.ai/v1/models", nil)
	if err != nil {
		return md.getDefaultXAIModels(), nil
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := md.httpClient.Do(req)
	if err != nil {
		return md.getDefaultXAIModels(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return md.getDefaultXAIModels(), nil
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
		} `json:"data"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return md.getDefaultXAIModels(), nil
	}

	var models []ModelInfo
	for _, m := range result.Data {
		// Filter to only include grok models
		if strings.Contains(strings.ToLower(m.ID), "grok") {
			// Create a friendly name from the ID
			name := m.ID
			description := "xAI Grok model"

			// Add specific descriptions for known models
			switch m.ID {
			case "grok-2-latest":
				name = "Grok 2 Latest"
				description = "Latest version - Auto-updates"
			case "grok-2-1212":
				name = "Grok 2 (Dec 2024)"
				description = "Stable snapshot - Dec 12, 2024"
			case "grok-vision-beta":
				name = "Grok Vision Beta"
				description = "Multimodal - Image understanding"
			default:
				// For any new models, create a friendly name
				name = strings.ReplaceAll(m.ID, "-", " ")
				name = strings.Title(name)
			}

			models = append(models, ModelInfo{
				ID:       m.ID,
				Name:     name,
				Provider: "xai",
				Description: description,
			})
		}
	}

	// If API returned no models, use defaults
	if len(models) == 0 {
		return md.getDefaultXAIModels(), nil
	}

	return models, nil
}

func (md *ModelDiscovery) getDefaultXAIModels() []ModelInfo {
	return []ModelInfo{
		{ID: "grok-4-1-fast-reasoning", Name: "Grok 4.1 Fast (Reasoning)", Provider: "xai", Description: "Latest - Deep reasoning"},
		{ID: "grok-4-1-fast-non-reasoning", Name: "Grok 4.1 Fast", Provider: "xai", Description: "Latest - Faster responses"},
		{ID: "grok-4-fast-reasoning", Name: "Grok 4 Fast (Reasoning)", Provider: "xai", Description: "Grok 4 with reasoning"},
		{ID: "grok-3", Name: "Grok 3", Provider: "xai", Description: "Most powerful"},
		{ID: "grok-3-mini", Name: "Grok 3 Mini", Provider: "xai", Description: "Fast & economical"},
		{ID: "grok-code-fast-1", Name: "Grok Code Fast", Provider: "xai", Description: "Code specialist"},
		{ID: "grok-2-vision-1212", Name: "Grok 2 Vision", Provider: "xai", Description: "Multimodal"},
	}
}

// GetAllModels returns models for all providers with available API keys
func (md *ModelDiscovery) GetAllModels(ctx context.Context) ([]ModelInfo, error) {
	providers := []string{"openai", "anthropic", "groq", "google", "mistral", "cohere", "deepseek", "openrouter", "together", "perplexity", "ollama", "xai"}

	var allModels []ModelInfo
	for _, provider := range providers {
		models, err := md.DiscoverModels(ctx, provider)
		if err != nil {
			continue // Skip providers with errors
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
