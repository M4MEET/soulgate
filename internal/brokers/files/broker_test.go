package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestBroker(t *testing.T) (*Broker, string, func()) {
	// Create temporary workspace
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	subFile := filepath.Join(subDir, "sub.txt")
	err = os.WriteFile(subFile, []byte("sub content"), 0644)
	require.NoError(t, err)

	// Create policy that allows workspace access
	pol := &policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "allow-workspace",
				Action:   "files.*",
				Resource: "./**",
				Decision: policy.DecisionAllow,
			},
			{
				Name:     "deny-parent",
				Action:   "files.*",
				Resource: "../**",
				Decision: policy.DecisionDeny,
			},
		},
	}

	engine := policy.NewEngine(pol)

	// Create audit logger
	auditPath := filepath.Join(tmpDir, "audit.db")
	auditLogger, err := audit.NewSQLiteLogger(auditPath)
	require.NoError(t, err)

	// Create broker
	broker, err := NewBroker(tmpDir, engine, auditLogger)
	require.NoError(t, err)

	cleanup := func() {
		auditLogger.Close()
	}

	return broker, tmpDir, cleanup
}

func TestFileBrokerReadFile(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Read existing file
	content, err := broker.ReadFile(ctx, brokerCtx, "test.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)

	// Read file in subdirectory
	content, err = broker.ReadFile(ctx, brokerCtx, "subdir/sub.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("sub content"), content)
}

func TestFileBrokerPathTraversal(t *testing.T) {
	broker, tmpDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Create a file outside workspace
	parentDir := filepath.Dir(tmpDir)
	outsideFile := filepath.Join(parentDir, "outside.txt")
	err := os.WriteFile(outsideFile, []byte("outside content"), 0644)
	require.NoError(t, err)
	defer os.Remove(outsideFile)

	// Try various path traversal attacks
	attacks := []string{
		"../outside.txt",
		"../../outside.txt",
		"./../outside.txt",
		"subdir/../../outside.txt",
		tmpDir + "/../outside.txt",
	}

	for _, attack := range attacks {
		t.Run("attack_"+attack, func(t *testing.T) {
			_, err := broker.ReadFile(ctx, brokerCtx, attack)
			assert.Error(t, err, "path traversal should be blocked: %s", attack)
			assert.Contains(t, err.Error(), "denied", "error should indicate access denial")
		})
	}
}

func TestFileBrokerAbsolutePath(t *testing.T) {
	broker, tmpDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Try to access absolute path outside workspace
	_, err := broker.ReadFile(ctx, brokerCtx, "/etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied")

	// Absolute path within workspace should work
	absPath := filepath.Join(tmpDir, "test.txt")
	content, err := broker.ReadFile(ctx, brokerCtx, absPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)
}

func TestFileBrokerListDir(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// List root directory
	files, err := broker.ListDir(ctx, brokerCtx, ".")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 2) // At least test.txt and subdir

	// Verify file info
	var foundTest, foundSubdir bool
	for _, f := range files {
		if f.Name == "test.txt" {
			foundTest = true
			assert.False(t, f.IsDir)
		}
		if f.Name == "subdir" {
			foundSubdir = true
			assert.True(t, f.IsDir)
		}
	}
	assert.True(t, foundTest, "should find test.txt")
	assert.True(t, foundSubdir, "should find subdir")
}

func TestFileBrokerStat(t *testing.T) {
	broker, _, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// Stat file
	info, err := broker.Stat(ctx, brokerCtx, "test.txt")
	require.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name)
	assert.False(t, info.IsDir)
	assert.Equal(t, int64(12), info.Size) // "test content" = 12 bytes

	// Stat directory
	info, err = broker.Stat(ctx, brokerCtx, "subdir")
	require.NoError(t, err)
	assert.Equal(t, "subdir", info.Name)
	assert.True(t, info.IsDir)
}

func TestFileBrokerPolicyDeny(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Create restrictive policy
	pol := &policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "deny-all",
				Action:   "files.*",
				Resource: "**",
				Decision: policy.DecisionDeny,
			},
		},
	}

	engine := policy.NewEngine(pol)

	// Create audit logger
	auditPath := filepath.Join(tmpDir, "audit.db")
	auditLogger, err := audit.NewSQLiteLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	// Create broker
	broker, err := NewBroker(tmpDir, engine, auditLogger)
	require.NoError(t, err)

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}

	// All operations should be denied
	_, err = broker.ReadFile(ctx, brokerCtx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied")

	_, err = broker.ListDir(ctx, brokerCtx, ".")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied")

	_, err = broker.Stat(ctx, brokerCtx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestFileBrokerAuditLog(t *testing.T) {
	broker, tmpDir, cleanup := setupTestBroker(t)
	defer cleanup()

	ctx := context.Background()
	brokerCtx := brokers.BrokerContext{
		PluginID:  "test-plugin",
		RunID:     "test-run",
		SessionID: "test-session",
	}

	// Perform operations
	broker.ReadFile(ctx, brokerCtx, "test.txt")
	broker.ListDir(ctx, brokerCtx, ".")

	// Query audit log
	auditPath := filepath.Join(tmpDir, "audit.db")
	auditLogger, err := audit.NewSQLiteLogger(auditPath)
	require.NoError(t, err)
	defer auditLogger.Close()

	filter := audit.QueryFilter{
		RunID: "test-run",
		Limit: 100,
	}

	events, err := auditLogger.Query(ctx, filter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(events), 2) // At least read and list

	// Verify event details
	for _, event := range events {
		assert.Equal(t, "test-run", event.RunID)
		assert.Equal(t, "test-session", event.SessionID)
		assert.Equal(t, "test-plugin", event.PluginID)
		assert.Equal(t, audit.CategoryBroker, event.Category)
	}
}
