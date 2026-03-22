package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Assets returns the web UI filesystem rooted at the dist/ directory.
// Falls back to the legacy flat files if dist/ doesn't exist (dev mode).
func Assets() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
