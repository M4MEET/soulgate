# Security Policy

## 🔒 Overview

SoulGate is a security-focused gateway system. This document outlines how to keep the project and your deployment secure.

---

## 🚨 Reporting Security Vulnerabilities

**DO NOT** open public GitHub issues for security vulnerabilities.

Instead:
1. Email: security@soulgate.example.com (replace with actual email)
2. Use GitHub Security Advisories (private disclosure)
3. Include: description, reproduction steps, impact assessment

We will respond within **48 hours** and provide a fix timeline.

---

## 🛡️ Security Best Practices

### 1. Never Commit Secrets

**❌ NEVER commit these to version control:**
- API keys (OpenAI, Anthropic, etc.)
- Database credentials
- Private keys (.key, .pem files)
- Auth tokens
- Webhook URLs with secrets
- Configuration files with credentials

**✅ Instead, use:**
```bash
# Environment variables
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Or a .env file (which is in .gitignore)
echo "OPENAI_API_KEY=sk-..." > .env
source .env
```

### 2. Use Configuration Templates

We provide example templates **without secrets**:
- `.soulgate/config.example.yml` - Copy to `config.yml`
- `.soulgate/policy.example.yml` - Copy to `policy.yml`
- `.soulgate/agents.example.yaml` - Copy to `agents.yaml`

**Actual config files are in `.gitignore` and will NOT be committed.**

### 3. Review `.gitignore`

Our `.gitignore` protects:
```
✓ .soulgate/config.yml       # May contain API keys
✓ .soulgate/agents.yaml      # May contain webhook URLs
✓ .soulgate/audit.db         # Contains sensitive logs
✓ .env files                 # Environment secrets
✓ *.key, *.pem               # Private keys
✓ secrets/ directory         # Any secret storage
```

### 4. Environment Variables

**Always prefer environment variables for secrets:**

```bash
# Good ✅
export OPENAI_API_KEY="sk-..."
./bin/soulgate run "your prompt"

# Bad ❌
# Don't hardcode in config.yml:
# api_key: "sk-123456..."  # Will be committed!
```

### 5. Audit Database Security

The audit database (`.soulgate/audit.db`) contains:
- All file access logs
- API calls
- Policy decisions
- User actions

**Protection:**
- ✅ In `.gitignore` by default
- ✅ Set proper file permissions: `chmod 600 .soulgate/audit.db`
- ✅ Regular backups (encrypted)
- ✅ Rotate/archive old logs

---

## 🔐 Deployment Security

### Production Checklist

- [ ] **API Keys**: Use environment variables only
- [ ] **File Permissions**: `chmod 600` for config files
- [ ] **Audit Logs**: Enable and monitor regularly
- [ ] **Policy File**: Review and test all rules
- [ ] **Network Access**: Disable if not needed
- [ ] **Command Execution**: Keep disabled unless required
- [ ] **HTTPS Only**: Use TLS for all API calls
- [ ] **Firewall**: Restrict outbound connections
- [ ] **Updates**: Keep SoulGate and dependencies updated

### Minimal Policy (Most Secure)

```yaml
# .soulgate/policy.yml - Strict Security
version: "1"
policies:
  # Only allow reading workspace files
  - name: "allow-workspace-reads"
    action: "files.read"
    resource: "./**"
    decision: allow

  # Deny everything else
  - name: "deny-all"
    action: "*"
    resource: "**"
    decision: deny
    priority: 999
```

### Environment-Specific Configuration

```bash
# Development
export SOULGATE_ENV="development"
export SOULGATE_LOG_LEVEL="debug"

# Staging
export SOULGATE_ENV="staging"
export SOULGATE_LOG_LEVEL="info"

# Production
export SOULGATE_ENV="production"
export SOULGATE_LOG_LEVEL="warn"
export SOULGATE_AUDIT_ENABLED="true"
```

---

## 🔍 Security Features

### 1. Path Traversal Prevention

**Built-in protection against:**
```bash
❌ ../../../etc/passwd
❌ /etc/passwd
❌ symlinks outside workspace
✅ ./workspace/file.txt
```

**Location**: `internal/brokers/files/permissions.go`

**Tests**: 7 comprehensive security tests

### 2. Policy Engine

**Enforces access control at runtime:**
- File system access
- Network requests (planned)
- Command execution (planned)

**Location**: `internal/policy/`

**Tests**: 4 policy tests including bypass attempts

### 3. Audit Logging

**Tracks everything:**
- Every file access
- Every API call
- Every policy decision
- Session information

**Location**: `internal/audit/`

**Query logs:**
```bash
soulgate audit query --type file.read
soulgate audit query --since 24h
soulgate audit query --status denied
```

### 4. Plugin Sandboxing

**WASM plugins are sandboxed:**
- Memory limits (64MB default)
- Execution timeout (30s default)
- No direct system access
- All I/O through host functions

**Location**: `internal/plugins/runtime/`

---

## 🚫 Common Security Mistakes

### ❌ Mistake 1: Committing API Keys

```yaml
# DON'T DO THIS - will be committed to git!
model:
  openai:
    api_key: "sk-proj-abc123..."  # ❌ EXPOSED
```

**Fix:**
```yaml
# Use environment variable instead
model:
  openai:
    api_key: ""  # ✅ Empty in config
```
```bash
export OPENAI_API_KEY="sk-proj-abc123..."
```

### ❌ Mistake 2: Permissive Policies

```yaml
# DON'T DO THIS - allows everything!
policies:
  - name: "allow-all"
    action: "*"
    resource: "/**"
    decision: allow  # ❌ DANGEROUS
```

**Fix:**
```yaml
# Start with deny, explicitly allow what's needed
policies:
  - name: "deny-all"
    action: "*"
    resource: "**"
    decision: deny
    priority: 999

  - name: "allow-workspace-only"
    action: "files.read"
    resource: "./**"
    decision: allow
    priority: 100
```

### ❌ Mistake 3: Ignoring Audit Logs

**Fix:**
```bash
# Regular monitoring
soulgate audit query --since 24h

# Look for denied access (potential attacks)
soulgate audit query --status denied

# Export for analysis
soulgate audit export --format json > audit-$(date +%Y%m%d).json
```

### ❌ Mistake 4: Running as Root

```bash
# DON'T RUN AS ROOT ❌
sudo soulgate run "..."

# Run as regular user ✅
soulgate run "..."
```

### ❌ Mistake 5: Disabled Audit Logging

```yaml
# DON'T DISABLE IN PRODUCTION ❌
audit:
  enabled: false

# ALWAYS ENABLE IN PRODUCTION ✅
audit:
  enabled: true
```

---

## 📋 Security Checklist for Contributors

Before submitting a PR:

- [ ] No secrets/keys in code or config files
- [ ] No hardcoded credentials
- [ ] Security tests passing
- [ ] Path traversal tests passing
- [ ] Policy engine tests passing
- [ ] Audit logging working correctly
- [ ] `.gitignore` updated if needed
- [ ] Documentation updated
- [ ] Security impact assessed

---

## 🔄 Keeping SoulGate Secure

### Regular Maintenance

```bash
# 1. Update dependencies
go get -u ./...
go mod tidy

# 2. Run security tests
make test-security

# 3. Check for vulnerabilities
go list -json -m all | nancy sleuth

# 4. Review audit logs
soulgate audit query --since 7d --status denied

# 5. Update policies as needed
soulgate policy validate
```

### Monitoring

**Watch for:**
- Repeated policy denials (potential attack)
- Unusual file access patterns
- Failed authentication attempts
- Excessive API usage
- Unexpected network connections

---

## 📚 Additional Resources

- [Policy Configuration Guide](docs/AGENTS.md)
- [Audit Log Format](internal/audit/README.md)
- [Broker Security](internal/brokers/README.md)
- [Plugin Security](internal/plugins/README.md)

---

## 🆘 Emergency Response

If you discover a security breach:

1. **Immediate Actions:**
   ```bash
   # Disable the system
   soulgate agents disable --all

   # Export audit logs
   soulgate audit export > breach-$(date +%Y%m%d-%H%M%S).json

   # Revoke API keys
   # (do this at your provider's dashboard)
   ```

2. **Investigation:**
   - Review audit logs for unauthorized access
   - Check policy violations
   - Identify attack vector

3. **Recovery:**
   - Update policies to prevent recurrence
   - Rotate all credentials
   - Update security configurations
   - Document incident

4. **Notification:**
   - Report to security team
   - Notify affected users if applicable
   - Create post-mortem document

---

## 📧 Contact

**Security Team**: security@soulgate.example.com
**Bug Reports**: https://github.com/M4MEET/soulgate/issues (non-security only)
**Documentation**: See `docs/` directory

---

**Last Updated**: 2026-02-14
**Version**: 0.1.0
