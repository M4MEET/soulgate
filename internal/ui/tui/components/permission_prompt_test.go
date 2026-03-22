package components

import (
	"testing"

	"github.com/M4MEET/soulgate/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeActivePrompt returns a PermissionPrompt that is Active with a buffered
// response channel and a non-nil Request. The channel has capacity 1 so that
// HandleKey can send without blocking.
func makeActivePrompt() (*PermissionPrompt, chan core.PermissionResponse) {
	ch := make(chan core.PermissionResponse, 1)
	req := &core.PermissionRequest{
		Action:      "files.read",
		Resource:    "./secret.txt",
		Description: "Read file: ./secret.txt",
		Reason:      "no matching rule (default deny)",
	}
	p := &PermissionPrompt{
		Active:   true,
		Request:  req,
		Response: ch,
	}
	return p, ch
}

// TestPermissionPromptHandleKey_Allow verifies that pressing "a" while the
// prompt is active sends an Approved=true, LearnPattern=false response and
// returns handled=true with a non-nil result.
func TestPermissionPromptHandleKey_Allow(t *testing.T) {
	p, ch := makeActivePrompt()

	handled, result := p.HandleKey("a")

	require.True(t, handled)
	require.NotNil(t, result)
	assert.True(t, result.Approved)
	assert.False(t, result.LearnPattern)

	// Channel must have received the response.
	resp := <-ch
	assert.True(t, resp.Approved)
	assert.False(t, resp.LearnPattern)

	// Prompt must be deactivated after a decision.
	assert.False(t, p.Active)
	assert.Nil(t, p.Response)
}

// TestPermissionPromptHandleKey_Learn verifies that pressing "l" while the
// prompt is active sends an Approved=true, LearnPattern=true response.
func TestPermissionPromptHandleKey_Learn(t *testing.T) {
	p, ch := makeActivePrompt()

	handled, result := p.HandleKey("l")

	require.True(t, handled)
	require.NotNil(t, result)
	assert.True(t, result.Approved)
	assert.True(t, result.LearnPattern)

	resp := <-ch
	assert.True(t, resp.Approved)
	assert.True(t, resp.LearnPattern)

	assert.False(t, p.Active)
	assert.Nil(t, p.Response)
}

// TestPermissionPromptHandleKey_Deny verifies that pressing "d" sends an
// Approved=false response and deactivates the prompt.
func TestPermissionPromptHandleKey_Deny(t *testing.T) {
	for _, key := range []string{"d", "D", "n", "N", "esc"} {
		t.Run("key="+key, func(t *testing.T) {
			p, ch := makeActivePrompt()

			handled, result := p.HandleKey(key)

			require.True(t, handled)
			require.NotNil(t, result)
			assert.False(t, result.Approved)
			assert.False(t, result.LearnPattern)

			resp := <-ch
			assert.False(t, resp.Approved)

			assert.False(t, p.Active)
			assert.Nil(t, p.Response)
		})
	}
}

// TestPermissionPromptHandleKey_Ignore verifies that an unrecognized key while
// the prompt is active returns handled=true but result==nil (the key is consumed
// so the rest of the TUI does not process it, but no decision is made).
func TestPermissionPromptHandleKey_Ignore(t *testing.T) {
	for _, key := range []string{"x", "enter", "tab", "space", "0"} {
		t.Run("key="+key, func(t *testing.T) {
			p, _ := makeActivePrompt()

			handled, result := p.HandleKey(key)

			assert.True(t, handled, "unrecognized key should be consumed")
			assert.Nil(t, result, "unrecognized key must not produce a decision")

			// Prompt must remain active — no decision was made.
			assert.True(t, p.Active)
			assert.NotNil(t, p.Response)
		})
	}
}

// TestPermissionPromptHandleKey_Inactive verifies that when Active=false,
// HandleKey returns handled=false regardless of the key pressed.
func TestPermissionPromptHandleKey_Inactive(t *testing.T) {
	p := &PermissionPrompt{
		Active:   false,
		Request:  nil,
		Response: nil,
	}

	for _, key := range []string{"a", "l", "d", "n", "esc", "x"} {
		t.Run("key="+key, func(t *testing.T) {
			handled, result := p.HandleKey(key)
			assert.False(t, handled, "inactive prompt must not consume keys")
			assert.Nil(t, result)
		})
	}
}
