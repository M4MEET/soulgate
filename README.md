# SoulGate

**Your AI, everywhere.** One binary. 13 platforms. 13 model providers. Full system access.

SoulGate is a personal AI gateway that connects any LLM to any messaging platform with full tool access — files, shell, browser, voice, code execution, and more. Deploy once, talk to your AI from Telegram, Discord, Slack, WhatsApp, Signal, Teams, Matrix, iMessage, IRC, Twitch, or the web.

```
soulgate gateway start
```

That's it. Your AI is now reachable from every platform you connect.

## Why SoulGate?

- **One command** — `soulgate gateway start` runs everything: HTTP API, WebSocket, Web UI
- **13 connectors** — Telegram, Discord, Slack, WhatsApp, Signal, Teams, Matrix, iMessage, IRC, Twitch, Nostr, Mattermost, Feishu
- **13 providers** — OpenAI, Anthropic, Groq, Gemini, Mistral, DeepSeek, Ollama, and more
- **45+ tools** — files, shell, browser automation, voice, image generation, code execution, semantic memory
- **Multi-agent** — delegate tasks to specialized sub-agents with roles and messaging
- **Security-first** — policy engine, audit logging, permission prompts, workspace boundaries
- **Self-contained** — single 43MB Go binary, no runtime dependencies

## Quick Start

```bash
# Install
git clone https://github.com/M4MEET/soulgate.git
cd soulgate && make build

# Configure (interactive — picks provider, enters API key)
./bin/soulgate tui

# Or set key and go
export ANTHROPIC_API_KEY="sk-ant-..."
./bin/soulgate gateway start
```

Open http://localhost:8080 for the web dashboard. Or connect a messaging platform:

```bash
# Telegram
export TELEGRAM_BOT_TOKEN="..."
soulgate connector telegram

# Discord
export DISCORD_BOT_TOKEN="..."
soulgate connector discord

# Slack (Socket Mode)
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."
soulgate connector slack

# WhatsApp (scans QR code in terminal)
soulgate connector whatsapp

# Signal (requires signal-cli)
soulgate connector signal --phone +1234567890

# And 8 more: teams, matrix, imessage, irc, twitch, nostr, mattermost, feishu
```

## Docker

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
docker compose up
```

## Architecture

```
User ──→ Connector ──→ Gateway ──→ Orchestrator ──→ Model Provider
              ↑           ↑            ↓
         Telegram     Web UI      Tool Execution
         Discord    WebSocket    ├── Files, Shell
         Slack       HTTP API    ├── Browser (Chrome)
         WhatsApp                ├── Voice (TTS/STT)
         Signal                  ├── Code Sandbox
         Teams                   ├── Image Gen (DALL-E)
         Matrix                  ├── Web Search
         iMessage                ├── Semantic Memory
         IRC/Twitch              └── Canvas/Artifacts
         Nostr
         Mattermost
         Feishu
```

## Tools

| Category | Tools |
|----------|-------|
| **Files** | read, write, list, delete, apply_patch |
| **Shell** | exec_command, process management (start/list/poll/log/write/kill) |
| **Web** | web_search, web_fetch, net_request |
| **Browser** | open, screenshot, click, type, eval, html (via Chrome DevTools) |
| **Voice** | speak (TTS), transcribe (STT) via OpenAI |
| **Images** | generate (DALL-E 3 / FAL.ai), edit |
| **Canvas** | create, update, list, preview (HTML/React/SVG/Mermaid artifacts) |
| **Memory** | key-value (write/get/search) + vector embeddings (index/recall/forget) |
| **Agents** | create, list, stop, delegate, message |
| **Code** | code_run (Python/Node/Go/Bash/Ruby), code_install |
| **Other** | pdf_read, cron scheduling, LLM task, model switching, introspect, configure |

## Model Providers

| Provider | Models | Env Variable |
|----------|--------|-------------|
| OpenAI | GPT-4.1, o3 | `OPENAI_API_KEY` |
| Anthropic | Claude Opus 4, Sonnet 4 | `ANTHROPIC_API_KEY` |
| Groq | Llama 3.3 70B | `GROQ_API_KEY` |
| Google | Gemini 2.5 Flash | `GOOGLE_API_KEY` |
| Mistral | Mistral Large | `MISTRAL_API_KEY` |
| DeepSeek | DeepSeek Chat | `DEEPSEEK_API_KEY` |
| xAI | Grok | `XAI_API_KEY` |
| Ollama | Any local model | (no key needed) |
| OpenRouter | 100+ models | `OPENROUTER_API_KEY` |
| Together | Llama, Mixtral | `TOGETHER_API_KEY` |
| Perplexity | Sonar | `PERPLEXITY_API_KEY` |
| Cohere | Command R+ | `COHERE_API_KEY` |
| Azure | GPT-4.1 | `AZURE_OPENAI_API_KEY` |

## CLI Reference

```
soulgate tui                    # Interactive terminal UI
soulgate gateway start          # Start gateway (HTTP + WebSocket + Web UI)
soulgate connector <platform>   # Connect a messaging platform
soulgate doctor                 # Diagnose configuration
soulgate reset --scope sessions # Clear conversation history
soulgate backup                 # Backup config and data
soulgate daemon start           # Run as background service
soulgate config show            # View current config
soulgate sessions export <id>   # Export session to JSON/Markdown/HTML
soulgate sessions search <q>    # Search across sessions
soulgate webhook add <name>     # Add inbound webhook
soulgate skills list            # List available skills
```

## Webhooks

Receive events from GitHub, GitLab, or any service:

```bash
# Add a webhook
soulgate webhook add github-push --format github --secret my-secret

# GitHub sends to: POST http://your-server:8080/webhook/github-push
# SoulGate processes the event and can respond via any connected channel
```

## Security

- **Policy engine** — allow/deny/require-approval rules for every tool
- **Audit logging** — every operation logged to date-rotated JSONL files
- **Permission prompts** — interactive approve/deny/learn in the TUI
- **Workspace boundaries** — file operations scoped to workspace by default
- **Trust mode** — temporary bypass with auto-expiry (30 min)

## Development

```bash
make build          # Build binary
make test           # Run tests
make lint           # Format and vet
soulgate doctor     # Check installation
```

## License

MIT
