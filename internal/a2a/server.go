package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// TaskExecutor is called by the A2A server to actually execute a task.
// It receives the task message and streams events back via the provided channel.
type TaskExecutor func(ctx context.Context, task *Task, events chan<- StreamResponse) error

// ServerConfig configures the A2A server.
type ServerConfig struct {
	AgentName   string
	Description string
	Version     string
	BaseURL     string
	Provider    *AgentProvider
	Skills      []AgentSkill
	IconURL     string
	Executor    TaskExecutor
	Store       *TaskStore
}

// Server implements the A2A protocol server endpoints.
type Server struct {
	config ServerConfig
	store  *TaskStore
	card   AgentCard
}

// NewServer creates a new A2A server.
func NewServer(cfg ServerConfig) *Server {
	if cfg.Store == nil {
		cfg.Store = NewTaskStore("")
	}

	card := AgentCard{
		Name:        cfg.AgentName,
		Description: cfg.Description,
		Version:     cfg.Version,
		URL:         cfg.BaseURL,
		Provider:    cfg.Provider,
		Capabilities: AgentCapabilities{
			Streaming:         true,
			PushNotifications: false,
		},
		Skills:      cfg.Skills,
		InputModes:  []string{"text/plain"},
		OutputModes: []string{"text/plain"},
		IconURL:     cfg.IconURL,
	}

	return &Server{
		config: cfg,
		store:  cfg.Store,
		card:   card,
	}
}

// GetStore returns the task store for external access.
func (s *Server) GetStore() *TaskStore {
	return s.store
}

// GetCard returns the agent card.
func (s *Server) GetCard() AgentCard {
	return s.card
}

// RegisterRoutes adds A2A endpoints to the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/.well-known/agent.json", s.handleAgentCard)
	mux.HandleFunc("/a2a", s.handleJSONRPC)
	mux.HandleFunc("/a2a/message/send", s.handleSendMessage)
	mux.HandleFunc("/a2a/message/stream", s.handleSendStreamingMessage)
	mux.HandleFunc("/a2a/tasks", s.handleListTasks)
	mux.HandleFunc("/a2a/tasks/", s.handleTaskRoutes)
}

// handleAgentCard serves the agent discovery card.
func (s *Server) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(s.card)
}

// handleJSONRPC handles the JSON-RPC 2.0 endpoint.
func (s *Server) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, nil, -32600, "method not allowed")
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "parse error")
		return
	}

	switch req.Method {
	case "SendMessage", "message/send":
		s.rpcSendMessage(w, r, req)
	case "SendStreamingMessage", "message/stream":
		s.rpcSendStreamingMessage(w, r, req)
	case "GetTask", "tasks/get":
		s.rpcGetTask(w, req)
	case "CancelTask", "tasks/cancel":
		s.rpcCancelTask(w, req)
	default:
		s.writeError(w, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

// --------------------------------------------------------------------------
// REST Handlers
// --------------------------------------------------------------------------

func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var params SendMessageParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	task := s.executeTask(r.Context(), params)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"task": task})
}

func (s *Server) handleSendStreamingMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	var params SendMessageParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	task := s.store.Create(params.Message, params.Message.ContextID)

	// Subscribe before starting execution
	sub, err := s.store.Subscribe(task.ID)
	if err != nil {
		http.Error(w, `{"error":"subscribe failed"}`, http.StatusInternalServerError)
		return
	}

	// Start execution in background
	go s.runTask(r.Context(), task)

	// Stream SSE events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Send initial task
	data, _ := json.Marshal(StreamResponse{Task: task})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	for evt := range sub {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	contextID := r.URL.Query().Get("contextId")
	tasks := s.store.List(contextID, 100)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
}

func (s *Server) handleTaskRoutes(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/a2a/tasks/")
	parts := strings.SplitN(rest, "/", 2)
	taskID := parts[0]

	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		// GET /a2a/tasks/{id}
		task, ok := s.store.Get(taskID)
		if !ok {
			http.Error(w, `{"error":"task not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"task": task})

	case action == "cancel" && r.Method == http.MethodPost:
		// POST /a2a/tasks/{id}/cancel
		if err := s.store.Cancel(taskID); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusBadRequest)
			return
		}
		task, _ := s.store.Get(taskID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"task": task})

	case action == "subscribe" && r.Method == http.MethodGet:
		// GET /a2a/tasks/{id}/subscribe (SSE)
		s.handleSubscribe(w, r, taskID)

	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (s *Server) handleSubscribe(w http.ResponseWriter, r *http.Request, taskID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	sub, err := s.store.Subscribe(taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for evt := range sub {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// --------------------------------------------------------------------------
// JSON-RPC Handlers
// --------------------------------------------------------------------------

func (s *Server) rpcSendMessage(w http.ResponseWriter, r *http.Request, req JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
	var params SendMessageParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.writeError(w, req.ID, -32602, "invalid params")
		return
	}

	task := s.executeTask(r.Context(), params)
	s.writeResult(w, req.ID, map[string]interface{}{"task": task})
}

func (s *Server) rpcSendStreamingMessage(w http.ResponseWriter, r *http.Request, req JSONRPCRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, req.ID, -32603, "streaming not supported")
		return
	}

	paramsBytes, _ := json.Marshal(req.Params)
	var params SendMessageParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.writeError(w, req.ID, -32602, "invalid params")
		return
	}

	task := s.store.Create(params.Message, params.Message.ContextID)
	sub, _ := s.store.Subscribe(task.ID)

	go s.runTask(r.Context(), task)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// Stream as JSON-RPC responses
	for evt := range sub {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: evt}
		data, _ := json.Marshal(resp)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func (s *Server) rpcGetTask(w http.ResponseWriter, req JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
	var params GetTaskParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.writeError(w, req.ID, -32602, "invalid params")
		return
	}

	task, ok := s.store.Get(params.TaskID)
	if !ok {
		s.writeError(w, req.ID, ErrTaskNotFound, "task not found")
		return
	}

	s.writeResult(w, req.ID, map[string]interface{}{"task": task})
}

func (s *Server) rpcCancelTask(w http.ResponseWriter, req JSONRPCRequest) {
	paramsBytes, _ := json.Marshal(req.Params)
	var params CancelTaskParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.writeError(w, req.ID, -32602, "invalid params")
		return
	}

	if err := s.store.Cancel(params.TaskID); err != nil {
		s.writeError(w, req.ID, ErrTaskNotCancelable, err.Error())
		return
	}

	task, _ := s.store.Get(params.TaskID)
	s.writeResult(w, req.ID, map[string]interface{}{"task": task})
}

// --------------------------------------------------------------------------
// Task Execution
// --------------------------------------------------------------------------

func (s *Server) executeTask(ctx context.Context, params SendMessageParams) *Task {
	task := s.store.Create(params.Message, params.Message.ContextID)
	s.runTask(ctx, task)

	// Return latest state
	updated, _ := s.store.Get(task.ID)
	return updated
}

func (s *Server) runTask(ctx context.Context, task *Task) {
	s.store.UpdateStatus(task.ID, TaskStateWorking, nil)

	if s.config.Executor == nil {
		s.store.UpdateStatus(task.ID, TaskStateFailed, &Message{
			MessageID: uuid.NewString(),
			Role:      "agent",
			Parts:     []Part{TextPart("no executor configured")},
		})
		return
	}

	events := make(chan StreamResponse, 32)
	go func() {
		defer close(events)
		if err := s.config.Executor(ctx, task, events); err != nil {
			s.store.UpdateStatus(task.ID, TaskStateFailed, &Message{
				MessageID: uuid.NewString(),
				Role:      "agent",
				Parts:     []Part{TextPart(fmt.Sprintf("execution error: %v", err))},
			})
		}
	}()

	for evt := range events {
		if evt.StatusUpdate != nil {
			s.store.UpdateStatus(task.ID, evt.StatusUpdate.Status.State, evt.StatusUpdate.Status.Message)
		}
		if evt.ArtifactUpdate != nil {
			s.store.AddArtifact(task.ID, evt.ArtifactUpdate.Artifact, evt.ArtifactUpdate.Append, evt.ArtifactUpdate.LastChunk)
		}
	}

	// If still working after executor returns, mark completed
	t, ok := s.store.Get(task.ID)
	if ok && t.Status.State == TaskStateWorking {
		s.store.UpdateStatus(task.ID, TaskStateCompleted, nil)
	}
}

// --------------------------------------------------------------------------
// JSON-RPC Helpers
// --------------------------------------------------------------------------

func (s *Server) writeResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	})
}
