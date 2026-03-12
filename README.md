# SoulGate 🚀

**Your powerful AI assistant in the terminal - like OpenClaw, but with security and privacy.**

SoulGate gives you a single, capable AI agent that can read, write, and manage your files through natural conversation. Everything is controlled by security policies and fully audited.

```bash
soulgate chat
>>> read the README and create a summary.txt
>>> list all Go files in this project
>>> add a comment to main.go explaining what it does
```

## Features

- 🤖 **Single Powerful Agent** - One AI that can do everything you need
- 📁 **File Operations** - Read, write, and list files through conversation
- 🔒 **Security Controls** - Policy engine controls what the agent can access
- 📊 **Full Audit Logs** - Every operation is logged to SQLite
- 🌐 **Multiple Providers** - OpenAI, Anthropic, or Gemini
- 🎨 **OpenClaw-Style UX** - Natural, interactive terminal experience

## Status

✅ **v0.1 - Ready to Use**

- ✅ Interactive chat with conversation memory
- ✅ File operations (read, write, list)
- ✅ Security policy engine
- ✅ Complete audit logging
- ✅ OpenAI & Anthropic support
- ✅ Global configuration (~/.soulgate/)

## Getting Started

**First-time users**: SoulGate features a beautiful **auto-triggering onboarding wizard** 🎯

```bash
# Build SoulGate
make build

# Initialize workspace
cd your-project
./bin/soulgate init

# Start interactive TUI (onboarding auto-triggers!)
./bin/soulgate tui
```

The onboarding wizard will guide you through:
- 🤖 **Model Selection** - Choose GPT-4o, Claude, Llama, or others
- 🔑 **API Key Setup** - Configure your credentials securely
- ✅ **Connection Test** - Verify everything works
- 🔌 **Integrations** - Optional: Slack, GitHub, Notion, etc.
- 📚 **Quick Tutorial** - Learn the basics in 2 minutes

**Already configured?** The system auto-detects existing setups and skips onboarding.

For advanced configuration options, see the [Onboarding Guide](docs/ONBOARDING_GUIDE.md).

## Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/M4MEET/soulgate.git
cd soulgate

# Build
make build

# Optional: Install to $GOPATH/bin
make install
```

### Interactive Setup (Recommended)

SoulGate features a user-friendly interactive setup wizard:

```bash
# Run the setup wizard
./bin/soulgate setup
```

The wizard will guide you through:
1. Workspace path configuration
2. Model provider setup (OpenAI, Anthropic, or Ollama)
3. Security policy configuration (strict/moderate/permissive)
4. Consolidated agents setup
5. Audit logging and notifications
6. Configuration review

### Quick Setup (Defaults)

For quick testing with default settings:

```bash
# Initialize with defaults
cd your-project
soulgate init

# Set your API key
export OPENAI_API_KEY=sk-...
# or
export ANTHROPIC_API_KEY=sk-ant-...
```

### Usage

```bash
# Run a prompt (starts agentic loop)
soulgate run "What files are in this directory?"

# More complex operations
soulgate run "Read README.md and summarize the key features"

# View audit log
soulgate audit tail

# Show active policies
soulgate policy show

# Manage agents
soulgate agents list
```

## Architecture

SoulGate sits between LLM agents and system resources:

```
User → CLI → Orchestrator → Model Provider (OpenAI/Anthropic)
                ↓
         Plugin Runtime (WASM)
                ↓
         Resource Brokers → Policy Engine → Audit Log
                ↓
         System Resources
```

### Key Components

- **Orchestrator**: Coordinates model calls, tool execution, and broker routing. Implements the agentic loop (model → tools → model) with conversation history.
- **Policy Engine**: Evaluates allow/deny decisions based on YAML policies with priority-based rule evaluation.
- **Resource Brokers**: Mediate access to files, network, secrets, execution. Currently FileBroker is fully implemented.
- **Model Providers**: OpenAI and Anthropic integrations with full tool calling support.
- **Plugin Runtime**: Executes sandboxed WASM plugins with controlled capabilities.
- **Audit Logger**: Records all operations to SQLite for forensic analysis and compliance.

## Documentation

- [Setup Guide](SETUP_GUIDE.md) - Interactive setup wizard walkthrough
- [Quick Start](QUICKSTART.md) - Complete feature walkthrough
- [Agentic Loop](AGENTIC_LOOP.md) - How the agentic loop and tool calling works
- [Architecture](ARCHITECTURE.md) - Detailed system design
- [Security](SECURITY.md) - Security model and threat analysis
- [Implementation Status](IMPLEMENTATION_STATUS.md) - Current status and roadmap

## Project Structure

```
soulgate/
├── cmd/soulgate/          # CLI entry point and commands
├── internal/              # Core implementation
│   ├── core/              # Orchestrator and session management
│   ├── model/             # LLM provider adapters
│   ├── policy/            # Policy engine
│   ├── audit/             # Audit logging
│   ├── brokers/           # Resource brokers (files, net, secrets, exec)
│   └── plugins/           # Plugin system (SDK, loader, runtime)
├── plugins/examples/      # Example plugins
├── demo/                  # Demo workspace and scripts
└── docs/                  # Documentation
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint code
make lint

# Build example plugin
make build-plugin

# Run all checks
make check
```

## Security

SoulGate is designed with security as the primary concern:

- **Default deny**: All operations denied unless explicitly allowed by policy
- **Sandbox isolation**: WASM plugins cannot access OS directly
- **Broker mediation**: All resource access goes through policy-enforced brokers
- **Audit everything**: Complete log of all operations for forensics
- **Path validation**: Prevents traversal attacks and workspace escapes
- **Schema validation**: All tool calls validated against JSON schemas

See [SECURITY.md](SECURITY.md) for security considerations and threat model.

## Roadmap

### v0.1 (Complete)
- [x] Project setup and dependencies
- [x] Core orchestrator with agentic loop
- [x] Model provider adapters (OpenAI, Anthropic)
- [x] Policy engine with priority-based rules
- [x] FileBroker (read/list operations)
- [x] Interactive setup wizard
- [x] CLI commands (init, setup, run, audit, plugin, policy, agents)
- [x] Comprehensive audit logging
- [x] Tool calling and execution
- [x] Multi-turn conversations

### v0.2 (Planned)
- [ ] File write operations with approval workflow
- [ ] WASM plugin runtime (full bridge)
- [ ] Example WASM plugins
- [ ] NetBroker for HTTP requests
- [ ] SecretBroker for credential management
- [ ] ExecBroker for command execution
- [ ] Streaming output
- [ ] Advanced policy conditions (time-based, context-aware)

### v0.3+ (Future)
- [ ] Plugin versioning and upgrades
- [ ] Daemon mode with API
- [ ] Web UI for monitoring
- [ ] Multi-agent coordination
- [ ] Remote model providers

## License

[Choose appropriate license - MIT, Apache 2.0, etc.]

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Credits

Built with:
- [wazero](https://github.com/tetratelabs/wazero) - Pure Go WASM runtime
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [SQLite](https://modernc.org/sqlite) - Embedded database
- [Zap](https://github.com/uber-go/zap) - Structured logging
