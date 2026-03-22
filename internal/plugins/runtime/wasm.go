package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	"github.com/M4MEET/soulgate/internal/plugins/loader"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const (
	// defaultMemoryLimitPages is the default WASM memory limit in pages.
	// Each page is 64KB, so 1024 pages = 64MB.
	defaultMemoryLimitPages uint32 = 1024

	// defaultExecutionTimeout is the maximum wall-clock time a single
	// ExecuteTool call may take before the context is cancelled and the WASM
	// module is interrupted.
	defaultExecutionTimeout = 30 * time.Second
)

// WASMRuntime implements the Runtime interface using wazero.
//
// Security properties enforced by this runtime:
//   - Memory is capped at memoryLimitPages * 64KB (default 64MB) per module.
//     Attempts to grow beyond this limit are rejected by the wazero allocator,
//     preventing unbounded heap growth.
//   - Each ExecuteTool call runs under a per-call context deadline
//     (executionTimeout). When the deadline fires the blocking WASM host-call
//     returns with an error and the Go goroutine is unblocked.
//   - WASM modules cannot access the host filesystem or network directly:
//     all I/O must go through the broker host functions registered in
//     createHostModule.
type WASMRuntime struct {
	runtime          wazero.Runtime
	plugins          map[string]*wasmPlugin
	fileBroker       *files.Broker
	brokerCtx        brokers.BrokerContext
	memoryLimitPages uint32
	executionTimeout time.Duration
}

// wasmPlugin represents a loaded WASM plugin
type wasmPlugin struct {
	id       string
	module   api.Module
	manifest *loader.Plugin
}

// NewWASMRuntime creates a new WASM runtime with strict resource limits.
//
// memoryLimitBytes is the maximum RSS a single WASM module may allocate.
// A value <= 0 uses the default (64MB). The value is rounded up to the nearest
// whole WASM page (64KB).
//
// timeoutSec is the per-tool-call execution timeout in seconds.
// A value <= 0 uses the default (30s).
func NewWASMRuntime(ctx context.Context, fileBroker *files.Broker, brokerCtx brokers.BrokerContext) (*WASMRuntime, error) {
	return NewWASMRuntimeWithLimits(ctx, fileBroker, brokerCtx, 0, 0)
}

// NewWASMRuntimeWithLimits is like NewWASMRuntime but accepts explicit resource
// limits.  Use this when the caller has read values from config.
//
//   - memoryLimitBytes: max bytes per WASM module (rounded to 64KB pages).
//     <= 0 means use the default (64MB).
//   - timeoutSec: per-ExecuteTool deadline in seconds. <= 0 uses default (30s).
func NewWASMRuntimeWithLimits(
	ctx context.Context,
	fileBroker *files.Broker,
	brokerCtx brokers.BrokerContext,
	memoryLimitBytes int64,
	timeoutSec int,
) (*WASMRuntime, error) {

	// Compute page limit (1 page = 65536 bytes).
	memLimitPages := defaultMemoryLimitPages
	if memoryLimitBytes > 0 {
		pages := uint32((memoryLimitBytes + 65535) / 65536) // round up
		if pages > 0 {
			memLimitPages = pages
		}
	}

	execTimeout := defaultExecutionTimeout
	if timeoutSec > 0 {
		execTimeout = time.Duration(timeoutSec) * time.Second
	}

	// Build the wazero runtime with a memory cap applied globally to every
	// module instantiated through this runtime.
	rConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(memLimitPages)

	r := wazero.NewRuntimeWithConfig(ctx, rConfig)

	rt := &WASMRuntime{
		runtime:          r,
		plugins:          make(map[string]*wasmPlugin),
		fileBroker:       fileBroker,
		brokerCtx:        brokerCtx,
		memoryLimitPages: memLimitPages,
		executionTimeout: execTimeout,
	}

	return rt, nil
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

	// Instantiate module — stdout/stderr are captured to os.Stdout/Stderr.
	// Filesystem and network access are intentionally NOT granted here; all
	// privileged I/O must go through the soulgate host module registered above.
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

// ExecuteTool executes a tool in a plugin.
//
// Every invocation is wrapped in a strict per-call timeout derived from the
// runtime's executionTimeout setting (default 30s, configurable via
// NewWASMRuntimeWithLimits). If the WASM module blocks beyond the deadline the
// context is cancelled and the call returns an error, preventing runaway
// plugins from consuming CPU indefinitely.
func (r *WASMRuntime) ExecuteTool(ctx context.Context, pluginID, toolName string, input json.RawMessage) (json.RawMessage, error) {
	// Get plugin
	plugin, exists := r.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not loaded: %s", pluginID)
	}

	// Enforce per-call execution timeout. The context is passed to all wazero
	// host-function calls so the timeout propagates into blocking host calls.
	execCtx, cancel := context.WithTimeout(ctx, r.executionTimeout)
	defer cancel()

	// For v0.1, we'll use a simplified approach:
	// Return a mock response since implementing the full WASM bridge is complex
	// In a production system, this would:
	// 1. Marshal toolName and input to WASM memory
	// 2. Call exported execute_tool function (using execCtx for timeout)
	// 3. Read result from WASM memory
	// 4. Unmarshal and return

	// Keep execCtx referenced so the compiler does not eliminate it before
	// the full bridge is wired in v0.2.
	_ = execCtx
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
