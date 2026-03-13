# Orchestration Agent

**Owner**: Agent 7 - Orchestration Specialist
**Independence Level**: LOW
**Status**: Active (Under Development)

## Overview

Core orchestration and component integration. Wires together model, plugins, and brokers to execute AI-driven workflows.

## Responsibilities

- Session management
- Run lifecycle
- Model-plugin-broker integration loop
- Tool execution pipeline
- Error recovery and retry
- Concurrent tool execution (planned)

## Package Structure

```
internal/core/
├── README.md
├── orchestrator.go    # Main orchestrator
└── session.go         # Session management
```

## Core Interface

```go
type Orchestrator struct {
    // internal fields
}

func (o *Orchestrator) Run(ctx, prompt) (*RunResult, error)
func (o *Orchestrator) GetSession() *Session
func (o *Orchestrator) Close() error
```

## Dependencies

**ALL other agents**:
- `internal/model` (LLM providers)
- `internal/plugins` (plugin runtime)
- `internal/brokers` (resource brokers)
- `internal/policy` (policy enforcement)
- `internal/audit` (event logging)
- `internal/config` (configuration)

## Usage

```go
orchestrator, err := core.NewOrchestrator(config)
defer orchestrator.Close()

result, err := orchestrator.Run(ctx, "Read and summarize file.txt")
```

## Testing

**Coverage Target**: 80%+
**Current**: 3 passing (needs extensive integration tests)

### Test Requirements
- Integration tests for full flow
- Error recovery tests
- Concurrent tool execution tests
- End-to-end tests

## Planned Work

- [ ] Complete model-plugin-broker integration loop
- [ ] Implement tool execution pipeline
- [ ] Add streaming output support
- [ ] Error recovery and retry logic
- [ ] Concurrent tool execution
- [ ] Performance optimization

## Coordination

**CRITICAL**: This agent depends on ALL others. Work on this AFTER other agents have stable interfaces.

Coordinate with:
- Model agent (completion API)
- Plugin agent (tool execution)
- Broker agents (resource access)
- Policy agent (enforcement)
- Audit agent (logging)

## Priority

**LOW PRIORITY**: Wait for Phase 1 and Phase 2 agents to stabilize before major work here.

## Contact

**Owner**: @orchestration-agent
