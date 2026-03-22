package policy

import (
	"regexp"
	"strings"
)

// PIIMatch represents a detected PII occurrence in text.
type PIIMatch struct {
	Type  string // "email", "phone", "ssn", "credit_card", "ip_address"
	Value string
	Start int
	End   int
}

// piiPattern pairs a PII type name with its compiled regular expression.
type piiPattern struct {
	typeName string
	re       *regexp.Regexp
}

// piiPatterns is the ordered list of PII detectors applied by DetectPII.
// Ordering matters: more-specific patterns (SSN, credit card) appear before
// the broad phone pattern so that overlapping matches are classified with the
// most precise type first.
var piiPatterns = []piiPattern{
	{
		typeName: "email",
		re:       regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
	},
	{
		typeName: "ssn",
		re:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	},
	{
		typeName: "credit_card",
		re:       regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`),
	},
	{
		typeName: "phone",
		re:       regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`),
	},
	{
		typeName: "ip_address",
		re:       regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
	},
}

// DetectPII scans text and returns all PII matches found.
// Multiple PII types may overlap for the same substring; all are returned.
// Matches are reported in the order: email, ssn, credit_card, phone, ip_address.
func DetectPII(text string) []PIIMatch {
	var matches []PIIMatch

	for _, p := range piiPatterns {
		locs := p.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			matches = append(matches, PIIMatch{
				Type:  p.typeName,
				Value: text[loc[0]:loc[1]],
				Start: loc[0],
				End:   loc[1],
			})
		}
	}

	return matches
}

// redactPlaceholders maps each PII type to the placeholder written into
// the redacted output.
var redactPlaceholders = map[string]string{
	"email":       "[EMAIL]",
	"ssn":         "[SSN]",
	"credit_card": "[CREDIT_CARD]",
	"phone":       "[PHONE]",
	"ip_address":  "[IP_ADDRESS]",
}

// RedactPII replaces all detected PII in text with type-specific placeholders.
// When multiple patterns match the same region the replacement that produces
// the longest redacted span wins (credit card beats phone for a 16-digit
// string, for example).  Patterns are applied in a single left-to-right pass
// to avoid double-substitution artifacts.
func RedactPII(text string) string {
	if text == "" {
		return text
	}

	// Build an interval map: position → (end, placeholder).
	// We keep the longest match at each starting position.
	type redactSpan struct {
		end         int
		placeholder string
	}

	spans := make(map[int]redactSpan)

	for _, p := range piiPatterns {
		placeholder, ok := redactPlaceholders[p.typeName]
		if !ok {
			placeholder = "[REDACTED]"
		}

		locs := p.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			start, end := loc[0], loc[1]
			existing, exists := spans[start]
			if !exists || (end-start) > (existing.end-start) {
				spans[start] = redactSpan{end: end, placeholder: placeholder}
			}
		}
	}

	if len(spans) == 0 {
		return text
	}

	// Walk through the string left-to-right, replacing matched spans.
	var sb strings.Builder
	i := 0
	for i < len(text) {
		if span, ok := spans[i]; ok {
			sb.WriteString(span.placeholder)
			i = span.end
		} else {
			sb.WriteByte(text[i])
			i++
		}
	}

	return sb.String()
}
