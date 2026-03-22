package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Message label styles
// ---------------------------------------------------------------------------

var (
	// User message
	userLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117")) // cyan
	userBodyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Assistant message
	assistantLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // green
	assistantDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	// Error message
	errorLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	errorHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	// System / informational message
	systemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

	// ── Retained for highlightInlineCode / highlightCodeLine callers ──────
	// These were previously used directly in this file. They are still
	// exported implicitly through the helper functions below.
	assistantBodyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	headingStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	headingPrimaryStyle  = headingStyle.Underline(true)
	bulletPrefixStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	quotePrefixStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	quoteBodyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
	codeCommentStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)
	codeStringStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	codeKeywordStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("176"))
	codeDefaultStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	inlineCodeStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Background(lipgloss.Color("236"))
	codeFenceLabelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	codeFenceBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

const (
	maxStreamingPreviewChars = 12000
	maxStreamingPreviewLines = 120

	// defaultRenderWidth is used when the caller does not supply a width.
	// Most terminal sessions are at least 80 columns wide.
	defaultRenderWidth = 80
)

// ---------------------------------------------------------------------------
// Public formatting functions
// ---------------------------------------------------------------------------

// FormatUserMessage renders a user message for the chat log.
// Label "you" in cyan, indented body text.
func FormatUserMessage(text string) string {
	var sb strings.Builder
	sb.WriteString(userLabelStyle.Render("  you"))
	sb.WriteString("\n")
	// User messages are plain text (not markdown) — render line-by-line.
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			sb.WriteString("\n")
		} else {
			sb.WriteString("  " + userBodyStyle.Render(line) + "\n")
		}
	}
	return sb.String()
}

// FormatAssistantMessage renders an AI response with rich markdown formatting.
// Label "assistant" in green, followed by markdown-rendered body.
func FormatAssistantMessage(text string) string {
	if text == "" {
		return assistantLabelStyle.Render("  assistant") + "\n" +
			assistantDimStyle.Render("  (empty response)") + "\n"
	}

	var sb strings.Builder
	sb.WriteString(assistantLabelStyle.Render("  assistant"))
	sb.WriteString("\n")
	sb.WriteString(RenderMarkdown(text, defaultRenderWidth))
	return AutolinkURLs(sb.String())
}

// FormatAssistantStreamingMessage renders a lightweight preview while tokens
// are streaming. It avoids the full markdown parse on every chunk; the final
// response is re-rendered with FormatAssistantMessage once streaming ends.
func FormatAssistantStreamingMessage(text string) string {
	var sb strings.Builder
	sb.WriteString(assistantLabelStyle.Render("  assistant"))
	sb.WriteString("\n")

	if text == "" {
		sb.WriteString("  " + assistantDimStyle.Render("... streaming"))
		return sb.String()
	}

	preview := text
	truncated := false
	if len(preview) > maxStreamingPreviewChars {
		preview = preview[len(preview)-maxStreamingPreviewChars:]
		truncated = true
	}

	lines := strings.Split(preview, "\n")
	if len(lines) > maxStreamingPreviewLines {
		lines = lines[len(lines)-maxStreamingPreviewLines:]
		truncated = true
	}

	if truncated {
		sb.WriteString("  " + assistantDimStyle.Render("... previewing recent output") + "\n")
	}

	inCodeBlock := false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			lang := strings.TrimPrefix(line, "```")
			if inCodeBlock && strings.TrimSpace(lang) != "" {
				sb.WriteString("  " + codeFenceLabelStyle.Render("  "+strings.TrimSpace(lang)) + "\n")
			}
			continue
		}
		if line == "" {
			sb.WriteString("\n")
			continue
		}
		if inCodeBlock {
			sb.WriteString("    " + codeDefaultStyle.Render(line) + "\n")
		} else {
			sb.WriteString("  " + assistantBodyStyle.Render(line) + "\n")
		}
	}

	sb.WriteString("  " + assistantDimStyle.Render("... streaming"))
	return sb.String()
}

// FormatErrorMessage renders an error with troubleshooting hints.
// Label "error" and message in red, hints in muted gray.
func FormatErrorMessage(err error) string {
	var sb strings.Builder
	sb.WriteString(errorLabelStyle.Render("  error") + "\n")
	sb.WriteString("  " + errorLabelStyle.Render(err.Error()) + "\n\n")
	sb.WriteString(errorHintStyle.Render("  Check: echo $OPENAI_API_KEY") + "\n")
	sb.WriteString(errorHintStyle.Render("  Check: cat ~/.soulgate/config.yml") + "\n")
	return sb.String()
}

// FormatSystemMessage renders a dim system/informational message.
// Used for status lines, notices, and internal events.
func FormatSystemMessage(text string) string {
	return systemStyle.Render("  "+text) + "\n"
}

// ---------------------------------------------------------------------------
// Legacy helpers — kept for compatibility with existing call-sites
// ---------------------------------------------------------------------------

// highlightCodeLine adds basic syntax highlighting to a single line of code.
// New code should prefer RenderMarkdown; this is used by the streaming preview.
func highlightCodeLine(line string, language string) string {
	keywords := map[string][]string{
		"go":         {"func", "package", "import", "type", "struct", "interface", "return", "if", "else", "for", "range", "var", "const", "defer", "go", "chan", "select", "switch", "case", "break"},
		"python":     {"def", "class", "import", "from", "return", "if", "else", "elif", "for", "while", "try", "except", "with", "as", "yield", "lambda", "pass", "raise"},
		"javascript": {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "try", "catch"},
		"js":         {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "try", "catch"},
		"typescript": {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "interface", "type"},
		"ts":         {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "interface", "type"},
		"bash":       {"if", "then", "else", "fi", "for", "do", "done", "while", "case", "esac", "function", "export", "local", "echo", "exit"},
		"sh":         {"if", "then", "else", "fi", "for", "do", "done", "while", "case", "esac", "function", "export", "local", "echo", "exit"},
		"rust":       {"fn", "let", "mut", "pub", "struct", "enum", "impl", "trait", "use", "mod", "if", "else", "for", "while", "match", "return", "self", "Self"},
		"java":       {"public", "private", "protected", "class", "interface", "void", "int", "String", "return", "if", "else", "for", "while", "new", "static", "final", "import"},
	}

	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
		return codeCommentStyle.Render(line)
	}

	if strings.Contains(line, "\"") || strings.Contains(line, "'") {
		return codeStringStyle.Render(line)
	}

	lang := strings.ToLower(language)
	if kwList, ok := keywords[lang]; ok {
		words := strings.Fields(trimmed)
		for _, word := range words {
			for _, kw := range kwList {
				if word == kw || strings.HasPrefix(word, kw+"(") || strings.HasPrefix(word, kw+".") {
					return codeKeywordStyle.Render(line)
				}
			}
		}
	}

	return codeDefaultStyle.Render(line)
}

// highlightInlineCode highlights backtick-delimited spans within a line of text.
func highlightInlineCode(text string) string {
	parts := strings.Split(text, "`")
	var sb strings.Builder

	for i, part := range parts {
		if i%2 == 1 {
			sb.WriteString(inlineCodeStyle.Render(" " + part + " "))
		} else {
			sb.WriteString(assistantBodyStyle.Render(part))
		}
	}

	return sb.String()
}
