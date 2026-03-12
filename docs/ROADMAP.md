# SoulGate Multi-Agent Development Roadmap

This document outlines the implementation roadmap for the multi-agent development workflow.

## Timeline Overview

**Total Duration**: 5-7 weeks

- **Setup** (Week 1): Agent registry, documentation, testing infrastructure
- **Phase 1** (Weeks 1-2): Independent agents (Audit, Policy, Config, Model)
- **Phase 2** (Weeks 3-4): Broker & Plugin agents
- **Phase 3** (Weeks 5-7): Integration & Orchestration

## Current Status

**Setup Phase**: ✅ COMPLETED

- [x] Agent registry created (docs/AGENTS.md)
- [x] Interface contracts documented (docs/INTERFACES.md)
- [x] Coordination protocols defined (docs/COORDINATION.md)
- [x] Testing strategy documented (docs/TESTING.md)
- [x] CODEOWNERS file created
- [x] Per-agent README files created
- [x] Makefile with per-agent test targets

## Phase 1: Independent Work (Weeks 1-2)

**Goal**: Stabilize low-coupling components that can work in parallel

**Status**: READY TO START

### Agent 1: Audit Specialist

**Current State**:
- ✅ Basic event schema
- ✅ SQLite logger implementation
- ✅ 4 tests passing
- ✅ Good coverage

**Tasks**:
- [ ] Optimize SQLite queries for large datasets (>100k events)
- [ ] Add event export formats (JSON, CSV)
- [ ] Implement webhook audit sink
- [ ] Add advanced query filtering
- [ ] Performance tuning and benchmarks
- [ ] Reach 90% test coverage

**Success Criteria**:
- Query 100k events in < 100ms
- Export events to JSON/CSV
- 90%+ test coverage

---

### Agent 2: Policy Specialist

**Current State**:
- ✅ Policy engine implemented
- ✅ Glob pattern matching
- ✅ 4 tests passing
- ✅ 100% coverage

**Tasks**:
- [ ] Add advanced pattern matching (CIDR blocks)
- [ ] Add regex pattern support
- [ ] Implement time-based rules (office hours, rate limits)
- [ ] Add policy composition/inheritance
- [ ] Build policy testing framework
- [ ] Add security bypass tests
- [ ] Performance optimization (rule caching)

**Success Criteria**:
- Support CIDR and regex patterns
- Policy testing framework functional
- All security tests passing
- 100% coverage maintained

---

### Agent 3: Config Specialist

**Current State**:
- ✅ Basic config structure
- ❌ No tests
- ❌ Low coverage

**Tasks**:
- [ ] Write comprehensive unit tests
- [ ] Add config validation with JSON schema
- [ ] Implement config migration/upgrade logic
- [ ] Add workspace templates
- [ ] Support environment-specific configs
- [ ] Add multi-workspace support
- [ ] Reach 85% test coverage

**Success Criteria**:
- Config validation working
- 85%+ test coverage
- Workspace templates available

---

### Agent 6: Model Integration Specialist

**Current State**:
- ✅ Provider interface defined
- ❌ OpenAI adapter incomplete
- ❌ Anthropic adapter incomplete
- ❌ No tests

**Tasks**:
- [ ] Complete OpenAI adapter (function calling)
- [ ] Complete Anthropic adapter (tool use blocks)
- [ ] Add streaming response support
- [ ] Implement token counting
- [ ] Add provider fallback/retry logic
- [ ] Write unit tests with mocks
- [ ] Write integration tests (with API keys)
- [ ] Reach 85% test coverage

**Success Criteria**:
- Both adapters working with real APIs
- Streaming support implemented
- 85%+ test coverage

---

**Phase 1 Coordination**:
- Weekly sync meeting (async) to review progress
- Interface change proposals as needed
- No breaking changes expected (independent work)

---

## Phase 2: Broker & Plugin Work (Weeks 3-4)

**Goal**: Build resource access layer with security enforcement

**Status**: WAITING FOR PHASE 1

**Dependencies**:
- Phase 1 must be complete (stable interfaces)
- Policy Agent must be ready (for broker integration)
- Audit Agent must be ready (for event logging)

### Agent 4: Security/FileBroker Specialist

**Current State**:
- ✅ Basic file operations (read, list, stat)
- ✅ Path traversal prevention
- ✅ 7 security tests passing
- ❌ Write operations not implemented

**Tasks**:
- [ ] Implement write operations with approval workflow
- [ ] Add diff generation for file modifications
- [ ] Implement virtual filesystem mapping
- [ ] Add file watching capabilities
- [ ] Performance optimization (caching, batching)
- [ ] Add comprehensive security tests
- [ ] Integration tests with Policy + Audit
- [ ] Reach 90% test coverage

**Success Criteria**:
- Write operations working with approval
- All security tests passing
- No regressions in path traversal prevention
- 90%+ test coverage

---

### Agent 5: Plugin Specialist

**Current State**:
- ✅ Manifest schema defined
- ✅ Loader implemented
- ❌ WASM runtime incomplete
- ❌ No host functions
- ❌ No tests

**Tasks**:
- [ ] Complete WASM memory bridge (host ↔ guest communication)
- [ ] Implement broker host functions (files_read, etc.)
- [ ] Build Rust SDK for plugin development
- [ ] Add plugin versioning and upgrade system
- [ ] Implement HTTP connector runtime (alternative to WASM)
- [ ] Write comprehensive unit tests
- [ ] Write security tests (sandbox isolation)
- [ ] Write integration tests (end-to-end plugin execution)
- [ ] Reach 85% test coverage

**Success Criteria**:
- WASM plugins can call broker host functions
- Rust SDK functional
- Sandbox isolation verified
- Example plugin working
- 85%+ test coverage

---

**Phase 2 Coordination**:
- Daily standups to align on broker protocol
- FileBroker and Plugin agents coordinate on host function signatures
- Security reviews for all changes

---

## Phase 3: Integration & Orchestration (Weeks 5-7)

**Goal**: Wire everything together into a working system

**Status**: WAITING FOR PHASE 1 & 2

**Dependencies**:
- ALL Phase 1 & 2 agents must have stable interfaces
- Integration tests must be in place
- Security tests must be passing

### Agent 7: Orchestration Specialist

**Current State**:
- ✅ Basic orchestrator structure
- ✅ Session management
- ✅ 3 tests passing
- ❌ Model-plugin-broker integration incomplete

**Tasks**:
- [ ] Complete model-plugin-broker integration loop
- [ ] Implement tool execution pipeline
- [ ] Add streaming output support
- [ ] Implement error recovery and retry logic
- [ ] Add concurrent tool execution
- [ ] Performance optimization
- [ ] Write comprehensive integration tests
- [ ] Write end-to-end tests
- [ ] Reach 80% test coverage

**Success Criteria**:
- Full flow working: prompt → model → plugin → broker → result
- Streaming output working
- Error recovery working
- 80%+ test coverage

---

### Agent 8: CLI/UX Specialist

**Current State**:
- ✅ Basic commands (init, run, audit, policy, plugin)
- ❌ No streaming output
- ❌ No interactive approvals
- ❌ No tests

**Tasks**:
- [ ] Improve run command with streaming output
- [ ] Add interactive approval prompts
- [ ] Implement tab completion
- [ ] Add progress indicators
- [ ] Improve error messages and help text
- [ ] Build audit query DSL
- [ ] Write CLI integration tests
- [ ] Reach 75% test coverage

**Success Criteria**:
- All commands working
- Streaming output functional
- Interactive approvals working
- 75%+ test coverage

---

**Phase 3 Coordination**:
- Continuous integration testing
- All agents must be available for coordination
- Daily standups
- Integration issues resolved immediately

---

## Verification Checkpoints

### After Phase 1
- [ ] All Phase 1 agents have 80%+ test coverage
- [ ] All Phase 1 tests passing
- [ ] No interface changes pending
- [ ] Documentation up to date

### After Phase 2
- [ ] All Phase 2 agents have 85%+ test coverage
- [ ] All security tests passing
- [ ] Integration tests passing
- [ ] No regressions in Phase 1 agents

### After Phase 3
- [ ] Full end-to-end flow working
- [ ] All 8 agents have target coverage
- [ ] All tests passing (unit + integration + security)
- [ ] Performance targets met
- [ ] Documentation complete

---

## Success Metrics

### Code Quality
- [ ] Overall test coverage > 80%
- [ ] All security tests passing
- [ ] No critical bugs
- [ ] No security vulnerabilities

### Development Efficiency
- [ ] Multiple agents working in parallel
- [ ] Interface changes coordinated smoothly
- [ ] Breaking changes rare (<1 per month)
- [ ] Per-agent tests run in <30s

### System Functionality
- [ ] Full orchestration flow working
- [ ] Plugins can execute tools
- [ ] Brokers enforce policies
- [ ] Audit logging complete
- [ ] CLI user-friendly

---

## Risk Management

### High Risk Areas

**1. WASM Runtime (Agent 5)**
- **Risk**: WASM integration complex, may take longer than expected
- **Mitigation**: Start early, implement HTTP runtime as backup
- **Fallback**: Use HTTP connector runtime instead of WASM

**2. Model Integration (Agent 6)**
- **Risk**: API changes from OpenAI/Anthropic
- **Mitigation**: Use versioned APIs, add adapter layer
- **Fallback**: Focus on one provider first (OpenAI)

**3. Orchestration Complexity (Agent 7)**
- **Risk**: Integration of all components complex
- **Mitigation**: Build incrementally, test each integration point
- **Fallback**: Start with simple flow (no plugins)

### Medium Risk Areas

**4. Security Tests (Agents 2, 4, 5)**
- **Risk**: Security vulnerabilities missed
- **Mitigation**: Comprehensive security test suite, external review
- **Fallback**: Security audit before production use

**5. Performance (Agents 1, 2, 4)**
- **Risk**: Performance targets not met
- **Mitigation**: Benchmarks early, optimize incrementally
- **Fallback**: Adjust targets based on real usage

---

## Next Steps (Week 1)

### Immediate Actions

1. **Assign Agent Owners**
   - Assign each agent to a team member or AI agent
   - Update AGENTS.md with ownership

2. **Setup Development Environment**
   - Clone repository
   - Install dependencies (Go, Rust for plugins)
   - Run existing tests: `make test`

3. **Phase 1 Kickoff**
   - Agents 1, 2, 3, 6 start their tasks
   - Create GitHub issues for each agent's tasks
   - Setup agent-specific branches

4. **Communication Channels**
   - Setup coordination channel (Slack/Discord)
   - Setup GitHub project board
   - Schedule weekly sync meeting

### Week 1 Goals

- [ ] All 4 Phase 1 agents have started work
- [ ] Agent 1: Query optimization started
- [ ] Agent 2: Advanced patterns started
- [ ] Agent 3: Tests written
- [ ] Agent 6: OpenAI adapter completed
- [ ] First weekly sync completed

---

## Long-Term Vision (Beyond Phase 3)

### Phase 4: Advanced Features
- Network broker (HTTP/HTTPS requests)
- Database broker (SQL queries)
- Cloud broker (AWS/GCP/Azure APIs)
- Advanced plugin features (persistent storage, IPC)

### Phase 5: Production Readiness
- Security audit
- Performance optimization
- Distributed audit logging
- High availability orchestration
- Production deployment guide

### Phase 6: Ecosystem
- Plugin marketplace
- Community plugins
- Plugin development tools
- Documentation and tutorials

---

## Resources

### Documentation
- [Agent Registry](AGENTS.md)
- [Interface Contracts](INTERFACES.md)
- [Coordination Protocols](COORDINATION.md)
- [Testing Strategy](TESTING.md)

### Tools
- Makefile: `make help` for all targets
- Per-agent tests: `make test-<agent>`
- Coverage: `make test-coverage-all`
- Status: `make agent-status`

### Contact
- Project Lead: TBD
- Security Reviewer: TBD
- Architecture Review: TBD

---

## Updates

This roadmap should be updated:
- Weekly during Phase 1-3
- When milestones are completed
- When timelines change
- When new risks are identified

**Last Updated**: 2026-02-14
**Next Review**: Week 1 (start of Phase 1)
