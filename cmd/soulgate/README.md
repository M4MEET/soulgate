# CLI/UX Agent

**Owner**: Agent 8 - CLI/UX Specialist
**Independence Level**: LOW
**Status**: Active

## Overview

Command-line interface and user experience. Provides CLI commands for interacting with SoulGate.

## Responsibilities

- Command implementations (init, run, audit, policy, plugin)
- Output formatting (JSON, table, pretty)
- Interactive approvals
- Progress indicators
- Error messages and help text

## Package Structure

```
cmd/soulgate/
├── README.md
├── main.go
└── cmd/
    ├── root.go
    ├── init.go
    ├── run.go
    ├── audit.go
    ├── policy.go
    └── plugin.go
```

## Commands

### `soulgate init`
Initialize a new workspace

### `soulgate run`
Execute a prompt with the orchestrator

### `soulgate audit`
Query and export audit logs

### `soulgate policy`
Manage policies (list, validate, test)

### `soulgate plugin`
Manage plugins (list, install, uninstall)

## Dependencies

- `internal/core` (orchestrator)
- `internal/config` (workspace setup)
- `internal/audit` (audit commands)
- `internal/policy` (policy commands)
- `internal/plugins` (plugin commands)
- `github.com/spf13/cobra` (CLI framework)

## Usage

```bash
# Initialize workspace
soulgate init

# Run a prompt
soulgate run "Read and summarize file.txt"

# Query audit logs
soulgate audit query --session-id abc123

# Validate policy
soulgate policy validate policy.yaml

# List plugins
soulgate plugin list
```

## Testing

**Coverage Target**: 75%+
**Current**: No tests

### Test Requirements
- Command execution tests
- Output formatting tests
- Error message tests
- Interactive feature tests

## Planned Work

- [ ] Improve run command with streaming
- [ ] Add interactive approval prompts
- [ ] Implement tab completion
- [ ] Add progress indicators
- [ ] Improve error messages and help
- [ ] Build audit query DSL

## Coordination

Uses all internal packages. Coordinate output format changes with agent owners.

## Contact

**Owner**: @cli-agent
