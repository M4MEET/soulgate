# Contributing to SoulGate

Thank you for your interest in contributing to SoulGate! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Testing Guidelines](#testing-guidelines)
- [Code Style](#code-style)
- [Submitting Changes](#submitting-changes)
- [Review Process](#review-process)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please:

- Be respectful and considerate
- Welcome newcomers and help them get started
- Focus on constructive feedback
- Respect differing viewpoints and experiences

## Getting Started

### Prerequisites

- **Go 1.22+** - [Install Go](https://golang.org/doc/install)
- **Git** - Version control
- **Make** - Build automation
- **API Keys** (for testing):
  - OpenAI API key (`OPENAI_API_KEY`)
  - Anthropic API key (`ANTHROPIC_API_KEY`)
  - Telegram bot token (`TELEGRAM_BOT_TOKEN`) - optional

### First-Time Setup

```bash
# 1. Fork the repository on GitHub

# 2. Clone your fork
git clone https://github.com/YOUR_USERNAME/soulgate.git
cd soulgate

# 3. Add upstream remote
git remote add upstream https://github.com/M4MEET/soulgate.git

# 4. Install dependencies
go mod download

# 5. Build the project
make build

# 6. Run tests
make test

# 7. Verify everything works
./bin/soulgate --version
```

## Development Setup

### Environment Variables

Create a `.env` file (not committed to Git):

```bash
# Model Provider API Keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Optional: Telegram Integration
export TELEGRAM_BOT_TOKEN="123456:ABC-DEF..."

# Optional: Development Settings
export SOULGATE_DEBUG=true
export SOULGATE_LOG_LEVEL=debug
```

Load it with:
```bash
source .env
```

### IDE Setup

**VS Code** (recommended):
```json
// .vscode/settings.json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  }
}
```

**GoLand/IntelliJ IDEA**:
- Enable Go modules support
- Configure Go fmt on save
- Install golangci-lint plugin

## Project Structure

```
soulGate/
├── cmd/soulgate/           # CLI entry point
│   ├── main.go            # Main entry
│   └── cmd/               # Command implementations
│       ├── root.go        # Root command
│       ├── gateway.go     # Gateway commands
│       ├── agent.go       # Agent commands
│       ├── observe.go     # Observer commands
│       ├── sessions.go    # Session commands
│       ├── skills.go      # Skills commands
│       └── ...
│
├── internal/              # Private application code
│   ├── protocol/          # WebSocket protocol (frames)
│   ├── gateway/           # Gateway server (routing, sessions)
│   ├── agent/             # Agent runtime (AI brain)
│   ├── observer/          # CLI observer (real-time display)
│   ├── auth/              # Authentication & pairing
│   ├── session/           # JSONL session storage
│   ├── skills/            # Markdown skills system
│   ├── connectors/        # External integrations
│   │   └── telegram/      # Telegram connector
│   ├── agents/config/     # Multi-agent configuration
│   ├── core/              # Core orchestrator
│   ├── model/             # LLM provider adapters
│   ├── policy/            # Policy engine
│   ├── audit/             # Audit logging
│   ├── brokers/           # Resource brokers
│   └── plugins/           # Plugin system
│
├── demo/                  # Demo scripts and workspace
│   ├── workspace/         # Example workspace
│   └── test_*.sh          # Test scripts
│
├── docs/                  # Documentation
├── .soulgate/             # Configuration examples
└── sessions/              # Session JSONL files (gitignored)
```

### Key Components

1. **Gateway** (`internal/gateway/`) - Central control plane that routes messages, manages sessions, and coordinates agents
2. **Protocol** (`internal/protocol/`) - WebSocket frame definitions (13 frame types)
3. **Agent Runtime** (`internal/agent/`) - Executes AI model calls and tool execution
4. **Observer** (`internal/observer/`) - Real-time CLI display with progress bars
5. **Session Storage** (`internal/session/`) - JSONL-based conversation persistence
6. **Skills System** (`internal/skills/`) - Markdown-based agent knowledge
7. **Authentication** (`internal/auth/`) - Token management and device pairing

## Development Workflow

### Branching Strategy

- `main` - Stable, production-ready code
- `develop` - Integration branch for features
- `feature/*` - New features
- `fix/*` - Bug fixes
- `docs/*` - Documentation updates

### Creating a Feature Branch

```bash
# Update your fork
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name

# Work on your feature
# ... make changes ...

# Keep your branch updated
git fetch upstream
git rebase upstream/main
```

### Making Changes

1. **Write tests first** (TDD approach recommended)
2. **Implement the feature**
3. **Run tests**: `make test`
4. **Run linter**: `make lint`
5. **Update documentation** if needed
6. **Commit with clear messages**

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

**Examples:**
```
feat(gateway): add session affinity routing

Implement sticky sessions to keep conversations with the same agent.
This improves context continuity and user experience.

Closes #123
```

```
fix(auth): prevent token validation race condition

Add mutex lock to token validation to prevent concurrent access
issues that could lead to incorrect token state.

Fixes #456
```

## Testing Guidelines

### Test Structure

- **Unit tests**: Test individual functions and methods
- **Integration tests**: Test component interactions
- **End-to-end tests**: Test complete workflows

### Running Tests

```bash
# All tests
make test

# Specific component
make test-unit           # Unit tests only
make test-security       # Security tests
make test-coverage       # With coverage report

# Individual package
go test ./internal/gateway/...
go test ./internal/auth/...

# Verbose output
go test -v ./...

# Run specific test
go test -run TestGatewayRouting ./internal/gateway/...
```

### Writing Tests

**Example unit test:**

```go
package gateway

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSessionCreation(t *testing.T) {
    gw := NewGateway(":8080")

    session := gw.CreateSession("test-id", protocol.RoleChannel)

    require.NotNil(t, session)
    assert.Equal(t, "test-id", session.ID)
    assert.Equal(t, protocol.RoleChannel, session.Role)
    assert.Equal(t, SessionStateActive, session.State)
}
```

**Example integration test:**

```go
func TestGatewayAgentRouting(t *testing.T) {
    // Setup
    gw := NewGateway(":8080")
    gw.Start()
    defer gw.Stop()

    // Connect mock agent
    agent := connectMockAgent(t, gw)

    // Send message
    msg := createTestMessage("test content")
    err := gw.RouteMessage(msg)

    // Verify
    require.NoError(t, err)
    assert.Equal(t, 1, agent.ReceivedCount())
}
```

### Test Coverage

Aim for:
- **80%+ coverage** on core components (gateway, protocol, agent, auth)
- **90%+ coverage** on security-critical code (auth, policy)
- **100% coverage** on critical security functions

Check coverage:
```bash
make test-coverage
open coverage.html
```

## Code Style

### Go Guidelines

Follow [Effective Go](https://golang.org/doc/effective_go.html) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

**Key principles:**
- Use `gofmt` for formatting
- Run `go vet` to catch common issues
- Use meaningful variable names
- Keep functions small and focused
- Document exported functions

### Example Code Style

```go
// Session represents a conversation session between a channel and agents.
// It maintains state, routing information, and statistics.
type Session struct {
    ID                   string
    Role                 protocol.ClientRole
    State                SessionState
    AssignedAgentID      string
    MessageCount         int
    CreatedAt            time.Time
    LastActivityAt       time.Time
    mu                   sync.RWMutex
}

// AssignAgent assigns an agent to this session.
// It updates the agent history and records the assignment timestamp.
func (s *Session) AssignAgent(agentID string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.AssignedAgentID = agentID
    s.AgentHistory = append(s.AgentHistory, agentID)
    s.LastActivityAt = time.Now()
}
```

### Linting

```bash
# Run linter
make lint

# Fix auto-fixable issues
gofmt -w .
go mod tidy

# Install golangci-lint (if needed)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Documentation

**Godoc comments:**
```go
// Package gateway implements the central control plane for SoulGate.
// It routes WebSocket frames between clients (agents, channels, observers)
// and maintains session state.
package gateway

// Gateway is the central WebSocket server that routes frames between clients.
// It manages connections, sessions, and implements intelligent routing strategies.
type Gateway struct {
    // ...
}
```

## Submitting Changes

### Pre-Submission Checklist

- [ ] Tests pass: `make test`
- [ ] Linter passes: `make lint`
- [ ] Code is formatted: `gofmt -w .`
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] Branch is up to date with `main`

### Creating a Pull Request

```bash
# Push your branch
git push origin feature/your-feature-name

# Create PR on GitHub
# - Use a clear title
# - Reference related issues
# - Describe what changed and why
# - Add screenshots/examples if relevant
```

### PR Template

```markdown
## Description
Brief description of changes

## Motivation
Why is this change needed?

## Changes
- Change 1
- Change 2

## Testing
How was this tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Lint passes
- [ ] No breaking changes (or documented)

Closes #123
```

## Review Process

### What Reviewers Look For

1. **Correctness** - Does it work as intended?
2. **Tests** - Are there adequate tests?
3. **Security** - Any security implications?
4. **Performance** - Any performance concerns?
5. **Style** - Does it follow project conventions?
6. **Documentation** - Is it well documented?

### Responding to Feedback

- Be open to feedback
- Ask questions if unclear
- Update PR based on feedback
- Mark conversations as resolved
- Re-request review when ready

### Merging

- Requires approval from maintainer
- All CI checks must pass
- Branch must be up to date with main
- Squash merge preferred for clean history

## Areas for Contribution

### Good First Issues

Look for issues labeled `good-first-issue`:
- Documentation improvements
- Test coverage improvements
- Small bug fixes
- CLI usability enhancements

### High Priority Areas

1. **Authentication System** - Enhance token management, add OAuth support
2. **Skills System** - Create more example skills, improve loader
3. **Connectors** - Add Discord, Slack, or other platform connectors
4. **Observer UX** - Improve real-time display, add filtering
5. **Documentation** - Tutorials, guides, API docs

### New Features

If proposing a new feature:
1. Open an issue first to discuss
2. Get feedback from maintainers
3. Agree on approach before coding
4. Follow the development workflow

## Getting Help

- **Documentation**: Check `docs/` and `*.md` files
- **Issues**: Search existing issues on GitHub
- **Discussions**: Use GitHub Discussions for questions
- **Code**: Read existing code for examples

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see LICENSE file).

## Recognition

Contributors will be acknowledged in:
- CONTRIBUTORS.md file
- Release notes
- Project documentation

---

Thank you for contributing to SoulGate! 🚀
