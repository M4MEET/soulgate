# Telegram Plugin

Telegram Bot integration for SoulGate - connect your AI agent to Telegram.

## Features

- ✅ **Send Messages**: Send text messages via Telegram Bot API
- ✅ **Receive Messages**: Listen for incoming messages and respond with AI
- ✅ **Send Photos**: Share images with captions
- ✅ **Send Documents**: Share files with captions
- ✅ **Policy Enforcement**: All messages checked against security policies
- ✅ **Audit Logging**: Every message send/receive is logged
- ✅ **Bot Info**: Query bot information

## Installation

The Telegram plugin is **compiled into SoulGate by default**. No separate installation needed.

## Setup

### 1. Create a Telegram Bot

1. Open Telegram and search for [@BotFather](https://t.me/botfather)
2. Send `/newbot` command
3. Follow the prompts to choose a name and username
4. **Save your bot token** (looks like `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`)

### 2. Configure Environment

Set your bot token as an environment variable:

```bash
export TELEGRAM_BOT_TOKEN="your-bot-token-here"
```

Or add to your shell profile (`~/.bashrc`, `~/.zshrc`):

```bash
echo 'export TELEGRAM_BOT_TOKEN="your-bot-token"' >> ~/.bashrc
source ~/.bashrc
```

### 3. Verify Installation

```bash
# List available messaging channels
soulgate messaging list

# Should show:
# Available messaging channels (1):
#   • telegram
```

## Usage

### Start the Bot

Start listening for messages and respond with AI:

```bash
soulgate messaging start
```

Output:
```
✓ Registered telegram channel
✓ Listening for messages on 1 channel(s)...

📨 [telegram] Message from @username: Hello!
🤖 Responding: Hi! How can I help you today?
```

The bot will:
1. Listen for incoming messages
2. Process each message with your AI model
3. Send the AI response back to the user

### Send Messages

Send a one-off message:

```bash
soulgate messaging send telegram <chat-id> "Hello from SoulGate!"
```

**Finding Chat IDs:**
- Personal chats: User's numeric ID (shown when they message your bot)
- Groups: Group ID (starts with `-`, shown when bot is added to group)

## Configuration

### Policy Rules

Add messaging permissions to `.soulgate/policy.yml`:

```yaml
policies:
  # Allow sending messages via Telegram
  - name: "allow-telegram-send"
    action: "messaging.send"
    resource: "telegram:*"
    decision: allow
    priority: 10

  # Allow receiving messages (optional, for audit)
  - name: "allow-telegram-receive"
    action: "messaging.receive"
    resource: "telegram:*"
    decision: allow
    priority: 10

  # Restrict to specific chat IDs (optional)
  - name: "allow-specific-chat"
    action: "messaging.send"
    resource: "telegram:123456789"
    decision: allow
    priority: 20

  - name: "deny-other-chats"
    action: "messaging.send"
    resource: "telegram:*"
    decision: deny
    priority: 15
```

### Advanced Configuration

The plugin reads configuration from:

1. **Environment variables**: `TELEGRAM_BOT_TOKEN`
2. **Manifest config**: `manifest.yml` (for plugin settings)
3. **Workspace config**: `.soulgate/config.yml` (future)

## Architecture

### Plugin Structure

```
plugins/telegram/
├── manifest.yml     # Plugin metadata and permissions
├── telegram.go      # Telegram Bot API client
├── register.go      # Plugin registration
└── README.md        # This file
```

### Channel Interface

The plugin implements the `messaging.Channel` interface:

```go
type Channel interface {
    GetName() string
    SendMessage(ctx context.Context, recipient string, message string) error
    ReceiveMessages(ctx context.Context, handler MessageHandler) error
    Stop() error
}
```

### Data Flow

```
Telegram Message → Bot API → TelegramChannel.ReceiveMessages()
                                      ↓
                           MessageHandler (with policy check)
                                      ↓
                           Orchestrator.Run(prompt) [AI processing]
                                      ↓
                           MessageBroker.SendMessage() [policy + audit]
                                      ↓
                           TelegramChannel.SendMessage()
                                      ↓
                           Bot API → Telegram Message
```

## Permissions

The plugin requires these permissions (declared in `manifest.yml`):

- `network.request:api.telegram.org/*` - Access Telegram Bot API
- `messaging.send:telegram:*` - Send messages via Telegram
- `messaging.receive:telegram:*` - Receive messages from Telegram

All permissions are enforced by the policy engine.

## API Reference

### TelegramChannel Methods

```go
// Send a text message
func (tc *TelegramChannel) SendMessage(ctx context.Context, recipient string, message string) error

// Start listening for incoming messages
func (tc *TelegramChannel) ReceiveMessages(ctx context.Context, handler MessageHandler) error

// Send a photo with optional caption
func (tc *TelegramChannel) SendPhoto(ctx context.Context, chatID string, photoURL string, caption string) error

// Send a document with optional caption
func (tc *TelegramChannel) SendDocument(ctx context.Context, chatID string, documentURL string, caption string) error

// Get bot information
func (tc *TelegramChannel) GetMe(ctx context.Context) (*models.User, error)

// Stop the bot
func (tc *TelegramChannel) Stop() error
```

## Examples

### Basic Bot

```bash
# Start bot and let AI respond to messages
export TELEGRAM_BOT_TOKEN="your-token"
soulgate messaging start
```

### Send Notification

```bash
# Send a one-time message
soulgate messaging send telegram 123456789 "Deployment completed successfully!"
```

### Custom AI Prompt

The bot sends all incoming messages to the AI orchestrator. Configure your AI model in `.soulgate/config.yml`:

```yaml
model:
  default_provider: openai
  openai:
    api_key: sk-...
    model: gpt-4
    temperature: 0.7
    max_tokens: 500
```

## Troubleshooting

### Bot Not Responding

1. **Check bot token**:
   ```bash
   echo $TELEGRAM_BOT_TOKEN
   ```

2. **Verify bot is running**:
   ```bash
   soulgate messaging list
   ```

3. **Check logs** for errors in terminal output

### Policy Denials

If messages are being denied:

1. Check `.soulgate/policy.yml` has messaging permissions
2. View audit logs:
   ```bash
   soulgate audit tail --category policy
   ```

### Chat ID Not Working

- For personal chats: Have the user message your bot first, then check logs for their ID
- For groups: Add bot to group as admin, then check logs for group ID
- Group IDs start with `-` (e.g., `-1001234567890`)

## Dependencies

- `github.com/go-telegram/bot` v1.18.0 - Telegram Bot API client
- Go 1.21+ required

## Development

### Running Tests

```bash
go test ./plugins/telegram/...
```

### Adding Features

To add new Telegram API features:

1. Add method to `TelegramChannel` struct
2. Use `tc.bot` (go-telegram/bot client) to call API
3. Update README with usage example

### Security Considerations

- ✅ Bot token stored in environment variable (not in code)
- ✅ All messages pass through policy engine
- ✅ Audit logging for all sends/receives
- ✅ No direct file system access from plugin
- ✅ Network requests scoped to Telegram API only

## Roadmap

- [ ] Inline keyboard support
- [ ] Callback query handling
- [ ] Media group sending
- [ ] Bot commands (/start, /help)
- [ ] Webhook mode (alternative to polling)
- [ ] Rate limiting
- [ ] Message threading support

## Support

- Report issues: [github.com/M4MEET/soulgate/issues](https://github.com/M4MEET/soulgate/issues)
- Telegram API docs: [core.telegram.org/bots/api](https://core.telegram.org/bots/api)
