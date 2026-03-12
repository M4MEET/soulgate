package messaging

import (
	"context"
	"fmt"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/policy"
)

// MessageBroker handles messaging platform integrations
type MessageBroker struct {
	policyEngine *policy.Engine
	auditLogger  audit.Logger
	channels     map[string]Channel
}

// Channel represents a messaging platform (Telegram, WhatsApp, Discord, etc.)
type Channel interface {
	// GetName returns the channel name (telegram, whatsapp, discord, etc.)
	GetName() string

	// SendMessage sends a message through this channel
	SendMessage(ctx context.Context, recipient string, message string) error

	// ReceiveMessages starts listening for incoming messages
	ReceiveMessages(ctx context.Context, handler MessageHandler) error

	// Stop stops the channel
	Stop() error
}

// MessageHandler is called when a message is received
type MessageHandler func(msg IncomingMessage) error

// IncomingMessage represents a received message
type IncomingMessage struct {
	Channel   string // telegram, whatsapp, etc.
	Sender    string // Username or ID
	ChatID    string // Chat/Group ID
	Text      string
	Timestamp int64
}

// OutgoingMessage represents a message to send
type OutgoingMessage struct {
	Channel   string
	Recipient string
	Text      string
}

// NewMessageBroker creates a new message broker
func NewMessageBroker(policyEngine *policy.Engine, auditLogger audit.Logger) *MessageBroker {
	return &MessageBroker{
		policyEngine: policyEngine,
		auditLogger:  auditLogger,
		channels:     make(map[string]Channel),
	}
}

// RegisterChannel registers a new messaging channel
func (mb *MessageBroker) RegisterChannel(channel Channel) error {
	name := channel.GetName()
	if _, exists := mb.channels[name]; exists {
		return fmt.Errorf("channel %s already registered", name)
	}
	mb.channels[name] = channel
	return nil
}

// SendMessage sends a message through a specific channel
func (mb *MessageBroker) SendMessage(ctx context.Context, msg OutgoingMessage) error {
	// Check policy
	result, err := mb.policyEngine.Evaluate(ctx, policy.PolicyRequest{
		Action:   "messaging.send",
		Resource: fmt.Sprintf("%s:%s", msg.Channel, msg.Recipient),
	})
	if err != nil {
		return fmt.Errorf("policy check failed: %w", err)
	}

	if result.Decision == policy.DecisionDeny {
		// Log denied attempt
		event := audit.NewEvent(audit.EventPolicyDeny, audit.CategoryPolicy).
			WithMetadata("action", "send_message").
			WithMetadata("channel", msg.Channel).
			WithMetadata("recipient", msg.Recipient).
			WithMetadata("decision", "denied").
			WithStatus(audit.StatusDenied)
		mb.auditLogger.Log(ctx, event)
		return fmt.Errorf("policy denied sending message to %s on %s", msg.Recipient, msg.Channel)
	}

	// Get channel
	channel, exists := mb.channels[msg.Channel]
	if !exists {
		return fmt.Errorf("channel %s not registered", msg.Channel)
	}

	// Send message
	err = channel.SendMessage(ctx, msg.Recipient, msg.Text)
	if err != nil {
		// Log failure
		event := audit.NewEvent(audit.EventNetRequest, audit.CategoryBroker).
			WithMetadata("action", "send_message").
			WithMetadata("channel", msg.Channel).
			WithMetadata("recipient", msg.Recipient).
			WithError(err).
			WithStatus(audit.StatusError)
		mb.auditLogger.Log(ctx, event)
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Log success
	event := audit.NewEvent(audit.EventNetRequest, audit.CategoryBroker).
		WithMetadata("action", "send_message").
		WithMetadata("channel", msg.Channel).
		WithMetadata("recipient", msg.Recipient).
		WithMetadata("message_length", len(msg.Text)).
		WithStatus(audit.StatusSuccess)
	mb.auditLogger.Log(ctx, event)

	return nil
}

// StartListening starts listening on all registered channels
func (mb *MessageBroker) StartListening(ctx context.Context, handler MessageHandler) error {
	for name, channel := range mb.channels {
		// Wrap handler with audit logging
		wrappedHandler := func(msg IncomingMessage) error {
			// Log received message
			event := audit.NewEvent(audit.EventNetRequest, audit.CategoryBroker).
				WithMetadata("channel", msg.Channel).
				WithMetadata("sender", msg.Sender).
				WithMetadata("chat_id", msg.ChatID).
				WithMetadata("message_length", len(msg.Text)).
				WithStatus(audit.StatusSuccess)
			mb.auditLogger.Log(ctx, event)

			// Call original handler
			return handler(msg)
		}

		go func(ch Channel, n string) {
			if err := ch.ReceiveMessages(ctx, wrappedHandler); err != nil {
				event := audit.NewEvent(audit.EventNetRequest, audit.CategoryBroker).
					WithMetadata("channel", n).
					WithMetadata("action", "receive_messages").
					WithError(err).
					WithStatus(audit.StatusError)
				mb.auditLogger.Log(ctx, event)
			}
		}(channel, name)
	}

	return nil
}

// Stop stops all channels
func (mb *MessageBroker) Stop() error {
	var lastErr error
	for _, channel := range mb.channels {
		if err := channel.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
