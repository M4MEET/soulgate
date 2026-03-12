# 🎬 SoulGate Interactive Demo

This document shows exactly what users will experience when installing and using SoulGate.

---

## 📦 Installation (30 seconds)

### Option 1: One-Command Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash
```

**What happens:**
```
╔═══════════════════════════════════════════════════════╗
║          SoulGate Installation                        ║
╚═══════════════════════════════════════════════════════╝

ℹ️  Detecting platform...
✅ Platform detected: darwin-arm64
ℹ️  Downloading SoulGate v0.1.0...
✅ SoulGate installed to /usr/local/bin/soulgate
✅ Installation verified!

soulgate version 0.1.0

╔═══════════════════════════════════════════════════════╗
║          Installation Complete! 🎉                    ║
╚═══════════════════════════════════════════════════════╝

🚀 Quick Start:

   1. Start the interactive terminal:
      soulgate

   2. Or run setup wizard:
      soulgate setup

   3. Get help:
      soulgate --help

📝 Note: You'll need an OpenAI or Anthropic API key.
   The interactive terminal will help you set it up!
```

### Option 2: Homebrew (macOS)

```bash
brew tap M4MEET/soulgate
brew install soulgate
```

---

## 🚀 First Run Experience

### Just Type: `soulgate`

```bash
$ soulgate
```

**You'll see:**

```
╔═══════════════════════════════════════════════════════╗
║     🤖 SoulGate Interactive AI Terminal 🤖           ║
╚═══════════════════════════════════════════════════════╝

Welcome! I'm your AI assistant with secure access to your
workspace. I can help you with:

  📁 File operations (read, search, analyze)
  💻 Code analysis and generation
  📚 Documentation
  🧪 Testing and quality checks
  📊 Project management

All with security controls and audit logging!

────────────────────────────────────────────────────────

📝 First time? Let me help you set up SoulGate...

Creating workspace...
✅ Workspace created!

Which AI provider do you want to use?
  1) OpenAI (GPT-4, GPT-3.5)
  2) Anthropic (Claude)
  3) Ollama (Local models)

Choice [1-3]: 1

🔑 API Key Not Found

I need an API key for openai to work.

Options:

1) Set it now:
   export OPENAI_API_KEY="sk-proj-..."

2) Get a key:
   https://platform.openai.com/api-keys

3) Skip for now (you can set it later)

Enter your API key (or press Enter to skip): sk-proj-abc123...

✅ API key set for this session!

To make it permanent, add this to your ~/.bashrc or ~/.zshrc:
   export OPENAI_API_KEY="sk-proj-abc123..."

✅ Setup complete! Let's get started...

────────────────────────────────────────────────────────

💬 Type your request below (or 'help' for commands, 'exit' to quit)

You: _
```

---

## 💬 Natural Language Interaction

### Example Session 1: Code Analysis

```
You: List all Go files in this project

🤖 Assistant:
I can help with that! Here are the Go files in your project:

📁 cmd/soulgate/main.go
📁 cmd/soulgate/cmd/root.go
📁 cmd/soulgate/cmd/setup.go
📁 cmd/soulgate/cmd/interactive.go
📁 internal/audit/event.go
📁 internal/audit/logger.go
📁 internal/audit/sqlite.go
... (showing 42 files)

You: What's the test coverage?

🤖 Assistant:
Running tests with coverage...

✅ Tests: 45 passing
📊 Coverage: 85.4%

Packages with low coverage:
  ⚠️ internal/model: 0% (needs tests)
  ⚠️ internal/plugins: 12% (needs improvement)

Would you like me to generate tests for these packages?

You: Yes, generate tests for internal/model

🤖 Assistant:
Generating tests for internal/model...

✅ Created: internal/model/provider_test.go
✅ Created: internal/model/schema_test.go

Generated 12 test cases covering:
  • Provider interface implementations
  • Schema validation
  • Request/response conversion
  • Error handling

Run 'go test ./internal/model' to verify!
```

---

### Example Session 2: Documentation

```
You: Generate API documentation

🤖 Assistant:
I'll generate API documentation for your project!

Analyzing codebase...
  ✓ Found 15 API endpoints
  ✓ Found 8 data models
  ✓ Found 3 authentication methods

Generating documentation...

✅ Created: docs/api/README.md
✅ Created: docs/api/endpoints.md
✅ Created: docs/api/models.md
✅ Created: docs/api/authentication.md
✅ Created: docs/api/openapi.yaml (OpenAPI 3.0 spec)

📖 Documentation available at: docs/api/README.md

You: Update the changelog with recent changes

🤖 Assistant:
Updating changelog...

Analyzing git history...
  ✓ Found 23 commits since last release
  ✓ Identified 5 features, 8 fixes, 2 breaking changes

✅ Updated: CHANGELOG.md

Added section for v0.2.0:
  - 5 new features
  - 8 bug fixes
  - 2 breaking changes
  - 23 commits from 4 contributors
```

---

### Example Session 3: Project Management

```
You: Show current sprint status

🤖 Assistant:
📊 Sprint 12 Status
────────────────────────────────────────────────────────

Sprint: Feb 1-14, 2024 (3 days remaining)

Progress:
  ✅ Completed: 18 points (60%)
  🔄 In Progress: 8 points (27%)
  📋 Todo: 4 points (13%)
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 87%

Team Velocity: 30 points/sprint (on track)

Developers:
  👤 Alice: 8/10 points (80%) ✅
  👤 Bob: 6/10 points (60%) 🔄
  👤 Carol: 4/10 points (40%) ⚠️  (needs support)

Top Risks:
  ⚠️  Task #45 blocked by external dependency
  ⚠️  Carol overcommitted, may miss deadline

You: Assign new tasks to balance workload

🤖 Assistant:
Analyzing workload...

Current situation:
  • Alice: 80% (2 points remaining)
  • Bob: 60% (4 points remaining)
  • Carol: 40% overcommitted

Rebalancing tasks...

✅ Assigned Task #67 (2 pts) to Alice
✅ Assigned Task #68 (3 pts) to Bob
✅ Moved Task #71 (5 pts) from Carol to Bob
✅ Assigned Task #72 (2 pts) to Carol

New balance:
  • Alice: 100% (10/10 points)
  • Bob: 90% (9/10 points)
  • Carol: 70% (7/10 points)

Workload balanced! Team is now on track to complete sprint.
```

---

## 🔧 Special Commands

While chatting, you can use special commands:

```
You: status

📊 SoulGate Status
────────────────────────────────────────────────────────
  Workspace:  /Users/you/project
  Provider:   openai (gpt-4)
  Audit:      ✅ Enabled
  Agents:     3 consolidated agents + 1 service


You: agents

🤖 Available Agents
────────────────────────────────────────────────────────
  ✅ Test & Quality Agent
     • Generate and run tests
     • Security scanning
     • Coverage analysis

  ✅ Docs & API Agent
     • Generate documentation
     • Create API specs
     • Update changelogs

  ✅ Project Management Agent
     • Task assignment
     • Sprint tracking
     • Report generation

  ✅ Notification Service
     • Multi-channel notifications
     • Console, Slack, Email, Webhooks


You: help

╔═══════════════════════════════════════════════════════╗
║              SoulGate Commands                        ║
╚═══════════════════════════════════════════════════════╝

💬 Natural Language:
   Just type what you want to do!
   Examples:
   • List all files in this directory
   • Run tests and show coverage
   • Generate API documentation
   • Show current sprint status

🔧 Special Commands:
   status    - Show workspace status
   agents    - List available agents
   help, ?   - Show this help
   clear     - Clear screen
   exit, quit - Exit terminal

📚 Full CLI:
   You can also use traditional commands:
   • Exit and run: soulgate --help


You: clear

[Screen clears and shows welcome banner again]


You: exit

👋 Goodbye! Thanks for using SoulGate.
```

---

## 🛡️ Security Features (Automatic)

Every operation is automatically protected:

### Path Traversal Prevention
```
You: Read the file ../../../etc/passwd

🤖 Assistant:
❌ Security Policy Violation

Action: files.read
Path: ../../../etc/passwd
Reason: Path traversal outside workspace

Only files within your workspace are accessible.
This action has been logged in the audit trail.
```

### Audit Logging
```bash
# View what AI did
$ soulgate audit query

📋 Recent Actions (last 10)
────────────────────────────────────────────────────────
2024-02-14 10:15:23  ✅ files.read    README.md
2024-02-14 10:15:45  ✅ tests.execute  go test ./...
2024-02-14 10:16:02  ✅ docs.generate  API documentation
2024-02-14 10:16:15  ❌ files.read     ../../../etc/passwd (denied)
```

---

## 🎯 Use Cases

### For Casual Users
"Just run `soulgate` and chat!"

```
You: What files are in this project?
You: Run the tests
You: Show me the README
You: What's in main.go?
```

### For Developers
```
You: Run tests and show coverage report
You: Generate tests for UserService
You: Analyze auth.go for security issues
You: Create API documentation
```

### For Teams
```
You: Show sprint progress
You: Assign tasks to balance workload
You: Generate weekly status report
You: Who's working on authentication?
```

---

## 📖 Traditional CLI (Still Available)

If you prefer traditional commands:

```bash
# Run a prompt
soulgate run "analyze code quality"

# Query audit logs
soulgate audit query --since 24h

# Check security policy
soulgate policy check src/auth.go

# Manage agents
soulgate agents list
soulgate agents enable test_quality

# Plugin management
soulgate plugin list
soulgate plugin install security-scanner
```

---

## 🔄 Switching Between Interactive and CLI

```bash
# Interactive mode (default)
$ soulgate
You: [type naturally]

# Or specific command
$ soulgate status
[shows status and exits]

# Or run a one-off prompt
$ soulgate run "show test coverage"
[executes and exits]
```

---

## 💡 Pro Tips

### 1. Set API Key Permanently
```bash
# Bash/Zsh
echo 'export OPENAI_API_KEY="sk-proj-your-key"' >> ~/.bashrc
source ~/.bashrc

# Fish
set -Ux OPENAI_API_KEY "sk-proj-your-key"
```

### 2. Use Local Models (Free!)
```bash
# Install Ollama
brew install ollama

# Pull a model
ollama pull llama2

# Configure SoulGate
$ soulgate setup
> Choose: 3) Ollama (Local models)

# No API key needed!
$ soulgate
You: [works with local model!]
```

### 3. Check Audit Trail
```bash
# See what AI did
soulgate audit query

# Last hour
soulgate audit query --since 1h

# Only denied actions
soulgate audit query --status denied
```

### 4. Custom Security Policy
```bash
# Edit policy
vim .soulgate/policy.yml

# Add custom rules
rules:
  - action: "files.write"
    pattern: "src/**/*.go"
    effect: "require_approval"
```

---

## 🆘 Troubleshooting

### "Command not found: soulgate"

```bash
# Check installation
which soulgate

# If not found, add to PATH
export PATH="$HOME/.local/bin:$PATH"

# Or reinstall
curl -fsSL https://raw.githubusercontent.com/M4MEET/soulgate/main/install.sh | bash
```

### "API key not configured"

```bash
# Option 1: Set for this session
export OPENAI_API_KEY="sk-proj-your-key"

# Option 2: Run setup again
soulgate setup

# Option 3: Use local model (no key needed)
soulgate setup
> Choose: 3) Ollama
```

### "Workspace not initialized"

```bash
# Make sure you're in your project directory
cd /path/to/your/project

# Then run
soulgate

# Or initialize manually
soulgate init
```

---

## 🎉 Summary

**For Normal Users:**
1. Install: `curl -fsSL https://soulgate.io/install.sh | bash`
2. Run: `soulgate`
3. Chat naturally: "show me the files", "run tests", etc.
4. That's it!

**What You Get:**
- ChatGPT-like interface in your terminal
- Secure access to your files and code
- Built-in security controls and audit logging
- No need to understand configuration files
- Auto-setup on first run
- Natural language commands

**Perfect For:**
- "I want AI to help with my code"
- "I don't want to learn complex CLIs"
- "I need audit trails for compliance"
- "I want ChatGPT but with file access"

---

**Made with ❤️ by the SoulGate team**

Start chatting: `soulgate` 🚀
