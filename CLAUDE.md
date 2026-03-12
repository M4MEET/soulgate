# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

SoulGate is a **security-focused agent gateway** that sits between LLM agents and system resources. The core principle: *The model is never trusted. The runtime enforces permissions.*

All agent operations flow through:
1. **Policy Engine** - evaluates allow/deny/require-approval decisions
2. **Resource Brokers** - mediate access to files, network, secrets, execution
3. **Audit Logger** - records every operation to SQLite
4. **Plugin Runtime** - executes WASM plugins in sandboxed isolation

## Common Commands

### Build and Test
```bash
# Build the binary
make build              # Creates bin/soulgate

# Run all tests
make test               # Unit + integration tests

# Run specific test suites
make test-unit          # Unit tests only
make test-security      # Critical security tests (path traversal, etc.)
make test-coverage      # Generate coverage report

# Per-component testing (for parallel development)
make test-audit         # Audit logging tests
make test-policy        # Policy engine tests
make test-broker        # FileBroker security tests
make test-plugin        # Plugin system tests
make test-model         # Model provider tests
make test-core          # Orchestrator tests
make test-cli           # CLI tests

# Quality checks
make lint               # Format and vet code
make check              # Run lint + test-unit + test-security
```

### Running SoulGate
```bash
# Initialize a workspace
cd demo/workspace
../../bin/soulgate init

# Run a prompt (requires OPENAI_API_KEY or ANTHROPIC_API_KEY)
export OPENAI_API_KEY="sk-..."
../../bin/soulgate run "Read example.txt and summarize"

# View policies and audit logs
../../bin/soulgate policy show
../../bin/soulgate audit tail --last 10
../../bin/soulgate plugin list
```

## Architecture

### Core Data Flow
```
User → CLI → Orchestrator → Model Provider (OpenAI/Anthropic)
                ↓
         Plugin Runtime (WASM)
                ↓
         Resource Brokers → Policy Engine → Audit Log
                ↓
         System Resources
```

### Key Components

**Orchestrator** (`internal/core/orchestrator.go`)
- Coordinates model calls, plugin execution, and tool routing
- Creates sessions and runs
- Integrates with audit logger
- Entry point: `NewOrchestrator(workspace)` → `Run(ctx, prompt)`

**Policy Engine** (`internal/policy/`)
- Returns `allow`, `deny`, or `require_approval` for each operation
- Evaluates rules with priority ordering (higher priority = evaluated first)
- Pattern matching for resources (glob patterns)
- Default-deny security model

**Resource Brokers** (`internal/brokers/`)
- **FileBroker** (`files/broker.go`): Mediates file operations with security enforcement
  - Path validation prevents traversal attacks (`../../etc/passwd`)
  - Workspace boundary enforcement
  - All operations checked against policy
  - Critical tests: `TestFileBrokerPathTraversal`, `TestFileBrokerPermissions`

**Plugin Runtime** (`internal/plugins/`)
- WASM execution via wazero (pure Go, no CGO)
- Plugins declare permissions in `manifest.yml`
- Cannot access OS directly - must call brokers through host functions
- Loader (`loader/`): Plugin discovery and validation
- Runtime (`runtime/`): Sandbox execution

**Audit Logger** (`internal/audit/`)
- SQLite-based (`audit.db`)
- Records: sessions, runs, tool calls, policy decisions, resources accessed
- Event model: `NewEvent(type, category).WithSessionID().WithRunID().WithMetadata()`
- Query interface for forensics

**Model Providers** (`internal/model/`)
- Adapters for OpenAI (`openai/client.go`) and Anthropic (`anthropic/client.go`)
- Unified interface: `provider.Provider`
- Tool calling: Converts plugin schemas to model-specific formats
- Schema validation: Never trust model output without validation

### Configuration

**Workspace Structure**
```
.soulgate/
├── config.yml       # Workspace configuration
├── policy.yml       # Security policies
└── audit.db         # Audit log (SQLite)
plugins/             # Installed plugins
```

**Config** (`.soulgate/config.yml`)
- Model provider selection (openai/anthropic)
- API keys (prefer env vars: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`)
- Plugin settings (timeout, memory limits)
- Audit settings

**Policy** (`.soulgate/policy.yml`)
```yaml
version: "1"
policies:
  - name: "allow-workspace-reads"
    action: "files.read"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "deny-parent-access"
    action: "files.*"
    resource: "../**"
    decision: deny
    priority: 20  # Higher priority wins
```

## Security Model

### Trust Boundaries
- **Model output**: Never trusted - all tool calls validated against schemas
- **Plugins**: Untrusted - run in WASM sandbox, cannot access OS directly
- **Brokers**: Trusted - only route to sensitive resources
- **Policy Engine**: Trusted - final enforcement before resource access

### Security Requirements
Before any release, verify:
- ✅ Path traversal blocked (`../../etc/passwd`)
- ✅ Workspace boundaries enforced (cannot escape workspace)
- ✅ Default-deny policy (no rule = denied)
- ✅ All broker calls logged to audit
- ✅ Tool schemas validated
- ✅ WASM plugins cannot access OS

Run security tests: `make test-security`

### Critical Security Tests
- `TestFileBrokerPathTraversal` - Path traversal attack prevention
- `TestFileBrokerPermissions` - Policy enforcement
- `TestPolicyEngineDefaultDeny` - Default-deny behavior
- `TestPluginSandboxIsolation` - WASM isolation

## Code Patterns

### Error Handling
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Log errors to audit: `audit.NewEvent(audit.EventError, category).WithError(err)`
- Return errors to CLI for user visibility

### Audit Logging
Every sensitive operation must be audited:
```go
event := audit.NewEvent(audit.EventFileRead, audit.CategoryFile).
    WithSessionID(sessionID).
    WithRunID(runID).
    WithMetadata("path", path).
    WithStatus(audit.StatusSuccess)
if err := logger.Log(ctx, event); err != nil {
    return fmt.Errorf("failed to log: %w", err)
}
```

### Policy Checks
All broker operations must check policy:
```go
decision, err := policyEngine.Evaluate(ctx, &policy.Request{
    Action:    "files.read",
    Resource:  normalizedPath,
    Plugin:    pluginName,
})
if decision == policy.Deny {
    return fmt.Errorf("access denied by policy")
}
```

### Path Validation
FileBroker must validate all paths:
```go
// In internal/brokers/files/permissions.go
normalizedPath, err := validatePath(requestedPath, workspaceRoot)
if err != nil {
    return "", fmt.Errorf("invalid path: %w", err)
}
```

## Development Workflow

### Adding a New Broker
1. Create package in `internal/brokers/<name>/`
2. Implement broker interface with policy checks
3. Add audit logging for all operations
4. Write security tests (especially for injection/traversal)
5. Add to orchestrator's broker registry

### Adding a New Policy Action
1. Define action constant in `internal/policy/policy.go`
2. Add matching logic in `internal/policy/matcher.go`
3. Update policy examples in `.soulgate/policy.example.yml`
4. Add tests in `internal/policy/engine_test.go`

### Integrating a New Model Provider
1. Create adapter in `internal/model/<provider>/client.go`
2. Implement `provider.Provider` interface
3. Add tool calling support (convert schemas to provider format)
4. Add to provider registry in `internal/model/provider.go`
5. Update config schema in `internal/config/config.go`

### Testing Strategy
- **Unit tests**: Each package has `*_test.go` files
- **Security tests**: Use `-tags=security` for security-critical tests
- **Integration tests**: `tests/integration/` (coming in v0.2)
- **Coverage**: Aim for 80%+ on core security components

## Current Status (v0.1)

**Implemented:**
- ✅ Core orchestrator and session management
- ✅ Policy engine with priority-based evaluation
- ✅ FileBroker with security enforcement
- ✅ Audit logging to SQLite
- ✅ CLI commands (init, run, policy, plugin, audit)
- ✅ OpenAI and Anthropic adapters (basic structure)
- ✅ Plugin manifest loading and discovery
- ✅ WASM runtime structure

**Simplified for v0.1:**
- Model integration returns mock responses (can be completed with API keys)
- WASM bridge structure exists but full memory bridge is v0.2
- Only read operations (write requires approval workflow in v0.2)

**Next Phase (v0.2):**
- File write operations with approval workflow
- NetBroker for HTTP requests
- SecretBroker for credential management
- ExecBroker for command execution (disabled by default)
- Full WASM plugin bridge

## Important Files

**Entry Points:**
- `cmd/soulgate/main.go` - CLI entry point
- `cmd/soulgate/cmd/root.go` - Root command and global flags

**Core Logic:**
- `internal/core/orchestrator.go` - Main coordination logic
- `internal/policy/engine.go` - Policy evaluation
- `internal/brokers/files/broker.go` - File access with security
- `internal/audit/logger.go` - Audit logging interface
- `internal/audit/sqlite.go` - SQLite implementation

**Configuration:**
- `internal/config/config.go` - Config schema
- `internal/config/workspace.go` - Workspace initialization
- `.soulgate/config.example.yml` - Config template
- `.soulgate/policy.example.yml` - Policy template

## Dependencies

Key dependencies (see `go.mod`):
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/tetratelabs/wazero` - WASM runtime (pure Go)
- `modernc.org/sqlite` - SQLite database (pure Go)
- `go.uber.org/zap` - Structured logging
- `github.com/gobwas/glob` - Pattern matching
- `github.com/santhosh-tekuri/jsonschema/v5` - Schema validation
- `github.com/stretchr/testify` - Testing utilities

## Additional Documentation

- `ARCHITECTURE.md` - Detailed architecture and design decisions
- `QUICKSTART.md` - Complete walkthrough of features
- `README.md` - Project overview and setup
- `SECURITY.md` - Security model and threat analysis
- `demo/README.md` - Demo workspace walkthrough
