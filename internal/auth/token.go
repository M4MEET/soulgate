package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
)

// Token represents an authentication token
type Token struct {
	Value      string
	ClientID   string
	Role       protocol.ClientRole
	IssuedAt   time.Time
	ExpiresAt  time.Time
	Metadata   map[string]string
	Revoked    bool
}

// TokenManager manages authentication tokens
type TokenManager struct {
	tokens map[string]*Token // token value -> token
	mu     sync.RWMutex
}

// NewTokenManager creates a new token manager
func NewTokenManager() *TokenManager {
	return &TokenManager{
		tokens: make(map[string]*Token),
	}
}

// GenerateToken generates a new authentication token
func (tm *TokenManager) GenerateToken(clientID string, role protocol.ClientRole, duration time.Duration) (*Token, error) {
	// Generate random token value
	tokenValue, err := generateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	token := &Token{
		Value:     tokenValue,
		ClientID:  clientID,
		Role:      role,
		IssuedAt:  now,
		ExpiresAt: now.Add(duration),
		Metadata:  make(map[string]string),
		Revoked:   false,
	}

	// Store token
	tm.mu.Lock()
	tm.tokens[tokenValue] = token
	tm.mu.Unlock()

	return token, nil
}

// ValidateToken validates a token and returns the associated token object
func (tm *TokenManager) ValidateToken(tokenValue string) (*Token, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	token, exists := tm.tokens[tokenValue]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}

	if token.Revoked {
		return nil, fmt.Errorf("token has been revoked")
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}

	return token, nil
}

// RevokeToken revokes a token
func (tm *TokenManager) RevokeToken(tokenValue string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	token, exists := tm.tokens[tokenValue]
	if !exists {
		return fmt.Errorf("token not found")
	}

	token.Revoked = true
	return nil
}

// RevokeClientTokens revokes all tokens for a client
func (tm *TokenManager) RevokeClientTokens(clientID string) int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	count := 0
	for _, token := range tm.tokens {
		if token.ClientID == clientID && !token.Revoked {
			token.Revoked = true
			count++
		}
	}

	return count
}

// CleanupExpired removes expired tokens
func (tm *TokenManager) CleanupExpired() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	count := 0

	for tokenValue, token := range tm.tokens {
		if now.After(token.ExpiresAt) {
			delete(tm.tokens, tokenValue)
			count++
		}
	}

	return count
}

// GetToken retrieves a token by value
func (tm *TokenManager) GetToken(tokenValue string) (*Token, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	token, exists := tm.tokens[tokenValue]
	return token, exists
}

// ListTokens returns all tokens for a client
func (tm *TokenManager) ListTokens(clientID string) []*Token {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tokens := make([]*Token, 0)
	for _, token := range tm.tokens {
		if token.ClientID == clientID {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// TokenCount returns the total number of active tokens
func (tm *TokenManager) TokenCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	count := 0
	for _, token := range tm.tokens {
		if !token.Revoked && time.Now().Before(token.ExpiresAt) {
			count++
		}
	}

	return count
}

// generateRandomToken generates a random token string
func generateRandomToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}
