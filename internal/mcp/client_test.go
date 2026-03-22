package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers — in-process mock MCP server
// ---------------------------------------------------------------------------

// mockServer acts as a minimal MCP server over an in-process pipe pair.
// It handles initialize, notifications/initialized, tools/list, and
// tools/call so tests can exercise Client without spawning a subprocess.
type mockServer struct {
	in  io.WriteCloser // server writes responses here (== client stdout)
	out *strings.Reader
}

// startMockServer wires an in-process pipe and returns a Client whose stdio
// is connected to the mock server, plus a channel that receives each inbound
// request line so the test can assert on it.
//
// The mock server automatically responds to:
//   - initialize  → InitializeResult with tools capability
//   - notifications/initialized → (no response; notification)
//   - tools/list  → the provided tools list
//   - tools/call  → a synthetic text result
//   - resources/list → empty list
//   - prompts/list   → empty list
func buildMockServerPair(t *testing.T, serverTools []Tool) (*Client, <-chan string) {
	t.Helper()

	// Bidirectional pipes:
	//   clientStdin  ← written by Client, read by mock server
	//   clientStdout ← written by mock server, read by Client
	clientStdinR, clientStdinW := io.Pipe()
	clientStdoutR, clientStdoutW := io.Pipe()

	requests := make(chan string, 16)

	// Run the mock server in the background.
	go func() {
		defer clientStdoutW.Close()
		dec := json.NewDecoder(clientStdinR)
		enc := json.NewEncoder(clientStdoutW)

		for {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return // client closed stdin
			}

			// Record the raw request for tests to inspect.
			requests <- string(raw)

			// Peek at the method.
			var frame struct {
				ID     *int64          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(raw, &frame); err != nil {
				return
			}

			// Notifications have no ID — do not send a response.
			if frame.ID == nil {
				continue
			}

			var result interface{}
			switch frame.Method {
			case "initialize":
				result = InitializeResult{
					ProtocolVersion: ProtocolVersion,
					ServerInfo:      ServerInfo{Name: "mock-server", Version: "0.0.1"},
					Capabilities: ServerCapabilities{
						Tools:     &ToolsServerCapability{},
						Resources: &ResourcesServerCapability{},
						Prompts:   &PromptsServerCapability{},
					},
				}
			case "tools/list":
				result = ListToolsResult{Tools: serverTools}
			case "tools/call":
				var p CallToolParams
				_ = json.Unmarshal(frame.Params, &p)
				result = CallToolResult{
					Content: []ToolContent{{
						Type: ToolContentTypeText,
						Text: fmt.Sprintf("called %s", p.Name),
					}},
				}
			case "resources/list":
				result = ListResourcesResult{Resources: nil}
			case "prompts/list":
				result = ListPromptsResult{Prompts: nil}
			default:
				result = map[string]string{"error": "unknown method"}
			}

			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      frame.ID,
				"result":  result,
			}
			if err := enc.Encode(resp); err != nil {
				return
			}
		}
	}()

	c := &Client{
		name:   "mock",
		stdin:  clientStdinW,
		stdout: newBufioReader(clientStdoutR),
		stderr: newBufioReader(strings.NewReader("")),
	}

	return c, requests
}

// newBufioReader builds a *bufio.Reader from an io.Reader (for pipe or NopCloser).
func newBufioReader(r io.Reader) *bufio.Reader {
	return bufio.NewReader(r)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestClientInitialize verifies that Start performs the MCP handshake and
// populates tools, resources, and prompts.
func TestClientInitialize(t *testing.T) {
	tools := []Tool{
		{Name: "greet", Description: "say hello", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	client, _ := buildMockServerPair(t, tools)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Manually trigger the handshake (skipping subprocess spawn).
	result, err := client.initialize(ctx)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if result.ServerInfo.Name != "mock-server" {
		t.Errorf("unexpected server name: %q", result.ServerInfo.Name)
	}
	client.serverInfo = result

	if err := client.sendNotification("notifications/initialized", nil); err != nil {
		t.Fatalf("notifications/initialized: %v", err)
	}

	if err := client.listTools(ctx); err != nil {
		t.Fatalf("listTools: %v", err)
	}

	got := client.Tools()
	if len(got) != 1 || got[0].Name != "greet" {
		t.Errorf("expected [greet], got %v", got)
	}
}

// TestClientCallTool verifies that CallTool sends the correct request and
// returns the text content from the mock server.
func TestClientCallTool(t *testing.T) {
	tools := []Tool{
		{Name: "echo", Description: "echo", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	client, _ := buildMockServerPair(t, tools)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.CallTool(ctx, "echo", map[string]interface{}{"msg": "hello"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	if result.Content[0].Text != "called echo" {
		t.Errorf("unexpected content: %q", result.Content[0].Text)
	}
}

// TestClientListTools verifies that the public ListTools method works.
func TestClientListTools(t *testing.T) {
	tools := []Tool{
		{Name: "tool_a", InputSchema: json.RawMessage(`{}`)},
		{Name: "tool_b", InputSchema: json.RawMessage(`{}`)},
	}
	client, _ := buildMockServerPair(t, tools)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 tools, got %d", len(got))
	}
}

// TestClientClosedReturnsError verifies that requests after Close return an error.
func TestClientClosedReturnsError(t *testing.T) {
	client, _ := buildMockServerPair(t, nil)

	// Mark closed without going through full Close() to avoid needing a real cmd.
	client.mu.Lock()
	client.closed = true
	client.mu.Unlock()

	_, err := client.CallTool(context.Background(), "noop", nil)
	if err == nil {
		t.Fatal("expected error after close, got nil")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestClientPoisonedReturnsError verifies that requests after a context
// cancellation (poisoned state) return a descriptive error.
func TestClientPoisonedReturnsError(t *testing.T) {
	// Build a server that blocks indefinitely so the context times out.
	clientStdinR, clientStdinW := io.Pipe()
	_, clientStdoutW := io.Pipe()
	// Swallow stdin but never write to stdout, forcing the client to wait.
	go func() {
		io.Copy(io.Discard, clientStdinR)
	}()
	// stdout reader that never produces data
	pr, _ := io.Pipe()

	client := &Client{
		name:   "blocking",
		stdin:  clientStdinW,
		stdout: bufio.NewReader(pr),
		stderr: bufio.NewReader(strings.NewReader("")),
	}
	// Suppress the "write notification" path during close cleanup.
	client.closed = false

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should time out and poison the client.
	_, err := client.sendRequest(ctx, "tools/list", nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Now attempt another request — should get a poison error.
	_, err2 := client.sendRequest(context.Background(), "tools/list", nil)
	if err2 == nil {
		t.Fatal("expected poisoned error, got nil")
	}
	if !strings.Contains(err2.Error(), "poisoned") {
		t.Errorf("unexpected error: %v", err2)
	}
	_ = clientStdoutW.Close() // cleanup
}

// TestIsMCPTool checks the separator detection helper.
func TestIsMCPTool(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"github__create_issue", true},
		{"filesystem__read_file", true},
		{"plain_tool", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsMCPTool(tc.name); got != tc.want {
			t.Errorf("IsMCPTool(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestBuildCmdEnvInheritance verifies that environment variables from
// cfg.Env are layered on top of the parent environment rather than replacing it.
func TestBuildCmdEnvInheritance(t *testing.T) {
	cfg := ServerConfig{
		Name:    "test",
		Command: "true",
		Env:     []string{"CUSTOM_VAR=hello"},
	}
	cmd := buildCmd(cfg)
	if cmd == nil {
		t.Fatal("buildCmd returned nil")
	}

	// cmd.Env should contain both our custom var and at least PATH from the
	// parent environment.
	envMap := make(map[string]bool, len(cmd.Env))
	for _, e := range cmd.Env {
		envMap[e] = true
	}

	if !envMap["CUSTOM_VAR=hello"] {
		t.Error("CUSTOM_VAR not found in cmd.Env")
	}

	// At least one PATH= entry should be present (inherited from parent).
	hasPath := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "PATH=") {
			hasPath = true
			break
		}
	}
	if !hasPath {
		t.Error("PATH not inherited in cmd.Env when cfg.Env is set")
	}
}

// TestBuildCmdNoEnvInheritsParent verifies that when cfg.Env is empty the
// subprocess inherits the parent environment implicitly (cmd.Env == nil).
func TestBuildCmdNoEnvInheritsParent(t *testing.T) {
	cfg := ServerConfig{
		Name:    "test",
		Command: "true",
	}
	cmd := buildCmd(cfg)
	if cmd.Env != nil {
		t.Errorf("expected nil cmd.Env for empty cfg.Env, got %v", cmd.Env)
	}
}
