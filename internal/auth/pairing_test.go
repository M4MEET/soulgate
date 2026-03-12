package auth

import (
	"testing"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPairingManager(t *testing.T) {
	tm := NewTokenManager()
	pm := NewPairingManager(tm)

	t.Run("Generate pairing code", func(t *testing.T) {
		code, err := pm.GeneratePairingCode("client-1", protocol.RoleAgent, 5*time.Minute)
		require.NoError(t, err)
		assert.NotEmpty(t, code.Code)
		assert.Len(t, code.Code, 6) // 6-digit code
		assert.Equal(t, "client-1", code.ClientID)
		assert.Equal(t, protocol.RoleAgent, code.Role)
		assert.False(t, code.Used)
	})

	t.Run("Validate pairing code", func(t *testing.T) {
		code, err := pm.GeneratePairingCode("client-2", protocol.RoleUI, 5*time.Minute)
		require.NoError(t, err)

		// Pair device
		token, err := pm.ValidatePairingCode(code.Code, "device-123")
		require.NoError(t, err)
		assert.NotEmpty(t, token.Value)
		assert.Equal(t, "device-123", token.ClientID)
		assert.Equal(t, protocol.RoleUI, token.Role)

		// Code should be marked as used
		savedCode, exists := pm.GetPairingCode(code.Code)
		require.True(t, exists)
		assert.True(t, savedCode.Used)
		assert.Equal(t, "device-123", savedCode.UsedBy)
	})

	t.Run("Invalid pairing code", func(t *testing.T) {
		_, err := pm.ValidatePairingCode("999999", "device-456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pairing code")
	})

	t.Run("Already used code", func(t *testing.T) {
		code, err := pm.GeneratePairingCode("client-3", protocol.RoleAgent, 5*time.Minute)
		require.NoError(t, err)

		// Use code
		_, err = pm.ValidatePairingCode(code.Code, "device-1")
		require.NoError(t, err)

		// Try to use again
		_, err = pm.ValidatePairingCode(code.Code, "device-2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already used")
	})

	t.Run("Expired pairing code", func(t *testing.T) {
		code, err := pm.GeneratePairingCode("client-4", protocol.RoleAgent, 1*time.Millisecond)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Try to use
		_, err = pm.ValidatePairingCode(code.Code, "device-3")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("List active codes", func(t *testing.T) {
		pm.GeneratePairingCode("client-5", protocol.RoleAgent, 5*time.Minute)
		pm.GeneratePairingCode("client-6", protocol.RoleUI, 5*time.Minute)

		activeCodes := pm.ListActiveCodes()
		assert.GreaterOrEqual(t, len(activeCodes), 2)
	})

	t.Run("Cleanup expired codes", func(t *testing.T) {
		pm.GeneratePairingCode("client-7", protocol.RoleAgent, 1*time.Millisecond)
		pm.GeneratePairingCode("client-8", protocol.RoleAgent, 1*time.Millisecond)

		time.Sleep(10 * time.Millisecond)

		count := pm.CleanupExpired()
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("Revoke pairing code", func(t *testing.T) {
		code, err := pm.GeneratePairingCode("client-9", protocol.RoleAgent, 5*time.Minute)
		require.NoError(t, err)

		// Revoke
		err = pm.RevokePairingCode(code.Code)
		require.NoError(t, err)

		// Try to use
		_, err = pm.ValidatePairingCode(code.Code, "device-4")
		assert.Error(t, err)
	})

	t.Run("Active code count", func(t *testing.T) {
		initialCount := pm.ActiveCodeCount()

		pm.GeneratePairingCode("client-10", protocol.RoleAgent, 5*time.Minute)
		pm.GeneratePairingCode("client-11", protocol.RoleAgent, 5*time.Minute)

		newCount := pm.ActiveCodeCount()
		assert.Equal(t, initialCount+2, newCount)
	})
}
