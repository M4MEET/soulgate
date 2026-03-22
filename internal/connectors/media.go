package connectors

import (
	"path/filepath"
	"regexp"
	"strings"
)

// MediaRef represents a media reference found in AI output.
type MediaRef struct {
	// Type is one of "image", "file", or "audio".
	Type string
	// Path is the local file path on disk.
	Path string
	// Alt is the alt-text or description accompanying the reference.
	Alt string
}

// imageExtensions is the set of file extensions we treat as images.
var imageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
	".svg":  true,
}

// audioExtensions is the set of file extensions we treat as audio.
var audioExtensions = map[string]bool{
	".mp3":  true,
	".ogg":  true,
	".oga":  true,
	".wav":  true,
	".m4a":  true,
	".opus": true,
	".flac": true,
}

// Compiled patterns for media reference markers in AI output.
var (
	// [image: /path/to/file.png] or [img: /path/to/file.png]
	reTaggedImage = regexp.MustCompile(`(?i)\[(?:image|img):\s*([^\]]+)\]`)

	// [file: /path/to/file.ext]
	reTaggedFile = regexp.MustCompile(`(?i)\[file:\s*([^\]]+)\]`)

	// [audio: /path/to/file.mp3]
	reTaggedAudio = regexp.MustCompile(`(?i)\[audio:\s*([^\]]+)\]`)

	// Markdown image syntax: ![alt text](path)
	reMarkdownImage = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
)

// ExtractMediaRefs scans AI response text for media references and returns
// them in the order they first appear.  It does not deduplicate.
func ExtractMediaRefs(text string) []MediaRef {
	var refs []MediaRef

	// [image: path] / [img: path]
	for _, m := range reTaggedImage.FindAllStringSubmatch(text, -1) {
		path := strings.TrimSpace(m[1])
		refs = append(refs, MediaRef{Type: "image", Path: path})
	}

	// [file: path]
	for _, m := range reTaggedFile.FindAllStringSubmatch(text, -1) {
		path := strings.TrimSpace(m[1])
		refs = append(refs, MediaRef{Type: "file", Path: path})
	}

	// [audio: path]
	for _, m := range reTaggedAudio.FindAllStringSubmatch(text, -1) {
		path := strings.TrimSpace(m[1])
		refs = append(refs, MediaRef{Type: "audio", Path: path})
	}

	// ![alt](path) — only when path ends in a known image extension
	for _, m := range reMarkdownImage.FindAllStringSubmatch(text, -1) {
		alt := m[1]
		path := strings.TrimSpace(m[2])
		if isImagePath(path) {
			refs = append(refs, MediaRef{Type: "image", Path: path, Alt: alt})
		}
	}

	// Bare path: the entire trimmed response is a single file/image/audio path.
	// This catches cases where the AI just returns a path as its whole answer.
	trimmed := strings.TrimSpace(text)
	if len(refs) == 0 && isBareFilePath(trimmed) {
		ext := strings.ToLower(filepath.Ext(trimmed))
		mediaType := classifyExt(ext)
		refs = append(refs, MediaRef{Type: mediaType, Path: trimmed})
	}

	return refs
}

// CleanMediaRefs removes media reference markers from text so that the
// remaining plain text can be sent as a caption or standalone message.
func CleanMediaRefs(text string) string {
	text = reTaggedImage.ReplaceAllString(text, "")
	text = reTaggedFile.ReplaceAllString(text, "")
	text = reTaggedAudio.ReplaceAllString(text, "")
	text = reMarkdownImage.ReplaceAllStringFunc(text, func(m string) string {
		// Only strip if the path is an image (not a web URL embedded in prose).
		sub := reMarkdownImage.FindStringSubmatch(m)
		if sub != nil && isImagePath(strings.TrimSpace(sub[2])) {
			return ""
		}
		return m
	})
	// Collapse multiple blank lines left behind by removed markers.
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

// isImagePath reports whether path ends in a known image extension and is not
// a plain HTTP/HTTPS URL (which should be left as prose).
func isImagePath(path string) bool {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	return imageExtensions[strings.ToLower(filepath.Ext(path))]
}

// isBareFilePath reports whether s looks like a standalone local file path
// (starts with "/" or "./" and has a non-empty extension).
func isBareFilePath(s string) bool {
	if strings.ContainsAny(s, "\n\r") {
		return false
	}
	if !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "./") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(s))
	return ext != "" && (imageExtensions[ext] || audioExtensions[ext] ||
		// common document extensions
		ext == ".pdf" || ext == ".zip" || ext == ".tar" || ext == ".gz" ||
		ext == ".txt" || ext == ".csv" || ext == ".json" || ext == ".xml" ||
		ext == ".docx" || ext == ".xlsx" || ext == ".pptx")
}

// classifyExt returns the MediaRef.Type for a lowercase file extension.
func classifyExt(ext string) string {
	if imageExtensions[ext] {
		return "image"
	}
	if audioExtensions[ext] {
		return "audio"
	}
	return "file"
}
