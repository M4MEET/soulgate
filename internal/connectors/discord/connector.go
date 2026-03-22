package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/M4MEET/soulgate/internal/connectors"
	"github.com/bwmarrin/discordgo"
)

const (
	// discordMaxMessageLen is Discord's hard limit for a single message.
	discordMaxMessageLen = 2000

	// defaultGatewayURL is the fallback when Config.GatewayURL is empty.
	defaultGatewayURL = "http://localhost:8080"
)

// Config holds all Discord connector configuration.
type Config struct {
	// BotToken is the Discord bot token (required).
	BotToken string

	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080.
	GatewayURL string

	// GuildID restricts the connector to a specific guild.
	// When empty the bot responds in all guilds it belongs to.
	GuildID string
}

// gatewayRequest is the payload sent to POST /api/chat.
type gatewayRequest struct {
	Message string `json:"message"`
}

// gatewayResponse is the payload returned from POST /api/chat.
type gatewayResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// Connector bridges Discord to the SoulGate HTTP API.
type Connector struct {
	config     Config
	session    *discordgo.Session
	httpClient *http.Client
	stopOnce   sync.Once
	stopped    chan struct{}
}

// New creates a new Discord connector.  It validates the configuration and
// creates a discordgo session, but does not yet open the Discord connection.
func New(config Config) (*Connector, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("discord: BotToken is required")
	}
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}

	s, err := discordgo.New("Bot " + config.BotToken)
	if err != nil {
		return nil, fmt.Errorf("discord: failed to create session: %w", err)
	}

	// Request the intents we need: guild messages, DMs, and message content.
	s.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent

	c := &Connector{
		config:     config,
		session:    s,
		httpClient: &http.Client{Timeout: 0}, // no timeout — agentic loops can be long
		stopped:    make(chan struct{}),
	}

	// Register the message handler before opening the connection so we don't
	// miss any events that arrive immediately after Open().
	s.AddHandler(c.handleMessage)

	return c, nil
}

// Start opens the Discord WebSocket connection and blocks until ctx is
// cancelled or Stop is called.
func (c *Connector) Start(ctx context.Context) error {
	log.Printf("discord: connecting to Discord...")
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("discord: failed to open session: %w", err)
	}

	log.Printf("discord: connected as %s#%s (gateway: %s)",
		c.session.State.User.Username,
		c.session.State.User.Discriminator,
		c.config.GatewayURL,
	)

	select {
	case <-ctx.Done():
		return c.Stop()
	case <-c.stopped:
		return nil
	}
}

// Stop closes the Discord session gracefully.  It is safe to call more than
// once.
func (c *Connector) Stop() error {
	var closeErr error
	c.stopOnce.Do(func() {
		log.Printf("discord: stopping connector")
		closeErr = c.session.Close()
		close(c.stopped)
	})
	return closeErr
}

// handleMessage is the discordgo event handler for MessageCreate events.
// It runs in a goroutine spawned by discordgo, so each invocation is
// already concurrent — we immediately hand off to processMessage so that the
// typing indicator can be shown and the HTTP call can proceed without blocking
// the event loop.
func (c *Connector) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages sent by any bot (including ourselves).
	if m.Author == nil || m.Author.Bot {
		return
	}

	// Optionally restrict to a specific guild.
	if c.config.GuildID != "" && m.GuildID != c.config.GuildID {
		return
	}

	isDM := m.GuildID == ""
	isMention := c.isMentioned(s, m)

	if !isDM && !isMention {
		// Only respond to DMs and explicit @mentions in guild channels.
		return
	}

	// Strip the bot mention from the message text so the model only sees the
	// actual request.
	text := c.stripMention(s, m)
	text = strings.TrimSpace(text)

	// Build a full prompt that includes attachment descriptions when present.
	prompt := c.buildPrompt(m, text)
	if prompt == "" {
		return
	}

	go c.processMessage(s, m, prompt)
}

// buildPrompt augments the text with descriptions of any Discord attachments
// (images, audio, other files) so the AI is aware of what was sent.
func (c *Connector) buildPrompt(m *discordgo.MessageCreate, text string) string {
	if len(m.Attachments) == 0 {
		return text
	}

	var sb strings.Builder
	if text != "" {
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	for _, att := range m.Attachments {
		ext := strings.ToLower(filepath.Ext(att.Filename))
		switch {
		case isDiscordImage(ext):
			fmt.Fprintf(&sb, "[Image attachment: %s — URL: %s]\n", att.Filename, att.URL)
		case isDiscordAudio(ext):
			fmt.Fprintf(&sb, "[Audio attachment: %s — URL: %s]\n", att.Filename, att.URL)
		default:
			fmt.Fprintf(&sb, "[File attachment: %s (%d bytes) — URL: %s]\n",
				att.Filename, att.Size, att.URL)
		}
	}

	return strings.TrimSpace(sb.String())
}

// processMessage sends the typing indicator, calls the gateway, then replies.
func (c *Connector) processMessage(s *discordgo.Session, m *discordgo.MessageCreate, text string) {
	log.Printf("discord: message from %s#%s in channel %s: %q",
		m.Author.Username, m.Author.Discriminator, m.ChannelID, text)

	// Show typing indicator while we wait for the gateway.
	if err := s.ChannelTyping(m.ChannelID); err != nil {
		log.Printf("discord: failed to send typing indicator: %v", err)
	}

	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("discord: gateway error: %v", err)
		response = fmt.Sprintf("Error reaching SoulGate: %v", err)
	}

	c.sendResponse(s, m.ChannelID, m.ID, response)

	// Add a checkmark reaction once the full response is delivered.
	if err := s.MessageReactionAdd(m.ChannelID, m.ID, "✅"); err != nil {
		log.Printf("discord: failed to add reaction: %v", err)
	}
}

// sendToGateway POSTs the message to the SoulGate /api/chat endpoint and
// returns the text response.
func (c *Connector) sendToGateway(message string) (string, error) {
	payload, err := json.Marshal(gatewayRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(c.config.GatewayURL, "/") + "/api/chat"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("POST %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	var gwResp gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gwResp); err != nil {
		return "", fmt.Errorf("failed to decode gateway response: %w", err)
	}

	if gwResp.Error != "" {
		return "", fmt.Errorf("gateway returned error: %s", gwResp.Error)
	}
	if gwResp.Response == "" {
		return "", fmt.Errorf("gateway returned empty response")
	}

	log.Printf("discord: gateway responded with %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendResponse parses media refs from the AI response and delivers them as
// Discord file attachments.  Plain text is sent as a reply (chunked if needed).
// If the response contains code blocks the first chunk is wrapped in an embed.
func (c *Connector) sendResponse(s *discordgo.Session, channelID, replyToID, response string) {
	refs := connectors.ExtractMediaRefs(response)
	caption := connectors.CleanMediaRefs(response)

	// Send any media attachments first.
	if len(refs) > 0 {
		c.sendMediaRefs(s, channelID, replyToID, refs, caption)
		// Caption was consumed by the media send; only send remaining text if
		// the caption helper left something behind (it shouldn't, but guard).
		if caption == "" {
			return
		}
	}

	// No media (or caption was not consumed) — send text.
	c.sendTextResponse(s, channelID, replyToID, caption)
}

// sendMediaRefs uploads local files as Discord attachments, sending any
// remaining caption as text alongside the first attachment.
func (c *Connector) sendMediaRefs(
	s *discordgo.Session,
	channelID, replyToID string,
	refs []connectors.MediaRef,
	caption string,
) {
	var discordFiles []*discordgo.File

	for _, ref := range refs {
		f, err := os.Open(ref.Path)
		if err != nil {
			log.Printf("discord: cannot open media file %q: %v", ref.Path, err)
			continue
		}
		// discordgo takes an io.Reader; we defer close after Send.
		discordFiles = append(discordFiles, &discordgo.File{
			Name:        filepath.Base(ref.Path),
			ContentType: mimeForRef(ref),
			Reader:      f,
		})
		defer f.Close() //nolint:revive // files are closed after Send returns
	}

	if len(discordFiles) == 0 {
		// All files failed to open — fall through to text.
		c.sendTextResponse(s, channelID, replyToID, caption)
		return
	}

	msg := &discordgo.MessageSend{
		Files: discordFiles,
		Reference: &discordgo.MessageReference{
			MessageID: replyToID,
			ChannelID: channelID,
		},
	}
	if caption != "" {
		// Put the caption in a simple embed so it doesn't clutter the file list.
		msg.Embed = &discordgo.MessageEmbed{
			Description: truncateEmbed(caption),
		}
	}

	if _, err := s.ChannelMessageSendComplex(channelID, msg); err != nil {
		log.Printf("discord: failed to send media: %v", err)
		// Fall back: send caption as plain text.
		if caption != "" {
			c.sendTextResponse(s, channelID, replyToID, caption)
		}
	}
}

// sendTextResponse sends the AI text response.  When the text contains a code
// block the first chunk is wrapped in a Discord embed for better formatting.
// Long responses are chunked to stay within Discord's 2000-char limit.
func (c *Connector) sendTextResponse(s *discordgo.Session, channelID, replyToID, response string) {
	if response == "" {
		return
	}

	// If the response contains a code block, use an embed for the first chunk.
	if strings.Contains(response, "```") {
		c.sendEmbed(s, channelID, replyToID, response)
		return
	}

	chunks := splitMessage(response, discordMaxMessageLen)
	for i, chunk := range chunks {
		var err error
		if i == 0 {
			_, err = s.ChannelMessageSendReply(channelID, chunk, &discordgo.MessageReference{
				MessageID: replyToID,
				ChannelID: channelID,
			})
		} else {
			_, err = s.ChannelMessageSend(channelID, chunk)
		}
		if err != nil {
			log.Printf("discord: failed to send message chunk %d/%d: %v", i+1, len(chunks), err)
		}
	}
}

// sendEmbed sends the response wrapped in a Discord embed.  If the text is too
// long for a single embed description (4096 chars), the overflow is sent as
// plain follow-up messages.
func (c *Connector) sendEmbed(s *discordgo.Session, channelID, replyToID, response string) {
	const embedMaxDesc = 4096

	head := response
	tail := ""
	if len(response) > embedMaxDesc {
		head = response[:embedMaxDesc]
		tail = response[embedMaxDesc:]
	}

	msg := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Description: head,
		},
		Reference: &discordgo.MessageReference{
			MessageID: replyToID,
			ChannelID: channelID,
		},
	}
	if _, err := s.ChannelMessageSendComplex(channelID, msg); err != nil {
		log.Printf("discord: failed to send embed: %v", err)
		// Fall back to plain chunked text.
		c.sendTextResponse(s, channelID, replyToID, response)
		return
	}

	// Send overflow as plain text.
	if tail != "" {
		for _, chunk := range splitMessage(tail, discordMaxMessageLen) {
			if _, err := s.ChannelMessageSend(channelID, chunk); err != nil {
				log.Printf("discord: failed to send embed overflow: %v", err)
			}
		}
	}
}

// isMentioned reports whether the bot user is mentioned in the message.
func (c *Connector) isMentioned(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	botID := s.State.User.ID
	for _, u := range m.Mentions {
		if u.ID == botID {
			return true
		}
	}
	return false
}

// stripMention removes all @BotName mentions from the message content so the
// model receives clean input.
func (c *Connector) stripMention(s *discordgo.Session, m *discordgo.MessageCreate) string {
	text := m.Content
	botID := s.State.User.ID
	// Discord encodes mentions as <@USERID> or <@!USERID>.
	text = strings.ReplaceAll(text, "<@"+botID+">", "")
	text = strings.ReplaceAll(text, "<@!"+botID+">", "")
	return text
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// splitMessage splits text into chunks of at most maxLen runes, breaking on
// whitespace boundaries where possible to avoid cutting words.
func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		chunk := text[:maxLen]
		// Try to break at the last newline within the chunk.
		idx := strings.LastIndex(chunk, "\n")
		if idx <= 0 {
			// Fall back to the last space.
			idx = strings.LastIndex(chunk, " ")
		}
		if idx <= 0 {
			// No whitespace found — hard-split at maxLen.
			idx = maxLen
		}

		chunks = append(chunks, strings.TrimRight(text[:idx], " "))
		text = strings.TrimLeft(text[idx:], " \n")
	}
	return chunks
}

// truncateEmbed truncates text to Discord's embed description limit.
func truncateEmbed(text string) string {
	const limit = 4096
	if len(text) <= limit {
		return text
	}
	return text[:limit-3] + "..."
}

// mimeForRef returns a plausible Content-Type for a media ref.
func mimeForRef(ref connectors.MediaRef) string {
	ext := strings.ToLower(filepath.Ext(ref.Path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".opus":
		return "audio/opus"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// isDiscordImage reports whether ext is a known image extension.
func isDiscordImage(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return true
	}
	return false
}

// isDiscordAudio reports whether ext is a known audio extension.
func isDiscordAudio(ext string) bool {
	switch ext {
	case ".mp3", ".ogg", ".oga", ".wav", ".m4a", ".opus", ".flac":
		return true
	}
	return false
}
