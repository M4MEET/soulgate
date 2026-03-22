package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/a2a"
)

// SetupA2A initializes the A2A server with the given executor and skills.
// Call this after NewGateway and before Start.
func (g *Gateway) SetupA2A(executor a2a.TaskExecutor, skills []a2a.AgentSkill) {
	baseURL := fmt.Sprintf("http://localhost:%d", g.config.Port)

	providerName := g.config.Provider
	if providerName == "" {
		providerName = "SoulGate"
	}

	g.a2aServer = a2a.NewServer(a2a.ServerConfig{
		AgentName:   "SoulGate",
		Description: "Personal AI agent with full system access — files, shell, web, memory, agents, and more.",
		Version:     "1.0.0",
		BaseURL:     baseURL,
		Provider: &a2a.AgentProvider{
			Organization: "SoulGate",
			URL:          baseURL,
		},
		Skills:   skills,
		Executor: executor,
		Store:    a2a.NewTaskStore(g.config.ConfigDir),
	})
}

// GetA2AServer returns the A2A server instance.
func (g *Gateway) GetA2AServer() *a2a.Server {
	return g.a2aServer
}

// GetA2ARegistry returns the A2A agent registry.
func (g *Gateway) GetA2ARegistry() *a2a.AgentRegistry {
	return g.a2aRegistry
}

// registerA2ARoutes adds A2A protocol routes to the mux.
func (g *Gateway) registerA2ARoutes(mux *http.ServeMux, apiHandler func(http.HandlerFunc) http.Handler) {
	if g.a2aServer != nil {
		// Register A2A protocol endpoints (agent card + JSON-RPC + REST)
		g.a2aServer.RegisterRoutes(mux)
		fmt.Println("A2A protocol enabled: /.well-known/agent.json, /a2a/*")
	}

	// Management API for remote agents (always available)
	mux.Handle("/api/a2a/agents", apiHandler(g.handleA2AAgents))
	mux.Handle("/api/a2a/agents/", apiHandler(g.handleA2AAgentDetail))
	mux.Handle("/api/a2a/tasks", apiHandler(g.handleA2ATasks))
	mux.Handle("/api/a2a/tasks/", apiHandler(g.handleA2ATaskDetail))
	mux.Handle("/api/a2a/send", apiHandler(g.handleA2ASend))
	mux.Handle("/api/a2a/card", apiHandler(g.handleA2ACard))
}

// --------------------------------------------------------------------------
// Management API Handlers
// --------------------------------------------------------------------------

// handleA2ACard returns this agent's card.
func (g *Gateway) handleA2ACard(w http.ResponseWriter, r *http.Request) {
	if g.a2aServer == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"configured": false,
			"message":    "A2A server not configured",
		})
		return
	}
	writeGatewayJSON(w, http.StatusOK, g.a2aServer.GetCard())
}

// handleA2AAgents manages the remote agent registry.
func (g *Gateway) handleA2AAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		agents := g.a2aRegistry.List()
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
			"agents": agents,
			"count":  len(agents),
		})

	case http.MethodPost:
		var body struct {
			URL    string `json:"url"`
			APIKey string `json:"apiKey,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if body.URL == "" {
			http.Error(w, `{"error":"url is required"}`, http.StatusBadRequest)
			return
		}
		agent, err := g.a2aRegistry.Add(body.URL)
		if err != nil {
			writeGatewayJSON(w, http.StatusBadGateway, map[string]string{
				"error": fmt.Sprintf("failed to discover agent: %v", err),
			})
			return
		}
		writeGatewayJSON(w, http.StatusCreated, agent)

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleA2AAgentDetail handles GET/DELETE for a specific remote agent.
func (g *Gateway) handleA2AAgentDetail(w http.ResponseWriter, r *http.Request) {
	agentURL := strings.TrimPrefix(r.URL.Path, "/api/a2a/agents/")
	// URL-decode the agent URL (it may be double-encoded)
	agentURL, _ = decodeA2AURL(agentURL)

	switch r.Method {
	case http.MethodGet:
		agent, ok := g.a2aRegistry.Get(agentURL)
		if !ok {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
		writeGatewayJSON(w, http.StatusOK, agent)

	case http.MethodDelete:
		g.a2aRegistry.Remove(agentURL)
		writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "removed"})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleA2ATasks lists A2A tasks.
func (g *Gateway) handleA2ATasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if g.a2aServer == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"tasks": []interface{}{}, "count": 0})
		return
	}

	contextID := r.URL.Query().Get("contextId")
	tasks := g.a2aServer.GetStore().List(contextID, 100)
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// handleA2ATaskDetail handles GET/POST for a specific task.
func (g *Gateway) handleA2ATaskDetail(w http.ResponseWriter, r *http.Request) {
	if g.a2aServer == nil {
		http.Error(w, `{"error":"A2A not configured"}`, http.StatusServiceUnavailable)
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/a2a/tasks/")
	parts := strings.SplitN(rest, "/", 2)
	taskID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		task, ok := g.a2aServer.GetStore().Get(taskID)
		if !ok {
			http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
			return
		}
		writeGatewayJSON(w, http.StatusOK, task)

	case action == "cancel" && r.Method == http.MethodPost:
		if err := g.a2aServer.GetStore().Cancel(taskID); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusBadRequest)
			return
		}
		task, _ := g.a2aServer.GetStore().Get(taskID)
		writeGatewayJSON(w, http.StatusOK, task)

	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

// handleA2ASend sends a message to a remote A2A agent.
func (g *Gateway) handleA2ASend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		AgentURL string `json:"agentUrl"`
		Message  string `json:"message"`
		APIKey   string `json:"apiKey,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if body.AgentURL == "" || body.Message == "" {
		http.Error(w, `{"error":"agentUrl and message are required"}`, http.StatusBadRequest)
		return
	}

	client := a2a.NewClient(body.AgentURL)
	if body.APIKey != "" {
		client.WithAPIKey(body.APIKey)
	}

	msg := a2a.Message{
		MessageID: a2aMessageID(),
		Role:      "user",
		Parts:     []a2a.Part{a2a.TextPart(body.Message)},
	}

	task, err := client.SendMessage(msg)
	if err != nil {
		writeGatewayJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("failed to send to remote agent: %v", err),
		})
		return
	}

	writeGatewayJSON(w, http.StatusOK, task)
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func decodeA2AURL(encoded string) (string, error) {
	// Simple URL reconstruction: the agent URL is the rest of the path
	// If it starts with http, use as-is; otherwise prepend https://
	if strings.HasPrefix(encoded, "http") {
		return encoded, nil
	}
	return "https://" + encoded, nil
}

func a2aMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
