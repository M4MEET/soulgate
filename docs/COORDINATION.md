# SoulGate Agent Coordination Protocols

This document defines the coordination protocols for multi-agent development on SoulGate, including interface changes, breaking changes, bug coordination, and security reviews.

## Overview

With 8 specialized agents working on different components, coordination is essential to maintain system integrity. These protocols ensure changes are communicated, reviewed, and implemented safely.

## Communication Channels

### Primary Channels
- **GitHub Issues**: Interface change proposals, breaking changes
- **GitHub Discussions**: Architecture discussions, design decisions
- **Pull Requests**: Code reviews, implementation coordination
- **AGENTS.md**: Agent status, ownership, contact info

### Coordination Artifacts
- **docs/AGENTS.md**: Agent registry (who owns what)
- **docs/INTERFACES.md**: Interface contracts (what can/cannot change)
- **docs/COORDINATION.md**: This document (how to coordinate)
- **docs/TESTING.md**: Testing requirements (how to verify)

---

## Protocol 1: Interface Change Proposal

**When to Use**: Any time you want to change a shared interface (see INTERFACES.md for list)

### Process

#### Step 1: Create Proposal
Create a GitHub issue using this template:

```markdown
## Interface Change Proposal

**Interface**: `package.InterfaceName`
**Owner Agent**: [Your agent name]
**Affected Agents**: [List all agents that use this interface]
**Change Type**: [BREAKING | COMPATIBLE | DEPRECATION]
**Priority**: [CRITICAL | HIGH | MEDIUM | LOW]

### Current Interface
```go
[Code snippet showing current interface]
```

### Proposed Interface
```go
[Code snippet showing proposed changes]
```

### Rationale
[Explain why this change is needed]

### Impact Analysis
- **Agent 1**: [What changes are required?]
- **Agent 2**: [What changes are required?]
- ...

### Migration Plan
1. [Step-by-step migration path]
2. [Include backward compatibility strategy if applicable]

### Backward Compatibility
[How will existing code continue to work? If breaking, explain migration path]

### Testing Plan
- [ ] Unit tests for new interface
- [ ] Integration tests for affected agents
- [ ] Backward compatibility tests
- [ ] Performance tests (if applicable)

### Timeline
- **Proposal**: [Today's date]
- **Review Period**: [48 hours for compatible, 1 week for breaking]
- **Implementation**: [Target date]
- **Migration Complete**: [Target date]

### Review Checklist
- [ ] Owner agent approves
- [ ] All affected agents approve
- [ ] Tests written
- [ ] Documentation updated
```

#### Step 2: Notify Affected Agents
- Tag all affected agents in the GitHub issue
- Post in coordination channel
- Update AGENTS.md if needed

#### Step 3: Review Period
- **Compatible changes**: Minimum 48 hours
- **Breaking changes**: Minimum 1 week
- **Critical security fixes**: Expedited review (notify all agents immediately)

During review:
- Affected agents comment on impact
- Suggest alternatives or improvements
- Identify additional affected components

#### Step 4: Approval
Change is approved when:
- [ ] Owner agent approves
- [ ] All affected agents approve (or provide acceptable alternatives)
- [ ] Security review passed (if applicable)
- [ ] Tests planned

#### Step 5: Implementation
1. Create feature branch
2. Implement changes
3. Write tests
4. Update documentation
5. Create pull request
6. Get code reviews from affected agents
7. Merge after all approvals

#### Step 6: Migration
For breaking changes:
1. Deploy with backward compatibility layer
2. Notify all agents to migrate
3. Track migration progress
4. Remove old interface after all agents migrated

---

## Protocol 2: Breaking Change Management

**Definition**: A breaking change is any change that requires updates in multiple agents or breaks existing functionality.

### Identifying Breaking Changes

Breaking changes include:
- Changing function signatures in shared interfaces
- Removing fields from shared structs
- Changing field types in shared structs
- Renaming public APIs
- Changing behavior of existing functions
- Removing or renaming constants/enums

### Breaking Change Process

#### Phase 1: Announcement (1 week before implementation)
1. Create GitHub issue with "BREAKING CHANGE" label
2. Notify all affected agents
3. Provide migration guide
4. Offer help with migration

#### Phase 2: Compatibility Layer
Implement changes with backward compatibility:

**Option A: Parallel APIs**
```go
// Old API (deprecated)
func OldFunction() error { /* ... */ }

// New API
func NewFunction() error { /* ... */ }
```

**Option B: Adapter Pattern**
```go
// New interface
type LoggerV2 interface {
    Log(ctx context.Context, event *EventV2) error
}

// Adapter for old interface
type LoggerV1Adapter struct {
    v2 LoggerV2
}

func (a *LoggerV1Adapter) Log(ctx context.Context, event *EventV1) error {
    // Convert EventV1 → EventV2
    return a.v2.Log(ctx, convertEvent(event))
}
```

**Option C: Feature Flags**
```go
if config.UseNewAPI {
    // New behavior
} else {
    // Old behavior
}
```

#### Phase 3: Migration Period
- Minimum 2 release cycles (or 2 weeks for active development)
- All agents update to new interface
- Track migration progress in GitHub issue
- Provide support for migration issues

#### Phase 4: Removal
After all agents migrated:
1. Remove old interface
2. Remove compatibility layer
3. Bump major version
4. Update documentation
5. Close migration issue

### Example: Breaking Change for Audit Events

```markdown
## BREAKING CHANGE: Audit Event Schema v2

### Timeline
- **Announcement**: 2024-01-01
- **Compatibility Layer**: 2024-01-08
- **Migration Deadline**: 2024-01-22
- **Removal**: 2024-01-29

### Changes
- `Event.ID` changed from `string` to `int64`
- `Event.Timestamp` changed from `time.Time` to `int64` (Unix timestamp)

### Migration Guide
```go
// Old code
event := &audit.Event{
    ID: "event-123",
    Timestamp: time.Now(),
}

// New code
event := &audit.EventV2{
    ID: 12345,
    Timestamp: time.Now().Unix(),
}
```

### Compatibility Layer
`EventV1ToV2` adapter provided for 2 release cycles.

### Migration Status
- [ ] Orchestrator Agent (owner: @agent7)
- [ ] FileBroker Agent (owner: @agent4)
- [ ] Plugin Agent (owner: @agent5)
```

---

## Protocol 3: Bug Coordination

**When to Use**: When a bug affects multiple agents or requires coordination to fix.

### Bug Report Template

```markdown
## Bug Report

**Severity**: [CRITICAL | HIGH | MEDIUM | LOW]
**Affected Agents**: [List all affected agents]
**Impact**: [Description of impact on system]

### Description
[Clear description of the bug]

### Reproduction Steps
1. [Step 1]
2. [Step 2]
...

### Expected Behavior
[What should happen]

### Actual Behavior
[What actually happens]

### Root Cause Analysis
[If known, explain the root cause]

### Affected Components
- **Agent 1**: [How is this agent affected?]
- **Agent 2**: [How is this agent affected?]

### Proposed Fix
[Description of proposed fix]

### Fix Owner
[Which agent will implement the fix?]

### Coordination Required
- [ ] Agent 1 needs to update X
- [ ] Agent 2 needs to update Y
- [ ] Integration tests need updating

### Testing Plan
- [ ] Unit tests
- [ ] Integration tests
- [ ] Regression tests
```

### Bug Severity Levels

**CRITICAL**: System broken, security vulnerability, data loss
- Response time: Immediate
- Fix timeline: Same day
- Coordination: Synchronous (video call)

**HIGH**: Major functionality broken, significant impact
- Response time: Within 4 hours
- Fix timeline: 1-2 days
- Coordination: GitHub issue + coordination channel

**MEDIUM**: Functionality degraded, workaround available
- Response time: Within 24 hours
- Fix timeline: 1 week
- Coordination: GitHub issue

**LOW**: Minor issue, no significant impact
- Response time: Within 1 week
- Fix timeline: Next sprint
- Coordination: GitHub issue

### Cross-Agent Bug Fix Process

1. **Identify**: File bug report with affected agents tagged
2. **Triage**: Assign severity and fix owner
3. **Coordinate**: Discuss fix approach with affected agents
4. **Implement**: Fix owner implements fix
5. **Review**: Affected agents review changes
6. **Test**: Run integration tests across all affected agents
7. **Deploy**: Merge fix atomically (all changes together)

---

## Protocol 4: Security Review

**When to Use**: Changes to security-critical components require security review.

### Security-Critical Components

These components require security review for all changes:
1. **Policy Engine** (`internal/policy/`)
   - Rule evaluation logic
   - Pattern matching
   - Decision making

2. **FileBroker** (`internal/brokers/files/`)
   - Path validation
   - Path traversal prevention
   - Symlink handling
   - Permission checks

3. **Plugin Runtime** (`internal/plugins/runtime/`)
   - WASM sandbox
   - Host function security
   - Resource limits
   - Permission enforcement

4. **Audit Logger** (`internal/audit/`)
   - Event integrity
   - Tamper prevention
   - Secure storage

### Security Review Process

#### Step 1: Security Review Request
Create GitHub issue with "security-review" label:

```markdown
## Security Review Request

**Component**: [Component name]
**Change Type**: [New feature | Bug fix | Refactor]
**Risk Level**: [HIGH | MEDIUM | LOW]

### Changes
[Description of security-relevant changes]

### Security Considerations
- [ ] Input validation
- [ ] Authentication/authorization
- [ ] Path traversal prevention
- [ ] Injection prevention (SQL, command, etc.)
- [ ] Resource limits
- [ ] Error message disclosure
- [ ] Cryptographic operations
- [ ] Sandbox isolation

### Threat Model
[What attacks could this code prevent or enable?]

### Security Tests
- [ ] Path traversal tests
- [ ] Permission bypass tests
- [ ] Input fuzzing tests
- [ ] Resource exhaustion tests
- [ ] Error condition tests

### Review Checklist
- [ ] Code follows security best practices
- [ ] All inputs validated
- [ ] All errors handled securely
- [ ] Tests cover security edge cases
- [ ] No security regressions
```

#### Step 2: Security Review
Security reviewer checks:
1. **Input Validation**: All inputs validated, sanitized, and bounded
2. **Error Handling**: Errors don't leak sensitive information
3. **Resource Limits**: No unbounded resource usage
4. **Privilege**: Principle of least privilege followed
5. **Tests**: Security test coverage adequate

#### Step 3: Security Approval
Change is approved when:
- [ ] Security reviewer approves
- [ ] All security tests pass
- [ ] No security regressions detected
- [ ] Security documentation updated

### Security Test Requirements

All security-critical changes must include:

**1. Path Traversal Tests** (for FileBroker)
```go
func TestPathTraversal(t *testing.T) {
    maliciousPaths := []string{
        "../etc/passwd",
        "../../etc/passwd",
        "./../etc/passwd",
        "dir/../../etc/passwd",
        "dir/../../../etc/passwd",
    }
    for _, path := range maliciousPaths {
        _, err := broker.ReadFile(ctx, brokerCtx, path)
        require.Error(t, err)
        require.Contains(t, err.Error(), "path traversal")
    }
}
```

**2. Policy Bypass Tests** (for Policy Engine)
```go
func TestPolicyBypass(t *testing.T) {
    // Test that policy cannot be bypassed by:
    // - Empty action
    // - Invalid resource
    // - Missing metadata
    // - Malformed request
}
```

**3. Sandbox Escape Tests** (for Plugin Runtime)
```go
func TestSandboxEscape(t *testing.T) {
    // Test that plugins cannot:
    // - Access host filesystem directly
    // - Execute arbitrary commands
    // - Exhaust host resources
    // - Access other plugins' memory
}
```

**4. Input Fuzzing** (for all components)
```go
func FuzzInputValidation(f *testing.F) {
    // Fuzz all input validation functions
}
```

---

## Protocol 5: Dependency Updates

**When to Use**: Updating shared dependencies that affect multiple agents.

### Dependency Update Process

1. **Announce**: Create issue with dependency update details
2. **Impact Analysis**: Which agents are affected?
3. **Test**: Run all affected agents' tests
4. **Coordinate**: Update all agents atomically
5. **Deploy**: Merge all updates together

### Example: Go Version Update

```markdown
## Dependency Update: Go 1.21 → Go 1.22

### Affected Agents
- All agents (Go runtime)

### Breaking Changes
- [List any Go 1.22 breaking changes]

### Benefits
- [List benefits of upgrade]

### Testing Plan
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Performance benchmarks unchanged

### Rollout Plan
1. Update go.mod
2. Run all tests
3. Fix any compatibility issues
4. Update CI/CD pipeline
5. Deploy
```

---

## Protocol 6: Release Coordination

**When to Use**: Coordinating releases across multiple agents.

### Release Process

#### Step 1: Release Planning
1. Create GitHub milestone for release
2. List all features/fixes to include
3. Identify dependencies between agents
4. Create release timeline

#### Step 2: Feature Freeze
- Set date for feature freeze
- Only bug fixes allowed after freeze
- All features must be complete and tested

#### Step 3: Integration Testing
- Run full integration test suite
- Test all agent interactions
- Verify no regressions

#### Step 4: Release Candidate
- Create RC branch
- Deploy to staging environment
- Run final tests

#### Step 5: Release
- Tag release version
- Update CHANGELOG.md
- Deploy to production
- Announce release

---

## Coordination Meeting Schedule

### Weekly Sync (Async)
**When**: Monday
**Duration**: Async (post updates by EOD)
**Attendees**: All agents

**Agenda**:
- Status updates from each agent
- Interface change proposals
- Upcoming breaking changes
- Blockers/dependencies

### Integration Review (Sync)
**When**: Thursday
**Duration**: 30 minutes
**Attendees**: Orchestration, affected agents

**Agenda**:
- Integration test results
- Cross-agent issues
- Coordination for next week

### Security Review (Sync)
**When**: As needed
**Duration**: 1 hour
**Attendees**: Security reviewer, affected agent

**Agenda**:
- Security change review
- Threat model discussion
- Security test review

---

## Decision-Making Framework

### Decision Types

**1. Local Decisions** (no coordination needed)
- Changes within agent's scope
- No interface changes
- No cross-agent impact
- Agent decides independently

**2. Coordination Decisions** (lightweight coordination)
- Interface changes (compatible)
- Minor behavior changes
- Documentation updates
- Use Interface Change Proposal protocol

**3. Architecture Decisions** (full coordination)
- Breaking changes
- New interfaces
- Major feature additions
- Major refactoring
- Requires approval from all affected agents

### Decision Authority

| Decision Type | Authority | Process |
|--------------|-----------|---------|
| Local changes | Agent owner | Implement and test |
| Compatible interface changes | Agent owner + affected agents | Interface Change Proposal (48hr) |
| Breaking changes | Agent owner + affected agents + architect | Breaking Change Management (1 week) |
| Security changes | Agent owner + security reviewer | Security Review |
| Architecture changes | All agents + architect | GitHub discussion + meeting |

---

## Conflict Resolution

### Conflict Types

**1. Technical Disagreement**
- Agents disagree on approach
- Resolution: Discuss in GitHub issue, architect decides if no consensus

**2. Priority Conflict**
- Multiple agents need same resource/time
- Resolution: Prioritize by business value, architect decides

**3. Interface Dispute**
- Cannot agree on interface design
- Resolution: Create RFC, all agents review, architect decides

**4. Resource Conflict**
- Agents need conflicting changes
- Resolution: Discuss alternatives, find compromise or architect decides

### Escalation Path

1. **Agent-to-Agent**: Discuss directly in GitHub issue
2. **Coordination Meeting**: Discuss in weekly sync
3. **Architect Review**: Escalate to architect
4. **Project Lead**: Final decision if needed

---

## Best Practices

### 1. Communicate Early
- Propose changes early in planning
- Don't surprise other agents with breaking changes
- Use draft PRs to get early feedback

### 2. Over-Communicate
- Better to over-communicate than under-communicate
- Tag affected agents liberally
- Document decisions in GitHub issues

### 3. Respect Boundaries
- Don't modify other agents' code without coordination
- Use interfaces, don't reach into implementation
- Follow the agent scope defined in AGENTS.md

### 4. Test Thoroughly
- Write tests before changing interfaces
- Run integration tests, not just unit tests
- Test backward compatibility

### 5. Document Everything
- Update documentation when changing interfaces
- Write migration guides for breaking changes
- Document design decisions

### 6. Be Responsive
- Respond to coordination requests within 24 hours
- Attend coordination meetings
- Review PRs that affect your agent

---

## FAQ

**Q: How do I know if my change requires coordination?**
A: Check INTERFACES.md. If you're changing a shared interface, coordination is required.

**Q: Can I make a breaking change if it's urgent?**
A: Yes, but you must still notify affected agents and provide a migration path. Security fixes may expedite the process.

**Q: What if I disagree with another agent's proposal?**
A: Comment on the GitHub issue with your concerns. Propose alternatives. Escalate if no consensus.

**Q: How long does coordination take?**
A: Compatible changes: 48 hours. Breaking changes: 1 week. Security reviews: 2-3 days.

**Q: What if an agent is unresponsive?**
A: Ping in coordination channel. If still unresponsive after 48 hours, escalate to architect.

**Q: Can I work on features in parallel with other agents?**
A: Yes! That's the goal. Just coordinate on shared interfaces.

---

## Updates

This coordination document should be updated when:
- New protocols are needed
- Existing protocols aren't working
- New communication channels are added
- Decision-making framework changes
