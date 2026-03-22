package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ToolSchemas returns the tool definitions for the embedding/vector tools.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "memory_index",
			"description": "Index a document into semantic memory for later retrieval via memory_recall. Use for important information, notes, or context you want to remember across conversations.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for this memory (e.g., 'user-preference-theme', 'project-api-endpoint')",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The text content to remember",
					},
					"tags": map[string]interface{}{
						"type":        "string",
						"description": "Comma-separated tags for organization (e.g., 'preference,ui,theme')",
					},
				},
				"required": []string{"id", "content"},
			},
		},
		{
			"name":        "memory_recall",
			"description": "Search semantic memory using natural language. Returns the most relevant memories based on meaning, not just keyword matching.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Natural language query describing what you're looking for",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results (default: 5)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "memory_forget",
			"description": "Remove a specific memory by its ID.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the memory to remove",
					},
				},
				"required": []string{"id"},
			},
		},
	}
}

// ExecuteTool dispatches a tool call to the appropriate handler.
func ExecuteTool(ctx context.Context, store *VectorStore, toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	case "memory_index":
		return executeIndex(ctx, store, args)
	case "memory_recall":
		return executeRecall(ctx, store, args)
	case "memory_forget":
		return executeForget(store, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func executeIndex(ctx context.Context, store *VectorStore, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	content, _ := args["content"].(string)
	tagsStr, _ := args["tags"].(string)

	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	if content == "" {
		return "", fmt.Errorf("content is required")
	}

	metadata := make(map[string]string)
	if tagsStr != "" {
		metadata["tags"] = tagsStr
		for _, tag := range strings.Split(tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				metadata["tag:"+tag] = "true"
			}
		}
	}

	if err := store.Add(ctx, id, content, metadata); err != nil {
		return "", fmt.Errorf("failed to index: %w", err)
	}

	return fmt.Sprintf(`{"status":"success","id":"%s","total_memories":%d}`, id, store.Count()), nil
}

func executeRecall(ctx context.Context, store *VectorStore, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	limit := 5
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	results, err := store.Search(ctx, query, limit)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return `{"status":"success","count":0,"results":[],"message":"No relevant memories found"}`, nil
	}

	type resultItem struct {
		ID         string  `json:"id"`
		Content    string  `json:"content"`
		Similarity float64 `json:"similarity"`
		Tags       string  `json:"tags,omitempty"`
	}

	items := make([]resultItem, len(results))
	for i, r := range results {
		items[i] = resultItem{
			ID:         r.Document.ID,
			Content:    r.Document.Content,
			Similarity: r.Similarity,
			Tags:       r.Document.Metadata["tags"],
		}
	}

	output, _ := json.Marshal(map[string]interface{}{
		"status":  "success",
		"count":   len(items),
		"results": items,
	})
	return string(output), nil
}

func executeForget(store *VectorStore, args map[string]interface{}) (string, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	if err := store.Delete(id); err != nil {
		return "", fmt.Errorf("failed to delete: %w", err)
	}

	return fmt.Sprintf(`{"status":"success","deleted":"%s","remaining":%d}`, id, store.Count()), nil
}
