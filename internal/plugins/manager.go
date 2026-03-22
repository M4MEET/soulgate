// Package plugins provides the unified plugin manager for SoulGate.
//
// Plugins extend SoulGate with new tools. Two runtimes are supported:
//
//   - script (default): The AI can create these on the fly. Each tool has a
//     "command" that is executed with JSON input on stdin. Simple, fast, any language.
//
//   - wasm: Sandboxed execution via wazero. Safer for untrusted third-party plugins.
//     Full WASM bridge is a future milestone.
//
// Directory layout:
//
//	plugins/
//	  weather/
//	    manifest.yml   # Declares tools, runtime, requirements
//	    run.py         # Script entrypoint (for script plugins)
//	  calculator/
//	    manifest.yml
//	    plugin.wasm    # WASM binary (for wasm plugins)
package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/plugins/sdk"
	"gopkg.in/yaml.v3"
)

// Manager loads, owns, and executes plugin tools.
type Manager struct {
	mu        sync.RWMutex
	pluginDir string
	plugins   map[string]*Plugin // name -> plugin
	toolIndex map[string]string  // qualified tool name -> plugin name
	timeout   time.Duration
}

// Plugin represents a loaded plugin.
type Plugin struct {
	Dir      string
	Manifest *sdk.Manifest
}

// ToolSchema is the shape returned to the orchestrator for tool registration.
type ToolSchema struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// NewManager creates a plugin manager for the given directory.
func NewManager(pluginDir string, timeoutSec int) *Manager {
	timeout := 30 * time.Second
	if timeoutSec > 0 {
		timeout = time.Duration(timeoutSec) * time.Second
	}
	return &Manager{
		pluginDir: pluginDir,
		plugins:   make(map[string]*Plugin),
		toolIndex: make(map[string]string),
		timeout:   timeout,
	}
}

// LoadAll scans the plugin directory and loads every valid plugin.
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		return nil // no plugins dir yet
	}

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("read plugin dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginPath := filepath.Join(m.pluginDir, entry.Name())
		if err := m.loadOne(pluginPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: plugin %s: %v\n", entry.Name(), err)
		}
	}
	return nil
}

// Reload re-scans the plugin directory (called after AI creates a new plugin).
func (m *Manager) Reload() error {
	m.mu.Lock()
	m.plugins = make(map[string]*Plugin)
	m.toolIndex = make(map[string]string)
	m.mu.Unlock()
	return m.LoadAll()
}

func (m *Manager) loadOne(dir string) error {
	manifestPath := filepath.Join(dir, "manifest.yml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	var manifest sdk.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// Resolve YAML input_schema maps → JSON
	if err := sdk.ResolveInputSchemas(manifest.Tools); err != nil {
		return fmt.Errorf("resolve schemas: %w", err)
	}

	if err := validateManifest(&manifest); err != nil {
		return err
	}

	// Default runtime is "script"
	if manifest.Runtime == "" {
		manifest.Runtime = "script"
	}

	// Check requirements
	if err := checkRequirements(manifest.Requires); err != nil {
		return fmt.Errorf("unmet requirements: %w", err)
	}

	plugin := &Plugin{Dir: dir, Manifest: &manifest}
	m.plugins[manifest.Name] = plugin

	// Index tools with plugin prefix: pluginname__toolname
	for _, tool := range manifest.Tools {
		qualifiedName := manifest.Name + "__" + tool.Name
		m.toolIndex[qualifiedName] = manifest.Name
	}

	return nil
}

// GetToolSchemas returns schemas for all loaded plugin tools.
func (m *Manager) GetToolSchemas() []ToolSchema {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var schemas []ToolSchema
	for _, plugin := range m.plugins {
		for _, tool := range plugin.Manifest.Tools {
			qualifiedName := plugin.Manifest.Name + "__" + tool.Name
			desc := tool.Description
			if plugin.Manifest.Description != "" {
				desc = fmt.Sprintf("[%s] %s", plugin.Manifest.Name, tool.Description)
			}
			schemas = append(schemas, ToolSchema{
				Name:        qualifiedName,
				Description: desc,
				InputSchema: tool.InputSchema,
			})
		}
	}
	return schemas
}

// IsPluginTool checks if a tool name belongs to a loaded plugin.
func (m *Manager) IsPluginTool(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.toolIndex[name]
	return ok
}

// ExecuteTool runs a plugin tool and returns the result string.
func (m *Manager) ExecuteTool(ctx context.Context, qualifiedName string, input json.RawMessage) (string, error) {
	m.mu.RLock()
	pluginName, ok := m.toolIndex[qualifiedName]
	if !ok {
		m.mu.RUnlock()
		return "", fmt.Errorf("unknown plugin tool: %s", qualifiedName)
	}
	plugin := m.plugins[pluginName]
	m.mu.RUnlock()

	// Find the tool definition
	parts := strings.SplitN(qualifiedName, "__", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid tool name: %s", qualifiedName)
	}
	toolName := parts[1]

	var toolDef *sdk.ToolDef
	for i := range plugin.Manifest.Tools {
		if plugin.Manifest.Tools[i].Name == toolName {
			toolDef = &plugin.Manifest.Tools[i]
			break
		}
	}
	if toolDef == nil {
		return "", fmt.Errorf("tool %s not found in plugin %s", toolName, pluginName)
	}

	switch plugin.Manifest.Runtime {
	case "script", "":
		return m.executeScript(ctx, plugin, toolDef, input)
	case "wasm":
		return "", fmt.Errorf("WASM runtime not yet implemented — use runtime: script")
	default:
		return "", fmt.Errorf("unsupported runtime: %s", plugin.Manifest.Runtime)
	}
}

// executeScript runs a script plugin tool.
// The tool's command is executed in the plugin directory.
// Input JSON is passed on stdin; stdout is captured as the result.
func (m *Manager) executeScript(ctx context.Context, plugin *Plugin, tool *sdk.ToolDef, input json.RawMessage) (string, error) {
	command := tool.Command
	if command == "" {
		// Fallback: look for entrypoint in manifest
		if plugin.Manifest.Entrypoint != "" {
			command = plugin.Manifest.Entrypoint
		} else {
			return "", fmt.Errorf("no command defined for tool %s", tool.Name)
		}
	}

	// Parse command into program + args
	cmdParts := strings.Fields(command)
	if len(cmdParts) == 0 {
		return "", fmt.Errorf("empty command for tool %s", tool.Name)
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, cmdParts[0], cmdParts[1:]...)
	cmd.Dir = plugin.Dir
	cmd.Stdin = bytes.NewReader(input)

	// Inherit env + add plugin config as env vars
	cmd.Env = os.Environ()
	for key, desc := range plugin.Manifest.Config {
		// Config keys map to env vars — value comes from environment
		_ = desc
		if val := os.Getenv(key); val != "" {
			cmd.Env = append(cmd.Env, key+"="+val)
		}
	}
	// Pass tool name and plugin name as env vars
	cmd.Env = append(cmd.Env, "SOULGATE_TOOL="+tool.Name)
	cmd.Env = append(cmd.Env, "SOULGATE_PLUGIN="+plugin.Manifest.Name)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return "", fmt.Errorf("plugin %s tool %s failed: %s", plugin.Manifest.Name, tool.Name, strings.TrimSpace(errMsg))
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		result = "(no output)"
	}
	return result, nil
}

// ListPlugins returns names of all loaded plugins.
func (m *Manager) ListPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	return names
}

// GetPlugin returns a loaded plugin by name.
func (m *Manager) GetPlugin(name string) (*Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

// --- Validation ---

func validateManifest(m *sdk.Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("version is required")
	}
	if len(m.Tools) == 0 {
		return fmt.Errorf("at least one tool is required")
	}

	seen := make(map[string]bool)
	for _, t := range m.Tools {
		if t.Name == "" {
			return fmt.Errorf("tool name is required")
		}
		if seen[t.Name] {
			return fmt.Errorf("duplicate tool name: %s", t.Name)
		}
		seen[t.Name] = true
		if t.Description == "" {
			return fmt.Errorf("tool %s: description is required", t.Name)
		}
		if len(t.InputSchema) == 0 {
			return fmt.Errorf("tool %s: input_schema is required", t.Name)
		}
	}

	switch m.Runtime {
	case "", "script", "wasm":
		// ok
	default:
		return fmt.Errorf("unsupported runtime: %s (use 'script' or 'wasm')", m.Runtime)
	}

	return nil
}

func checkRequirements(req sdk.RequirementsDef) error {
	for _, envVar := range req.Env {
		if os.Getenv(envVar) == "" {
			return fmt.Errorf("required env var %s not set", envVar)
		}
	}
	for _, bin := range req.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("required binary %s not found on PATH", bin)
		}
	}
	return nil
}
