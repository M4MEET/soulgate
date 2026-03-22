package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── DetectPII ─────────────────────────────────────────────────────────────────

func TestDetectPII_Email(t *testing.T) {
	matches := DetectPII("Send report to alice@example.com please")
	found := false
	for _, m := range matches {
		if m.Type == "email" && m.Value == "alice@example.com" {
			found = true
		}
	}
	assert.True(t, found, "expected email match")
}

func TestDetectPII_Phone(t *testing.T) {
	matches := DetectPII("Call 555-867-5309 for help")
	found := false
	for _, m := range matches {
		if m.Type == "phone" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestDetectPII_SSN(t *testing.T) {
	matches := DetectPII("SSN: 123-45-6789")
	found := false
	for _, m := range matches {
		if m.Type == "ssn" && m.Value == "123-45-6789" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestDetectPII_CreditCard(t *testing.T) {
	matches := DetectPII("Card: 4111 1111 1111 1111")
	found := false
	for _, m := range matches {
		if m.Type == "credit_card" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestDetectPII_IPAddress(t *testing.T) {
	matches := DetectPII("Server is at 192.168.1.100")
	found := false
	for _, m := range matches {
		if m.Type == "ip_address" && m.Value == "192.168.1.100" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestDetectPII_NoMatches(t *testing.T) {
	matches := DetectPII("Nothing sensitive here.")
	assert.Empty(t, matches)
}

func TestDetectPII_Empty(t *testing.T) {
	matches := DetectPII("")
	assert.Empty(t, matches)
}

func TestDetectPII_Multiple(t *testing.T) {
	matches := DetectPII("Email user@test.com and call 800-555-0100")
	types := map[string]bool{}
	for _, m := range matches {
		types[m.Type] = true
	}
	assert.True(t, types["email"])
	assert.True(t, types["phone"])
}

func TestDetectPII_MatchPositions(t *testing.T) {
	text := "Email: alice@example.com end"
	matches := DetectPII(text)
	for _, m := range matches {
		if m.Type == "email" {
			assert.Equal(t, "alice@example.com", text[m.Start:m.End])
		}
	}
}

// ── RedactPII ─────────────────────────────────────────────────────────────────

func TestRedactPII_Email(t *testing.T) {
	out := RedactPII("Contact user@corp.io for info")
	assert.Contains(t, out, "[EMAIL]")
	assert.NotContains(t, out, "user@corp.io")
}

func TestRedactPII_Phone(t *testing.T) {
	out := RedactPII("Call 800-555-0100 now")
	assert.Contains(t, out, "[PHONE]")
	assert.NotContains(t, out, "800-555-0100")
}

func TestRedactPII_SSN(t *testing.T) {
	out := RedactPII("My SSN is 123-45-6789.")
	assert.Contains(t, out, "[SSN]")
	assert.NotContains(t, out, "123-45-6789")
}

func TestRedactPII_CreditCard(t *testing.T) {
	out := RedactPII("Pay with 4111-1111-1111-1111")
	assert.Contains(t, out, "[CREDIT_CARD]")
}

func TestRedactPII_IPAddress(t *testing.T) {
	out := RedactPII("Host 10.0.0.1 is down")
	assert.Contains(t, out, "[IP_ADDRESS]")
	assert.NotContains(t, out, "10.0.0.1")
}

func TestRedactPII_NoMatches(t *testing.T) {
	in := "Nothing sensitive here."
	assert.Equal(t, in, RedactPII(in))
}

func TestRedactPII_Empty(t *testing.T) {
	assert.Equal(t, "", RedactPII(""))
}

func TestRedactPII_MultipleInOneLine(t *testing.T) {
	out := RedactPII("Email user@test.com or call 555-123-4567")
	assert.Contains(t, out, "[EMAIL]")
	assert.Contains(t, out, "[PHONE]")
	assert.NotContains(t, out, "user@test.com")
	assert.NotContains(t, out, "555-123-4567")
}

func TestRedactPII_PlaceholderIsLonger(t *testing.T) {
	// Verify the output length is reasonable (placeholders replace originals).
	in := "user@example.com"
	out := RedactPII(in)
	assert.Equal(t, "[EMAIL]", out)
}
