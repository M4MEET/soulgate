package telegram

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/M4MEET/soulgate/internal/brokers/messaging"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// TelegramChannel implements the Channel interface for Telegram
type TelegramChannel struct {
	bot      *bot.Bot
	token    string
	ctx      context.Context
	cancel   context.CancelFunc
	handlers []messaging.MessageHandler
}

// Config holds Telegram configuration
type Config struct {
	BotToken string
}

// NewTelegramChannel creates a new Telegram channel
func NewTelegramChannel(cfg Config) (*TelegramChannel, error) {
	if cfg.BotToken == "" {
		// Try to get from environment
		cfg.BotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	}

	if cfg.BotToken == "" {
		return nil, fmt.Errorf("Telegram bot token not configured (set TELEGRAM_BOT_TOKEN)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)

	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Will be handled by ReceiveMessages
		}),
	}

	b, err := bot.New(cfg.BotToken, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	return &TelegramChannel{
		bot:      b,
		token:    cfg.BotToken,
		ctx:      ctx,
		cancel:   cancel,
		handlers: make([]messaging.MessageHandler, 0),
	}, nil
}

// GetName returns the channel name
func (tc *TelegramChannel) GetName() string {
	return "telegram"
}

// SendMessage sends a message to a Telegram chat
func (tc *TelegramChannel) SendMessage(ctx context.Context, recipient string, message string) error {
	_, err := tc.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: recipient,
		Text:   message,
	})
	if err != nil {
		return fmt.Errorf("failed to send Telegram message: %w", err)
	}
	return nil
}

// ReceiveMessages starts listening for incoming messages
func (tc *TelegramChannel) ReceiveMessages(ctx context.Context, handler messaging.MessageHandler) error {
	// Register message handler
	tc.bot.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		msg := messaging.IncomingMessage{
			Channel:   "telegram",
			Sender:    update.Message.From.Username,
			ChatID:    fmt.Sprintf("%d", update.Message.Chat.ID),
			Text:      update.Message.Text,
			Timestamp: int64(update.Message.Date),
		}

		// Call handler
		if err := handler(msg); err != nil {
			// Log error but continue
			fmt.Printf("Error handling message: %v\n", err)
		}
	})

	// Start bot (blocking)
	tc.bot.Start(ctx)
	return nil
}

// Stop stops the Telegram bot
func (tc *TelegramChannel) Stop() error {
	tc.cancel()
	return nil
}

// SendPhoto sends a photo to a Telegram chat
func (tc *TelegramChannel) SendPhoto(ctx context.Context, chatID string, photoURL string, caption string) error {
	_, err := tc.bot.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:  chatID,
		Photo:   &models.InputFileString{Data: photoURL},
		Caption: caption,
	})
	return err
}

// SendDocument sends a document to a Telegram chat
func (tc *TelegramChannel) SendDocument(ctx context.Context, chatID string, documentURL string, caption string) error {
	_, err := tc.bot.SendDocument(ctx, &bot.SendDocumentParams{
		ChatID:   chatID,
		Document: &models.InputFileString{Data: documentURL},
		Caption:  caption,
	})
	return err
}

// GetMe returns information about the bot
func (tc *TelegramChannel) GetMe(ctx context.Context) (*models.User, error) {
	return tc.bot.GetMe(ctx)
}
