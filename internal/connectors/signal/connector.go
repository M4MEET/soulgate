// Package signal provides a Signal messenger connector for SoulGate.
//
// It bridges Signal to the SoulGate HTTP /api/chat endpoint by spawning
// signal-cli as a subprocess and communicating over its JSON-RPC interface
// (stdin/stdout).
//
// The connector responds to:
//   - Direct messages (any 1-1 message sent to the registered number)
//   - Group messages where the bot's phone number is explicitly @mentioned
//
// signal-cli must be installed and the phone number must already be registered.
// Run "signal-cli -u +1234567890 register" and verify before using this
// connector.
//
// JSON-RPC protocol (signal-cli jsonRpc mode):
//
//	Incoming (stdout): {"jsonrpc":"2.0","method":"receive","params":{"envelope":{...}}}
//	Outgoing (stdin):  {"jsonrpc":"2.0","id":"1","method":"send","params":{...}}
package signal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	defaultGatewayURL = "http://localhost:8080"
	defaultSignalCLI  = "signal-cli"
)

// Config holds all configuration for the Signal connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// PhoneNumber is the Signal phone number this bot is registered as,
	// in E.164 format (e.g. "+15551234567"). Required.
	PhoneNumber string

	// SignalCLI is the path to the signal-cli binary.
	// Defaults to "signal-cli" (resolved via PATH) when empty.
	SignalCLI string
}

// rpcRequest is a JSON-RPC 2.0 request written to signal-cli's stdin.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// sendParams is the params object for the "send" RPC method (DM).
type sendParams struct {
	Recipient []string `json:"recipient"`
	Message   string   `json:"message"`
}

// sendGroupParams is the params object for the "send" RPC method (group).
type sendGroupParams struct {
	GroupID string `json:"groupId"`
	Message string `json:"message"`
}

// rpcNotification is a JSON-RPC 2.0 notification from signal-cli's stdout.
// Notifications have no "id" field.
type rpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// receiveParams is the "params" of a "receive" notification.
type receiveParams struct {
	Envelope envelope `json:"envelope"`
}

// envelope is the top-level Signal message wrapper.
type envelope struct {
	Source       string      `json:"source"`
	SourceDevice int         `json:"sourceDevice"`
	Timestamp    int64       `json:"timestamp"`
	DataMessage  dataMessage `json:"dataMessage"`
}

// dataMessage carries the actual text payload.
type dataMessage struct {
	Timestamp int64      `json:"timestamp"`
	Message   string     `json:"message"`
	GroupInfo *groupInfo `json:"groupInfo"`
	Mentions  []mention  `json:"mentions"`
}

// groupInfo is present when the message was sent to a group.
type groupInfo struct {
	GroupID string `json:"groupId"`
	Type    string `json:"type"`
}

// mention represents an @mention within a Signal message.
type mention struct {
	// Number is the E.164 phone number of the mentioned party.
	Number string `json:"number"`
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

// Connector bridges Signal to the SoulGate HTTP API via signal-cli JSON-RPC.
type Connector struct {
	config     Config
	httpClient *http.Client

	// cmd is the running signal-cli process.
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	// rpcIDCounter is used to generate unique JSON-RPC request IDs.
	rpcIDCounter atomic.Uint64

	// mu guards writes to stdin so concurrent goroutines don't interleave.
	mu sync.Mutex

	stopOnce sync.Once
	stopped  chan struct{}
}

// New creates a new Signal connector.  It validates the configuration and
// verifies that signal-cli can be located, but does not yet spawn the process.
func New(config Config) (*Connector, error) {
	if config.PhoneNumber == "" {
		return nil, fmt.Errorf("signal: PhoneNumber is required")
	}
	if !strings.HasPrefix(config.PhoneNumber, "+") {
		return nil, fmt.Errorf("signal: PhoneNumber must be in E.164 format (e.g. +15551234567), got %q", config.PhoneNumber)
	}
	if config.GatewayURL == "" {
		config.GatewayURL = defaultGatewayURL
	}
	if config.SignalCLI == "" {
		config.SignalCLI = defaultSignalCLI
	}

	// Verify the binary exists before we do anything else so the error is
	// actionable: "signal-cli not found in PATH" is clearer than a fork error.
	if _, err := exec.LookPath(config.SignalCLI); err != nil {
		return nil, fmt.Errorf("signal: signal-cli binary not found (%q): %w\n"+
			"Install signal-cli from https://github.com/AsamK/signal-cli/releases "+
			"and ensure it is on your PATH, or set --signal-cli to the full path", config.SignalCLI, err)
	}

	return &Connector{
		config:     config,
		httpClient: &http.Client{Timeout: 0}, // no timeout — agentic loops can be long
		stopped:    make(chan struct{}),
	}, nil
}

// Start spawns "signal-cli -u <phone> jsonRpc", reads incoming JSON-RPC
// notifications from its stdout, and blocks until ctx is cancelled or Stop is
// called.
func (c *Connector) Start(ctx context.Context) error {
	args := []string{"-u", c.config.PhoneNumber, "jsonRpc"}
	c.cmd = exec.CommandContext(ctx, c.config.SignalCLI, args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("signal: failed to open stdin pipe: %w", err)
	}
	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("signal: failed to open stdout pipe: %w", err)
	}

	// Capture stderr for diagnostic logging.
	stderrReader, stderrWriter := io.Pipe()
	c.cmd.Stderr = stderrWriter

	if err := c.cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("signal: signal-cli binary not found: %w", err)
		}
		return fmt.Errorf("signal: failed to start signal-cli: %w", err)
	}

	log.Printf("signal: signal-cli started (pid=%d, number=%s, gateway=%s)",
		c.cmd.Process.Pid, c.config.PhoneNumber, c.config.GatewayURL)

	// Drain stderr in the background so the process never blocks on it.
	go func() {
		defer stderrWriter.Close()
		scanner := bufio.NewScanner(stderrReader)
		for scanner.Scan() {
			log.Printf("signal-cli stderr: %s", scanner.Text())
		}
	}()

	// Read JSON-RPC notifications from stdout.
	go c.readLoop()

	// Wait for ctx cancellation or an explicit Stop call.
	select {
	case <-ctx.Done():
		return c.Stop()
	case <-c.stopped:
		return nil
	}
}

// Stop terminates the signal-cli subprocess gracefully.  It is safe to call
// more than once.
func (c *Connector) Stop() error {
	var stopErr error
	c.stopOnce.Do(func() {
		log.Printf("signal: stopping connector")

		// Close stdin so signal-cli knows to exit cleanly.
		if c.stdin != nil {
			_ = c.stdin.Close()
		}
		// Kill the process if it does not exit on its own.
		if c.cmd != nil && c.cmd.Process != nil {
			if err := c.cmd.Process.Kill(); err != nil {
				// "process already finished" is not an error we care about.
				if !errors.Is(err, exec.ErrNotFound) {
					stopErr = fmt.Errorf("signal: failed to kill signal-cli: %w", err)
				}
			}
		}
		close(c.stopped)
	})
	return stopErr
}

// readLoop scans signal-cli's stdout line-by-line and dispatches each
// JSON-RPC notification to handleNotification.
func (c *Connector) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	// Signal messages can be large (base64-encoded attachments); increase buffer.
	const maxScanBuf = 4 * 1024 * 1024 // 4 MiB
	scanner.Buffer(make([]byte, 64*1024), maxScanBuf)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var notif rpcNotification
		if err := json.Unmarshal(line, &notif); err != nil {
			log.Printf("signal: failed to parse JSON-RPC line: %v (line: %s)", err, line)
			continue
		}

		// We only care about "receive" notifications.
		if notif.Method != "receive" {
			continue
		}

		c.handleNotification(notif)
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("signal: stdout scanner error: %v", err)
	}

	// stdout closed — the process has exited.  Trigger stopped so Start returns.
	c.stopOnce.Do(func() { close(c.stopped) })
}

// handleNotification parses a "receive" notification and dispatches message
// handling to a goroutine so the read loop is never blocked.
func (c *Connector) handleNotification(notif rpcNotification) {
	var params receiveParams
	if err := json.Unmarshal(notif.Params, &params); err != nil {
		log.Printf("signal: failed to parse receive params: %v", err)
		return
	}

	env := params.Envelope

	// Ignore echo of our own messages.
	if env.Source == c.config.PhoneNumber {
		return
	}

	text := strings.TrimSpace(env.DataMessage.Message)
	if text == "" {
		return // not a text message (reaction, attachment with no caption, etc.)
	}

	isGroup := env.DataMessage.GroupInfo != nil
	isMentioned := c.isMentioned(env.DataMessage)

	if isGroup && !isMentioned {
		// In group chats, only respond when explicitly @mentioned.
		return
	}

	// Strip the @mention from the text before forwarding.
	if isMentioned {
		text = c.stripMention(text)
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
	}

	if isGroup {
		groupID := env.DataMessage.GroupInfo.GroupID
		log.Printf("signal: group message from %s in group %s: %q", env.Source, groupID, text)
		go c.processGroupMessage(groupID, text)
	} else {
		log.Printf("signal: DM from %s: %q", env.Source, text)
		go c.processDM(env.Source, text)
	}
}

// processDM calls the gateway and sends the response back as a direct message.
func (c *Connector) processDM(recipient, text string) {
	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("signal: gateway error (DM from %s): %v", recipient, err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	if err := c.sendReply(recipient, response); err != nil {
		log.Printf("signal: failed to send DM reply to %s: %v", recipient, err)
	}
}

// processGroupMessage calls the gateway and sends the response back to the group.
func (c *Connector) processGroupMessage(groupID, text string) {
	response, err := c.sendToGateway(text)
	if err != nil {
		log.Printf("signal: gateway error (group %s): %v", groupID, err)
		response = fmt.Sprintf("SoulGate error: %v", err)
	}

	if err := c.sendGroupReply(groupID, response); err != nil {
		log.Printf("signal: failed to send group reply to %s: %v", groupID, err)
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

	log.Printf("signal: gateway responded with %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// sendReply sends a text message to an individual Signal recipient via
// JSON-RPC.  Long messages are split into chunks to avoid signal-cli limits.
func (c *Connector) sendReply(recipient, message string) error {
	const maxChunk = 4096
	chunks := splitMessage(message, maxChunk)
	for _, chunk := range chunks {
		params, err := json.Marshal(sendParams{
			Recipient: []string{recipient},
			Message:   chunk,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal send params: %w", err)
		}
		if err := c.writeRPC("send", params); err != nil {
			return err
		}
	}
	return nil
}

// sendGroupReply sends a text message to a Signal group via JSON-RPC.
func (c *Connector) sendGroupReply(groupID, message string) error {
	const maxChunk = 4096
	chunks := splitMessage(message, maxChunk)
	for _, chunk := range chunks {
		params, err := json.Marshal(sendGroupParams{
			GroupID: groupID,
			Message: chunk,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal group send params: %w", err)
		}
		if err := c.writeRPC("send", params); err != nil {
			return err
		}
	}
	return nil
}

// writeRPC encodes a JSON-RPC 2.0 request and writes it as a single line to
// signal-cli's stdin.  mu ensures writes from concurrent goroutines do not
// interleave.
func (c *Connector) writeRPC(method string, params json.RawMessage) error {
	id := fmt.Sprintf("%d", c.rpcIDCounter.Add(1))
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("signal: failed to marshal RPC request: %w", err)
	}
	data = append(data, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write(data); err != nil {
		return fmt.Errorf("signal: failed to write RPC request to stdin: %w", err)
	}
	return nil
}

// isMentioned reports whether the bot's own phone number is listed in the
// message's mention list.  Signal encodes @mentions as structured objects
// alongside the free-text body.
func (c *Connector) isMentioned(dm dataMessage) bool {
	for _, m := range dm.Mentions {
		if m.Number == c.config.PhoneNumber {
			return true
		}
	}
	// Fallback: check for a literal "@<number>" substring in the message text.
	// Some clients include this even when the structured mention list is absent.
	return strings.Contains(dm.Message, "@"+c.config.PhoneNumber) ||
		strings.Contains(dm.Message, "@"+strings.TrimPrefix(c.config.PhoneNumber, "+"))
}

// stripMention removes @<phone> patterns for our own number from the message
// text so the model receives clean input.
func (c *Connector) stripMention(text string) string {
	text = strings.ReplaceAll(text, "@"+c.config.PhoneNumber, "")
	// Also handle the variant without the leading "+".
	text = strings.ReplaceAll(text, "@"+strings.TrimPrefix(c.config.PhoneNumber, "+"), "")
	return text
}

// splitMessage divides text into chunks of at most maxLen bytes, preferring to
// break on newline or space boundaries to avoid cutting words.
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
