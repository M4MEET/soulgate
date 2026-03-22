package patch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// tmpDir creates a temporary directory and registers a cleanup to remove it.
func tmpDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "patch-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// writeFile creates a file inside dir with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent dirs for %q: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %q: %v", name, err)
	}
}

// readFile returns the content of a file inside dir.
func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("failed to read %q: %v", name, err)
	}
	return string(data)
}

// --------------------------------------------------------------------------
// parsePatch tests
// We test the parser indirectly: Apply is called against a temp dir to verify
// that parsing produced the right operations. For pure parse tests we only
// need a parsePatch call, which is package-internal.
// --------------------------------------------------------------------------

func TestParseAddFile(t *testing.T) {
	patches, err := parsePatch("*** Begin Patch\n*** Add File: hello.go\npackage main\n*** End Patch")
	if err != nil {
		t.Fatalf("parsePatch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	p := patches[0]
	if p.action != actionAdd {
		t.Errorf("expected actionAdd, got %q", p.action)
	}
	if p.path != "hello.go" {
		t.Errorf("expected path hello.go, got %q", p.path)
	}
	if !strings.Contains(p.content, "package main") {
		t.Errorf("content missing expected text; got: %q", p.content)
	}
}

func TestParseUpdateFile(t *testing.T) {
	patchText := "*** Begin Patch\n*** Update File: main.go\n@@@ func main() { @@@\n- fmt.Println(\"old\")\n+ fmt.Println(\"new\")\n*** End Patch"
	patches, err := parsePatch(patchText)
	if err != nil {
		t.Fatalf("parsePatch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	p := patches[0]
	if p.action != actionUpdate {
		t.Errorf("expected actionUpdate, got %q", p.action)
	}
	if p.path != "main.go" {
		t.Errorf("expected path main.go, got %q", p.path)
	}
	if len(p.hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(p.hunks))
	}
	h := p.hunks[0]
	if h.find != "func main() {" {
		t.Errorf("unexpected find context: %q", h.find)
	}
	if len(h.remove) != 1 || h.remove[0] != `fmt.Println("old")` {
		t.Errorf("unexpected remove lines: %v", h.remove)
	}
	if len(h.add) != 1 || h.add[0] != `fmt.Println("new")` {
		t.Errorf("unexpected add lines: %v", h.add)
	}
}

func TestParseDeleteFile(t *testing.T) {
	patches, err := parsePatch("*** Begin Patch\n*** Delete File: old.go\n*** End Patch")
	if err != nil {
		t.Fatalf("parsePatch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	p := patches[0]
	if p.action != actionDelete {
		t.Errorf("expected actionDelete, got %q", p.action)
	}
	if p.path != "old.go" {
		t.Errorf("expected path old.go, got %q", p.path)
	}
}

func TestParseMoveFile(t *testing.T) {
	patches, err := parsePatch("*** Begin Patch\n*** Move File: src/old.go -> src/new.go\n*** End Patch")
	if err != nil {
		t.Fatalf("parsePatch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	p := patches[0]
	if p.action != actionMove {
		t.Errorf("expected actionMove, got %q", p.action)
	}
	if p.path != "src/old.go" {
		t.Errorf("expected path src/old.go, got %q", p.path)
	}
	if p.moveTo != "src/new.go" {
		t.Errorf("expected moveTo src/new.go, got %q", p.moveTo)
	}
}

func TestParseMultipleActions(t *testing.T) {
	patchText := "*** Begin Patch\n*** Add File: added.txt\nhello\n*** Delete File: removed.txt\n*** Move File: a.go -> b.go\n*** End Patch"
	patches, err := parsePatch(patchText)
	if err != nil {
		t.Fatalf("parsePatch: %v", err)
	}
	if len(patches) != 3 {
		t.Fatalf("expected 3 patches, got %d", len(patches))
	}
	if patches[0].action != actionAdd {
		t.Errorf("patch[0] action: want add, got %q", patches[0].action)
	}
	if patches[1].action != actionDelete {
		t.Errorf("patch[1] action: want delete, got %q", patches[1].action)
	}
	if patches[2].action != actionMove {
		t.Errorf("patch[2] action: want move, got %q", patches[2].action)
	}
}

// --------------------------------------------------------------------------
// Apply tests
// --------------------------------------------------------------------------

func TestApplyAddFile(t *testing.T) {
	dir := tmpDir(t)

	patchText := "*** Begin Patch\n*** Add File: greet.txt\nhello world\n*** End Patch"
	result, err := Apply(dir, patchText)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result.FilesCreated != 1 {
		t.Errorf("expected FilesCreated=1, got %d", result.FilesCreated)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	content := readFile(t, dir, "greet.txt")
	if !strings.Contains(content, "hello world") {
		t.Errorf("expected file content to contain 'hello world', got: %q", content)
	}
}

func TestApplyAddFileCreatesParentDirs(t *testing.T) {
	dir := tmpDir(t)

	patchText := "*** Begin Patch\n*** Add File: sub/dir/file.txt\ncontent\n*** End Patch"
	result, err := Apply(dir, patchText)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result.FilesCreated != 1 {
		t.Errorf("expected FilesCreated=1, got %d", result.FilesCreated)
	}

	if _, err := os.Stat(filepath.Join(dir, "sub", "dir", "file.txt")); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestApplyUpdateFile(t *testing.T) {
	dir := tmpDir(t)
	// The hunk finder uses strings.Index on the full file content, so write a
	// file where the context string appears verbatim.
	writeFile(t, dir, "app.go", "package main\n\nfunc main() {\n\tfmt.Println(\"old\")\n}\n")

	patchText := "*** Begin Patch\n*** Update File: app.go\n@@@ func main() { @@@\n- \tfmt.Println(\"old\")\n+ \tfmt.Println(\"new\")\n*** End Patch"
	result, err := Apply(dir, patchText)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result.FilesModified != 1 {
		t.Errorf("expected FilesModified=1, got %d", result.FilesModified)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}

	content := readFile(t, dir, "app.go")
	if strings.Contains(content, `"old"`) {
		t.Errorf("old content still present: %q", content)
	}
	if !strings.Contains(content, `"new"`) {
		t.Errorf("new content not found: %q", content)
	}
}

func TestApplyDeleteFile(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "old.txt", "goodbye\n")

	patchText := "*** Begin Patch\n*** Delete File: old.txt\n*** End Patch"
	result, err := Apply(dir, patchText)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result.FilesDeleted != 1 {
		t.Errorf("expected FilesDeleted=1, got %d", result.FilesDeleted)
	}

	if _, err := os.Stat(filepath.Join(dir, "old.txt")); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestApplyMoveFile(t *testing.T) {
	dir := tmpDir(t)
	writeFile(t, dir, "old.txt", "data\n")

	patchText := "*** Begin Patch\n*** Move File: old.txt -> new.txt\n*** End Patch"
	result, err := Apply(dir, patchText)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if result.FilesMoved != 1 {
		t.Errorf("expected FilesMoved=1, got %d", result.FilesMoved)
	}

	if _, err := os.Stat(filepath.Join(dir, "old.txt")); !os.IsNotExist(err) {
		t.Error("expected old.txt to no longer exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Errorf("expected new.txt to exist: %v", err)
	}
}

// --------------------------------------------------------------------------
// Security: path traversal
// --------------------------------------------------------------------------

func TestPathTraversalBlocked(t *testing.T) {
	dir := tmpDir(t)

	traversalCases := []struct {
		name      string
		patchText string
	}{
		{
			name:      "dotdot in add path",
			patchText: "*** Begin Patch\n*** Add File: ../../etc/passwd\nevil\n*** End Patch",
		},
		{
			name:      "dotdot in delete path",
			patchText: "*** Begin Patch\n*** Delete File: ../outside.txt\n*** End Patch",
		},
		{
			name:      "dotdot in move source",
			patchText: "*** Begin Patch\n*** Move File: ../../src.txt -> dst.txt\n*** End Patch",
		},
		{
			name:      "dotdot in move destination",
			patchText: "*** Begin Patch\n*** Move File: src.txt -> ../../dst.txt\n*** End Patch",
		},
		{
			name:      "absolute path in add",
			patchText: "*** Begin Patch\n*** Add File: /etc/passwd\nevil\n*** End Patch",
		},
	}

	for _, tc := range traversalCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Apply(dir, tc.patchText)
			// Either a top-level error or an error entry in result.Errors is acceptable.
			if err != nil {
				return
			}
			if len(result.Errors) == 0 {
				t.Errorf("expected path traversal to be blocked, but no error was reported")
			}
		})
	}
}

// --------------------------------------------------------------------------
// validatePath unit tests (package-internal function)
// --------------------------------------------------------------------------

func TestValidatePath(t *testing.T) {
	root := "/workspace"

	tests := []struct {
		relPath   string
		wantError bool
	}{
		{"file.txt", false},
		{"sub/dir/file.go", false},
		{"../escape.txt", true},
		{"../../etc/passwd", true},
		{"/etc/passwd", true},
		// sub/../file.txt cleans to file.txt which is within root.
		{"sub/../file.txt", false},
	}

	for _, tc := range tests {
		err := validatePath(root, tc.relPath)
		if tc.wantError && err == nil {
			t.Errorf("validatePath(%q, %q): expected error, got nil", root, tc.relPath)
		}
		if !tc.wantError && err != nil {
			t.Errorf("validatePath(%q, %q): unexpected error: %v", root, tc.relPath, err)
		}
	}
}

// --------------------------------------------------------------------------
// applyHunk unit tests (package-internal function)
// --------------------------------------------------------------------------

func TestApplyHunk_Basic(t *testing.T) {
	content := "line1\nfunc foo() {\n\treturn 1\n}\nline5\n"
	h := hunk{
		find:   "func foo() {",
		remove: []string{"\treturn 1"},
		add:    []string{"\treturn 42"},
	}
	result, err := applyHunk(content, h)
	if err != nil {
		t.Fatalf("applyHunk: %v", err)
	}
	if !strings.Contains(result, "return 42") {
		t.Errorf("expected return 42 in result: %q", result)
	}
	if strings.Contains(result, "return 1") {
		t.Errorf("expected return 1 to be removed from result: %q", result)
	}
}

func TestApplyHunk_ContextNotFound(t *testing.T) {
	_, err := applyHunk("some content\n", hunk{find: "nonexistent context"})
	if err == nil {
		t.Error("expected error when context not found")
	}
}

// --------------------------------------------------------------------------
// ToolSchemas sanity check
// --------------------------------------------------------------------------

func TestToolSchemas(t *testing.T) {
	schemas := ToolSchemas()
	if len(schemas) == 0 {
		t.Fatal("ToolSchemas returned empty slice")
	}
	schema := schemas[0]
	if schema["name"] != "apply_patch" {
		t.Errorf("expected tool name apply_patch, got %v", schema["name"])
	}
	inputSchema, ok := schema["input_schema"].(map[string]interface{})
	if !ok {
		t.Fatal("input_schema is not a map")
	}
	props, ok := inputSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties is not a map")
	}
	if _, ok := props["patch"]; !ok {
		t.Error("expected 'patch' property in schema")
	}
}
