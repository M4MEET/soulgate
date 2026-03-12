package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client
type Client struct {
	id       string
	role     protocol.ClientRole
	conn     *websocket.Conn
	send     chan []byte
	gateway  *Gateway
	metadata protocol.Metadata

	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new client
func NewClient(id string, role protocol.ClientRole, conn *websocket.Conn, gateway *Gateway) *Client {
	return &Client{
		id:       id,
		role:     role,
		conn:     conn,
		send:     make(chan []byte, 256),
		gateway:  gateway,
		metadata: make(protocol.Metadata),
	}
}

// ID returns the client ID
func (c *Client) ID() string {
	return c.id
}

// Role returns the client role
func (c *Client) Role() protocol.ClientRole {
	return c.role
}

// Start starts the client's read and write pumps
func (c *Client) Start(ctx context.Context) {
	go c.writePump(ctx)
	go c.readPump(ctx)
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.gateway.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error: %v\n", err)
			}
			break
		}

		// Parse frame
		frame, err := protocol.ParseFrame(message)
		if err != nil {
			c.sendError(fmt.Sprintf("failed to parse frame: %v", err))
			continue
		}

		// Validate frame
		if err := protocol.ValidateFrame(frame); err != nil {
			c.sendError(fmt.Sprintf("invalid frame: %v", err))
			continue
		}

		// Route frame
		if err := c.gateway.RouteFrame(ctx, c, frame); err != nil {
			c.sendError(fmt.Sprintf("failed to route frame: %v", err))
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Send sends a frame to the client
func (c *Client) Send(frame interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client closed")
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// sendError sends an error frame to the client
func (c *Client) sendError(message string) {
	frame := &protocol.EventErrorFrame{
		Type:      protocol.FrameEventError,
		Error:     message,
		Timestamp: time.Now().Unix(),
	}
	c.Send(frame)
}

// Close closes the client connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.send)
	return c.conn.Close()
}
