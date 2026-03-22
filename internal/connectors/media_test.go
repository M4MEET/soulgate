package connectors

import (
	"testing"
)

func TestExtractMediaRefs_TaggedImage(t *testing.T) {
	refs := ExtractMediaRefs("Here is the result [image: /tmp/output.png] enjoy!")
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Type != "image" || refs[0].Path != "/tmp/output.png" {
		t.Errorf("unexpected ref: %+v", refs[0])
	}
}

func TestExtractMediaRefs_TaggedImgAlias(t *testing.T) {
	refs := ExtractMediaRefs("[img: ./chart.jpg]")
	if len(refs) != 1 || refs[0].Type != "image" || refs[0].Path != "./chart.jpg" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestExtractMediaRefs_TaggedFile(t *testing.T) {
	refs := ExtractMediaRefs("Download: [file: /reports/report.pdf]")
	if len(refs) != 1 || refs[0].Type != "file" || refs[0].Path != "/reports/report.pdf" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestExtractMediaRefs_TaggedAudio(t *testing.T) {
	refs := ExtractMediaRefs("[audio: /tmp/speech.mp3]")
	if len(refs) != 1 || refs[0].Type != "audio" || refs[0].Path != "/tmp/speech.mp3" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestExtractMediaRefs_MarkdownImage(t *testing.T) {
	refs := ExtractMediaRefs("See the plot: ![bar chart](./plot.png)")
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Type != "image" || refs[0].Path != "./plot.png" || refs[0].Alt != "bar chart" {
		t.Errorf("unexpected ref: %+v", refs[0])
	}
}

func TestExtractMediaRefs_MarkdownImageHTTPSkipped(t *testing.T) {
	// HTTP URLs inside markdown images must not be extracted as local paths.
	refs := ExtractMediaRefs("![logo](https://example.com/logo.png)")
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs for HTTP URL, got %d: %+v", len(refs), refs)
	}
}

func TestExtractMediaRefs_BareImagePath(t *testing.T) {
	refs := ExtractMediaRefs("/tmp/result.jpg")
	if len(refs) != 1 || refs[0].Type != "image" || refs[0].Path != "/tmp/result.jpg" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestExtractMediaRefs_BareFilePath(t *testing.T) {
	refs := ExtractMediaRefs("./output.pdf")
	if len(refs) != 1 || refs[0].Type != "file" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestExtractMediaRefs_NoMedia(t *testing.T) {
	refs := ExtractMediaRefs("Just plain text with no media references here.")
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs, got %d: %+v", len(refs), refs)
	}
}

func TestCleanMediaRefs_RemovesTaggedImage(t *testing.T) {
	input := "Here is the result [image: /tmp/output.png] and some more text."
	got := CleanMediaRefs(input)
	want := "Here is the result  and some more text."
	if got != want {
		t.Errorf("CleanMediaRefs(%q) = %q, want %q", input, got, want)
	}
}

func TestCleanMediaRefs_RemovesMarkdownImage(t *testing.T) {
	input := "Plot:\n\n![chart](./data.png)\n\nExplanation follows."
	got := CleanMediaRefs(input)
	if got != "Plot:\n\nExplanation follows." {
		t.Errorf("unexpected cleaned text: %q", got)
	}
}

func TestCleanMediaRefs_PreservesHTTPMarkdownImage(t *testing.T) {
	// HTTP URL markdown images in prose should not be removed.
	input := "Visit ![site](https://example.com/img.png) for details."
	got := CleanMediaRefs(input)
	if got != input {
		t.Errorf("should not have removed HTTP image: %q", got)
	}
}

func TestCleanMediaRefs_MultipleRefs(t *testing.T) {
	input := "[image: /a.png] some text [file: /b.pdf] end"
	got := CleanMediaRefs(input)
	want := "some text  end"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
