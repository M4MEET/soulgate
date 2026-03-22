package audit

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of audit event
type EventType string

const (
	// Run events
	EventRunStart    EventType = "run.start"
	EventRunComplete EventType = "run.complete"
	EventRunError    EventType = "run.error"

	// Model events
	EventModelCall     EventType = "model.call"
	EventModelResponse EventType = "model.response"
	EventModelError    EventType = "model.error"

	// Tool events
	EventToolExecute EventType = "tool.execute"
	EventToolSuccess EventType = "tool.success"
	EventToolError   EventType = "tool.error"

	// Policy events
	EventPolicyEvaluate EventType = "policy.evaluate"
	EventPolicyAllow    EventType = "policy.allow"
	EventPolicyDeny     EventType = "policy.deny"

	// Broker events
	EventFileRead    EventType = "file.read"
	EventFileWrite   EventType = "file.write"
	EventFileList    EventType = "file.list"
	EventFileStat    EventType = "file.stat"
	EventFileDelete  EventType = "file.delete"
	EventNetRequest  EventType = "net.request"
	EventExecCommand EventType = "exec.command"

	// Plugin events
	EventPluginLoad   EventType = "plugin.load"
	EventPluginUnload EventType = "plugin.unload"

	// Session events
	EventSessionStart EventType = "session.start"
	EventSessionEnd   EventType = "session.end"
)

// EventCategory groups related event types
type EventCategory string

const (
	CategoryRun     EventCategory = "run"
	CategoryModel   EventCategory = "model"
	CategoryTool    EventCategory = "tool"
	CategoryPolicy  EventCategory = "policy"
	CategoryBroker  EventCategory = "broker"
	CategoryPlugin  EventCategory = "plugin"
	CategorySession EventCategory = "session"
)

// EventStatus represents the outcome of an operation
type EventStatus string

const (
	StatusSuccess EventStatus = "success"
	StatusError   EventStatus = "error"
	StatusDenied  EventStatus = "denied"
	StatusPending EventStatus = "pending"
)

// Decision represents a policy decision
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Event represents a single audit event
type Event struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	SessionID string                 `json:"session_id,omitempty"`
	RunID     string                 `json:"run_id,omitempty"`
	Type      EventType              `json:"type"`
	Category  EventCategory          `json:"category"`
	PluginID  string                 `json:"plugin_id,omitempty"`
	Action    string                 `json:"action,omitempty"`
	Resource  string                 `json:"resource,omitempty"`
	Decision  Decision               `json:"decision,omitempty"`
	Status    EventStatus            `json:"status"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewEvent creates a new audit event with default values
func NewEvent(eventType EventType, category EventCategory) *Event {
	return &Event{
		ID:        generateID(),
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Category:  category,
		Status:    StatusSuccess,
		Metadata:  make(map[string]interface{}),
	}
}

// WithSessionID sets the session ID
func (e *Event) WithSessionID(sessionID string) *Event {
	e.SessionID = sessionID
	return e
}

// WithRunID sets the run ID
func (e *Event) WithRunID(runID string) *Event {
	e.RunID = runID
	return e
}

// WithPlugin sets the plugin ID
func (e *Event) WithPlugin(pluginID string) *Event {
	e.PluginID = pluginID
	return e
}

// WithAction sets the action
func (e *Event) WithAction(action string) *Event {
	e.Action = action
	return e
}

// WithResource sets the resource
func (e *Event) WithResource(resource string) *Event {
	e.Resource = resource
	return e
}

// WithDecision sets the policy decision
func (e *Event) WithDecision(decision Decision) *Event {
	e.Decision = decision
	return e
}

// WithStatus sets the event status
func (e *Event) WithStatus(status EventStatus) *Event {
	e.Status = status
	return e
}

// WithError sets an error message
func (e *Event) WithError(err error) *Event {
	if err != nil {
		e.Error = err.Error()
		e.Status = StatusError
	}
	return e
}

// WithMetadata adds metadata key-value pairs
func (e *Event) WithMetadata(key string, value interface{}) *Event {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// ToJSON serializes the event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// generateID generates a unique event ID
func generateID() string {
	return uuid.NewString()
}
