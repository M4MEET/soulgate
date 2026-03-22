package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeActiveDialog returns a ConfirmationDialog that is Active and has a
// non-nil PendingAction so that HandleKey exercises the full code path.
func makeActiveDialog() *ConfirmationDialog {
	return &ConfirmationDialog{
		Active:  true,
		Message: "Are you sure you want to delete everything?",
		Command: "rm -rf /",
		PendingAction: func() tea.Cmd {
			return nil
		},
	}
}

// TestConfirmationHandleKey_Yes verifies that pressing "y" (or "Y") while the
// dialog is active returns handled=true and confirmed=true, and deactivates
// the dialog.
func TestConfirmationHandleKey_Yes(t *testing.T) {
	for _, key := range []string{"y", "Y"} {
		t.Run("key="+key, func(t *testing.T) {
			d := makeActiveDialog()

			handled, confirmed := d.HandleKey(key)

			require.True(t, handled)
			assert.True(t, confirmed)

			// Dialog must be deactivated after a decision.
			assert.False(t, d.Active)
			assert.Nil(t, d.PendingAction)
		})
	}
}

// TestConfirmationHandleKey_No verifies that pressing "n" (or "N") returns
// handled=true and confirmed=false.
func TestConfirmationHandleKey_No(t *testing.T) {
	for _, key := range []string{"n", "N"} {
		t.Run("key="+key, func(t *testing.T) {
			d := makeActiveDialog()

			handled, confirmed := d.HandleKey(key)

			require.True(t, handled)
			assert.False(t, confirmed)
			assert.False(t, d.Active)
		})
	}
}

// TestConfirmationHandleKey_Esc verifies that pressing "esc" returns
// handled=true and confirmed=false (escape is treated as cancel).
func TestConfirmationHandleKey_Esc(t *testing.T) {
	d := makeActiveDialog()

	handled, confirmed := d.HandleKey("esc")

	require.True(t, handled)
	assert.False(t, confirmed)
	assert.False(t, d.Active)
}

// TestConfirmationHandleKey_Inactive verifies that when Active=false,
// HandleKey returns handled=false and confirmed=false regardless of the key.
func TestConfirmationHandleKey_Inactive(t *testing.T) {
	d := &ConfirmationDialog{
		Active:  false,
		Message: "some message",
		Command: "some-cmd",
	}

	for _, key := range []string{"y", "n", "esc", "a"} {
		t.Run("key="+key, func(t *testing.T) {
			handled, confirmed := d.HandleKey(key)
			assert.False(t, handled, "inactive dialog must not consume keys")
			assert.False(t, confirmed)
		})
	}
}
