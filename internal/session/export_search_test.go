package session

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers to build a consistent test session ---

func buildTestStorage(t *testing.T) (*Storage, string, []Entry) {
	t.Helper()

	dir, err := os.MkdirTemp("", "sg-export-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	st, err := NewStorage(dir)
	require.NoError(t, err)

	sid := "export-session"
	require.NoError(t, st.LogMessage(sid, "alice", "Hello, summarize example.txt"))
	require.NoError(t, st.LogToolCall(sid, "read_file", map[string]interface{}{"path": "example.txt"}))
	require.NoError(t, st.LogToolResult(sid, "read_file", "This is example content.", nil))
	require.NoError(t, st.LogResponse(sid, "The file says: This is example content."))

	entries, err := st.ReadSession(sid)
	require.NoError(t, err)
	require.Len(t, entries, 4)

	return st, sid, entries
}

// --- ExportJSON tests ---

func TestExportJSON_ContainsUserMessage(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportJSON(entries, &buf))

	out := buf.String()
	assert.Contains(t, out, "Hello, summarize example.txt")
	assert.Contains(t, out, "read_file")
	assert.Contains(t, out, "This is example content.")
}

func TestExportJSON_ValidJSONArray(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportJSON(entries, &buf))

	// The output must begin with '[' (a JSON array).
	trimmed := strings.TrimSpace(buf.String())
	assert.True(t, strings.HasPrefix(trimmed, "["), "expected JSON array, got: %s", trimmed[:min(30, len(trimmed))])
}

// --- ExportMarkdown tests ---

func TestExportMarkdown_UserMessageBlockquoted(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportMarkdown(entries, &buf))

	assert.Contains(t, buf.String(), "> Hello, summarize example.txt")
}

func TestExportMarkdown_ToolCallFenced(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportMarkdown(entries, &buf))

	md := buf.String()
	assert.Contains(t, md, "read_file")
	assert.Contains(t, md, "```json")
}

func TestExportMarkdown_ResponseIncluded(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportMarkdown(entries, &buf))

	assert.Contains(t, buf.String(), "The file says: This is example content.")
}

// --- ExportHTML tests ---

func TestExportHTML_IsValidHTML(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportHTML(entries, &buf))

	h := buf.String()
	assert.Contains(t, h, "<!DOCTYPE html>")
	assert.Contains(t, h, "</html>")
}

func TestExportHTML_ContainsUserMessage(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportHTML(entries, &buf))

	assert.Contains(t, buf.String(), "Hello, summarize example.txt")
}

func TestExportHTML_ToolCallsCollapsible(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportHTML(entries, &buf))

	h := buf.String()
	assert.Contains(t, h, "<details>")
	assert.Contains(t, h, "</details>")
	assert.Contains(t, h, "read_file")
}

func TestExportHTML_DarkThemeCSS(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportHTML(entries, &buf))

	// Dark background colour should be present in the inline CSS.
	assert.Contains(t, buf.String(), "#0d1117")
}

func TestExportHTML_SelfContained(t *testing.T) {
	_, _, entries := buildTestStorage(t)

	var buf bytes.Buffer
	require.NoError(t, ExportHTML(entries, &buf))

	h := buf.String()
	// No external links.
	assert.NotContains(t, h, `<link `)
	assert.NotContains(t, h, `<script src`)
}

// --- Search tests ---

func TestSearch_FindsMatchInUserMessage(t *testing.T) {
	st, sid, _ := buildTestStorage(t)

	results, err := Search(st, "summarize")
	require.NoError(t, err)
	require.NotEmpty(t, results)

	assert.Equal(t, sid, results[0].SessionID)
	assert.Contains(t, strings.ToLower(results[0].Context), "summarize")
}

func TestSearch_CaseInsensitive(t *testing.T) {
	st, _, _ := buildTestStorage(t)

	upper, err := Search(st, "SUMMARIZE")
	require.NoError(t, err)

	lower, err := Search(st, "summarize")
	require.NoError(t, err)

	assert.Equal(t, len(lower), len(upper), "case-insensitive search should return same count")
}

func TestSearch_NoResultsForAbsentQuery(t *testing.T) {
	st, _, _ := buildTestStorage(t)

	results, err := Search(st, "ZZZNOTPRESENT")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearch_MatchInToolName(t *testing.T) {
	st, _, _ := buildTestStorage(t)

	results, err := Search(st, "read_file")
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestSearch_MatchInResponse(t *testing.T) {
	st, _, _ := buildTestStorage(t)

	results, err := Search(st, "The file says")
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestSearch_EmptyQueryReturnsError(t *testing.T) {
	st, _, _ := buildTestStorage(t)

	_, err := Search(st, "")
	assert.Error(t, err)
}

func TestSearch_MultipleSessionsSearched(t *testing.T) {
	dir, err := os.MkdirTemp("", "sg-multi-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	st, err := NewStorage(dir)
	require.NoError(t, err)

	require.NoError(t, st.LogMessage("session-a", "user", "unique-token-alpha"))
	require.NoError(t, st.LogMessage("session-b", "user", "unique-token-beta"))

	resultsAlpha, err := Search(st, "unique-token-alpha")
	require.NoError(t, err)
	assert.Len(t, resultsAlpha, 1)
	assert.Equal(t, "session-a", resultsAlpha[0].SessionID)

	// A query matching both sessions should return two results.
	resultsBoth, err := Search(st, "unique-token")
	require.NoError(t, err)
	assert.Len(t, resultsBoth, 2)
}

func TestBuildContext_IncludesQueryTerm(t *testing.T) {
	ctx := buildContext("The quick brown fox jumps over the lazy dog", "fox")
	assert.Contains(t, strings.ToLower(ctx), "fox")
}

func TestBuildContext_TruncatesLongContent(t *testing.T) {
	long := strings.Repeat("a", 500) + "needle" + strings.Repeat("b", 500)
	ctx := buildContext(long, "needle")
	assert.Contains(t, ctx, "needle")
	// Context should not be the full 1006-character string.
	assert.Less(t, len(ctx), len(long))
	assert.Contains(t, ctx, "...")
}

// min is available in Go 1.21+; include a local copy for older toolchains.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
