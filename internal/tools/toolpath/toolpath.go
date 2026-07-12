// Package toolpath resolves model-supplied relative paths against the
// workspace root, enforcing the workspace boundary. Tool inputs come from
// the LLM and are never trusted: absolute paths and ../ traversal are
// rejected rather than silently re-rooted.
package toolpath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Resolve joins rel onto workspaceRoot and returns the absolute path.
// It fails if rel is absolute or escapes the workspace via traversal.
func Resolve(workspaceRoot, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is empty")
	}
	cleaned := filepath.Clean(rel)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path %q must be relative to the workspace", rel)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", rel)
	}

	rootClean := filepath.Clean(workspaceRoot)
	abs := filepath.Clean(filepath.Join(rootClean, cleaned))
	workspaceRel, err := filepath.Rel(rootClean, abs)
	if err != nil || workspaceRel == ".." || strings.HasPrefix(workspaceRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", rel)
	}
	return abs, nil
}
