package telegram

import (
	"context"
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/brokers/messaging"
	"github.com/M4MEET/soulgate/internal/plugins/loader"
)

func init() {
	// Register Telegram channel plugin on import
	loader.RegisterChannelPlugin("telegram", CreateTelegramChannel)
}

// CreateTelegramChannel is the factory function for creating Telegram channels
func CreateTelegramChannel(ctx context.Context, config map[string]interface{}) (messaging.Channel, error) {
	// Get bot token from config or environment
	botToken := ""
	if token, ok := config["bot_token"].(string); ok {
		botToken = token
	}
	if botToken == "" {
		botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if botToken == "" {
		return nil, fmt.Errorf("bot_token not configured (set TELEGRAM_BOT_TOKEN)")
	}

	// Create Telegram channel
	return NewTelegramChannel(Config{
		BotToken: botToken,
	})
}
