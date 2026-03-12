# Multi-Agent Development Quickstart

This guide helps you get started with SoulGate's multi-agent development workflow.

## What is Multi-Agent Development?

SoulGate is organized into 8 specialized agents, each responsible for a specific component:

1. **Audit Agent** - Event logging
2. **Policy Agent** - Security policies
3. **Config Agent** - Configuration
4. **FileBroker Agent** - File operations
5. **Plugin Agent** - Plugin system
6. **Model Agent** - LLM integration
7. **Orchestration Agent** - Component wiring
8. **CLI Agent** - User interface

Each agent can be developed independently in parallel, with clear boundaries and coordination protocols.

## Getting Started

### 1. Understanding Your Role

If you're working on a specific agent, read:
1. `docs/AGENTS.md` - Find your agent's responsibilities
2. `internal/<your-agent>/README.md` - Detailed agent documentation
3. `docs/INTERFACES.md` - Interfaces you depend on or own
4. `docs/COORDINATION.md` - How to coordinate changes

### 2. Setup Your Environment

```bash
# Clone repository
git clone https://github.com/yourusername/soulgate.git
cd soulgate

# Install dependencies
go mod download

# Run existing tests
make test

# Run your agent's tests
make test-<agent>  # e.g., make test-audit
```

### 3. Development Workflow

#### Working on Your Agent

```bash
# 1. Create a feature branch
git checkout -b agent-audit/optimize-queries

# 2. Make changes in your agent's scope
vim internal/audit/sqlite.go

# 3. Run your agent's tests
make test-audit

# 4. Check coverage
make test-coverage

# 5. Commit changes
git add .
git commit -m "Optimize SQLite queries for large datasets"

# 6. Push and create PR
git push origin agent-audit/optimize-queries
```

#### Testing Your Changes

```bash
# Run your agent's tests only
make test-audit          # Fast (< 30s)

# Run all tests
make test                # Slower (includes integration)

# Run security tests (if applicable)
make test-security

# Run benchmarks
make bench-audit
```

### 4. Coordination Protocol

#### When You DON'T Need Coordination

You can work independently if:
- Changes are within your agent's scope
- No interface changes
- No breaking changes

Just make changes, test, and create a PR.

#### When You NEED Coordination

Coordinate when:
- Changing a shared interface (see `docs/INTERFACES.md`)
- Making a breaking change
- Affecting other agents

**Process**:
1. Create GitHub issue with "interface-change" label
2. Tag affected agents
3. Wait for approval (48h for compatible, 1 week for breaking)
4. Implement with backward compatibility
5. Migrate all agents
6. Remove old interface

See `docs/COORDINATION.md` for detailed protocols.

## Common Tasks

### Adding a New Feature

```bash
# 1. Check if feature requires coordination
# - Does it change a shared interface? → Coordinate
# - Does it affect other agents? → Coordinate
# - Is it within your scope? → Work independently

# 2. Write tests first (TDD)
vim internal/audit/sqlite_test.go

# 3. Implement feature
vim internal/audit/sqlite.go

# 4. Run tests
make test-audit

# 5. Update documentation
vim internal/audit/README.md

# 6. Create PR
```

### Fixing a Bug

```bash
# 1. Write a test that reproduces the bug
vim internal/policy/engine_test.go

# 2. Fix the bug
vim internal/policy/engine.go

# 3. Verify test passes
make test-policy

# 4. Check for regressions
make test

# 5. Create PR with bug report
```

### Changing an Interface

```bash
# 1. Create interface change proposal
# File GitHub issue using template in docs/COORDINATION.md

# 2. Wait for approval from affected agents

# 3. Implement with backward compatibility
vim internal/audit/event.go

# 4. Update all consumers (or coordinate with them)

# 5. Run integration tests
make test-integration

# 6. Create PR with migration guide
```

## Useful Commands

### Testing
```bash
make test              # Run all tests
make test-<agent>      # Run agent-specific tests
make test-coverage     # Generate coverage report
make test-security     # Run security tests
make agent-status      # Show test status for all agents
```

### Building
```bash
make build             # Build binary
make build-release     # Build optimized binary
make install           # Install to $GOPATH/bin
```

### Development
```bash
make dev               # Build and run
make lint              # Run linters
make check             # Run all checks (lint + test + security)
make clean             # Remove build artifacts
```

### Help
```bash
make help              # Show all available targets
```

## Agent-Specific Guides

### Audit Agent (Agent 1)
**Scope**: `internal/audit/`
**Focus**: Event logging, SQLite optimization
**Tests**: `make test-audit`

### Policy Agent (Agent 2)
**Scope**: `internal/policy/`
**Focus**: Pattern matching, security rules
**Tests**: `make test-policy`
**Security**: All changes require security review

### Config Agent (Agent 3)
**Scope**: `internal/config/`
**Focus**: Config management, workspace setup
**Tests**: `make test-config`

### FileBroker Agent (Agent 4)
**Scope**: `internal/brokers/`
**Focus**: File operations, security enforcement
**Tests**: `make test-broker`
**Security**: All changes require security review

### Plugin Agent (Agent 5)
**Scope**: `internal/plugins/`
**Focus**: WASM runtime, plugin SDK
**Tests**: `make test-plugin`
**Security**: All changes require security review

### Model Agent (Agent 6)
**Scope**: `internal/model/`
**Focus**: LLM adapters, tool schemas
**Tests**: `make test-model`

### Orchestration Agent (Agent 7)
**Scope**: `internal/core/`
**Focus**: Component integration
**Tests**: `make test-core`
**Note**: Wait for other agents to stabilize first

### CLI Agent (Agent 8)
**Scope**: `cmd/soulgate/`
**Focus**: User interface, commands
**Tests**: `make test-cli`

## Phase-Based Development

### Phase 1 (Weeks 1-2): Independent Agents
**Active Agents**: Audit, Policy, Config, Model
**Goal**: Stabilize low-coupling components
**Coordination**: Weekly sync (async)

### Phase 2 (Weeks 3-4): Brokers & Plugins
**Active Agents**: FileBroker, Plugin
**Goal**: Build resource access layer
**Coordination**: Daily standups

### Phase 3 (Weeks 5-7): Integration
**Active Agents**: Orchestration, CLI
**Goal**: Wire everything together
**Coordination**: Continuous integration

See `docs/ROADMAP.md` for detailed timeline.

## FAQ

**Q: Which agent should I work on?**
A: Check `docs/ROADMAP.md` for current phase and available agents.

**Q: Can I work on multiple agents?**
A: Yes, but focus on one at a time. Coordinate with other team members.

**Q: How do I know if I need to coordinate?**
A: Check if your change affects a file listed in `docs/INTERFACES.md` under "Critical Files".

**Q: What if my tests are failing?**
A: Don't merge failing tests. Fix them or coordinate with affected agents if the failure is due to interface changes.

**Q: How long does coordination take?**
A: 48 hours for compatible changes, 1 week for breaking changes.

**Q: Can I skip security review?**
A: No, security-critical agents (Policy, FileBroker, Plugin) require security review for all changes.

## Resources

- [Agent Registry](AGENTS.md) - Agent responsibilities
- [Interface Contracts](INTERFACES.md) - Shared interfaces
- [Coordination Protocols](COORDINATION.md) - Coordination process
- [Testing Strategy](TESTING.md) - Testing requirements
- [Roadmap](ROADMAP.md) - Implementation timeline
- [CODEOWNERS](../CODEOWNERS) - Code ownership

## Getting Help

- **Agent-specific questions**: Ping agent owner in PR
- **Interface changes**: Create GitHub issue
- **Security concerns**: Tag @security-reviewer
- **Coordination issues**: Post in coordination channel

## Next Steps

1. Read your agent's README: `internal/<agent>/README.md`
2. Check current phase in `docs/ROADMAP.md`
3. Look at open issues for your agent
4. Start working on assigned tasks
5. Run tests frequently: `make test-<agent>`
6. Coordinate when needed

Happy coding! 🚀
