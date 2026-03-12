# SoulGate

A security-focused AI gateway for the terminal. SoulGate sits between LLM agents and your system, enforcing policies and auditing every operation.

## Install

```bash
git clone https://github.com/M4MEET/soulgate.git
cd soulgate
make build
```

The binary is built to `bin/soulgate`. Optionally move it to your PATH:

```bash
cp bin/soulgate /usr/local/bin/
```

## Setup

### 1. Initialize a workspace

```bash
cd your-project
soulgate init
```

This creates a `.soulgate/` directory with default config and policies.

### 2. Set your API key

SoulGate supports multiple providers. Set the key for your preferred one:

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-..."

# Groq (fast inference)
export GROQ_API_KEY="gsk_..."

# Google (Gemini)
export GOOGLE_API_KEY="..."

# Ollama (local, no key needed)
# Just have ollama running locally
```

### 3. Start chatting

```bash
# Interactive TUI (recommended)
soulgate tui

# Single prompt
soulgate run "Read example.txt and summarize it"
```

First launch opens an onboarding wizard that walks you through model selection, API key setup, and basic configuration.

## Usage

### Interactive mode

```bash
soulgate tui
```

Inside the TUI:

| Command | Description |
|---|---|
| `/help` | Show all commands and shortcuts |
| `/model` | Switch AI model or provider |
| `/status` | Show current configuration |
| `/tools` | List available tools |
| `/skills` | List loaded skills |
| `/memory` | Show conversation memory |
| `/setup` | Integration setup wizard |
| `/clear` | Clear screen |
| `!command` | Run a shell command |

Keyboard shortcuts: `Tab` autocomplete, `Up/Down` history, `PgUp/PgDn` scroll, `Ctrl+C` exit.

### Single prompt mode

```bash
soulgate run "What files are in this directory?"
soulgate run "Read main.go and explain what it does"
```

### Policy management

```bash
soulgate policy show       # View active policies
```

Policies control what the AI agent can access. Edit `.soulgate/policy.yml`:

```yaml
version: "1"
policies:
  - name: "allow-workspace-reads"
    action: "files.read"
    resource: "./**"
    decision: allow
    priority: 10

  - name: "deny-parent-access"
    action: "files.*"
    resource: "../**"
    decision: deny
    priority: 20
```

Higher priority rules are evaluated first. Default is deny (no matching rule = denied).

### Audit logs

```bash
soulgate audit tail           # Recent events
soulgate audit tail --last 20 # Last 20 events
```

Every file read, write, tool call, and policy decision is recorded to `.soulgate/audit.db`.

### Plugins

```bash
soulgate plugin list    # List installed plugins
```

Plugins run in a WASM sandbox and cannot access the OS directly. They declare required permissions in a `manifest.yml`.

## Configuration

Workspace config lives in `.soulgate/config.yml`. Example templates are provided:

- `.soulgate/config.example.yml` - Full configuration reference
- `.soulgate/policy.example.yml` - Policy rules reference
- `.soulgate/agents.example.yaml` - Agent configuration reference

### Supported providers

| Provider | Models | Key variable |
|---|---|---|
| OpenAI | GPT-4o, GPT-4o-mini, GPT-4-Turbo | `OPENAI_API_KEY` |
| Anthropic | Claude Opus 4, Sonnet 4, Haiku 4 | `ANTHROPIC_API_KEY` |
| Groq | Llama 3.3, Mixtral, Gemma 2 | `GROQ_API_KEY` |
| Google | Gemini 2.0 Flash, 1.5 Pro/Flash | `GOOGLE_API_KEY` |
| Mistral | Large, Medium, Small | `MISTRAL_API_KEY` |
| DeepSeek | V3, Coder | `DEEPSEEK_API_KEY` |
| xAI | Grok 4.1, Grok 3 | `XAI_API_KEY` |
| OpenRouter | 100+ models | `OPENROUTER_API_KEY` |
| Together | Llama 3.1 405B, Qwen 2.5 | `TOGETHER_API_KEY` |
| Perplexity | Sonar, Sonar Pro | `PERPLEXITY_API_KEY` |
| Cohere | Command R+, Command R | `COHERE_API_KEY` |
| Ollama | Any local model | (none, runs locally) |

Switch models at any time with `/model` in the TUI.

## Security model

- **Default deny** - operations are denied unless explicitly allowed by policy
- **Workspace boundaries** - agents cannot access files outside the workspace
- **Path validation** - traversal attacks (`../../etc/passwd`) are blocked
- **WASM sandbox** - plugins run in isolation, no direct OS access
- **Audit trail** - every operation is logged to SQLite

## Development

```bash
make build          # Build binary
make test           # Run all tests
make lint           # Format and vet
make check          # Lint + tests
make test-coverage  # Coverage report
```

## Project structure

```
cmd/soulgate/       CLI entry point and commands
internal/
  core/             Orchestrator and session management
  model/            LLM provider adapters
  policy/           Policy engine
  audit/            Audit logging (SQLite)
  brokers/          Resource brokers (files, net, exec)
  plugins/          WASM plugin system
  ui/               Terminal UI (Bubble Tea)
  skills/           Skills system
  config/           Configuration management
plugins/            Plugin manifests
demo/               Demo workspace
```

## License

MIT
