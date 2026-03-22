// Package irc provides a connector that bridges IRC channels to the SoulGate
// HTTP API. It uses only stdlib — IRC is a simple line-oriented text protocol
// over TCP, so no external dependencies are needed.
package irc

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
	defaultGatewayURL    = "http://localhost:8080"
	defaultServer        = "irc.libera.chat:6697"
	reconnectDelay       = 5 * time.Second
	pingTimeout          = 3 * time.Minute
	writeTimeout         = 30 * time.Second
	maxIRCMessageLen     = 450 // Conservative limit to stay under the 512-byte RFC limit with prefix overhead
)

// Config holds all IRC connector configuration.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080.
	GatewayURL string

	// Server is the IRC server address in host:port form.
	// Defaults to irc.libera.chat:6697.
	Server string

	// Nick is the bot nickname.
	Nick string

	// Channels is the list of channels to join, e.g. ["#soulgate"].
	Channels []string

	// UseTLS enables TLS. Defaults to true.
	UseTLS bool

	// Password is an optional server password (PASS command).
	Password string
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

// ircMessage represents a parsed IRC protocol message.
type ircMessage struct {
	// Prefix is the :nick!user@host prefix (without the leading colon).
	Prefix string
	// Nick is extracted from Prefix for convenience.
	Nick string
	// Command is the IRC command or numeric reply.
	Command string
	// Params are the positional parameters.
	Params []string
	// Trailing is the final parameter after the colon separator.
	Trailing string
}

// Connector bridges an IRC network to the SoulGate HTTP API.
type Connector struct {
	config     Config
	httpClient *http.Client

	mu       sync.Mutex
	conn     net.Conn
	writer   *bufio.Writer
	stopped  chan struct{}
	stopOnce sync.Once
}

// New creates a new IRC connector, applying defaults where values are absent.
func New(cfg Config) (*Connector, error) {
	if cfg.Nick == "" {
		return nil, fmt.Errorf("irc: Nick is required")
	}
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = defaultGatewayURL
	}
	if cfg.Server == "" {
		cfg.Server = defaultServer
	}
	if !cfg.UseTLS {
		// Explicit false is fine; we only default to true when the field is
		// omitted and the caller constructed the struct with UseTLS unset.
		// Because Go zero-value for bool is false, we cannot distinguish
		// "unset" from "explicitly false" without a pointer.  Callers who
		// want TLS must set UseTLS: true explicitly.
	}

	return &Connector{
		config:     cfg,
		httpClient: &http.Client{Timeout: 0}, // agentic loops can be long
		stopped:    make(chan struct{}),
	}, nil
}

// Start connects to the IRC server and handles messages until ctx is cancelled.
// It automatically reconnects on unexpected disconnections.
func (c *Connector) Start(ctx context.Context) error {
	for {
		if err := c.run(ctx); err != nil {
			// Surface deliberate stops.
			select {
			case <-c.stopped:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			log.Printf("irc: disconnected: %v — reconnecting in %s", err, reconnectDelay)
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
			// Send QUIT so the server knows we're leaving.
			_ = c.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			_, _ = fmt.Fprintf(c.writer, "QUIT :shutdown\r\n")
			_ = c.writer.Flush()
			c.conn.Close()
		}
		c.mu.Unlock()
	})
}

// run performs a single connection attempt: connect, register, join channels,
// and read until the connection closes or ctx is cancelled.
func (c *Connector) run(ctx context.Context) error {
	conn, err := c.dial()
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

	// Registration phase.
	if err := c.register(); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	reader := bufio.NewReader(conn)

	// Wait for MOTD end (376 or 422) before joining channels so we don't race
	// with server-side throttling.
	if err := c.waitForReady(ctx, reader); err != nil {
		return err
	}

	// Join configured channels.
	for _, ch := range c.config.Channels {
		if err := c.writeLine(fmt.Sprintf("JOIN %s", ch)); err != nil {
			return fmt.Errorf("JOIN %s: %w", ch, err)
		}
	}

	log.Printf("irc: connected to %s, joined %v", c.config.Server, c.config.Channels)

	// Main read loop.
	pingTimer := time.NewTimer(pingTimeout)
	defer pingTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopped:
			return nil
		default:
		}

		// Extend read deadline on each iteration; we'll reset it on PONG.
		conn.SetReadDeadline(time.Now().Add(pingTimeout + 30*time.Second))

		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		msg := parseIRCMessage(line)
		c.handleMessage(ctx, msg)

		// Reset the ping timer whenever we receive any server traffic.
		if !pingTimer.Stop() {
			select {
			case <-pingTimer.C:
			default:
			}
		}
		pingTimer.Reset(pingTimeout)
	}
}

// dial opens a TCP (or TLS) connection to the configured server.
func (c *Connector) dial() (net.Conn, error) {
	if c.config.UseTLS {
		host, _, _ := net.SplitHostPort(c.config.Server)
		return tls.Dial("tcp", c.config.Server, &tls.Config{
			ServerName: host,
		})
	}
	return net.DialTimeout("tcp", c.config.Server, 30*time.Second)
}

// register sends PASS (if configured), NICK, and USER to the server.
func (c *Connector) register() error {
	if c.config.Password != "" {
		if err := c.writeLine(fmt.Sprintf("PASS %s", c.config.Password)); err != nil {
			return err
		}
	}
	if err := c.writeLine(fmt.Sprintf("NICK %s", c.config.Nick)); err != nil {
		return err
	}
	return c.writeLine(fmt.Sprintf("USER %s 0 * :SoulGate IRC Bot", c.config.Nick))
}

// waitForReady reads server messages until we see the end-of-MOTD numeric
// (376) or the no-MOTD numeric (422), handling PING during the wait.
func (c *Connector) waitForReady(ctx context.Context, r *bufio.Reader) error {
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
			return fmt.Errorf("read during registration: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")
		msg := parseIRCMessage(line)

		// Keep the server happy during registration.
		if msg.Command == "PING" {
			_ = c.writeLine("PONG :" + msg.Trailing)
		}

		// 376 = end of MOTD, 422 = no MOTD (both signal "ready").
		if msg.Command == "376" || msg.Command == "422" {
			return nil
		}

		// 433 = nick already in use — append underscore and retry.
		if msg.Command == "433" {
			c.config.Nick += "_"
			if err := c.writeLine(fmt.Sprintf("NICK %s", c.config.Nick)); err != nil {
				return err
			}
		}
	}
}

// handleMessage dispatches a parsed IRC message to the appropriate handler.
func (c *Connector) handleMessage(ctx context.Context, msg *ircMessage) {
	switch msg.Command {
	case "PING":
		_ = c.writeLine("PONG :" + msg.Trailing)

	case "PRIVMSG":
		c.handlePrivmsg(ctx, msg)
	}
}

// handlePrivmsg processes a PRIVMSG and determines whether to respond.
//
// The bot responds when:
//   - The target is a channel and the message contains the bot nick (mention).
//   - The target is the bot nick itself (direct message / query).
func (c *Connector) handlePrivmsg(ctx context.Context, msg *ircMessage) {
	if len(msg.Params) == 0 {
		return
	}

	target := msg.Params[0]
	text := msg.Trailing
	nick := msg.Nick
	botNick := c.config.Nick

	isChannel := strings.HasPrefix(target, "#") || strings.HasPrefix(target, "&")
	isDM := strings.EqualFold(target, botNick)

	if isChannel {
		// Only respond when explicitly mentioned.
		lower := strings.ToLower(text)
		lowerNick := strings.ToLower(botNick)
		if !strings.Contains(lower, lowerNick) {
			return
		}
		// Strip the mention prefix "botnick: " or "botnick, " from the text.
		text = stripNickPrefix(text, botNick)
	} else if !isDM {
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	// Determine where to send the reply: channel for channel msgs, nick for DMs.
	replyTo := target
	if isDM {
		replyTo = nick
	}

	log.Printf("irc: message from %s in %s: %q", nick, target, text)

	// Call the gateway asynchronously so we don't block the read loop.
	go func() {
		response, err := c.sendToGateway(text)
		if err != nil {
			log.Printf("irc: gateway error: %v", err)
			response = fmt.Sprintf("Error: %v", err)
		}

		c.sendResponse(replyTo, nick, response, isChannel)
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

// sendResponse delivers a response to replyTo, splitting across multiple
// PRIVMSG lines if necessary to stay within the IRC message length limit.
// For channel replies it prefixes the first line with "nick: ".
func (c *Connector) sendResponse(replyTo, fromNick, text string, isChannel bool) {
	lines := splitLines(text, maxIRCMessageLen)
	for i, line := range lines {
		var payload string
		if isChannel && i == 0 {
			payload = fmt.Sprintf("%s: %s", fromNick, line)
		} else {
			payload = line
		}

		if err := c.writeLine(fmt.Sprintf("PRIVMSG %s :%s", replyTo, payload)); err != nil {
			log.Printf("irc: failed to send PRIVMSG: %v", err)
			return
		}
	}
}

// writeLine sends a single IRC protocol line (appending \r\n) to the server.
// It is safe to call from multiple goroutines.
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

// parseIRCMessage parses a raw IRC line into an ircMessage.
//
// IRC message format (RFC 1459):
//
//	[ ":" prefix SPACE ] command { SPACE param } [ SPACE ":" trailing ] CRLF
func parseIRCMessage(line string) *ircMessage {
	msg := &ircMessage{}

	// Extract prefix.
	if strings.HasPrefix(line, ":") {
		idx := strings.Index(line, " ")
		if idx < 0 {
			msg.Prefix = line[1:]
			return msg
		}
		msg.Prefix = line[1:idx]
		line = line[idx+1:]

		// Extract nick from prefix (nick!user@host).
		if bangIdx := strings.Index(msg.Prefix, "!"); bangIdx >= 0 {
			msg.Nick = msg.Prefix[:bangIdx]
		} else {
			msg.Nick = msg.Prefix
		}
	}

	// Split into command + params.
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

// stripNickPrefix removes a leading "nick: " or "nick, " mention from text.
func stripNickPrefix(text, nick string) string {
	lower := strings.ToLower(text)
	lowerNick := strings.ToLower(nick)

	for _, sep := range []string{": ", ", ", " "} {
		prefix := lowerNick + sep
		if strings.HasPrefix(lower, prefix) {
			return text[len(prefix):]
		}
	}

	return text
}

// splitLines splits text into chunks no longer than maxLen, preferring line
// and then word boundaries.
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

		// Prefer to break on a newline.
		if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
			chunks = append(chunks, strings.TrimRight(text[:idx], "\r\n "))
			text = strings.TrimLeft(text[idx:], "\r\n ")
			continue
		}

		// Fall back to a word boundary.
		if idx := strings.LastIndex(chunk, " "); idx > 0 {
			chunks = append(chunks, strings.TrimRight(text[:idx], " "))
			text = strings.TrimLeft(text[idx:], " ")
			continue
		}

		// Hard split.
		chunks = append(chunks, chunk)
		text = text[maxLen:]
	}

	return chunks
}
