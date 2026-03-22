package canvas

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// previewServer is a short-lived HTTP server that serves a single artifact file.
type previewServer struct {
	srv      *http.Server
	listener net.Listener
	port     int
	stopOnce sync.Once
}

// startPreviewServer starts an HTTP server on a random free port that serves
// the artifact at filePath. It returns the server and the URL at which the
// artifact is reachable.
//
// The server shuts itself down automatically after ttl elapses. Pass a zero
// ttl to keep the server running until the caller explicitly calls stop().
func startPreviewServer(filePath string, ttl time.Duration) (*previewServer, string, error) {
	// Bind to port 0 so the OS assigns a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", fmt.Errorf("canvas: failed to listen: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	// Serve the single artifact file at the root path.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeFile(w, r, filePath)
	})

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	ps := &previewServer{
		srv:      srv,
		listener: ln,
		port:     port,
	}

	go func() {
		_ = srv.Serve(ln) // returns when the server is shut down
	}()

	// Auto-shutdown after ttl if a non-zero duration was requested.
	if ttl > 0 {
		go func() {
			timer := time.NewTimer(ttl)
			defer timer.Stop()
			<-timer.C
			ps.stop()
		}()
	}

	url := fmt.Sprintf("http://localhost:%d/", port)
	return ps, url, nil
}

// stop shuts the preview server down gracefully. Safe to call multiple times.
func (ps *previewServer) stop() {
	ps.stopOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = ps.srv.Shutdown(ctx)
	})
}

// PreviewManager tracks all active preview servers so the process does not
// accumulate leaked goroutines over time.
type PreviewManager struct {
	mu      sync.Mutex
	servers map[string]*previewServer // artifact ID -> server
}

// NewPreviewManager creates an empty preview manager.
func NewPreviewManager() *PreviewManager {
	return &PreviewManager{
		servers: make(map[string]*previewServer),
	}
}

// defaultPreviewTTL is how long a preview server stays alive before
// shutting itself down automatically.
const defaultPreviewTTL = 30 * time.Minute

// StartPreview starts (or restarts) a preview server for the given artifact.
// Returns the URL at which the artifact can be viewed in a browser.
func (pm *PreviewManager) StartPreview(artifactID, filePath string) (string, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Shut down any existing server for this artifact.
	if existing, ok := pm.servers[artifactID]; ok {
		existing.stop()
		delete(pm.servers, artifactID)
	}

	ps, url, err := startPreviewServer(filePath, defaultPreviewTTL)
	if err != nil {
		return "", err
	}

	pm.servers[artifactID] = ps
	return url, nil
}

// StopAll shuts every active preview server. Intended for process cleanup.
func (pm *PreviewManager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for id, ps := range pm.servers {
		ps.stop()
		delete(pm.servers, id)
	}
}
