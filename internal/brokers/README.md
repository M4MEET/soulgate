# FileBroker Agent

**Owner**: Agent 4 - Security/FileBroker Specialist
**Independence Level**: MEDIUM
**Status**: Active

## Overview

File system access with security enforcement. Handles path validation, traversal prevention, policy enforcement, and audit logging for all file operations.

## Responsibilities

- File operations (read, list, stat, write)
- Path validation and traversal prevention
- Policy enforcement integration
- Audit logging
- Virtual filesystem mapping (planned)

## Package Structure

```
internal/brokers/
├── README.md
├── broker.go              # Broker interface (CRITICAL INTERFACE)
├── files/
│   ├── broker.go          # FileBroker implementation
│   └── permissions.go     # Path validation (SECURITY-CRITICAL)
```

## Core Interfaces

```go
type Broker interface {
    Name() string
    Execute(ctx, brokerCtx, operation) (*Result, error)
    Close() error
}
```

**WARNING**: Broker interface is used by orchestrator and plugins.

## Dependencies

- `internal/policy` (policy enforcement)
- `internal/audit` (event logging)

## Security Requirements

**CRITICAL**: All changes must maintain:
- Path traversal prevention
- Policy enforcement on ALL operations
- Audit logging on ALL operations
- Symlink handling security

## Usage

```go
broker := files.NewBroker(config, policyEngine, auditLogger)

result, err := broker.Execute(ctx, brokerCtx, operation)
```

## Testing

**Coverage Target**: 90%+ (security-critical)
**Current**: 7 passing

### Security Test Matrix

| Path | Expected |
|------|----------|
| `../etc/passwd` | Error: path traversal |
| `../../etc/passwd` | Error: path traversal |
| `/etc/passwd` | Error: outside boundary |
| `file.txt` | Success |

### Test Requirements
- Path traversal tests (CRITICAL)
- Permission tests
- Policy integration tests
- Audit integration tests

## Planned Work

- [ ] Implement write operations with approval
- [ ] Add diff generation for modifications
- [ ] Virtual filesystem mapping
- [ ] File watching capabilities
- [ ] Performance optimization (caching)

## Coordination

- PolicyRequest changes → coordinate with Policy agent
- Event schema changes → coordinate with Audit agent
- Broker interface changes → coordinate with Orchestration and Plugin agents

## Security Review

ALL changes require security review. See docs/COORDINATION.md Protocol 4.

## Contact

**Owner**: @broker-agent
**Security**: @security-reviewer
