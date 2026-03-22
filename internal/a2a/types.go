// Package a2a implements Google's Agent-to-Agent (A2A) protocol for
// cross-system agent interoperability.
//
// Spec: https://google.github.io/A2A/
package a2a

import "time"

// --------------------------------------------------------------------------
// Task States
// --------------------------------------------------------------------------

type TaskState string

const (
	TaskStateSubmitted    TaskState = "submitted"
	TaskStateWorking     TaskState = "working"
	TaskStateCompleted   TaskState = "completed"
	TaskStateFailed      TaskState = "failed"
	TaskStateCanceled    TaskState = "canceled"
	TaskStateInputReq    TaskState = "input-required"
	TaskStateAuthReq     TaskState = "auth-required"
	TaskStateRejected    TaskState = "rejected"
)

func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	}
	return false
}

// --------------------------------------------------------------------------
// Core Types
// --------------------------------------------------------------------------

// Part is a single content element in a message or artifact.
type Part struct {
	Type      string      `json:"type"` // "text", "file", "data"
	Text      string      `json:"text,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	File      *FilePart   `json:"file,omitempty"`
	MediaType string      `json:"mediaType,omitempty"`
	Filename  string      `json:"filename,omitempty"`
}

type FilePart struct {
	URL   string `json:"url,omitempty"`
	Bytes string `json:"bytes,omitempty"` // base64
}

// TextPart is a convenience constructor.
func TextPart(text string) Part {
	return Part{Type: "text", Text: text}
}

// Message is a single turn in a conversation.
type Message struct {
	MessageID string            `json:"messageId"`
	Role      string            `json:"role"` // "user" or "agent"
	Parts     []Part            `json:"parts"`
	TaskID    string            `json:"taskId,omitempty"`
	ContextID string            `json:"contextId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Artifact is a named output produced by a task.
type Artifact struct {
	ArtifactID  string            `json:"artifactId"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Parts       []Part            `json:"parts"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TaskStatus holds the current state of a task.
type TaskStatus struct {
	State     TaskState  `json:"state"`
	Message   *Message   `json:"message,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// Task is the core unit of work in A2A.
type Task struct {
	ID        string            `json:"id"`
	ContextID string            `json:"contextId,omitempty"`
	Status    TaskStatus        `json:"status"`
	Artifacts []Artifact        `json:"artifacts,omitempty"`
	History   []Message         `json:"history,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// --------------------------------------------------------------------------
// Agent Card (Discovery)
// --------------------------------------------------------------------------

// AgentCard is served at /.well-known/agent.json for discovery.
type AgentCard struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	URL          string            `json:"url"`
	Provider     *AgentProvider    `json:"provider,omitempty"`
	Capabilities AgentCapabilities `json:"capabilities"`
	Skills       []AgentSkill      `json:"skills"`
	InputModes   []string          `json:"defaultInputModes"`
	OutputModes  []string          `json:"defaultOutputModes"`
	Security     map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	IconURL      string            `json:"iconUrl,omitempty"`
	DocURL       string            `json:"documentationUrl,omitempty"`
}

type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url"`
}

type AgentCapabilities struct {
	Streaming         bool `json:"streaming"`
	PushNotifications bool `json:"pushNotifications"`
	ExtendedAgentCard bool `json:"extendedAgentCard,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

type SecurityScheme struct {
	Type   string `json:"type"` // "apiKey", "http", "oauth2"
	Scheme string `json:"scheme,omitempty"`
	In     string `json:"in,omitempty"`
	Name   string `json:"name,omitempty"`
}

// --------------------------------------------------------------------------
// JSON-RPC Request / Response
// --------------------------------------------------------------------------

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard A2A error codes
const (
	ErrTaskNotFound          = -32001
	ErrTaskNotCancelable     = -32002
	ErrPushNotSupported      = -32003
	ErrUnsupportedOperation  = -32004
	ErrContentTypeNotSupported = -32005
	ErrInvalidAgentResponse  = -32006
)

// --------------------------------------------------------------------------
// Method-specific Params / Results
// --------------------------------------------------------------------------

// SendMessageParams is the params for SendMessage / SendStreamingMessage.
type SendMessageParams struct {
	Message       Message                `json:"message"`
	Configuration *SendMessageConfig     `json:"configuration,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type SendMessageConfig struct {
	AcceptedOutputModes []string `json:"acceptedOutputModes,omitempty"`
	HistoryLength       *int     `json:"historyLength,omitempty"`
	ReturnImmediately   bool     `json:"returnImmediately,omitempty"`
}

// GetTaskParams is the params for GetTask.
type GetTaskParams struct {
	TaskID        string `json:"id"`
	HistoryLength *int   `json:"historyLength,omitempty"`
}

// CancelTaskParams is the params for CancelTask.
type CancelTaskParams struct {
	TaskID string `json:"id"`
}

// --------------------------------------------------------------------------
// Streaming Events
// --------------------------------------------------------------------------

// TaskStatusUpdateEvent is sent via SSE when a task's status changes.
type TaskStatusUpdateEvent struct {
	TaskID    string     `json:"taskId"`
	ContextID string     `json:"contextId,omitempty"`
	Status    TaskStatus `json:"status"`
}

// TaskArtifactUpdateEvent is sent via SSE when an artifact is produced.
type TaskArtifactUpdateEvent struct {
	TaskID    string   `json:"taskId"`
	ContextID string   `json:"contextId,omitempty"`
	Artifact  Artifact `json:"artifact"`
	Append    bool     `json:"append"`
	LastChunk bool     `json:"lastChunk"`
}

// StreamResponse wraps SSE event payloads.
type StreamResponse struct {
	Task           *Task                    `json:"task,omitempty"`
	StatusUpdate   *TaskStatusUpdateEvent   `json:"statusUpdate,omitempty"`
	ArtifactUpdate *TaskArtifactUpdateEvent `json:"artifactUpdate,omitempty"`
}

// --------------------------------------------------------------------------
// Push Notification Config
// --------------------------------------------------------------------------

type PushNotificationConfig struct {
	ID     string              `json:"id"`
	TaskID string              `json:"taskId"`
	URL    string              `json:"url"`
	Token  string              `json:"token,omitempty"`
	Auth   *AuthenticationInfo `json:"authentication,omitempty"`
}

type AuthenticationInfo struct {
	Scheme      string `json:"scheme"`
	Credentials string `json:"credentials"`
}

// --------------------------------------------------------------------------
// Remote Agent (client-side representation)
// --------------------------------------------------------------------------

// RemoteAgent represents a discovered remote A2A agent.
type RemoteAgent struct {
	URL       string    `json:"url"`
	Card      AgentCard `json:"card"`
	AddedAt   time.Time `json:"added_at"`
	LastSeen  time.Time `json:"last_seen"`
	Status    string    `json:"status"` // "online", "offline", "unknown"
	TaskCount int       `json:"task_count"`
}
