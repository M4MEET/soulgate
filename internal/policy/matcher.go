package policy

import (
	"strings"

	"github.com/gobwas/glob"
)

// Matcher handles pattern matching for policies
type Matcher struct {
	cache map[string]glob.Glob
}

// NewMatcher creates a new pattern matcher
func NewMatcher() *Matcher {
	return &Matcher{
		cache: make(map[string]glob.Glob),
	}
}

// MatchAction checks if an action matches a pattern
func (m *Matcher) MatchAction(pattern, action string) bool {
	// Exact match
	if pattern == action {
		return true
	}

	// Wildcard match
	if pattern == "*" {
		return true
	}

	// Category wildcard (e.g., "files.*" matches "files.read")
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(action, prefix+".")
	}

	return false
}

// MatchResource checks if a resource matches a glob pattern
func (m *Matcher) MatchResource(pattern, resource string) (bool, error) {
	// Get or compile glob pattern
	g, ok := m.cache[pattern]
	if !ok {
		var err error
		g, err = glob.Compile(pattern, '/')
		if err != nil {
			return false, err
		}
		m.cache[pattern] = g
	}

	// Normalize paths for comparison
	normalizedResource := normalizePath(resource)
	normalizedPattern := normalizePath(pattern)

	// Check if pattern is absolute
	isPatternAbsolute := strings.HasPrefix(normalizedPattern, "/")
	isResourceAbsolute := strings.HasPrefix(normalizedResource, "/")

	// If both are absolute or both are relative, do direct match
	if isPatternAbsolute == isResourceAbsolute {
		return g.Match(normalizedResource), nil
	}

	// Handle mixed absolute/relative
	// If pattern is absolute and resource is relative, no match
	if isPatternAbsolute && !isResourceAbsolute {
		return false, nil
	}

	// If resource is absolute and pattern is relative, treat pattern as workspace-relative
	// This shouldn't normally happen in our use case, but handle it gracefully
	return g.Match(normalizedResource), nil
}

// normalizePath normalizes a path for comparison
func normalizePath(path string) string {
	// Remove redundant slashes
	path = strings.ReplaceAll(path, "//", "/")

	// Trim trailing slashes
	path = strings.TrimSuffix(path, "/")

	return path
}
