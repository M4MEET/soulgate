# Config Agent

**Owner**: Agent 3 - Config Specialist
**Independence Level**: HIGH
**Status**: Active

## Overview

Configuration management and workspace setup for SoulGate. Handles YAML parsing, validation, environment overrides, and workspace initialization.

## Responsibilities

- Configuration structure and loading
- YAML parsing and validation
- Environment variable overrides
- Workspace initialization
- Multi-workspace support (planned)

## Package Structure

```
internal/config/
├── README.md
├── config.go      # Main config structure (CRITICAL INTERFACE)
└── workspace.go   # Workspace management
```

## Core Interface

```go
type Config struct {
    Workspace WorkspaceConfig
    Model     ModelConfig
    Plugins   PluginsConfig
    Audit     AuditConfig
    Policy    PolicyConfig
}
```

**WARNING**: Config structure is used by all agents. Changes require coordination.

## Dependencies

- stdlib (os, path/filepath)
- gopkg.in/yaml.v3

## Usage

```go
// Load config from file
cfg, err := config.LoadFromFile("soulgate.yaml")

// Load with environment overrides
cfg, err := config.LoadWithEnv("soulgate.yaml")

// Initialize workspace
err := cfg.Workspace.Init()
```

## Testing

**Coverage Target**: 80%+
**Current**: Tests needed

### Test Requirements
- YAML parsing
- Validation
- Environment overrides
- Malformed config handling

## Planned Work

- [ ] Add config migration/upgrade logic
- [ ] Implement validation with JSON schema
- [ ] Add workspace templates
- [ ] Support multi-workspace setups
- [ ] Environment-specific config profiles

## Coordination

Config schema changes affect all agents. Follow Interface Change Protocol.

## Contact

**Owner**: @config-agent
