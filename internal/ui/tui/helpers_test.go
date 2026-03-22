package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/stretchr/testify/require"
)

func TestEnsureThinkingPlaceholderInsertsBeforeStreamPanel(t *testing.T) {
	m := InteractiveChatModel{
		messages:           []string{"msg-1", formatAIStreamingResponse("")},
		streamPanelIndex:   1,
		thinkingPanelIndex: -1,
	}

	m.ensureThinkingPlaceholder()

	require.Len(t, m.messages, 3)
	require.Equal(t, 1, m.thinkingPanelIndex)
	require.Equal(t, 2, m.streamPanelIndex)
	require.True(t, strings.HasPrefix(m.messages[m.thinkingPanelIndex], "  thinking"))
}

func TestFlushStreamingPreviewRendersThinkingAndAssistantPanels(t *testing.T) {
	m := InteractiveChatModel{
		messages:             []string{formatThinkingPanel(""), formatAIStreamingResponse("")},
		thinkingPanelIndex:   0,
		streamPanelIndex:     1,
		thinkingBuffer:       formatThinkingStatus("retrying model call"),
		streamBuffer:         "partial response",
		streamFlushScheduled: true,
	}

	m.flushStreamingPreview()

	require.False(t, m.streamFlushScheduled)
	require.Contains(t, m.messages[m.thinkingPanelIndex], "retrying model call")
	require.Contains(t, m.messages[m.streamPanelIndex], "partial response")
}

func TestRefreshOutputRespectsAutoScroll(t *testing.T) {
	var msgs []string
	for i := 0; i < 40; i++ {
		msgs = append(msgs, "line")
	}

	m := InteractiveChatModel{
		messages: msgs,
		output:   viewport.New(80, 5),
	}

	m.autoScroll = false
	m.refreshOutput(true)
	require.False(t, m.output.AtBottom())

	m.autoScroll = true
	m.refreshOutput(true)
	require.True(t, m.output.AtBottom())
}
