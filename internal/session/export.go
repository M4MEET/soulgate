package session

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
	"time"
)

// ExportJSON writes all entries as a pretty-printed JSON array to w.
func ExportJSON(entries []Entry, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

// ExportMarkdown writes a human-readable Markdown transcript to w.
//
// Conventions:
//   - User messages are blockquoted with "> "
//   - Assistant responses are plain paragraphs
//   - Tool calls / results appear as fenced code blocks
func ExportMarkdown(entries []Entry, w io.Writer) error {
	bw := &errWriter{w: w}

	bw.writef("# Session Transcript\n\n")
	bw.writef("_Exported %s_\n\n", time.Now().Format("2006-01-02 15:04:05"))
	bw.writef("---\n\n")

	for _, entry := range entries {
		ts := time.Unix(entry.Timestamp, 0).Format("15:04:05")
		data := toStringMap(entry.Data)

		switch entry.Type {
		case "message", "event.message":
			sender := extractString(data, "sender", "username")
			if sender == "" {
				sender = extractStringDirect(data, "sender")
			}
			text := extractStringDirect(data, "text")
			bw.writef("**[%s] %s:**\n", ts, escapeMarkdown(sender))
			for _, line := range strings.Split(text, "\n") {
				bw.writef("> %s\n", line)
			}
			bw.writef("\n")

		case "response", "cmd.channel.send":
			text := extractStringDirect(data, "text")
			bw.writef("**[%s] Assistant:**\n\n", ts)
			bw.writef("%s\n\n", text)

		case "tool_call", "event.tool.start":
			toolName := extractStringDirect(data, "tool_name")
			if toolName == "" {
				toolName = extractStringDirect(data, "toolName")
			}
			args := data["args"]
			argsJSON, _ := json.MarshalIndent(args, "", "  ")
			bw.writef("**[%s] Tool Call: `%s`**\n\n", ts, toolName)
			bw.writef("```json\n%s\n```\n\n", string(argsJSON))

		case "tool_result", "event.tool.end":
			toolName := extractStringDirect(data, "tool_name")
			if toolName == "" {
				toolName = extractStringDirect(data, "toolName")
			}
			result := data["result"]
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			errMsg := extractStringDirect(data, "error")

			bw.writef("**[%s] Tool Result: `%s`**\n\n", ts, toolName)
			if errMsg != "" {
				bw.writef("> **Error:** %s\n\n", errMsg)
			} else {
				bw.writef("```json\n%s\n```\n\n", string(resultJSON))
			}

		default:
			// Generic fallback: render as a JSON code block with the entry type as label.
			rawJSON, _ := json.MarshalIndent(entry.Data, "", "  ")
			bw.writef("**[%s] %s**\n\n```json\n%s\n```\n\n", ts, entry.Type, string(rawJSON))
		}
	}

	return bw.err
}

// ExportHTML writes a self-contained, dark-themed HTML transcript to w.
// All CSS is inlined; no external resources are required.
func ExportHTML(entries []Entry, w io.Writer) error {
	bw := &errWriter{w: w}

	bw.writef(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>SoulGate Session Transcript</title>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{background:#0d1117;color:#c9d1d9;font-family:'Segoe UI',system-ui,sans-serif;font-size:15px;line-height:1.6;padding:24px}
  .container{max-width:860px;margin:0 auto}
  h1{color:#e6edf3;font-size:1.4rem;margin-bottom:4px}
  .meta{color:#6e7681;font-size:.85rem;margin-bottom:28px}
  .entry{margin-bottom:20px;border-radius:8px;overflow:hidden}
  .entry-header{font-size:.78rem;color:#6e7681;padding:4px 12px;background:#161b22;border-bottom:1px solid #21262d}
  .entry-body{padding:12px 16px}
  /* user message */
  .user .entry-header{border-left:3px solid #388bfd}
  .user .entry-body{background:#0d1936;border-left:3px solid #388bfd}
  .user .entry-body p{color:#79c0ff}
  /* assistant response */
  .assistant .entry-header{border-left:3px solid #3fb950}
  .assistant .entry-body{background:#0d1f0d;border-left:3px solid #3fb950}
  .assistant .entry-body p{white-space:pre-wrap;color:#aff5b4}
  /* tool call */
  .tool-call .entry-header{border-left:3px solid #d29922}
  .tool-call .entry-body{background:#1c1711;border-left:3px solid #d29922}
  /* tool result */
  .tool-result .entry-header{border-left:3px solid #bc8cff}
  .tool-result .entry-body{background:#17111c;border-left:3px solid #bc8cff}
  /* generic */
  .generic .entry-header{border-left:3px solid #6e7681}
  .generic .entry-body{background:#161b22;border-left:3px solid #6e7681}
  /* code / pre */
  pre{background:#010409;border:1px solid #21262d;border-radius:6px;padding:12px;overflow-x:auto;font-size:.85rem;font-family:'Cascadia Code','Fira Mono',monospace;color:#e6edf3;margin:0}
  .error-badge{color:#f85149;font-weight:600}
  /* collapsible tool sections */
  details{cursor:pointer}
  summary{list-style:none;outline:none;color:#8b949e;font-size:.85rem}
  summary::-webkit-details-marker{display:none}
  summary::before{content:'[+] ';font-family:monospace}
  details[open] summary::before{content:'[-] '}
  details[open]{margin-top:8px}
</style>
</head>
<body>
<div class="container">
<h1>SoulGate Session Transcript</h1>
<p class="meta">Exported %s &mdash; %d entries</p>
`, time.Now().Format("2006-01-02 15:04:05"), len(entries))

	for _, entry := range entries {
		ts := time.Unix(entry.Timestamp, 0).Format("15:04:05")
		data := toStringMap(entry.Data)

		switch entry.Type {
		case "message", "event.message":
			sender := extractString(data, "sender", "username")
			if sender == "" {
				sender = extractStringDirect(data, "sender")
			}
			text := extractStringDirect(data, "text")
			bw.writef(`<div class="entry user">
<div class="entry-header">%s &mdash; User: %s</div>
<div class="entry-body"><p>%s</p></div>
</div>
`, ts, html.EscapeString(sender), nl2br(html.EscapeString(text)))

		case "response", "cmd.channel.send":
			text := extractStringDirect(data, "text")
			bw.writef(`<div class="entry assistant">
<div class="entry-header">%s &mdash; Assistant</div>
<div class="entry-body"><p>%s</p></div>
</div>
`, ts, nl2br(html.EscapeString(text)))

		case "tool_call", "event.tool.start":
			toolName := extractStringDirect(data, "tool_name")
			if toolName == "" {
				toolName = extractStringDirect(data, "toolName")
			}
			args := data["args"]
			argsJSON, _ := json.MarshalIndent(args, "", "  ")
			bw.writef(`<div class="entry tool-call">
<div class="entry-header">%s &mdash; Tool Call</div>
<div class="entry-body">
<details><summary>%s</summary>
<pre>%s</pre>
</details>
</div>
</div>
`, ts, html.EscapeString(toolName), html.EscapeString(string(argsJSON)))

		case "tool_result", "event.tool.end":
			toolName := extractStringDirect(data, "tool_name")
			if toolName == "" {
				toolName = extractStringDirect(data, "toolName")
			}
			errMsg := extractStringDirect(data, "error")
			result := data["result"]
			resultJSON, _ := json.MarshalIndent(result, "", "  ")

			errorSection := ""
			if errMsg != "" {
				errorSection = fmt.Sprintf(`<p class="error-badge">Error: %s</p>`, html.EscapeString(errMsg))
			}
			bw.writef(`<div class="entry tool-result">
<div class="entry-header">%s &mdash; Tool Result</div>
<div class="entry-body">
%s
<details><summary>%s</summary>
<pre>%s</pre>
</details>
</div>
</div>
`, ts, errorSection, html.EscapeString(toolName), html.EscapeString(string(resultJSON)))

		default:
			rawJSON, _ := json.MarshalIndent(entry.Data, "", "  ")
			bw.writef(`<div class="entry generic">
<div class="entry-header">%s &mdash; %s</div>
<div class="entry-body"><pre>%s</pre></div>
</div>
`, ts, html.EscapeString(entry.Type), html.EscapeString(string(rawJSON)))
		}
	}

	bw.writef("</div>\n</body>\n</html>\n")
	return bw.err
}

// --- helpers ---

// errWriter wraps an io.Writer and accumulates the first write error.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) writef(format string, args ...interface{}) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

// toStringMap safely converts the Data field of an Entry to map[string]interface{}.
func toStringMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// extractString drills into a nested map key then retrieves a string sub-key.
// e.g. extractString(data, "sender", "username") returns data["sender"]["username"].
func extractString(data map[string]interface{}, outerKey, innerKey string) string {
	outer, ok := data[outerKey]
	if !ok {
		return ""
	}
	inner, ok := outer.(map[string]interface{})
	if !ok {
		return ""
	}
	s, _ := inner[innerKey].(string)
	return s
}

// extractStringDirect retrieves a top-level string value from a data map.
func extractStringDirect(data map[string]interface{}, key string) string {
	s, _ := data[key].(string)
	return s
}

// nl2br replaces newline characters with HTML line breaks.
func nl2br(s string) string {
	return strings.ReplaceAll(s, "\n", "<br>")
}

// escapeMarkdown escapes characters that have special meaning in Markdown.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"*", `\*`,
		"_", `\_`,
		"`", "\\`",
		"[", `\[`,
		"]", `\]`,
	)
	return replacer.Replace(s)
}
