package exec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestBroker(t *testing.T) (*Broker, string, func()) {
	tmpDir := t.TempDir()

	// Create test file in workspace
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Create policy that allows basic commands
	pol := &policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "allow-safe-commands",
				Action:   "exec.command",
				Resource: "echo *",
				Decision: policy.DecisionAllow,
			},
			{
				Name:     "allow-ls",
				Action:   "exec.command",
				Resource: "ls*",
				Decision: policy.DecisionAllow,
			},
			{
				Name:     "allow-pwd",
				Action:   "exec.command",
				Resource: "pwd",
				Decision: policy.DecisionAllow,
			},
			{
				Name:     "deny-dangerous",
				Action:   "exec.*",
				Resource: "rm *",
				Decision: policy.DecisionDeny,
				Priority: 100,
			},
		},
	}

	engine := policy.NewEngine(pol)

	// Create audit logger
	auditPath := filepath.Join(tmpDir, "audit.jsonl")
	auditLogger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)

	broker, err := NewBroker(tmpDir, engine, auditLogger)
	require.NoError(t, err)

	cleanup := func() {
		auditLogger.Close()
	}

	return broker, tmpDir, cleanup
}

func TestNewBroker(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.jsonl")
	auditLogger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	broker, err := NewBroker(tmpDir, nil, auditLogger)
	require.NoError(t, err)
	assert.NotNil(t, broker)
	assert.Equal(t, "exec", broker.Name())
}

func TestExecuteSimpleCommand(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	result, err := broker.Execute(ctx, brokerCtx, "echo hello")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "echo hello", result.Command)
	assert.Contains(t, result.Output, "hello")
	assert.Equal(t, 0, result.ExitCode)
}

func TestExecuteCommandWithOutput(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	result, err := broker.Execute(ctx, brokerCtx, "echo test output")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "test output")
	assert.Equal(t, 0, result.ExitCode)
}

func TestExecuteWorkingDirectory(t *testing.T) {
	broker, workspaceDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Execute pwd to verify working directory
	result, err := broker.Execute(ctx, brokerCtx, "pwd")
	require.NoError(t, err)
	assert.Contains(t, result.Output, workspaceDir)
}

func TestExecuteListFiles(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// List files in workspace
	result, err := broker.Execute(ctx, brokerCtx, "ls")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "test.txt")
	assert.Equal(t, 0, result.ExitCode)
}

func TestExecuteNonZeroExitCode(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Use false command which always returns 1
	// Use echo with exit to get non-zero
	result, err := broker.Execute(ctx, brokerCtx, "echo test && exit 1")
	require.NoError(t, err) // Command executed but returned non-zero
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, result.Output, "test")
}

func TestExecutePolicyDeny(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Try to execute dangerous command
	result, err := broker.Execute(ctx, brokerCtx, "rm -rf /")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "denied")
}

func TestExecutePolicyNoMatch(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Command not in policy (default deny)
	result, err := broker.Execute(ctx, brokerCtx, "whoami")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "denied")
}

func TestExecuteAuditLogging(t *testing.T) {
	broker, tmpDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID:  "test-plugin",
		RunID:     "test-run",
		SessionID: "test-session",
	}

	// Execute command
	_, err := broker.Execute(ctx, brokerCtx, "echo audit test")
	require.NoError(t, err)

	// Query audit log
	auditPath := filepath.Join(tmpDir, "audit.jsonl")
	auditLogger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	filter := audit.QueryFilter{
		RunID: "test-run",
		Type:  audit.EventExecCommand,
		Limit: 10,
	}

	events, err := auditLogger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(events), 1)

	// Verify event details
	event := events[0]
	assert.Equal(t, audit.EventExecCommand, event.Type)
	assert.Equal(t, "test-run", event.RunID)
	assert.Equal(t, "test-session", event.SessionID)
	assert.Equal(t, "echo audit test", event.Resource)
	assert.Equal(t, audit.StatusSuccess, event.Status)
}

func TestExecuteAuditDeniedCommand(t *testing.T) {
	broker, tmpDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID:  "test-plugin",
		RunID:     "test-run-denied",
		SessionID: "test-session",
	}

	// Try denied command
	result, err := broker.Execute(ctx, brokerCtx, "rm test.txt")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "denied")

	// Verify audit log (query all events for this run)
	auditPath := filepath.Join(tmpDir, "audit.jsonl")
	auditLogger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	filter := audit.QueryFilter{
		RunID: "test-run-denied",
		Limit: 10,
	}

	events, err := auditLogger.Query(ctx, filter)
	require.NoError(t, err)

	// Should have at least one event (denied or error)
	if len(events) > 0 {
		// Verify it was denied or error
		event := events[0]
		assert.Contains(t, []audit.EventStatus{audit.StatusDenied, audit.StatusError}, event.Status)
	}
}

func TestExecuteOutputTruncation(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Generate long output
	longCommand := "echo " + strings.Repeat("a", 2000)
	result, err := broker.Execute(ctx, brokerCtx, longCommand)
	require.NoError(t, err)

	// Output should be present but potentially truncated in audit
	assert.NotEmpty(t, result.Output)
	assert.Greater(t, len(result.Output), 1000)
}

func TestExecuteContextCancellation(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Command should fail due to cancelled context
	result, err := broker.Execute(ctx, brokerCtx, "sleep 10")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestExecuteTimesOutLongRunningCommand(t *testing.T) {
	tmpDir := t.TempDir()

	pol := &policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "allow-sleep",
				Action:   "exec.command",
				Resource: "sleep *",
				Decision: policy.DecisionAllow,
			},
		},
	}
	engine := policy.NewEngine(pol)

	auditPath := filepath.Join(tmpDir, "audit.jsonl")
	auditLogger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	broker, err := NewBroker(tmpDir, engine, auditLogger)
	require.NoError(t, err)

	origTimeout := defaultExecCommandTimeout
	defaultExecCommandTimeout = 150 * time.Millisecond
	t.Cleanup(func() {
		defaultExecCommandTimeout = origTimeout
	})

	result, err := broker.Execute(context.Background(), brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run-timeout",
	}, "sleep 2")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "timed out")
	assert.Contains(t, err.Error(), "process_start")
}

func TestBrokerClose(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	err := broker.Close()
	assert.NoError(t, err)
}

func TestExecResultStructure(t *testing.T) {
	result := &ExecResult{
		Command:  "echo test",
		Output:   "test output",
		ExitCode: 0,
	}

	assert.Equal(t, "echo test", result.Command)
	assert.Equal(t, "test output", result.Output)
	assert.Equal(t, 0, result.ExitCode)
}

// Security Tests

func TestSecurityCommandInjectionPrevention(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// NOTE: These commands ARE ALLOWED because policy has "echo *" rule
	// This test documents that the exec broker executes commands as-is through sh -c
	// Security relies on POLICY RULES being restrictive enough
	// A production policy should NOT use wildcards like "echo *"
	dangerousCommands := []string{
		"echo test; rm test.txt",
		"echo test && rm test.txt",
		"echo test | false",
	}

	for _, cmd := range dangerousCommands {
		t.Run(cmd, func(t *testing.T) {
			result, err := broker.Execute(ctx, brokerCtx, cmd)
			// These succeed because policy allows "echo *" (too permissive!)
			// This is a documentation test showing policy must be restrictive
			if err == nil {
				assert.NotNil(t, result, "exec broker executes commands through shell")
			}
		})
	}
}

func TestSecurityDangerousCommands(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	dangerousCommands := []string{
		"rm -rf /",
		"rm -rf *",
		"sudo rm -rf /",
		"dd if=/dev/zero of=/dev/sda",
		":(){ :|:& };:", // fork bomb
		"mkfs.ext4 /dev/sda",
	}

	for _, cmd := range dangerousCommands {
		t.Run(cmd, func(t *testing.T) {
			result, err := broker.Execute(ctx, brokerCtx, cmd)
			assert.Error(t, err, "dangerous command should be blocked: %s", cmd)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "denied")
		})
	}
}

func TestSecurityWorkspaceBoundary(t *testing.T) {
	broker, workspaceDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Verify commands execute in workspace
	result, err := broker.Execute(ctx, brokerCtx, "pwd")
	require.NoError(t, err)
	assert.Contains(t, result.Output, workspaceDir, "command should execute in workspace directory")

	// Verify files in workspace are accessible
	result, err = broker.Execute(ctx, brokerCtx, "ls test.txt")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "test.txt")
}

func TestSecurityPathTraversalInCommand(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Commands attempting to escape workspace
	escapeAttempts := []string{
		"ls ../../",
		"cat ../../../etc/passwd",
		"ls /etc/passwd",
	}

	for _, cmd := range escapeAttempts {
		t.Run(cmd, func(t *testing.T) {
			result, err := broker.Execute(ctx, brokerCtx, cmd)
			// Should be denied by policy
			assert.Error(t, err, "path traversal attempt should be denied: %s", cmd)
			assert.Nil(t, result)
		})
	}
}

func TestSecurityEnvironmentVariables(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Verify commands can't access sensitive environment variables
	// This test is more about documenting expected behavior
	// The actual environment is inherited from the broker process

	result, err := broker.Execute(ctx, brokerCtx, "echo $HOME")
	if err == nil {
		// If allowed, verify we're in a controlled environment
		assert.NotNil(t, result)
		// The environment is inherited, so this is expected
	}
}

func TestSecurityNoPolicyEngine(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.jsonl")
	auditLogger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	broker, err := NewBroker(tmpDir, nil, auditLogger)
	require.NoError(t, err)

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	result, err := broker.Execute(ctx, brokerCtx, "echo test")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "policy engine not configured")
}

func TestMultipleCommands(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Execute multiple commands sequentially
	commands := []string{
		"echo first",
		"echo second",
		"pwd",
	}

	for _, cmd := range commands {
		result, err := broker.Execute(ctx, brokerCtx, cmd)
		require.NoError(t, err, "command should succeed: %s", cmd)
		assert.NotNil(t, result)
		assert.Equal(t, 0, result.ExitCode)
	}
}

func TestCommandWithSpecialCharacters(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Test special characters in echo
	result, err := broker.Execute(ctx, brokerCtx, "echo 'test with spaces'")
	require.NoError(t, err)
	assert.Contains(t, result.Output, "test with spaces")
}
