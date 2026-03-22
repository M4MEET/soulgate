package files

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validatePath validates and cleans a file path, ensuring it stays within workspace boundaries
func validatePath(workspaceRoot, requestedPath string) (string, error) {
	// Clean the path to remove . and .. components
	cleaned := filepath.Clean(requestedPath)

	// Resolve to absolute path
	var absPath string
	if filepath.IsAbs(cleaned) {
		absPath = cleaned
	} else {
		// Treat as relative to workspace root
		absPath = filepath.Join(workspaceRoot, cleaned)
	}

	// Ensure the resolved path is absolute and cleaned
	absPath = filepath.Clean(absPath)

	// Resolve symlinks to prevent symlink-based traversal attacks
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist yet, that's okay for write operations
		// Just verify the parent directory exists or use the path as-is
		parent := filepath.Dir(absPath)
		if parent != absPath { // Not root
			resolved, err = filepath.EvalSymlinks(parent)
			if err != nil {
				// Parent doesn't exist either, use original path for validation
				resolved = absPath
			} else {
				// Reconstruct path with resolved parent
				resolved = filepath.Join(resolved, filepath.Base(absPath))
			}
		} else {
			resolved = absPath
		}
	}

	// SECURITY: Enforce workspace boundary to prevent path traversal
	// Ensure the resolved path is within or equal to workspace root
	if !isWithinWorkspace(workspaceRoot, resolved) {
		return "", fmt.Errorf("access denied: path outside workspace boundary")
	}

	return resolved, nil
}

// isWithinWorkspace checks if a path is within the workspace boundary
func isWithinWorkspace(workspaceRoot, targetPath string) bool {
	// Both paths should be absolute and cleaned
	root := filepath.Clean(workspaceRoot)
	target := filepath.Clean(targetPath)

	// The target path must start with the workspace root
	// Use filepath.Rel to check if target is within root
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}

	// If the relative path starts with "..", it's outside the workspace
	if strings.HasPrefix(rel, "..") {
		return false
	}

	return true
}

// protectedCoreDirs are directories the AI must not write to or delete from.
// These contain the SoulGate core runtime — modifications could crash the system.
// The AI can still READ these files (for introspection/bug reports).
var protectedCoreDirs = []string{
	"internal",
	"cmd",
}

// protectedCoreFiles are specific files the AI must not write to or delete.
var protectedCoreFiles = []string{
	"go.mod",
	"go.sum",
	"Makefile",
	"main.go",
}

// isCoreProtected checks if a relative path falls within the protected core.
// Returns true if the path should be blocked from write/delete operations.
func isCoreProtected(relPath string) bool {
	// Normalize separators
	normalized := filepath.ToSlash(relPath)

	// Check protected directories
	for _, dir := range protectedCoreDirs {
		if normalized == dir || strings.HasPrefix(normalized, dir+"/") {
			return true
		}
	}

	// Check protected files (exact match on the base or relative path)
	for _, file := range protectedCoreFiles {
		if normalized == file {
			return true
		}
	}

	return false
}

// getRelativePath returns a path relative to workspace root
func getRelativePath(workspaceRoot, absPath string) (string, error) {
	// Get relative path for display/policy evaluation
	rel, err := filepath.Rel(workspaceRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	// If relative path starts with .., it's outside workspace (should not happen after validatePath)
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path outside workspace boundary")
	}

	return rel, nil
}
