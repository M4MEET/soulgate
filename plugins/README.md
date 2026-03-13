# SoulGate Plugins

This directory contains SoulGate plugins that extend functionality through a plugin architecture.

## Plugin Architecture

SoulGate uses a **plugin-based architecture** where features are extensible rather than built-in. This keeps the core lean and allows users to install only the plugins they need.

### Plugin Types

- **Channel Plugins**: Messaging platform integrations (Telegram, WhatsApp, Discord)
- **Tool Plugins**: AI tools that can be called by the model
- **Broker Plugins**: Custom resource brokers for specialized access patterns

## Available Plugins

### Telegram (`telegram/`)

Telegram Bot integration for SoulGate.

**Features:**
- Send and receive messages via Telegram Bot API
- AI-powered message responses
- Photo and document sending
- Policy-enforced message access

**Installation:**
The Telegram plugin is compiled into SoulGate by default. No installation needed.

**Configuration:**
1. Create a bot with [@BotFather](https://t.me/botfather)
2. Set your bot token:
   ```bash
   export TELEGRAM_BOT_TOKEN="your-bot-token"
   ```

**Usage:**
```bash
# Start listening for messages
soulgate messaging start

# Send a message
soulgate messaging send telegram <chat-id> <message>

# List available channels
soulgate messaging list
```

**Permissions Required:**
- `network.request:api.telegram.org/*` - Telegram API access
- `messaging.send:telegram:*` - Send messages
- `messaging.receive:telegram:*` - Receive messages

## Plugin Development

### Architecture Overview

```
Plugin System (v0.1)
├── Go Plugins (compiled-in)
│   ├── Register via init() functions
│   ├── Factory pattern for channel creation
│   └── Runtime: native Go execution
└── WASM Plugins (v0.2+)
    ├── Sandboxed execution
    ├── Host function bridge
    └── Runtime: wazero WASM engine
```

### Creating a Channel Plugin

1. **Create plugin directory:**
   ```bash
   mkdir -p plugins/yourplugin
   ```

2. **Create manifest.yml:**
   ```yaml
   name: yourplugin
   version: 1.0.0
   description: Your channel description
   type: channel

   provides:
     channel:
       name: yourplugin
       description: Your platform
       capabilities:
         - send_message
         - receive_message

   permissions:
     - network.request:api.yourplatform.com/*
     - messaging.send:yourplugin:*
     - messaging.receive:yourplugin:*

   config:
     required:
       - name: api_key
         description: API key for your platform
         env: YOURPLUGIN_API_KEY
         sensitive: true
   ```

3. **Implement the Channel interface:**
   ```go
   package yourplugin

   import (
       "context"
       "github.com/M4MEET/soulgate/internal/brokers/messaging"
   )

   type YourChannel struct {
       // Your fields
   }

   func (c *YourChannel) GetName() string {
       return "yourplugin"
   }

   func (c *YourChannel) SendMessage(ctx context.Context, recipient string, message string) error {
       // Implementation
   }

   func (c *YourChannel) ReceiveMessages(ctx context.Context, handler messaging.MessageHandler) error {
       // Implementation
   }

   func (c *YourChannel) Stop() error {
       // Cleanup
   }
   ```

4. **Register the plugin:**
   ```go
   package yourplugin

   import (
       "context"
       "github.com/M4MEET/soulgate/internal/brokers/messaging"
       "github.com/M4MEET/soulgate/internal/plugins/loader"
   )

   func init() {
       loader.RegisterChannelPlugin("yourplugin", CreateChannel)
   }

   func CreateChannel(ctx context.Context, config map[string]interface{}) (messaging.Channel, error) {
       // Create and return your channel
   }
   ```

5. **Import in cmd/soulgate/cmd/messaging.go:**
   ```go
   import (
       _ "github.com/M4MEET/soulgate/plugins/yourplugin"
   )
   ```

### Plugin Interface

All plugins implement the base `Plugin` interface:

```go
type Plugin interface {
    Initialize(ctx context.Context, config map[string]interface{}) error
    Shutdown(ctx context.Context) error
}
```

Channel plugins also implement:

```go
type ChannelPlugin interface {
    Plugin
    GetChannel() messaging.Channel
}
```

### Security Model

Plugins are **untrusted** and run with restricted permissions:

- ✅ **Declare permissions**: Plugins must declare what they need in manifest.yml
- ✅ **Policy enforcement**: All operations checked by policy engine
- ✅ **Audit logging**: Every plugin action is logged
- ✅ **Sandbox (WASM)**: WASM plugins run in isolated sandbox (v0.2+)
- ✅ **No direct OS access**: Must use brokers for resources

### Roadmap

**v0.1** (Current):
- ✅ Go plugin architecture with registry
- ✅ Telegram channel plugin
- ✅ Plugin manifest and permissions
- ✅ Policy enforcement for messaging

**v0.2** (Next):
- [ ] WASM plugin loading with wazero
- [ ] Host function bridge (file, network, messaging)
- [ ] WhatsApp channel plugin
- [ ] Discord channel plugin
- [ ] Plugin marketplace discovery

**v0.3**:
- [ ] Tool plugins (AI tools)
- [ ] Broker plugins (custom resource access)
- [ ] Plugin dependency management
- [ ] Hot reload support

## Examples

### Example: Telegram Bot

See `telegram/` for a complete channel plugin implementation.

Key files:
- `manifest.yml` - Plugin metadata and permissions
- `telegram.go` - Telegram Bot API implementation
- `register.go` - Plugin registration

## Contributing

To contribute a new plugin:

1. Create plugin directory with manifest.yml
2. Implement the plugin interface
3. Write tests
4. Document in plugin README
5. Submit PR

Ensure your plugin:
- Follows security best practices
- Declares minimal permissions
- Includes error handling
- Has comprehensive tests
- Documents configuration

## Support

- **Issues**: Report bugs at [github.com/M4MEET/soulgate/issues](https://github.com/M4MEET/soulgate/issues)
- **Discussions**: Ask questions in GitHub Discussions
- **Documentation**: See `/docs` for architecture details
