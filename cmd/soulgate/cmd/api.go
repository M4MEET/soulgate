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
	"syscall"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/core"
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
