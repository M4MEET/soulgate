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
		// Command autocomplete
		commands := []string{"/status", "/tools", "/skills", "/memory", "/soul", "/schedule", "/history", "/clear", "/help", "/model", "/debug", "/hub", "/setup", "/onboarding", "/exit", "/quit"}
		newAutocomplete := filterStrings(commands, value)

		// Only show if there are suggestions and not exact match
		wasShowing := m.showAutocomplete
		m.showAutocomplete = len(newAutocomplete) > 0 && value != newAutocomplete[0]

		// Reset index if suggestions changed
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
	cmdLower = strings.TrimPrefix(cmdLower, "!") // Remove ! prefix for shell commands

	// Dangerous deletion patterns
	deletionPatterns := []string{
		"rm -rf",
		"rm -r",
		"rm ",
		"sudo rm",
		"delete",
		"files_delete",
		"rmdir",
		"unlink",
	}

	// Privilege escalation
	privilegePatterns := []string{
		"sudo ",
		"su ",
		"doas ",
	}

	// System modification
	systemPatterns := []string{
		"mkfs",
		"dd if=",
		"format",
		"fdisk",
		"parted",
		"shutdown",
		"reboot",
		"halt",
		"poweroff",
		"systemctl stop",
		"systemctl disable",
		"kill -9",
		"pkill",
		"killall",
	}

	// Network/security sensitive
	networkPatterns := []string{
		"chmod 777",
		"chmod -r 777",
		"chown",
		"curl | sh",
		"wget | sh",
		"curl | bash",
		"wget | bash",
	}

	allPatterns := append(deletionPatterns, privilegePatterns...)
	allPatterns = append(allPatterns, systemPatterns...)
	allPatterns = append(allPatterns, networkPatterns...)

	for _, pattern := range allPatterns {
		if strings.Contains(cmdLower, pattern) {
			return true
		}
	}

	return false
}

// getSensitiveMessage returns a warning message for sensitive commands
func getSensitiveMessage(cmd string) string {
	cmdLower := strings.ToLower(cmd)

	// Check for different types of dangerous operations
	if strings.Contains(cmdLower, "rm -rf") {
		return "⚠️  DANGER: Recursive force delete - files CANNOT be recovered!"
	}
	if strings.Contains(cmdLower, "rm -r") || strings.Contains(cmdLower, "rmdir") {
		return "⚠️  WARNING: This will delete directories and their contents."
	}
	if strings.Contains(cmdLower, "rm ") || strings.Contains(cmdLower, "delete") {
		return "⚠️  This command will DELETE files permanently."
	}
	if strings.Contains(cmdLower, "sudo") || strings.Contains(cmdLower, "su ") {
		return "⚠️  This command requires ELEVATED PRIVILEGES."
	}
	if strings.Contains(cmdLower, "shutdown") || strings.Contains(cmdLower, "reboot") ||
	   strings.Contains(cmdLower, "halt") || strings.Contains(cmdLower, "poweroff") {
		return "⚠️  This will SHUTDOWN or REBOOT your system."
	}
	if strings.Contains(cmdLower, "kill") || strings.Contains(cmdLower, "pkill") {
		return "⚠️  This will TERMINATE running processes."
	}
	if strings.Contains(cmdLower, "mkfs") || strings.Contains(cmdLower, "format") ||
	   strings.Contains(cmdLower, "fdisk") || strings.Contains(cmdLower, "dd if=") {
		return "⚠️  EXTREME DANGER: This can DESTROY entire disk partitions!"
	}
	if strings.Contains(cmdLower, "chmod 777") {
		return "⚠️  SECURITY RISK: This makes files world-writable!"
	}
	if strings.Contains(cmdLower, "curl | sh") || strings.Contains(cmdLower, "wget | bash") {
		return "⚠️  SECURITY RISK: Executing remote scripts without review!"
	}

	return "⚠️  This is a potentially dangerous operation."
}

// formatAIResponse formats AI response with elegant styling
func formatAIResponse(text string) string {
	if text == "" {
		return colorMuted("(empty response)")
	}

	var sb strings.Builder

	// Header with gradient effect
	sb.WriteString(colorAccent("╭─ 🐙 "))
	sb.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")).
		Bold(true).
		Render("AI Response"))
	sb.WriteString(colorAccent(" "))
	sb.WriteString(colorAccent(strings.Repeat("─", 30)))
	sb.WriteString("\n")
	sb.WriteString(colorAccent("│") + "\n")

	// Process text for code blocks and formatting
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	codeLanguage := ""

	for _, line := range lines {
		// Detect code blocks
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				// Extract language
				codeLanguage = strings.TrimPrefix(line, "```")
				codeLanguage = strings.TrimSpace(codeLanguage)
				if codeLanguage == "" {
					codeLanguage = "code"
				}

				// Pretty code block header
				sb.WriteString(colorAccent("│ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("39")).
						Render("┌─ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("45")).
						Bold(true).
						Render(codeLanguage+" ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("39")).
						Render(strings.Repeat("─", 42-len(codeLanguage))) + "\n")
			} else {
				// Code block footer
				sb.WriteString(colorAccent("│ ") +
					lipgloss.NewStyle().
						Foreground(lipgloss.Color("39")).
						Render("└"+strings.Repeat("─", 48)) + "\n")
			}
			continue
		}

		// Format based on context
		if inCodeBlock {
			// Code block - syntax highlighting by color
			styledLine := highlightCodeLine(line, codeLanguage)
			sb.WriteString(colorAccent("│ ") + "  " + styledLine + "\n")
		} else if strings.HasPrefix(line, "✓") || strings.HasPrefix(line, "✅") {
			// Success message with icon
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("82")).
					Bold(true).
					Render(line) + "\n")
		} else if strings.HasPrefix(line, "✗") || strings.HasPrefix(line, "❌") {
			// Error message with icon
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("196")).
					Bold(true).
					Render(line) + "\n")
		} else if strings.HasPrefix(line, "⚠") || strings.HasPrefix(line, "⚠️") {
			// Warning message
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("226")).
					Bold(true).
					Render(line) + "\n")
		} else if strings.HasPrefix(line, "##") {
			// H2 Heading
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("214")).
					Bold(true).
					Render(line) + "\n")
		} else if strings.HasPrefix(line, "#") {
			// H1 Heading
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("208")).
					Bold(true).
					Underline(true).
					Render(line) + "\n")
		} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "•") {
			// List item with prettier bullet
			trimmed := strings.TrimLeft(line, "- *•")
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("45")).
					Render("  • ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("252")).
					Render(trimmed) + "\n")
		} else if strings.HasPrefix(line, ">") {
			// Quote/blockquote
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("244")).
					Italic(true).
					Render(line) + "\n")
		} else if strings.Contains(line, "`") && !inCodeBlock {
			// Inline code
			styledLine := highlightInlineCode(line)
			sb.WriteString(colorAccent("│ ") + styledLine + "\n")
		} else {
			// Normal text
			sb.WriteString(colorAccent("│ ") +
				lipgloss.NewStyle().
					Foreground(lipgloss.Color("252")).
					Render(line) + "\n")
		}
	}

	// Footer
	sb.WriteString(colorAccent("│") + "\n")
	sb.WriteString(colorAccent("╰"))
	sb.WriteString(colorAccent(strings.Repeat("─", 54)))

	return sb.String()
}

// highlightCodeLine adds basic syntax highlighting to code
func highlightCodeLine(line string, language string) string {
	// Keywords by language
	keywords := map[string][]string{
		"go":     {"func", "package", "import", "type", "struct", "interface", "return", "if", "else", "for", "range"},
		"python": {"def", "class", "import", "from", "return", "if", "else", "for", "while", "try", "except"},
		"javascript": {"function", "const", "let", "var", "return", "if", "else", "for", "while", "class"},
		"bash":   {"if", "then", "else", "fi", "for", "do", "done", "while", "case", "esac"},
	}

	// Apply basic coloring
	styledLine := line

	// Comments
	if strings.Contains(line, "//") || strings.Contains(line, "#") {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			Render(line)
	}

	// Strings
	if strings.Contains(line, "\"") || strings.Contains(line, "'") {
		styledLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("179")).
			Render(line)
		return styledLine
	}

	// Numbers
	for _, char := range line {
		if char >= '0' && char <= '9' {
			return lipgloss.NewStyle().
				Foreground(lipgloss.Color("141")).
				Render(line)
		}
	}

	// Check for keywords
	lang := strings.ToLower(language)
	if kwList, ok := keywords[lang]; ok {
		for _, kw := range kwList {
			if strings.Contains(line, kw) {
				return lipgloss.NewStyle().
					Foreground(lipgloss.Color("213")).
					Bold(true).
					Render(line)
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
	result := ""

	for i, part := range parts {
		if i%2 == 1 {
			// Inside backticks
			result += lipgloss.NewStyle().
				Foreground(lipgloss.Color("117")).
				Background(lipgloss.Color("236")).
				Render(part)
		} else {
			result += lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Render(part)
		}
	}

	return result
}
