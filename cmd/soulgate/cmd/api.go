package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start SoulGate HTTP API server",
	Long: `Start an HTTP API server that exposes the full SoulGate orchestrator.

This is how external integrations (Telegram bot, web UI, etc.) connect to SoulGate.
The API runs the same agentic loop as the CLI/TUI — tools, memory, plugins, audit, everything.

Endpoints:
  POST /api/chat     Send a message, get an AI response
  GET  /api/health   Health check`,
	Run: runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.Flags().StringP("port", "p", "8080", "Port to run the API server on")
	apiCmd.Flags().StringP("host", "H", "localhost", "Host to bind the API server to")
}

type chatRequest struct {
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"`
}

type chatResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// ── Cron types ────────────────────────────────────────────────────────────────

type cronJob struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Schedule  string `json:"schedule"`
	Task      string `json:"task"`
	Status    string `json:"status"` // active | paused | error
	LastRun   string `json:"last_run,omitempty"`
	NextRun   string `json:"next_run,omitempty"`
	CreatedAt string `json:"created_at"`
}

type cronStore struct {
	mu   sync.RWMutex
	jobs map[string]*cronJob
}

func newCronStore() *cronStore {
	return &cronStore{jobs: make(map[string]*cronJob)}
}

func (s *cronStore) list() []*cronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*cronJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		cp := *j
		out = append(out, &cp)
	}
	return out
}

func (s *cronStore) add(j *cronJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
}

func (s *cronStore) get(id string) (*cronJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	cp := *j
	return &cp, true
}

func (s *cronStore) delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.jobs[id]
	if ok {
		delete(s.jobs, id)
	}
	return ok
}

func (s *cronStore) toggle(id string) (*cronJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	if j.Status == "active" {
		j.Status = "paused"
	} else {
		j.Status = "active"
	}
	cp := *j
	return &cp, true
}

func (s *cronStore) markRun(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return
	}
	j.LastRun = time.Now().UTC().Format(time.RFC3339)
}

// idFromPath extracts the path segment after a given prefix.
// e.g. idFromPath("/api/cron/", "/api/cron/abc123/toggle") → "abc123"
func idFromPath(prefix, path string) string {
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}

func runAPI(cmd *cobra.Command, args []string) {
	port, _ := cmd.Flags().GetString("port")
	host, _ := cmd.Flags().GetString("host")

	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		log.Fatalf("Failed to load workspace: %v", err)
	}

	// Initialize the full orchestrator (tools, memory, plugins, audit, everything)
	orch, err := core.NewOrchestrator(workspace)
	if err != nil {
		log.Fatalf("Failed to initialize orchestrator: %v", err)
	}

	mux := http.NewServeMux()

	// Chat endpoint — runs the full agentic loop
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req chatRequest
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, chatResponse{Error: "Failed to read request body"})
			return
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, chatResponse{Error: "Invalid JSON"})
			return
		}
		if req.Message == "" {
			writeJSON(w, http.StatusBadRequest, chatResponse{Error: "Message is required"})
			return
		}

		log.Printf("📨 /api/chat: %q", req.Message)

		// Run through the full orchestrator — same as CLI/TUI
		ctx := r.Context()
		result, err := orch.Run(ctx, req.Message)
		if err != nil {
			log.Printf("❌ Orchestrator error: %v", err)
			writeJSON(w, http.StatusInternalServerError, chatResponse{Error: fmt.Sprintf("AI error: %v", err)})
			return
		}

		response := ""
		if result != nil {
			response = result.Response
		}
		log.Printf("✅ Response: %d chars", len(response))
		writeJSON(w, http.StatusOK, chatResponse{Response: response})
	})

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		provider, model := orch.GetCurrentProvider()
		writeJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"provider": provider,
			"model":    model,
		})
	})

	// ── Cron endpoints ────────────────────────────────────────────────────────

	crons := newCronStore()

	// GET /api/cron — list all jobs
	// POST /api/cron — create a job
	mux.HandleFunc("/api/cron", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, "":
			writeJSON(w, http.StatusOK, map[string]interface{}{"jobs": crons.list()})

		case http.MethodPost:
			var payload struct {
				Name     string `json:"name"`
				Schedule string `json:"schedule"`
				Task     string `json:"task"`
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
				return
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
				return
			}
			if payload.Name == "" || payload.Schedule == "" || payload.Task == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, schedule, and task are required"})
				return
			}
			job := &cronJob{
				ID:        uuid.New().String(),
				Name:      payload.Name,
				Schedule:  payload.Schedule,
				Task:      payload.Task,
				Status:    "active",
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}
			crons.add(job)
			log.Printf("Cron job created: %s (%s)", job.Name, job.Schedule)
			writeJSON(w, http.StatusCreated, job)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// DELETE /api/cron/{id}       — delete a job
	// POST   /api/cron/{id}/toggle — pause or resume
	// POST   /api/cron/{id}/run    — run immediately (via orchestrator)
	mux.HandleFunc("/api/cron/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasSuffix(path, "/toggle") && r.Method == http.MethodPost {
			id := idFromPath("/api/cron/", strings.TrimSuffix(path, "/toggle"))
			job, ok := crons.toggle(id)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
				return
			}
			writeJSON(w, http.StatusOK, job)
			return
		}

		if strings.HasSuffix(path, "/run") && r.Method == http.MethodPost {
			id := idFromPath("/api/cron/", strings.TrimSuffix(path, "/run"))
			job, ok := crons.get(id)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
				return
			}
			// Run the task asynchronously through the orchestrator
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if _, err := orch.Run(ctx, job.Task); err != nil {
					log.Printf("Cron job %q run error: %v", job.Name, err)
				}
				crons.markRun(id)
			}()
			writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
			return
		}

		if r.Method == http.MethodDelete {
			id := idFromPath("/api/cron/", path)
			if !crons.delete(id) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	handler := corsMiddleware(mux)

	addr := fmt.Sprintf("%s:%s", host, port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No write timeout — agentic loops can take minutes
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("Shutting down API server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	fmt.Printf("🚀 SoulGate API server running on http://%s\n", addr)
	fmt.Printf("   POST /api/chat   — send messages\n")
	fmt.Printf("   GET  /api/health — health check\n")
	fmt.Printf("   Provider: %s\n", workspace.Config.Model.DefaultProvider)
	fmt.Println()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
