// Package whatsapp provides a WhatsApp connector for SoulGate.
//
// It uses the go.mau.fi/whatsmeow library (WhatsApp Web multi-device API) and
// forwards messages to the SoulGate HTTP /api/chat endpoint, mirroring the
// pattern established by the Discord connector.
//
// Session data is stored in a SQLite database (via modernc.org/sqlite, pure Go)
// so that re-pairing is not needed after a restart.
//
// First-time setup: the connector prints a QR code to the terminal.  The user
// scans it with WhatsApp on their phone (Linked Devices → Link a Device).
// Subsequent starts reconnect automatically using the stored session.
//
// The bot responds to:
//   - Direct messages (any message sent 1-1 to the linked number)
//   - Group messages that @mention the bot's number
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	// modernc.org/sqlite registers the "sqlite" database/sql driver (pure Go,
	// no CGO).  The whatsmeow sqlstore accepts any dialect whose name starts
	// with "sqlite", so this is compatible without mattn/go-sqlite3.
	_ "modernc.org/sqlite"
)

const (
	defaultGatewayURL = "http://localhost:8080"
	defaultDataDir    = ".soulgate/whatsapp"
)

// Config holds all configuration for the WhatsApp connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// DataDir is the directory used for session storage (SQLite database).
	// Defaults to .soulgate/whatsapp when empty.
	DataDir string
}

// gatewayRequest is the JSON body sent to POST /api/chat.
type gatewayRequest struct {
	Message string `json:"message"`
}

// gatewayResponse is the JSON body returned from POST /api/chat.
type gatewayResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// Connector bridges WhatsApp to the SoulGate HTTP API.
type Connector struct {
	config     Config
	client     *whatsmeow.Client
	db         *sqlstore.Container
	httpClient *http.Client
	stopOnce   sync.Once
	stopped    chan struct{}
}

// New creates a new Connector: initialises the SQLite session store and the
// whatsmeow client, but does not yet connect to WhatsApp.
func New(config Config) (*Connector, error) {
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}
	if config.DataDir == "" {
		config.DataDir = defaultDataDir
	}

	if err := os.MkdirAll(config.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("whatsapp: failed to create data directory %q: %w", config.DataDir, err)
	}

	dbPath := config.DataDir + "/session.db"
	// Use the modernc "sqlite" driver.  The DSN uses the file: URI scheme so
	// that pragmas can be appended; foreign keys are enabled as recommended by
	// the whatsmeow documentation.
	dsn := "file:" + dbPath + "?_foreign_keys=on"

	dbLog := waLog.Stdout("DB", "WARN", true)
	container, err := sqlstore.New(context.Background(), "sqlite", dsn, dbLog)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: failed to open session database: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("whatsapp: failed to get device from store: %w", err)
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	waClient := whatsmeow.NewClient(deviceStore, clientLog)

	c := &Connector{
		config:     config,
		client:     waClient,
		db:         container,
		httpClient: &http.Client{Timeout: 0}, // no timeout — agentic loops can be long
		stopped:    make(chan struct{}),
	}

	waClient.AddEventHandler(c.handleEvent)

	return c, nil
}

// Start connects to WhatsApp.  On the first run (no stored session) it
// displays a QR code in the terminal for the user to scan.  On subsequent
// runs it reconnects automatically using the stored session.
//
// Start blocks until ctx is cancelled or Stop is called.
func (c *Connector) Start(ctx context.Context) error {
	if c.client.Store.ID == nil {
		// No session stored — begin QR-code pairing flow.
		qrChan, err := c.client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("whatsapp: failed to get QR channel: %w", err)
		}

		if err := c.client.Connect(); err != nil {
			return fmt.Errorf("whatsapp: failed to connect for QR pairing: %w", err)
		}

		fmt.Println("WhatsApp connector: scan the QR code below with WhatsApp on your phone.")
		fmt.Println("Open WhatsApp > Linked Devices > Link a Device.")
		fmt.Println()

		for item := range qrChan {
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				// Render the QR code using half-block Unicode characters so it
				// fits in a standard terminal without extra tooling.
				qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
				fmt.Printf("\n(QR code refreshes in %s)\n\n", item.Timeout.Round(1e9))

			case whatsmeow.QRChannelSuccess.Event:
				fmt.Println("WhatsApp connector: pairing successful — session saved.")

			case whatsmeow.QRChannelTimeout.Event:
				return fmt.Errorf("whatsapp: QR code timed out before scanning")

			case whatsmeow.QRChannelEventError:
				return fmt.Errorf("whatsapp: pairing error: %w", item.Error)

			default:
				log.Printf("whatsapp: QR channel event: %s", item.Event)
			}
		}
	} else {
		// Session exists — reconnect without user interaction.
		if err := c.client.Connect(); err != nil {
			return fmt.Errorf("whatsapp: failed to connect: %w", err)
		}
		fmt.Printf("WhatsApp connector: reconnected as %s\n", c.client.Store.ID)
	}

	log.Printf("whatsapp: listening for messages (gateway: %s)", c.config.GatewayURL)

	// Block until the context is cancelled or Stop is called.
	select {
	case <-ctx.Done():
		return c.Stop()
	case <-c.stopped:
		return nil
	}
}

// Stop disconnects from WhatsApp gracefully.  It is safe to call more than once.
func (c *Connector) Stop() error {
	var disconnectErr error
	c.stopOnce.Do(func() {
		log.Printf("whatsapp: stopping connector")
		c.client.Disconnect()
		close(c.stopped)
	})
	return disconnectErr
}

// handleEvent is the whatsmeow event handler registered in New.
// All events arrive here; we only act on *events.Message.
func (c *Connector) handleEvent(evt any) {
	msg, ok := evt.(*events.Message)
	if !ok {
		return
	}
	c.handleMessage(msg)
}

// handleMessage decides whether to respond to an incoming WhatsApp message
// and, if so, dispatches the work to a goroutine so the event loop is not
// blocked during the (potentially slow) HTTP call to the gateway.
func (c *Connector) handleMessage(evt *events.Message) {
	// Ignore messages sent by ourselves (echo of our own replies).
	if evt.Info.IsFromMe {
		return
	}

	// Extract the plain text from the message proto.  WhatsApp wraps text in
	// either the Conversation field (simple messages) or ExtendedTextMessage
	// (messages with previews / formatted text).
	text := extractText(evt.Message)
	if text == "" {
		return // not a text message (image, audio, sticker, etc.)
	}

	isDM := !evt.Info.IsGroup
	isMentioned := c.isMentioned(evt)

	if !isDM && !isMentioned {
		// In groups, only respond when explicitly mentioned.
		return
	}

	// Strip the mention tag from the message before forwarding to the gateway.
	if isMentioned {
		text = c.stripMention(text)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	senderPhone := evt.Info.Sender.User
	chatJID := evt.Info.Chat

	log.Printf("whatsapp: message from %s in %s: %q", senderPhone, chatJID, text)

	go func() {
		response, err := c.sendToGateway(text)
		if err != nil {
			log.Printf("whatsapp: gateway error: %v", err)
			response = fmt.Sprintf("SoulGate error: %v", err)
		}

		if sendErr := c.sendReply(chatJID, response); sendErr != nil {
			log.Printf("whatsapp: failed to send reply to %s: %v", chatJID, sendErr)
		}
	}()
}

// sendToGateway POSTs the message text to the SoulGate /api/chat endpoint and
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

	log.Printf("whatsapp: gateway responded with %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendReply sends a text message back to the given WhatsApp chat JID.
// WhatsApp does not have a hard message-length limit in the same way Discord
// does, but we cap each chunk at 4096 characters to stay within safe bounds.
func (c *Connector) sendReply(chat types.JID, text string) error {
	const maxChunk = 4096
	chunks := splitMessage(text, maxChunk)
	for _, chunk := range chunks {
		msg := &waE2E.Message{
			Conversation: &chunk,
		}
		if _, err := c.client.SendMessage(context.Background(), chat, msg); err != nil {
			return fmt.Errorf("SendMessage to %s: %w", chat, err)
		}
	}
	return nil
}

// isMentioned reports whether our own JID is referenced in the message's
// ContextInfo.MentionedJID list (WhatsApp's mechanism for @mentions).
func (c *Connector) isMentioned(evt *events.Message) bool {
	if c.client.Store.ID == nil {
		return false
	}
	ownUser := c.client.Store.ID.User // phone number string, e.g. "15551234567"

	mentions := extractMentions(evt.Message)
	for _, jid := range mentions {
		if jid == ownUser || strings.HasPrefix(jid, ownUser+"@") {
			return true
		}
	}
	return false
}

// stripMention removes @<phone> patterns from a message so the model receives
// clean input.  WhatsApp encodes mentions as the raw phone number in the
// ContextInfo; the visible text usually contains "@<phone>" literally.
func (c *Connector) stripMention(text string) string {
	if c.client.Store.ID == nil {
		return text
	}
	own := c.client.Store.ID.User
	// Remove the @ prefix variant that WhatsApp renders in the message body.
	text = strings.ReplaceAll(text, "@"+own, "")
	return text
}

// extractText returns the plain text body of a WhatsApp message proto, or the
// empty string when the message carries no readable text.
func extractText(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	// Simple text message.
	if c := msg.GetConversation(); c != "" {
		return c
	}
	// Text message with link preview or formatting.
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if t := ext.GetText(); t != "" {
			return t
		}
	}
	// Image / video / document with a caption.
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetCaption()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetCaption()
	}
	return ""
}

// extractMentions returns the list of phone-number strings that appear in the
// message's ContextInfo MentionedJID field.
func extractMentions(msg *waE2E.Message) []string {
	if msg == nil {
		return nil
	}
	// ContextInfo is present on ExtendedTextMessage and several media types.
	var ctx *waE2E.ContextInfo
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		ctx = ext.GetContextInfo()
	}
	if ctx == nil {
		return nil
	}
	return ctx.GetMentionedJID()
}

// splitMessage divides text into chunks of at most maxLen bytes, preferring to
// break on newlines or spaces to avoid cutting words mid-way.
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
		idx := strings.LastIndex(chunk, "\n")
		if idx <= 0 {
			idx = strings.LastIndex(chunk, " ")
		}
		if idx <= 0 {
			idx = maxLen
		}

		chunks = append(chunks, strings.TrimRight(text[:idx], " "))
		text = strings.TrimLeft(text[idx:], " \n")
	}
	return chunks
}
