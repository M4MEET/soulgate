# Audit Agent

**Owner**: Agent 1 - Audit Specialist
**Independence Level**: HIGH
**Status**: Active

## Overview

The Audit Agent is responsible for audit logging and event tracking throughout SoulGate. It provides a centralized system for logging all significant events (policy decisions, broker operations, plugin executions, model calls) to a SQLite database.

## Responsibilities

- Audit event schema definition
- Event logging to SQLite
- Event querying and filtering
- Export formats (JSON, webhooks - planned)
- Event streaming (planned)

## Package Structure

```
internal/audit/
├── README.md          # This file
├── event.go           # Event types and schema (CRITICAL INTERFACE)
├── logger.go          # Logger interface and implementations
├── sqlite.go          # SQLite audit logger implementation
├── event_test.go      # Event tests
├── logger_test.go     # Logger tests
└── sqlite_test.go     # SQLite tests
```

## Core Interfaces

### Event Schema (CRITICAL)

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
```

**WARNING**: This schema is used by ALL agents. Any changes require coordination (see docs/COORDINATION.md).

### Logger Interface

```go
type Logger interface {
    Log(ctx context.Context, event *Event) error
    Query(ctx context.Context, filter QueryFilter) ([]*Event, error)
    Close() error
}
```

## Dependencies

- **stdlib only** (database/sql, encoding/json, time)
- **modernc.org/sqlite** for SQLite implementation

No internal dependencies on other agents.

## Usage

### Creating a Logger

```go
import "github.com/yourusername/soulgate/internal/audit"

// Create SQLite logger
logger, err := audit.NewSQLiteLogger(config)
if err != nil {
    return err
}
defer logger.Close()
```

### Logging Events

```go
event := &audit.Event{
    ID:        "evt-123",
    Timestamp: time.Now(),
    SessionID: "session-abc",
    RunID:     "run-xyz",
    EventType: audit.EventTypeBroker,
    Component: "files",
    Action:    "read_file",
    Resource:  "/path/to/file.txt",
    Status:    audit.EventStatusSuccess,
}

err := logger.Log(ctx, event)
```

### Querying Events

```go
filter := audit.QueryFilter{
    SessionID:  "session-abc",
    EventType:  audit.EventTypeBroker,
    StartTime:  time.Now().Add(-1 * time.Hour),
    EndTime:    time.Now(),
}

events, err := logger.Query(ctx, filter)
```

## Testing

**Coverage Target**: 85%+

### Running Tests

```bash
# Unit tests
go test -v ./internal/audit/...

# With coverage
go test -v -coverprofile=coverage.txt ./internal/audit/...

# View coverage
go tool cover -html=coverage.txt
```

### Test Files

- `event_test.go` - Event serialization, validation
- `logger_test.go` - Logger interface tests
- `sqlite_test.go` - SQLite implementation tests

### Current Status

- **Tests**: 4 passing
- **Coverage**: Good

## Performance Targets

| Operation | Target | Current |
|-----------|--------|---------|
| Log event | < 1ms | ✓ |
| Query 1000 events | < 10ms | ✓ |
| Query 100k events | < 100ms | TBD |

## Planned Work

### Phase 1 (Current)
- [x] Basic event schema
- [x] SQLite logger implementation
- [x] Query filtering

### Phase 2 (Next)
- [ ] Optimize SQLite queries for large audit logs
- [ ] Add event streaming/export capabilities
- [ ] Implement webhook audit sink
- [ ] Add event filtering and aggregation
- [ ] Performance tuning for high-volume logging

### Phase 3 (Future)
- [ ] Event aggregation and analytics
- [ ] Real-time event streaming
- [ ] Distributed audit log support

## Coordination Points

### Event Schema Changes

**CRITICAL**: The Event schema is used by all agents. Any changes must follow the Interface Change Protocol (see docs/COORDINATION.md).

**Process**:
1. Propose change in GitHub issue
2. Notify all agents (tag @all-agents)
3. Wait for approval (minimum 1 week for breaking changes)
4. Implement with backward compatibility
5. Migrate all agents
6. Remove old schema after 2 release cycles

### Consumers

The following components log audit events:
- Orchestrator (session/run events)
- FileBroker (file operation events)
- Policy Engine (policy decision events)
- Plugin Runtime (plugin execution events)
- Model Adapters (model call events)

Any schema changes affect all of these.

## Security Considerations

### Event Integrity

- Events are append-only (no updates or deletes)
- Event IDs are unique (UUID v4)
- Timestamps are immutable

### Sensitive Data

- Avoid logging sensitive data in event messages
- Use metadata for structured data
- Consider PII in resource paths

### SQLite Security

- Database file permissions: 0600 (owner read/write only)
- No external network access
- Local file access only

## API Stability

| Interface | Stability | Version |
|-----------|-----------|---------|
| `Event` | Stable | v1 |
| `Logger` | Stable | v1 |
| `QueryFilter` | Stable | v1 |

## Contributing

See docs/AGENTS.md for agent responsibilities and coordination protocols.

### Before Making Changes

1. Check if your change affects the Event schema
2. If yes, follow the Interface Change Protocol
3. If no, make changes and ensure tests pass
4. Update this README if adding new features

### Code Review

- Audit agent owner reviews all changes
- If Event schema changes, all agents must review
- Security-critical changes require security review

## Contact

**Owner**: @audit-agent (see CODEOWNERS)
**Coordination**: See docs/COORDINATION.md
