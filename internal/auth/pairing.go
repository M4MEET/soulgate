package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
)

// PairingCode represents a temporary pairing code
type PairingCode struct {
	Code      string
	ClientID  string
	Role      protocol.ClientRole
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
	UsedBy    string
	UsedAt    time.Time
}

// PairingManager manages device pairing
type PairingManager struct {
	codes  map[string]*PairingCode // code -> pairing code
	mu     sync.RWMutex
	tokens *TokenManager
}

// NewPairingManager creates a new pairing manager
func NewPairingManager(tokens *TokenManager) *PairingManager {
	return &PairingManager{
		codes:  make(map[string]*PairingCode),
		tokens: tokens,
	}
}

// GeneratePairingCode generates a new pairing code
func (pm *PairingManager) GeneratePairingCode(clientID string, role protocol.ClientRole, duration time.Duration) (*PairingCode, error) {
	// Generate 6-digit pairing code
	code, err := generateNumericCode(6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate pairing code: %w", err)
	}

	// Ensure code is unique
	pm.mu.Lock()
	for {
		if _, exists := pm.codes[code]; !exists {
			break
		}
		// Code collision, generate new one
		code, err = generateNumericCode(6)
		if err != nil {
			pm.mu.Unlock()
			return nil, err
		}
	}

	now := time.Now()
	pairingCode := &PairingCode{
		Code:      code,
		ClientID:  clientID,
		Role:      role,
		CreatedAt: now,
		ExpiresAt: now.Add(duration),
		Used:      false,
	}

	pm.codes[code] = pairingCode
	pm.mu.Unlock()

	return pairingCode, nil
}

// ValidatePairingCode validates a pairing code and issues a token
func (pm *PairingManager) ValidatePairingCode(code, deviceID string) (*Token, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pairingCode, exists := pm.codes[code]
	if !exists {
		return nil, fmt.Errorf("invalid pairing code")
	}

	if pairingCode.Used {
		return nil, fmt.Errorf("pairing code already used")
	}

	if time.Now().After(pairingCode.ExpiresAt) {
		return nil, fmt.Errorf("pairing code expired")
	}

	// Mark as used
	pairingCode.Used = true
	pairingCode.UsedBy = deviceID
	pairingCode.UsedAt = time.Now()

	// Generate auth token (valid for 30 days)
	token, err := pm.tokens.GenerateToken(deviceID, pairingCode.Role, 30*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Add metadata
	token.Metadata["paired_from"] = pairingCode.ClientID
	token.Metadata["pairing_code"] = code

	return token, nil
}

// RevokePairingCode revokes a pairing code
func (pm *PairingManager) RevokePairingCode(code string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pairingCode, exists := pm.codes[code]
	if !exists {
		return fmt.Errorf("pairing code not found")
	}

	pairingCode.Used = true
	pairingCode.UsedBy = "revoked"
	pairingCode.UsedAt = time.Now()

	return nil
}

// GetPairingCode retrieves a pairing code
func (pm *PairingManager) GetPairingCode(code string) (*PairingCode, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pairingCode, exists := pm.codes[code]
	return pairingCode, exists
}

// ListActiveCodes lists all active (unused, unexpired) pairing codes
func (pm *PairingManager) ListActiveCodes() []*PairingCode {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	now := time.Now()
	activeCodes := make([]*PairingCode, 0)

	for _, pairingCode := range pm.codes {
		if !pairingCode.Used && now.Before(pairingCode.ExpiresAt) {
			activeCodes = append(activeCodes, pairingCode)
		}
	}

	return activeCodes
}

// CleanupExpired removes expired pairing codes
func (pm *PairingManager) CleanupExpired() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	count := 0

	for code, pairingCode := range pm.codes {
		if now.After(pairingCode.ExpiresAt) || pairingCode.Used {
			delete(pm.codes, code)
			count++
		}
	}

	return count
}

// ActiveCodeCount returns the number of active pairing codes
func (pm *PairingManager) ActiveCodeCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	now := time.Now()
	count := 0

	for _, pairingCode := range pm.codes {
		if !pairingCode.Used && now.Before(pairingCode.ExpiresAt) {
			count++
		}
	}

	return count
}

// generateNumericCode generates a random numeric code of specified length
func generateNumericCode(length int) (string, error) {
	code := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}
