# Audit System

## Overview

The audit system provides centralized event logging for SoulGate. All significant events (policy decisions, broker operations, plugin executions, model calls) are recorded to an append-only JSONL (JSON Lines) file.

## Package Structure

```
internal/audit/
├── README.md          # This file
├── event.go           # Event types and schema
├── logger.go          # Logger interface
├── jsonl.go           # JSONL audit logger implementation
├── event_test.go      # Event tests
└── jsonl_test.go      # JSONL logger tests
```

## Logger Interface

```go
type Logger interface {
    Log(ctx context.Context, event *Event) error
    Query(ctx context.Context, filter QueryFilter) ([]*Event, error)
    Close() error
}
```

## Usage

### Creating a Logger

```go
import "github.com/M4MEET/soulgate/internal/audit"

logger, err := audit.NewJSONLLogger(".soulgate/audit.jsonl")
if err != nil {
    return err
}
defer logger.Close()
```

### Logging Events

```go
event := audit.NewEvent(audit.EventFileRead, audit.CategoryBroker).
    WithSessionID("session-abc").
    WithRunID("run-xyz").
    WithResource("/path/to/file.txt").
    WithStatus(audit.StatusSuccess)

err := logger.Log(ctx, event)
```

### Querying Events

```go
filter := audit.QueryFilter{
    SessionID: "session-abc",
    Type:      audit.EventFileRead,
    Limit:     50,
}

events, err := logger.Query(ctx, filter)
```

## Testing

```bash
go test -v ./internal/audit/...
```

## Security

- Audit file permissions: 0600 (owner read/write only)
- Events are append-only (no updates or deletes)
- Avoid logging sensitive data in event messages
