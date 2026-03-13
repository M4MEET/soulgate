package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// SearchResult is a single result returned by a web search.
type SearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// SearchOptions configures a web search request.
type SearchOptions struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"` // default 10
	Freshness  string `json:"freshness,omitempty"`   // "day", "week", "month"
	Country    string `json:"country,omitempty"`     // ISO 3166-1 alpha-2 country code
}

// searchClient is the shared HTTP client used by all search providers.
// The 15-second timeout is intentionally short: search APIs are low-latency
// and a stalled request should not block the agentic loop.
var searchClient = &http.Client{
	Timeout: 15 * time.Second,
}

// Search performs a web search using available providers.
// It tries Brave Search first (when BRAVE_API_KEY is set) and falls back to
// DuckDuckGo Instant Answers, which requires no API key.
func Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	if opts.Query == "" {
		return nil, fmt.Errorf("search: query is required")
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 10
	}

	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		results, err := searchBrave(ctx, opts, key)
		if err == nil {
			return results, nil
		}
		// Fall through to DuckDuckGo on any Brave error so the caller always
		// gets a best-effort answer.
	}

	return searchDuckDuckGo(ctx, opts)
}

// --------------------------------------------------------------------------
// Brave Search
// --------------------------------------------------------------------------

// braveResponse is the top-level envelope returned by the Brave Web Search API.
// Only the fields we consume are decoded; the rest are discarded.
type braveResponse struct {
	Web struct {
		Results []braveResult `json:"results"`
	} `json:"web"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func searchBrave(ctx context.Context, opts SearchOptions, apiKey string) ([]SearchResult, error) {
	const braveSearchURL = "https://api.search.brave.com/res/v1/web/search"

	params := url.Values{}
	params.Set("q", opts.Query)
	params.Set("count", fmt.Sprintf("%d", opts.MaxResults))
	if opts.Country != "" {
		params.Set("country", opts.Country)
	}
	if opts.Freshness != "" {
		// Brave uses "pd" (past day), "pw" (past week), "pm" (past month).
		switch opts.Freshness {
		case "day":
			params.Set("freshness", "pd")
		case "week":
			params.Set("freshness", "pw")
		case "month":
			params.Set("freshness", "pm")
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, braveSearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("brave search: failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", apiKey)

	resp, err := searchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave search: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("brave search: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var parsed braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("brave search: failed to decode response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		results = append(results, SearchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
		})
	}

	return results, nil
}

// --------------------------------------------------------------------------
// DuckDuckGo Instant Answers (fallback)
// --------------------------------------------------------------------------

// ddgResponse is the envelope returned by the DuckDuckGo Instant Answer API.
// The API mixes different result types; we surface the ones most useful to an
// agent (abstract text, related topics with text snippets).
type ddgResponse struct {
	// Heading is the canonical title for the main result.
	Heading string `json:"Heading"`
	// AbstractText is a plain-text summary of the main result.
	AbstractText string `json:"AbstractText"`
	// AbstractURL is the source URL for the abstract.
	AbstractURL string `json:"AbstractURL"`
	// RelatedTopics contains additional related results.
	RelatedTopics []ddgTopic `json:"RelatedTopics"`
}

type ddgTopic struct {
	// Text is a short snippet (used for leaf topics).
	Text string `json:"Text"`
	// FirstURL is the result URL.
	FirstURL string `json:"FirstURL"`
	// Topics contains nested topics for category nodes; we skip them.
	Topics []ddgTopic `json:"Topics"`
}

func searchDuckDuckGo(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	const ddgURL = "https://api.duckduckgo.com/"

	params := url.Values{}
	params.Set("q", opts.Query)
	params.Set("format", "json")
	// no_html=1 returns plain text in text fields.
	params.Set("no_html", "1")
	// skip_disambig=1 avoids disambiguation pages.
	params.Set("skip_disambig", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ddgURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo search: failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := searchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo search: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo search: unexpected status %d", resp.StatusCode)
	}

	var parsed ddgResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("duckduckgo search: failed to decode response: %w", err)
	}

	var results []SearchResult

	// Include the top-level abstract if present.
	if parsed.AbstractText != "" {
		results = append(results, SearchResult{
			Title:       parsed.Heading,
			URL:         parsed.AbstractURL,
			Description: parsed.AbstractText,
		})
	}

	// Collect leaf topics (skip category groups that only have nested Topics).
	for _, topic := range parsed.RelatedTopics {
		if topic.Text == "" || topic.FirstURL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:       topic.FirstURL, // DDG does not provide per-topic titles
			URL:         topic.FirstURL,
			Description: topic.Text,
		})
		if len(results) >= opts.MaxResults {
			break
		}
	}

	return results, nil
}
