package components

import (
	"sort"
	"strings"
	"unicode"
)

// FuzzyResult holds a matched item and its relevance score.
// Higher scores indicate a better match.
type FuzzyResult struct {
	Text  string
	Score int
}

// FuzzyMatch performs fuzzy matching of pattern against target.
// It returns true when every rune in pattern appears, in order, somewhere
// within target, along with a score that reflects match quality.
//
// Scoring contributions (accumulated per matched character):
//   - Exact prefix position (character i of pattern matches position i of target): +10
//   - Consecutive run (this match immediately follows the previous match): +5
//   - Word-boundary position (character in target preceded by '/', '-', '_', or space): +3
//   - Case-sensitive match (rune equality without folding): +1
//   - Non-consecutive match not at a boundary: +0
func FuzzyMatch(pattern, target string) (bool, int) {
	if pattern == "" {
		// Empty pattern matches everything with a neutral score.
		return true, 0
	}

	patRunes := []rune(pattern)
	tgtRunes := []rune(target)

	if len(patRunes) > len(tgtRunes) {
		return false, 0
	}

	score := 0
	patIdx := 0
	prevMatchPos := -2 // sentinel: no previous match

	for tgtIdx := 0; tgtIdx < len(tgtRunes) && patIdx < len(patRunes); tgtIdx++ {
		pr := patRunes[patIdx]
		tr := tgtRunes[tgtIdx]

		// Case-insensitive equality check is the gate for a match.
		if !runesEqualFold(pr, tr) {
			continue
		}

		// The runes match (case-insensitively). Now accumulate score.

		// Exact prefix: both pattern position and target position are the same index.
		if patIdx == tgtIdx {
			score += 10
		}

		// Consecutive: this target position immediately follows the last matched position.
		if tgtIdx == prevMatchPos+1 {
			score += 5
		}

		// Word boundary: the character before this position is a separator.
		if tgtIdx > 0 && isWordBoundary(tgtRunes[tgtIdx-1]) {
			score += 3
		}

		// Case-sensitive bonus: the runes are identical without folding.
		if pr == tr {
			score += 1
		}

		prevMatchPos = tgtIdx
		patIdx++
	}

	if patIdx < len(patRunes) {
		// Not all pattern characters were consumed.
		return false, 0
	}

	return true, score
}

// FuzzyFilter returns the subset of items that fuzzy-match pattern, sorted by
// descending score (best match first). Items that do not match are excluded.
// When pattern is empty every item is returned, preserving original order.
func FuzzyFilter(pattern string, items []string) []FuzzyResult {
	if pattern == "" {
		results := make([]FuzzyResult, len(items))
		for i, item := range items {
			results[i] = FuzzyResult{Text: item, Score: 0}
		}
		return results
	}

	var results []FuzzyResult
	for _, item := range items {
		matched, score := FuzzyMatch(pattern, item)
		if matched {
			results = append(results, FuzzyResult{Text: item, Score: score})
		}
	}

	// Stable sort: higher score first; ties keep original order.
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// isWordBoundary reports whether r is a separator character that precedes a
// logical "word" start: forward slash, hyphen, underscore, or whitespace.
func isWordBoundary(r rune) bool {
	switch r {
	case '/', '-', '_':
		return true
	}
	return unicode.IsSpace(r)
}

// runesEqualFold reports whether a and b are equal under simple Unicode case
// folding (equivalent to strings.EqualFold for single characters).
func runesEqualFold(a, b rune) bool {
	if a == b {
		return true
	}
	return strings.EqualFold(string(a), string(b))
}
