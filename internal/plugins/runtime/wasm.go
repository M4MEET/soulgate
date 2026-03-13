package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	"github.com/M4MEET/soulgate/internal/plugins/loader"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMRuntime implements the Runtime interface using wazero
type WASMRuntime struct {
	runtime    wazero.Runtime
	plugins    map[string]*wasmPlugin
	fileBroker *files.Broker
	brokerCtx  brokers.BrokerContext
}

// wasmPlugin represents a loaded WASM plugin
type wasmPlugin struct {
	id       string
	module   api.Module
	manifest *loader.Plugin
}

// NewWASMRuntime creates a new WASM runtime
func NewWASMRuntime(ctx context.Context, fileBroker *files.Broker, brokerCtx brokers.BrokerContext) (*WASMRuntime, error) {
	// Create wazero runtime with default configuration
	r := wazero.NewRuntime(ctx)

	runtime := &WASMRuntime{
		runtime:    r,
		plugins:    make(map[string]*wasmPlugin),
		fileBroker: fileBroker,
		brokerCtx:  brokerCtx,
	}

	return runtime, nil
}

// LoadPlugin loads a WASM plugin
func (r *WASMRuntime) LoadPlugin(ctx context.Context, plugin *loader.Plugin) error {
	pluginID := plugin.GetID()

	// Check if already loaded
	if _, exists := r.plugins[pluginID]; exists {
		return fmt.Errorf("plugin already loaded: %s", pluginID)
	}

	// Read WASM file
	wasmBytes, err := os.ReadFile(plugin.WASMPath)
	if err != nil {
		return fmt.Errorf("failed to read WASM file: %w", err)
	}

	// Instantiate WASI
	wasi_snapshot_preview1.MustInstantiate(ctx, r.runtime)

	// Create host module with broker functions
	if err := r.createHostModule(ctx, pluginID); err != nil {
		return fmt.Errorf("failed to create host module: %w", err)
	}

	// Compile WASM module
	compiledModule, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile WASM module: %w", err)
	}

	// Instantiate module
	config := wazero.NewModuleConfig().
		WithName(pluginID).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr)

	module, err := r.runtime.InstantiateModule(ctx, compiledModule, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	// Store plugin
	r.plugins[pluginID] = &wasmPlugin{
		id:       pluginID,
		module:   module,
		manifest: plugin,
	}

	return nil
}

// ExecuteTool executes a tool in a plugin
func (r *WASMRuntime) ExecuteTool(ctx context.Context, pluginID, toolName string, input json.RawMessage) (json.RawMessage, error) {
	// Get plugin
	plugin, exists := r.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not loaded: %s", pluginID)
	}

	// For v0.1, we'll use a simplified approach:
	// Return a mock response since implementing the full WASM bridge is complex
	// In a production system, this would:
	// 1. Marshal toolName and input to WASM memory
	// 2. Call exported execute_tool function
	// 3. Read result from WASM memory
	// 4. Unmarshal and return

	_ = plugin // Use plugin variable

	// Mock response for now
	result := map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Tool %s executed (mock response for v0.1)", toolName),
		"input":   json.RawMessage(input),
	}

	output, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return output, nil
}

// UnloadPlugin unloads a plugin
func (r *WASMRuntime) UnloadPlugin(ctx context.Context, pluginID string) error {
	plugin, exists := r.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not loaded: %s", pluginID)
	}

	// Close module
	if err := plugin.module.Close(ctx); err != nil {
		return fmt.Errorf("failed to close module: %w", err)
	}

	// Remove from map
	delete(r.plugins, pluginID)

	return nil
}

// Close closes the runtime
func (r *WASMRuntime) Close(ctx context.Context) error {
	// Close all plugins
	for pluginID := range r.plugins {
		r.UnloadPlugin(ctx, pluginID)
	}

	// Close runtime
	return r.runtime.Close(ctx)
}

// createHostModule creates host functions that plugins can import
func (r *WASMRuntime) createHostModule(ctx context.Context, pluginID string) error {
	// For v0.1, we'll create a minimal host module
	// In production, this would include:
	// - files_read(path_ptr, path_len) -> (content_ptr, content_len, error)
	// - files_list(path_ptr, path_len) -> (list_ptr, list_len, error)
	// - files_stat(path_ptr, path_len) -> (stat_ptr, stat_len, error)

	_, err := r.runtime.NewHostModuleBuilder("soulgate").
		// Export empty module for now - full implementation in v0.2
		Instantiate(ctx)

	return err
}
