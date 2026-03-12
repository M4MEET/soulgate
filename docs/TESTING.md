# SoulGate Testing Strategy

This document defines the testing strategy for each agent, coverage requirements, test types, and testing coordination protocols.

## Overview

Testing is critical for multi-agent development. Each agent must maintain high test coverage and ensure their changes don't break other agents' functionality.

## Testing Principles

1. **Test Independently**: Each agent can run their tests without depending on other agents
2. **Test Interfaces**: Test that your implementations satisfy shared interfaces
3. **Test Integration**: Test interactions between agents
4. **Test Security**: Security-critical components must have extensive security tests
5. **Test Performance**: Track performance metrics to detect regressions

## Coverage Requirements

### Minimum Coverage Targets

| Agent | Unit Tests | Integration Tests | Security Tests | Overall Coverage |
|-------|-----------|-------------------|----------------|------------------|
| Audit | 90% | Required | N/A | 85%+ |
| Policy | 90% | Required | Required | 90%+ |
| Config | 85% | Required | N/A | 80%+ |
| FileBroker | 90% | Required | **Required** | 90%+ |
| Plugin | 85% | Required | **Required** | 85%+ |
| Model | 85% | Required | N/A | 80%+ |
| Orchestration | 80% | **Required** | N/A | 80%+ |
| CLI | 75% | Required | N/A | 75%+ |

**Note**: Security-critical components (Policy, FileBroker, Plugin) have higher requirements.

---

## Test Types by Agent

### Agent 1: Audit Specialist

#### Unit Tests
**File**: `internal/audit/*_test.go`
**Coverage Target**: 90%

**Test Cases**:
```go
// Event serialization
func TestEvent_MarshalJSON(t *testing.T)
func TestEvent_UnmarshalJSON(t *testing.T)

// Event validation
func TestEvent_Validate(t *testing.T)
func TestEventType_Valid(t *testing.T)

// SQLite operations
func TestSQLiteLogger_Log(t *testing.T)
func TestSQLiteLogger_Query(t *testing.T)
func TestSQLiteLogger_QueryWithFilters(t *testing.T)
func TestSQLiteLogger_Close(t *testing.T)

// Query filtering
func TestQueryFilter_Apply(t *testing.T)
func TestQueryFilter_DateRange(t *testing.T)
func TestQueryFilter_EventType(t *testing.T)
func TestQueryFilter_Status(t *testing.T)
```

#### Integration Tests
**File**: `internal/audit/integration_test.go`

**Test Cases**:
- SQLite database creation and initialization
- Multiple concurrent loggers
- Large event volume (1000+ events)
- Query performance with large datasets

#### Performance Tests
**File**: `internal/audit/bench_test.go`

**Benchmarks**:
```go
func BenchmarkLogger_Log(b *testing.B)
func BenchmarkLogger_Query(b *testing.B)
func BenchmarkLogger_QueryLargeDataset(b *testing.B)
```

**Performance Targets**:
- Log event: < 1ms
- Query 1000 events: < 10ms
- Query 100k events: < 100ms

---

### Agent 2: Policy Specialist

#### Unit Tests
**File**: `internal/policy/*_test.go`
**Coverage Target**: 90%

**Test Cases**:
```go
// Pattern matching
func TestMatcher_Match(t *testing.T)
func TestMatcher_Glob(t *testing.T)
func TestMatcher_Wildcard(t *testing.T)
func TestMatcher_CIDR(t *testing.T) // NEW

// Rule evaluation
func TestEngine_Evaluate(t *testing.T)
func TestEngine_EvaluateMultipleRules(t *testing.T)
func TestEngine_Priority(t *testing.T)

// Policy loading
func TestLoader_LoadFromFile(t *testing.T)
func TestLoader_LoadFromYAML(t *testing.T)
func TestLoader_Validation(t *testing.T)
```

#### Security Tests
**File**: `internal/policy/security_test.go`

**Test Cases**:
```go
// Bypass attempts
func TestEngine_CannotBypassWithEmptyAction(t *testing.T)
func TestEngine_CannotBypassWithInvalidResource(t *testing.T)
func TestEngine_CannotBypassWithMalformedRequest(t *testing.T)

// Edge cases
func TestEngine_DenyOverridesAllow(t *testing.T)
func TestEngine_DefaultDenyWhenNoMatch(t *testing.T)

// Pattern matching edge cases
func TestMatcher_NoRegexInjection(t *testing.T)
func TestMatcher_NoGlobEscape(t *testing.T)
```

#### Integration Tests
**File**: `internal/policy/integration_test.go`

**Test Cases**:
- Load policy from file, evaluate requests
- Policy inheritance and composition
- Policy updates without restart

#### Performance Tests
**Benchmarks**:
```go
func BenchmarkEngine_Evaluate(b *testing.B)
func BenchmarkEngine_EvaluateComplexRules(b *testing.B)
func BenchmarkMatcher_Match(b *testing.B)
```

**Performance Targets**:
- Single rule evaluation: < 100µs
- 100 rules evaluation: < 10ms
- Pattern matching: < 10µs

---

### Agent 3: Config Specialist

#### Unit Tests
**File**: `internal/config/*_test.go`
**Coverage Target**: 85%

**Test Cases**:
```go
// Config parsing
func TestConfig_Load(t *testing.T)
func TestConfig_LoadFromYAML(t *testing.T)
func TestConfig_LoadWithEnvOverrides(t *testing.T)

// Validation
func TestConfig_Validate(t *testing.T)
func TestConfig_ValidateWorkspace(t *testing.T)
func TestConfig_ValidateModel(t *testing.T)
func TestConfig_ValidatePlugins(t *testing.T)

// Workspace initialization
func TestWorkspace_Init(t *testing.T)
func TestWorkspace_InitWithTemplate(t *testing.T)
```

#### Integration Tests
**File**: `internal/config/integration_test.go`

**Test Cases**:
- Load config from file, initialize workspace
- Environment variable overrides
- Multi-workspace setup

#### Error Handling Tests
**Test Cases**:
```go
// Malformed configs
func TestConfig_MalformedYAML(t *testing.T)
func TestConfig_MissingRequiredFields(t *testing.T)
func TestConfig_InvalidTypes(t *testing.T)
func TestConfig_UnknownFields(t *testing.T)
```

---

### Agent 4: Security/FileBroker Specialist

#### Unit Tests
**File**: `internal/brokers/files/*_test.go`
**Coverage Target**: 90%

**Test Cases**:
```go
// File operations
func TestFileBroker_ReadFile(t *testing.T)
func TestFileBroker_WriteFile(t *testing.T)
func TestFileBroker_ListDir(t *testing.T)
func TestFileBroker_Stat(t *testing.T)

// Path validation
func TestValidatePath(t *testing.T)
func TestValidatePath_Absolute(t *testing.T)
func TestValidatePath_WithinBoundary(t *testing.T)
```

#### Security Tests (CRITICAL)
**File**: `internal/brokers/files/security_test.go`

**Test Cases**:
```go
// Path traversal prevention
func TestFileBroker_PathTraversal_ParentDir(t *testing.T)
func TestFileBroker_PathTraversal_MultipleParents(t *testing.T)
func TestFileBroker_PathTraversal_HiddenParent(t *testing.T)
func TestFileBroker_PathTraversal_MixedSeparators(t *testing.T)

// Symlink handling
func TestFileBroker_Symlink_WithinBoundary(t *testing.T)
func TestFileBroker_Symlink_OutsideBoundary(t *testing.T)

// Permission checks
func TestFileBroker_Permission_Denied(t *testing.T)
func TestFileBroker_Permission_ReadOnly(t *testing.T)

// Boundary checks
func TestFileBroker_BoundaryEnforcement(t *testing.T)
```

**Security Test Matrix**:
| Path | Expected Result |
|------|-----------------|
| `../etc/passwd` | Error: path traversal |
| `../../etc/passwd` | Error: path traversal |
| `./../etc/passwd` | Error: path traversal |
| `dir/../../etc/passwd` | Error: path traversal |
| `dir/../file.txt` | Success (stays in boundary) |
| `/etc/passwd` | Error: outside boundary |
| `~/.ssh/id_rsa` | Error: outside boundary |

#### Integration Tests
**File**: `internal/brokers/files/integration_test.go`

**Test Cases**:
- Read file → policy check → audit log
- Write file → approval workflow → policy check → audit log
- List directory → filter by policy → audit log

#### Performance Tests
**Benchmarks**:
```go
func BenchmarkFileBroker_ReadFile(b *testing.B)
func BenchmarkFileBroker_ListDir(b *testing.B)
func BenchmarkFileBroker_Stat(b *testing.B)
```

**Performance Targets**:
- Read file (< 1KB): < 5ms
- List directory (< 100 files): < 10ms
- Stat file: < 1ms

---

### Agent 5: Plugin Specialist

#### Unit Tests
**File**: `internal/plugins/*_test.go`
**Coverage Target**: 85%

**Test Cases**:
```go
// Manifest parsing
func TestManifest_Parse(t *testing.T)
func TestManifest_Validate(t *testing.T)

// Plugin loading
func TestLoader_Load(t *testing.T)
func TestLoader_Validate(t *testing.T)

// WASM runtime
func TestRuntime_LoadPlugin(t *testing.T)
func TestRuntime_ExecuteTool(t *testing.T)
func TestRuntime_UnloadPlugin(t *testing.T)
```

#### Security Tests (CRITICAL)
**File**: `internal/plugins/security_test.go`

**Test Cases**:
```go
// Sandbox isolation
func TestPlugin_CannotAccessHostFilesystem(t *testing.T)
func TestPlugin_CannotExecuteCommands(t *testing.T)
func TestPlugin_CannotAccessNetwork(t *testing.T)

// Resource limits
func TestPlugin_MemoryLimit(t *testing.T)
func TestPlugin_CPULimit(t *testing.T)
func TestPlugin_TimeLimit(t *testing.T)

// Permission enforcement
func TestPlugin_CannotExceedPermissions(t *testing.T)
func TestPlugin_BrokerAccessDenied(t *testing.T)

// Memory safety
func TestPlugin_NoMemoryLeaks(t *testing.T)
func TestPlugin_NoBufferOverflows(t *testing.T)
```

#### Integration Tests
**File**: `internal/plugins/integration_test.go`

**Test Cases**:
- Load plugin → execute tool → return result
- Plugin requests broker access → policy check → execute
- Plugin error handling and recovery
- Multiple plugins running concurrently

#### WASM Tests
**File**: `internal/plugins/wasm_test.go`

**Test Cases**:
```go
// Memory bridge
func TestWASM_MemoryBridge_HostToGuest(t *testing.T)
func TestWASM_MemoryBridge_GuestToHost(t *testing.T)

// Host functions
func TestWASM_HostFunction_FilesRead(t *testing.T)
func TestWASM_HostFunction_NetRequest(t *testing.T)
```

---

### Agent 6: Model Integration Specialist

#### Unit Tests
**File**: `internal/model/*_test.go`
**Coverage Target**: 85%

**Test Cases**:
```go
// Provider interface
func TestProvider_Complete(t *testing.T)
func TestProvider_Name(t *testing.T)
func TestProvider_SupportedFeatures(t *testing.T)

// Schema conversion
func TestToolSchema_ToOpenAI(t *testing.T)
func TestToolSchema_ToAnthropic(t *testing.T)
func TestToolSchema_FromJSON(t *testing.T)

// OpenAI adapter
func TestOpenAIProvider_Complete(t *testing.T)
func TestOpenAIProvider_FunctionCalling(t *testing.T)

// Anthropic adapter
func TestAnthropicProvider_Complete(t *testing.T)
func TestAnthropicProvider_ToolUse(t *testing.T)
```

#### Mock Tests
**File**: `internal/model/mock_test.go`

**Test Cases**:
- Mock provider for testing without API calls
- Test request/response conversion
- Test error handling

#### Integration Tests (with API keys)
**File**: `internal/model/integration_test.go`

**Test Cases**:
```go
// Real API calls (skipped if no API key)
func TestOpenAI_RealAPICall(t *testing.T)
func TestAnthropic_RealAPICall(t *testing.T)

// Tool calling
func TestOpenAI_ToolCalling(t *testing.T)
func TestAnthropic_ToolCalling(t *testing.T)
```

#### Error Handling Tests
**Test Cases**:
```go
// Rate limits
func TestProvider_RateLimit(t *testing.T)

// Timeouts
func TestProvider_Timeout(t *testing.T)

// Invalid responses
func TestProvider_InvalidResponse(t *testing.T)

// Network errors
func TestProvider_NetworkError(t *testing.T)
```

---

### Agent 7: Orchestration Specialist

#### Unit Tests
**File**: `internal/core/*_test.go`
**Coverage Target**: 80%

**Test Cases**:
```go
// Session management
func TestOrchestrator_CreateSession(t *testing.T)
func TestOrchestrator_GetSession(t *testing.T)

// Run lifecycle
func TestOrchestrator_Run(t *testing.T)
func TestOrchestrator_Close(t *testing.T)
```

#### Integration Tests (CRITICAL)
**File**: `internal/core/integration_test.go`

**Test Cases**:
```go
// Full flow: model → plugin → broker → result
func TestOrchestrator_FullFlow(t *testing.T)
func TestOrchestrator_ModelCallsPlugin(t *testing.T)
func TestOrchestrator_PluginCallsBroker(t *testing.T)

// Error recovery
func TestOrchestrator_ModelError(t *testing.T)
func TestOrchestrator_PluginError(t *testing.T)
func TestOrchestrator_BrokerError(t *testing.T)

// Concurrent execution
func TestOrchestrator_ConcurrentTools(t *testing.T)
func TestOrchestrator_ToolTimeout(t *testing.T)
```

#### End-to-End Tests
**File**: `internal/core/e2e_test.go`

**Test Cases**:
```go
// Real scenarios
func TestE2E_ReadFile(t *testing.T)
func TestE2E_WriteFileWithApproval(t *testing.T)
func TestE2E_PluginExecution(t *testing.T)
func TestE2E_MultiStepTask(t *testing.T)
```

#### Performance Tests
**Benchmarks**:
```go
func BenchmarkOrchestrator_Run(b *testing.B)
func BenchmarkOrchestrator_ConcurrentTools(b *testing.B)
```

---

### Agent 8: CLI/UX Specialist

#### Unit Tests
**File**: `cmd/soulgate/cmd/*_test.go`
**Coverage Target**: 75%

**Test Cases**:
```go
// Command parsing
func TestRootCmd_Execute(t *testing.T)
func TestInitCmd_Execute(t *testing.T)
func TestRunCmd_Execute(t *testing.T)
func TestAuditCmd_Execute(t *testing.T)
func TestPolicyCmd_Execute(t *testing.T)
func TestPluginCmd_Execute(t *testing.T)

// Output formatting
func TestOutput_JSON(t *testing.T)
func TestOutput_Table(t *testing.T)
func TestOutput_Pretty(t *testing.T)
```

#### Integration Tests
**File**: `cmd/soulgate/cmd/integration_test.go`

**Test Cases**:
```go
// CLI commands
func TestCLI_Init(t *testing.T)
func TestCLI_Run(t *testing.T)
func TestCLI_Audit(t *testing.T)
func TestCLI_Policy(t *testing.T)
func TestCLI_Plugin(t *testing.T)

// Interactive features
func TestCLI_Approval(t *testing.T)
func TestCLI_Streaming(t *testing.T)
```

#### Error Message Tests
**Test Cases**:
```go
// User-friendly errors
func TestCLI_ErrorMessages(t *testing.T)
func TestCLI_HelpText(t *testing.T)
func TestCLI_ValidationErrors(t *testing.T)
```

---

## Test Infrastructure

### Test Utilities

**File**: `internal/testutil/testutil.go`

Shared test utilities for all agents:

```go
// Temporary workspace
func NewTestWorkspace(t *testing.T) string

// Mock config
func NewTestConfig(t *testing.T) *config.Config

// Mock audit logger
func NewTestAuditLogger(t *testing.T) audit.Logger

// Mock policy engine
func NewTestPolicyEngine(t *testing.T) *policy.Engine

// Test context
func NewTestContext(t *testing.T) context.Context
```

### Test Fixtures

**Directory**: `testdata/`

Test fixtures for all agents:
- `testdata/configs/` - Test configuration files
- `testdata/policies/` - Test policy files
- `testdata/plugins/` - Test plugin manifests
- `testdata/files/` - Test files for FileBroker

---

## CI/CD Testing Pipeline

### Per-Agent Tests (Parallel)

Each agent runs tests independently:

```yaml
# .github/workflows/agent-audit.yml
name: Audit Agent Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - run: go test -v -race -coverprofile=coverage.txt ./internal/audit/...
      - run: go test -v -bench=. ./internal/audit/...
      - uses: codecov/codecov-action@v4
        with:
          files: ./coverage.txt
          flags: audit
```

### Integration Tests (Sequential)

Integration tests run after all agent tests pass:

```yaml
# .github/workflows/integration.yml
name: Integration Tests
on: [push, pull_request]
jobs:
  integration:
    runs-on: ubuntu-latest
    needs: [test-audit, test-policy, test-config, test-broker, test-plugin, test-model, test-core, test-cli]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -v -race -tags=integration ./...
```

### Security Tests (Required)

Security tests run for security-critical agents:

```yaml
# .github/workflows/security.yml
name: Security Tests
on: [push, pull_request]
jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -v -tags=security ./internal/policy/...
      - run: go test -v -tags=security ./internal/brokers/files/...
      - run: go test -v -tags=security ./internal/plugins/...
```

---

## Test Coordination

### Interface Contract Tests

When an interface changes, contract tests verify all implementations still work:

**File**: `internal/contracts/broker_test.go`

```go
// TestBrokerContract tests that any broker implementation satisfies the interface
func TestBrokerContract(t *testing.T, broker brokers.Broker) {
    ctx := context.Background()
    brokerCtx := brokers.Context{
        SessionID: "test-session",
        RunID:     "test-run",
    }

    // Test basic operations
    result, err := broker.Execute(ctx, brokerCtx, operation)
    require.NoError(t, err)
    require.NotNil(t, result)

    // Test error handling
    invalidOp := brokers.Operation{Action: "invalid"}
    _, err = broker.Execute(ctx, brokerCtx, invalidOp)
    require.Error(t, err)
}

// Run contract test against FileBroker
func TestFileBrokerContract(t *testing.T) {
    broker := files.NewBroker(config)
    TestBrokerContract(t, broker)
}

// Future brokers run the same test
func TestNetworkBrokerContract(t *testing.T) {
    broker := network.NewBroker(config)
    TestBrokerContract(t, broker)
}
```

### Cross-Agent Integration Tests

**File**: `tests/integration/cross_agent_test.go`

```go
// Test that all agents work together
func TestCrossAgent_FullRun(t *testing.T) {
    // Setup all components
    cfg := testutil.NewTestConfig(t)
    auditLogger := audit.NewSQLiteLogger(cfg.Audit)
    policyEngine := policy.NewEngine(cfg.Policy)
    fileBroker := files.NewBroker(cfg.Workspace, policyEngine, auditLogger)

    // Test full flow
    // ...
}
```

---

## Test Execution

### Running Tests Locally

```bash
# Run all tests
make test

# Run agent-specific tests
make test-audit
make test-policy
make test-config
make test-broker
make test-plugin
make test-model
make test-core
make test-cli

# Run integration tests
make test-integration

# Run security tests
make test-security

# Run with coverage
make test-coverage

# Run benchmarks
make bench
```

### Makefile Targets

**File**: `Makefile`

```makefile
.PHONY: test test-audit test-policy test-config test-broker test-plugin test-model test-core test-cli test-integration test-security test-coverage bench

test:
	go test -v -race ./...

test-audit:
	go test -v -race -coverprofile=coverage-audit.txt ./internal/audit/...

test-policy:
	go test -v -race -coverprofile=coverage-policy.txt ./internal/policy/...

test-config:
	go test -v -race -coverprofile=coverage-config.txt ./internal/config/...

test-broker:
	go test -v -race -coverprofile=coverage-broker.txt ./internal/brokers/...

test-plugin:
	go test -v -race -coverprofile=coverage-plugin.txt ./internal/plugins/...

test-model:
	go test -v -race -coverprofile=coverage-model.txt ./internal/model/...

test-core:
	go test -v -race -coverprofile=coverage-core.txt ./internal/core/...

test-cli:
	go test -v -race -coverprofile=coverage-cli.txt ./cmd/soulgate/...

test-integration:
	go test -v -race -tags=integration ./tests/integration/...

test-security:
	go test -v -tags=security ./internal/policy/...
	go test -v -tags=security ./internal/brokers/files/...
	go test -v -tags=security ./internal/plugins/...

test-coverage:
	go test -v -race -coverprofile=coverage.txt ./...
	go tool cover -html=coverage.txt -o coverage.html

bench:
	go test -v -bench=. -benchmem ./...
```

---

## Testing Best Practices

### 1. Test Naming
- Use descriptive test names: `TestFunction_Scenario_ExpectedResult`
- Example: `TestFileBroker_PathTraversal_ReturnsError`

### 2. Table-Driven Tests
```go
func TestMatcher_Match(t *testing.T) {
    tests := []struct {
        name     string
        pattern  string
        input    string
        expected bool
    }{
        {"exact match", "file.txt", "file.txt", true},
        {"wildcard", "*.txt", "file.txt", true},
        {"no match", "*.txt", "file.go", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := matcher.Match(tt.pattern, tt.input)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

### 3. Test Isolation
- Each test should be independent
- Use `t.Cleanup()` for resource cleanup
- Don't share state between tests

### 4. Use Subtests
```go
func TestFileBroker(t *testing.T) {
    broker := setupBroker(t)

    t.Run("ReadFile", func(t *testing.T) {
        // Test read operations
    })

    t.Run("WriteFile", func(t *testing.T) {
        // Test write operations
    })
}
```

### 5. Mock External Dependencies
```go
type mockProvider struct {
    completeFunc func(ctx, req) (*Response, error)
}

func (m *mockProvider) Complete(ctx, req) (*Response, error) {
    if m.completeFunc != nil {
        return m.completeFunc(ctx, req)
    }
    return nil, errors.New("not implemented")
}
```

---

## FAQ

**Q: How often should I run tests?**
A: Run unit tests before every commit. Integration tests before pushing. CI runs all tests automatically.

**Q: What if my agent's tests are failing?**
A: Fix them before merging. Don't merge failing tests. If blocked, coordinate with affected agents.

**Q: How do I test changes that affect multiple agents?**
A: Run integration tests. Coordinate with affected agents to run their tests too.

**Q: What if tests are slow?**
A: Use `t.Parallel()` for parallel tests. Use mocks to avoid expensive operations. Run benchmarks to find bottlenecks.

**Q: When should I write integration tests?**
A: When your change affects interactions between agents or changes shared interfaces.

---

## Updates

This testing document should be updated when:
- New agents are added
- Testing requirements change
- New test types are needed
- CI/CD pipeline changes
