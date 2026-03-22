package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/M4MEET/soulgate/internal/connectors/discord"
	feishuconnector "github.com/M4MEET/soulgate/internal/connectors/feishu"
	imessageconnector "github.com/M4MEET/soulgate/internal/connectors/imessage"
	ircconnector "github.com/M4MEET/soulgate/internal/connectors/irc"
	mattermostconnector "github.com/M4MEET/soulgate/internal/connectors/mattermost"
	matrixconnector "github.com/M4MEET/soulgate/internal/connectors/matrix"
	nostrconnector "github.com/M4MEET/soulgate/internal/connectors/nostr"
	signalconnector "github.com/M4MEET/soulgate/internal/connectors/signal"
	slackconnector "github.com/M4MEET/soulgate/internal/connectors/slack"
	teamsconnector "github.com/M4MEET/soulgate/internal/connectors/teams"
	"github.com/M4MEET/soulgate/internal/connectors/telegram"
	twitchconnector "github.com/M4MEET/soulgate/internal/connectors/twitch"
	"github.com/M4MEET/soulgate/internal/connectors/whatsapp"
	"github.com/spf13/cobra"
)

var connectorCmd = &cobra.Command{
	Use:   "connector",
	Short: "Connector commands",
	Long: `Run connectors that bridge messaging platforms to the Gateway.

Connectors:
- Telegram: Connect Telegram bot to Gateway via WebSocket protocol
- Discord:  Connect Discord bot to Gateway via HTTP /api/chat endpoint
- Slack:    Connect Slack bot to Gateway via Socket Mode + HTTP /api/chat endpoint
- WhatsApp: Connect WhatsApp (multi-device) to Gateway via HTTP /api/chat endpoint
- Signal:   Connect Signal messenger to Gateway via signal-cli JSON-RPC
- Teams:    Connect Microsoft Teams bot to Gateway via Bot Framework v4 webhook
- Matrix:   Connect Matrix bot to Gateway via Matrix Client-Server API long-poll
- iMessage: Connect Apple iMessage to Gateway by polling chat.db (macOS only)
- IRC:         Connect an IRC bot to any IRC network via HTTP /api/chat endpoint
- Twitch:      Connect a Twitch chat bot to one or more channels via HTTP /api/chat endpoint
- Nostr:       Connect to Nostr relays and receive mentions (read-only, v1)
- Mattermost:  Connect a Mattermost bot to the Gateway via WebSocket + REST API
- Feishu:      Connect a Feishu (Lark) bot to the Gateway via event subscription webhook`,
}

var connectorTelegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Start Telegram connector",
	Long: `Start the Telegram connector that bridges Telegram to the Gateway.

The connector:
1. Connects to Gateway as a 'channel' role client
2. Listens for incoming Telegram messages
3. Forwards messages to Gateway as event.message frames
4. Receives cmd.channel.send frames from Gateway
5. Sends responses back to Telegram users

Requirements:
- TELEGRAM_BOT_TOKEN environment variable
- Running Gateway instance

Example:
  export TELEGRAM_BOT_TOKEN="your-bot-token"
  soulgate connector telegram --gateway ws://localhost:8080/ws`,
	RunE: runConnectorTelegram,
}

var (
	connectorGatewayURL string
	connectorClientID   string

	// Discord-specific flags.
	discordGatewayURL string
	discordGuildID    string

	// Slack-specific flags.
	slackGatewayURL string

	// WhatsApp-specific flags.
	whatsappGatewayURL string
	whatsappDataDir    string

	// Signal-specific flags.
	signalGatewayURL  string
	signalPhoneNumber string
	signalCLIBinary   string

	// Teams-specific flags.
	teamsGatewayURL   string
	teamsAppID        string
	teamsAppPassword  string
	teamsListenAddr   string

	// Matrix-specific flags.
	matrixGatewayURL     string
	matrixHomeserverURL  string
	matrixAccessToken    string
	matrixUserID         string
	matrixSinceTokenPath string

	// iMessage-specific flags.
	imessageGatewayURL   string
	imessagePollInterval time.Duration
	imessageChatDBPath   string

	// IRC-specific flags.
	ircGatewayURL string
	ircServer     string
	ircNick       string
	ircChannels   []string
	ircUseTLS     bool
	ircPassword   string

	// Twitch-specific flags.
	twitchGatewayURL string
	twitchOAuthToken string
	twitchNick       string
	twitchChannels   []string

	// Nostr-specific flags.
	nostrGatewayURL string
	nostrPrivateKey string
	nostrRelays     []string

	// Mattermost-specific flags.
	mattermostGatewayURL  string
	mattermostServerURL   string
	mattermostToken       string
	mattermostBotUsername string

	// Feishu-specific flags.
	feishuGatewayURL   string
	feishuAppID        string
	feishuAppSecret    string
	feishuListenAddr   string
	feishuVerifyToken  string
)

func init() {
	rootCmd.AddCommand(connectorCmd)
	connectorCmd.AddCommand(connectorTelegramCmd)
	connectorCmd.AddCommand(connectorDiscordCmd)
	connectorCmd.AddCommand(connectorSlackCmd)
	connectorCmd.AddCommand(connectorWhatsAppCmd)
	connectorCmd.AddCommand(connectorSignalCmd)
	connectorCmd.AddCommand(connectorTeamsCmd)
	connectorCmd.AddCommand(connectorMatrixCmd)
	connectorCmd.AddCommand(connectorIMessageCmd)
	connectorCmd.AddCommand(connectorIRCCmd)
	connectorCmd.AddCommand(connectorTwitchCmd)
	connectorCmd.AddCommand(connectorNostrCmd)
	connectorCmd.AddCommand(connectorMattermostCmd)
	connectorCmd.AddCommand(connectorFeishuCmd)

	connectorTelegramCmd.Flags().StringVar(&connectorGatewayURL, "gateway", "ws://localhost:8080/ws", "Gateway WebSocket URL")
	connectorTelegramCmd.Flags().StringVar(&connectorClientID, "client-id", "", "Custom client ID (auto-generated if not provided)")

	connectorDiscordCmd.Flags().StringVar(&discordGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorDiscordCmd.Flags().StringVar(&discordGuildID, "guild-id", "", "Restrict bot to a specific Discord guild ID (optional)")

	connectorSlackCmd.Flags().StringVar(&slackGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")

	connectorWhatsAppCmd.Flags().StringVar(&whatsappGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorWhatsAppCmd.Flags().StringVar(&whatsappDataDir, "data-dir", ".soulgate/whatsapp", "Directory for WhatsApp session storage")

	connectorSignalCmd.Flags().StringVar(&signalGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorSignalCmd.Flags().StringVar(&signalPhoneNumber, "phone", "", "Signal phone number in E.164 format, e.g. +15551234567 (required)")
	connectorSignalCmd.Flags().StringVar(&signalCLIBinary, "signal-cli", "signal-cli", "Path to the signal-cli binary")

	connectorTeamsCmd.Flags().StringVar(&teamsGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorTeamsCmd.Flags().StringVar(&teamsAppID, "app-id", "", "Microsoft Teams Bot App ID (MicrosoftAppId) (required)")
	connectorTeamsCmd.Flags().StringVar(&teamsAppPassword, "app-password", "", "Microsoft Teams Bot App Password (required)")
	connectorTeamsCmd.Flags().StringVar(&teamsListenAddr, "listen", ":3978", "Address to listen for incoming Teams webhook events")

	connectorMatrixCmd.Flags().StringVar(&matrixGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorMatrixCmd.Flags().StringVar(&matrixHomeserverURL, "homeserver", "", "Matrix homeserver URL, e.g. https://matrix.org (required)")
	connectorMatrixCmd.Flags().StringVar(&matrixAccessToken, "access-token", "", "Matrix bot access token (required)")
	connectorMatrixCmd.Flags().StringVar(&matrixUserID, "user-id", "", "Matrix bot user ID, e.g. @mybot:matrix.org (required)")
	connectorMatrixCmd.Flags().StringVar(&matrixSinceTokenPath, "since-token-path", ".matrix_since_token", "File path to persist the /sync since token")

	connectorIMessageCmd.Flags().StringVar(&imessageGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorIMessageCmd.Flags().DurationVar(&imessagePollInterval, "poll-interval", 2*time.Second, "How often to poll chat.db for new messages")
	connectorIMessageCmd.Flags().StringVar(&imessageChatDBPath, "chat-db", "~/Library/Messages/chat.db", "Path to the iMessage SQLite database")

	connectorIRCCmd.Flags().StringVar(&ircGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorIRCCmd.Flags().StringVar(&ircServer, "server", "irc.libera.chat:6697", "IRC server address in host:port form")
	connectorIRCCmd.Flags().StringVar(&ircNick, "nick", "", "Bot nickname (required)")
	connectorIRCCmd.Flags().StringArrayVar(&ircChannels, "channel", nil, "Channel(s) to join, e.g. --channel '#soulgate' (repeatable)")
	connectorIRCCmd.Flags().BoolVar(&ircUseTLS, "tls", true, "Use TLS (default true)")
	connectorIRCCmd.Flags().StringVar(&ircPassword, "password", "", "Optional server password (PASS command)")
	_ = connectorIRCCmd.MarkFlagRequired("nick")

	connectorTwitchCmd.Flags().StringVar(&twitchGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorTwitchCmd.Flags().StringVar(&twitchOAuthToken, "oauth-token", "", "Twitch OAuth token (or set TWITCH_OAUTH_TOKEN)")
	connectorTwitchCmd.Flags().StringVar(&twitchNick, "nick", "", "Twitch bot username in lowercase (required)")
	connectorTwitchCmd.Flags().StringArrayVar(&twitchChannels, "channel", nil, "Channel(s) to join, e.g. --channel '#streamer' (repeatable)")
	_ = connectorTwitchCmd.MarkFlagRequired("nick")

	connectorNostrCmd.Flags().StringVar(&nostrGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorNostrCmd.Flags().StringVar(&nostrPrivateKey, "private-key", "", "Hex-encoded Nostr private key, 64 hex chars (or set NOSTR_PRIVATE_KEY)")
	connectorNostrCmd.Flags().StringArrayVar(&nostrRelays, "relay", nil, "Relay WebSocket URL (repeatable), e.g. --relay wss://relay.damus.io")

	connectorMattermostCmd.Flags().StringVar(&mattermostGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorMattermostCmd.Flags().StringVar(&mattermostServerURL, "server", "", "Mattermost server URL, e.g. https://mattermost.example.com (required)")
	connectorMattermostCmd.Flags().StringVar(&mattermostToken, "token", "", "Personal access token or bot token (or set MATTERMOST_TOKEN)")
	connectorMattermostCmd.Flags().StringVar(&mattermostBotUsername, "bot-username", "", "Bot's Mattermost username for @mention detection (auto-detected if not set)")

	connectorFeishuCmd.Flags().StringVar(&feishuGatewayURL, "gateway", "http://localhost:8080", "SoulGate HTTP API base URL")
	connectorFeishuCmd.Flags().StringVar(&feishuAppID, "app-id", "", "Feishu App ID from the Developer Console (or set FEISHU_APP_ID)")
	connectorFeishuCmd.Flags().StringVar(&feishuAppSecret, "app-secret", "", "Feishu App Secret (or set FEISHU_APP_SECRET)")
	connectorFeishuCmd.Flags().StringVar(&feishuListenAddr, "listen", ":3979", "Address to listen for incoming Feishu webhook events")
	connectorFeishuCmd.Flags().StringVar(&feishuVerifyToken, "verify-token", "", "Feishu event subscription verification token (optional)")
}

func runConnectorTelegram(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Telegram Connector...")
	fmt.Println("─────────────────────────────────")

	// Check for bot token
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable not set")
	}

	// Create connector
	config := &telegram.Config{
		GatewayURL: connectorGatewayURL,
		BotToken:   botToken,
		ClientID:   connectorClientID,
	}

	connector, err := telegram.NewConnector(config)
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}
	defer connector.Close()

	// Setup signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Connect to Gateway
	if err := connector.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Gateway: %w", err)
	}

	// Start connector
	if err := connector.Start(ctx); err != nil {
		if err == context.Canceled {
			fmt.Println("\nConnector stopped")
			return nil
		}
		return fmt.Errorf("connector error: %w", err)
	}

	return nil
}

var connectorDiscordCmd = &cobra.Command{
	Use:   "discord",
	Short: "Start Discord connector",
	Long: `Start the Discord connector that bridges Discord to the SoulGate HTTP API.

The connector:
1. Connects to Discord as a bot
2. Listens for DMs and channel messages that @mention the bot
3. Forwards message text to the SoulGate HTTP /api/chat endpoint
4. Splits long responses into multiple messages (Discord 2000-char limit)
5. Replies directly to the user's message

Requirements:
- DISCORD_BOT_TOKEN environment variable
- Running SoulGate API server (soulgate api)

Discord bot setup:
- Create a bot at https://discord.com/developers/applications
- Enable "Message Content Intent" in the bot settings
- Invite the bot with scopes: bot + applications.commands
- Required permissions: Read Messages, Send Messages, Read Message History

Example:
  export DISCORD_BOT_TOKEN="your-bot-token"
  soulgate connector discord --gateway http://localhost:8080`,
	RunE: runConnectorDiscord,
}

func runConnectorDiscord(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Discord Connector...")
	fmt.Println("─────────────────────────────────")

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("DISCORD_BOT_TOKEN environment variable not set")
	}

	cfg := discord.Config{
		BotToken:   botToken,
		GatewayURL: discordGatewayURL,
		GuildID:    discordGuildID,
	}

	connector, err := discord.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Discord connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", discordGatewayURL)
	if discordGuildID != "" {
		fmt.Printf("Guild ID:    %s\n", discordGuildID)
	}
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nDiscord connector stopped")
	return nil
}

var connectorSlackCmd = &cobra.Command{
	Use:   "slack",
	Short: "Start Slack connector",
	Long: `Start the Slack connector that bridges Slack to the SoulGate HTTP API.

The connector:
1. Connects to Slack via Socket Mode (no public HTTP endpoint required)
2. Listens for DMs, app mentions (@bot), and messages in invited channels
3. Forwards message text to the SoulGate HTTP /api/chat endpoint
4. Replies in-thread when the original message is part of a thread
5. Formats long responses across multiple Block Kit section blocks

Requirements:
- SLACK_BOT_TOKEN environment variable  (xoxb-... OAuth bot token)
- SLACK_APP_TOKEN environment variable  (xapp-... app-level token)
- Running SoulGate API server (soulgate api)

Slack app setup:
- Create an app at https://api.slack.com/apps
- Enable Socket Mode and generate an app-level token (xapp-)
- Subscribe to bot events: message.channels, message.groups, message.im, app_mention
- Install the app to your workspace and copy the bot token (xoxb-)
- Required scopes: chat:write, channels:history, groups:history, im:history, app_mentions:read

Example:
  export SLACK_BOT_TOKEN="xoxb-..."
  export SLACK_APP_TOKEN="xapp-..."
  soulgate connector slack --gateway http://localhost:8080`,
	RunE: runConnectorSlack,
}

func runConnectorSlack(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Slack Connector...")
	fmt.Println("─────────────────────────────────")

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN environment variable not set (needs xoxb-... bot token)")
	}

	appToken := os.Getenv("SLACK_APP_TOKEN")
	if appToken == "" {
		return fmt.Errorf("SLACK_APP_TOKEN environment variable not set (needs xapp-... app-level token for Socket Mode)")
	}

	cfg := slackconnector.Config{
		BotToken:   botToken,
		AppToken:   appToken,
		GatewayURL: slackGatewayURL,
	}

	connector, err := slackconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Slack connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", slackGatewayURL)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	defer connector.Stop()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nSlack connector stopped")
	return nil
}

var connectorWhatsAppCmd = &cobra.Command{
	Use:   "whatsapp",
	Short: "Start WhatsApp connector",
	Long: `Start the WhatsApp connector that bridges WhatsApp to the SoulGate HTTP API.

The connector uses the WhatsApp Web multi-device API (go.mau.fi/whatsmeow) and:
1. On first run: displays a QR code in the terminal for you to scan with WhatsApp
   Open WhatsApp > Linked Devices > Link a Device, then scan the QR code.
2. On subsequent runs: reconnects automatically using the stored session.
3. Listens for incoming direct messages and group messages where the bot is mentioned.
4. Forwards message text to the SoulGate HTTP /api/chat endpoint.
5. Sends the AI response back to the same WhatsApp chat.

Session data is stored in a SQLite database in --data-dir so re-pairing is not
needed after a restart.

Requirements:
- Running SoulGate API server (soulgate api)
- A WhatsApp account to link (scan the QR code on first run)

Example:
  soulgate connector whatsapp --gateway http://localhost:8080 --data-dir .soulgate/whatsapp`,
	RunE: runConnectorWhatsApp,
}

func runConnectorWhatsApp(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting WhatsApp Connector...")
	fmt.Println("─────────────────────────────────")

	cfg := whatsapp.Config{
		GatewayURL: whatsappGatewayURL,
		DataDir:    whatsappDataDir,
	}

	connector, err := whatsapp.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", whatsappGatewayURL)
	fmt.Printf("Data dir:    %s\n", whatsappDataDir)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nWhatsApp connector stopped")
	return nil
}

var connectorSignalCmd = &cobra.Command{
	Use:   "signal",
	Short: "Start Signal connector",
	Long: `Start the Signal connector that bridges Signal messenger to the SoulGate HTTP API.

The connector:
1. Spawns signal-cli in JSON-RPC mode (signal-cli -u <phone> jsonRpc)
2. Reads incoming messages from signal-cli's stdout
3. Forwards message text to the SoulGate HTTP /api/chat endpoint
4. Sends AI responses back via signal-cli's JSON-RPC send method

The bot responds to:
- Direct messages (any 1-1 message sent to the registered number)
- Group messages that @mention the bot's phone number

Requirements:
- signal-cli installed and on PATH (or set --signal-cli to the full path)
  Download from: https://github.com/AsamK/signal-cli/releases
- The phone number must already be registered and verified with signal-cli:
  signal-cli -u +15551234567 register
  signal-cli -u +15551234567 verify <code>
- Running SoulGate API server (soulgate api)

Example:
  soulgate connector signal --phone +15551234567 --gateway http://localhost:8080
  soulgate connector signal --phone +15551234567 --signal-cli /usr/local/bin/signal-cli`,
	RunE: runConnectorSignal,
}

func runConnectorSignal(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Signal Connector...")
	fmt.Println("─────────────────────────────────")

	if signalPhoneNumber == "" {
		return fmt.Errorf("--phone is required (e.g. --phone +15551234567)")
	}

	cfg := signalconnector.Config{
		GatewayURL:  signalGatewayURL,
		PhoneNumber: signalPhoneNumber,
		SignalCLI:   signalCLIBinary,
	}

	connector, err := signalconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Signal connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Phone:       %s\n", signalPhoneNumber)
	fmt.Printf("Gateway URL: %s\n", signalGatewayURL)
	fmt.Printf("signal-cli:  %s\n", signalCLIBinary)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nSignal connector stopped")
	return nil
}

var connectorTeamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Start Microsoft Teams connector",
	Long: `Start the Microsoft Teams connector that bridges Teams to the SoulGate HTTP API.

The connector:
1. Starts an HTTP server to receive incoming Bot Framework v4 webhook events
2. Parses Activity JSON sent by Teams (type=message)
3. Strips @mentions and HTML tags from the message text
4. Forwards clean message text to the SoulGate HTTP /api/chat endpoint
5. Sends the AI response back via the Bot Framework REST API
6. Handles 1:1 chats, group chats, and channel conversations

Authentication:
- Incoming: Teams sends a JWT Bearer token; structural validation is performed
- Outgoing: The connector fetches an OAuth2 token using AppID + AppPassword and
  uses it when posting reply activities to the Bot Framework service URL

Requirements:
- TEAMS_APP_ID or --app-id      (Microsoft Teams Bot App ID / MicrosoftAppId)
- TEAMS_APP_PASSWORD or --app-password  (Bot App Password / client secret)
- Running SoulGate API server (soulgate api)

Teams bot setup:
- Create an Azure Bot resource at https://portal.azure.com
- Note the App ID (MicrosoftAppId) and generate a client secret (AppPassword)
- Set the messaging endpoint to: https://your-host:3978/api/messages
  (use ngrok or similar for local development)
- Add the Teams channel in the Azure Bot > Channels > Teams

Example:
  export TEAMS_APP_ID="your-app-id"
  export TEAMS_APP_PASSWORD="your-app-password"
  soulgate connector teams --gateway http://localhost:8080 --listen :3978`,
	RunE: runConnectorTeams,
}

func runConnectorTeams(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Microsoft Teams Connector...")
	fmt.Println("─────────────────────────────────")

	// Prefer explicit flags; fall back to environment variables.
	appID := teamsAppID
	if appID == "" {
		appID = os.Getenv("TEAMS_APP_ID")
	}
	appPassword := teamsAppPassword
	if appPassword == "" {
		appPassword = os.Getenv("TEAMS_APP_PASSWORD")
	}

	if appID == "" {
		return fmt.Errorf("Teams App ID is required: set --app-id or TEAMS_APP_ID")
	}
	if appPassword == "" {
		return fmt.Errorf("Teams App Password is required: set --app-password or TEAMS_APP_PASSWORD")
	}

	cfg := teamsconnector.Config{
		GatewayURL:  teamsGatewayURL,
		AppID:       appID,
		AppPassword: appPassword,
		ListenAddr:  teamsListenAddr,
	}

	connector, err := teamsconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Teams connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", teamsGatewayURL)
	fmt.Printf("Listen addr: %s\n", teamsListenAddr)
	fmt.Printf("App ID:      %s\n", appID)
	fmt.Println("Webhook URL: POST http://<your-host>" + teamsListenAddr + "/api/messages")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nTeams connector stopped")
	return nil
}

var connectorMatrixCmd = &cobra.Command{
	Use:   "matrix",
	Short: "Start Matrix connector",
	Long: `Start the Matrix connector that bridges Matrix to the SoulGate HTTP API.

The connector:
1. Long-polls the Matrix /_matrix/client/v3/sync endpoint for new events
2. Filters for m.room.message events with msgtype m.text
3. Skips own messages (sender == bot UserID) to prevent echo loops
4. Strips @mention patterns from the message text
5. Forwards clean message text to the SoulGate HTTP /api/chat endpoint
6. Sends the AI response back via PUT /_matrix/client/v3/rooms/{roomId}/send/...
7. Persists the since token to disk to avoid re-processing old messages on restart

The bot responds in every room it has been invited to.  Invite the bot to a
room and it will reply to every m.text message sent there.

Requirements:
- MATRIX_ACCESS_TOKEN or --access-token   (Matrix bot access token)
- MATRIX_USER_ID or --user-id             (e.g. @mybot:matrix.org)
- --homeserver                             (e.g. https://matrix.org)
- Running SoulGate API server (soulgate api)

Matrix bot setup:
1. Register a new account on your homeserver (this is the bot account)
2. Log in and obtain an access token:
   curl -X POST https://matrix.org/_matrix/client/v3/login \
     -d '{"type":"m.login.password","user":"mybot","password":"..."}'
3. Note the access_token and user_id from the response
4. Invite @mybot:matrix.org to any room you want it to respond in

Example:
  export MATRIX_ACCESS_TOKEN="syt_..."
  export MATRIX_USER_ID="@mybot:matrix.org"
  soulgate connector matrix --homeserver https://matrix.org --gateway http://localhost:8080`,
	RunE: runConnectorMatrix,
}

func runConnectorMatrix(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Matrix Connector...")
	fmt.Println("─────────────────────────────────")

	// Prefer explicit flags; fall back to environment variables.
	accessToken := matrixAccessToken
	if accessToken == "" {
		accessToken = os.Getenv("MATRIX_ACCESS_TOKEN")
	}
	userID := matrixUserID
	if userID == "" {
		userID = os.Getenv("MATRIX_USER_ID")
	}

	if matrixHomeserverURL == "" {
		return fmt.Errorf("--homeserver is required (e.g. --homeserver https://matrix.org)")
	}
	if accessToken == "" {
		return fmt.Errorf("Matrix access token is required: set --access-token or MATRIX_ACCESS_TOKEN")
	}
	if userID == "" {
		return fmt.Errorf("Matrix user ID is required: set --user-id or MATRIX_USER_ID (e.g. @mybot:matrix.org)")
	}

	cfg := matrixconnector.Config{
		GatewayURL:     matrixGatewayURL,
		HomeserverURL:  matrixHomeserverURL,
		AccessToken:    accessToken,
		UserID:         userID,
		SinceTokenPath: matrixSinceTokenPath,
	}

	connector, err := matrixconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Matrix connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Homeserver:  %s\n", matrixHomeserverURL)
	fmt.Printf("User ID:     %s\n", userID)
	fmt.Printf("Gateway URL: %s\n", matrixGatewayURL)
	fmt.Printf("Since token: %s\n", matrixSinceTokenPath)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nMatrix connector stopped")
	return nil
}

var connectorIMessageCmd = &cobra.Command{
	Use:   "imessage",
	Short: "Start iMessage connector (macOS only)",
	Long: `Start the iMessage connector that bridges Apple iMessage to the SoulGate HTTP API.

The connector:
1. Polls ~/Library/Messages/chat.db (read-only) for new incoming messages
2. Forwards each message text to the SoulGate HTTP /api/chat endpoint
3. Sends AI responses back to the original sender via AppleScript

macOS permissions required:
- Full Disk Access: System Settings > Privacy & Security > Full Disk Access
  Grant access to your terminal app (Terminal, iTerm2, etc.) so it can read
  ~/Library/Messages/chat.db.
- Automation: System Settings > Privacy & Security > Automation
  Enable the toggle for your terminal app under "Messages" so osascript can
  send replies.
- The Messages app must be running or launchable.

This connector only works on macOS.

Example:
  soulgate connector imessage --gateway http://localhost:8080
  soulgate connector imessage --gateway http://localhost:8080 --poll-interval 5s`,
	RunE: runConnectorIMessage,
}

func runConnectorIMessage(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting iMessage Connector...")
	fmt.Println("─────────────────────────────────")

	cfg := imessageconnector.Config{
		GatewayURL:   imessageGatewayURL,
		PollInterval: imessagePollInterval,
		ChatDBPath:   imessageChatDBPath,
	}

	connector, err := imessageconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create iMessage connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL:   %s\n", imessageGatewayURL)
	fmt.Printf("Chat DB:       %s\n", imessageChatDBPath)
	fmt.Printf("Poll interval: %s\n", imessagePollInterval)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\niMessage connector stopped")
	return nil
}

var connectorIRCCmd = &cobra.Command{
	Use:   "irc",
	Short: "Start IRC connector",
	Long: `Start the IRC connector that bridges an IRC network to the SoulGate HTTP API.

The connector uses only stdlib (net, crypto/tls, bufio) — no external IRC library required.

The connector:
1. Connects to the configured IRC server (TLS by default)
2. Registers with NICK / USER and optionally sends PASS
3. JOINs the configured channels
4. Responds to direct messages (queries) and channel messages that mention the bot nick
5. Forwards message text to the SoulGate HTTP /api/chat endpoint
6. Sends the AI response back via PRIVMSG, splitting long responses across multiple lines
7. Handles PING/PONG keepalive
8. Auto-reconnects on unexpected disconnection

Requirements:
- Running SoulGate API server (soulgate api)
- --nick flag (required)
- --channel flag (at least one, repeatable)

Example:
  soulgate connector irc --nick soulgate-bot --channel '#soulgate' --server irc.libera.chat:6697
  soulgate connector irc --nick mybot --channel '#myproject' --channel '#general' --no-tls`,
	RunE: runConnectorIRC,
}

func runConnectorIRC(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting IRC Connector...")
	fmt.Println("─────────────────────────────────")

	if len(ircChannels) == 0 {
		return fmt.Errorf("at least one --channel is required (e.g. --channel '#soulgate')")
	}

	cfg := ircconnector.Config{
		GatewayURL: ircGatewayURL,
		Server:     ircServer,
		Nick:       ircNick,
		Channels:   ircChannels,
		UseTLS:     ircUseTLS,
		Password:   ircPassword,
	}

	connector, err := ircconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create IRC connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", ircGatewayURL)
	fmt.Printf("Server:      %s\n", ircServer)
	fmt.Printf("Nick:        %s\n", ircNick)
	fmt.Printf("Channels:    %v\n", ircChannels)
	fmt.Printf("TLS:         %v\n", ircUseTLS)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nIRC connector stopped")
	return nil
}

var connectorTwitchCmd = &cobra.Command{
	Use:   "twitch",
	Short: "Start Twitch chat connector",
	Long: `Start the Twitch chat connector that bridges Twitch chat to the SoulGate HTTP API.

Twitch chat is IRC-over-TLS with OAuth authentication. This connector uses only
stdlib (net, crypto/tls, bufio) — no external library required.

The connector:
1. Connects to irc.chat.twitch.tv:6697 via TLS
2. Authenticates with PASS oauth:TOKEN and NICK username
3. Requests Twitch capabilities (membership, tags, commands)
4. JOINs the configured channels
5. Responds only when the bot name is @mentioned (to avoid flooding busy chats)
6. Forwards message text to the SoulGate HTTP /api/chat endpoint
7. Sends the AI response back prefixed with @username
8. Enforces Twitch's 20-messages-per-30-seconds rate limit
9. Auto-reconnects on unexpected disconnection and honours RECONNECT commands

Requirements:
- Running SoulGate API server (soulgate api)
- Twitch OAuth token: https://twitchapps.com/tmi/ or set TWITCH_OAUTH_TOKEN
- --nick flag set to your bot's Twitch username (lowercase, required)
- --channel flag (at least one, repeatable)

Example:
  export TWITCH_OAUTH_TOKEN="oauth:abcdef123456"
  soulgate connector twitch --nick mySoulBot --channel '#streamer'
  soulgate connector twitch --nick mySoulBot --channel '#chan1' --channel '#chan2'`,
	RunE: runConnectorTwitch,
}

func runConnectorTwitch(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Twitch Connector...")
	fmt.Println("─────────────────────────────────")

	// Prefer explicit flag; fall back to environment variable.
	oauthToken := twitchOAuthToken
	if oauthToken == "" {
		oauthToken = os.Getenv("TWITCH_OAUTH_TOKEN")
	}
	if oauthToken == "" {
		return fmt.Errorf("Twitch OAuth token is required: set --oauth-token or TWITCH_OAUTH_TOKEN")
	}

	if len(twitchChannels) == 0 {
		return fmt.Errorf("at least one --channel is required (e.g. --channel '#streamer')")
	}

	cfg := twitchconnector.Config{
		GatewayURL: twitchGatewayURL,
		OAuthToken: oauthToken,
		Nick:       twitchNick,
		Channels:   twitchChannels,
	}

	connector, err := twitchconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Twitch connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", twitchGatewayURL)
	fmt.Printf("Nick:        %s\n", twitchNick)
	fmt.Printf("Channels:    %v\n", twitchChannels)
	fmt.Println("Rate limit:  20 messages / 30 seconds (Twitch limit)")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nTwitch connector stopped")
	return nil
}

var connectorNostrCmd = &cobra.Command{
	Use:   "nostr",
	Short: "Start Nostr connector (read-only v1)",
	Long: `Start the Nostr connector that bridges Nostr relays to the SoulGate HTTP API.

Nostr is a decentralized protocol where clients communicate through relay servers
using WebSockets and JSON.  Events are cryptographically signed by their authors.

The connector (v1 - read-only):
1. Connects to each configured relay via WebSocket (gorilla/websocket)
2. Subscribes for kind=1 (text note) and kind=4 (encrypted DM) events that tag
   our public key in a "p" tag - i.e., direct @mentions
3. Validates every received event ID against the NIP-01 canonical hash to prevent
   relay-injected tampered events
4. Forwards mention content to the SoulGate HTTP /api/chat endpoint
5. Logs "Would reply: <response>" - publishing requires secp256k1 signing (v2)
6. Automatically reconnects to any relay that disconnects

Kind=4 DMs are received as NIP-04 AES-256-CBC ciphertext.  Decryption requires
the ECDH shared secret and is deferred to v2.

Public key is derived from the private key using pure-Go secp256k1 arithmetic.
No external crypto dependency is needed for v1.

Requirements:
- NOSTR_PRIVATE_KEY or --private-key   (hex-encoded 32-byte secp256k1 key)
- At least one --relay flag
- Running SoulGate API server (soulgate api)

Generating a key pair (openssl):
  openssl rand -hex 32    # this is your private key
  soulgate connector nostr --private-key <hex> --relay wss://relay.damus.io
    -> prints your pubkey on startup

Example:
  export NOSTR_PRIVATE_KEY="0123456789abcdef..."
  soulgate connector nostr \
    --relay wss://relay.damus.io \
    --relay wss://nos.lol \
    --gateway http://localhost:8080`,
	RunE: runConnectorNostr,
}

func runConnectorNostr(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Nostr Connector...")
	fmt.Println("─────────────────────────────────")

	// Prefer explicit flag; fall back to environment variable.
	privKey := nostrPrivateKey
	if privKey == "" {
		privKey = os.Getenv("NOSTR_PRIVATE_KEY")
	}
	if privKey == "" {
		return fmt.Errorf("Nostr private key is required: set --private-key or NOSTR_PRIVATE_KEY")
	}

	if len(nostrRelays) == 0 {
		return fmt.Errorf("at least one --relay is required (e.g. --relay wss://relay.damus.io)")
	}

	cfg := nostrconnector.Config{
		GatewayURL: nostrGatewayURL,
		PrivateKey: privKey,
		Relays:     nostrRelays,
	}

	connector, err := nostrconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Nostr connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Public key:  %s\n", connector.PubKey())
	fmt.Printf("Relays:      %v\n", nostrRelays)
	fmt.Printf("Gateway URL: %s\n", nostrGatewayURL)
	fmt.Println("Mode:        read-only (publishing deferred to v2)")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nNostr connector stopped")
	return nil
}

var connectorMattermostCmd = &cobra.Command{
	Use:   "mattermost",
	Short: "Start Mattermost connector",
	Long: `Start the Mattermost connector that bridges Mattermost to the SoulGate HTTP API.

The connector:
1. Fetches the bot user profile from /api/v4/users/me to obtain the bot user ID
2. Connects to the Mattermost WebSocket API at wss://<server>/api/v4/websocket
3. Authenticates via an authentication_challenge WebSocket message
4. Listens for "posted" events (new messages)
5. Filters: ignores the bot's own messages, only responds to DMs or @mentions
6. Forwards message text to the SoulGate HTTP /api/chat endpoint
7. Replies via POST /api/v4/posts with channel_id and root_id for threading
8. Reconnects automatically on WebSocket disconnect

Requirements:
- MATTERMOST_TOKEN environment variable or --token flag
- --server flag with the Mattermost server URL (required)
- Running SoulGate API server (soulgate api)

Mattermost bot setup:
- Create a bot account: System Console > Integrations > Bot Accounts > Add Bot Account
  (or use a personal access token from User > Account Settings > Security > Personal Access Tokens)
- Copy the bot access token
- Invite the bot to channels or teams as needed

Example:
  export MATTERMOST_TOKEN="your-bot-token"
  soulgate connector mattermost --server https://mattermost.example.com --gateway http://localhost:8080`,
	RunE: runConnectorMattermost,
}

func runConnectorMattermost(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Mattermost Connector...")
	fmt.Println("─────────────────────────────────")

	token := mattermostToken
	if token == "" {
		token = os.Getenv("MATTERMOST_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("Mattermost token is required: set --token or MATTERMOST_TOKEN")
	}

	if mattermostServerURL == "" {
		return fmt.Errorf("--server is required (e.g. --server https://mattermost.example.com)")
	}

	cfg := mattermostconnector.Config{
		GatewayURL:  mattermostGatewayURL,
		ServerURL:   mattermostServerURL,
		Token:       token,
		BotUsername: mattermostBotUsername,
	}

	connector, err := mattermostconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Mattermost connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Server URL:  %s\n", mattermostServerURL)
	fmt.Printf("Gateway URL: %s\n", mattermostGatewayURL)
	if mattermostBotUsername != "" {
		fmt.Printf("Bot user:    %s\n", mattermostBotUsername)
	}
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nMattermost connector stopped")
	return nil
}

var connectorFeishuCmd = &cobra.Command{
	Use:   "feishu",
	Short: "Start Feishu (Lark) connector",
	Long: `Start the Feishu (Lark) connector that bridges Feishu to the SoulGate HTTP API.

The connector:
1. Starts an HTTP server on --listen to receive Feishu event subscription webhooks
2. Handles the URL verification challenge that Feishu sends on first configuration
3. Receives im.message.receive_v1 events for incoming messages
4. Extracts text content from the message payload
5. Forwards message text to the SoulGate HTTP /api/chat endpoint
6. Obtains a tenant_access_token from the Feishu auth API (cached and refreshed automatically)
7. Sends the AI response via POST /open-apis/im/v1/messages to the originating chat
8. Works for both DMs (p2p) and group chats

Authentication:
- Incoming: optional verification token matching validates the event source
- Outgoing: tenant_access_token obtained using AppID + AppSecret (auto-managed)

Requirements:
- FEISHU_APP_ID or --app-id         (Feishu application App ID)
- FEISHU_APP_SECRET or --app-secret (Feishu application App Secret)
- Running SoulGate API server (soulgate api)
- A publicly reachable webhook endpoint (use ngrok for local development)

Feishu bot setup:
- Create an application at https://open.feishu.cn/app (or Lark developer console)
- Under "Permissions & Scopes", add: im:message, im:message:send_as_bot
- Under "Event Subscriptions", set the webhook URL to:
    https://your-host:3979/feishu/events
- Subscribe to the event: im.message.receive_v1
- Copy the App ID and App Secret from "Credentials & Basic Info"

Example:
  export FEISHU_APP_ID="cli_..."
  export FEISHU_APP_SECRET="your-app-secret"
  soulgate connector feishu --gateway http://localhost:8080 --listen :3979`,
	RunE: runConnectorFeishu,
}

func runConnectorFeishu(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting Feishu (Lark) Connector...")
	fmt.Println("─────────────────────────────────")

	appID := feishuAppID
	if appID == "" {
		appID = os.Getenv("FEISHU_APP_ID")
	}
	appSecret := feishuAppSecret
	if appSecret == "" {
		appSecret = os.Getenv("FEISHU_APP_SECRET")
	}

	if appID == "" {
		return fmt.Errorf("Feishu App ID is required: set --app-id or FEISHU_APP_ID")
	}
	if appSecret == "" {
		return fmt.Errorf("Feishu App Secret is required: set --app-secret or FEISHU_APP_SECRET")
	}

	cfg := feishuconnector.Config{
		GatewayURL:  feishuGatewayURL,
		AppID:       appID,
		AppSecret:   appSecret,
		ListenAddr:  feishuListenAddr,
		VerifyToken: feishuVerifyToken,
	}

	connector, err := feishuconnector.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Feishu connector: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Printf("Gateway URL: %s\n", feishuGatewayURL)
	fmt.Printf("Listen addr: %s\n", feishuListenAddr)
	fmt.Printf("App ID:      %s\n", appID)
	fmt.Printf("Webhook URL: POST http://<your-host>%s/feishu/events\n", feishuListenAddr)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	if err := connector.Start(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("connector error: %w", err)
	}

	fmt.Println("\nFeishu connector stopped")
	return nil
}

