// Package components provides reusable TUI rendering primitives.
//
// markdown.go implements a line-by-line markdown renderer that converts
// common markdown constructs to styled terminal output via lipgloss.
// It intentionally avoids external markdown libraries to keep the
// dependency surface small and to retain full control over terminal
// escape sequences.
package components

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Markdown styles
// ---------------------------------------------------------------------------

var (
	// Headers — bold cyan for visual hierarchy
	mdH1Style = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true).Underline(true)
	mdH2Style = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	mdH3Style = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))

	// Body text
	mdBodyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// Bold / italic inline spans
	mdBoldStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	mdItalicStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("183")).Italic(true)

	// Code
	mdInlineCodeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Background(lipgloss.Color("236"))
	mdCodeLangStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	mdCodeBorderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	mdCodeDefaultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
	mdCodeKeywordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("176"))
	mdCodeStringStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("179"))
	mdCodeCommentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)

	// Lists
	mdBulletStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Blockquote
	mdQuoteBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	mdQuoteBodyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)

	// Links — rendered in green (same convention as OpenClaw)
	mdLinkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	// Horizontal rule
	mdRuleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	// Ordered list number
	mdOrdNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// ---------------------------------------------------------------------------
// Inline pattern regexps (compiled once at package init)
// ---------------------------------------------------------------------------

var (
	// Bold: **text** or __text__
	reBold = regexp.MustCompile(`\*\*([^*\n]+)\*\*|__([^_\n]+)__`)
	// Italic: *text* (single, not **) or _text_ (single, not __)
	// We use a simplified form that avoids look-around assertions; false
	// positives inside code are acceptable given code spans are handled first.
	reItalicStar  = regexp.MustCompile(`\*([^*\n]+)\*`)
	reItalicScore = regexp.MustCompile(`_([^_\n]+)_`)
	// Inline code: `text`
	reInlineCode = regexp.MustCompile("`([^`\n]+)`")
	// Markdown link: [text](url)
	reLink = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\n]+)\)`)
	// Ordered list item
	reOrderedItem = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
)

// ---------------------------------------------------------------------------
// mdSpan tracks a styled inline segment
// ---------------------------------------------------------------------------

type mdSpan struct {
	start, end int
	inner      string
	kind       string // "code" | "bold" | "italic" | "link"
	extra      string // for links: the URL
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// RenderMarkdown converts markdown text to styled terminal output using
// lipgloss. width controls the maximum inner column used for separator lines
// and code block borders; pass 0 to use the default of 76.
//
// Supported constructs:
//   - Fenced code blocks (``` lang) with syntax highlighting
//   - Inline code (`text`)
//   - Bold (**text** / __text__)
//   - Italic (*text* / _text_)
//   - H1 / H2 / H3 headings (# / ## / ###)
//   - Unordered lists (- / * / •) with one level of nesting
//   - Ordered lists (1. 2. …)
//   - Blockquotes (> text)
//   - Markdown links ([text](url)) — green text + OSC-8 hyperlinks
//   - Horizontal rules (--- / *** / ___)
func RenderMarkdown(text string, width int) string {
	if width <= 0 {
		width = 76
	}
	borderWidth := width - 4
	if borderWidth < 20 {
		borderWidth = 20
	}
	if borderWidth > 60 {
		borderWidth = 60
	}

	var sb strings.Builder
	lines := strings.Split(text, "\n")

	inCodeBlock := false
	codeLanguage := ""

	for i, line := range lines {
		// ── Fenced code block delimiter ──────────────────────────────────────
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeLanguage = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if codeLanguage == "" {
					codeLanguage = "code"
				}
				sb.WriteString("\n")
				sb.WriteString("    " + mdCodeLangStyle.Render(codeLanguage) + "\n")
				sb.WriteString(mdCodeBorderStyle.Render("   "+strings.Repeat("─", borderWidth)) + "\n")
			} else {
				inCodeBlock = false
				sb.WriteString(mdCodeBorderStyle.Render("   "+strings.Repeat("─", borderWidth)) + "\n")
				sb.WriteString("\n")
				codeLanguage = ""
			}
			continue
		}

		// ── Inside code block ─────────────────────────────────────────────────
		if inCodeBlock {
			sb.WriteString("    " + mdHighlightCodeLine(line, codeLanguage) + "\n")
			continue
		}

		// ── Horizontal rule ───────────────────────────────────────────────────
		if mdIsHorizontalRule(line) {
			sb.WriteString("\n")
			sb.WriteString(mdRuleStyle.Render("  "+strings.Repeat("─", borderWidth)) + "\n")
			sb.WriteString("\n")
			continue
		}

		// ── Headings ──────────────────────────────────────────────────────────
		if strings.HasPrefix(line, "### ") {
			sb.WriteString("\n  " + mdH3Style.Render(strings.TrimPrefix(line, "### ")) + "\n")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			sb.WriteString("\n  " + mdH2Style.Render(strings.TrimPrefix(line, "## ")) + "\n")
			continue
		}
		if strings.HasPrefix(line, "# ") {
			sb.WriteString("\n  " + mdH1Style.Render(strings.TrimPrefix(line, "# ")) + "\n")
			continue
		}

		// ── Blockquote ────────────────────────────────────────────────────────
		if strings.HasPrefix(line, "> ") || line == ">" {
			quoted := strings.TrimPrefix(line, "> ")
			if quoted == ">" {
				quoted = ""
			}
			sb.WriteString("  " + mdQuoteBorderStyle.Render("│ ") + mdQuoteBodyStyle.Render(quoted) + "\n")
			continue
		}

		// ── Indented unordered list (2+ spaces) ───────────────────────────────
		if strings.HasPrefix(line, "  - ") || strings.HasPrefix(line, "  * ") {
			itemText := strings.TrimSpace(line)
			itemText = mdExtractListItemText(itemText)
			sb.WriteString("  " + mdBulletStyle.Render("      ◦  ") + mdRenderInline(itemText) + "\n")
			continue
		}

		// ── Unordered list ────────────────────────────────────────────────────
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
			itemText := mdExtractListItemText(line)
			sb.WriteString("  " + mdBulletStyle.Render("  •  ") + mdRenderInline(itemText) + "\n")
			continue
		}

		// ── Ordered list ──────────────────────────────────────────────────────
		if m := reOrderedItem.FindStringSubmatch(line); m != nil {
			number := m[1]
			itemText := m[2]
			sb.WriteString("  " + mdOrdNumStyle.Render("  "+number+". ") + mdRenderInline(itemText) + "\n")
			continue
		}

		// ── Blank line ────────────────────────────────────────────────────────
		if strings.TrimSpace(line) == "" {
			// Collapse consecutive blank lines to one.
			if i > 0 && strings.TrimSpace(lines[i-1]) == "" {
				continue
			}
			sb.WriteString("\n")
			continue
		}

		// ── Normal text line ─────────────────────────────────────────────────
		sb.WriteString("  " + mdRenderInline(line) + "\n")
	}

	// If a code block was opened but never closed, emit the closing border.
	if inCodeBlock {
		sb.WriteString(mdCodeBorderStyle.Render("   "+strings.Repeat("─", borderWidth)) + "\n\n")
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// Inline markdown rendering
// ---------------------------------------------------------------------------

// mdRenderInline applies inline formatting (bold, italic, inline code, links)
// to a single line of text. It processes spans in priority order:
// code > links > bold > italic.
func mdRenderInline(text string) string {
	if text == "" {
		return ""
	}

	spans := mdCollectSpans(text)
	if len(spans) == 0 {
		return mdBodyStyle.Render(text)
	}

	var sb strings.Builder
	pos := 0
	for _, s := range spans {
		if s.start < pos {
			continue // skip overlapping span
		}
		if s.start > pos {
			sb.WriteString(mdBodyStyle.Render(text[pos:s.start]))
		}
		switch s.kind {
		case "code":
			sb.WriteString(mdInlineCodeStyle.Render(" " + s.inner + " "))
		case "bold":
			sb.WriteString(mdBoldStyle.Render(s.inner))
		case "italic":
			sb.WriteString(mdItalicStyle.Render(s.inner))
		case "link":
			rendered := mdLinkStyle.Render(s.inner)
			// OSC-8 hyperlink wrapping for terminals that support it.
			sb.WriteString("\033]8;;" + s.extra + "\033\\" + rendered + "\033]8;;\033\\")
		}
		pos = s.end
	}
	if pos < len(text) {
		sb.WriteString(mdBodyStyle.Render(text[pos:]))
	}
	return sb.String()
}

// mdCollectSpans finds all inline markup spans in text, sorted by start
// position with overlaps removed. Code spans have highest priority (their
// interior is opaque), followed by links, bold, then italic.
func mdCollectSpans(text string) []mdSpan {
	var spans []mdSpan

	// 1. Inline code — highest priority; content is literal.
	for _, loc := range reInlineCode.FindAllStringIndex(text, -1) {
		sub := reInlineCode.FindStringSubmatch(text[loc[0]:loc[1]])
		if len(sub) >= 2 {
			spans = append(spans, mdSpan{loc[0], loc[1], sub[1], "code", ""})
		}
	}

	// 2. Links.
	for _, loc := range reLink.FindAllStringIndex(text, -1) {
		if mdCoveredBy(spans, loc[0], loc[1]) {
			continue
		}
		sub := reLink.FindStringSubmatch(text[loc[0]:loc[1]])
		if len(sub) >= 3 {
			spans = append(spans, mdSpan{loc[0], loc[1], sub[1], "link", sub[2]})
		}
	}

	// 3. Bold.
	for _, loc := range reBold.FindAllStringIndex(text, -1) {
		if mdCoveredBy(spans, loc[0], loc[1]) {
			continue
		}
		sub := reBold.FindStringSubmatch(text[loc[0]:loc[1]])
		inner := sub[1]
		if inner == "" {
			inner = sub[2]
		}
		spans = append(spans, mdSpan{loc[0], loc[1], inner, "bold", ""})
	}

	// 4. Italic (star form).
	for _, loc := range reItalicStar.FindAllStringIndex(text, -1) {
		if mdCoveredBy(spans, loc[0], loc[1]) {
			continue
		}
		sub := reItalicStar.FindStringSubmatch(text[loc[0]:loc[1]])
		if len(sub) >= 2 {
			spans = append(spans, mdSpan{loc[0], loc[1], sub[1], "italic", ""})
		}
	}

	// 5. Italic (underscore form).
	for _, loc := range reItalicScore.FindAllStringIndex(text, -1) {
		if mdCoveredBy(spans, loc[0], loc[1]) {
			continue
		}
		sub := reItalicScore.FindStringSubmatch(text[loc[0]:loc[1]])
		if len(sub) >= 2 {
			spans = append(spans, mdSpan{loc[0], loc[1], sub[1], "italic", ""})
		}
	}

	return mdSortAndDedup(spans)
}

// ---------------------------------------------------------------------------
// Code highlighting
// ---------------------------------------------------------------------------

// mdHighlightCodeLine applies basic syntax highlighting to a single line of
// code. It delegates to the existing highlightCodeLine function in messages.go
// but uses the md-prefixed styles for consistency.
func mdHighlightCodeLine(line, language string) string {
	trimmed := strings.TrimSpace(line)

	// Comments
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "--") {
		return mdCodeCommentStyle.Render(line)
	}

	// String-heavy lines
	if strings.Contains(line, `"`) || strings.Contains(line, "'") {
		return mdCodeStringStyle.Render(line)
	}

	// Keyword-first-word detection
	lang := strings.ToLower(language)
	if kws, ok := mdKeywords[lang]; ok {
		words := strings.Fields(trimmed)
		for _, w := range words {
			w = strings.TrimRight(w, "({[;:")
			for _, kw := range kws {
				if w == kw {
					return mdCodeKeywordStyle.Render(line)
				}
			}
		}
	}

	return mdCodeDefaultStyle.Render(line)
}

var mdKeywords = map[string][]string{
	"go":         {"func", "package", "import", "type", "struct", "interface", "return", "if", "else", "for", "range", "var", "const", "defer", "go", "chan", "select", "switch", "case", "break", "continue", "map", "make", "new"},
	"python":     {"def", "class", "import", "from", "return", "if", "else", "elif", "for", "while", "try", "except", "with", "as", "yield", "lambda", "pass", "raise", "async", "await"},
	"javascript": {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "try", "catch", "throw"},
	"js":         {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "try", "catch"},
	"typescript": {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "interface", "type", "enum"},
	"ts":         {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class", "async", "await", "import", "export", "default", "new", "this", "interface", "type"},
	"bash":       {"if", "then", "else", "fi", "for", "do", "done", "while", "case", "esac", "function", "export", "local", "echo", "exit", "return", "source"},
	"sh":         {"if", "then", "else", "fi", "for", "do", "done", "while", "case", "esac", "function", "export", "local", "echo", "exit"},
	"rust":       {"fn", "let", "mut", "pub", "struct", "enum", "impl", "trait", "use", "mod", "if", "else", "for", "while", "match", "return", "self", "Self", "async", "await"},
	"java":       {"public", "private", "protected", "class", "interface", "void", "int", "String", "return", "if", "else", "for", "while", "new", "static", "final", "import"},
	"ruby":       {"def", "end", "class", "module", "if", "else", "elsif", "unless", "while", "do", "return", "yield", "require", "include"},
	"sql":        {"SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "ON", "INSERT", "UPDATE", "DELETE", "CREATE", "TABLE", "INDEX", "DROP"},
	"yaml":       {},
	"json":       {},
	"toml":       {},
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mdIsHorizontalRule returns true for lines composed entirely of 3+ dashes,
// underscores, or asterisks (with optional spaces between them).
func mdIsHorizontalRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	var counts [3]int // 0=dash, 1=underscore, 2=asterisk
	for _, r := range trimmed {
		switch r {
		case '-':
			counts[0]++
		case '_':
			counts[1]++
		case '*':
			counts[2]++
		case ' ':
			// allowed
		default:
			return false
		}
	}
	for _, c := range counts {
		if c >= 3 {
			return true
		}
	}
	return false
}

// mdExtractListItemText strips the leading bullet character(s) from an
// unordered list item line.
func mdExtractListItemText(line string) string {
	for _, prefix := range []string{"- ", "* ", "• "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return strings.TrimSpace(line)
}

// mdCoveredBy reports whether [start, end) overlaps any span in the slice.
func mdCoveredBy(spans []mdSpan, start, end int) bool {
	for _, s := range spans {
		if start < s.end && end > s.start {
			return true
		}
	}
	return false
}

// mdSortAndDedup sorts spans by start position and removes those that overlap
// an earlier span (first-encountered wins).
func mdSortAndDedup(spans []mdSpan) []mdSpan {
	// Insertion sort — spans per line are almost always fewer than 10.
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].start < spans[j-1].start; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
	out := spans[:0]
	maxEnd := -1
	for _, s := range spans {
		if s.start >= maxEnd {
			out = append(out, s)
			if s.end > maxEnd {
				maxEnd = s.end
			}
		}
	}
	return out
}
