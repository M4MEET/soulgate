// Package twitch provides a connector that bridges Twitch chat to the
// SoulGate HTTP API. Twitch chat is IRC-over-TLS with OAuth authentication and
// a handful of Twitch-specific capabilities, so no external dependencies are
// required — just stdlib net, bufio, and crypto/tls.
package twitch

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultGatewayURL = "http://localhost:8080"
	twitchIRCServer   = "irc.chat.twitch.tv:6697"

	// Twitch rate limits: 20 messages per 30 seconds for regular bots.
	// We enforce this conservatively as 20 messages / 30 s = one message
	// every 1.5 s.  We use a token-bucket style ticker.
	twitchRateWindow = 30 * time.Second
	twitchMaxMsgs    = 20

	// maxMessageLen keeps responses well within Twitch's 500-character limit.
	maxMessageLen = 480

	reconnectDelay = 5 * time.Second
	writeTimeout   = 30 * time.Second
	pingTimeout    = 5 * time.Minute
)

// Config holds all Twitch connector configuration.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080.
	GatewayURL string

	// OAuthToken is the Twitch OAuth token, with or without the "oauth:" prefix.
	// Set via --oauth-token flag or TWITCH_OAUTH_TOKEN environment variable.
	OAuthToken string

	// Nick is the Twitch username of the bot account (must be lowercase).
	Nick string

	// Channels is the list of Twitch channels to join, e.g. ["#streamer"].
	// Channel names must include the leading "#".
	Channels []string
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

// twitchMessage holds a parsed IRC message from the Twitch server.
type twitchMessage struct {
	// Tags contains the IRCv3 tag map (key=value pairs before the prefix).
	Tags map[string]string
	// Prefix is the :nick!user@host portion (without the leading colon).
	Prefix string
	// Nick is extracted from Prefix for convenience.
	Nick string
	// Command is the IRC command or Twitch-specific command.
	Command string
	// Params are the positional parameters.
	Params []string
	// Trailing is the final parameter after the " :" separator.
	Trailing string
}

// rateLimiter enforces Twitch's 20-messages-per-30-seconds constraint.
// It uses a simple token bucket: tokens are added at a fixed interval and
// each send call consumes one token.
type rateLimiter struct {
	mu     sync.Mutex
	tokens int
	ticker *time.Ticker
	done   chan struct{}
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		tokens: twitchMaxMsgs,
		// Replenish one token every (30s / 20) = 1.5 seconds.
		ticker: time.NewTicker(twitchRateWindow / twitchMaxMsgs),
		done:   make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-rl.ticker.C:
				rl.mu.Lock()
				if rl.tokens < twitchMaxMsgs {
					rl.tokens++
				}
				rl.mu.Unlock()
			case <-rl.done:
				return
			}
		}
	}()

	return rl
}

// acquire blocks until a send token is available or ctx is cancelled.
func (rl *rateLimiter) acquire(ctx context.Context) error {
	for {
		rl.mu.Lock()
		if rl.tokens > 0 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (rl *rateLimiter) stop() {
	rl.ticker.Stop()
	close(rl.done)
}

// Connector bridges Twitch chat to the SoulGate HTTP API.
type Connector struct {
	config     Config
	httpClient *http.Client
	rl         *rateLimiter

	mu       sync.Mutex
	conn     net.Conn
	writer   *bufio.Writer
	stopped  chan struct{}
	stopOnce sync.Once
}

// New creates a new Twitch connector, normalising configuration values.
func New(cfg Config) (*Connector, error) {
	if cfg.Nick == "" {
		return nil, fmt.Errorf("twitch: Nick is required")
	}
	if cfg.OAuthToken == "" {
		return nil, fmt.Errorf("twitch: OAuthToken is required (set TWITCH_OAUTH_TOKEN)")
	}
	if len(cfg.Channels) == 0 {
		return nil, fmt.Errorf("twitch: at least one channel is required")
	}

	if cfg.GatewayURL == "" {
		cfg.GatewayURL = defaultGatewayURL
	}

	// Normalise nick to lowercase (Twitch requirement).
	cfg.Nick = strings.ToLower(cfg.Nick)

	// Ensure the token has the "oauth:" prefix.
	if !strings.HasPrefix(cfg.OAuthToken, "oauth:") {
		cfg.OAuthToken = "oauth:" + cfg.OAuthToken
	}

	// Normalise channel names: must be lowercase and start with "#".
	for i, ch := range cfg.Channels {
		ch = strings.ToLower(ch)
		if !strings.HasPrefix(ch, "#") {
			ch = "#" + ch
		}
		cfg.Channels[i] = ch
	}

	return &Connector{
		config:     cfg,
		httpClient: &http.Client{Timeout: 0}, // agentic loops can be long
		rl:         newRateLimiter(),
		stopped:    make(chan struct{}),
	}, nil
}

// Start connects to Twitch IRC and processes messages until ctx is cancelled.
// It reconnects automatically on unexpected disconnections.
func (c *Connector) Start(ctx context.Context) error {
	defer c.rl.stop()

	for {
		if err := c.run(ctx); err != nil {
			select {
			case <-c.stopped:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			log.Printf("twitch: disconnected: %v — reconnecting in %s", err, reconnectDelay)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopped:
			return nil
		case <-time.After(reconnectDelay):
		}
	}
}

// Stop signals the connector to shut down gracefully.
func (c *Connector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopped)
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			_, _ = fmt.Fprintf(c.writer, "QUIT :shutdown\r\n")
			_ = c.writer.Flush()
			c.conn.Close()
		}
		c.mu.Unlock()
	})
}

// run performs one connection attempt: authenticate, request capabilities,
// join channels, and read until the connection drops or ctx is cancelled.
func (c *Connector) run(ctx context.Context) error {
	host, _, _ := net.SplitHostPort(twitchIRCServer)
	conn, err := tls.Dial("tcp", twitchIRCServer, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.writer = bufio.NewWriter(conn)
	c.mu.Unlock()

	defer func() {
		conn.Close()
		c.mu.Lock()
		c.conn = nil
		c.writer = nil
		c.mu.Unlock()
	}()

	// Authenticate and request Twitch capabilities.
	if err := c.authenticate(); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	reader := bufio.NewReader(conn)

	// Wait for the server to confirm login (001 numeric or GLOBALUSERSTATE).
	if err := c.waitForLogin(ctx, reader); err != nil {
		return err
	}

	// Join all configured channels.
	for _, ch := range c.config.Channels {
		if err := c.writeLine(fmt.Sprintf("JOIN %s", ch)); err != nil {
			return fmt.Errorf("JOIN %s: %w", ch, err)
		}
	}

	log.Printf("twitch: connected as %s, joined %v", c.config.Nick, c.config.Channels)

	// Main read loop.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopped:
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(pingTimeout + 30*time.Second))

		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		msg := parseTwitchMessage(line)
		c.handleMessage(ctx, msg)
	}
}

// authenticate sends the OAuth PASS, NICK, and capability requests.
func (c *Connector) authenticate() error {
	// Capability requests must be sent before PASS/NICK on Twitch.
	if err := c.writeLine("CAP REQ :twitch.tv/membership twitch.tv/tags twitch.tv/commands"); err != nil {
		return err
	}
	if err := c.writeLine(fmt.Sprintf("PASS %s", c.config.OAuthToken)); err != nil {
		return err
	}
	return c.writeLine(fmt.Sprintf("NICK %s", c.config.Nick))
}

// waitForLogin reads until we see the 001 welcome numeric or GLOBALUSERSTATE,
// both of which indicate successful authentication. It handles PING during the
// wait and returns an error on authentication failure (notice or 403).
func (c *Connector) waitForLogin(ctx context.Context, r *bufio.Reader) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopped:
			return fmt.Errorf("stopped")
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		line, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read during login: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		msg := parseTwitchMessage(line)

		switch msg.Command {
		case "PING":
			_ = c.writeLine("PONG :" + msg.Trailing)

		case "001":
			// Welcome — login successful.
			return nil

		case "GLOBALUSERSTATE":
			// Twitch sends this after capability grant — also signals success.
			return nil

		case "NOTICE":
			// Authentication failure typically arrives as a NOTICE with the
			// message "Login authentication failed".
			if strings.Contains(strings.ToLower(msg.Trailing), "login authentication failed") {
				return fmt.Errorf("authentication failed: %s", msg.Trailing)
			}
		}
	}
}

// handleMessage dispatches a parsed message to the appropriate handler.
func (c *Connector) handleMessage(ctx context.Context, msg *twitchMessage) {
	switch msg.Command {
	case "PING":
		_ = c.writeLine("PONG :" + msg.Trailing)

	case "PRIVMSG":
		c.handlePrivmsg(ctx, msg)

	case "RECONNECT":
		// Twitch can request a reconnect gracefully.
		log.Printf("twitch: server requested reconnect")
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}
}

// handlePrivmsg processes a chat message and responds if the bot is mentioned.
func (c *Connector) handlePrivmsg(ctx context.Context, msg *twitchMessage) {
	if len(msg.Params) == 0 {
		return
	}

	channel := msg.Params[0]
	text := msg.Trailing
	username := msg.Nick
	botNick := c.config.Nick

	// Skip messages from the bot itself to prevent echo loops.
	if strings.EqualFold(username, botNick) {
		return
	}

	// Only respond when mentioned to avoid flooding busy chats.
	lower := strings.ToLower(text)
	lowerNick := strings.ToLower(botNick)
	if !strings.Contains(lower, "@"+lowerNick) && !strings.HasPrefix(lower, lowerNick+":") && !strings.HasPrefix(lower, lowerNick+" ") {
		return
	}

	// Strip the @mention or prefix from the text.
	text = stripTwitchMention(text, botNick)
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	log.Printf("twitch: message from %s in %s: %q", username, channel, text)

	go func() {
		response, err := c.sendToGateway(text)
		if err != nil {
			log.Printf("twitch: gateway error: %v", err)
			response = fmt.Sprintf("@%s Error: %v", username, err)
		} else {
			// Prefix the response with @username for Twitch convention.
			response = fmt.Sprintf("@%s %s", username, response)
		}

		c.sendResponse(ctx, channel, response)
	}()
}

// sendToGateway POSTs the message text to /api/chat and returns the response.
func (c *Connector) sendToGateway(message string) (string, error) {
	payload, err := json.Marshal(gatewayRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.config.GatewayURL, "/") + "/api/chat"
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	var gwResp gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gwResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if gwResp.Error != "" {
		return "", fmt.Errorf("gateway error: %s", gwResp.Error)
	}
	if gwResp.Response == "" {
		return "", fmt.Errorf("gateway returned empty response")
	}

	return gwResp.Response, nil
}

// sendResponse sends one or more PRIVMSG lines to channel, rate-limiting each
// send and splitting the text to respect Twitch's character limit.
func (c *Connector) sendResponse(ctx context.Context, channel, text string) {
	lines := splitLines(text, maxMessageLen)
	for _, line := range lines {
		if err := c.rl.acquire(ctx); err != nil {
			log.Printf("twitch: rate limiter cancelled: %v", err)
			return
		}

		if err := c.writeLine(fmt.Sprintf("PRIVMSG %s :%s", channel, line)); err != nil {
			log.Printf("twitch: failed to send PRIVMSG: %v", err)
			return
		}
	}
}

// writeLine sends a single IRC line (appending \r\n). Safe for concurrent use.
func (c *Connector) writeLine(line string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writer == nil {
		return fmt.Errorf("not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := fmt.Fprintf(c.writer, "%s\r\n", line); err != nil {
		return err
	}
	return c.writer.Flush()
}

// parseTwitchMessage parses a raw Twitch IRC line. It handles the IRCv3 tags
// extension (@key=value;key2=value2 prefix) in addition to the standard IRC
// message format.
//
// Format:
//
//	[ "@" tags SPACE ] [ ":" prefix SPACE ] command { SPACE param } [ " :" trailing ]
func parseTwitchMessage(line string) *twitchMessage {
	msg := &twitchMessage{
		Tags: make(map[string]string),
	}

	// Extract IRCv3 tags.
	if strings.HasPrefix(line, "@") {
		idx := strings.Index(line, " ")
		if idx < 0 {
			return msg
		}
		tagStr := line[1:idx]
		line = line[idx+1:]

		for _, kv := range strings.Split(tagStr, ";") {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				msg.Tags[parts[0]] = parts[1]
			} else if len(parts) == 1 {
				msg.Tags[parts[0]] = ""
			}
		}
	}

	// Extract prefix.
	if strings.HasPrefix(line, ":") {
		idx := strings.Index(line, " ")
		if idx < 0 {
			msg.Prefix = line[1:]
			return msg
		}
		msg.Prefix = line[1:idx]
		line = line[idx+1:]

		if bangIdx := strings.Index(msg.Prefix, "!"); bangIdx >= 0 {
			msg.Nick = msg.Prefix[:bangIdx]
		} else {
			msg.Nick = msg.Prefix
		}
	}

	// Extract trailing.
	if idx := strings.Index(line, " :"); idx >= 0 {
		msg.Trailing = line[idx+2:]
		line = line[:idx]
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return msg
	}

	msg.Command = parts[0]
	if len(parts) > 1 {
		msg.Params = parts[1:]
	}

	return msg
}

// stripTwitchMention removes "@botNick " or "botNick: " prefixes from text.
func stripTwitchMention(text, nick string) string {
	lower := strings.ToLower(text)
	lowerNick := strings.ToLower(nick)

	for _, pat := range []string{
		"@" + lowerNick + " ",
		"@" + lowerNick + ",",
		lowerNick + ": ",
		lowerNick + ", ",
		lowerNick + " ",
	} {
		if strings.HasPrefix(lower, pat) {
			return strings.TrimLeft(text[len(pat):], " ")
		}
	}

	// The mention may appear mid-message; return everything after it.
	for _, pat := range []string{"@" + lowerNick, lowerNick} {
		if idx := strings.Index(lower, pat); idx >= 0 {
			after := strings.TrimLeft(text[idx+len(pat):], ":, ")
			if after != "" {
				return after
			}
		}
	}

	return text
}

// splitLines splits text into chunks of at most maxLen bytes, preferring
// newline and then space boundaries.
func splitLines(text string, maxLen int) []string {
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

		if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
			chunks = append(chunks, strings.TrimRight(text[:idx], "\r\n "))
			text = strings.TrimLeft(text[idx:], "\r\n ")
			continue
		}

		if idx := strings.LastIndex(chunk, " "); idx > 0 {
			chunks = append(chunks, strings.TrimRight(text[:idx], " "))
			text = strings.TrimLeft(text[idx:], " ")
			continue
		}

		chunks = append(chunks, chunk)
		text = text[maxLen:]
	}

	return chunks
}
