package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ThinkingBudget
// ---------------------------------------------------------------------------

func TestThinkingBudget(t *testing.T) {
	cases := []struct {
		level    ThinkingLevel
		expected int
	}{
		{ThinkOff, 0},
		{ThinkMinimal, 512},
		{ThinkLow, 2048},
		{ThinkMedium, 4096},
		{ThinkHigh, 8192},
		{ThinkMax, 16384},
		{ThinkAdaptive, -1},
	}

	for _, tc := range cases {
		d := DefaultDirectives()
		d.ThinkingLevel = tc.level
		assert.Equal(t, tc.expected, d.ThinkingBudget(), "level=%s", tc.level)
	}
}

func TestThinkingBudgetUnknownLevel(t *testing.T) {
	d := DefaultDirectives()
	d.ThinkingLevel = ThinkingLevel("bogus")
	// Unknown levels fall through to the default -1 (adaptive).
	assert.Equal(t, -1, d.ThinkingBudget())
}

// ---------------------------------------------------------------------------
// Directives.String
// ---------------------------------------------------------------------------

func TestDirectivesString(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		d := DefaultDirectives()
		assert.Equal(t, "defaults", d.String())
	})

	t.Run("think level set", func(t *testing.T) {
		d := DefaultDirectives()
		d.ThinkingLevel = ThinkHigh
		assert.Equal(t, "think:high", d.String())
	})

	t.Run("fast mode", func(t *testing.T) {
		d := DefaultDirectives()
		d.FastMode = true
		assert.Equal(t, "fast", d.String())
	})

	t.Run("verbose on", func(t *testing.T) {
		d := DefaultDirectives()
		d.VerboseMode = "on"
		assert.Equal(t, "verbose:on", d.String())
	})

	t.Run("reasoning stream", func(t *testing.T) {
		d := DefaultDirectives()
		d.ReasoningShow = "stream"
		assert.Equal(t, "reasoning:stream", d.String())
	})

	t.Run("multiple active", func(t *testing.T) {
		d := DefaultDirectives()
		d.ThinkingLevel = ThinkMedium
		d.FastMode = true
		d.VerboseMode = "full"
		d.ReasoningShow = "on"
		s := d.String()
		assert.Contains(t, s, "think:medium")
		assert.Contains(t, s, "fast")
		assert.Contains(t, s, "verbose:full")
		assert.Contains(t, s, "reasoning:on")
	})
}

// ---------------------------------------------------------------------------
// /think directive
// ---------------------------------------------------------------------------

func TestParseThinkDirective(t *testing.T) {
	cases := []struct {
		input    string
		expected ThinkingLevel
	}{
		{"/think high", ThinkHigh},
		{"/think low", ThinkLow},
		{"/think medium", ThinkMedium},
		{"/think off", ThinkOff},
		{"/think minimal", ThinkMinimal},
		{"/think xhigh", ThinkMax},
		{"/think adaptive", ThinkAdaptive},
	}

	for _, tc := range cases {
		d := DefaultDirectives()
		_, applied := ParseDirectives(tc.input, d)
		require.Len(t, applied, 1, "input=%q", tc.input)
		assert.Equal(t, tc.expected, d.ThinkingLevel, "input=%q", tc.input)
	}
}

func TestParseThinkDirectiveInvalidLevel(t *testing.T) {
	d := DefaultDirectives()
	original := d.ThinkingLevel
	_, applied := ParseDirectives("/think turbo", d)
	assert.Empty(t, applied, "unknown level should not be applied")
	assert.Equal(t, original, d.ThinkingLevel, "level should not change on invalid input")
}

func TestParseThinkDirectiveMissingArg(t *testing.T) {
	d := DefaultDirectives()
	original := d.ThinkingLevel
	_, applied := ParseDirectives("/think", d)
	assert.Empty(t, applied)
	assert.Equal(t, original, d.ThinkingLevel)
}

// ---------------------------------------------------------------------------
// /fast directive
// ---------------------------------------------------------------------------

func TestParseFastDirective(t *testing.T) {
	t.Run("fast on", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/fast on", d)
		require.Len(t, applied, 1)
		assert.True(t, d.FastMode)
	})

	t.Run("fast off", func(t *testing.T) {
		d := DefaultDirectives()
		d.FastMode = true
		_, applied := ParseDirectives("/fast off", d)
		require.Len(t, applied, 1)
		assert.False(t, d.FastMode)
	})

	t.Run("bare /fast toggles off→on", func(t *testing.T) {
		d := DefaultDirectives() // FastMode starts false
		_, applied := ParseDirectives("/fast", d)
		require.Len(t, applied, 1)
		assert.True(t, d.FastMode)
	})

	t.Run("bare /fast toggles on→off", func(t *testing.T) {
		d := DefaultDirectives()
		d.FastMode = true
		_, applied := ParseDirectives("/fast", d)
		require.Len(t, applied, 1)
		assert.False(t, d.FastMode)
	})

	t.Run("invalid argument", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/fast maybe", d)
		assert.Empty(t, applied)
	})
}

// ---------------------------------------------------------------------------
// /verbose directive
// ---------------------------------------------------------------------------

func TestParseVerboseDirective(t *testing.T) {
	modes := []string{"off", "on", "full"}
	for _, mode := range modes {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/verbose "+mode, d)
		require.Len(t, applied, 1, "mode=%s", mode)
		assert.Equal(t, mode, d.VerboseMode, "mode=%s", mode)
	}

	t.Run("bare /verbose defaults to on", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/verbose", d)
		require.Len(t, applied, 1)
		assert.Equal(t, "on", d.VerboseMode)
	})

	t.Run("invalid mode", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/verbose chatty", d)
		assert.Empty(t, applied)
	})
}

// ---------------------------------------------------------------------------
// /reasoning directive
// ---------------------------------------------------------------------------

func TestParseReasoningDirective(t *testing.T) {
	modes := []string{"off", "on", "stream"}
	for _, mode := range modes {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/reasoning "+mode, d)
		require.Len(t, applied, 1, "mode=%s", mode)
		assert.Equal(t, mode, d.ReasoningShow, "mode=%s", mode)
	}

	t.Run("bare /reasoning defaults to on", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/reasoning", d)
		require.Len(t, applied, 1)
		assert.Equal(t, "on", d.ReasoningShow)
	})
}

// ---------------------------------------------------------------------------
// /temperature directive
// ---------------------------------------------------------------------------

func TestParseTemperature(t *testing.T) {
	t.Run("valid values", func(t *testing.T) {
		validCases := []struct {
			input    string
			expected float64
		}{
			{"/temperature 0.0", 0.0},
			{"/temperature 0.7", 0.7},
			{"/temperature 1.0", 1.0},
			{"/temperature 2.0", 2.0},
			{"/temperature 1.5", 1.5},
		}
		for _, tc := range validCases {
			d := DefaultDirectives()
			_, applied := ParseDirectives(tc.input, d)
			require.Len(t, applied, 1, "input=%q", tc.input)
			assert.InDelta(t, tc.expected, d.Temperature, 0.001, "input=%q", tc.input)
		}
	})

	t.Run("out of range values are rejected", func(t *testing.T) {
		invalid := []string{"/temperature -0.1", "/temperature 2.1", "/temperature 3"}
		for _, s := range invalid {
			d := DefaultDirectives()
			_, applied := ParseDirectives(s, d)
			assert.Empty(t, applied, "should reject: %s", s)
			assert.Equal(t, -1.0, d.Temperature, "temperature should not change: %s", s)
		}
	})

	t.Run("non-numeric rejected", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/temperature hot", d)
		assert.Empty(t, applied)
	})

	t.Run("missing arg", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/temperature", d)
		assert.Empty(t, applied)
	})
}

// ---------------------------------------------------------------------------
// /maxtokens directive
// ---------------------------------------------------------------------------

func TestParseMaxTokens(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/maxtokens 4096", d)
		require.Len(t, applied, 1)
		assert.Equal(t, 4096, d.MaxTokens)
	})

	t.Run("zero rejected", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/maxtokens 0", d)
		assert.Empty(t, applied)
	})

	t.Run("negative rejected", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/maxtokens -100", d)
		assert.Empty(t, applied)
	})

	t.Run("non-numeric rejected", func(t *testing.T) {
		d := DefaultDirectives()
		_, applied := ParseDirectives("/maxtokens lots", d)
		assert.Empty(t, applied)
	})
}

// ---------------------------------------------------------------------------
// Message cleaning
// ---------------------------------------------------------------------------

func TestDirectivesStripped(t *testing.T) {
	msg := "/think high\n/fast on\nPlease summarise the file."
	d := DefaultDirectives()
	cleaned, applied := ParseDirectives(msg, d)

	assert.Equal(t, "Please summarise the file.", cleaned)
	assert.Len(t, applied, 2)
	assert.Equal(t, ThinkHigh, d.ThinkingLevel)
	assert.True(t, d.FastMode)
}

func TestMixedDirectivesAndText(t *testing.T) {
	msg := "Some context here.\n/think medium\nMore text below.\n/verbose full\nFinal line."
	d := DefaultDirectives()
	cleaned, applied := ParseDirectives(msg, d)

	assert.Equal(t, "Some context here.\nMore text below.\nFinal line.", cleaned)
	assert.Len(t, applied, 2)
	assert.Equal(t, ThinkMedium, d.ThinkingLevel)
	assert.Equal(t, "full", d.VerboseMode)
}

func TestNoDirectivesLeavesMessageIntact(t *testing.T) {
	msg := "Just a regular message with no directives."
	d := DefaultDirectives()
	cleaned, applied := ParseDirectives(msg, d)

	assert.Equal(t, msg, cleaned)
	assert.Empty(t, applied)
}

func TestUnknownSlashCommandLeftInMessage(t *testing.T) {
	// A slash command that doesn't match any directive should stay in the message.
	msg := "/unknown-command arg\nReal content."
	d := DefaultDirectives()
	cleaned, applied := ParseDirectives(msg, d)

	assert.Contains(t, cleaned, "/unknown-command arg")
	assert.Contains(t, cleaned, "Real content.")
	assert.Empty(t, applied)
}

func TestEmptyMessageRemainsEmpty(t *testing.T) {
	d := DefaultDirectives()
	cleaned, applied := ParseDirectives("", d)
	assert.Equal(t, "", cleaned)
	assert.Empty(t, applied)
}

func TestOnlyDirectivesProducesEmptyCleaned(t *testing.T) {
	msg := "/think low\n/fast on"
	d := DefaultDirectives()
	cleaned, applied := ParseDirectives(msg, d)
	assert.Equal(t, "", cleaned)
	assert.Len(t, applied, 2)
}

// ---------------------------------------------------------------------------
// ApplyToRequest
// ---------------------------------------------------------------------------

func TestApplyToRequestWithOverrides(t *testing.T) {
	d := DefaultDirectives()
	d.Temperature = 0.8
	d.MaxTokens = 2048
	d.ThinkingLevel = ThinkHigh
	d.FastMode = true

	req := make(map[string]interface{})
	d.ApplyToRequest(req)

	assert.Equal(t, true, req["fast_mode"])
	assert.InDelta(t, 0.8, req["temperature"], 0.001)
	assert.Equal(t, 2048, req["max_tokens"])
	assert.Equal(t, 8192, req["thinking_budget"])
}

func TestApplyToRequestDefaults(t *testing.T) {
	d := DefaultDirectives()
	req := make(map[string]interface{})
	d.ApplyToRequest(req)

	// Temperature -1 must not be written.
	_, hastemp := req["temperature"]
	assert.False(t, hastemp)

	// MaxTokens 0 must not be written.
	_, hasmax := req["max_tokens"]
	assert.False(t, hasmax)

	// FastMode false must not be written.
	_, hasfast := req["fast_mode"]
	assert.False(t, hasfast)

	// Adaptive thinking → budget=-1 → must not be written.
	_, hasbudget := req["thinking_budget"]
	assert.False(t, hasbudget)
}

func TestApplyToRequestThinkOff(t *testing.T) {
	d := DefaultDirectives()
	d.ThinkingLevel = ThinkOff

	req := make(map[string]interface{})
	d.ApplyToRequest(req)

	// ThinkOff budget is 0, which is >= 0, so it must be written.
	assert.Equal(t, 0, req["thinking_budget"])
}
