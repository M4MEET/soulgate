package observer

import (
	"fmt"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
)

// Color codes for terminal output
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorGray    = "\033[90m"
	ColorBold    = "\033[1m"
)

// Formatter formats frames for display
type Formatter struct {
	verbose bool
}

// NewFormatter creates a new formatter
func NewFormatter(verbose bool) *Formatter {
	return &Formatter{
		verbose: verbose,
	}
}

// FormatEventMessage formats an incoming message event
func (f *Formatter) FormatEventMessage(frame *protocol.EventMessageFrame) {
	timestamp := f.formatTimestamp(frame.Timestamp)
	sender := frame.Sender.Username
	if sender == "" {
		sender = frame.Sender.ID
	}

	fmt.Printf("%s%s📨 Message%s [%s%s%s] from %s@%s:%s\n",
		ColorBold, ColorBlue, ColorReset,
		ColorCyan, frame.Channel, ColorReset,
		ColorGreen, sender, ColorReset,
	)
	fmt.Printf("   %s%s%s\n", ColorGray, frame.Text, ColorReset)

	if f.verbose {
		fmt.Printf("   %sConversation: %s | Session: %s | Time: %s%s\n",
			ColorGray, frame.ConversationID, frame.SessionID, timestamp, ColorReset)
	}
	fmt.Println()
}

// FormatToolStart formats a tool start event
func (f *Formatter) FormatToolStart(frame *protocol.EventToolStartFrame) {
	timestamp := f.formatTimestamp(frame.Timestamp)

	fmt.Printf("%s%s🔧 Tool Started%s [%s%s%s]:\n",
		ColorBold, ColorYellow, ColorReset,
		ColorCyan, frame.ToolName, ColorReset,
	)

	if len(frame.Args) > 0 {
		fmt.Printf("   %sArgs:%s\n", ColorGray, ColorReset)
		for key, value := range frame.Args {
			valueStr := fmt.Sprintf("%v", value)
			if len(valueStr) > 100 {
				valueStr = valueStr[:97] + "..."
			}
			fmt.Printf("     %s%s%s = %s\n", ColorCyan, key, ColorReset, valueStr)
		}
	}

	if f.verbose {
		fmt.Printf("   %sSession: %s | Tool ID: %s | Time: %s%s\n",
			ColorGray, frame.SessionID, frame.ToolID, timestamp, ColorReset)
	}
	fmt.Println()
}

// FormatToolEnd formats a tool end event
func (f *Formatter) FormatToolEnd(frame *protocol.EventToolEndFrame) {
	timestamp := f.formatTimestamp(frame.Timestamp)
	duration := f.formatDuration(frame.Duration)

	status := "✅ Tool Completed"
	statusColor := ColorGreen
	if frame.Error != "" {
		status = "❌ Tool Failed"
		statusColor = ColorRed
	}

	fmt.Printf("%s%s%s%s [%s%s%s] %s(%s)%s:\n",
		ColorBold, statusColor, status, ColorReset,
		ColorCyan, frame.ToolName, ColorReset,
		ColorGray, duration, ColorReset,
	)

	if frame.Error != "" {
		fmt.Printf("   %sError:%s %s%s%s\n",
			ColorRed, ColorReset, ColorRed, frame.Error, ColorReset)
	} else if frame.Result != nil {
		resultStr := f.formatResult(frame.Result)
		fmt.Printf("   %sResult:%s\n%s\n", ColorGray, ColorReset, resultStr)
	}

	if f.verbose {
		fmt.Printf("   %sSession: %s | Tool ID: %s | Time: %s%s\n",
			ColorGray, frame.SessionID, frame.ToolID, timestamp, ColorReset)
	}
	fmt.Println()
}

// FormatToolLog formats a tool log event
func (f *Formatter) FormatToolLog(frame *protocol.EventToolLogFrame) {
	icon := "ℹ️"
	color := ColorBlue
	levelUpper := strings.ToUpper(frame.Level)

	switch frame.Level {
	case "warn", "warning":
		icon = "⚠️"
		color = ColorYellow
	case "error":
		icon = "❌"
		color = ColorRed
	case "info":
		icon = "ℹ️"
		color = ColorBlue
	}

	fmt.Printf("   %s%s %s%s %s%s%s\n",
		color, icon, levelUpper, ColorReset,
		ColorGray, frame.Message, ColorReset,
	)
}

// FormatToolProgress formats a tool progress event
func (f *Formatter) FormatToolProgress(frame *protocol.EventToolProgressFrame) {
	percentage := int(frame.Progress * 100)

	// Create progress bar
	barWidth := 20
	filled := int(float64(barWidth) * frame.Progress)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	fmt.Printf("   %s⏳ Progress%s [%s%d%%%s] %s%s%s %s%s%s\n",
		ColorYellow, ColorReset,
		ColorCyan, percentage, ColorReset,
		ColorGray, bar, ColorReset,
		ColorGray, frame.Status, ColorReset,
	)

	// Show current/total if available
	if frame.Total > 0 {
		unitStr := frame.Unit
		if unitStr == "" {
			unitStr = "items"
		}
		fmt.Printf("     %s%d/%d %s%s\n",
			ColorGray, frame.Current, frame.Total, unitStr, ColorReset,
		)
	}
}

// FormatToolOutput formats a tool output stream event
func (f *Formatter) FormatToolOutput(frame *protocol.EventToolOutputFrame) {
	streamColor := ColorGray
	streamIcon := "📤"

	switch frame.Stream {
	case "stdout":
		streamColor = ColorGreen
		streamIcon = "📤"
	case "stderr":
		streamColor = ColorRed
		streamIcon = "📤"
	}

	// Show line number if available
	if frame.LineNum > 0 {
		fmt.Printf("   %s%s [%s L%d]%s %s\n",
			streamColor, streamIcon, frame.Stream, frame.LineNum, ColorReset,
			frame.Data,
		)
	} else {
		fmt.Printf("   %s%s [%s]%s %s\n",
			streamColor, streamIcon, frame.Stream, ColorReset,
			frame.Data,
		)
	}
}

// FormatChannelSend formats a channel send command
func (f *Formatter) FormatChannelSend(frame *protocol.CmdChannelSendFrame) {
	timestamp := f.formatTimestamp(frame.Timestamp)

	fmt.Printf("%s%s🤖 Agent Response%s [%s%s%s]:\n",
		ColorBold, ColorMagenta, ColorReset,
		ColorCyan, frame.Channel, ColorReset,
	)
	fmt.Printf("   %s%s%s\n", ColorGray, frame.Text, ColorReset)

	if f.verbose {
		fmt.Printf("   %sConversation: %s | Session: %s | Time: %s%s\n",
			ColorGray, frame.ConversationID, frame.SessionID, timestamp, ColorReset)
	}
	fmt.Println()
}

// FormatError formats an error event
func (f *Formatter) FormatError(frame *protocol.EventErrorFrame) {
	timestamp := f.formatTimestamp(frame.Timestamp)

	fmt.Printf("%s%s❌ Error%s",
		ColorBold, ColorRed, ColorReset,
	)

	if frame.Code != "" {
		fmt.Printf(" [%s%s%s]", ColorYellow, frame.Code, ColorReset)
	}

	fmt.Printf(":\n   %s%s%s\n", ColorRed, frame.Error, ColorReset)

	if f.verbose && frame.SessionID != "" {
		fmt.Printf("   %sSession: %s | Time: %s%s\n",
			ColorGray, frame.SessionID, timestamp, ColorReset)
	}
	fmt.Println()
}

// FormatGeneric formats a generic frame
func (f *Formatter) FormatGeneric(frame *protocol.Frame) {
	if !f.verbose {
		return
	}

	timestamp := f.formatTimestamp(frame.Timestamp)
	fmt.Printf("%s📋 %s%s (Time: %s)\n",
		ColorGray, frame.Type, ColorReset, timestamp)

	if len(frame.Data) > 0 {
		fmt.Printf("   Data: %v\n", frame.Data)
	}
	fmt.Println()
}

// formatTimestamp formats a Unix timestamp
func (f *Formatter) formatTimestamp(ts int64) string {
	t := time.Unix(ts, 0)
	return t.Format("15:04:05")
}

// formatDuration formats a duration in milliseconds
func (f *Formatter) formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.2fs", float64(ms)/1000.0)
}

// formatResult formats a tool result for display
func (f *Formatter) formatResult(result interface{}) string {
	resultStr := fmt.Sprintf("%v", result)

	// Truncate long results
	lines := strings.Split(resultStr, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
		lines = append(lines, fmt.Sprintf("%s... (%d more lines)%s", ColorGray, len(lines)-20, ColorReset))
	}

	// Indent each line
	var formatted []string
	for _, line := range lines {
		if len(line) > 200 {
			line = line[:197] + "..."
		}
		formatted = append(formatted, fmt.Sprintf("     %s%s%s", ColorGray, line, ColorReset))
	}

	return strings.Join(formatted, "\n")
}
