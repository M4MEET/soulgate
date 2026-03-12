# Policy Agent

**Owner**: Agent 2 - Policy Specialist
**Independence Level**: HIGH
**Status**: Active

## Overview

The Policy Agent is responsible for the security policy engine and rule evaluation. It provides pattern matching, rule priority management, and policy decision-making for all resource access requests.

## Responsibilities

- Security policy engine
- Pattern matching (glob, wildcards, CIDR blocks)
- Rule priority and evaluation
- Policy validation and loading
- Policy decision-making (allow/deny/ask)

## Package Structure

```
internal/policy/
├── README.md          # This file
├── policy.go          # Policy types and interfaces (CRITICAL INTERFACE)
├── engine.go          # Policy engine implementation
├── matcher.go         # Pattern matching logic
├── loader.go          # Policy loading and validation
├── policy_test.go     # Policy tests
├── engine_test.go     # Engine tests
├── matcher_test.go    # Matcher tests
└── security_test.go   # Security tests (planned)
```

## Core Interfaces

### PolicyRequest (CRITICAL)

```go
type PolicyRequest struct {
    SessionID   string
    RunID       string
    Actor       string
    Action      string
    Resource    string
    ResourceType string
    Metadata    map[string]string
}
```

**WARNING**: This type is used by ALL broker agents. Any changes require coordination (see docs/COORDINATION.md).

### PolicyResult (CRITICAL)

```go
type PolicyResult struct {
    Decision  Decision
    Rule      *Rule
    Reason    string
    Metadata  map[string]interface{}
}

type Decision string
const (
    DecisionAllow Decision = "allow"
    DecisionDeny  Decision = "deny"
    DecisionAsk   Decision = "ask"
)
```

### Engine

```go
type Engine struct {
    // internal fields
}

func NewEngine(config Config) (*Engine, error)
func (e *Engine) Evaluate(ctx context.Context, req PolicyRequest) (*PolicyResult, error)
```

## Dependencies

- **stdlib** (context, strings, regexp, etc.)
- **github.com/gobwas/glob** for glob pattern matching

No internal dependencies on other SoulGate agents.

## Usage

### Creating an Engine

```go
import "github.com/yourusername/soulgate/internal/policy"

config := policy.Config{
    PolicyFile: "policy.yaml",
}

engine, err := policy.NewEngine(config)
if err != nil {
    return err
}
```

### Evaluating Requests

```go
req := policy.PolicyRequest{
    SessionID:    "session-abc",
    RunID:        "run-xyz",
    Actor:        "plugin-foo",
    Action:       "files.read",
    Resource:     "/workspace/file.txt",
    ResourceType: "file",
}

result, err := engine.Evaluate(ctx, req)
if err != nil {
    return err
}

switch result.Decision {
case policy.DecisionAllow:
    // Allow the operation
case policy.DecisionDeny:
    // Deny the operation
case policy.DecisionAsk:
    // Prompt user for approval
}
```

### Policy File Format

```yaml
# policy.yaml
policies:
  - name: "allow-workspace-reads"
    effect: allow
    actions:
      - "files.read"
      - "files.list"
    resources:
      - "/workspace/**"
    priority: 100

  - name: "deny-system-files"
    effect: deny
    actions:
      - "files.*"
    resources:
      - "/etc/**"
      - "/sys/**"
      - "/proc/**"
    priority: 200  # Higher priority = evaluated first

  - name: "ask-for-writes"
    effect: ask
    actions:
      - "files.write"
      - "files.delete"
    resources:
      - "/workspace/**"
    priority: 150
```

## Pattern Matching

### Supported Patterns

**Glob Patterns**:
- `*` - matches any sequence of characters
- `?` - matches any single character
- `**` - matches any number of directories
- `[abc]` - matches a, b, or c
- `[a-z]` - matches any character from a to z

**Examples**:
- `*.txt` - matches all .txt files
- `/workspace/**/*.go` - matches all .go files recursively
- `/data/file-?.txt` - matches file-1.txt, file-a.txt, etc.

**Wildcards**:
- `files.*` - matches files.read, files.write, files.delete, etc.

### Planned Pattern Types

- **CIDR blocks**: `192.168.1.0/24` for network policies
- **Regex**: `/^user-\d+$/` for complex patterns
- **Time-based**: `time:09:00-17:00` for office hours

## Testing

**Coverage Target**: 90%+ (security-critical component)

### Running Tests

```bash
# Unit tests
go test -v ./internal/policy/...

# Security tests
go test -v -tags=security ./internal/policy/...

# With coverage
go test -v -coverprofile=coverage.txt ./internal/policy/...
```

### Test Files

- `policy_test.go` - Policy type tests
- `engine_test.go` - Engine evaluation tests
- `matcher_test.go` - Pattern matching tests
- `security_test.go` - Security bypass tests (planned)

### Current Status

- **Tests**: 4 passing
- **Coverage**: 100%

### Security Test Requirements

All policy changes must pass security tests:
- Cannot bypass with empty action
- Cannot bypass with invalid resource
- Cannot bypass with malformed request
- Deny rules override allow rules
- Default deny when no match

## Performance Targets

| Operation | Target | Current |
|-----------|--------|---------|
| Single rule evaluation | < 100µs | ✓ |
| 100 rules evaluation | < 10ms | ✓ |
| Pattern matching | < 10µs | ✓ |

## Planned Work

### Phase 1 (Current)
- [x] Basic policy engine
- [x] Glob pattern matching
- [x] Priority-based evaluation
- [x] Policy loading from YAML

### Phase 2 (Next)
- [ ] Add advanced pattern matching (CIDR blocks, regex)
- [ ] Implement time-based rules (office hours, rate limits)
- [ ] Add policy composition/inheritance
- [ ] Build policy testing framework
- [ ] Performance optimization (rule caching)

### Phase 3 (Future)
- [ ] Policy templates and presets
- [ ] Policy impact analysis
- [ ] Policy simulation mode
- [ ] Policy version control

## Coordination Points

### PolicyRequest/Result Changes

**CRITICAL**: PolicyRequest and PolicyResult types are used by all broker agents. Any changes must follow the Interface Change Protocol (see docs/COORDINATION.md).

**Process**:
1. Propose change in GitHub issue
2. Notify all broker agents
3. Add new fields (backward compatible preferred)
4. Deprecate old fields (2-version cycle)
5. Update all broker call sites

### Consumers

The following components evaluate policies:
- FileBroker (file operation policies)
- NetworkBroker (network request policies - planned)
- DatabaseBroker (database query policies - planned)
- CloudBroker (cloud API policies - planned)

### New Decision Types

Adding a new decision type (e.g., `DecisionAudit`) is a BREAKING change that affects all brokers.

**Example**:
```go
// Proposed: Add "audit" decision
const DecisionAudit Decision = "audit" // allow but log extensively
```

This requires:
1. Proposal in GitHub issue
2. Review by all broker agents
3. Update broker logic to handle new decision
4. Coordinate deployment

## Security Considerations

### Policy Bypass Prevention

The policy engine is a security-critical component. All changes must be reviewed for:
- Pattern matching bypass attempts
- Priority manipulation
- Malformed request handling
- Default deny behavior

### Rule Priority

- Higher priority rules are evaluated first
- Deny rules should have higher priority than allow rules
- Default deny when no rules match

### Pattern Security

- No regex injection vulnerabilities
- No glob escape sequences
- Bounded pattern matching (no infinite loops)

### Audit Integration

- All policy decisions are logged to audit system
- Include rule matched, decision, and reason
- No sensitive data in audit logs

## API Stability

| Interface | Stability | Version |
|-----------|-----------|---------|
| `PolicyRequest` | Stable | v1 |
| `PolicyResult` | Stable | v1 |
| `Engine` | Stable | v1 |
| `Decision` | Stable | v1 |

## Rule Evaluation Logic

### Priority-Based Evaluation

1. Load all rules
2. Sort by priority (descending)
3. For each rule:
   - Check if action matches
   - Check if resource matches
   - If both match, return decision
4. If no rules match, return default deny

### Matching Logic

**Action Matching**:
- Exact match: `files.read` matches `files.read`
- Wildcard match: `files.*` matches `files.read`, `files.write`, etc.

**Resource Matching**:
- Glob pattern: `/workspace/**` matches `/workspace/file.txt`, `/workspace/subdir/file.txt`, etc.

### Example Evaluation

```yaml
policies:
  - name: "deny-system"
    effect: deny
    actions: ["files.*"]
    resources: ["/etc/**"]
    priority: 200

  - name: "allow-workspace"
    effect: allow
    actions: ["files.read"]
    resources: ["/workspace/**"]
    priority: 100
```

Request: `files.read /etc/passwd`
1. Check "deny-system" (priority 200): action matches, resource matches → **DENY**

Request: `files.read /workspace/file.txt`
1. Check "deny-system" (priority 200): action matches, resource doesn't match → continue
2. Check "allow-workspace" (priority 100): action matches, resource matches → **ALLOW**

Request: `files.write /workspace/file.txt`
1. Check "deny-system" (priority 200): action doesn't match → continue
2. Check "allow-workspace" (priority 100): action doesn't match → continue
3. No match → **DENY (default)**

## Contributing

See docs/AGENTS.md for agent responsibilities and coordination protocols.

### Before Making Changes

1. Check if your change affects PolicyRequest/Result types
2. If yes, follow the Interface Change Protocol
3. If no, make changes and ensure tests pass
4. All changes require security review
5. Update this README if adding new features

### Code Review

- Policy agent owner reviews all changes
- If PolicyRequest/Result changes, all broker agents must review
- All changes require security review (see docs/COORDINATION.md)

## Contact

**Owner**: @policy-agent (see CODEOWNERS)
**Security Review**: @security-reviewer
**Coordination**: See docs/COORDINATION.md
