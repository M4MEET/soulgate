package canvas

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ArtifactType enumerates the supported canvas artifact types.
type ArtifactType string

const (
	TypeHTML    ArtifactType = "html"
	TypeReact   ArtifactType = "react"
	TypeSVG     ArtifactType = "svg"
	TypeMermaid ArtifactType = "mermaid"
)

// validTypes is the set of accepted artifact type values.
var validTypes = map[ArtifactType]bool{
	TypeHTML:    true,
	TypeReact:   true,
	TypeSVG:     true,
	TypeMermaid: true,
}

// Artifact represents a single canvas artifact saved to disk.
type Artifact struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Type        ArtifactType `json:"type"`
	Content     string       `json:"content"`
	Description string       `json:"description"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	FilePath    string       `json:"file_path"`
}

// Manager manages canvas artifacts on disk.
type Manager struct {
	dir       string // absolute path to .soulgate/canvas/
	artifacts map[string]*Artifact
	mu        sync.RWMutex
}

// NewManager creates a Manager rooted at dir, creating the directory if needed.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("canvas: failed to create directory %q: %w", dir, err)
	}

	m := &Manager{
		dir:       dir,
		artifacts: make(map[string]*Artifact),
	}

	if err := m.loadIndex(); err != nil {
		return nil, fmt.Errorf("canvas: failed to load index: %w", err)
	}

	return m, nil
}

// Create saves a new artifact to disk and returns it.
func (m *Manager) Create(title string, typ ArtifactType, content, description string) (*Artifact, error) {
	if !validTypes[typ] {
		return nil, fmt.Errorf("canvas: unsupported type %q (valid: html, react, svg, mermaid)", typ)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("canvas: failed to generate artifact id: %w", err)
	}

	now := time.Now()
	a := &Artifact{
		ID:          id,
		Title:       title,
		Type:        typ,
		Content:     content,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
		FilePath:    filepath.Join(m.dir, id+".html"),
	}

	rendered, err := renderArtifact(a)
	if err != nil {
		return nil, fmt.Errorf("canvas: failed to render artifact: %w", err)
	}

	if err := os.WriteFile(a.FilePath, []byte(rendered), 0o644); err != nil {
		return nil, fmt.Errorf("canvas: failed to write artifact: %w", err)
	}

	m.mu.Lock()
	m.artifacts[id] = a
	m.mu.Unlock()

	if err := m.saveIndex(); err != nil {
		return nil, fmt.Errorf("canvas: failed to save index: %w", err)
	}

	return a, nil
}

// Update overwrites an existing artifact's content and re-renders to disk.
func (m *Manager) Update(id, content string) (*Artifact, error) {
	m.mu.Lock()
	a, ok := m.artifacts[id]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("canvas: artifact %q not found", id)
	}

	a.Content = content
	a.UpdatedAt = time.Now()
	m.mu.Unlock()

	rendered, err := renderArtifact(a)
	if err != nil {
		return nil, fmt.Errorf("canvas: failed to render artifact: %w", err)
	}

	if err := os.WriteFile(a.FilePath, []byte(rendered), 0o644); err != nil {
		return nil, fmt.Errorf("canvas: failed to write artifact: %w", err)
	}

	if err := m.saveIndex(); err != nil {
		return nil, fmt.Errorf("canvas: failed to save index: %w", err)
	}

	return a, nil
}

// Get returns the artifact with the given ID, or an error if not found.
func (m *Manager) Get(id string) (*Artifact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.artifacts[id]
	if !ok {
		return nil, fmt.Errorf("canvas: artifact %q not found", id)
	}
	return a, nil
}

// List returns all known artifacts.
func (m *Manager) List() []*Artifact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Artifact, 0, len(m.artifacts))
	for _, a := range m.artifacts {
		out = append(out, a)
	}
	return out
}

// ---------------------------------------------------------------------------
// Index persistence
// ---------------------------------------------------------------------------

// indexPath returns the path of the JSON index file.
func (m *Manager) indexPath() string {
	return filepath.Join(m.dir, "index.json")
}

// loadIndex reads the artifact index from disk. Missing index is not an error.
func (m *Manager) loadIndex() error {
	data, err := os.ReadFile(m.indexPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read index: %w", err)
	}

	var list []*Artifact
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse index: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range list {
		m.artifacts[a.ID] = a
	}
	return nil
}

// saveIndex writes the artifact index atomically to disk.
func (m *Manager) saveIndex() error {
	m.mu.RLock()
	list := make([]*Artifact, 0, len(m.artifacts))
	for _, a := range m.artifacts {
		list = append(list, a)
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	tmp := m.indexPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}
	if err := os.Rename(tmp, m.indexPath()); err != nil {
		return fmt.Errorf("rename index: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ID generation
// ---------------------------------------------------------------------------

func generateID() (string, error) {
	b := make([]byte, 6) // 12 hex chars — short but collision-resistant enough
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ---------------------------------------------------------------------------
// HTML rendering
// ---------------------------------------------------------------------------

// renderArtifact converts an artifact into a self-contained HTML file.
func renderArtifact(a *Artifact) (string, error) {
	switch a.Type {
	case TypeHTML:
		return a.Content, nil

	case TypeReact:
		return renderReact(a.Title, a.Content), nil

	case TypeSVG:
		return renderSVG(a.Title, a.Content), nil

	case TypeMermaid:
		return renderMermaid(a.Title, a.Content), nil

	default:
		return "", fmt.Errorf("unsupported artifact type: %q", a.Type)
	}
}

func renderReact(title, jsxContent string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + escapeHTML(title) + `</title>
  <script src="https://unpkg.com/react@18/umd/react.production.min.js"></script>
  <script src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js"></script>
  <script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>
  <style>
    body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; }
  </style>
</head>
<body>
  <div id="root"></div>
  <script type="text/babel">
` + jsxContent + `
  </script>
</body>
</html>`
}

func renderSVG(title, svgContent string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + escapeHTML(title) + `</title>
  <style>
    body { margin: 0; display: flex; justify-content: center; align-items: center; min-height: 100vh; background: #f5f5f5; }
    svg { max-width: 100%; height: auto; }
  </style>
</head>
<body>
` + svgContent + `
</body>
</html>`
}

func renderMermaid(title, diagramContent string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + escapeHTML(title) + `</title>
  <script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>
  <style>
    body { margin: 0; display: flex; justify-content: center; align-items: center; min-height: 100vh; background: #fff; padding: 2rem; box-sizing: border-box; }
    .mermaid { max-width: 100%; }
  </style>
</head>
<body>
  <div class="mermaid">
` + diagramContent + `
  </div>
  <script>mermaid.initialize({ startOnLoad: true });</script>
</body>
</html>`
}

// escapeHTML escapes the five characters that are special in HTML attribute and
// text contexts. This is intentionally minimal — we only need it for the title.
func escapeHTML(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			out = append(out, '&', 'a', 'm', 'p', ';')
		case '<':
			out = append(out, '&', 'l', 't', ';')
		case '>':
			out = append(out, '&', 'g', 't', ';')
		case '"':
			out = append(out, '&', 'q', 'u', 'o', 't', ';')
		case '\'':
			out = append(out, '&', '#', '3', '9', ';')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
