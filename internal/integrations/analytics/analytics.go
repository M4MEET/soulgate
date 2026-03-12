package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/integrations"
)

// Integration implements Analytics integration for tracking usage
type Integration struct {
	mu         sync.RWMutex
	stats      map[string]*ToolStats
	configured bool
}

// ToolStats tracks statistics for a tool
type ToolStats struct {
	TotalCalls    int       `json:"total_calls"`
	SuccessCalls  int       `json:"success_calls"`
	FailedCalls   int       `json:"failed_calls"`
	LastUsed      time.Time `json:"last_used"`
	AverageTimeMs int64     `json:"average_time_ms"`
}

// New creates a new Analytics integration
func New() *Integration {
	return &Integration{
		stats: make(map[string]*ToolStats),
	}
}

// Name returns the integration name
func (i *Integration) Name() string {
	return "analytics"
}

// Description returns what this integration does
func (i *Integration) Description() string {
	return "Analytics - track integration usage, tool call statistics, and performance metrics"
}

// RequiredConfig returns required configuration fields
func (i *Integration) RequiredConfig() []integrations.ConfigField {
	return []integrations.ConfigField{
		{
			Name:        "enabled",
			Description: "Enable analytics tracking",
			Required:    false,
			Default:     "true",
			Example:     "true",
		},
	}
}

// Setup configures the integration
func (i *Integration) Setup(ctx context.Context, config map[string]string) error {
	enabled := config["enabled"]
	if enabled == "" || enabled == "true" {
		i.configured = true
	}

	return nil
}

// GetTools returns available analytics tools
func (i *Integration) GetTools() []integrations.Tool {
	return []integrations.Tool{
		{
			Name:        "analytics_get_stats",
			Description: "Get usage statistics for all tools",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleGetStats,
		},
		{
			Name:        "analytics_get_tool_stats",
			Description: "Get statistics for a specific tool",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tool_name": {
						"type": "string",
						"description": "Name of the tool"
					}
				},
				"required": ["tool_name"]
			}`),
			Handler: i.handleGetToolStats,
		},
		{
			Name:        "analytics_most_used",
			Description: "Get list of most used tools",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"limit": {
						"type": "integer",
						"description": "Number of tools to return (default: 10)"
					}
				}
			}`),
			Handler: i.handleMostUsed,
		},
		{
			Name:        "analytics_clear",
			Description: "Clear all analytics data",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
			Handler: i.handleClear,
		},
	}
}

// IsConfigured returns whether the integration is configured
func (i *Integration) IsConfigured() bool {
	return i.configured
}

// Close cleans up resources
func (i *Integration) Close() error {
	return nil
}

// RecordCall records a tool call for analytics
func (i *Integration) RecordCall(toolName string, success bool, durationMs int64) {
	i.mu.Lock()
	defer i.mu.Unlock()

	stats, exists := i.stats[toolName]
	if !exists {
		stats = &ToolStats{}
		i.stats[toolName] = stats
	}

	stats.TotalCalls++
	if success {
		stats.SuccessCalls++
	} else {
		stats.FailedCalls++
	}
	stats.LastUsed = time.Now()

	// Update average time (simple moving average)
	if stats.AverageTimeMs == 0 {
		stats.AverageTimeMs = durationMs
	} else {
		stats.AverageTimeMs = (stats.AverageTimeMs + durationMs) / 2
	}
}

// Tool handlers

func (i *Integration) handleGetStats(ctx context.Context, input json.RawMessage) (string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	result := map[string]*ToolStats{}
	for name, stats := range i.stats {
		result[name] = stats
	}

	output, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (i *Integration) handleGetToolStats(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", err
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	stats, exists := i.stats[params.ToolName]
	if !exists {
		return `{"error": "tool not found"}`, nil
	}

	output, err := json.Marshal(stats)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (i *Integration) handleMostUsed(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Limit int `json:"limit"`
	}
	json.Unmarshal(input, &params)

	if params.Limit == 0 {
		params.Limit = 10
	}

	i.mu.RLock()
	defer i.mu.RUnlock()

	// Create list of tools with call counts
	type toolUsage struct {
		Name       string `json:"name"`
		TotalCalls int    `json:"total_calls"`
	}

	var tools []toolUsage
	for name, stats := range i.stats {
		tools = append(tools, toolUsage{
			Name:       name,
			TotalCalls: stats.TotalCalls,
		})
	}

	// Sort by total calls (simple bubble sort for small datasets)
	for i := 0; i < len(tools)-1; i++ {
		for j := 0; j < len(tools)-i-1; j++ {
			if tools[j].TotalCalls < tools[j+1].TotalCalls {
				tools[j], tools[j+1] = tools[j+1], tools[j]
			}
		}
	}

	// Limit results
	if len(tools) > params.Limit {
		tools = tools[:params.Limit]
	}

	output, err := json.Marshal(tools)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (i *Integration) handleClear(ctx context.Context, input json.RawMessage) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.stats = make(map[string]*ToolStats)

	return `{"status": "cleared"}`, nil
}

// GetSummary returns a summary of analytics
func (i *Integration) GetSummary() string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	totalCalls := 0
	totalTools := len(i.stats)

	for _, stats := range i.stats {
		totalCalls += stats.TotalCalls
	}

	return fmt.Sprintf("Total tools used: %d, Total calls: %d", totalTools, totalCalls)
}
