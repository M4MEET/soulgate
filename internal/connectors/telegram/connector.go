package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Connector is a Telegram bot that connects to the Gateway
type Connector struct {
	config      *Config
	gatewayURL  string
	clientID    string
	conn        *websocket.Conn
	done        chan struct{}
	bot         *bot.Bot
	botCtx      context.Context
	botCancel   context.CancelFunc
}

// Config holds Telegram connector configuration
type Config struct {
	GatewayURL string
	BotToken   string
	ClientID   string
}

// NewConnector creates a new Telegram connector
func NewConnector(cfg *Config) (*Connector, error) {
	if cfg.BotToken == "" {
		cfg.BotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("Telegram bot token not configured (set TELEGRAM_BOT_TOKEN)")
	}

	if cfg.ClientID == "" {
		cfg.ClientID = fmt.Sprintf("telegram-%s", uuid.New().String()[:8])
	}

	// Create Telegram bot
	botCtx, botCancel := signal.NotifyContext(context.Background(), os.Interrupt)

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Messages will be handled by our custom handler
		}),
	}

	b, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		botCancel()
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	return &Connector{
		config:     cfg,
		gatewayURL: cfg.GatewayURL,
		clientID:   cfg.ClientID,
		done:       make(chan struct{}),
		bot:        b,
		botCtx:     botCtx,
		botCancel:  botCancel,
	}, nil
}

// Connect connects to the Gateway
func (c *Connector) Connect(ctx context.Context) error {
	u, err := url.Parse(c.gatewayURL)
	if err != nil {
		return fmt.Errorf("invalid gateway URL: %w", err)
	}

	fmt.Printf("🔌 Connecting to Gateway at %s...\n", c.gatewayURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn

	// Get bot info
	botInfo, err := c.bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	// Send connect frame
	connectFrame := &protocol.ConnectFrame{
		Type:     protocol.FrameConnect,
		Role:     protocol.RoleChannel,
		ClientID: c.clientID,
		Version:  protocol.ProtocolVersion,
		Metadata: protocol.Metadata{
			"channel":     "telegram",
			"bot_id":      botInfo.ID,
			"bot_username": botInfo.Username,
		},
		Timestamp: time.Now().Unix(),
	}

	if err := c.sendFrame(connectFrame); err != nil {
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

	fmt.Printf("✓ Connected as %s\n", ackFrame.ClientID)
	fmt.Printf("✓ Telegram Bot: @%s (ID: %d)\n", botInfo.Username, botInfo.ID)
	fmt.Println("📱 Listening for Telegram messages...")

	return nil
}

// Start starts the Telegram connector
func (c *Connector) Start(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	// Register Telegram message handler
	c.bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, c.handleTelegramMessage)

	// Start Telegram bot in background
	go func() {
		c.bot.Start(c.botCtx)
	}()

	// Start Gateway read loop
	go c.readLoop(ctx)

	// Wait for context cancellation or done signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		return nil
	}
}

// handleTelegramMessage handles incoming Telegram messages
func (c *Connector) handleTelegramMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	username := update.Message.From.Username
	if username == "" {
		username = fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName)
	}

	fmt.Printf("📨 Received message from @%s: %s\n", username, update.Message.Text)

	// Create event.message frame
	frame := &protocol.EventMessageFrame{
		Type:           protocol.FrameEventMessage,
		Channel:        "telegram",
		ConversationID: fmt.Sprintf("%d", update.Message.Chat.ID),
		MessageID:      fmt.Sprintf("%d", update.Message.ID),
		Text:           update.Message.Text,
		Sender: protocol.Sender{
			ID:       fmt.Sprintf("%d", update.Message.From.ID),
			Username: username,
			Name:     update.Message.From.FirstName,
		},
		Timestamp: int64(update.Message.Date),
	}

	// Send to Gateway
	if err := c.sendFrame(frame); err != nil {
		fmt.Printf("❌ Failed to send message to Gateway: %v\n", err)
	}
}

// readLoop reads and processes frames from the Gateway
func (c *Connector) readLoop(ctx context.Context) {
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
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
		if err := c.handleFrame(ctx, frame); err != nil {
			fmt.Printf("❌ Failed to handle frame: %v\n", err)
		}
	}
}

// handleFrame handles different frame types
func (c *Connector) handleFrame(ctx context.Context, frame interface{}) error {
	switch f := frame.(type) {
	case *protocol.CmdChannelSendFrame:
		return c.handleChannelSend(ctx, f)

	case *protocol.Frame:
		// Handle ping/pong
		if f.Type == protocol.FramePing {
			pongFrame := &protocol.Frame{
				Type:      protocol.FramePong,
				Timestamp: time.Now().Unix(),
			}
			return c.sendFrame(pongFrame)
		}
		return nil

	default:
		// Ignore other frame types
		return nil
	}
}

// handleChannelSend handles send commands from Gateway
func (c *Connector) handleChannelSend(ctx context.Context, frame *protocol.CmdChannelSendFrame) error {
	if frame.Channel != "telegram" {
		return nil // Not for us
	}

	fmt.Printf("🤖 Sending response to chat %s: %s\n", frame.ConversationID, frame.Text)

	// Send message via Telegram API
	_, err := c.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: frame.ConversationID,
		Text:   frame.Text,
	})

	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}

	fmt.Printf("✅ Message sent successfully\n\n")
	return nil
}

// sendFrame sends a frame to the Gateway
func (c *Connector) sendFrame(frame interface{}) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("failed to marshal frame: %w", err)
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Close closes the connector
func (c *Connector) Close() error {
	// Stop Telegram bot
	c.botCancel()

	// Close Gateway connection
	if c.conn != nil {
		err := c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			return err
		}
		return c.conn.Close()
	}
	return nil
}
