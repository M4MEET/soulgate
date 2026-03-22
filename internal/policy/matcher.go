package policy

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gobwas/glob"
)

// compiledGlob wraps a compiled glob and any error that occurred during
// compilation.  Storing both lets us cache compile failures without retrying
// and still surface the original error to callers.
type compiledGlob struct {
	g   glob.Glob
	err error
}

// Matcher handles pattern matching for policies.
// It is safe for concurrent use; compiled glob patterns are cached so each
// distinct pattern string is compiled exactly once.
type Matcher struct {
	mu    sync.RWMutex
	cache map[string]compiledGlob
}

// NewMatcher creates a new pattern matcher
func NewMatcher() *Matcher {
	return &Matcher{
		cache: make(map[string]compiledGlob),
	}
}

// compile returns a compiledGlob for pattern, using a cached copy when
// available.  Each distinct pattern string is compiled at most once.
func (m *Matcher) compile(pattern string) compiledGlob {
	// Fast path: pattern already compiled.
	m.mu.RLock()
	if cg, ok := m.cache[pattern]; ok {
		m.mu.RUnlock()
		return cg
	}
	m.mu.RUnlock()

	// Slow path: compile and cache.
	g, err := glob.Compile(pattern, '/')
	if err != nil {
		err = fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		g = nil
	}
	cg := compiledGlob{g: g, err: err}

	m.mu.Lock()
	m.cache[pattern] = cg
	m.mu.Unlock()

	return cg
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

// MatchResource checks if a resource matches a glob pattern.
// Returns an error only when the pattern itself is malformed; a pattern that
// simply does not match returns (false, nil).
func (m *Matcher) MatchResource(pattern, resource string) (bool, error) {
	cg := m.compile(pattern)
	if cg.err != nil {
		return false, cg.err
	}

	// Normalize paths for comparison
	normalizedResource := normalizePath(resource)
	normalizedPattern := normalizePath(pattern)

	// Check if pattern is absolute
	isPatternAbsolute := strings.HasPrefix(normalizedPattern, "/")
	isResourceAbsolute := strings.HasPrefix(normalizedResource, "/")

	// If both are absolute or both are relative, do direct match
	if isPatternAbsolute == isResourceAbsolute {
		return cg.g.Match(normalizedResource), nil
	}

	// Handle mixed absolute/relative
	// If pattern is absolute and resource is relative, no match
	if isPatternAbsolute && !isResourceAbsolute {
		return false, nil
	}

	// If resource is absolute and pattern is relative, treat pattern as workspace-relative
	// This shouldn't normally happen in our use case, but handle it gracefully
	return cg.g.Match(normalizedResource), nil
}

// normalizePath normalizes a path for comparison
func normalizePath(path string) string {
	// Remove redundant slashes
	path = strings.ReplaceAll(path, "//", "/")

	// Trim trailing slashes
	path = strings.TrimSuffix(path, "/")

	return path
}
