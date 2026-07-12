package toolpath

import (
	"path/filepath"
	"testing"
)

func TestResolveAllowsWorkspacePaths(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"out.mp3", "audio/clip.wav", "./nested/../img.png"} {
		got, err := Resolve(root, rel)
		if err != nil {
			t.Errorf("Resolve(%q) unexpected error: %v", rel, err)
			continue
		}
		if want := filepath.Join(root, filepath.Clean(rel)); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", rel, got, want)
		}
	}
}

func TestResolveBlocksEscapes(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"",
		"..",
		"../evil.mp3",
		"../../etc/passwd",
		"audio/../../outside.png",
		"/etc/passwd",
		"/tmp/x.wav",
	} {
		if got, err := Resolve(root, rel); err == nil {
			t.Errorf("Resolve(%q) = %q, want error", rel, got)
		}
	}
}
