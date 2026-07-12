package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// FormatToolCall renders a tool call start in tree style (┌─).
func FormatToolCall(toolName string, args string) string {
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	argsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	argsSummary := AbbreviateArgs(args, 80)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  ┌─ "))
	sb.WriteString(toolStyle.Render(toolName))
	if argsSummary != "" {
		sb.WriteString(argsStyle.Render(" " + argsSummary))
	}
	sb.WriteString("\n")
	return sb.String()
}

// FormatToolResult renders a tool call result in tree style (└─).
func FormatToolResult(toolName string, result string, duration time.Duration) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	resultStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	summary := AbbreviateResult(result, 120)

	var sb strings.Builder
	sb.WriteString(dimStyle.Render("  └─ "))
	sb.WriteString(okStyle.Render(fmt.Sprintf("done %s", duration.Round(time.Millisecond))))
	if summary != "" {
		sb.WriteString(resultStyle.Render(" " + summary))
	}
	sb.WriteString("\n")
	return sb.String()
}

// FormatThinkingPanel renders the thinking output panel (non-streaming mode).
func FormatThinkingPanel(content string) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(labelStyle.Render("thinking"))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  " + strings.Repeat("─", 50)))
	sb.WriteString("\n")
	if content != "" {
		sb.WriteString(content)
	}
	return sb.String()
}

// FormatThinkingIteration renders an iteration marker in the thinking panel.
func FormatThinkingIteration(iteration int) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	iterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	return dimStyle.Render("  ── ") + iterStyle.Render(fmt.Sprintf("iteration %d", iteration)) + "\n"
}

// FormatThinkingModelCall renders a model call event in the thinking panel.
func FormatThinkingModelCall(provider string) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	provStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
	return dimStyle.Render("  ⟶  ") + provStyle.Render("calling "+provider+"...") + "\n"
}

// FormatThinkingModelDone renders a model response event in the thinking panel.
func FormatThinkingModelDone(modelName string, stopReason string, tokens int, duration time.Duration) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	var sb strings.Builder
	sb.WriteString(dimStyle.Render("  ⟵  "))
	sb.WriteString(okStyle.Render(modelName))
	sb.WriteString(infoStyle.Render(fmt.Sprintf(" %s, %d tok, %s",
		stopReason, tokens, duration.Round(time.Millisecond))))
	sb.WriteString("\n")
	return sb.String()
}

// FormatThinkingStatus renders a general status line in the thinking panel.
func FormatThinkingStatus(message string) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "working..."
	}
	return dimStyle.Render("  •  ") + statusStyle.Render(msg) + "\n"
}

// FormatThinkingTokenUsage renders cumulative token usage.
func FormatThinkingTokenUsage(total int) string {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	return dimStyle.Render("  Σ  ") + infoStyle.Render(fmt.Sprintf("total tokens %d", total)) + "\n"
}

// ToolBoxConfig holds display options for FormatToolBox.
type ToolBoxConfig struct {
	// MaxOutputLines caps how many output lines are shown before truncating.
	// Zero means use the default (8).
	MaxOutputLines int
}

// FormatToolBox renders a tool call in a rich box format:
//
//	⚡ exec_command
//	│ command: ls -la
//	│ ─────────────────
//	│ total 30752
//	│ drwxr-xr-x  56 ...
//	│ (12 lines, showing first 8)
//	└─ ✓ 27ms
//
// The cfg parameter is optional; pass nil to use defaults.
func FormatToolBox(toolName string, args string, result string, duration time.Duration, cfg *ToolBoxConfig) string {
	const defaultMaxLines = 8

	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	argsKeyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	argsValStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	truncStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	durStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	maxLines := defaultMaxLines
	if cfg != nil && cfg.MaxOutputLines > 0 {
		maxLines = cfg.MaxOutputLines
	}

	border := borderStyle.Render("  │ ")
	var sb strings.Builder
	sb.WriteString("\n")

	// Header: ⚡ tool_name
	sb.WriteString("  ")
	sb.WriteString(toolStyle.Render("⚡ " + toolName))
	sb.WriteString("\n")

	// Arguments: parse key: value pairs from the abbreviated args string
	if cleaned := strings.TrimSpace(args); cleaned != "" && cleaned != "{}" && cleaned != "null" {
		pairs := parseArgPairs(cleaned)
		if len(pairs) > 0 {
			for _, p := range pairs {
				sb.WriteString(border)
				sb.WriteString(argsKeyStyle.Render(p[0] + ": "))
				val := p[1]
				if len(val) > 100 {
					val = val[:100] + "..."
				}
				sb.WriteString(argsValStyle.Render(val))
				sb.WriteString("\n")
			}
		} else {
			// Fallback: show raw abbreviated args
			abbrev := AbbreviateArgs(cleaned, 120)
			if abbrev != "" {
				sb.WriteString(border)
				sb.WriteString(argsValStyle.Render(abbrev))
				sb.WriteString("\n")
			}
		}
	}

	// Output block
	if result = strings.TrimSpace(result); result != "" {
		// Separator before output
		sb.WriteString(border)
		sb.WriteString(sepStyle.Render(strings.Repeat("─", 40)))
		sb.WriteString("\n")

		lines := strings.Split(result, "\n")
		totalLines := len(lines)

		shown := lines
		truncated := false
		if totalLines > maxLines {
			shown = lines[:maxLines]
			truncated = true
		}

		for _, line := range shown {
			sb.WriteString(border)
			sb.WriteString(outputStyle.Render(line))
			sb.WriteString("\n")
		}

		if truncated {
			sb.WriteString(border)
			sb.WriteString(truncStyle.Render(fmt.Sprintf("(%d lines, showing first %d)", totalLines, maxLines)))
			sb.WriteString("\n")
		}
	}

	// Footer: └─ ✓ duration
	sb.WriteString(borderStyle.Render("  └─ "))
	sb.WriteString(okStyle.Render("✓ "))
	sb.WriteString(durStyle.Render(duration.Round(time.Millisecond).String()))
	sb.WriteString("\n")

	return sb.String()
}

// parseArgPairs extracts key/value pairs from a JSON-like args string.
// It handles simple flat objects like {"command": "ls -la", "path": "/tmp"}.
// Returns nil if the structure is not parseable as key/value pairs.
func parseArgPairs(args string) [][2]string {
	// Strip outer braces if present
	s := strings.TrimSpace(args)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	var pairs [][2]string

	// Split on commas that are not inside nested structures.
	// We use a simple state machine (no full JSON parser).
	depth := 0
	start := 0
	var segments []string

	for i, ch := range s {
		switch ch {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				segments = append(segments, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	segments = append(segments, strings.TrimSpace(s[start:]))

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		// Expect: "key": "value" or "key": value
		colonIdx := strings.Index(seg, ":")
		if colonIdx < 0 {
			return nil // not a key:value structure
		}
		key := strings.Trim(strings.TrimSpace(seg[:colonIdx]), "\"")
		val := strings.TrimSpace(seg[colonIdx+1:])
		// Strip surrounding quotes from value
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		// Unescape common JSON escape sequences
		val = strings.ReplaceAll(val, "\\n", "\n")
		val = strings.ReplaceAll(val, "\\t", "\t")
		val = strings.ReplaceAll(val, "\\\"", "\"")
		if key == "" {
			continue
		}
		pairs = append(pairs, [2]string{key, val})
	}

	return pairs
}

// AbbreviateArgs shortens tool arguments for display, removing JSON punctuation
// and truncating to maxLen characters.
func AbbreviateArgs(args string, maxLen int) string {
	args = strings.TrimSpace(args)
	if args == "" || args == "{}" || args == "null" {
		return ""
	}
	args = strings.ReplaceAll(args, "\"", "")
	args = strings.ReplaceAll(args, "{", "")
	args = strings.ReplaceAll(args, "}", "")
	args = strings.TrimSpace(args)
	if len(args) > maxLen {
		args = args[:maxLen] + "..."
	}
	return args
}

// AbbreviateResult shortens tool results for display, taking only the first
// line and truncating to maxLen characters.
func AbbreviateResult(result string, maxLen int) string {
	result = strings.TrimSpace(result)
	if result == "" {
		return ""
	}
	if idx := strings.IndexByte(result, '\n'); idx >= 0 {
		result = result[:idx]
	}
	if len(result) > maxLen {
		result = result[:maxLen] + "..."
	}
	return result
}
