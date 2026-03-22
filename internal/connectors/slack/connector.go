// Package slack provides a Slack Socket Mode connector for SoulGate.
//
// It bridges Slack messages (DMs, app mentions, and channel messages) to the
// SoulGate HTTP API at /api/chat, forwarding responses back as threaded replies.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Config holds the configuration for the Slack connector.
type Config struct {
	// BotToken is the xoxb-... OAuth bot token.
	BotToken string

	// AppToken is the xapp-... app-level token required for Socket Mode.
	AppToken string

	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string
}

// Connector bridges Slack to the SoulGate HTTP API via Socket Mode.
type Connector struct {
	config Config
	api    *slack.Client
	socket *socketmode.Client

	// httpClient is used for gateway requests. Zero timeout allows long agentic loops.
	httpClient *http.Client

	// stopCh is closed when Stop() is called to signal the event loop to exit.
	stopCh chan struct{}
}

// gatewayRequest matches the /api/chat request body expected by the SoulGate API server.
type gatewayRequest struct {
	Message string `json:"message"`
}

// gatewayResponse matches the /api/chat response body returned by the SoulGate API server.
type gatewayResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// New creates a new Slack connector. It validates the config but does not
// open any network connections — call Start to connect via Socket Mode.
func New(config Config) (*Connector, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("slack: BotToken (xoxb-...) is required")
	}
	if config.AppToken == "" {
		return nil, fmt.Errorf("slack: AppToken (xapp-...) is required for Socket Mode")
	}
	if !strings.HasPrefix(config.AppToken, "xapp-") {
		return nil, fmt.Errorf("slack: AppToken must start with xapp- (got %q)", config.AppToken[:min(len(config.AppToken), 10)])
	}
	if config.GatewayURL == "" {
		config.GatewayURL = "http://localhost:8080"
	}

	api := slack.New(
		config.BotToken,
		slack.OptionAppLevelToken(config.AppToken),
	)

	socket := socketmode.New(api)

	return &Connector{
		config:     config,
		api:        api,
		socket:     socket,
		httpClient: &http.Client{Timeout: 0}, // no timeout — agentic loops can take minutes
		stopCh:     make(chan struct{}),
	}, nil
}

// Start connects to Slack via Socket Mode and blocks until ctx is cancelled or
// Stop is called. It spawns the event loop in a goroutine and then runs the
// Socket Mode client (which owns the WebSocket connection) in the current
// goroutine so that the socket can be cleanly shut down on return.
func (c *Connector) Start(ctx context.Context) error {
	log.Printf("slack connector: connecting to Slack via Socket Mode")
	log.Printf("slack connector: gateway URL: %s", c.config.GatewayURL)

	// Dispatch events concurrently while the Socket Mode client runs.
	go c.runEventLoop(ctx)

	// Run blocks until the Socket Mode connection is closed.
	if err := c.socket.RunContext(ctx); err != nil {
		return fmt.Errorf("slack connector: socket mode error: %w", err)
	}
	return nil
}

// Stop signals the connector to shut down gracefully.
func (c *Connector) Stop() error {
	select {
	case <-c.stopCh:
		// Already stopped.
	default:
		close(c.stopCh)
	}
	return nil
}

// runEventLoop reads events from the socket mode client and dispatches them.
// It exits when ctx is cancelled or stopCh is closed.
func (c *Connector) runEventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case evt, ok := <-c.socket.Events:
			if !ok {
				return
			}
			c.handleEvent(evt)
		}
	}
}

// handleEvent dispatches a Socket Mode event to the appropriate handler.
func (c *Connector) handleEvent(evt socketmode.Event) {
	switch evt.Type {
	case socketmode.EventTypeConnected:
		log.Printf("slack connector: connected to Slack via Socket Mode")

	case socketmode.EventTypeConnectionError:
		log.Printf("slack connector: connection error: %v", evt.Data)

	case socketmode.EventTypeInvalidAuth:
		log.Printf("slack connector: invalid auth — check BotToken and AppToken")

	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
		if !ok {
			log.Printf("slack connector: unexpected EventsAPI data type: %T", evt.Data)
			return
		}

		// Acknowledge receipt immediately — Slack requires a 3-second acknowledgement.
		c.socket.Ack(*evt.Request)

		if eventsAPIEvent.Type != slackevents.CallbackEvent {
			return
		}

		// Route based on inner event type.
		switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			c.handleMessage(ev)
		case *slackevents.AppMentionEvent:
			c.handleAppMention(ev)
		}

	default:
		// Ignore other event types (hello, slash commands, interactive, etc.)
	}
}

// handleMessage processes an Events API message event.
// It responds to DMs and channel messages where the bot has been invited,
// skipping bot messages and message-changed/deleted sub-events.
func (c *Connector) handleMessage(ev *slackevents.MessageEvent) {
	// Ignore messages from bots (including ourselves) to prevent loops.
	if ev.BotID != "" {
		return
	}
	// Ignore message subtypes we do not handle (message_changed, message_deleted,
	// bot_message, etc.). Only handle plain new messages.
	if ev.SubType != "" {
		return
	}
	if ev.Text == "" {
		return
	}

	// Only respond to DMs ("im") or channel/group messages.
	// ChannelType "im" = direct message, "channel" = public, "group" = private.
	switch ev.ChannelType {
	case "im", "channel", "group", "mpim":
		// Accepted.
	default:
		return
	}

	log.Printf("slack connector: message from user=%s channel=%s: %s", ev.User, ev.Channel, ev.Text)

	text := cleanMentions(ev.Text)

	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("slack connector: gateway error for message: %v", err)
		_ = c.sendResponse(ev.Channel, ev.ThreadTimeStamp, fmt.Sprintf("Cannot reach SoulGate API: %v", err))
		return
	}

	if err := c.sendResponse(ev.Channel, ev.ThreadTimeStamp, response); err != nil {
		log.Printf("slack connector: failed to send response: %v", err)
	}
}

// handleAppMention processes an Events API app_mention event.
// App mentions arrive when a user types @bot anywhere they can see the bot.
func (c *Connector) handleAppMention(ev *slackevents.AppMentionEvent) {
	// Ignore mentions from bots.
	if ev.BotID != "" {
		return
	}
	if ev.Text == "" {
		return
	}

	log.Printf("slack connector: app mention from user=%s channel=%s: %s", ev.User, ev.Channel, ev.Text)

	// Strip the @mention prefix so the model sees clean input.
	text := cleanMentions(ev.Text)

	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("slack connector: gateway error for mention: %v", err)
		_ = c.sendResponse(ev.Channel, ev.ThreadTimeStamp, fmt.Sprintf("Cannot reach SoulGate API: %v", err))
		return
	}

	if err := c.sendResponse(ev.Channel, ev.ThreadTimeStamp, response); err != nil {
		log.Printf("slack connector: failed to send mention response: %v", err)
	}
}

// sendToGateway POSTs the message to the SoulGate /api/chat endpoint and
// returns the response text. The HTTP client has no timeout because agentic
// loops involving tool use can take several minutes to complete.
func (c *Connector) sendToGateway(message string) (string, error) {
	body, err := json.Marshal(gatewayRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.config.GatewayURL+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("POST /api/chat: %w", err)
	}
	defer resp.Body.Close()

	var gwResp gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gwResp); err != nil {
		return "", fmt.Errorf("decode response (HTTP %d): %w", resp.StatusCode, err)
	}

	if gwResp.Error != "" {
		return "", fmt.Errorf("gateway error: %s", gwResp.Error)
	}
	if gwResp.Response == "" {
		return "", fmt.Errorf("gateway returned an empty response (HTTP %d)", resp.StatusCode)
	}

	log.Printf("slack connector: gateway response: %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendResponse sends a reply to a Slack channel. If threadTS is non-empty the
// reply is posted inside that thread. Responses are formatted with Block Kit:
// a mrkdwn section block for rich rendering, with the plain-text fallback for
// notifications and accessibility.
func (c *Connector) sendResponse(channelID, threadTS, response string) error {
	blocks := formatBlocks(response)

	opts := []slack.MsgOption{
		slack.MsgOptionText(response, false),   // fallback plain text for notifications
		slack.MsgOptionBlocks(blocks...),        // rich Block Kit rendering
		slack.MsgOptionDisableLinkUnfurl(),
	}

	if threadTS != "" {
		opts = append(opts, slack.MsgOptionTS(threadTS))
	}

	_, _, err := c.api.PostMessage(channelID, opts...)
	if err != nil {
		return fmt.Errorf("PostMessage to channel=%s thread=%s: %w", channelID, threadTS, err)
	}
	return nil
}

// formatBlocks converts a plain-text response into a Slack Block Kit layout.
// Long responses are split into multiple section blocks to stay within Slack's
// 3000-character-per-block limit.
func formatBlocks(response string) []slack.Block {
	const maxBlockLen = 3000

	if len(response) <= maxBlockLen {
		txt := slack.NewTextBlockObject(slack.MarkdownType, response, false, false)
		return []slack.Block{slack.NewSectionBlock(txt, nil, nil)}
	}

	// Split into chunks on paragraph boundaries where possible.
	var blocks []slack.Block
	remaining := response

	for len(remaining) > 0 {
		chunk := remaining
		if len(chunk) > maxBlockLen {
			// Try to break at a paragraph boundary within the limit.
			breakAt := strings.LastIndex(remaining[:maxBlockLen], "\n\n")
			if breakAt <= 0 {
				// Fall back to a newline boundary.
				breakAt = strings.LastIndex(remaining[:maxBlockLen], "\n")
			}
			if breakAt <= 0 {
				// Hard split at the character limit.
				breakAt = maxBlockLen
			}
			chunk = remaining[:breakAt]
			remaining = strings.TrimSpace(remaining[breakAt:])
		} else {
			remaining = ""
		}

		txt := slack.NewTextBlockObject(slack.MarkdownType, chunk, false, false)
		blocks = append(blocks, slack.NewSectionBlock(txt, nil, nil))
	}

	return blocks
}

// cleanMentions removes Slack user mention tokens (e.g. "<@U12345>") from the
// start and throughout the message text so the model receives clean input.
var mentionRE = regexp.MustCompile(`<@[A-Z0-9]+>`)

func cleanMentions(text string) string {
	cleaned := mentionRE.ReplaceAllString(text, "")
	return strings.TrimSpace(cleaned)
}

// min returns the smaller of two ints. Used in error formatting above.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
