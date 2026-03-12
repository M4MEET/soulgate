# SoulGate Onboarding Guide

This guide explains the different ways to configure SoulGate and when to use each approach.

## Overview

SoulGate offers **three complementary setup methods**, each designed for different use cases:

| Method | When to Use | Interface | Features |
|--------|-------------|-----------|----------|
| **TUI Onboarding** | First-time setup, quick start | Interactive visual wizard | Auto-triggers on first run, 8-step guided flow |
| **CLI Setup** | Advanced configuration, automation | Command-line wizard | Full security/agents/audit config, scriptable |
| **Manual Config** | Team deployments, CI/CD | Direct file editing | Complete control, version control friendly |

---

## 1. TUI Onboarding (Recommended for First-Time Users)

### What It Does

The **TUI (Terminal User Interface) Onboarding** is a beautiful, interactive wizard that auto-triggers the first time you run `soulgate tui` or `soulgate interactive`. It walks you through:

1. **Welcome** - Introduction to SoulGate
2. **Model Selection** - Choose your AI model (GPT-4o, Claude, Llama, etc.)
3. **API Keys** - Configure provider credentials
4. **Connection Test** - Verify API access
5. **Integrations** - Optional: Setup Slack, GitHub, Notion, etc.
6. **Dependencies** - Auto-install integration packages
7. **Tutorial** - Quick start guide
8. **Complete** - Ready to chat!

### When It Auto-Triggers

The TUI onboarding **automatically launches** when:
- You run `soulgate tui` or `soulgate interactive`
- The workspace has a `.soulgate/` directory (initialized)
- The `.soulgate/.onboarding_complete` marker file **doesn't exist**

### Manual Trigger

You can manually enter onboarding anytime by:
```bash
# Inside the TUI
/onboarding

# Or start TUI which will trigger onboarding if not completed
soulgate tui
```

### What Gets Created

After completion, the onboarding creates:
- `.soulgate/config.yml` - Your model and integration settings
- `.soulgate/.onboarding_complete` - Marker to prevent re-triggering
- API keys stored securely in config

### Example Flow

```bash
# First time - auto-triggers onboarding
cd ~/my-project
soulgate init
soulgate tui

# ╭─ 🎯 Welcome to SoulGate ─────────────────────╮
# │ ████████████████████░░░░░░░░░░░░░░░░░░░░░ 40%│
# │                                              │
# │ Choose your default AI model:                │
# │                                              │
# │  1. ⚡ GPT-4o-mini ⭐ Recommended             │
# │     Fast & economical - Simple tasks         │
# │                                              │
# │  2. 🧠 GPT-4o                                │
# │     Most capable - Complex coding & analysis │
# │                                              │
# │  3. 🎭 Claude Sonnet 4 ⭐ Recommended         │
# │     Balanced - Great for most tasks          │
# │                                              │
# ╰──────────────────────────────────────────────╯
```

---

## 2. CLI Setup (Advanced Configuration)

### What It Does

The **CLI Setup Wizard** (`soulgate setup`) is a comprehensive command-line wizard that configures:

1. **Model Provider** - AI model selection and API keys
2. **Security Policy** - File access, network, execution permissions
3. **Plugin Management** - Enable/disable plugins
4. **Audit Logging** - Security event logging configuration
5. **Advanced Settings** - Timeouts, rate limits, memory limits
6. **Agent Configuration** - Multi-agent orchestration setup

### When to Use

Use `soulgate setup` when you need:
- **Advanced security configuration** (custom policies, path restrictions)
- **Team setup** (configuring multiple agents, shared policies)
- **CI/CD integration** (scripted, non-interactive setup)
- **Enterprise deployments** (audit logs, compliance requirements)
- **Reconfiguration** (changing security settings after initial setup)

### Usage

```bash
# Interactive wizard
soulgate setup

# Specific sections only
soulgate setup --model-only      # Just configure AI model
soulgate setup --policy-only     # Just configure security
soulgate setup --agents-only     # Just configure agents

# Non-interactive (for scripts)
soulgate setup --non-interactive \
  --provider openai \
  --api-key sk-... \
  --model gpt-4o-mini \
  --policy strict
```

### What Gets Created

The setup wizard creates/updates:
- `.soulgate/config.yml` - Full configuration
- `.soulgate/policy.yml` - Security policies
- `.soulgate/agents.yaml` - Agent definitions
- `.soulgate/audit.db` - Audit log database
- `plugins/` - Plugin directory

### Example: Security-First Setup

```bash
soulgate setup

# 1. MODEL PROVIDER
#    Provider: OpenAI
#    Model: gpt-4o-mini
#    API Key: ********
#
# 2. SECURITY POLICY CONFIGURATION
#    Mode: strict
#    ✓ Only workspace files
#    ✓ No network access
#    ✓ No command execution
#
# 3. AUDIT LOGGING
#    ✓ Enabled
#    Location: .soulgate/audit.db
#
# ✓ Setup complete!
```

---

## 3. Manual Configuration

### When to Use

Edit config files directly when:
- **Version control** - Committing team configuration to git
- **Infrastructure as Code** - Terraform, Ansible, etc.
- **Advanced customization** - Fine-tuning settings not exposed in wizards
- **Bulk changes** - Updating multiple settings at once

### Configuration Files

#### `.soulgate/config.yml`

Main configuration file:

```yaml
workspace:
  root: /Users/you/project
  config_dir: .soulgate

model:
  default_provider: openai
  openai:
    api_key: sk-...  # Or use OPENAI_API_KEY env var
    model: gpt-4o-mini
    max_tokens: 4096
    temperature: 0.7
  anthropic:
    api_key: sk-ant-...
    model: claude-sonnet-4-20250514
    max_tokens: 4096

plugins:
  dir: plugins
  timeout: 30
  max_memory: 67108864  # 64MB

audit:
  enabled: true
  database_path: .soulgate/audit.db

policy:
  file_path: .soulgate/policy.yml

execution:
  max_iterations: 10
  total_timeout_sec: 300
  max_tokens: 100000

integrations:
  slack:
    enabled: true
    config:
      slack_token: xoxb-...
      slack_default_channel: "#general"
  github:
    enabled: true
    config:
      github_token: ghp_...
```

#### `.soulgate/policy.yml`

Security policies:

```yaml
version: "1"

policies:
  # Allow reading workspace files
  - name: allow-workspace-reads
    action: files.read
    resource: "./**"
    decision: allow
    priority: 10

  # Deny parent directory access
  - name: deny-parent-access
    action: files.*
    resource: "../**"
    decision: deny
    priority: 20  # Higher priority = evaluated first

  # Require approval for network access
  - name: require-approval-network
    action: net.http
    resource: "*"
    decision: require_approval
    priority: 15
```

### Environment Variables

Override config with environment variables:

```bash
# API Keys (recommended over storing in config)
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GROQ_API_KEY="gsk_..."
export XAI_API_KEY="xai-..."

# Provider Selection
export SOULGATE_PROVIDER="openai"
export SOULGATE_MODEL="gpt-4o-mini"

# Integration Tokens
export SLACK_BOT_TOKEN="xoxb-..."
export GITHUB_TOKEN="ghp_..."
export NOTION_TOKEN="secret_..."
```

---

## Migration & Backward Compatibility

### Existing Workspaces

If you already have a configured SoulGate workspace, the **auto-migration system** ensures you won't be forced through onboarding:

**What Happens:**
1. When you run `soulgate tui`, the system checks for `.onboarding_complete`
2. If not found, it checks for existing API keys in config or environment
3. If API keys exist, it creates `.onboarding_complete` marker automatically
4. You proceed directly to the chat interface

**Why This Matters:**
- Upgrading SoulGate won't disrupt your workflow
- Existing configurations remain valid
- No need to manually create marker files

### Fresh Installs

For new users, the first run of `soulgate tui` will:
1. Auto-trigger the onboarding wizard
2. Walk through 8 setup steps
3. Create `.onboarding_complete` marker
4. Future runs skip directly to chat

---

## Comparison Table

### Features by Setup Method

| Feature | TUI Onboarding | CLI Setup | Manual Config |
|---------|----------------|-----------|---------------|
| **Visual Interface** | ✅ Beautiful UI | ⚠️ Text-based | ❌ File editing |
| **Auto-trigger** | ✅ First run | ❌ Manual | ❌ Manual |
| **Model Selection** | ✅ Guided | ✅ Wizard | ✅ Full control |
| **API Key Setup** | ✅ Interactive | ✅ Prompted | ✅ File/env |
| **Integration Setup** | ✅ Optional | ⚠️ Limited | ✅ Full control |
| **Security Policy** | ❌ Basic | ✅ Advanced | ✅ Full control |
| **Agent Config** | ❌ | ✅ Full | ✅ Full control |
| **Audit Setup** | ❌ | ✅ | ✅ Full control |
| **Scriptable** | ❌ | ✅ | ✅ |
| **Team Friendly** | ❌ | ⚠️ | ✅ Git-friendly |
| **Best For** | First-time users | Advanced config | Teams, CI/CD |

---

## Troubleshooting

### "Onboarding keeps triggering"

**Cause:** The `.onboarding_complete` marker is missing.

**Solution:**
```bash
# Manually create the marker
touch .soulgate/.onboarding_complete

# Or complete the onboarding wizard
soulgate tui
# Follow wizard to completion
```

### "I want to redo onboarding"

**Solution:**
```bash
# Delete the marker
rm .soulgate/.onboarding_complete

# Next TUI launch will trigger onboarding
soulgate tui

# Or use /onboarding command inside TUI
```

### "API key validation fails"

**Symptom:** Error like "invalid token format"

**Solutions:**
```bash
# Check key format
echo $OPENAI_API_KEY     # Should start with sk-
echo $ANTHROPIC_API_KEY  # Should start with sk-ant-

# Verify key works
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Re-run setup
soulgate setup --model-only
```

### "Integration test fails"

**Symptom:** During onboarding, connection test fails

**Solutions:**

1. **Slack:** Verify bot token scopes at `https://api.slack.com/apps`
2. **GitHub:** Check token permissions at `https://github.com/settings/tokens`
3. **Notion:** Ensure integration has page access
4. **Network:** Check firewall/proxy settings
5. **Timeout:** Increase timeout (default: 10s)

---

## Best Practices

### For Individual Developers

1. **Use TUI onboarding** for initial setup
2. **Store API keys** in environment variables (not config files)
3. **Use `/onboarding`** to reconfigure when switching projects
4. **Check audit logs** periodically: `soulgate audit tail`

### For Teams

1. **Commit** `.soulgate/policy.yml` to version control
2. **Document** required environment variables in README
3. **Use** `soulgate setup --non-interactive` in CI/CD
4. **Provide** example `.soulgate/config.example.yml` (without secrets)
5. **Enable** audit logging for security compliance

### For CI/CD

```bash
# Non-interactive setup in CI
export OPENAI_API_KEY="${CI_OPENAI_KEY}"

soulgate init
soulgate setup --non-interactive \
  --provider openai \
  --model gpt-4o-mini \
  --policy strict

# Run SoulGate commands
soulgate run "Analyze test failures"
```

---

## Next Steps

After completing onboarding:

1. **Try the TUI:** `soulgate tui` or `soulgate interactive`
2. **Explore commands:** `/help` inside TUI or `soulgate --help`
3. **Configure integrations:** `soulgate setup` → Integrations
4. **Review security:** `cat .soulgate/policy.yml`
5. **Check audit logs:** `soulgate audit tail --last 20`

For more information:
- [Quickstart Guide](../QUICKSTART.md)
- [Architecture Overview](../ARCHITECTURE.md)
- [Security Model](../SECURITY.md)
