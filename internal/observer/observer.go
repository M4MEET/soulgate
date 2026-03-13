package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Observer is a CLI observer that connects to the Gateway
type Observer struct {
	gatewayURL string
	clientID   string
	conn       *websocket.Conn
	done       chan struct{}
	formatter  *Formatter
}

// Config holds observer configuration
type Config struct {
	GatewayURL string
	ClientID   string
	Verbose    bool
}

// NewObserver creates a new observer
func NewObserver(config *Config) *Observer {
	if config.ClientID == "" {
		config.ClientID = fmt.Sprintf("observer-%s", uuid.New().String()[:8])
	}

	return &Observer{
		gatewayURL: config.GatewayURL,
		clientID:   config.ClientID,
		done:       make(chan struct{}),
		formatter:  NewFormatter(config.Verbose),
	}
}

// Connect connects to the Gateway
func (o *Observer) Connect(ctx context.Context) error {
	u, err := url.Parse(o.gatewayURL)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}

	fmt.Printf("🔌 Connecting to Gateway at %s...\n", o.gatewayURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	o.conn = conn

	// Send connect frame
	connectFrame := &protocol.ConnectFrame{
		Type:     protocol.FrameConnect,
		Role:     protocol.RoleUI,
		ClientID: o.clientID,
		Version:  protocol.ProtocolVersion,
		Metadata: protocol.Metadata{
			"type": "cli_observer",
		},
		Timestamp: time.Now().Unix(),
	}

	if err := o.sendFrame(connectFrame); err != nil {
		return fmt.Errorf("failed to send connect frame: %w", err)
	}

	// Wait for connect.ack
	_, message, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read connect.ack: %w", err)
	}

	frame, err := protocol.ParseFrame(message)
	if err != nil {
		return fmt.Errorf("failed to parse connect.ack: %w", err)
	}

	ackFrame, ok := frame.(*protocol.ConnectAckFrame)
	if !ok {
		return fmt.Errorf("expected connect.ack, got %T", frame)
	}

	fmt.Printf("✓ Connected as %s (session: %s)\n", ackFrame.ClientID, ackFrame.SessionID)
	fmt.Println("👀 Observing events... (Ctrl+C to stop)")

	return nil
}

// Start starts observing events
func (o *Observer) Start(ctx context.Context) error {
	if o.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Start read loop
	go o.readLoop(ctx)

	// Wait for context cancellation or done signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-o.done:
		return nil
	}
}

// readLoop reads and processes frames from the Gateway
func (o *Observer) readLoop(ctx context.Context) {
	defer close(o.done)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := o.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("\n❌ Connection error: %v\n", err)
			}
			return
		}

		// Parse frame
		frame, err := protocol.ParseFrame(message)
		if err != nil {
			fmt.Printf("❌ Failed to parse frame: %v\n", err)
			continue
		}

		// Handle frame
		o.handleFrame(frame)
	}
}

// handleFrame handles different frame types
func (o *Observer) handleFrame(frame interface{}) {
	switch f := frame.(type) {
	case *protocol.EventMessageFrame:
		o.formatter.FormatEventMessage(f)

	case *protocol.EventToolStartFrame:
		o.formatter.FormatToolStart(f)

	case *protocol.EventToolEndFrame:
		o.formatter.FormatToolEnd(f)

	case *protocol.EventToolLogFrame:
		o.formatter.FormatToolLog(f)

	case *protocol.EventToolProgressFrame:
		o.formatter.FormatToolProgress(f)

	case *protocol.EventToolOutputFrame:
		o.formatter.FormatToolOutput(f)

	case *protocol.CmdChannelSendFrame:
		o.formatter.FormatChannelSend(f)

	case *protocol.EventErrorFrame:
		o.formatter.FormatError(f)

	case *protocol.Frame:
		// Ping/Pong or other base frames
		if f.Type == protocol.FramePing || f.Type == protocol.FramePong {
			// Silently handle heartbeats
			return
		}
		o.formatter.FormatGeneric(f)

	default:
		fmt.Printf("⚠️  Unknown frame type: %T\n", frame)
	}
}

// sendFrame sends a frame to the Gateway
func (o *Observer) sendFrame(frame interface{}) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	return o.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the observer connection
func (o *Observer) Close() error {
	if o.conn != nil {
		// Send close message
		err := o.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			return err
		}

		return o.conn.Close()
	}
	return nil
}
