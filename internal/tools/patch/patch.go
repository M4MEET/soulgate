// Package patch implements the apply_patch tool for multi-file patching.
package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/tools/toolpath"
)

// PatchResult summarises the outcome of Apply.
type PatchResult struct {
	FilesModified int      `json:"files_modified"`
	FilesCreated  int      `json:"files_created"`
	FilesDeleted  int      `json:"files_deleted"`
	FilesMoved    int      `json:"files_moved"`
	Errors        []string `json:"errors,omitempty"`
}

type patchAction string

const (
	actionAdd    patchAction = "add"
	actionUpdate patchAction = "update"
	actionDelete patchAction = "delete"
	actionMove   patchAction = "move"
)

type filePatch struct {
	action  patchAction
	path    string
	moveTo  string
	content string
	hunks   []hunk
}

type hunk struct {
	find   string
	remove []string
	add    []string
}

// Apply parses and applies a structured patch within workspaceRoot.
func Apply(workspaceRoot string, patchText string) (*PatchResult, error) {
	patches, err := parsePatch(patchText)
	if err != nil {
		return nil, fmt.Errorf("apply_patch: parse error: %w", err)
	}

	result := &PatchResult{}

	for _, p := range patches {
		if err := validatePath(workspaceRoot, p.path); err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		// Core protection: block patches to protected core files
		if isCoreProtected(p.path) {
			result.Errors = append(result.Errors, fmt.Sprintf("core protection: cannot modify '%s' — protected core file", p.path))
			continue
		}
		if p.moveTo != "" {
			if err := validatePath(workspaceRoot, p.moveTo); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			if isCoreProtected(p.moveTo) {
				result.Errors = append(result.Errors, fmt.Sprintf("core protection: cannot move to '%s' — protected core path", p.moveTo))
				continue
			}
		}

		switch p.action {
		case actionAdd:
			fullPath := filepath.Join(workspaceRoot, p.path)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("create dirs for %s: %v", p.path, err))
				continue
			}
			if err := os.WriteFile(fullPath, []byte(p.content), 0o644); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("write %s: %v", p.path, err))
				continue
			}
			result.FilesCreated++

		case actionUpdate:
			fullPath := filepath.Join(workspaceRoot, p.path)
			data, err := os.ReadFile(fullPath)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("read %s: %v", p.path, err))
				continue
			}
			content := string(data)
			hunkErr := false
			for _, h := range p.hunks {
				content, err = applyHunk(content, h)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("hunk in %s: %v", p.path, err))
					hunkErr = true
					break
				}
			}
			if !hunkErr {
				if writeErr := os.WriteFile(fullPath, []byte(content), 0o644); writeErr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("write %s: %v", p.path, writeErr))
					continue
				}
				result.FilesModified++
			}

		case actionDelete:
			fullPath := filepath.Join(workspaceRoot, p.path)
			if err := os.Remove(fullPath); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("delete %s: %v", p.path, err))
				continue
			}
			result.FilesDeleted++

		case actionMove:
			oldPath := filepath.Join(workspaceRoot, p.path)
			newPath := filepath.Join(workspaceRoot, p.moveTo)
			if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("create dirs for %s: %v", p.moveTo, err))
				continue
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("move %s -> %s: %v", p.path, p.moveTo, err))
				continue
			}
			result.FilesMoved++
		}
	}

	return result, nil
}

// isCoreProtected checks if a path targets SoulGate's protected core.
func isCoreProtected(relPath string) bool {
	normalized := filepath.ToSlash(filepath.Clean(relPath))
	protectedDirs := []string{"internal", "cmd"}
	for _, dir := range protectedDirs {
		if normalized == dir || strings.HasPrefix(normalized, dir+"/") {
			return true
		}
	}
	protectedFiles := []string{"go.mod", "go.sum", "Makefile", "main.go"}
	for _, file := range protectedFiles {
		if normalized == file {
			return true
		}
	}
	return false
}

func validatePath(root, rel string) error {
	_, err := toolpath.Resolve(root, rel)
	return err
}

func parsePatch(text string) ([]filePatch, error) {
	lines := strings.Split(text, "\n")
	var patches []filePatch
	var current *filePatch
	inPatch := false
	var contentLines []string
	var currentHunk *hunk

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "*** Begin Patch" {
			inPatch = true
			continue
		}
		if trimmed == "*** End Patch" {
			if current != nil {
				finalizePatch(current, contentLines, currentHunk)
				patches = append(patches, *current)
			}
			break
		}
		if !inPatch {
			continue
		}

		if strings.HasPrefix(trimmed, "*** Add File:") {
			if current != nil {
				finalizePatch(current, contentLines, currentHunk)
				patches = append(patches, *current)
			}
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Add File:"))
			current = &filePatch{action: actionAdd, path: path}
			contentLines = nil
			currentHunk = nil
			continue
		}
		if strings.HasPrefix(trimmed, "*** Update File:") {
			if current != nil {
				finalizePatch(current, contentLines, currentHunk)
				patches = append(patches, *current)
			}
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Update File:"))
			current = &filePatch{action: actionUpdate, path: path}
			contentLines = nil
			currentHunk = nil
			continue
		}
		if strings.HasPrefix(trimmed, "*** Delete File:") {
			if current != nil {
				finalizePatch(current, contentLines, currentHunk)
				patches = append(patches, *current)
			}
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Delete File:"))
			patches = append(patches, filePatch{action: actionDelete, path: path})
			current = nil
			contentLines = nil
			currentHunk = nil
			continue
		}
		if strings.HasPrefix(trimmed, "*** Move File:") {
			if current != nil {
				finalizePatch(current, contentLines, currentHunk)
				patches = append(patches, *current)
			}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Move File:"))
			parts := strings.SplitN(rest, "->", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid move syntax: %s", trimmed)
			}
			patches = append(patches, filePatch{
				action: actionMove,
				path:   strings.TrimSpace(parts[0]),
				moveTo: strings.TrimSpace(parts[1]),
			})
			current = nil
			contentLines = nil
			currentHunk = nil
			continue
		}

		if current == nil {
			continue
		}

		if current.action == actionAdd {
			contentLines = append(contentLines, line)
		} else if current.action == actionUpdate {
			if strings.HasPrefix(trimmed, "@@@") && strings.HasSuffix(trimmed, "@@@") {
				if currentHunk != nil {
					current.hunks = append(current.hunks, *currentHunk)
				}
				find := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "@@@"), "@@@"))
				currentHunk = &hunk{find: find}
			} else if currentHunk != nil {
				if strings.HasPrefix(line, "- ") {
					currentHunk.remove = append(currentHunk.remove, line[2:])
				} else if strings.HasPrefix(line, "+ ") {
					currentHunk.add = append(currentHunk.add, line[2:])
				}
			}
		}
	}

	return patches, nil
}

func finalizePatch(p *filePatch, contentLines []string, currentHunk *hunk) {
	if p.action == actionAdd && len(contentLines) > 0 {
		p.content = strings.Join(contentLines, "\n")
	}
	if p.action == actionUpdate && currentHunk != nil {
		p.hunks = append(p.hunks, *currentHunk)
	}
}

// ToolSchemas returns the JSON tool schema definition for the apply_patch tool.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "apply_patch",
			"description": "Apply a structured multi-file patch. Can add, update, delete, and move files in a single operation.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"patch": map[string]interface{}{
						"type":        "string",
						"description": "The patch content using *** Begin Patch / *** End Patch format",
					},
				},
				"required": []string{"patch"},
			},
		},
	}
}

// ExecuteTool dispatches apply_patch tool calls to Apply and returns the
// PatchResult as a JSON string.
func ExecuteTool(ctx context.Context, workspaceRoot string, name string, args map[string]interface{}) (string, error) {
	_ = ctx
	if name != "apply_patch" {
		return "", fmt.Errorf("patch: unknown tool %q", name)
	}
	patchText, ok := args["patch"].(string)
	if !ok || patchText == "" {
		return "", fmt.Errorf("apply_patch: missing required argument 'patch'")
	}
	result, err := Apply(workspaceRoot, patchText)
	if err != nil {
		return "", err
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}

func applyHunk(content string, h hunk) (string, error) {
	idx := strings.Index(content, h.find)
	if idx < 0 {
		return content, fmt.Errorf("context not found: %q", h.find)
	}

	lineEnd := strings.Index(content[idx:], "\n")
	if lineEnd < 0 {
		lineEnd = len(content)
	} else {
		lineEnd += idx
	}

	remaining := content[lineEnd:]
	for _, rm := range h.remove {
		rmIdx := strings.Index(remaining, rm)
		if rmIdx >= 0 {
			after := remaining[rmIdx+len(rm):]
			if len(after) > 0 && after[0] == '\n' {
				after = after[1:]
			}
			remaining = remaining[:rmIdx] + after
		}
	}

	addText := ""
	if len(h.add) > 0 {
		addText = "\n" + strings.Join(h.add, "\n")
	}

	return content[:lineEnd] + addText + remaining, nil
}
