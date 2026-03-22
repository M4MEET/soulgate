// Package mcp provides type definitions for the Model Context Protocol (MCP).
//
// MCP uses JSON-RPC 2.0 as its wire format, transported over stdio as
// newline-delimited JSON objects. The protocol handshake begins with an
// "initialize" request from the client, followed by an "initialized"
// notification, after which the full set of capabilities may be used.
//
// Protocol version targeted: 2024-11-05
package mcp

import "encoding/json"

// ---------------------------------------------------------------------------
// JSON-RPC 2.0 base types
// ---------------------------------------------------------------------------

// JSONRPCVersion is the literal value required in every JSON-RPC 2.0 message.
const JSONRPCVersion = "2.0"

// ProtocolVersion is the MCP protocol version this package targets.
const ProtocolVersion = "2024-11-05"

// Request is a JSON-RPC 2.0 request frame. When ID is zero the message is
// conventionally treated as a notification by older MCP implementations;
// use Notification for explicit fire-and-forget messages.
type Request struct {
	// JSONRPC must always equal JSONRPCVersion ("2.0").
	JSONRPC string `json:"jsonrpc"`
	// ID uniquely identifies this request within a session. The matching
	// Response must echo the same value.
	ID int64 `json:"id"`
	// Method is the RPC method name (e.g. "tools/list", "tools/call").
	Method string `json:"method"`
	// Params carries method-specific arguments. May be nil.
	Params interface{} `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response frame. A well-formed response carries
// exactly one of Result (success) or Error (failure); never both.
//
// The ID field uses *int64 so that server-initiated notifications (which have
// no ID) can be distinguished from responses with ID 0.
type Response struct {
	// JSONRPC must always equal JSONRPCVersion ("2.0").
	JSONRPC string `json:"jsonrpc"`
	// ID echoes the ID from the corresponding Request. Nil for notifications.
	ID *int64 `json:"id,omitempty"`
	// Result holds the success payload. Mutually exclusive with Error.
	Result *json.RawMessage `json:"result,omitempty"`
	// Error holds the failure detail. Mutually exclusive with Result.
	Error *RPCError `json:"error,omitempty"`
	// Method is populated for server-initiated notifications.
	Method string `json:"method,omitempty"`
	// Params is populated for server-initiated notifications.
	Params *json.RawMessage `json:"params,omitempty"`
}

// Notification is a JSON-RPC 2.0 one-way message. The server or client may
// emit notifications at any time; no response is sent or expected.
type Notification struct {
	// JSONRPC must always equal JSONRPCVersion ("2.0").
	JSONRPC string `json:"jsonrpc"`
	// Method is the notification name (e.g. "notifications/initialized").
	Method string `json:"method"`
	// Params carries notification-specific data. May be nil.
	Params interface{} `json:"params,omitempty"`
}

// RPCError is the JSON-RPC 2.0 error object embedded in a Response when the
// call fails. It also satisfies the standard Go error interface.
type RPCError struct {
	// Code is a numeric error code. Negative values -32768 through -32000 are
	// reserved by the JSON-RPC spec; MCP uses that range for protocol errors.
	Code int `json:"code"`
	// Message is a short, human-readable description of the error.
	Message string `json:"message"`
	// Data carries optional additional detail. May be any JSON value.
	Data json.RawMessage `json:"data,omitempty"`
}

// Error satisfies the error interface so RPCError values can be used directly
// in Go error-handling idioms.
func (e *RPCError) Error() string { return e.Message }

// Standard JSON-RPC 2.0 and MCP error codes.
const (
	// ErrCodeParse is returned when the server receives malformed JSON.
	ErrCodeParse = -32700
	// ErrCodeInvalidRequest is returned when the JSON is valid but not a
	// conformant request object.
	ErrCodeInvalidRequest = -32600
	// ErrCodeMethodNotFound is returned when the method does not exist.
	ErrCodeMethodNotFound = -32601
	// ErrCodeInvalidParams is returned when method parameters are invalid.
	ErrCodeInvalidParams = -32602
	// ErrCodeInternal is returned for unexpected internal errors.
	ErrCodeInternal = -32603
)

// ---------------------------------------------------------------------------
// MCP Initialize handshake
// ---------------------------------------------------------------------------

// InitializeParams is the parameter object for the "initialize" request. The
// client sends this as the very first message in every MCP session.
type InitializeParams struct {
	// ProtocolVersion is the MCP version the client prefers (e.g. "2024-11-05").
	ProtocolVersion string `json:"protocolVersion"`
	// Capabilities describes the optional MCP features this client supports.
	Capabilities ClientCapabilities `json:"capabilities"`
	// ClientInfo identifies the connecting application to the server.
	ClientInfo ClientInfo `json:"clientInfo"`
}

// ClientInfo carries human-readable identification for the connecting client.
type ClientInfo struct {
	// Name is the application name (e.g. "claude-desktop", "soulgate").
	Name string `json:"name"`
	// Version is the application version string (e.g. "1.0.0").
	Version string `json:"version"`
}

// ClientCapabilities declares which optional MCP features the client supports.
// A nil pointer for any field means the client does not support that feature.
//
// ClientCaps is an alias for ClientCapabilities retained for compatibility with
// existing code within this package.
type ClientCapabilities struct {
	// Roots is non-nil when the client supports the workspace roots feature.
	Roots *RootsClientCapability `json:"roots,omitempty"`
	// Sampling is non-nil when the client supports server-initiated LLM
	// sampling (i.e. the server may ask the client to run a prompt).
	Sampling *SamplingClientCapability `json:"sampling,omitempty"`
}

// ClientCaps is an alias for ClientCapabilities for use in the initialize
// handshake where brevity is preferred.
type ClientCaps = ClientCapabilities

// RootsClientCapability describes client support for workspace roots.
type RootsClientCapability struct {
	// ListChanged indicates the client will send a roots/listChanged
	// notification whenever its set of roots changes.
	ListChanged bool `json:"listChanged"`
}

// RootsCap is an alias for RootsClientCapability.
type RootsCap = RootsClientCapability

// SamplingClientCapability is a marker that signals the client can handle
// sampling/createMessage requests from the server. Reserved for future fields.
type SamplingClientCapability struct{}

// SamplingCap is an alias for SamplingClientCapability.
type SamplingCap = SamplingClientCapability

// InitializeResult is returned by the server in response to "initialize".
type InitializeResult struct {
	// ProtocolVersion is the MCP version the server will use for this session.
	ProtocolVersion string `json:"protocolVersion"`
	// Capabilities describes the optional MCP features this server supports.
	Capabilities ServerCapabilities `json:"capabilities"`
	// ServerInfo identifies the server application.
	ServerInfo ServerInfo `json:"serverInfo"`
}

// ServerInfo carries human-readable identification for the MCP server.
type ServerInfo struct {
	// Name is the server application name (e.g. "soulgate-mcp").
	Name string `json:"name"`
	// Version is the server application version string.
	Version string `json:"version"`
}

// ServerCapabilities declares which optional MCP features the server supports.
// A nil pointer for any field means the server does not support that feature.
type ServerCapabilities struct {
	// Tools is non-nil when the server exposes callable tools.
	Tools *ToolsServerCapability `json:"tools,omitempty"`
	// Resources is non-nil when the server exposes readable resources.
	Resources *ResourcesServerCapability `json:"resources,omitempty"`
	// Prompts is non-nil when the server exposes prompt templates.
	Prompts *PromptsServerCapability `json:"prompts,omitempty"`
	// Logging is non-nil when the server supports structured log messages.
	Logging *LoggingServerCapability `json:"logging,omitempty"`
}

// ToolsServerCapability describes server support for the tools feature.
type ToolsServerCapability struct {
	// ListChanged indicates the server emits tools/listChanged notifications
	// when the available tool set changes at runtime.
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesServerCapability describes server support for the resources feature.
type ResourcesServerCapability struct {
	// Subscribe indicates the server supports resource-change subscriptions
	// (resources/subscribe and resources/unsubscribe methods).
	Subscribe bool `json:"subscribe,omitempty"`
	// ListChanged indicates the server emits resources/listChanged
	// notifications when the available resource set changes at runtime.
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsServerCapability describes server support for the prompts feature.
type PromptsServerCapability struct {
	// ListChanged indicates the server emits prompts/listChanged notifications
	// when the available prompt set changes at runtime.
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingServerCapability is a marker that signals the server can emit
// structured log messages via notifications/message. Reserved for future fields.
type LoggingServerCapability struct{}

// ---------------------------------------------------------------------------
// MCP Tools
// ---------------------------------------------------------------------------

// Tool describes a single callable tool exposed by an MCP server.
type Tool struct {
	// Name is the unique identifier used in CallToolParams. Must be stable
	// across sessions for a given server version.
	Name string `json:"name"`
	// Description is a human-readable explanation of the tool's purpose and
	// behaviour. Shown to the LLM to guide tool selection.
	Description string `json:"description,omitempty"`
	// InputSchema is a JSON Schema object ({"type":"object",...}) describing
	// the expected arguments map. Stored as raw JSON so any valid schema can
	// be embedded without additional Go struct definitions.
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListToolsResult is the response body for the "tools/list" method.
type ListToolsResult struct {
	// Tools is the complete, ordered list of tools available on this server.
	Tools []Tool `json:"tools"`
}

// CallToolParams is the parameter object for the "tools/call" method.
type CallToolParams struct {
	// Name identifies the tool to invoke; must match a Tool.Name.
	Name string `json:"name"`
	// Arguments is a free-form map of argument values. The server validates
	// these against the tool's InputSchema before executing.
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// CallToolResult is the response body for the "tools/call" method.
type CallToolResult struct {
	// Content is the ordered list of content blocks produced by the tool.
	Content []ToolContent `json:"content"`
	// IsError signals that the tool execution itself failed (as opposed to a
	// transport-level or protocol error). When true, Content typically carries
	// an error description the LLM can act on.
	IsError bool `json:"isError,omitempty"`
}

// ToolContent is a single output block from a tool call.
// The Type field determines which other fields carry meaningful data.
type ToolContent struct {
	// Type is "text" for plain-text output or "image" for base64-encoded
	// binary image data.
	Type string `json:"type"`
	// Text holds the plain-text output when Type is ToolContentTypeText.
	Text string `json:"text,omitempty"`
	// MIMEType describes the image format (e.g. "image/png") when Type is
	// ToolContentTypeImage.
	MIMEType string `json:"mimeType,omitempty"`
	// Data is the base64-encoded image bytes when Type is ToolContentTypeImage.
	Data string `json:"data,omitempty"`
}

// ToolContentType constants enumerate the allowed values for ToolContent.Type.
const (
	ToolContentTypeText  = "text"
	ToolContentTypeImage = "image"
)

// ---------------------------------------------------------------------------
// MCP Resources
// ---------------------------------------------------------------------------

// Resource describes a single readable resource exposed by an MCP server.
type Resource struct {
	// URI is the stable, unique address of the resource. Clients use this
	// value verbatim in ReadResourceParams (e.g. "file:///workspace/README.md").
	URI string `json:"uri"`
	// Name is a short, human-readable label shown in UIs and to the LLM.
	Name string `json:"name"`
	// Description is an optional longer explanation of the resource content.
	Description string `json:"description,omitempty"`
	// MIMEType is the optional IANA media type of the resource
	// (e.g. "text/plain", "application/json").
	MIMEType string `json:"mimeType,omitempty"`
}

// ListResourcesResult is the response body for the "resources/list" method.
type ListResourcesResult struct {
	// Resources is the complete, ordered list of resources available on this
	// server.
	Resources []Resource `json:"resources"`
}

// ReadResourceParams is the parameter object for the "resources/read" method.
type ReadResourceParams struct {
	// URI identifies the resource to read; must match a Resource.URI.
	URI string `json:"uri"`
}

// ReadResourceResult is the response body for the "resources/read" method.
type ReadResourceResult struct {
	// Contents is the ordered list of content blocks that make up the resource.
	// A single resource may span multiple blocks (e.g. a paginated document).
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is a single block of content within a resource response.
// Text-based content uses Text; binary content uses Blob (base64-encoded).
type ResourceContent struct {
	// URI is the address of the resource this content belongs to. Matches the
	// URI in ReadResourceParams or the resource's canonical URI.
	URI string `json:"uri"`
	// MIMEType is the IANA media type of this content block.
	MIMEType string `json:"mimeType,omitempty"`
	// Text holds decoded text for text-based content blocks.
	Text string `json:"text,omitempty"`
	// Blob holds base64-encoded raw bytes for binary content blocks.
	Blob string `json:"blob,omitempty"`
}

// ---------------------------------------------------------------------------
// MCP Prompts
// ---------------------------------------------------------------------------

// Prompt describes a single prompt template exposed by an MCP server.
type Prompt struct {
	// Name is the unique identifier used in GetPromptParams. Must be stable
	// across sessions for a given server version.
	Name string `json:"name"`
	// Description is a human-readable summary of the prompt's purpose,
	// shown to LLMs and users when selecting prompts.
	Description string `json:"description,omitempty"`
	// Arguments defines the named parameters callers may supply when fetching
	// this prompt. Required arguments must be provided; optional ones may be
	// omitted.
	Arguments []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument defines a single named parameter accepted by a Prompt.
type PromptArgument struct {
	// Name is the argument identifier used as a key in GetPromptParams.Arguments.
	Name string `json:"name"`
	// Description explains the expected value or semantics of this argument.
	Description string `json:"description,omitempty"`
	// Required is true when the caller must provide a value for this argument;
	// false for optional arguments.
	Required bool `json:"required,omitempty"`
}

// ListPromptsResult is the response body for the "prompts/list" method.
type ListPromptsResult struct {
	// Prompts is the complete, ordered list of prompt templates available on
	// this server.
	Prompts []Prompt `json:"prompts"`
}

// GetPromptParams is the parameter object for the "prompts/get" method.
type GetPromptParams struct {
	// Name identifies the prompt template to render; must match a Prompt.Name.
	Name string `json:"name"`
	// Arguments is a map from argument names (matching PromptArgument.Name) to
	// their string values. Required arguments must be present.
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult is the response body for the "prompts/get" method.
type GetPromptResult struct {
	// Description is an optional human-readable summary of the rendered prompt,
	// which may differ from the template description when arguments are applied.
	Description string `json:"description,omitempty"`
	// Messages is the ordered list of conversation turns that form the fully
	// rendered prompt, ready to be sent to an LLM.
	Messages []PromptMessage `json:"messages"`
}

// PromptMessage is a single conversation turn within a rendered prompt.
type PromptMessage struct {
	// Role identifies the participant for this turn.
	// Use PromptRoleUser or PromptRoleAssistant.
	Role string `json:"role"`
	// Content holds the payload of this message turn.
	Content PromptContent `json:"content"`
}

// PromptMessageRole constants enumerate the allowed values for PromptMessage.Role.
const (
	// PromptRoleUser represents a turn produced by the human participant.
	PromptRoleUser = "user"
	// PromptRoleAssistant represents a turn produced by the AI assistant.
	PromptRoleAssistant = "assistant"
)

// PromptContent is the payload of a PromptMessage.
// The Type field determines which other fields carry meaningful data.
type PromptContent struct {
	// Type is "text" for plain-text content or "resource" for an embedded
	// resource block.
	Type string `json:"type"`
	// Text holds the message text when Type is PromptContentTypeText.
	Text string `json:"text,omitempty"`
	// Resource holds the embedded resource content when Type is
	// PromptContentTypeResource.
	Resource *ResourceContent `json:"resource,omitempty"`
}

// PromptContentType constants enumerate the allowed values for PromptContent.Type.
const (
	// PromptContentTypeText indicates the message is plain text.
	PromptContentTypeText = "text"
	// PromptContentTypeResource indicates the message embeds a resource block.
	PromptContentTypeResource = "resource"
)

// ---------------------------------------------------------------------------
// MCP Server configuration
// ---------------------------------------------------------------------------

// ServerConfig describes how to launch and connect to an external MCP server
// process. SoulGate uses this to manage the lifecycle of server sub-processes
// that are started on demand and communicate via stdio.
type ServerConfig struct {
	// Name is a human-readable label for this server (used in logs and UI).
	Name string `json:"name"`
	// Command is the executable to run. May be an absolute path or a
	// PATH-resolved command name (e.g. "npx", "python3").
	Command string `json:"command"`
	// Args are the command-line arguments passed verbatim to Command.
	Args []string `json:"args,omitempty"`
	// Env is a list of "KEY=VALUE" strings injected into the server's
	// environment. When nil the process inherits the parent environment
	// unchanged; entries here are merged on top.
	Env []string `json:"env,omitempty"`
	// WorkDir is the working directory for the server process. Defaults to
	// the SoulGate workspace root when empty.
	WorkDir string `json:"workDir,omitempty"`
	// Enabled controls whether this server is started automatically when
	// SoulGate loads its configuration. When false the entry is retained for
	// reference but no process is launched.
	Enabled bool `json:"enabled"`
}
