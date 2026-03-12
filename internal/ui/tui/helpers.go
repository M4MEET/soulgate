package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// addMessage adds a message to the chat history and updates the display
func (m *InteractiveChatModel) addMessage(text string) {
	m.messages = append(m.messages, text)
	content := strings.Join(m.messages, "\n\n")
	m.output.SetContent(content)
	m.output.GotoBottom()
}

// updateAutocomplete updates the autocomplete suggestions based on current input
func (m *InteractiveChatModel) updateAutocomplete() {
	value := m.input.Value()
	if strings.HasPrefix(value, "/") {
		commands := []string{"/status", "/tools", "/skills", "/memory", "/soul", "/schedule", "/history", "/clear", "/help", "/model", "/debug", "/hub", "/setup", "/onboarding", "/exit", "/quit"}
		newAutocomplete := filterStrings(commands, value)

		wasShowing := m.showAutocomplete
		m.showAutocomplete = len(newAutocomplete) > 0 && value != newAutocomplete[0]

		if !wasShowing && m.showAutocomplete {
			m.autocompleteIndex = 0
		}

		m.autocomplete = newAutocomplete
	} else {
		m.showAutocomplete = false
		m.autocomplete = []string{}
		m.autocompleteIndex = 0
	}
}

// filterStrings returns items that start with prefix (but not exact matches)
func filterStrings(items []string, prefix string) []string {
	var filtered []string
	for _, s := range items {
		if strings.HasPrefix(s, prefix) && s != prefix {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// isSensitiveCommand checks if a command is potentially dangerous
func isSensitiveCommand(cmd string) bool {
	cmdLower := strings.ToLower(strings.TrimSpace(cmd))
	cmdLower = strings.TrimPrefix(cmdLower, "!")

	patterns := []string{
		"rm -rf", "rm -r", "rm ", "sudo rm", "delete", "files_delete", "rmdir", "unlink",
		"sudo ", "su ", "doas ",
		"mkfs", "dd if=", "format", "fdisk", "parted",
		"shutdown", "reboot", "halt", "poweroff",
		"systemctl stop", "systemctl disable",
		"kill -9", "pkill", "killall",
		"chmod 777", "chmod -r 777", "chown",
		"curl | sh", "wget | sh", "curl | bash", "wget | bash",
	}

	for _, pattern := range patterns {
		if strings.Contains(cmdLower, pattern) {
			return true
		}
	}
	return false
}

// getSensitiveMessage returns a warning message for sensitive commands
func getSensitiveMessage(cmd string) string {
	cmdLower := strings.ToLower(cmd)
	switch {
	case strings.Contains(cmdLower, "rm -rf"):
		return "Recursive force delete - files CANNOT be recovered!"
	case strings.Contains(cmdLower, "rm -r") || strings.Contains(cmdLower, "rmdir"):
		return "This will delete directories and their contents."
	case strings.Contains(cmdLower, "rm ") || strings.Contains(cmdLower, "delete"):
		return "This command will DELETE files permanently."
	case strings.Contains(cmdLower, "sudo") || strings.Contains(cmdLower, "su "):
		return "This command requires ELEVATED PRIVILEGES."
	case strings.Contains(cmdLower, "shutdown") || strings.Contains(cmdLower, "reboot") ||
		strings.Contains(cmdLower, "halt") || strings.Contains(cmdLower, "poweroff"):
		return "This will SHUTDOWN or REBOOT your system."
	case strings.Contains(cmdLower, "kill") || strings.Contains(cmdLower, "pkill"):
		return "This will TERMINATE running processes."
	case strings.Contains(cmdLower, "mkfs") || strings.Contains(cmdLower, "format") ||
		strings.Contains(cmdLower, "fdisk") || strings.Contains(cmdLower, "dd if="):
		return "This can DESTROY entire disk partitions!"
	case strings.Contains(cmdLower, "chmod 777"):
		return "This makes files world-writable!"
	case strings.Contains(cmdLower, "curl | sh") || strings.Contains(cmdLower, "wget | bash"):
		return "Executing remote scripts without review!"
	default:
		return "This is a potentially dangerous operation."
	}
}

// formatAIResponse formats AI response with clean, minimal styling
func formatAIResponse(text string) string {
	if text == "" {
		return colorMuted("  (empty response)")
	}

	var sb strings.Builder

	// Clean label
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("246")).
		Render("  assistant"))
	sb.WriteString("\n")

	// Process text for code blocks and formatting
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	codeLanguage := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				codeLanguage = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if codeLanguage == "" {
					codeLanguage = "code"
				}
				// Code block header - subtle
				sb.WriteString("\n")
				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("240")).
					Render("    "+codeLanguage))
				sb.WriteString("\n")
				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("238")).
					Render("   "+strings.Repeat("─", 50)))
				sb.WriteString("\n")
			} else {
				// Code block footer
				sb.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("238")).
					Render("   "+strings.Repeat("─", 50)))
				sb.WriteString("\n")
			}
			continue
		}

		if inCodeBlock {
			styledLine := highlightCodeLine(line, codeLanguage)
			sb.WriteString("    " + styledLine + "\n")
		} else if strings.HasPrefix(line, "##") {
			heading := strings.TrimLeft(line, "# ")
			sb.WriteString("\n  " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Bold(true).
				Render(heading) + "\n")
		} else if strings.HasPrefix(line, "#") {
			heading := strings.TrimLeft(line, "# ")
			sb.WriteString("\n  " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Bold(true).
				Underline(true).
				Render(heading) + "\n")
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
			trimmed := strings.TrimLeft(line, "-*• ")
			sb.WriteString("  " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("246")).
				Render("  - ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("252")).
					Render(trimmed) + "\n")
		} else if strings.HasPrefix(line, ">") {
			quoted := strings.TrimLeft(line, "> ")
			sb.WriteString("  " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("238")).
				Render("  | ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("244")).
					Italic(true).
					Render(quoted) + "\n")
		} else if strings.Contains(line, "`") && !inCodeBlock {
			styledLine := highlightInlineCode(line)
			sb.WriteString("  " + styledLine + "\n")
		} else if line == "" {
			sb.WriteString("\n")
		} else {
			sb.WriteString("  " + lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Render(line) + "\n")
		}
	}

	return sb.String()
}

// highlightCodeLine adds basic syntax highlighting to code
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

	// Comments
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")).
			Italic(true).
			Render(line)
	}

	// Strings
	if strings.Contains(line, "\"") || strings.Contains(line, "'") {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("179")).
			Render(line)
	}

	// Check for keywords
	lang := strings.ToLower(language)
	if kwList, ok := keywords[lang]; ok {
		words := strings.Fields(trimmed)
		for _, word := range words {
			for _, kw := range kwList {
				if word == kw || strings.HasPrefix(word, kw+"(") || strings.HasPrefix(word, kw+".") {
					return lipgloss.NewStyle().
						Foreground(lipgloss.Color("176")).
						Render(line)
				}
			}
		}
	}

	// Default code color
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("117")).
		Render(line)
}

// highlightInlineCode highlights `code` in text
func highlightInlineCode(text string) string {
	parts := strings.Split(text, "`")
	var sb strings.Builder

	for i, part := range parts {
		if i%2 == 1 {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("117")).
				Background(lipgloss.Color("236")).
				Render(" "+part+" "))
		} else {
			sb.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Render(part))
		}
	}

	return sb.String()
}
