package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// ProtocolVersion is the current protocol version
const ProtocolVersion = "1.0"

// ClientRole defines the role of a connected client
type ClientRole string

const (
	RoleAgent   ClientRole = "agent"
	RoleChannel ClientRole = "channel"
	RoleUI      ClientRole = "ui"
	RoleNode    ClientRole = "node"
)

// FrameType defines the type of frame being sent
type FrameType string

const (
	// Connection frames
	FrameConnect    FrameType = "connect"
	FrameConnectAck FrameType = "connect.ack"
	FrameDisconnect FrameType = "disconnect"
	FramePing       FrameType = "ping"
	FramePong       FrameType = "pong"

	// Event frames (Gateway → Clients)
	FrameEventMessage      FrameType = "event.message"
	FrameEventToolStart    FrameType = "event.tool.start"
	FrameEventToolEnd      FrameType = "event.tool.end"
	FrameEventToolLog      FrameType = "event.tool.log"
	FrameEventToolProgress FrameType = "event.tool.progress"
	FrameEventToolOutput   FrameType = "event.tool.output"
	FrameEventError        FrameType = "event.error"

	// Command frames (Clients → Gateway)
	FrameCmdChannelSend FrameType = "cmd.channel.send"
	FrameCmdApprove     FrameType = "cmd.approve"
	FrameCmdReject      FrameType = "cmd.reject"
)

// Frame is the base structure for all protocol frames
type Frame struct {
	Type      FrameType              `json:"type"`
	Timestamp int64                  `json:"ts"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NewFrame creates a new frame with timestamp
func NewFrame(frameType FrameType) *Frame {
	return &Frame{
		Type:      frameType,
		Timestamp: time.Now().Unix(),
		Data:      make(map[string]interface{}),
	}
}

// ConnectFrame is sent by clients to register with the Gateway
type ConnectFrame struct {
	Type      FrameType  `json:"type"` // "connect"
	Role      ClientRole `json:"role"` // agent, channel, ui, node
	ClientID  string     `json:"clientId"`
	Token     string     `json:"token,omitempty"` // Authentication token (optional)
	Version   string     `json:"version,omitempty"`
	Metadata  Metadata   `json:"metadata,omitempty"`
	Timestamp int64      `json:"ts"`
}

// ConnectAckFrame is sent by Gateway to acknowledge connection
type ConnectAckFrame struct {
	Type      FrameType `json:"type"` // "connect.ack"
	ClientID  string    `json:"clientId"`
	SessionID string    `json:"sessionId,omitempty"`
	Version   string    `json:"version"`
	Timestamp int64     `json:"ts"`
}

// EventMessageFrame represents an incoming message from a channel
type EventMessageFrame struct {
	Type           FrameType `json:"type"` // "event.message"
	Channel        string    `json:"channel"`
	ConversationID string    `json:"conversationId"`
	MessageID      string    `json:"messageId,omitempty"`
	Text           string    `json:"text"`
	Sender         Sender    `json:"sender"`
	SessionID      string    `json:"sessionId,omitempty"`
	Timestamp      int64     `json:"ts"`
}

// EventToolStartFrame indicates a tool is starting execution
type EventToolStartFrame struct {
	Type           FrameType              `json:"type"` // "event.tool.start"
	SessionID      string                 `json:"sessionId"`
	ToolName       string                 `json:"toolName"`
	ToolID         string                 `json:"toolId"` // Unique ID for this tool execution
	Args           map[string]interface{} `json:"args"`
	Metadata       ToolMetadata           `json:"metadata,omitempty"`
	Timestamp      int64                  `json:"ts"`
}

// EventToolEndFrame indicates a tool finished execution
type EventToolEndFrame struct {
	Type         FrameType   `json:"type"` // "event.tool.end"
	SessionID    string      `json:"sessionId"`
	ToolName     string      `json:"toolName"`
	ToolID       string      `json:"toolId"`
	Result       interface{} `json:"result"`
	Error        string      `json:"error,omitempty"`
	ErrorCode    string      `json:"errorCode,omitempty"`
	ErrorStack   string      `json:"errorStack,omitempty"`
	Duration     int64       `json:"duration"` // milliseconds
	BytesRead    int64       `json:"bytesRead,omitempty"`
	BytesWritten int64       `json:"bytesWritten,omitempty"`
	ExitCode     int         `json:"exitCode,omitempty"`
	Metadata     ToolMetadata `json:"metadata,omitempty"`
	Timestamp    int64       `json:"ts"`
}

// EventToolLogFrame is emitted during tool execution for progress updates
type EventToolLogFrame struct {
	Type      FrameType `json:"type"` // "event.tool.log"
	SessionID string    `json:"sessionId"`
	ToolID    string    `json:"toolId"`
	Level     string    `json:"level"` // info, warn, error
	Message   string    `json:"message"`
	Timestamp int64     `json:"ts"`
}

// EventToolProgressFrame reports progress during long-running tool execution
type EventToolProgressFrame struct {
	Type       FrameType `json:"type"` // "event.tool.progress"
	SessionID  string    `json:"sessionId"`
	ToolID     string    `json:"toolId"`
	ToolName   string    `json:"toolName"`
	Progress   float64   `json:"progress"`   // 0.0 to 1.0 (percentage)
	Status     string    `json:"status"`     // human-readable status
	Current    int64     `json:"current,omitempty"`    // current progress value
	Total      int64     `json:"total,omitempty"`      // total expected value
	Unit       string    `json:"unit,omitempty"`       // bytes, files, lines, etc.
	Timestamp  int64     `json:"ts"`
}

// EventToolOutputFrame streams tool output in chunks
type EventToolOutputFrame struct {
	Type      FrameType `json:"type"` // "event.tool.output"
	SessionID string    `json:"sessionId"`
	ToolID    string    `json:"toolId"`
	ToolName  string    `json:"toolName"`
	Stream    string    `json:"stream"`  // stdout, stderr, or custom stream name
	Data      string    `json:"data"`    // output chunk
	LineNum   int       `json:"lineNum,omitempty"` // line number if applicable
	Timestamp int64     `json:"ts"`
}

// ToolMetadata provides context about tool execution
type ToolMetadata struct {
	WorkspacePath string            `json:"workspacePath,omitempty"`
	Plugin        string            `json:"plugin,omitempty"`
	PolicyChecked bool              `json:"policyChecked,omitempty"`
	Cached        bool              `json:"cached,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Custom        map[string]string `json:"custom,omitempty"`
}

// EventErrorFrame represents an error event
type EventErrorFrame struct {
	Type      FrameType `json:"type"` // "event.error"
	SessionID string    `json:"sessionId,omitempty"`
	Error     string    `json:"error"`
	Code      string    `json:"code,omitempty"`
	Timestamp int64     `json:"ts"`
}

// CmdChannelSendFrame is sent by agent to send a message to a channel
type CmdChannelSendFrame struct {
	Type           FrameType `json:"type"` // "cmd.channel.send"
	Channel        string    `json:"channel"`
	ConversationID string    `json:"conversationId"`
	Text           string    `json:"text"`
	SessionID      string    `json:"sessionId,omitempty"`
	Metadata       Metadata  `json:"metadata,omitempty"`
	Timestamp      int64     `json:"ts"`
}

// CmdApproveFrame approves a pending action
type CmdApproveFrame struct {
	Type       FrameType `json:"type"` // "cmd.approve"
	SessionID  string    `json:"sessionId"`
	ApprovalID string    `json:"approvalId"`
	Timestamp  int64     `json:"ts"`
}

// CmdRejectFrame rejects a pending action
type CmdRejectFrame struct {
	Type       FrameType `json:"type"` // "cmd.reject"
	SessionID  string    `json:"sessionId"`
	ApprovalID string    `json:"approvalId"`
	Reason     string    `json:"reason,omitempty"`
	Timestamp  int64     `json:"ts"`
}

// Sender represents the sender of a message
type Sender struct {
	ID       string   `json:"id"`
	Username string   `json:"username,omitempty"`
	Name     string   `json:"name,omitempty"`
	Metadata Metadata `json:"metadata,omitempty"`
}

// Metadata holds arbitrary key-value pairs
type Metadata map[string]interface{}

// ParseFrame parses a JSON frame into the appropriate struct
func ParseFrame(data []byte) (interface{}, error) {
	var base Frame
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to parse frame: %w", err)
	}

	switch base.Type {
	case FrameConnect:
		var frame ConnectFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameConnectAck:
		var frame ConnectAckFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventMessage:
		var frame EventMessageFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventToolStart:
		var frame EventToolStartFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventToolEnd:
		var frame EventToolEndFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventToolLog:
		var frame EventToolLogFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventToolProgress:
		var frame EventToolProgressFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventToolOutput:
		var frame EventToolOutputFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameEventError:
		var frame EventErrorFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameCmdChannelSend:
		var frame CmdChannelSendFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameCmdApprove:
		var frame CmdApproveFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FrameCmdReject:
		var frame CmdRejectFrame
		if err := json.Unmarshal(data, &frame); err != nil {
			return nil, err
		}
		return &frame, nil

	case FramePing, FramePong:
		return &base, nil

	default:
		return nil, fmt.Errorf("unknown frame type: %s", base.Type)
	}
}

// ToJSON converts a frame to JSON bytes
func ToJSON(frame interface{}) ([]byte, error) {
	return json.Marshal(frame)
}

// ValidateFrame validates a frame's required fields
func ValidateFrame(frame interface{}) error {
	switch f := frame.(type) {
	case *ConnectFrame:
		if f.Role == "" {
			return fmt.Errorf("role is required")
		}
		if f.ClientID == "" {
			return fmt.Errorf("clientId is required")
		}

	case *EventMessageFrame:
		if f.Channel == "" {
			return fmt.Errorf("channel is required")
		}
		if f.ConversationID == "" {
			return fmt.Errorf("conversationId is required")
		}
		if f.Text == "" {
			return fmt.Errorf("text is required")
		}

	case *CmdChannelSendFrame:
		if f.Channel == "" {
			return fmt.Errorf("channel is required")
		}
		if f.ConversationID == "" {
			return fmt.Errorf("conversationId is required")
		}
		if f.Text == "" {
			return fmt.Errorf("text is required")
		}
	}

	return nil
}
