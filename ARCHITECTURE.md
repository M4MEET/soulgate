# SoulGate Architecture

SoulGate is a local-first agent gateway that runs tool actions through sandboxed plugins with policy enforcement, approvals, and audit logs.

The security posture is built on one principle:

> The model is never trusted. The runtime enforces permissions.

---

## Goals

### What SoulGate guarantees (by design)
- No “total device control” by default
- Default-deny permissions for plugins
- Sandboxed plugin execution
- Brokered access to OS/network/secrets
- Policy enforcement outside the model
- Auditable execution (what happened, not what the model claimed)

### Non-goals
- “Let the model run arbitrary shell commands” as a default feature
- Silent background exfiltration-friendly integrations

---

## High-level component model

### Components
1) **Daemon (Orchestrator)**
   - Owns sessions, runs, tool routing, state, and policy enforcement.
   - Minimal OS privileges.
2) **Model Router**
   - Provider adapters (cloud/local) with a consistent interface.
   - Enforces structured tool-call output and schema validation.
3) **Plugin Runtime**
   - Executes plugins in an isolated sandbox (WASM default).
4) **Brokers**
   - The only route to sensitive resources:
     - FileBroker, NetBroker, SecretBroker, ExecBroker, ApprovalBroker
5) **Policy Engine**
   - Decides allow/deny/require-approval for every broker call.
6) **Audit Log**
   - Flight recorder for all tool actions, approvals, and resource touches.
7) **CLI**
   - Primary control plane for v0.1 (start/stop/run/approvals/policies/audit/plugins).

---

## Trust boundaries

SoulGate treats plugins and model outputs as untrusted.

- The **model** can propose actions.
- A **plugin** can implement tools.
- Only the **daemon + brokers** can actually touch sensitive resources.
- The **policy engine** sits between “request” and “execution”.

---

## Data flow (one tool call)

1) User runs: `soulgate run "..."`  
2) Orchestrator calls the model with:
   - conversation context
   - tool schemas available for this workspace
   - policy constraints (as guidance only)
3) Model responds with:
   - natural language
   - and/or tool calls (structured)
4) Orchestrator validates:
   - tool call schema
   - plugin existence/version pin
5) Orchestrator asks Policy Engine:
   - allow / deny / require approval
6) If approval required:
   - ApprovalBroker creates an approval item
   - user approves/denies via CLI
7) Plugin runs inside sandbox:
   - any sensitive action is requested via brokers
8) Brokers enforce:
   - permission scopes + policy decisions
   - generate audit events
9) Tool result returns to orchestrator
10) Orchestrator optionally calls model to summarize and returns output

---

## Broker model (the security core)

### FileBroker
Responsibilities:
- Enforce path scopes (`read`/`write`) per plugin + policy.
- Offer virtual FS mapping for plugins.
- For writes:
  - stage changes
  - compute diff
  - require approval if policy says so
- Emit audit events:
  - paths read/written
  - write diffs hash/metadata

### NetBroker
Responsibilities:
- Enforce domain allowlists.
- Optional: DNS pinning / IP constraints (enterprise).
- Apply request limits (size/time).
- Emit audit events:
  - domain, method, status, size (redacted payload by policy)

### SecretBroker
Responsibilities:
- Prefer “call on behalf” patterns (plugin never sees raw secrets).
- Provide short-lived tokens when required.
- Emit audit events:
  - secret handle used (never the secret)
  - purpose metadata

### ExecBroker (disabled by default)
Responsibilities:
- If enabled, allow only allowlisted commands and working dirs.
- Always auditable, typically approval-gated.

### ApprovalBroker
Responsibilities:
- Persist approval requests
- Provide CLI-friendly approval display:
  - diffs, destinations, action summaries
- Record approve/deny decisions + reasons

---

## Plugin system

### Plugin packaging
Each plugin has:
- `manifest.yml` describing:
  - identity: name/version/publisher
  - runtime type (WASM/container)
  - permissions requested
  - tools exposed + JSON schemas
- an artifact:
  - WASM binary (recommended) or container image

### Plugin permissions (capabilities)
Permissions are declarative and enforceable.
Examples:
- `files.read`: list of allowed path patterns
- `files.write`: list of allowed path patterns
- `network.request`: list of allowed domains
- `secrets.use`: list of allowed secret handles
- `exec.run`: disabled unless explicitly enabled

Default is **deny** for everything not declared and allowed by policy.

### Plugin execution
- Plugins run out-of-process in a sandbox.
- They cannot access OS resources directly.
- They call brokers through a narrow protocol.

---

## Policy engine

### Policy decisions
For any requested action, policy returns one of:
- `allow`
- `deny`
- `require_approval`

### Matching dimensions (typical)
- plugin identity (publisher/name/version)
- workspace/environment
- action type (files.write, network.request, secrets.use, exec.run)
- resource (path/domain/secret handle)
- user/role (enterprise)

### Enforcement points
- On plugin install/upgrade:
  - reject forbidden permissions
  - reject wildcard permissions in enterprise mode
- On broker call:
  - final enforcement before any resource is touched

---

## Audit logging

Audit is a first-class product feature.

### What gets logged
- run/session metadata
- tool calls requested/executed
- plugin identity/version/hash
- policy decision and approval outcome
- resources touched (paths/domains/secret handles)

### Redaction
- Secrets and sensitive payloads are redacted by policy.
- Logs should be exportable without leaking credentials.

### Export and sinks
- v0.1: SQLite + JSONL export
- v0.2+: syslog/webhook sinks; SIEM integrations

---

## Model Router

### Provider adapters
SoulGate supports multiple inference backends via adapters:
- cloud APIs
- local servers (e.g., Ollama)
- VPC inference servers (e.g., vLLM)

### Strict tool calling
- Tool calls must be valid JSON conforming to the tool schema.
- Invalid tool calls are rejected (and optionally re-asked).

### Role routing (optional)
- planner: higher quality
- tool-call formatter: cheaper/faster
- summarizer: cheaper

---

## CLI control plane

CLI is the primary interface in early releases.

Core command groups:
- lifecycle: `init/start/stop/status`
- runs: `run`, `session list`, `session tail`
- approvals: `approvals list/watch/approve/deny`
- plugins: `plugin install/info/upgrade/remove/verify`
- policy: `policy show/apply/test/validate`
- audit: `audit tail/query/export`

All “list/query” commands should support `--json` for scripting.

---

## Repository structure (recommended)

/cmd
  soulgate/               daemon
  soulgate-cli/           optional thin client (or same binary)
/internal
  api/                    local API handlers
  core/                   sessions, orchestration, runs
  model/                  provider adapters + router
  policy/                 rules + matching + enforcement
  audit/                  event schema + sinks
  approvals/              queue + workflows
  plugins/
    loader/               install/discovery/lockfile
    runtime/              wasm/container runners
    sdk/                  contracts/schemas
  brokers/
    files/
    net/
    secrets/
    exec/
/plugins
  examples/
  official/
/docs
  ARCHITECTURE.md
  TODO.md
  SECURITY.md
  PLUGIN_SDK.md
  POLICY.md

---

## Security posture summary

SoulGate differs from "device-control" agents by construction:
- plugins are sandboxed,
- permissions are explicit and enforced,
- sensitive actions go through brokers,
- approvals can be mandatory,
- everything is auditable.

This makes third-party plugins possible without granting blanket control of the host.

---

## Recommended Tech Stack (v0.1)

### Core System
- **Language**: Go 1.22+ (daemon, CLI, brokers, orchestrator)
  - Single static binary
  - Strong concurrency support
  - Excellent CLI/daemon ecosystem
  - Easy cross-platform deployment

- **Plugin Runtime**: WASM via [wazero](https://wazero.io/)
  - Pure Go WASM runtime (no CGO dependencies)
  - Strong sandbox boundary for third-party plugins
  - Resource limits (memory, CPU, timeouts)
  - Clean host function interface for brokers

- **Storage**: SQLite ([modernc.org/sqlite](https://modernc.org/sqlite))
  - Pure Go implementation
  - Perfect for local-first architecture
  - Audit logs, sessions, approval queue
  - No external dependencies

- **CLI Framework**: [Cobra](https://github.com/spf13/cobra)
  - Industry standard for Go CLIs
  - Built-in help, completions, flags

- **Configuration**: YAML + environment overrides
  - Human-readable policies
  - Easy to version control

### Plugin Development
- **v0.1**: Rust → WASM (best developer experience for WASM)
- **Future**: TypeScript, Go, or HTTP/gRPC connector plugins

### Why This Stack
This combination provides the fastest path to a secure, production-ready v0.1:
- **Go**: Fast development, single binary, strong security ecosystem
- **WASM**: True sandboxing without containers or complex runtimes
- **SQLite**: Zero-ops embedded database, perfect for local-first
- **wazero**: Pure Go means no CGO, easier builds, better portability

---

## Getting Started with Development

### Prerequisites
- Go 1.22 or later
- Rust toolchain (for building plugins)
- Git

### Quick Start

```bash
# 1. Initialize Go module
cd /Users/demon/soulGate
go mod init github.com/yourusername/soulgate

# 2. Add core dependencies
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get gopkg.in/yaml.v3@latest
go get modernc.org/sqlite@latest
go get go.uber.org/zap@latest
go get github.com/tetratelabs/wazero@latest
go get github.com/gobwas/glob@latest
go get github.com/santhosh-tekuri/jsonschema/v5@latest
go get github.com/stretchr/testify@latest

# 3. Create directory structure (see Repository Structure above)
mkdir -p cmd/soulgate/cmd
mkdir -p internal/{core,model,policy,audit,brokers/files,plugins/{sdk,loader,runtime}}
mkdir -p plugins/examples/file_reader/src
mkdir -p docs demo

# 4. Start with Phase 1: Core Foundation
# See implementation plan for detailed guidance
```

### Development Phases

The project is structured into 8 implementation phases (see detailed implementation plan):

**Phase 0: Bootstrap** (Day 1)
- Set up Go module and dependencies
- Create directory structure

**Phase 1: Foundation** (Days 2-4)
- Orchestrator and session management
- Audit logging system

**Phase 2: Model Router** (Days 5-7)
- OpenAI and Anthropic adapters
- Tool calling support

**Phase 3: Policy Engine** (Days 8-9)
- Policy evaluation
- Pattern matching

**Phase 4: FileBroker** (Days 10-12)
- Read-only file operations
- Security enforcement

**Phase 5: Plugin System** (Days 13-16)
- WASM runtime with wazero
- Host function interface

**Phase 6: Example Plugin** (Days 17-18)
- Rust-based file_reader plugin
- Demonstrates WASM SDK

**Phase 7: CLI** (Days 19-21)
- Command implementation
- User interface

**Phase 8: Integration** (Days 22-23)
- End-to-end wiring
- Demo creation

### Target Demo

The v0.1 goal is to achieve this working demo:

```bash
# User runs a prompt
soulgate run "Read the file example.txt and tell me its contents"

# Behind the scenes:
# 1. Model (GPT-4/Claude) receives prompt + tool schemas
# 2. Model responds with tool call: read_file(path="example.txt")
# 3. Orchestrator validates tool call
# 4. Policy engine evaluates: files.read on "./example.txt" → allow
# 5. Plugin (WASM) executes via runtime
# 6. Plugin calls FileBroker via host function
# 7. FileBroker reads file and returns contents
# 8. Model receives result and summarizes for user
# 9. Audit log captures entire flow

# User sees response
"The file contains: [content summary]"

# Verify in audit log
soulgate audit tail --last 5
```

### Critical Security Requirements

Before v0.1 release, verify:
- [ ] FileBroker prevents path traversal (`../../etc/passwd` blocked)
- [ ] Workspace boundaries enforced (cannot read outside workspace)
- [ ] Policy defaults to deny
- [ ] WASM plugins cannot access OS directly
- [ ] All broker calls logged to audit database
- [ ] Tool schemas validated (never trust model output)

### Documentation

See detailed implementation plan in `.claude/plans/` for:
- Detailed component designs
- Code examples and interfaces
- Testing strategies
- Timeline and milestones

---

## Contributing

### Code Organization Principles
- **Separation of concerns**: Model, policy, execution, audit are independent
- **Security by default**: Default-deny, explicit permissions, mandatory validation
- **Testability**: All components have clean interfaces for mocking
- **Auditability**: Every action leaves a trace

### Security Guidelines
- Never trust model output without validation
- All file access must go through FileBroker
- All operations must pass through policy engine
- No direct OS access from plugins
- Audit events must be emitted before and after sensitive operations

### Testing Requirements
- Unit tests for all core logic
- Security tests for all brokers (especially path traversal)
- Integration tests for end-to-end flows
- WASM plugin sandbox tests

---

## Project Status

**Current Status**: Architecture designed, ready for implementation

**Next Milestone**: v0.1 - Working prototype with:
- File reading via WASM plugins
- Policy enforcement
- Audit logging
- CLI interface

**Estimated Timeline**: 5-6 weeks to v0.1

**Future Roadmap** (v0.2+):
- File write operations with approval workflow
- NetBroker, SecretBroker, ExecBroker
- Web UI
- Plugin marketplace
- Advanced policy conditions
- SIEM integrations
