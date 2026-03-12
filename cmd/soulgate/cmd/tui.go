package cmd

import (
	"fmt"
	"strings"
	"time"
)

// TUI components for SoulGate terminal interface
// Inspired by OpenClaw's TUI but optimized for Go

// Spinner represents an animated spinner
type Spinner struct {
	frames   []string
	current  int
	message  string
	done     bool
	stopChan chan bool
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames: []string{
			"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
		},
		message:  message,
		stopChan: make(chan bool),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopChan:
				return
			case <-ticker.C:
				if !s.done {
					fmt.Printf("\r  %s %s", colorAccent(s.frames[s.current]), colorMuted(s.message))
					s.current = (s.current + 1) % len(s.frames)
				}
			}
		}
	}()
}

// Update changes the spinner message
func (s *Spinner) Update(message string) {
	s.message = message
}

// Stop stops the spinner and shows a final message
func (s *Spinner) Stop(finalMessage string, status string) {
	s.done = true
	s.stopChan <- true

	// Clear the spinner line
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")

	// Print final status
	switch status {
	case "success":
		fmt.Printf("  %s %s\n", colorSuccess("✓"), finalMessage)
	case "error":
		fmt.Printf("  %s %s\n", colorError("✗"), finalMessage)
	case "info":
		fmt.Printf("  %s %s\n", colorInfo("●"), finalMessage)
	default:
		fmt.Printf("  %s\n", finalMessage)
	}
}

// ProgressBar represents a progress bar
type ProgressBar struct {
	width   int
	current int
	total   int
	label   string
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int, width int, label string) *ProgressBar {
	return &ProgressBar{
		width:   width,
		current: 0,
		total:   total,
		label:   label,
	}
}

// Update updates the progress bar
func (p *ProgressBar) Update(current int) {
	p.current = current
	p.render()
}

// Increment increments the progress
func (p *ProgressBar) Increment() {
	p.current++
	p.render()
}

// render draws the progress bar
func (p *ProgressBar) render() {
	percent := float64(p.current) / float64(p.total)
	filled := int(percent * float64(p.width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	fmt.Printf("\r  %s %s %3.0f%% (%d/%d)",
		colorMuted(p.label),
		colorAccent(bar),
		percent*100,
		p.current,
		p.total)
}

// Finish completes the progress bar
func (p *ProgressBar) Finish() {
	p.Update(p.total)
	fmt.Println()
}

// Box draws a box around text
func Box(title string, content string, width int) string {
	var sb strings.Builder

	// Top border
	sb.WriteString(colorAccent("╭─" + " " + title + " "))
	padding := width - len(title) - 6
	if padding > 0 {
		sb.WriteString(colorAccent(strings.Repeat("─", padding)))
	}
	sb.WriteString(colorAccent("╮\n"))

	// Content lines
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		sb.WriteString(colorAccent("│") + " " + line)
		linePadding := width - visibleLength(line) - 4
		if linePadding > 0 {
			sb.WriteString(strings.Repeat(" ", linePadding))
		}
		sb.WriteString(" " + colorAccent("│") + "\n")
	}

	// Bottom border
	sb.WriteString(colorAccent("╰" + strings.Repeat("─", width-2) + "╯\n"))

	return sb.String()
}

// Table creates a simple table
type Table struct {
	headers []string
	rows    [][]string
}

// NewTable creates a new table
func NewTable(headers []string) *Table {
	return &Table{
		headers: headers,
		rows:    [][]string{},
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(row []string) {
	t.rows = append(t.rows, row)
}

// Render renders the table
func (t *Table) Render() string {
	if len(t.rows) == 0 {
		return ""
	}

	// Calculate column widths
	widths := make([]int, len(t.headers))
	for i, header := range t.headers {
		widths[i] = len(header)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) && visibleLength(cell) > widths[i] {
				widths[i] = visibleLength(cell)
			}
		}
	}

	var sb strings.Builder

	// Top border
	sb.WriteString(colorAccent("┌"))
	for i, width := range widths {
		sb.WriteString(colorAccent(strings.Repeat("─", width+2)))
		if i < len(widths)-1 {
			sb.WriteString(colorAccent("┬"))
		}
	}
	sb.WriteString(colorAccent("┐\n"))

	// Headers
	sb.WriteString(colorAccent("│"))
	for i, header := range t.headers {
		sb.WriteString(" " + colorBold(header) + strings.Repeat(" ", widths[i]-len(header)+1))
		sb.WriteString(colorAccent("│"))
	}
	sb.WriteString("\n")

	// Header separator
	sb.WriteString(colorAccent("├"))
	for i, width := range widths {
		sb.WriteString(colorAccent(strings.Repeat("─", width+2)))
		if i < len(widths)-1 {
			sb.WriteString(colorAccent("┼"))
		}
	}
	sb.WriteString(colorAccent("┤\n"))

	// Rows
	for _, row := range t.rows {
		sb.WriteString(colorAccent("│"))
		for i, cell := range row {
			if i < len(widths) {
				sb.WriteString(" " + cell + strings.Repeat(" ", widths[i]-visibleLength(cell)+1))
				sb.WriteString(colorAccent("│"))
			}
		}
		sb.WriteString("\n")
	}

	// Bottom border
	sb.WriteString(colorAccent("└"))
	for i, width := range widths {
		sb.WriteString(colorAccent(strings.Repeat("─", width+2)))
		if i < len(widths)-1 {
			sb.WriteString(colorAccent("┴"))
		}
	}
	sb.WriteString(colorAccent("┘\n"))

	return sb.String()
}

// visibleLength calculates the visible length of a string (excluding ANSI codes)
func visibleLength(s string) int {
	// Simple ANSI code removal (basic implementation)
	inEscape := false
	length := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
		} else if inEscape && r == 'm' {
			inEscape = false
		} else if !inEscape {
			length++
		}
	}
	return length
}

// ToolExecution displays a tool execution
type ToolExecution struct {
	Name   string
	Args   string
	Status string
	Result string
}

// Render renders a tool execution display
func (te *ToolExecution) Render() string {
	var sb strings.Builder

	// Tool header
	statusColor := colorAccent
	statusIcon := "●"
	switch te.Status {
	case "running":
		statusIcon = "⟳"
		statusColor = colorInfo
	case "success":
		statusIcon = "✓"
		statusColor = colorSuccess
	case "error":
		statusIcon = "✗"
		statusColor = colorError
	}

	sb.WriteString(fmt.Sprintf("  %s %s %s\n",
		statusColor(statusIcon),
		colorBold(te.Name),
		colorMuted(te.Args)))

	// Result (if available)
	if te.Result != "" {
		// Show first 3 lines of result
		lines := strings.Split(te.Result, "\n")
		maxLines := 3
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}
		for _, line := range lines {
			if len(line) > 80 {
				line = line[:80] + "..."
			}
			sb.WriteString(colorMuted("    " + line + "\n"))
		}
		if len(strings.Split(te.Result, "\n")) > maxLines {
			sb.WriteString(colorMuted("    [...]\n"))
		}
	}

	return sb.String()
}

// Banner displays the SoulGate banner
func Banner() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(colorAccent("  ╔═══════════════════════════════════════════════════════════╗\n"))
	sb.WriteString(colorAccent("  ║") + "                                                           " + colorAccent("║\n"))
	sb.WriteString(colorAccent("  ║") + "         " + colorAccentBright("🐙  SoulGate") + "  -  Your AI Guardian              " + colorAccent("║\n"))
	sb.WriteString(colorAccent("  ║") + "                                                           " + colorAccent("║\n"))
	sb.WriteString(colorAccent("  ╚═══════════════════════════════════════════════════════════╝\n"))
	sb.WriteString("\n")
	sb.WriteString(colorMuted("      Intelligent • Adaptable • Multi-Tasking\n"))
	sb.WriteString("\n")

	return sb.String()
}

// StatusLine displays a status line
func StatusLine(items map[string]string) string {
	var parts []string
	for key, value := range items {
		parts = append(parts, colorMuted(key+":") + " " + colorAccent(value))
	}
	return "  " + strings.Join(parts, colorMuted(" • ")) + "\n"
}

// WaitingMessages are whimsical waiting phrases
var WaitingMessages = []string{
	"pondering the mysteries",
	"contemplating deeply",
	"gathering thoughts",
	"processing wisely",
	"thinking carefully",
	"considering options",
	"analyzing data",
	"computing answers",
	"formulating response",
	"brewing wisdom",
}

// GetWaitingMessage returns a waiting message
func GetWaitingMessage(tick int) string {
	return WaitingMessages[tick%len(WaitingMessages)]
}
