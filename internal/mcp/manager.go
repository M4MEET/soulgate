package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// separator is placed between the server name and tool name when prefixing
// combined tool names to avoid collisions across servers.
const separator = "__"

// IsMCPTool returns true if the tool name looks like an MCP-prefixed tool
// (contains the "__" separator).
func IsMCPTool(name string) bool {
	return strings.Contains(name, separator)
}

// ServerStatus summarises the runtime state of a single managed server.
type ServerStatus struct {
	// Name is the server's unique identifier within the manager.
	Name string

	// Running is true when the server's Client is connected and healthy.
	Running bool

	// Tools is the number of tools currently available from this server.
	Tools int

	// Resources is the number of resources currently available from this server.
	Resources int

	// Prompts is the number of prompt templates currently available from this server.
	Prompts int
}

// Manager manages a collection of MCP server connections, aggregates their
// capabilities, and routes operations to the correct server.
//
// All exported methods are safe for concurrent use.
type Manager struct {
	mu        sync.RWMutex
	clients   map[string]*Client // server name -> client
	toolIndex map[string]string  // combined tool name -> server name
}

// NewManager creates an empty Manager ready for servers to be added.
func NewManager() *Manager {
	return &Manager{
		clients:   make(map[string]*Client),
		toolIndex: make(map[string]string),
	}
}

// AddServer registers a new MCP server using cfg. It creates a Client and
// stores it internally but does not start the connection. Returns an error if
// a server with the same name already exists.
func (m *Manager) AddServer(cfg ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[cfg.Name]; exists {
		return fmt.Errorf("mcp manager: server %q already registered", cfg.Name)
	}

	m.clients[cfg.Name] = NewClient(cfg)
	return nil
}

// StartAll starts every registered server. If a server fails to start the
// error is printed to stderr as a warning and the loop continues with the
// remaining servers. After all start attempts complete, the tool index is
// rebuilt from the servers that started successfully.
func (m *Manager) StartAll(ctx context.Context) error {
	// Collect the names first under a read lock so we don't hold the write
	// lock for the duration of potentially slow network/process startups.
	m.mu.RLock()
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	m.mu.RUnlock()

	for _, name := range names {
		if err := m.StartServer(ctx, name); err != nil {
			// Warn and continue – partial startup is acceptable.
			fmt.Printf("warning: mcp manager: failed to start server %q: %v\n", name, err)
		}
	}

	m.rebuildIndex()
	return nil
}

// StartServer starts the named server and rebuilds the tool index on success.
// Returns an error if the server is not registered or fails to start.
func (m *Manager) StartServer(ctx context.Context, name string) error {
	m.mu.RLock()
	client, ok := m.clients[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("mcp manager: server %q not registered", name)
	}

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("mcp manager: start server %q: %w", name, err)
	}

	m.rebuildIndex()
	return nil
}

// StopServer stops the named server and rebuilds the tool index.
// Returns an error if the server is not registered or fails to stop.
func (m *Manager) StopServer(name string) error {
	m.mu.RLock()
	client, ok := m.clients[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("mcp manager: server %q not registered", name)
	}

	if err := client.Close(); err != nil {
		return fmt.Errorf("mcp manager: stop server %q: %w", name, err)
	}

	m.rebuildIndex()
	return nil
}

// StopAll stops all registered servers. Errors from individual servers are
// collected and returned as a combined error.
func (m *Manager) StopAll() error {
	m.mu.RLock()
	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	m.mu.RUnlock()

	var errs []string
	for _, name := range names {
		if err := m.StopServer(name); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("mcp manager: stop all: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetAllTools aggregates tools from all running servers and returns the combined
// list. Tool names are prefixed with the server name using the "__" separator
// (e.g. "github__create_issue") to avoid collisions across servers.
//
// Exception: when a server has exactly one tool whose name matches the server
// name, the tool is included without a prefix.
func (m *Manager) GetAllTools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []Tool
	for serverName, client := range m.clients {
		if !client.IsRunning() {
			continue
		}
		tools := client.Tools()
		for _, t := range tools {
			prefixed := t
			prefixed.Name = combinedToolName(serverName, t.Name, tools)
			all = append(all, prefixed)
		}
	}
	return all
}

// GetAllResources aggregates resources from all running servers.
func (m *Manager) GetAllResources() []Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []Resource
	for _, client := range m.clients {
		if !client.IsRunning() {
			continue
		}
		all = append(all, client.Resources()...)
	}
	return all
}

// GetAllPrompts aggregates prompts from all running servers.
func (m *Manager) GetAllPrompts() []Prompt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []Prompt
	for _, client := range m.clients {
		if !client.IsRunning() {
			continue
		}
		all = append(all, client.Prompts()...)
	}
	return all
}

// CallTool routes a tool call to the correct server. toolName may be either a
// plain server tool name stored in the index or a prefixed name of the form
// "servername__toolname". The original (unprefixed) tool name is forwarded to
// the server.
func (m *Manager) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (*CallToolResult, error) {
	serverName, originalName, err := m.resolveToolRoute(toolName)
	if err != nil {
		return nil, err
	}

	m.mu.RLock()
	client, ok := m.clients[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("mcp manager: server %q not found for tool %q", serverName, toolName)
	}
	if !client.IsRunning() {
		return nil, fmt.Errorf("mcp manager: server %q is not running", serverName)
	}

	return client.CallTool(ctx, originalName, args)
}

// ReadResource tries each running server that advertises resources until one
// returns a result for the given uri. Returns an error if no server can serve
// the resource.
func (m *Manager) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	m.mu.RLock()
	candidates := make([]*Client, 0, len(m.clients))
	for _, client := range m.clients {
		if client.IsRunning() && len(client.Resources()) > 0 {
			candidates = append(candidates, client)
		}
	}
	m.mu.RUnlock()

	if len(candidates) == 0 {
		return nil, fmt.Errorf("mcp manager: no running server has resources available")
	}

	var lastErr error
	for _, client := range candidates {
		result, err := client.ReadResource(ctx, uri)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("mcp manager: no server could read resource %q: %w", uri, lastErr)
}

// GetPrompt finds the running server that advertises a prompt with the given
// name and asks it to render the prompt with the supplied arguments.
func (m *Manager) GetPrompt(ctx context.Context, name string, args map[string]string) (*GetPromptResult, error) {
	m.mu.RLock()
	var target *Client
	for _, client := range m.clients {
		if !client.IsRunning() {
			continue
		}
		for _, p := range client.Prompts() {
			if p.Name == name {
				target = client
				break
			}
		}
		if target != nil {
			break
		}
	}
	m.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("mcp manager: prompt %q not found on any running server", name)
	}

	return target.GetPrompt(ctx, name, args)
}

// ListServers returns a snapshot of the status of every registered server.
func (m *Manager) ListServers() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]ServerStatus, 0, len(m.clients))
	for name, client := range m.clients {
		running := client.IsRunning()
		status := ServerStatus{
			Name:    name,
			Running: running,
		}
		if running {
			status.Tools = len(client.Tools())
			status.Resources = len(client.Resources())
			status.Prompts = len(client.Prompts())
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// RebuildIndex rebuilds the internal tool-routing index from all running
// clients. Callers can use this after live capability updates (e.g. when a
// server signals tools/list_changed).
func (m *Manager) RebuildIndex() {
	m.rebuildIndex()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// rebuildIndex rebuilds the toolIndex map under the write lock. It maps every
// combined tool name (as produced by combinedToolName) to the server name that
// owns that tool.
func (m *Manager) rebuildIndex() {
	m.mu.Lock()
	defer m.mu.Unlock()

	index := make(map[string]string, len(m.toolIndex))
	for serverName, client := range m.clients {
		if !client.IsRunning() {
			continue
		}
		tools := client.Tools()
		for _, t := range tools {
			combined := combinedToolName(serverName, t.Name, tools)
			index[combined] = serverName
		}
	}
	m.toolIndex = index
}

// resolveToolRoute returns the server name and original (unprefixed) tool name
// for the given toolName. It first consults the toolIndex for an exact match.
// If no match is found it attempts to split on the separator "__" and look up
// the left-hand side as a server name.
func (m *Manager) resolveToolRoute(toolName string) (serverName, originalName string, err error) {
	// Fast path: exact lookup in the pre-built index.
	m.mu.RLock()
	srvName, ok := m.toolIndex[toolName]
	m.mu.RUnlock()

	if ok {
		// Determine the original (unprefixed) tool name for the server.
		if idx := strings.Index(toolName, separator); idx != -1 {
			return srvName, toolName[idx+len(separator):], nil
		}
		// The combined name had no prefix (single-tool same-name rule).
		return srvName, toolName, nil
	}

	// Slow path: try to parse "server__tool" manually.
	if idx := strings.Index(toolName, separator); idx != -1 {
		srvName = toolName[:idx]
		orig := toolName[idx+len(separator):]
		m.mu.RLock()
		_, exists := m.clients[srvName]
		m.mu.RUnlock()
		if exists {
			return srvName, orig, nil
		}
	}

	return "", "", fmt.Errorf("mcp manager: tool %q not found in any running server", toolName)
}

// combinedToolName returns the name under which tool t should appear in the
// aggregated list for serverName.
//
// The no-prefix rule: when the server has exactly one tool and that tool's
// name equals the server name, the tool is returned as-is without a prefix.
func combinedToolName(serverName, toolName string, serverTools []Tool) string {
	if len(serverTools) == 1 && serverTools[0].Name == serverName {
		return toolName
	}
	return serverName + separator + toolName
}
