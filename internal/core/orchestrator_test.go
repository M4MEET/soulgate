package core

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestratorBasic(t *testing.T) {
	// Create temporary workspace
	tmpDir := t.TempDir()

	// Initialize workspace
	workspace, err := config.InitWorkspace(tmpDir)
	require.NoError(t, err)

	// Set test API key in config
	workspace.Config.Model.OpenAI.APIKey = "test-key-for-testing"

	// Create orchestrator
	orch, err := NewOrchestrator(workspace)
	require.NoError(t, err)
	defer orch.Close()

	// Verify session was created
	session := orch.GetSession()
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, tmpDir, session.WorkspaceID)
	assert.Equal(t, SessionActive, session.Status)

	// Verify audit log was created (date-rotated file in the logs/ subdirectory)
	auditDir := workspace.ConfigDir + "/logs"
	entries, err := os.ReadDir(auditDir)
	require.NoError(t, err)
	var hasAuditFile bool
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 6 && e.Name()[:5] == "audit" {
			hasAuditFile = true
			break
		}
	}
	assert.True(t, hasAuditFile, "audit log file should exist in config dir/logs")
}

func TestOrchestratorRun(t *testing.T) {
	// Skip if no real API key is set (since this will call the actual API)
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test that requires real API key")
	}

	// Create temporary workspace
	tmpDir := t.TempDir()

	// Initialize workspace
	workspace, err := config.InitWorkspace(tmpDir)
	require.NoError(t, err)

	// Set API key from environment or use test key
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		workspace.Config.Model.OpenAI.APIKey = key
	} else if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		workspace.Config.Model.Anthropic.APIKey = key
		workspace.Config.Model.DefaultProvider = "anthropic"
	}

	// Create orchestrator
	orch, err := NewOrchestrator(workspace)
	require.NoError(t, err)
	defer orch.Close()

	// Execute a run
	ctx := context.Background()
	result, err := orch.Run(ctx, "test prompt")
	if err != nil {
		// Skip on quota/auth errors — these are API-side, not code bugs
		errStr := err.Error()
		if strings.Contains(errStr, "quota") || strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "401") || strings.Contains(errStr, "insufficient") {
			t.Skipf("Skipping: API error (not a code bug): %v", err)
		}
		require.NoError(t, err)
	}
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.RunID)
	assert.NotEmpty(t, result.Response)

	// Verify run was recorded in session
	session := orch.GetSession()
	assert.Len(t, session.Runs, 1)

	run := session.Runs[0]
	assert.Equal(t, "test prompt", run.Prompt)
	assert.Equal(t, RunCompleted, run.Status)
	assert.NotEmpty(t, run.Result)
}

func TestOrchestratorAuditLog(t *testing.T) {
	// Skip if no real API key is set (since this will call the actual API)
	if os.Getenv("OPENAI_API_KEY") == "" && os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test that requires real API key")
	}

	// Create temporary workspace
	tmpDir := t.TempDir()

	// Initialize workspace
	workspace, err := config.InitWorkspace(tmpDir)
	require.NoError(t, err)

	// Set API key from environment or use test key
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		workspace.Config.Model.OpenAI.APIKey = key
	} else if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		workspace.Config.Model.Anthropic.APIKey = key
		workspace.Config.Model.DefaultProvider = "anthropic"
	}

	// Create orchestrator
	orch, err := NewOrchestrator(workspace)
	require.NoError(t, err)

	// Execute a run
	ctx := context.Background()
	_, err = orch.Run(ctx, "test prompt")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "quota") || strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "401") || strings.Contains(errStr, "insufficient") {
			t.Skipf("Skipping: API error (not a code bug): %v", err)
		}
		require.NoError(t, err)
	}

	// Query audit log for events
	auditLogger := orch.GetAuditLogger()

	// Query all events
	filter := audit.DefaultQueryFilter()
	events, err := auditLogger.Query(ctx, filter)
	require.NoError(t, err)
	assert.NotEmpty(t, events)

	// Should have at least: session.start, run.start, run.complete
	assert.GreaterOrEqual(t, len(events), 3)

	// Close orchestrator to flush logs
	err = orch.Close()
	require.NoError(t, err)
}
