# SoulGate CLI Usage Guide

## Table of Contents

1. [Installation](#installation)
2. [Quick Start](#quick-start)
3. [Interactive Setup](#interactive-setup)
4. [Commands Reference](#commands-reference)
5. [Configuration](#configuration)
6. [Agent Management](#agent-management)
7. [Production Deployment](#production-deployment)
8. [Examples](#examples)

---

## Installation

### Option 1: Build from Source

```bash
# Clone the repository
git clone https://github.com/M4MEET/soulgate.git
cd soulgate

# Build the CLI
make build-cli

# Optionally install to system
make install-cli
```

### Option 2: Download Pre-built Binary

```bash
# Linux (amd64)
curl -L https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-linux-amd64.tar.gz | tar xz

# macOS (Apple Silicon)
curl -L https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-darwin-arm64.tar.gz | tar xz

# macOS (Intel)
curl -L https://github.com/M4MEET/soulgate/releases/download/v0.1.0/soulgate-darwin-amd64.tar.gz | tar xz

# Make executable
chmod +x soulgate

# Move to PATH
sudo mv soulgate /usr/local/bin/
```

### Verify Installation

```bash
soulgate --version
# Output: soulgate version 0.1.0 (commit: abc1234, built: 2026-02-14)
```

---

## Quick Start

### 1. Build the CLI (if not installed)

```bash
make quickstart
```

This will build the CLI and show you next steps.

### 2. Initialize Your Workspace

**Quick initialization** (uses defaults):

```bash
./bin/soulgate init
```

**Interactive setup** (recommended for first-time users):

```bash
./bin/soulgate setup
```

### 3. Check Status

```bash
./bin/soulgate status
```

### 4. Run Your First Prompt

```bash
./bin/soulgate run "List all Go files in this directory"
```

---

## Interactive Setup

The `setup` command provides a guided wizard for configuring SoulGate.

```bash
./bin/soulgate setup
```

### Setup Wizard Steps

#### 1. **Workspace Configuration**
   - Choose workspace path (default: current directory)
   - Check if already initialized

#### 2. **Model Provider Configuration**
   - Choose provider: OpenAI, Anthropic, Ollama, or skip
   - Enter API key (or use environment variable)
   - Select model name

   **Providers:**
   - **OpenAI**: GPT-4, GPT-3.5-turbo
   - **Anthropic**: Claude 3 Opus, Sonnet, Haiku
   - **Ollama**: Local models (Llama2, Mistral, etc.)

#### 3. **Security Policy Configuration**
   - Choose security mode:
     - **Strict**: Only workspace files, no network, no execution
     - **Moderate**: Workspace + project files, limited network
     - **Permissive**: Home directory access, network enabled
     - **Custom**: Configure everything manually

#### 4. **Consolidated Agents Configuration**
   - **Test & Quality Agent**: Test generation, execution, CI, security
   - **Docs & API Agent**: Documentation and API spec generation
   - **Project Management Agent**: Task assignment and sprint planning

   Configure:
   - Test coverage targets
   - Task assignment strategies
   - Documentation coverage

#### 5. **Audit & Notifications**
   - Enable audit logging
   - Configure notification channels (console, Slack, email, webhook)

#### 6. **Review & Apply**
   - Review all settings
   - Apply configuration
   - Initialize workspace

---

## Commands Reference

### Core Commands

#### `soulgate setup`
Interactive setup wizard for first-time configuration.

```bash
soulgate setup
```

#### `soulgate init [path]`
Quick initialization with default settings.

```bash
soulgate init                    # Initialize current directory
soulgate init /path/to/project   # Initialize specific directory
```

#### `soulgate status`
Show workspace status and configuration.

```bash
soulgate status
```

**Output includes:**
- Workspace path and status
- Model provider configuration
- Security policy summary
- Consolidated agents status
- Audit logging status
- Installed plugins

#### `soulgate run "<prompt>"`
Run a prompt with SoulGate.

```bash
soulgate run "What files are in this directory?"
soulgate run "Create a README.md file"
soulgate run "Run tests and show coverage"
```

**Options:**
- `--model <name>`: Override model
- `--provider <provider>`: Override provider
- `--max-tokens <n>`: Set max tokens
- `--stream`: Enable streaming output

### Agent Management

#### `soulgate agents list`
List all consolidated agents and their status.

```bash
soulgate agents list
```

**Output:**
```
📦 Test & Quality Agent
   Status:       ✅ Enabled
   Description:  Test generation, execution, CI, and security scanning
   Consolidates: Test Agent (14), Test Runner (23), CI Agent (29), Security Fix (12)

📦 Docs & API Agent
   Status:       ✅ Enabled
   Description:  Documentation generation and API specification management
   Consolidates: Docs Agent (13), API Agent (15)

...
```

#### `soulgate agents enable <agent>`
Enable a specific agent.

```bash
soulgate agents enable test_quality
soulgate agents enable docs_api
soulgate agents enable project_mgmt
```

#### `soulgate agents disable <agent>`
Disable a specific agent.

```bash
soulgate agents disable test_quality
```

#### `soulgate agents info <agent>`
Show detailed information about an agent.

```bash
soulgate agents info test_quality
```

**Output:**
```
╔═══════════════════════════════════════════════════════╗
║ Agent: test_quality                                   ║
╚═══════════════════════════════════════════════════════╝

Configuration:
  enabled                  : true
  mode                     :
  coverage_target          : 85
  security_scan            : true
  parallel                 : true
  max_concurrency          : 4
  timeout                  : 10m0s
```

### Audit Commands

#### `soulgate audit query`
Query audit logs.

```bash
soulgate audit query                          # Show recent events
soulgate audit query --type file.read         # Filter by event type
soulgate audit query --since 1h               # Events from last hour
soulgate audit query --plugin plugin-id       # Events from specific plugin
```

#### `soulgate audit export`
Export audit logs.

```bash
soulgate audit export --format json > audit.json
soulgate audit export --format csv > audit.csv
soulgate audit export --since 24h > today.json
```

### Policy Commands

#### `soulgate policy check <path>`
Check if a path is allowed by policy.

```bash
soulgate policy check ./myfile.txt
soulgate policy check /etc/passwd
```

**Output:**
```
Policy Check: ./myfile.txt
Action:       file.read
Decision:     ✅ ALLOW
Rule:         workspace-read (priority: 100)
```

#### `soulgate policy list`
List all policy rules.

```bash
soulgate policy list
```

#### `soulgate policy validate`
Validate policy configuration.

```bash
soulgate policy validate
```

### Plugin Commands

#### `soulgate plugin list`
List installed plugins.

```bash
soulgate plugin list
```

#### `soulgate plugin install <path>`
Install a plugin.

```bash
soulgate plugin install ./my-plugin.wasm
soulgate plugin install https://example.com/plugin.wasm
```

#### `soulgate plugin uninstall <plugin-id>`
Uninstall a plugin.

```bash
soulgate plugin uninstall my-plugin
```

### Version and Help

#### `soulgate --version`
Show version information.

```bash
soulgate --version
```

#### `soulgate --help`
Show help for all commands.

```bash
soulgate --help
soulgate agents --help
soulgate audit --help
```

---

## Configuration

SoulGate stores configuration in `.soulgate/` directory:

```
.soulgate/
├── config.yml          # Main configuration
├── policy.yml          # Security policies
├── agents.yaml         # Consolidated agents config
└── audit.db            # Audit log database
```

### Configuration Files

#### `.soulgate/config.yml`

```yaml
workspace:
  root: /path/to/project
  config_dir: .soulgate

model:
  provider: openai
  name: gpt-4
  base_url: ""
  api_key_env: OPENAI_API_KEY

policy:
  default_action: deny
  policy_file: .soulgate/policy.yml

audit:
  enabled: true
  database: .soulgate/audit.db
  export_webhooks: []

plugins:
  dir: plugins
  enabled: true
```

#### `.soulgate/agents.yaml`

```yaml
test_quality:
  enabled: true
  mode: ""  # auto-detect
  coverage_target: 85
  security_scan: true
  parallel: true
  max_concurrency: 4
  timeout: 10m

docs_api:
  enabled: true
  auto_generate_docs: true
  api_spec_format: "openapi3"
  docs_coverage_target: 80

project_mgmt:
  enabled: true
  auto_assign: true
  assignment_strategy: "skill_based"
  sprint_duration: 336h  # 14 days
  max_tasks_per_dev: 5

notification:
  enabled: true
  enabled_channels:
    - console
  min_level: "info"
```

### Environment Variables

```bash
# Model API Keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Configuration Override
export SOULGATE_CONFIG_DIR="/custom/path/.soulgate"
export SOULGATE_LOG_LEVEL="debug"
```

---

## Agent Management

### Test & Quality Agent

**Modes:**
- `generate`: Generate tests for untested code
- `execute`: Run tests locally
- `ci`: Full CI pipeline
- `security`: Security vulnerability scanning
- `coverage`: Coverage analysis

**Configuration:**

```yaml
test_quality:
  mode: "ci"
  coverage_target: 90
  security_scan: true
  parallel: true
  max_concurrency: 8
  timeout: 30m
  test_patterns:
    - "./..."
    - "./integration/..."
```

### Docs & API Agent

**Operations:**
- `generate-docs`: Generate API documentation
- `generate-api-spec`: Create OpenAPI/Swagger spec
- `update-changelog`: Update CHANGELOG.md from commits
- `check-docs-coverage`: Check documentation coverage
- `generate-examples`: Generate code examples
- `validate-api-spec`: Validate API specification

**Configuration:**

```yaml
docs_api:
  auto_generate_docs: true
  api_spec_format: "openapi3"  # or "swagger2"
  changelog_auto: true
  docs_coverage_target: 85
  examples_format: "markdown"  # or "godoc"
```

### Project Management Agent

**Features:**
- `assign`: Assign tasks to developers
- `track`: Track sprint progress
- `plan`: Sprint planning
- `report`: Generate reports
- `balance`: Balance workload

**Configuration:**

```yaml
project_mgmt:
  auto_assign: true
  assignment_strategy: "skill_based"  # or "round_robin", "workload_balanced"
  auto_update_status: true
  sprint_duration: 336h  # 14 days
  max_tasks_per_dev: 5
  skill_map:
    alice:
      - "Go"
      - "Security"
    bob:
      - "Go"
      - "Plugins"
```

---

## Production Deployment

### 1. Build Production Binary

```bash
make build-cli
```

This creates an optimized binary with:
- Version information embedded
- Build time and git commit
- Stripped debug symbols

### 2. Build for Multiple Platforms

```bash
make build-all
```

Creates binaries for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### 3. Create Distribution Packages

```bash
make dist
```

Creates compressed packages in `dist/packages/`:
- `.tar.gz` for Linux/macOS
- `.zip` for Windows

### 4. System Installation

```bash
# Install to /usr/local/bin
make install-cli

# Verify
which soulgate
soulgate --version
```

### 5. Systemd Service (Linux)

Create `/etc/systemd/system/soulgate.service`:

```ini
[Unit]
Description=SoulGate Agent Gateway
After=network.target

[Service]
Type=simple
User=soulgate
WorkingDirectory=/opt/soulgate
ExecStart=/usr/local/bin/soulgate run --daemon
Environment="OPENAI_API_KEY=sk-..."
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable soulgate
sudo systemctl start soulgate
sudo systemctl status soulgate
```

### 6. Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN make build-cli

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bin/soulgate .
ENTRYPOINT ["./soulgate"]
```

Build and run:

```bash
docker build -t soulgate:latest .
docker run -it --rm \
  -v $(pwd):/workspace \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  soulgate:latest status
```

---

## Examples

### Example 1: Complete Setup Flow

```bash
# 1. Build the CLI
make build-cli

# 2. Run interactive setup
./bin/soulgate setup

# Choose:
# - Provider: OpenAI
# - Model: gpt-4
# - Security: Moderate
# - Agents: Enable all
# - Audit: Enable
# - Notifications: Console

# 3. Set API key
export OPENAI_API_KEY="sk-..."

# 4. Check status
./bin/soulgate status

# 5. Run first prompt
./bin/soulgate run "List all files in this directory"
```

### Example 2: Enable Test Agent for CI

```bash
# Enable test agent
./bin/soulgate agents enable test_quality

# Configure for CI mode
# Edit .soulgate/agents.yaml:
# test_quality:
#   mode: "ci"
#   coverage_target: 90
#   parallel: true

# Run tests
./bin/soulgate run "Run all tests with coverage"
```

### Example 3: Documentation Generation

```bash
# Enable docs agent
./bin/soulgate agents enable docs_api

# Generate documentation
./bin/soulgate run "Generate API documentation for all packages"

# Generate OpenAPI spec
./bin/soulgate run "Generate OpenAPI 3.0 specification"

# Update changelog
./bin/soulgate run "Update CHANGELOG.md from git commits since last release"
```

### Example 4: Project Management

```bash
# Enable PM agent
./bin/soulgate agents enable project_mgmt

# Configure skill map
# Edit .soulgate/agents.yaml with team skills

# Assign tasks
./bin/soulgate run "Assign open tasks based on developer skills"

# Track sprint
./bin/soulgate run "Show current sprint progress"

# Generate report
./bin/soulgate run "Generate weekly progress report"
```

### Example 5: Security Scanning

```bash
# Enable test agent with security mode
./bin/soulgate agents enable test_quality

# Run security scan
./bin/soulgate run "Scan codebase for security vulnerabilities"

# Check specific file
./bin/soulgate policy check ./internal/brokers/files/broker.go

# Review audit logs
./bin/soulgate audit query --type policy.deny --since 24h
```

---

## Troubleshooting

### CLI Not Found

```bash
# Check if binary exists
ls -la ./bin/soulgate

# Rebuild
make clean build-cli

# Or install to system
make install-cli
```

### Permission Denied

```bash
# Make binary executable
chmod +x ./bin/soulgate

# Or for system installation
sudo chmod +x /usr/local/bin/soulgate
```

### API Key Not Set

```bash
# Check environment variable
echo $OPENAI_API_KEY

# Set it
export OPENAI_API_KEY="sk-..."

# Or add to ~/.bashrc or ~/.zshrc
echo 'export OPENAI_API_KEY="sk-..."' >> ~/.bashrc
source ~/.bashrc
```

### Workspace Not Initialized

```bash
# Check status
./bin/soulgate status

# If not initialized, run setup
./bin/soulgate setup
```

### Agent Not Enabled

```bash
# List agents
./bin/soulgate agents list

# Enable specific agent
./bin/soulgate agents enable test_quality
```

---

## Next Steps

1. **Explore Commands**: Run `soulgate --help` to see all available commands
2. **Configure Agents**: Customize `.soulgate/agents.yaml` for your needs
3. **Review Policies**: Check `.soulgate/policy.yml` and adjust security rules
4. **Install Plugins**: Add custom functionality via WASM plugins
5. **Monitor Audit Logs**: Use `soulgate audit query` to track activity
6. **Production Deploy**: Follow production deployment guide above

---

## Support

- **Documentation**: See `docs/` directory
- **Examples**: Check `examples/` directory
- **Issues**: https://github.com/M4MEET/soulgate/issues
- **Discussions**: https://github.com/M4MEET/soulgate/discussions

---

**Version**: 0.1.0
**Last Updated**: 2026-02-14
