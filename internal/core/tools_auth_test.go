package core

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/M4MEET/soulgate/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionChecksForToolCall_BrokerManaged verifies that broker-managed tools
// (files_read, files_write, exec_command, net_request) return nil checks because
// the broker enforces policy internally.
func TestPermissionChecksForToolCall_BrokerManaged(t *testing.T) {
	brokerManagedTools := []string{
		"files_read",
		"files_write",
		"files_list",
		"files_delete",
		"exec_command",
		"net_request",
	}

	orch := &Orchestrator{}
	for _, toolName := range brokerManagedTools {
		t.Run(toolName, func(t *testing.T) {
			toolCall := model.ToolCall{
				ID:    "call_1",
				Name:  toolName,
				Input: json.RawMessage(`{"path":"./test.txt"}`),
			}
			checks, err := orch.permissionChecksForToolCall(toolCall)
			require.NoError(t, err)
			assert.Nil(t, checks, "broker-managed tool %q must return nil checks", toolName)
		})
	}
}

// TestPermissionChecksForToolCall_WebSearch verifies that web_search produces a
// check with action=web.search, the query as resource, and net.request fallback.
func TestPermissionChecksForToolCall_WebSearch(t *testing.T) {
	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_ws",
		Name:  "web_search",
		Input: json.RawMessage(`{"query":"golang testing"}`),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "web.search", checks[0].Action)
	assert.Equal(t, "query:golang testing", checks[0].Resource)
	assert.Contains(t, checks[0].FallbackActions, "net.request")
}

// TestPermissionChecksForToolCall_WebSearchEmpty verifies that an empty query
// returns an error instead of a permission check.
func TestPermissionChecksForToolCall_WebSearchEmpty(t *testing.T) {
	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_ws_empty",
		Name:  "web_search",
		Input: json.RawMessage(`{"query":"   "}`),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	assert.Error(t, err)
	assert.Nil(t, checks)
}

// TestPermissionChecksForToolCall_ProcessStart verifies that process_start
// produces a check with action=process.start and exec.command fallback.
func TestPermissionChecksForToolCall_ProcessStart(t *testing.T) {
	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_ps",
		Name:  "process_start",
		Input: json.RawMessage(`{"command":"npm start"}`),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "process.start", checks[0].Action)
	assert.Equal(t, "npm start", checks[0].Resource)
	assert.Contains(t, checks[0].FallbackActions, "exec.command")
}

// TestPermissionChecksForToolCall_ProcessStartEmpty verifies that an empty
// command returns an error.
func TestPermissionChecksForToolCall_ProcessStartEmpty(t *testing.T) {
	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_ps_empty",
		Name:  "process_start",
		Input: json.RawMessage(`{"command":""}`),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	assert.Error(t, err)
	assert.Nil(t, checks)
}

// TestPermissionChecksForToolCall_NoPermission verifies that tools flagged as
// NoPermissionRequired (memory_write, switch_model, agent_create) return nil checks.
func TestPermissionChecksForToolCall_NoPermission(t *testing.T) {
	noPermTools := []string{
		"memory_write",
		"memory_get",
		"memory_search",
		"switch_model",
		"agent_create",
		"agent_list",
		"agent_stop",
		"soulgate_introspect",
		"soulgate_configure",
		"llm_task",
	}

	orch := &Orchestrator{}
	for _, toolName := range noPermTools {
		t.Run(toolName, func(t *testing.T) {
			toolCall := model.ToolCall{
				ID:    "call_np",
				Name:  toolName,
				Input: json.RawMessage(`{}`),
			}
			checks, err := orch.permissionChecksForToolCall(toolCall)
			require.NoError(t, err)
			assert.Nil(t, checks, "no-permission tool %q must return nil checks", toolName)
		})
	}
}

// TestPermissionChecksForToolCall_ApplyPatch verifies that a multi-file patch
// produces one permissionCheck per affected file.
func TestPermissionChecksForToolCall_ApplyPatch(t *testing.T) {
	patch := "*** Begin Patch\n*** Add File: foo.txt\n+hello\n*** Update File: bar.txt\n-old\n+new\n*** End Patch"
	input, _ := json.Marshal(map[string]string{"patch": patch})

	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_patch",
		Name:  "apply_patch",
		Input: json.RawMessage(input),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	require.NoError(t, err)
	require.Len(t, checks, 2, "one check per file directive")

	for _, c := range checks {
		assert.Equal(t, "patch.apply", c.Action)
		assert.NotEmpty(t, c.Resource)
		assert.NotEmpty(t, c.FallbackActions)
	}
}

// TestPermissionChecksForToolCall_MCPTool verifies that a tool name in the
// server__tool MCP namespace produces an mcp.tool_call check.
func TestPermissionChecksForToolCall_MCPTool(t *testing.T) {
	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_mcp",
		Name:  "myserver__do_something",
		Input: json.RawMessage(`{"key":"value"}`),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "mcp.tool_call", checks[0].Action)
	assert.Equal(t, "myserver__do_something", checks[0].Resource)
}

// TestPermissionChecksForToolCall_IntegrationTool verifies that an unknown tool
// name (not in registry, not MCP-namespaced) produces an integration.call check
// with net.request as a fallback.
func TestPermissionChecksForToolCall_IntegrationTool(t *testing.T) {
	orch := &Orchestrator{}
	toolCall := model.ToolCall{
		ID:    "call_int",
		Name:  "some_custom_integration_tool",
		Input: json.RawMessage(`{}`),
	}

	checks, err := orch.permissionChecksForToolCall(toolCall)
	require.NoError(t, err)
	require.Len(t, checks, 1)
	assert.Equal(t, "integration.call", checks[0].Action)
	assert.Equal(t, "integration:some_custom_integration_tool", checks[0].Resource)
	assert.Contains(t, checks[0].FallbackActions, "net.request")
}

// TestPermissionRegistryCompleteness iterates over every entry in toolPermissionDefs
// and calls both runtimeChecksFromRegistry and schemaChecksFromRegistry to ensure
// neither panics and that each function returns a result consistent with the definition.
func TestPermissionRegistryCompleteness(t *testing.T) {
	for toolName, def := range toolPermissionDefs {
		toolName, def := toolName, def // capture
		t.Run(toolName, func(t *testing.T) {
			// Build a plausible input for tools that need ResourceFromInput.
			input := buildTestInput(toolName)

			toolCall := model.ToolCall{
				ID:    fmt.Sprintf("call_%s", toolName),
				Name:  toolName,
				Input: json.RawMessage(input),
			}

			// runtimeChecksFromRegistry must not panic and must be consistent.
			checks, err := runtimeChecksFromRegistry(toolCall)
			if def.BrokerManaged || def.NoPermissionRequired {
				require.NoError(t, err)
				assert.Nil(t, checks, "broker-managed/no-permission tool must return nil runtime checks")
			} else {
				// May return an error only for tools that require non-empty fields
				// (e.g., apply_patch with no directives). For our valid test inputs
				// it should succeed.
				if err == nil {
					assert.NotNil(t, checks, "non-broker, non-no-permission tool must return checks")
				}
			}

			// schemaChecksFromRegistry must not panic.
			schemaChecks := schemaChecksFromRegistry(toolName)
			if def.NoPermissionRequired {
				assert.Nil(t, schemaChecks, "no-permission tool must return nil schema checks")
			} else {
				assert.NotNil(t, schemaChecks, "permissioned tool must return schema checks")
			}
		})
	}
}

// buildTestInput returns a JSON input string that satisfies each tool's required fields.
func buildTestInput(toolName string) string {
	switch toolName {
	case "web_search":
		return `{"query":"test query"}`
	case "web_fetch":
		return `{"url":"https://example.com"}`
	case "process_start":
		return `{"command":"echo hello"}`
	case "process_poll", "process_log", "process_write", "process_kill":
		return `{"id":"proc_abc"}`
	case "pdf_read":
		return `{"path":"./document.pdf"}`
	case "apply_patch":
		return `{"patch":"*** Begin Patch\n*** Add File: test.txt\n+line\n*** End Patch"}`
	default:
		return `{}`
	}
}
