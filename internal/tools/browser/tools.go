package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// ToolSchemas returns the tool definitions for all browser tools in the
// SoulGate tool catalogue format (map[string]interface{} with "name",
// "description", and "input_schema" keys).
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "browser_open",
			"description": "Open a URL in a headless browser. Returns the page title and visible text content (truncated to 32 KB). Use this instead of net_request when you need JavaScript-rendered content or need to interact with the page afterwards.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Full URL to navigate to, e.g. \"https://example.com\".",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			"name":        "browser_screenshot",
			"description": "Take a PNG screenshot of the current browser page and save it to disk. Returns the absolute path to the saved file.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Destination file path for the screenshot (e.g. \"screenshot.png\"). If omitted, a timestamped file is created in /tmp.",
					},
				},
			},
		},
		{
			"name":        "browser_click",
			"description": "Click a DOM element matched by a CSS selector on the current browser page.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for the element to click, e.g. \"#submit-button\" or \"button[type=submit]\".",
					},
				},
				"required": []string{"selector"},
			},
		},
		{
			"name":        "browser_type",
			"description": "Clear an input field matched by a CSS selector and type text into it on the current browser page.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for the input element, e.g. \"#search-input\".",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Text to type into the field.",
					},
				},
				"required": []string{"selector", "text"},
			},
		},
		{
			"name":        "browser_eval",
			"description": "Evaluate a JavaScript expression in the context of the current browser page and return the result as a string.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"script": map[string]interface{}{
						"type":        "string",
						"description": "JavaScript expression to evaluate, e.g. \"document.title\" or \"window.location.href\".",
					},
				},
				"required": []string{"script"},
			},
		},
		{
			"name":        "browser_html",
			"description": "Get the outer HTML of a DOM element matched by a CSS selector (defaults to the full page body). Returns at most 32 KB.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS selector for the element whose HTML should be returned. Defaults to \"body\" when omitted.",
					},
				},
			},
		},
	}
}

// ExecuteTool dispatches a named browser tool call to the appropriate handler.
// mgr is the shared Manager; ctx is the caller's context (used only for
// cancellation — each operation also applies its own deadline internally).
func ExecuteTool(ctx context.Context, mgr *Manager, toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	case "browser_open":
		return executeBrowserOpen(mgr, args)
	case "browser_screenshot":
		return executeBrowserScreenshot(mgr, args)
	case "browser_click":
		return executeBrowserClick(mgr, args)
	case "browser_type":
		return executeBrowserType(mgr, args)
	case "browser_eval":
		return executeBrowserEval(mgr, args)
	case "browser_html":
		return executeBrowserHTML(mgr, args)
	default:
		return "", fmt.Errorf("browser: unknown tool %q", toolName)
	}
}

// ---------------------------------------------------------------------------
// Individual tool handlers
// ---------------------------------------------------------------------------

// executeBrowserOpen navigates to a URL and returns the page title + text.
func executeBrowserOpen(mgr *Manager, args map[string]interface{}) (string, error) {
	url, ok := stringArg(args, "url")
	if !ok || strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("browser_open: 'url' argument is required")
	}

	var title, text string

	err := mgr.run(
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Title(&title),
		// innerText gives human-readable text without HTML tags.
		chromedp.Text("body", &text, chromedp.ByQuery, chromedp.NodeVisible),
	)
	if err != nil {
		return "", fmt.Errorf("browser_open: %w", err)
	}

	result := fmt.Sprintf("Title: %s\n\n%s", title, truncate(text))
	return result, nil
}

// executeBrowserScreenshot captures a full-page PNG screenshot.
func executeBrowserScreenshot(mgr *Manager, args map[string]interface{}) (string, error) {
	destPath, _ := stringArg(args, "path")
	if strings.TrimSpace(destPath) == "" {
		destPath = filepath.Join(os.TempDir(),
			fmt.Sprintf("soulgate-screenshot-%d.png", time.Now().UnixMilli()))
	}

	// Resolve to absolute path so the result is unambiguous.
	absPath, err := filepath.Abs(destPath)
	if err != nil {
		return "", fmt.Errorf("browser_screenshot: invalid path %q: %w", destPath, err)
	}

	var buf []byte
	err = mgr.run(
		chromedp.FullScreenshot(&buf, 90),
	)
	if err != nil {
		return "", fmt.Errorf("browser_screenshot: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("browser_screenshot: failed to create directory: %w", err)
	}

	if err := os.WriteFile(absPath, buf, 0o644); err != nil {
		return "", fmt.Errorf("browser_screenshot: failed to write file: %w", err)
	}

	return fmt.Sprintf("Screenshot saved to: %s (%d bytes)", absPath, len(buf)), nil
}

// executeBrowserClick clicks the first element matched by a CSS selector.
func executeBrowserClick(mgr *Manager, args map[string]interface{}) (string, error) {
	selector, ok := stringArg(args, "selector")
	if !ok || strings.TrimSpace(selector) == "" {
		return "", fmt.Errorf("browser_click: 'selector' argument is required")
	}

	err := mgr.run(
		chromedp.Click(selector, chromedp.ByQuery),
	)
	if err != nil {
		return "", fmt.Errorf("browser_click: %w", err)
	}

	return fmt.Sprintf("Clicked element: %s", selector), nil
}

// executeBrowserType clears an input and types text into it.
func executeBrowserType(mgr *Manager, args map[string]interface{}) (string, error) {
	selector, ok := stringArg(args, "selector")
	if !ok || strings.TrimSpace(selector) == "" {
		return "", fmt.Errorf("browser_type: 'selector' argument is required")
	}

	text, ok := stringArg(args, "text")
	if !ok {
		return "", fmt.Errorf("browser_type: 'text' argument is required")
	}

	err := mgr.run(
		chromedp.Clear(selector, chromedp.ByQuery),
		chromedp.SendKeys(selector, text, chromedp.ByQuery),
	)
	if err != nil {
		return "", fmt.Errorf("browser_type: %w", err)
	}

	return fmt.Sprintf("Typed %d character(s) into: %s", len(text), selector), nil
}

// executeBrowserEval evaluates JavaScript and returns the result as a string.
func executeBrowserEval(mgr *Manager, args map[string]interface{}) (string, error) {
	script, ok := stringArg(args, "script")
	if !ok || strings.TrimSpace(script) == "" {
		return "", fmt.Errorf("browser_eval: 'script' argument is required")
	}

	var result interface{}
	err := mgr.run(
		chromedp.Evaluate(script, &result),
	)
	if err != nil {
		return "", fmt.Errorf("browser_eval: %w", err)
	}

	return fmt.Sprintf("%v", result), nil
}

// executeBrowserHTML returns the outer HTML of a selector (defaults to body).
func executeBrowserHTML(mgr *Manager, args map[string]interface{}) (string, error) {
	selector, _ := stringArg(args, "selector")
	if strings.TrimSpace(selector) == "" {
		selector = "body"
	}

	var html string
	err := mgr.run(
		chromedp.OuterHTML(selector, &html, chromedp.ByQuery),
	)
	if err != nil {
		return "", fmt.Errorf("browser_html: %w", err)
	}

	return truncate(html), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stringArg extracts a string value from an args map.
func stringArg(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
