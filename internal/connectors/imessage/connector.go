// Package imessage provides an iMessage connector for SoulGate on macOS.
//
// It bridges Apple's iMessage to the SoulGate HTTP /api/chat endpoint by:
//  1. Polling the iMessage SQLite database (~/Library/Messages/chat.db) for new
//     incoming messages.
//  2. Forwarding each new message text to the SoulGate HTTP /api/chat endpoint.
//  3. Sending the AI response back to the original sender via AppleScript.
//
// # macOS permissions required
//
// The process (typically your terminal app) must have Full Disk Access enabled
// in System Settings > Privacy & Security > Full Disk Access, or the SQLite
// open will fail with "operation not permitted".
//
// Additionally, the first time a reply is sent the system will prompt for
// Automation access: System Settings > Privacy & Security > Automation, enable
// the toggle for your terminal app under "Messages".
//
// # Limitations
//
//   - macOS only — a runtime check returns [ErrNotMacOS] on other platforms.
//   - chat.db is opened read-only; the connector never modifies it.
//   - Replies are sent via osascript which spawns a subprocess per message.
//   - The Messages app must be running (or at least launchable) for osascript
//     to deliver messages.
package imessage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	// modernc.org/sqlite is a pure-Go SQLite driver — no CGO required and
	// already a project dependency.
	_ "modernc.org/sqlite"
)

// ErrNotMacOS is returned by New when the connector is instantiated on a
// non-macOS platform.
var ErrNotMacOS = errors.New("imessage: connector is only supported on macOS")

const (
	defaultGatewayURL    = "http://localhost:8080"
	defaultPollInterval  = 2 * time.Second
	defaultChatDBPath    = "~/Library/Messages/chat.db"
	defaultMaxMessageLen = 2000 // split responses into chunks of this size
)

// Config holds all configuration for the iMessage connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// PollInterval is how often chat.db is queried for new messages.
	// Defaults to 2 seconds when zero.
	PollInterval time.Duration

	// ChatDBPath is the path to the iMessage SQLite database.
	// The tilde (~) prefix is expanded to the current user's home directory.
	// Defaults to ~/Library/Messages/chat.db when empty.
	ChatDBPath string
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

// iMessage is a parsed row from the message / handle join.
type iMessage struct {
	rowID    int64
	text     string
	phone    string // sender's phone number / Apple ID from handle.id
	date     int64
}

// Connector polls the iMessage SQLite database for incoming messages and
// delivers AI responses via AppleScript.
type Connector struct {
	config     Config
	db         *sql.DB
	httpClient *http.Client

	// lastRowID is the highest message ROWID already processed.
	// Guarded by mu to allow safe concurrent access from tests.
	lastRowID int64
	mu        sync.Mutex

	stopOnce sync.Once
	stopped  chan struct{}
}

// New creates and returns a new iMessage Connector.
//
// It validates the configuration, expands the ChatDBPath tilde, and opens the
// iMessage database in read-only mode.  On non-macOS platforms it returns
// [ErrNotMacOS] immediately without touching the filesystem.
func New(config Config) (*Connector, error) {
	if runtime.GOOS != "darwin" {
		return nil, ErrNotMacOS
	}

	// Apply defaults.
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}
	if config.PollInterval == 0 {
		config.PollInterval = defaultPollInterval
	}
	if config.ChatDBPath == "" {
		config.ChatDBPath = defaultChatDBPath
	}

	// Expand leading ~ to the user's home directory.
	config.ChatDBPath = expandTilde(config.ChatDBPath)

	// Open chat.db in read-only mode so we can never corrupt the live
	// iMessage database.  The WAL journal mode used by Messages is compatible
	// with concurrent read-only opens.
	dsn := fmt.Sprintf("file:%s?mode=ro&_journal_mode=WAL&immutable=1", config.ChatDBPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("imessage: failed to open chat.db at %s: %w\n"+
			"Ensure your terminal app has Full Disk Access in\n"+
			"System Settings > Privacy & Security > Full Disk Access", config.ChatDBPath, err)
	}

	// Verify we can actually read from the database before returning.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("imessage: cannot read chat.db: %w\n"+
			"Ensure your terminal app has Full Disk Access in\n"+
			"System Settings > Privacy & Security > Full Disk Access", err)
	}

	// Seed lastRowID to the current maximum so we only process messages that
	// arrive after the connector starts (i.e. we do not replay history).
	lastRowID, err := queryMaxRowID(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("imessage: failed to determine current message high-water mark: %w", err)
	}

	return &Connector{
		config:     config,
		db:         db,
		httpClient: &http.Client{Timeout: 0}, // no timeout — agentic loops can be long
		lastRowID:  lastRowID,
		stopped:    make(chan struct{}),
	}, nil
}

// Start begins polling chat.db for new incoming messages and blocks until ctx
// is cancelled or Stop is called.
func (c *Connector) Start(ctx context.Context) error {
	log.Printf("imessage: connector started (db=%s, poll=%s, gateway=%s)",
		c.config.ChatDBPath, c.config.PollInterval, c.config.GatewayURL)

	ticker := time.NewTicker(c.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return c.Stop()
		case <-c.stopped:
			return nil
		case <-ticker.C:
			c.pollMessages()
		}
	}
}

// Stop shuts the connector down.  It is safe to call more than once.
func (c *Connector) Stop() error {
	var closeErr error
	c.stopOnce.Do(func() {
		log.Printf("imessage: stopping connector")
		closeErr = c.db.Close()
		close(c.stopped)
	})
	return closeErr
}

// pollMessages queries chat.db for messages with ROWID greater than the last
// processed one and dispatches each in its own goroutine so the poll loop is
// never blocked by slow gateway calls.
func (c *Connector) pollMessages() {
	c.mu.Lock()
	since := c.lastRowID
	c.mu.Unlock()

	messages, err := c.queryNewMessages(since)
	if err != nil {
		log.Printf("imessage: failed to query new messages: %v", err)
		return
	}

	for _, msg := range messages {
		// Update the high-water mark before dispatching so that even if
		// processMessage panics we do not re-deliver the same message.
		c.mu.Lock()
		if msg.rowID > c.lastRowID {
			c.lastRowID = msg.rowID
		}
		c.mu.Unlock()

		go c.processMessage(msg)
	}
}

// queryMaxRowID returns the current maximum message ROWID in chat.db, or 0 if
// the table is empty.
func queryMaxRowID(db *sql.DB) (int64, error) {
	var rowID sql.NullInt64
	err := db.QueryRow(`SELECT MAX(ROWID) FROM message`).Scan(&rowID)
	if err != nil {
		return 0, fmt.Errorf("SELECT MAX(ROWID): %w", err)
	}
	if !rowID.Valid {
		return 0, nil // empty database
	}
	return rowID.Int64, nil
}

// queryNewMessages returns incoming (is_from_me = 0) messages whose ROWID is
// strictly greater than since, ordered oldest-first.
func (c *Connector) queryNewMessages(since int64) ([]iMessage, error) {
	const q = `
		SELECT
			m.ROWID,
			COALESCE(m.text, ''),
			m.date,
			h.id AS phone
		FROM message m
		JOIN handle h ON m.handle_id = h.ROWID
		WHERE
			m.ROWID > ?
			AND m.is_from_me = 0
			AND m.text IS NOT NULL
			AND m.text != ''
		ORDER BY m.ROWID ASC`

	rows, err := c.db.Query(q, since)
	if err != nil {
		return nil, fmt.Errorf("query new messages: %w", err)
	}
	defer rows.Close()

	var messages []iMessage
	for rows.Next() {
		var msg iMessage
		if err := rows.Scan(&msg.rowID, &msg.text, &msg.date, &msg.phone); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// processMessage forwards the message to the gateway and sends the reply back
// via AppleScript.  It logs errors instead of returning them so that one bad
// message does not stop the poll loop.
func (c *Connector) processMessage(msg iMessage) {
	text := strings.TrimSpace(msg.text)
	if text == "" {
		return
	}

	log.Printf("imessage: incoming from %s (ROWID=%d): %q", msg.phone, msg.rowID, text)

	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("imessage: gateway error for message %d: %v", msg.rowID, err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	chunks := splitMessage(response, defaultMaxMessageLen)
	for i, chunk := range chunks {
		if err := c.sendReply(msg.phone, chunk); err != nil {
			log.Printf("imessage: failed to send reply chunk %d/%d to %s: %v",
				i+1, len(chunks), msg.phone, err)
		}
	}
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

	log.Printf("imessage: gateway responded with %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendReply sends a text message to phone via AppleScript.
//
// The osascript command used is:
//
//	tell application "Messages"
//	    send "<escaped text>" to buddy "<phone>" of (1st account whose service type = iMessage)
//	end tell
//
// Requirements:
//   - The Messages app must be running or launchable.
//   - The process must have Automation access for Messages in
//     System Settings > Privacy & Security > Automation.
func (c *Connector) sendReply(phone, message string) error {
	escaped := escapeAppleScript(message)
	script := fmt.Sprintf(
		`tell application "Messages" to send "%s" to buddy "%s" of (1st account whose service type = iMessage)`,
		escaped, phone,
	)

	cmd := exec.Command("osascript", "-e", script)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return fmt.Errorf("osascript failed: %w: %s", err, stderrStr)
		}
		return fmt.Errorf("osascript failed: %w", err)
	}

	log.Printf("imessage: reply sent to %s (%d chars)", phone, len(message))
	return nil
}

// escapeAppleScript escapes a string for embedding inside AppleScript double
// quotes.  It escapes backslashes first (so subsequent replacements do not
// double-escape), then double quotes.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// expandTilde replaces a leading ~ with the value of $HOME, falling back to
// os.UserHomeDir if $HOME is not set.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home := os.Getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			// If we cannot determine the home directory, return the path as-is
			// and let the SQL open surface a useful error.
			return path
		}
	}

	return home + path[1:]
}

// splitMessage divides text into chunks of at most maxLen runes, preferring to
// break on newline or space boundaries to avoid splitting mid-word.
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

		// Prefer to break on the last newline within the chunk.
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
