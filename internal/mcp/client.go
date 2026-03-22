package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Client communicates with a single MCP server over its stdio streams.
// Each JSON-RPC message is a single line of JSON terminated by '\n'.
//
// Concurrency model:
//   - mu serialises writes to stdin and the paired stdout read so that
//     only one request/response round-trip is in flight at a time.
//   - A background goroutine drains stderr so the subprocess never
//     blocks on its own error output.
//   - poisoned is set when a context cancellation leaves an abandoned
//     read goroutine; no new requests may be issued until the client is
//     closed and a new one created.
type Client struct {
	name       string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	stderr     *bufio.Reader
	mu         sync.Mutex
	nextID     atomic.Int64
	serverInfo *InitializeResult
	tools      []Tool
	resources  []Resource
	prompts    []Prompt
	closed     bool
	poisoned   bool // true after a context-cancelled sendRequest
}

// NewClient creates a Client from cfg but does not start the subprocess.
// Call Start to spawn the process and perform the MCP handshake.
func NewClient(cfg ServerConfig) *Client {
	return &Client{
		name: cfg.Name,
		cmd:  buildCmd(cfg),
	}
}

// buildCmd constructs the exec.Cmd from a ServerConfig without starting it.
//
// Environment handling: the subprocess always inherits the parent process
// environment so that commands like npx, python3, or uv can locate
// executables on PATH. Any entries in cfg.Env are merged on top, allowing
// callers to override or extend specific variables without stripping the
// inherited environment.
func buildCmd(cfg ServerConfig) *exec.Cmd {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		// Start from the parent environment and layer cfg.Env on top.
		cmd.Env = append(os.Environ(), cfg.Env...)
	}
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	return cmd
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// Start spawns the MCP server subprocess and performs the initialization
// handshake defined by the MCP protocol:
//
//  1. Pipe stdin / stdout / stderr.
//  2. Send an initialize request and wait for InitializeResult.
//  3. Send a notifications/initialized notification (fire-and-forget).
//  4. Discover available capabilities (tools, resources, prompts).
//
// The context controls the overall startup timeout; it is not retained after
// Start returns.
func (c *Client) Start(ctx context.Context) error {
	// Wire up pipes before starting the process.
	stdinPipe, err := c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp client %q: stdin pipe: %w", c.name, err)
	}

	stdoutPipe, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp client %q: stdout pipe: %w", c.name, err)
	}

	stderrPipe, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("mcp client %q: stderr pipe: %w", c.name, err)
	}

	c.stdin = stdinPipe
	c.stdout = bufio.NewReader(stdoutPipe)
	c.stderr = bufio.NewReader(stderrPipe)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp client %q: start subprocess: %w", c.name, err)
	}

	// Drain stderr in the background so the subprocess never blocks.
	go c.drainStderr()

	// Perform the MCP initialize handshake.
	result, err := c.initialize(ctx)
	if err != nil {
		// Best-effort cleanup if handshake fails.
		_ = c.cmd.Process.Kill()
		return fmt.Errorf("mcp client %q: initialize: %w", c.name, err)
	}
	c.serverInfo = result

	// Acknowledge the handshake with a fire-and-forget notification.
	if err := c.sendNotification("notifications/initialized", nil); err != nil {
		return fmt.Errorf("mcp client %q: notifications/initialized: %w", c.name, err)
	}

	// Discover what the server can do based on its advertised capabilities.
	if err := c.discoverCapabilities(ctx); err != nil {
		return fmt.Errorf("mcp client %q: discover capabilities: %w", c.name, err)
	}

	return nil
}

// Close signals the MCP server to shut down and waits for the subprocess to
// exit.  It first closes stdin so the server sees EOF, then waits up to five
// seconds before sending SIGKILL.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Close stdin to signal EOF to the server.
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Wait for the subprocess to exit with a deadline.
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()

	select {
	case <-time.After(5 * time.Second):
		if killErr := c.cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("mcp client %q: kill after timeout: %w", c.name, killErr)
		}
		return fmt.Errorf("mcp client %q: subprocess did not exit within timeout; killed", c.name)
	case err := <-done:
		if err != nil {
			// Non-zero exit is expected on clean shutdown for many servers.
			return nil
		}
		return nil
	}
}

// IsRunning reports whether the subprocess is still alive.
func (c *Client) IsRunning() bool {
	if c.cmd == nil || c.cmd.Process == nil {
		return false
	}
	// ProcessState is set only after the process exits.
	return c.cmd.ProcessState == nil
}

// ---------------------------------------------------------------------------
// Getters
// ---------------------------------------------------------------------------

// Name returns the human-readable server name supplied in ServerConfig.
func (c *Client) Name() string { return c.name }

// ServerInfo returns the server identification returned during the MCP
// initialization handshake. Returns nil if Start has not been called or
// the handshake failed.
func (c *Client) ServerInfo() *InitializeResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.serverInfo
}

// Tools returns the list of tools discovered during Start.
// ListTools is a public alias kept for API symmetry with Resources and Prompts.
func (c *Client) Tools() []Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Tool, len(c.tools))
	copy(out, c.tools)
	return out
}

// ListTools is an alias for Tools. It fetches a fresh copy of the tool list
// from the server, refreshing the internal cache. This is useful after a
// tools/listChanged notification.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.listTools(ctx); err != nil {
		return nil, fmt.Errorf("mcp client %q: ListTools: %w", c.name, err)
	}
	return c.Tools(), nil
}

// Resources returns the list of resources discovered during Start.
func (c *Client) Resources() []Resource {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Resource, len(c.resources))
	copy(out, c.resources)
	return out
}

// Prompts returns the list of prompts discovered during Start.
func (c *Client) Prompts() []Prompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Prompt, len(c.prompts))
	copy(out, c.prompts)
	return out
}

// ---------------------------------------------------------------------------
// High-level MCP operations
// ---------------------------------------------------------------------------

// CallTool invokes the named tool on the MCP server with the provided
// arguments and returns the structured result.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error) {
	raw, err := c.sendRequest(ctx, "tools/call", CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: tools/call %q: %w", c.name, name, err)
	}

	var result CallToolResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, fmt.Errorf("mcp client %q: tools/call %q unmarshal: %w", c.name, name, err)
	}
	return &result, nil
}

// ReadResource fetches the content of the resource identified by uri.
func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	raw, err := c.sendRequest(ctx, "resources/read", ReadResourceParams{URI: uri})
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: resources/read %q: %w", c.name, uri, err)
	}

	var result ReadResourceResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, fmt.Errorf("mcp client %q: resources/read %q unmarshal: %w", c.name, uri, err)
	}
	return &result, nil
}

// GetPrompt renders the named prompt template with the provided arguments.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*GetPromptResult, error) {
	raw, err := c.sendRequest(ctx, "prompts/get", GetPromptParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("mcp client %q: prompts/get %q: %w", c.name, name, err)
	}

	var result GetPromptResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, fmt.Errorf("mcp client %q: prompts/get %q unmarshal: %w", c.name, name, err)
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Core JSON-RPC transport
// ---------------------------------------------------------------------------

// sendRequest performs one synchronous JSON-RPC round-trip.
//
// Design: the mutex serialises the write+read cycle so that at most one
// request is in flight at a time. This avoids multiplexing complexity and
// matches the common MCP server implementation pattern.
//
// Context cancellation: when the context is cancelled, the call returns
// immediately with ctx.Err().  The background goroutine that is blocking on
// stdout continues to run until it receives a response (or an I/O error).
// Once it receives the response it sends it on a buffered channel and exits —
// the response is silently discarded.  The channel is buffered (size 1) so the
// goroutine never leaks.
//
// If the context is cancelled the client is marked as poisoned, preventing
// subsequent callers from interleaving their reads with the abandoned goroutine.
// The caller that caused the cancellation is responsible for closing the client
// if it wants to reuse the connection cleanly.
func (c *Client) sendRequest(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("mcp client %q: client is closed", c.name)
	}
	if c.poisoned {
		return nil, fmt.Errorf("mcp client %q: connection is poisoned due to a prior context cancellation; close and reconnect", c.name)
	}

	id := c.nextID.Add(1)

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	line, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	line = append(line, '\n')

	if _, err := c.stdin.Write(line); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read responses in a buffered-channel goroutine so we can respect ctx
	// cancellation without blocking the caller indefinitely.
	type readResult struct {
		raw *json.RawMessage
		err error
	}
	ch := make(chan readResult, 1)

	go func() {
		for {
			raw, readErr := c.readResponse(id)
			if readErr != nil {
				ch <- readResult{err: readErr}
				return
			}
			if raw != nil {
				ch <- readResult{raw: raw}
				return
			}
			// raw == nil means an unrelated server notification; keep reading.
		}
	}()

	select {
	case <-ctx.Done():
		// Mark connection as poisoned: the goroutine above still holds an
		// implicit read on stdout.  No new request should be issued until the
		// goroutine drains that frame.
		c.poisoned = true
		return nil, fmt.Errorf("mcp client %q: %w", c.name, ctx.Err())
	case r := <-ch:
		return r.raw, r.err
	}
}

// readResponse reads one line from stdout and returns the result payload if
// the response ID matches the expected id.  It returns (nil, nil) when the
// line is a server-initiated notification (no ID or mismatched ID) so that the
// caller can loop and read the next line.
func (c *Client) readResponse(expectedID int64) (*json.RawMessage, error) {
	rawLine, err := c.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read response line: %w", err)
	}

	var resp Response
	if err := json.Unmarshal([]byte(rawLine), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// Server-initiated notification: no ID field.
	if resp.ID == nil {
		return nil, nil
	}

	// Response for a different in-flight request (should not happen with
	// serialised requests, but be defensive).
	if *resp.ID != expectedID {
		return nil, nil
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	if resp.Result == nil {
		empty := json.RawMessage("{}")
		return &empty, nil
	}

	return resp.Result, nil
}

// sendNotification writes a JSON-RPC notification (no ID) and does not wait
// for a response.
func (c *Client) sendNotification(method string, params interface{}) error {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	line, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	line = append(line, '\n')

	if _, err := c.stdin.Write(line); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// MCP handshake and capability discovery
// ---------------------------------------------------------------------------

// initialize sends the MCP initialize request and returns the server's result.
func (c *Client) initialize(ctx context.Context) (*InitializeResult, error) {
	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ClientInfo: ClientInfo{
			Name:    "soulgate",
			Version: "0.1.0",
		},
		Capabilities: ClientCaps{},
	}

	raw, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result InitializeResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal initialize result: %w", err)
	}
	return &result, nil
}

// discoverCapabilities calls tools/list, resources/list, and prompts/list
// depending on what the server advertised in its initialize response.
func (c *Client) discoverCapabilities(ctx context.Context) error {
	if c.serverInfo == nil {
		return nil
	}
	caps := c.serverInfo.Capabilities

	if caps.Tools != nil {
		if err := c.listTools(ctx); err != nil {
			return fmt.Errorf("list tools: %w", err)
		}
	}

	if caps.Resources != nil {
		if err := c.listResources(ctx); err != nil {
			return fmt.Errorf("list resources: %w", err)
		}
	}

	if caps.Prompts != nil {
		if err := c.listPrompts(ctx); err != nil {
			return fmt.Errorf("list prompts: %w", err)
		}
	}

	return nil
}

// listTools fetches and caches all tools the server exposes.
func (c *Client) listTools(ctx context.Context) error {
	raw, err := c.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return err
	}

	var result ListToolsResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return fmt.Errorf("unmarshal tools/list: %w", err)
	}

	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()
	return nil
}

// listResources fetches and caches all resources the server exposes.
func (c *Client) listResources(ctx context.Context) error {
	raw, err := c.sendRequest(ctx, "resources/list", nil)
	if err != nil {
		return err
	}

	var result ListResourcesResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return fmt.Errorf("unmarshal resources/list: %w", err)
	}

	c.mu.Lock()
	c.resources = result.Resources
	c.mu.Unlock()
	return nil
}

// listPrompts fetches and caches all prompts the server exposes.
func (c *Client) listPrompts(ctx context.Context) error {
	raw, err := c.sendRequest(ctx, "prompts/list", nil)
	if err != nil {
		return err
	}

	var result ListPromptsResult
	if err := json.Unmarshal(*raw, &result); err != nil {
		return fmt.Errorf("unmarshal prompts/list: %w", err)
	}

	c.mu.Lock()
	c.prompts = result.Prompts
	c.mu.Unlock()
	return nil
}

// ---------------------------------------------------------------------------
// Background helpers
// ---------------------------------------------------------------------------

// drainStderr reads and discards all output written by the subprocess to its
// stderr.  This prevents the server from blocking when its stderr buffer fills.
// A future enhancement could route these lines to a structured logger.
func (c *Client) drainStderr() {
	for {
		_, err := c.stderr.ReadString('\n')
		if err != nil {
			// io.EOF or pipe closed – subprocess has exited.
			return
		}
	}
}
