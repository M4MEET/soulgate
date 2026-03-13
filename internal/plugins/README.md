# Plugin Agent

**Owner**: Agent 5 - Plugin Specialist
**Independence Level**: MEDIUM
**Status**: Active (Under Development)

## Overview

Plugin system, WASM runtime, and SDK. Enables extensibility through WebAssembly plugins with secure sandboxing and host function access.

## Responsibilities

- Plugin manifest schema and validation
- WASM runtime and execution
- Plugin loader and lifecycle
- Host function implementation (broker access)
- Rust SDK for plugin development

## Package Structure

```
internal/plugins/
├── README.md
├── sdk/
│   ├── manifest.go        # Plugin manifest (CRITICAL INTERFACE)
│   └── protocol.go        # Plugin protocol
├── loader/
│   ├── loader.go          # Plugin loading
│   └── validator.go       # Manifest validation
└── runtime/
    ├── runtime.go         # Runtime interface
    └── wasm.go            # WASM implementation (SECURITY-CRITICAL)
```

## Core Interfaces

```go
type Manifest struct {
    Name        string
    Version     string
    Description string
    Runtime     RuntimeType
    Entrypoint  string
    Tools       []ToolManifest
    Permissions []Permission
}
```

**WARNING**: Manifest schema affects external plugin developers. Version carefully.

```go
type Runtime interface {
    LoadPlugin(ctx, plugin) error
    ExecuteTool(ctx, pluginID, toolName, input) (output, error)
    UnloadPlugin(ctx, pluginID) error
    Close(ctx) error
}
```

## Dependencies

- `internal/audit` (plugin event logging)
- WASM runtime library (TBD: wasmer, wasmtime, or wazero)

## Security Requirements

**CRITICAL**: Plugins must be sandboxed:
- No direct host filesystem access
- No command execution
- No unbounded resource usage
- Permission-gated broker access

## Usage

```go
// Load plugin
runtime := wasm.NewRuntime(config)
err := runtime.LoadPlugin(ctx, plugin)

// Execute tool
result, err := runtime.ExecuteTool(ctx, "plugin-id", "tool-name", input)
```

## Testing

**Coverage Target**: 85%+
**Current**: No tests (needs comprehensive suite)

### Security Test Requirements
- Sandbox escape attempts
- Resource exhaustion tests
- Permission bypass tests
- Memory safety tests

## Planned Work

- [ ] Complete WASM memory bridge
- [ ] Implement broker host functions
- [ ] Build Rust SDK
- [ ] Add plugin versioning
- [ ] HTTP connector runtime (alternative to WASM)

## Coordination

- Manifest changes → breaking change for plugin developers
- Protocol changes → update SDK and examples
- Host functions → coordinate with Broker agents

## Security Review

ALL changes require security review.

## Contact

**Owner**: @plugin-agent
**Security**: @security-reviewer
