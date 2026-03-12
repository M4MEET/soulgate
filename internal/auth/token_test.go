package auth

import (
	"testing"
	"time"

	"github.com/M4MEET/soulgate/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenManager(t *testing.T) {
	tm := NewTokenManager()

	t.Run("Generate token", func(t *testing.T) {
		token, err := tm.GenerateToken("client-1", protocol.RoleAgent, 1*time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token.Value)
		assert.Equal(t, "client-1", token.ClientID)
		assert.Equal(t, protocol.RoleAgent, token.Role)
		assert.False(t, token.Revoked)
	})

	t.Run("Validate token", func(t *testing.T) {
		token, err := tm.GenerateToken("client-2", protocol.RoleUI, 1*time.Hour)
		require.NoError(t, err)

		// Valid token
		validated, err := tm.ValidateToken(token.Value)
		require.NoError(t, err)
		assert.Equal(t, token.ClientID, validated.ClientID)
		assert.Equal(t, token.Role, validated.Role)
	})

	t.Run("Invalid token", func(t *testing.T) {
		_, err := tm.ValidateToken("invalid-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("Revoked token", func(t *testing.T) {
		token, err := tm.GenerateToken("client-3", protocol.RoleChannel, 1*time.Hour)
		require.NoError(t, err)

		// Revoke token
		err = tm.RevokeToken(token.Value)
		require.NoError(t, err)

		// Validate should fail
		_, err = tm.ValidateToken(token.Value)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "revoked")
	})

	t.Run("Expired token", func(t *testing.T) {
		token, err := tm.GenerateToken("client-4", protocol.RoleAgent, 1*time.Millisecond)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		// Validate should fail
		_, err = tm.ValidateToken(token.Value)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("Cleanup expired", func(t *testing.T) {
		// Create expired tokens
		tm.GenerateToken("client-5", protocol.RoleAgent, 1*time.Millisecond)
		tm.GenerateToken("client-6", protocol.RoleAgent, 1*time.Millisecond)

		time.Sleep(10 * time.Millisecond)

		count := tm.CleanupExpired()
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("List tokens for client", func(t *testing.T) {
		tm.GenerateToken("client-7", protocol.RoleAgent, 1*time.Hour)
		tm.GenerateToken("client-7", protocol.RoleAgent, 1*time.Hour)

		tokens := tm.ListTokens("client-7")
		assert.GreaterOrEqual(t, len(tokens), 2)
	})

	t.Run("Revoke client tokens", func(t *testing.T) {
		tm.GenerateToken("client-8", protocol.RoleAgent, 1*time.Hour)
		tm.GenerateToken("client-8", protocol.RoleAgent, 1*time.Hour)

		count := tm.RevokeClientTokens("client-8")
		assert.GreaterOrEqual(t, count, 2)
	})
}
