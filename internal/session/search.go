package session

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// SearchResult holds a single match found during a session search.
type SearchResult struct {
	// SessionID is the identifier of the session containing the match.
	SessionID string
	// Timestamp is the wall-clock time of the matched entry.
	Timestamp time.Time
	// EntryType is the session entry type (e.g. "message", "response").
	EntryType string
	// Content is the text of the matched field.
	Content string
	// Context is a short excerpt of Content surrounding the match, useful for
	// display purposes.  At most contextRadius runes are shown on each side.
	Context string
}

const contextRadius = 80

// Search scans every session stored in storage for entries whose text content
// contains query (case-insensitive).  All matching entries across all sessions
// are returned; the order follows the on-disk order within each session and
// sessions are visited in the order returned by ListSessions.
func Search(storage *Storage, query string) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query must not be empty")
	}

	sessions, err := storage.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	queryLower := strings.ToLower(query)
	var results []SearchResult

	for _, sessionID := range sessions {
		entries, err := storage.ReadSession(sessionID)
		if err != nil {
			// Skip unreadable sessions rather than aborting the entire search.
			continue
		}

		for _, entry := range entries {
			candidates := extractTextFields(entry)
			for _, text := range candidates {
				if strings.Contains(strings.ToLower(text), queryLower) {
					results = append(results, SearchResult{
						SessionID: sessionID,
						Timestamp: time.Unix(entry.Timestamp, 0),
						EntryType: entry.Type,
						Content:   text,
						Context:   buildContext(text, queryLower),
					})
					// One result per entry even if multiple fields match.
					break
				}
			}
		}
	}

	return results, nil
}

// extractTextFields returns all human-readable string values embedded in an
// entry's Data map.  The function handles the known entry types produced by
// Storage as well as generic map[string]interface{} data.
func extractTextFields(entry Entry) []string {
	data := toStringMap(entry.Data)
	var texts []string

	switch entry.Type {
	case "message", "event.message":
		if t := extractStringDirect(data, "text"); t != "" {
			texts = append(texts, t)
		}
		// Nested sender name
		if name := extractString(data, "sender", "name"); name != "" {
			texts = append(texts, name)
		}
		if username := extractString(data, "sender", "username"); username != "" {
			texts = append(texts, username)
		}

	case "response", "cmd.channel.send":
		if t := extractStringDirect(data, "text"); t != "" {
			texts = append(texts, t)
		}

	case "tool_call", "event.tool.start":
		if t := extractStringDirect(data, "tool_name"); t != "" {
			texts = append(texts, t)
		}
		if t := extractStringDirect(data, "toolName"); t != "" {
			texts = append(texts, t)
		}

	case "tool_result", "event.tool.end":
		if t := extractStringDirect(data, "tool_name"); t != "" {
			texts = append(texts, t)
		}
		if t := extractStringDirect(data, "toolName"); t != "" {
			texts = append(texts, t)
		}
		if t := extractStringDirect(data, "error"); t != "" {
			texts = append(texts, t)
		}
		// Also search inside the result field if it is a plain string.
		if result, ok := data["result"].(string); ok && result != "" {
			texts = append(texts, result)
		}

	default:
		// For unknown types, collect all top-level string values.
		for _, v := range data {
			if s, ok := v.(string); ok && s != "" {
				texts = append(texts, s)
			}
		}
	}

	return texts
}

// buildContext constructs a short excerpt of content centered on the first
// occurrence of queryLower within content (case-insensitive comparison).
func buildContext(content, queryLower string) string {
	contentLower := strings.ToLower(content)
	idx := strings.Index(contentLower, queryLower)
	if idx < 0 {
		// Fallback: return the leading characters of content.
		return truncate(content, contextRadius*2)
	}

	// Work with rune offsets to handle multi-byte characters correctly.
	runes := []rune(content)
	byteToRune := make([]int, len(content)+1)
	ri := 0
	for bi := range content {
		byteToRune[bi] = ri
		ri++
	}
	byteToRune[len(content)] = ri

	matchRuneStart := byteToRune[idx]
	queryRunes := utf8.RuneCountInString(queryLower)
	matchRuneEnd := matchRuneStart + queryRunes

	start := matchRuneStart - contextRadius
	if start < 0 {
		start = 0
	}
	end := matchRuneEnd + contextRadius
	if end > len(runes) {
		end = len(runes)
	}

	excerpt := string(runes[start:end])
	if start > 0 {
		excerpt = "..." + excerpt
	}
	if end < len(runes) {
		excerpt = excerpt + "..."
	}
	return excerpt
}

// truncate returns at most maxRunes runes of s, appending "..." when truncated.
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
