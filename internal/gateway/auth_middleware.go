package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// APIToken represents a named API token for HTTP endpoint access.
type APIToken struct {
	Value     string    `json:"value"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

// rateLimiter is a token-bucket rate limiter for a single API token.
type rateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// Allow consumes one token, refilling the bucket first based on elapsed time.
// Returns true if the request is within the rate limit.
func (rl *rateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now

	if rl.tokens < 1 {
		return false
	}
	rl.tokens--
	return true
}

// APIAuth manages API tokens and per-token rate limiters for HTTP endpoints.
// The zero value is not usable; create with NewAPIAuth.
type APIAuth struct {
	mu           sync.RWMutex
	tokens       map[string]*APIToken    // token value -> token
	rateLimiters map[string]*rateLimiter // token value -> limiter

	// Rate-limit configuration (applies to newly created limiters).
	requestsPerMinute float64 // default: 60

	// path is the optional JSON file used for persistence.
	path string
}

// NewAPIAuth creates an APIAuth with the given requests-per-minute cap.
// Pass 0 to use the default of 60 requests per minute.
func NewAPIAuth(requestsPerMinute float64) *APIAuth {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 60
	}
	return &APIAuth{
		tokens:            make(map[string]*APIToken),
		rateLimiters:      make(map[string]*rateLimiter),
		requestsPerMinute: requestsPerMinute,
	}
}

// NewAPIAuthFromFile creates an APIAuth that loads existing tokens from path
// (if the file exists) and persists all mutations back to the same file.
func NewAPIAuthFromFile(path string, requestsPerMinute float64) (*APIAuth, error) {
	a := NewAPIAuth(requestsPerMinute)
	a.path = path
	if err := a.load(); err != nil {
		return nil, err
	}
	return a, nil
}

// load reads tokens from the backing JSON file. Missing file is not an error.
func (a *APIAuth) load() error {
	if a.path == "" {
		return nil
	}
	data, err := os.ReadFile(a.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load api tokens: %w", err)
	}

	var list []*APIToken
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse api tokens: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tok := range list {
		a.tokens[tok.Value] = tok
		a.rateLimiters[tok.Value] = &rateLimiter{
			tokens:     a.requestsPerMinute,
			maxTokens:  a.requestsPerMinute,
			refillRate: a.requestsPerMinute / 60.0,
			lastRefill: time.Now(),
		}
	}
	return nil
}

// save writes all tokens to the backing JSON file.
// The caller must hold at least a read lock or ensure single-threaded access.
func (a *APIAuth) save() error {
	if a.path == "" {
		return nil
	}

	list := make([]*APIToken, 0, len(a.tokens))
	for _, tok := range a.tokens {
		copy := *tok
		list = append(list, &copy)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal api tokens: %w", err)
	}
	if err := os.WriteFile(a.path, data, 0o600); err != nil {
		return fmt.Errorf("write api tokens: %w", err)
	}
	return nil
}

// CreateToken generates a new sg_ prefixed API token with an optional label.
func (a *APIAuth) CreateToken(name string) (*APIToken, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	value := "sg_" + hex.EncodeToString(raw)

	tok := &APIToken{
		Value:     value,
		Name:      name,
		CreatedAt: time.Now(),
		Revoked:   false,
	}

	a.mu.Lock()
	a.tokens[value] = tok
	a.rateLimiters[value] = &rateLimiter{
		tokens:     a.requestsPerMinute,
		maxTokens:  a.requestsPerMinute,
		refillRate: a.requestsPerMinute / 60.0,
		lastRefill: time.Now(),
	}
	if err := a.save(); err != nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("persist token: %w", err)
	}
	a.mu.Unlock()
	return tok, nil
}

// RevokeToken marks the named token as revoked. Returns an error if it does
// not exist.
func (a *APIAuth) RevokeToken(value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	tok, ok := a.tokens[value]
	if !ok {
		return fmt.Errorf("token not found")
	}
	tok.Revoked = true
	return a.save()
}

// ListTokens returns a snapshot of all tokens (including revoked ones).
func (a *APIAuth) ListTokens() []*APIToken {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make([]*APIToken, 0, len(a.tokens))
	for _, t := range a.tokens {
		copy := *t
		out = append(out, &copy)
	}
	return out
}

// validate checks the token value and the rate limiter. Returns the token and
// nil on success, or nil and an error describing the failure.
func (a *APIAuth) validate(value string) (*APIToken, error) {
	a.mu.RLock()
	tok, ok := a.tokens[value]
	rl := a.rateLimiters[value]
	a.mu.RUnlock()

	if !ok || tok.Revoked {
		return nil, fmt.Errorf("invalid or revoked token")
	}
	if rl != nil && !rl.Allow() {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	return tok, nil
}

// bearerToken extracts the token value from an "Authorization: Bearer sg_xxx"
// header. Returns an empty string when the header is absent or malformed.
func bearerToken(r *http.Request) string {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return ""
	}
	// Accept both "Bearer " and "bearer " (case-insensitive prefix check).
	lower := strings.ToLower(hdr)
	if !strings.HasPrefix(lower, "bearer ") {
		return ""
	}
	return strings.TrimSpace(hdr[7:])
}

// isLocalhost reports whether the request originated from the loopback address.
func isLocalhost(r *http.Request) bool {
	addr := r.RemoteAddr
	// RemoteAddr is "host:port" for TCP connections.
	colon := strings.LastIndex(addr, ":")
	if colon != -1 {
		addr = addr[:colon]
	}
	// Strip IPv6 brackets.
	addr = strings.Trim(addr, "[]")
	switch addr {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	return false
}

// apiAuthMiddleware wraps an http.Handler with Bearer-token authentication and
// per-token rate limiting.
//
// When devMode is true, requests from localhost bypass authentication entirely,
// which is convenient during local development. In production set devMode to
// false to enforce authentication for all callers.
//
// Requests that include a valid "Authorization: Bearer sg_xxx" header and are
// within the rate limit are forwarded to next. All other requests receive a
// 401 or 429 JSON error.
func apiAuthMiddleware(apiAuth *APIAuth, devMode bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always pass CORS preflight through so the browser can negotiate.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Dev-mode bypass for localhost.
		if devMode && isLocalhost(r) {
			next.ServeHTTP(w, r)
			return
		}

		value := bearerToken(r)
		if value == "" {
			writeAPIAuthError(w, http.StatusUnauthorized, "missing or malformed Authorization header; expected: Bearer sg_xxx")
			return
		}

		_, err := apiAuth.validate(value)
		if err != nil {
			if strings.Contains(err.Error(), "rate limit") {
				writeAPIAuthError(w, http.StatusTooManyRequests, err.Error())
				return
			}
			writeAPIAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeAPIAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}
