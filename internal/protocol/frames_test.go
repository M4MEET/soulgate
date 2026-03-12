package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectFrame(t *testing.T) {
	frame := &ConnectFrame{
		Type:      FrameConnect,
		Role:      RoleAgent,
		ClientID:  "agent-1",
		Version:   ProtocolVersion,
		Timestamp: time.Now().Unix(),
	}

	// Validate
	err := ValidateFrame(frame)
	require.NoError(t, err)

	// Serialize
	data, err := ToJSON(frame)
	require.NoError(t, err)

	// Parse
	parsed, err := ParseFrame(data)
	require.NoError(t, err)

	parsedFrame, ok := parsed.(*ConnectFrame)
	require.True(t, ok)
	assert.Equal(t, RoleAgent, parsedFrame.Role)
	assert.Equal(t, "agent-1", parsedFrame.ClientID)
}

func TestEventMessageFrame(t *testing.T) {
	frame := &EventMessageFrame{
		Type:           FrameEventMessage,
		Channel:        "telegram",
		ConversationID: "123",
		Text:           "Hello",
		Sender: Sender{
			ID:       "user-1",
			Username: "testuser",
		},
		Timestamp: time.Now().Unix(),
	}

	// Validate
	err := ValidateFrame(frame)
	require.NoError(t, err)

	// Serialize
	data, err := ToJSON(frame)
	require.NoError(t, err)

	// Parse
	parsed, err := ParseFrame(data)
	require.NoError(t, err)

	parsedFrame, ok := parsed.(*EventMessageFrame)
	require.True(t, ok)
	assert.Equal(t, "telegram", parsedFrame.Channel)
	assert.Equal(t, "Hello", parsedFrame.Text)
}

func TestEventToolFrames(t *testing.T) {
	// Tool start
	startFrame := &EventToolStartFrame{
		Type:      FrameEventToolStart,
		SessionID: "sess-1",
		ToolName:  "bash",
		ToolID:    "tool-exec-1",
		Args: map[string]interface{}{
			"command": "ls -la",
		},
		Timestamp: time.Now().Unix(),
	}

	data, err := ToJSON(startFrame)
	require.NoError(t, err)

	parsed, err := ParseFrame(data)
	require.NoError(t, err)
	_, ok := parsed.(*EventToolStartFrame)
	require.True(t, ok)

	// Tool end
	endFrame := &EventToolEndFrame{
		Type:      FrameEventToolEnd,
		SessionID: "sess-1",
		ToolName:  "bash",
		ToolID:    "tool-exec-1",
		Result:    "file1.txt\nfile2.txt",
		Duration:  150,
		Timestamp: time.Now().Unix(),
	}

	data, err = ToJSON(endFrame)
	require.NoError(t, err)

	parsed, err = ParseFrame(data)
	require.NoError(t, err)
	parsedEnd, ok := parsed.(*EventToolEndFrame)
	require.True(t, ok)
	assert.Equal(t, int64(150), parsedEnd.Duration)
}

func TestCmdChannelSendFrame(t *testing.T) {
	frame := &CmdChannelSendFrame{
		Type:           FrameCmdChannelSend,
		Channel:        "telegram",
		ConversationID: "123",
		Text:           "Response from agent",
		SessionID:      "sess-1",
		Timestamp:      time.Now().Unix(),
	}

	// Validate
	err := ValidateFrame(frame)
	require.NoError(t, err)

	// Serialize
	data, err := ToJSON(frame)
	require.NoError(t, err)

	// Parse
	parsed, err := ParseFrame(data)
	require.NoError(t, err)

	parsedFrame, ok := parsed.(*CmdChannelSendFrame)
	require.True(t, ok)
	assert.Equal(t, "telegram", parsedFrame.Channel)
	assert.Equal(t, "Response from agent", parsedFrame.Text)
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		frame   interface{}
		wantErr bool
	}{
		{
			name: "valid connect frame",
			frame: &ConnectFrame{
				Type:     FrameConnect,
				Role:     RoleAgent,
				ClientID: "agent-1",
			},
			wantErr: false,
		},
		{
			name: "missing role",
			frame: &ConnectFrame{
				Type:     FrameConnect,
				ClientID: "agent-1",
			},
			wantErr: true,
		},
		{
			name: "missing clientId",
			frame: &ConnectFrame{
				Type: FrameConnect,
				Role: RoleAgent,
			},
			wantErr: true,
		},
		{
			name: "valid event message",
			frame: &EventMessageFrame{
				Type:           FrameEventMessage,
				Channel:        "telegram",
				ConversationID: "123",
				Text:           "Hello",
			},
			wantErr: false,
		},
		{
			name: "missing channel",
			frame: &EventMessageFrame{
				Type:           FrameEventMessage,
				ConversationID: "123",
				Text:           "Hello",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFrame(tt.frame)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := ParseFrame([]byte("invalid json"))
	assert.Error(t, err)
}

func TestParseUnknownFrameType(t *testing.T) {
	data := []byte(`{"type": "unknown.frame", "ts": 123456}`)
	_, err := ParseFrame(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown frame type")
}

func TestMetadata(t *testing.T) {
	metadata := Metadata{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	frame := &ConnectFrame{
		Type:      FrameConnect,
		Role:      RoleAgent,
		ClientID:  "agent-1",
		Metadata:  metadata,
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(frame)
	require.NoError(t, err)

	var parsed ConnectFrame
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "value1", parsed.Metadata["key1"])
	assert.Equal(t, float64(123), parsed.Metadata["key2"]) // JSON numbers become float64
	assert.Equal(t, true, parsed.Metadata["key3"])
}
