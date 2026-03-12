# SoulGate Interface Contracts

This document defines the shared interfaces between agents, their stability guarantees, versioning strategy, and change protocols.

## Overview

Interface contracts are the boundaries between agents. Changes to these interfaces require coordination across multiple agents to prevent breaking changes.

## Interface Stability Matrix

| Interface | Owner Agent | Stability | Version | Consumers |
|-----------|------------|-----------|---------|-----------|
| `audit.Logger` | Audit | Stable | v1 | Orchestrator, FileBroker |
| `audit.Event` | Audit | Stable | v1 | ALL agents |
| `audit.QueryFilter` | Audit | Stable | v1 | CLI |
| `policy.Engine` | Policy | Stable | v1 | FileBroker, future brokers |
| `policy.PolicyRequest` | Policy | Stable | v1 | ALL brokers |
| `policy.PolicyResult` | Policy | Stable | v1 | ALL brokers |
| `config.Config` | Config | Stable | v1 | ALL agents |
| `broker.Broker` | FileBroker | Unstable | v0 | Orchestrator, Plugins |
| `broker.Context` | FileBroker | Unstable | v0 | Orchestrator, Plugins |
| `sdk.Manifest` | Plugin | Unstable | v0 | Plugins, Loader |
| `sdk.Protocol` | Plugin | Unstable | v0 | Plugins, Runtime |
| `model.Provider` | Model | Unstable | v0 | Orchestrator |
| `model.ToolSchema` | Model | Unstable | v0 | Orchestrator, Plugins |

**Stability Levels**:
- **Stable**: No breaking changes without major version bump. Backward compatible additions only.
- **Unstable**: May change at any time. Coordinate changes with all consumers.
- **Deprecated**: Will be removed in future version. Migration path provided.

## Critical Interfaces

These interfaces are shared across multiple agents and require strict coordination for any changes.

### 1. Audit Event Schema

**File**: `internal/audit/event.go`
**Owner**: Audit Agent
**Consumers**: ALL agents (any component that logs events)
**Stability**: Stable (v1)

**Interface**:
```go
type Event struct {
    ID          string                 `json:"id"`
    Timestamp   time.Time              `json:"timestamp"`
    SessionID   string                 `json:"session_id,omitempty"`
    RunID       string                 `json:"run_id,omitempty"`
    EventType   EventType              `json:"event_type"`
    Component   string                 `json:"component"`
    Action      string                 `json:"action"`
    Resource    string                 `json:"resource,omitempty"`
    Status      EventStatus            `json:"status"`
    Message     string                 `json:"message,omitempty"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    Error       string                 `json:"error,omitempty"`
}

type EventType string
const (
    EventTypeSession   EventType = "session"
    EventTypeRun       EventType = "run"
    EventTypePolicy    EventType = "policy"
    EventTypeBroker    EventType = "broker"
    EventTypePlugin    EventType = "plugin"
    EventTypeModel     EventType = "model"
)

type EventStatus string
const (
    EventStatusStarted  EventStatus = "started"
    EventStatusSuccess  EventStatus = "success"
    EventStatusFailure  EventStatus = "failure"
    EventStatusDenied   EventStatus = "denied"
)
```

**Change Protocol**:
1. Propose schema change in GitHub issue/discussion
2. Impact analysis: Which agents log events? What breaks?
3. Add new fields (backward compatible) OR create Event v2 with migration
4. Deprecate old fields (2-version cycle minimum)
5. Update all logging call sites across all agents
6. Version the schema in the struct (add version field if needed)

**Example Change Process**:
```
# Adding a new field (SAFE - backward compatible):
1. Add field to Event struct with omitempty tag
2. Update audit agent to handle new field
3. Announce to all agents
4. Agents update at their own pace

# Changing a field type (UNSAFE - breaking):
1. Create Event v2 struct
2. Support both v1 and v2 in audit logger
3. Migrate all consumers to v2
4. Remove v1 after 2 release cycles
```

**Testing Requirements**:
- All agents must pass their existing audit logging tests
- New audit tests for schema changes
- Integration test: full run with audit enabled

---

### 2. Policy Request/Result Types

**File**: `internal/policy/policy.go`
**Owner**: Policy Agent
**Consumers**: ALL broker agents (FileBroker, future NetworkBroker, etc.)
**Stability**: Stable (v1)

**Interface**:
```go
type PolicyRequest struct {
    SessionID   string
    RunID       string
    Actor       string
    Action      string
    Resource    string
    ResourceType string
    Metadata    map[string]string
}

type PolicyResult struct {
    Decision  Decision
    Rule      *Rule
    Reason    string
    Metadata  map[string]interface{}
}

type Decision string
const (
    DecisionAllow Decision = "allow"
    DecisionDeny  Decision = "deny"
    DecisionAsk   Decision = "ask"
)
```

**Change Protocol**:
1. Policy agent proposes change
2. All broker agents review impact
3. Add new fields (backward compatible preferred)
4. Deprecate old fields (2-version cycle)
5. Update all broker call sites

**Example Changes**:
```go
// Adding a new decision type (BREAKING - affects all brokers):
const DecisionAudit Decision = "audit" // allow but log extensively

// Adding optional context (SAFE):
type PolicyRequest struct {
    // ... existing fields ...
    Context map[string]interface{} `json:"context,omitempty"` // NEW
}
```

**Testing Requirements**:
- All broker tests must pass
- Policy engine tests for new decision types
- End-to-end test: run through orchestrator

---

### 3. Plugin Manifest Schema

**File**: `internal/plugins/sdk/manifest.go`
**Owner**: Plugin Agent
**Consumers**: Plugin loader, plugin runtime, external plugin developers
**Stability**: Unstable (v0)

**Interface**:
```go
type Manifest struct {
    Name        string                 `yaml:"name" json:"name"`
    Version     string                 `yaml:"version" json:"version"`
    Description string                 `yaml:"description" json:"description"`
    Author      string                 `yaml:"author,omitempty" json:"author,omitempty"`
    License     string                 `yaml:"license,omitempty" json:"license,omitempty"`
    Runtime     RuntimeType            `yaml:"runtime" json:"runtime"`
    Entrypoint  string                 `yaml:"entrypoint" json:"entrypoint"`
    Tools       []ToolManifest         `yaml:"tools" json:"tools"`
    Permissions []Permission           `yaml:"permissions" json:"permissions"`
    Config      map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

type RuntimeType string
const (
    RuntimeWASM RuntimeType = "wasm"
    RuntimeHTTP RuntimeType = "http"
)

type ToolManifest struct {
    Name        string      `yaml:"name" json:"name"`
    Description string      `yaml:"description" json:"description"`
    InputSchema interface{} `yaml:"input_schema" json:"input_schema"`
}

type Permission struct {
    Broker   string   `yaml:"broker" json:"broker"`
    Actions  []string `yaml:"actions" json:"actions"`
    Resources []string `yaml:"resources,omitempty" json:"resources,omitempty"`
}
```

**Change Protocol**:
1. Plugin agent proposes manifest change
2. Review plugin compatibility impact (breaks external plugins!)
3. Version manifest schema in the manifest itself
4. Support multiple manifest versions during transition
5. Provide migration guide for plugin developers

**Example Versioning**:
```yaml
# manifest.yaml (v1)
manifest_version: "1"
name: "my-plugin"
# ...

# manifest.yaml (v2)
manifest_version: "2"
name: "my-plugin"
# ... new fields ...
```

**Testing Requirements**:
- Manifest parsing tests for all supported versions
- Plugin loader tests with v1 and v2 manifests
- Example plugin updated to new version

---

### 4. Tool Schema Format

**File**: `internal/model/schema.go`
**Owner**: Model Agent
**Consumers**: Model adapters (OpenAI, Anthropic), orchestrator, plugin runtime
**Stability**: Unstable (v0)

**Interface**:
```go
type ToolSchema struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema interface{} `json:"input_schema"` // JSON Schema object
}

// JSON Schema format (matches OpenAI/Anthropic specs)
type JSONSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]interface{} `json:"properties,omitempty"`
    Required   []string               `json:"required,omitempty"`
    // ... additional JSON Schema fields
}
```

**Change Protocol**:
1. Model agent proposes schema format change
2. Plugin and Orchestration agents review
3. Ensure compatibility with OpenAI/Anthropic JSON Schema specs
4. Update all adapters in lockstep
5. Test with both OpenAI and Anthropic APIs

**Provider Compatibility**:
- OpenAI: Uses `function` with `parameters` (JSON Schema)
- Anthropic: Uses `tools` with `input_schema` (JSON Schema)
- Must support both formats

**Testing Requirements**:
- Schema conversion tests (plugin → OpenAI format)
- Schema conversion tests (plugin → Anthropic format)
- Round-trip tests (plugin → API → plugin)

---

### 5. Broker Interface

**File**: `internal/brokers/broker.go`
**Owner**: FileBroker Agent (interface definition)
**Consumers**: Orchestrator, plugin runtime (host functions)
**Stability**: Unstable (v0)

**Interface**:
```go
type Broker interface {
    Name() string
    Execute(ctx context.Context, brokerCtx Context, operation Operation) (*Result, error)
    Close() error
}

type Context struct {
    SessionID string
    RunID     string
    PluginID  string
    Metadata  map[string]string
}

type Operation struct {
    Action   string
    Resource string
    Input    map[string]interface{}
}

type Result struct {
    Success bool
    Output  interface{}
    Error   string
}
```

**Change Protocol**:
1. Broker agent proposes interface change
2. Orchestration and Plugin agents review
3. Coordinate change across all broker implementations
4. Update orchestrator broker invocation logic
5. Update plugin host function signatures

**Future Brokers**:
- NetworkBroker (HTTP requests)
- DatabaseBroker (SQL queries)
- CloudBroker (AWS/GCP/Azure APIs)

**Testing Requirements**:
- All broker implementations pass interface tests
- Orchestrator integration tests with brokers
- Plugin host function tests

---

## Versioning Strategy

### Semantic Versioning for Interfaces

**Format**: `vMAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes (incompatible API changes)
- **MINOR**: New features (backward compatible additions)
- **PATCH**: Bug fixes (backward compatible fixes)

**Example**:
```go
// v1.0.0
type Event struct {
    ID string
    Timestamp time.Time
}

// v1.1.0 - MINOR (added optional field)
type Event struct {
    ID string
    Timestamp time.Time
    SessionID string `json:"session_id,omitempty"` // NEW
}

// v2.0.0 - MAJOR (changed required field type)
type Event struct {
    ID int64 // BREAKING: was string
    Timestamp time.Time
    SessionID string
}
```

### Interface Versioning in Code

**Option 1: Type Aliases** (for backward compatibility)
```go
// v1
type Event struct { /* ... */ }

// v2 (keeping v1 for compatibility)
type EventV2 struct { /* new fields */ }
type Event = EventV1 // alias for backward compat
```

**Option 2: Interface Versions** (for multiple implementations)
```go
type LoggerV1 interface {
    Log(ctx context.Context, event *EventV1) error
}

type LoggerV2 interface {
    Log(ctx context.Context, event *EventV2) error
    LogBatch(ctx context.Context, events []*EventV2) error // NEW
}
```

**Option 3: Manifest Versions** (for external artifacts)
```yaml
manifest_version: "2"
```

---

## Change Request Template

Use this template when proposing an interface change:

```markdown
## Interface Change Request

**Interface**: `package.InterfaceName`
**Owner Agent**: [Agent name]
**Affected Agents**: [List of consumer agents]
**Change Type**: [BREAKING | COMPATIBLE | DEPRECATION]

### Current Interface
[Code snippet of current interface]

### Proposed Interface
[Code snippet of proposed interface]

### Rationale
[Why is this change needed?]

### Impact Analysis
[Which agents are affected? What changes are required?]

### Migration Plan
1. [Step 1]
2. [Step 2]
...

### Backward Compatibility
[How will existing code continue to work? Or migration path if breaking]

### Testing Plan
[What tests will verify the change works correctly?]

### Timeline
- Proposal: [Date]
- Review period: [Date range]
- Implementation: [Date]
- Migration complete: [Date]

### Approvals
- [ ] Owner agent
- [ ] Affected agent 1
- [ ] Affected agent 2
- [ ] Architecture review
```

---

## Deprecation Policy

When deprecating an interface:

1. **Announce**: Add deprecation notice in code and documentation
2. **Timeline**: Minimum 2 release cycles (or 6 months)
3. **Migration**: Provide migration guide and tooling if possible
4. **Warning**: Add runtime warnings when deprecated interface is used
5. **Remove**: Remove after timeline expires

**Example**:
```go
// Deprecated: Use EventV2 instead. Will be removed in v3.0.0.
type Event struct {
    // ...
}
```

---

## Interface Testing Requirements

All interface changes must include:

1. **Unit Tests**: Test the interface implementation in isolation
2. **Contract Tests**: Test that implementations satisfy the interface
3. **Integration Tests**: Test interactions between agents
4. **Backward Compatibility Tests**: Test old and new versions work together

**Example Contract Test**:
```go
func TestBrokerInterface(t *testing.T, broker brokers.Broker) {
    // Test that any broker implementation satisfies the interface
    ctx := context.Background()
    result, err := broker.Execute(ctx, brokerCtx, operation)
    require.NoError(t, err)
    require.NotNil(t, result)
}

// Run against all implementations
func TestFileBrokerInterface(t *testing.T) {
    broker := files.NewBroker(config)
    TestBrokerInterface(t, broker)
}
```

---

## Cross-Agent Communication Patterns

### Pattern 1: Direct Interface Call
```
Agent A → Interface → Agent B
```
**Example**: Orchestrator calls Model.Complete()

### Pattern 2: Event-Driven
```
Agent A → Event → Audit Logger
```
**Example**: FileBroker logs audit events (fire-and-forget)

### Pattern 3: Policy-Gated
```
Agent A → Policy Check → Agent B
```
**Example**: FileBroker checks policy before file operations

### Pattern 4: Plugin-Mediated
```
Orchestrator → Plugin Runtime → Plugin → Broker
```
**Example**: Model calls plugin tool, plugin requests file access

---

## FAQ

**Q: When do I need to coordinate an interface change?**
A: Anytime you change a type that's used by another agent (see Interface Stability Matrix above).

**Q: How do I know which agents consume my interface?**
A: Check the "Consumers" column in the Interface Stability Matrix, or search the codebase.

**Q: Can I add optional fields to a struct?**
A: Yes, if the field is tagged with `omitempty` and consumers handle missing fields gracefully.

**Q: How long does the review process take?**
A: Minimum 48 hours for minor changes, 1 week for breaking changes.

**Q: What if I need to make a breaking change urgently?**
A: Discuss with affected agents. If critical (security bug), may expedite but still require testing.

---

## Updates

This interface documentation should be updated when:
- New interfaces are added
- Interface stability changes (unstable → stable)
- New consumers are added
- Versioning strategy changes
