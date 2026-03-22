package tui

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	tea "github.com/charmbracelet/bubbletea"
)

// GatewayClient manages the WebSocket connection to the Gateway
type GatewayClient struct {
	conn     *websocket.Conn
	clientID string
	program  *tea.Program
	done     chan struct{}
	mu       sync.Mutex // protects writes to conn
}

// Gateway-related tea.Msg types

type gatewayMessageMsg struct {
	frame *protocol.EventMessageFrame
}

type gatewayConnectedMsg struct {
	clientID string
}

type gatewayDisconnectedMsg struct {
	err error
}

type gatewayResponseMsg struct {
	frame    *protocol.EventMessageFrame
	response string
	err      error
}

// NewGatewayClient connects to the Gateway as an agent role
func NewGatewayClient(gatewayURL string, program *tea.Program) (*GatewayClient, error) {
	clientID := fmt.Sprintf("tui-agent-%s", uuid.New().String()[:8])

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(gatewayURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway: %w", err)
	}

	// Send connect frame as agent
	connectFrame := &protocol.ConnectFrame{
		Type:     protocol.FrameConnect,
		Role:     protocol.RoleAgent,
		ClientID: clientID,
		Version:  protocol.ProtocolVersion,
		Metadata: protocol.Metadata{
			"type": "tui-agent",
		},
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(connectFrame)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to marshal connect frame: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send connect frame: %w", err)
	}

	// Wait for connect.ack
	_, message, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read connect.ack: %w", err)
	}

	frame, err := protocol.ParseFrame(message)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to parse connect.ack: %w", err)
	}

	if _, ok := frame.(*protocol.ConnectAckFrame); !ok {
		conn.Close()
		return nil, fmt.Errorf("expected connect.ack, got %T", frame)
	}

	return &GatewayClient{
		conn:     conn,
		clientID: clientID,
		program:  program,
		done:     make(chan struct{}),
	}, nil
}

// Start begins the read loop (run as goroutine)
func (gw *GatewayClient) Start() {
	defer close(gw.done)

	// Notify TUI of connection
	gw.program.Send(gatewayConnectedMsg{clientID: gw.clientID})

	for {
		_, message, err := gw.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				gw.program.Send(gatewayDisconnectedMsg{err: err})
			} else {
				gw.program.Send(gatewayDisconnectedMsg{})
			}
			return
		}

		frame, err := protocol.ParseFrame(message)
		if err != nil {
			continue
		}

		switch f := frame.(type) {
		case *protocol.EventMessageFrame:
			gw.program.Send(gatewayMessageMsg{frame: f})

		case *protocol.Frame:
			if f.Type == protocol.FramePing {
				gw.SendFrame(&protocol.Frame{
					Type:      protocol.FramePong,
					Timestamp: time.Now().Unix(),
				})
			}
		}
	}
}

// SendFrame sends a frame to the Gateway (thread-safe)
func (gw *GatewayClient) SendFrame(frame interface{}) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	return gw.conn.WriteMessage(websocket.TextMessage, data)
}

// SendChannelResponse sends a response back to the originating channel
func (gw *GatewayClient) SendChannelResponse(channel, conversationID, sessionID, text string) error {
	return gw.SendFrame(&protocol.CmdChannelSendFrame{
		Type:           protocol.FrameCmdChannelSend,
		Channel:        channel,
		ConversationID: conversationID,
		Text:           text,
		SessionID:      sessionID,
		Timestamp:      time.Now().Unix(),
	})
}

// SendToolStart broadcasts a tool start event
func (gw *GatewayClient) SendToolStart(sessionID, toolName, toolID string) error {
	return gw.SendFrame(&protocol.EventToolStartFrame{
		Type:      protocol.FrameEventToolStart,
		SessionID: sessionID,
		ToolName:  toolName,
		ToolID:    toolID,
		Timestamp: time.Now().Unix(),
	})
}

// SendToolEnd broadcasts a tool end event
func (gw *GatewayClient) SendToolEnd(sessionID, toolName, toolID string, result interface{}, toolErr error, durationMs int64) error {
	frame := &protocol.EventToolEndFrame{
		Type:      protocol.FrameEventToolEnd,
		SessionID: sessionID,
		ToolName:  toolName,
		ToolID:    toolID,
		Result:    result,
		Duration:  durationMs,
		Timestamp: time.Now().Unix(),
	}
	if toolErr != nil {
		frame.Error = toolErr.Error()
	}
	return gw.SendFrame(frame)
}

// Close closes the gateway connection
func (gw *GatewayClient) Close() error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	_ = gw.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	return gw.conn.Close()
}
