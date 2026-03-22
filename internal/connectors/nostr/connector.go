// Package nostr provides a Nostr protocol connector for SoulGate.
//
// Nostr (Notes and Other Stuff Transmitted by Relays) is a decentralized
// protocol where clients communicate through relay servers using WebSockets
// and JSON messages.
//
// Architecture:
//
//	Connector connects to each relay via WebSocket
//	    -> Sends REQ subscription for kind=1 events mentioning our pubkey
//	    -> Reads EVENT messages from the relay
//	    -> Filters for events that tag our pubkey in the "p" tag
//	    -> POSTs message content to SoulGate /api/chat
//	    -> Logs the gateway response (publishing requires secp256k1 signing — v2)
//
// Nostr protocol basics (NIPs implemented):
//   - NIP-01: Basic protocol (REQ, EVENT, CLOSE message types)
//   - Kind 1: text note (public mentions)
//   - Kind 4: encrypted DM (received as-is; NIP-04 decryption is v2)
//
// Event signing (NIP-01) requires secp256k1 ECDSA which needs an external
// dependency not currently in go.mod.  Publishing is therefore deferred to v2.
// In v1 the connector operates in read-only mode: it receives mentions and
// forwards them to the gateway, then logs "Would reply: <response>".
//
// References:
//
//	https://github.com/nostr-protocol/nostr
//	https://github.com/nostr-protocol/nips/blob/master/01.md
package nostr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultGatewayURL = "http://localhost:8080"

	// subscriptionID is the REQ subscription identifier sent to each relay.
	// Each relay connection uses this same ID because each connection is
	// independent and there is no cross-relay subscription state.
	subscriptionID = "sg-sub1"

	// reconnectDelay is how long to wait before reconnecting to a relay after
	// an unexpected disconnection.
	reconnectDelay = 10 * time.Second

	// writeTimeout is the deadline applied when sending WebSocket messages.
	writeTimeout = 30 * time.Second

	// readTimeout is the WebSocket read deadline; reset on every message.  A
	// relay that goes silent for this long is considered dead.
	readTimeout = 5 * time.Minute

	// pingInterval controls how often we send WebSocket pings to keep the
	// connection alive through NAT and proxy devices.
	pingInterval = 90 * time.Second
)

// Config holds all configuration for the Nostr connector.
type Config struct {
	// GatewayURL is the base URL of the SoulGate HTTP API.
	// Defaults to http://localhost:8080 when empty.
	GatewayURL string

	// PrivateKey is the hex-encoded Nostr private key (32 bytes, 64 hex chars).
	// The public key is derived from this.
	// Example: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	PrivateKey string

	// Relays is the list of WebSocket relay URLs to connect to.
	// Example: ["wss://relay.damus.io", "wss://nos.lol"]
	Relays []string
}

// nostrEvent is the JSON representation of a Nostr event (NIP-01).
type nostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig,omitempty"`
}

// eventID computes the NIP-01 event ID: SHA256 of the canonical serialisation.
//
// The canonical form is:
//
//	[0, pubkey, created_at, kind, tags, content]
//
// The ID is the hex-encoded SHA256 digest of that JSON array.
func (e *nostrEvent) computeID() (string, error) {
	canonical := []interface{}{
		0,
		e.PubKey,
		e.CreatedAt,
		e.Kind,
		e.Tags,
		e.Content,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal canonical event: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
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

// Connector bridges Nostr relays to the SoulGate HTTP API.
//
// One goroutine is started per relay.  Each goroutine independently subscribes
// and reconnects on failure.  Incoming events from any relay are forwarded to
// the gateway in their own goroutine so relay read loops are never blocked.
type Connector struct {
	config     Config
	pubKey     string // hex-encoded public key derived from PrivateKey
	httpClient *http.Client

	// stopped is closed when Stop is called.
	stopped  chan struct{}
	stopOnce sync.Once

	// wg tracks all relay goroutines so Stop can wait for them.
	wg sync.WaitGroup
}

// New creates and validates a Nostr connector.
//
// The PrivateKey is validated for correct length.  The public key is derived
// deterministically from the private key bytes using the secp256k1 curve's
// x-coordinate of G*privKey — in v1 we use a simplified derivation that
// produces the correct hex pubkey for subscription filtering.
func New(cfg Config) (*Connector, error) {
	if cfg.PrivateKey == "" {
		return nil, fmt.Errorf("nostr: PrivateKey is required (hex-encoded 32-byte secp256k1 key)")
	}

	privBytes, err := hex.DecodeString(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("nostr: PrivateKey is not valid hex: %w", err)
	}
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("nostr: PrivateKey must be 32 bytes (64 hex chars), got %d bytes", len(privBytes))
	}

	if len(cfg.Relays) == 0 {
		return nil, fmt.Errorf("nostr: at least one relay URL is required")
	}

	for i, relay := range cfg.Relays {
		if !strings.HasPrefix(relay, "ws://") && !strings.HasPrefix(relay, "wss://") {
			return nil, fmt.Errorf("nostr: relay[%d] %q must start with ws:// or wss://", i, relay)
		}
	}

	if cfg.GatewayURL == "" {
		cfg.GatewayURL = defaultGatewayURL
	}

	pubKey, err := derivePubKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("nostr: failed to derive public key: %w", err)
	}

	return &Connector{
		config:     cfg,
		pubKey:     pubKey,
		httpClient: &http.Client{Timeout: 0}, // agentic gateway calls may be long
		stopped:    make(chan struct{}),
	}, nil
}

// PubKey returns the hex-encoded Nostr public key derived from the configured
// private key.  This can be used to construct npub addresses or to set up relay
// subscriptions manually.
func (c *Connector) PubKey() string {
	return c.pubKey
}

// Start connects to all configured relays and listens for mentions.
// It blocks until ctx is cancelled or Stop is called.
// Relay connections are maintained independently and reconnect on failure.
func (c *Connector) Start(ctx context.Context) error {
	log.Printf("nostr connector: starting (pubkey: %s, relays: %v, gateway: %s)",
		c.pubKey, c.config.Relays, c.config.GatewayURL)

	for _, relayURL := range c.config.Relays {
		c.wg.Add(1)
		go func(url string) {
			defer c.wg.Done()
			c.runRelay(ctx, url)
		}(relayURL)
	}

	// Block until context is cancelled or Stop is called.
	select {
	case <-ctx.Done():
	case <-c.stopped:
	}

	// Wait for all relay goroutines to finish.
	c.wg.Wait()
	log.Printf("nostr connector: stopped")
	return nil
}

// Stop signals all relay goroutines to shut down.  Safe to call multiple times.
func (c *Connector) Stop() {
	c.stopOnce.Do(func() {
		log.Printf("nostr connector: shutting down")
		close(c.stopped)
	})
}

// runRelay manages the lifetime of a single relay connection with automatic
// reconnection.  It exits when ctx is cancelled or c.stopped is closed.
func (c *Connector) runRelay(ctx context.Context, relayURL string) {
	for {
		// Check for shutdown before attempting (re)connect.
		select {
		case <-c.stopped:
			return
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("nostr connector: connecting to relay %s", relayURL)

		if err := c.connectAndListen(ctx, relayURL); err != nil {
			// Only log if not a deliberate shutdown.
			select {
			case <-c.stopped:
				return
			case <-ctx.Done():
				return
			default:
				log.Printf("nostr connector: relay %s disconnected: %v — reconnecting in %s",
					relayURL, err, reconnectDelay)
			}
		}

		select {
		case <-c.stopped:
			return
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// connectAndListen opens a WebSocket connection to a single relay, subscribes
// for mentions, and reads events until the connection closes or ctx is done.
func (c *Connector) connectAndListen(ctx context.Context, relayURL string) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, relayURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	// Configure ping/pong keep-alive.
	conn.SetPingHandler(func(data string) error {
		return conn.WriteControl(websocket.PongMessage, []byte(data),
			time.Now().Add(writeTimeout))
	})

	log.Printf("nostr connector: connected to %s", relayURL)

	// Send subscription request.
	if err := c.sendSubscription(conn); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	// Start a goroutine to send periodic pings.
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil,
					time.Now().Add(writeTimeout)); err != nil {
					return
				}
			case <-c.stopped:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	defer func() { <-pingDone }()

	// Read loop.
	for {
		// Check for shutdown before blocking on read.
		select {
		case <-c.stopped:
			return nil
		case <-ctx.Done():
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(readTimeout))

		_, message, err := conn.ReadMessage()
		if err != nil {
			// If we are stopping, the error is expected.
			select {
			case <-c.stopped:
				return nil
			case <-ctx.Done():
				return nil
			default:
			}
			return fmt.Errorf("read: %w", err)
		}

		c.handleRelayMessage(ctx, relayURL, message)
	}
}

// sendSubscription sends a REQ message to the relay subscribing to kind=1
// events that tag our pubkey in a "p" tag (i.e., direct mentions).
//
// Nostr REQ format (NIP-01):
//
//	["REQ", <subscription_id>, <filter>, ...]
//
// Filter with "#p" matches events that include our pubkey in a "p" tag, which
// is the standard way to mention someone in Nostr.
func (c *Connector) sendSubscription(conn *websocket.Conn) error {
	// Filter: kind=1 (text notes) that mention our pubkey in a "p" tag.
	// We also include kind=4 (encrypted DMs) for completeness; the content will
	// be the NIP-04 ciphertext which we log but do not decrypt in v1.
	filter := map[string]interface{}{
		"kinds": []int{1, 4},
		"#p":    []string{c.pubKey},
		// Limit to events since connector start to avoid processing history.
		"since": time.Now().Unix(),
	}

	req := []interface{}{
		"REQ",
		subscriptionID,
		filter,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal REQ: %w", err)
	}

	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write REQ: %w", err)
	}

	log.Printf("nostr connector: subscribed on relay with filter kinds=[1,4] #p=%s", c.pubKey)
	return nil
}

// handleRelayMessage parses a raw message received from a relay and dispatches
// it to the appropriate handler.
//
// Nostr relay message formats (NIP-01):
//
//	["EVENT", <subscription_id>, <event_object>]
//	["EOSE",  <subscription_id>]            — end of stored events
//	["NOTICE", <message>]                   — human-readable relay message
//	["OK",    <event_id>, <status>, <msg>]  — ack for published events
func (c *Connector) handleRelayMessage(ctx context.Context, relayURL string, raw []byte) {
	// All Nostr relay messages are JSON arrays.
	var envelope []json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		log.Printf("nostr connector: relay %s: failed to parse message: %v", relayURL, err)
		return
	}

	if len(envelope) < 1 {
		return
	}

	var msgType string
	if err := json.Unmarshal(envelope[0], &msgType); err != nil {
		log.Printf("nostr connector: relay %s: failed to parse message type: %v", relayURL, err)
		return
	}

	switch msgType {
	case "EVENT":
		if len(envelope) < 3 {
			return
		}
		var event nostrEvent
		if err := json.Unmarshal(envelope[2], &event); err != nil {
			log.Printf("nostr connector: relay %s: failed to parse EVENT: %v", relayURL, err)
			return
		}
		// Dispatch in a separate goroutine so the read loop is never blocked
		// by a slow gateway call.
		go c.processEvent(ctx, relayURL, &event)

	case "EOSE":
		// End of stored events — the relay is now streaming live events.
		log.Printf("nostr connector: relay %s: end of stored events, streaming live", relayURL)

	case "NOTICE":
		// Human-readable message from the relay (rate limit warnings, etc.).
		if len(envelope) >= 2 {
			var notice string
			if err := json.Unmarshal(envelope[1], &notice); err == nil {
				log.Printf("nostr connector: relay %s NOTICE: %s", relayURL, notice)
			}
		}

	case "OK":
		// Acknowledgement for a published event.  Not relevant in v1 (read-only).

	default:
		log.Printf("nostr connector: relay %s: unknown message type %q", relayURL, msgType)
	}
}

// processEvent validates and processes a received Nostr event.
//
// For kind=1 (text notes):
//   - Verifies the event mentions our pubkey in a "p" tag
//   - Verifies the event ID matches the NIP-01 canonical hash
//   - Forwards the content to the SoulGate gateway
//   - Logs "Would reply: <response>" (publishing deferred to v2)
//
// For kind=4 (encrypted DMs):
//   - Logs that a DM was received (NIP-04 decryption deferred to v2)
func (c *Connector) processEvent(ctx context.Context, relayURL string, event *nostrEvent) {
	// Ignore our own events to prevent echo loops.
	if event.PubKey == c.pubKey {
		return
	}

	// Validate the event ID matches the NIP-01 canonical hash.
	// This prevents relay-injected events with tampered content.
	expectedID, err := event.computeID()
	if err != nil {
		log.Printf("nostr connector: relay %s: failed to compute event ID: %v", relayURL, err)
		return
	}
	if event.ID != expectedID {
		log.Printf("nostr connector: relay %s: event ID mismatch (got %s, expected %s) — ignoring",
			relayURL, event.ID, expectedID)
		return
	}

	switch event.Kind {
	case 1:
		// Text note — check that it actually tags our pubkey.
		if !c.isMentioned(event) {
			return
		}

		content := strings.TrimSpace(event.Content)
		if content == "" {
			return
		}

		log.Printf("nostr connector: kind=1 mention from %s (id=%s): %q", event.PubKey, event.ID, content)

		response, err := c.sendToGateway(content)
		if err != nil {
			log.Printf("nostr connector: gateway error for event %s: %v", event.ID, err)
			return
		}

		// v1: log the response instead of publishing a reply event.
		// Publishing requires secp256k1 signing (NIP-01) which needs an
		// external dependency.  This is planned for v2.
		log.Printf("nostr connector: Would reply to %s (event %s): %s",
			event.PubKey, event.ID, response)

	case 4:
		// Encrypted DM (NIP-04).  The content is AES-256-CBC encrypted with
		// the shared ECDH secret.  We log the receipt but do not decrypt in v1.
		log.Printf("nostr connector: kind=4 encrypted DM from %s (id=%s) — NIP-04 decryption deferred to v2",
			event.PubKey, event.ID)

	default:
		log.Printf("nostr connector: relay %s: unexpected kind=%d in event %s", relayURL, event.Kind, event.ID)
	}
}

// isMentioned reports whether our pubkey appears in any "p" tag of the event.
func (c *Connector) isMentioned(event *nostrEvent) bool {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] == c.pubKey {
			return true
		}
	}
	return false
}

// sendToGateway POSTs the message text to the SoulGate /api/chat endpoint
// and returns the text response.
func (c *Connector) sendToGateway(message string) (string, error) {
	payload, err := json.Marshal(gatewayRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	apiURL := strings.TrimRight(c.config.GatewayURL, "/") + "/api/chat"
	resp, err := c.httpClient.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("POST %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	var gwResp gatewayResponse
	if err := json.NewDecoder(resp.Body).Decode(&gwResp); err != nil {
		return "", fmt.Errorf("decode gateway response (HTTP %d): %w", resp.StatusCode, err)
	}

	if gwResp.Error != "" {
		return "", fmt.Errorf("gateway error: %s", gwResp.Error)
	}
	if gwResp.Response == "" {
		return "", fmt.Errorf("gateway returned empty response (HTTP %d)", resp.StatusCode)
	}

	log.Printf("nostr connector: gateway response: %d chars", len(gwResp.Response))
	return gwResp.Response, nil
}

// derivePubKey derives the Nostr public key (secp256k1 x-coordinate) from the
// given 32-byte private key scalar.
//
// Nostr public keys are the x-coordinate of the secp256k1 point G*privKey,
// encoded as a 32-byte big-endian integer in hex (NIP-01 "schnorr" keys,
// also called "x-only" public keys per BIP-340).
//
// Pure Go secp256k1 multiplication is implemented here so that the connector
// compiles without any CGO or external dependencies.  The implementation uses
// standard projective point arithmetic on the secp256k1 curve.
//
// secp256k1 parameters (all values are hex-encoded big integers):
//
//	p  = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F
//	a  = 0 (secp256k1 is y^2 = x^3 + 7)
//	b  = 7
//	Gx = 79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798
//	Gy = 483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8
//	n  = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
func derivePubKey(privKey []byte) (string, error) {
	// secp256k1 field prime p.
	pHex := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEFFFFFC2F"
	// Generator point x and y coordinates.
	gxHex := "79BE667EF9DCBBAC55A06295CE870B07029BFCDB2DCE28D959F2815B16F81798"
	gyHex := "483ADA7726A3C4655DA4FBFC0E1108A8FD17B448A68554199C47D08FFB10D4B8"

	p, ok := new(secp256k1Int).SetHex(pHex)
	if !ok {
		return "", fmt.Errorf("parse p")
	}
	gx, ok := new(secp256k1Int).SetHex(gxHex)
	if !ok {
		return "", fmt.Errorf("parse Gx")
	}
	gy, ok := new(secp256k1Int).SetHex(gyHex)
	if !ok {
		return "", fmt.Errorf("parse Gy")
	}

	k := new(secp256k1Int).SetBytes(privKey)

	rx, _, err := scalarMult(gx, gy, k, p)
	if err != nil {
		return "", fmt.Errorf("scalar mult: %w", err)
	}

	// Return 32-byte big-endian hex of the x-coordinate.
	xBytes := rx.Bytes32()
	return hex.EncodeToString(xBytes[:]), nil
}
