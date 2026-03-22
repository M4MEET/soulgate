package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers/exec"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	brokersnet "github.com/M4MEET/soulgate/internal/brokers/net"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/M4MEET/soulgate/internal/tools/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildDispatchOrchestrator creates a lightweight Orchestrator suitable for
// dispatch routing tests. It uses a permissive allow-all policy so that
// broker-level policy checks do not block execution.
func buildDispatchOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".soulgate")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	auditLogger, err := audit.NewJSONLLogger(filepath.Join(configDir, "audit.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = auditLogger.Close() })

	// Permissive policy: allow every action on every resource.
	allowAllPolicy := &policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "allow-all",
				Action:   "*",
				Resource: "**",
				Decision: policy.DecisionAllow,
				Priority: 1,
			},
		},
	}
	policyEngine := policy.NewEngine(allowAllPolicy)

	fileBroker, err := files.NewBroker(tmpDir, policyEngine, auditLogger)
	require.NoError(t, err)

	execBroker, err := exec.NewBroker(tmpDir, policyEngine, auditLogger)
	require.NoError(t, err)

	netBroker, err := brokersnet.NewBroker(policyEngine, auditLogger)
	require.NoError(t, err)

	memStore, err := NewMemoryStore(configDir)
	require.NoError(t, err)

	cfg := config.DefaultConfig()
	cfg.Workspace.Root = tmpDir
	cfg.Workspace.ConfigDir = configDir

	return &Orchestrator{
		workspace: &config.Workspace{
			Root:      tmpDir,
			ConfigDir: configDir,
			Config:    cfg,
		},
		audit:           auditLogger,
		session:         NewSession(tmpDir),
		policyEngine:    policyEngine,
		fileBroker:      fileBroker,
		execBroker:      execBroker,
		netBroker:       netBroker,
		integrationsReg: integrations.NewRegistry(),
		toolRegistry:    NewToolRegistry(),
		memoryStore:     memStore,
		agentManager:    NewAgentManager(),
		processManager:  process.NewManagerWithWorkspace(tmpDir),
		directives:      DefaultDirectives(),
		loopDetector:    NewLoopDetector(),
	}
}

// TestExecuteToolCallRouting_DoesNotPanicForKnownTools verifies that
// executeToolCall routes each known tool to the correct handler without
// panicking. Tools that need live external services (net_request, web_search,
// etc.) are expected to return an error — we only verify the absence of panics.
func TestExecuteToolCallRouting_DoesNotPanicForKnownTools(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    json.RawMessage
	}{
		{
			name:     "files_read reaches file broker",
			toolName: "files_read",
			input:    json.RawMessage(`{"path":"./nonexistent.txt"}`),
		},
		{
			name:     "files_list reaches file broker",
			toolName: "files_list",
			input:    json.RawMessage(`{"path":"."}`),
		},
		{
			name:     "files_write reaches file broker",
			toolName: "files_write",
			input:    json.RawMessage(`{"path":"./out.txt","content":"hello"}`),
		},
		{
			name:     "memory_write is dispatched without panic",
			toolName: "memory_write",
			input:    json.RawMessage(`{"key":"k","value":"v"}`),
		},
		{
			name:     "memory_get is dispatched without panic",
			toolName: "memory_get",
			input:    json.RawMessage(`{"key":"k"}`),
		},
		{
			name:     "switch_model dispatched without panic",
			toolName: "switch_model",
			input:    json.RawMessage(`{"model":"gpt-4.1","reason":"test"}`),
		},
		{
			name:     "soulgate_introspect dispatched without panic",
			toolName: "soulgate_introspect",
			input:    json.RawMessage(`{"section":"status"}`),
		},
		{
			name:     "soulgate_configure dispatched without panic",
			toolName: "soulgate_configure",
			input:    json.RawMessage(`{"action":"unknown_action_for_test"}`),
		},
		{
			name:     "agent_list dispatched without panic",
			toolName: "agent_list",
			input:    json.RawMessage(`{}`),
		},
		{
			name:     "apply_patch dispatched without panic",
			toolName: "apply_patch",
			input:    json.RawMessage(`{"patch":"*** Begin Patch\n*** Add File: testpatch.txt\n+line\n*** End Patch"}`),
		},
		{
			name:     "process_list dispatched without panic",
			toolName: "process_list",
			input:    json.RawMessage(`{}`),
		},
	}

	orch := buildDispatchOrchestrator(t)
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCall := model.ToolCall{
				ID:    "tc_" + tt.toolName,
				Name:  tt.toolName,
				Input: tt.input,
			}
			assert.NotPanics(t, func() {
				_, _ = orch.executeToolCall(ctx, "run-test", toolCall)
			})
		})
	}
}

// TestExecuteToolCallRouting_FilesWriteThenRead verifies the round-trip:
// files_write followed by files_read returns the written content.
func TestExecuteToolCallRouting_FilesWriteThenRead(t *testing.T) {
	orch := buildDispatchOrchestrator(t)
	ctx := context.Background()

	writeCall := model.ToolCall{
		ID:    "tc_write",
		Name:  "files_write",
		Input: json.RawMessage(`{"path":"./hello.txt","content":"dispatch test content"}`),
	}
	_, err := orch.executeToolCall(ctx, "run-write", writeCall)
	require.NoError(t, err)

	readCall := model.ToolCall{
		ID:    "tc_read",
		Name:  "files_read",
		Input: json.RawMessage(`{"path":"./hello.txt"}`),
	}
	result, err := orch.executeToolCall(ctx, "run-read", readCall)
	require.NoError(t, err)
	assert.Equal(t, "dispatch test content", result)
}

// TestExecuteToolCallRouting_FilesListReturnsString verifies that files_list
// returns a string result (the listing) without error on the workspace root.
func TestExecuteToolCallRouting_FilesListReturnsString(t *testing.T) {
	orch := buildDispatchOrchestrator(t)
	ctx := context.Background()

	toolCall := model.ToolCall{
		ID:    "tc_list",
		Name:  "files_list",
		Input: json.RawMessage(`{"path":"."}`),
	}

	result, err := orch.executeToolCall(ctx, "run-list", toolCall)
	require.NoError(t, err)
	assert.IsType(t, "", result)
}

// TestExecuteToolCallRouting_UnknownToolReturnsError verifies that an unknown
// tool name (not in the registry, not MCP-namespaced, not an integration)
// returns an error rather than panicking.
func TestExecuteToolCallRouting_UnknownToolReturnsError(t *testing.T) {
	orch := buildDispatchOrchestrator(t)
	ctx := context.Background()

	toolCall := model.ToolCall{
		ID:    "tc_unknown",
		Name:  "this_tool_does_not_exist_anywhere",
		Input: json.RawMessage(`{}`),
	}

	_, err := orch.executeToolCall(ctx, "run-unknown", toolCall)
	assert.Error(t, err)
}
