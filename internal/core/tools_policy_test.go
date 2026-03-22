package core

import (
	"encoding/json"
	"testing"

	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetToolSchemasFiltersPermissionedToolsWithoutPolicyApproval(t *testing.T) {
	orch := &Orchestrator{
		policyEngine:    policy.NewEngine(nil),
		integrationsReg: integrations.NewRegistry(),
	}

	tools := orch.getToolSchemas()
	names := toolNameSet(tools)

	assert.False(t, names["exec_command"])
	assert.False(t, names["files_read"])
	assert.False(t, names["web_search"])
	assert.True(t, names["memory_get"])
	assert.True(t, names["switch_model"])
}

func TestGetToolSchemasAllowsLegacyExecRulesForProcessTools(t *testing.T) {
	orch := &Orchestrator{
		policyEngine: policy.NewEngine(&policy.Policy{
			Version: "1",
			Policies: []policy.PolicyRule{
				{
					Name:     "allow-exec",
					Action:   "exec.*",
					Resource: "**",
					Decision: policy.DecisionAllow,
				},
			},
		}),
		integrationsReg: integrations.NewRegistry(),
	}

	tools := orch.getToolSchemas()
	names := toolNameSet(tools)

	assert.True(t, names["exec_command"])
	assert.True(t, names["process_start"])
	assert.True(t, names["cron_list"])
	assert.False(t, names["web_search"])
}

func TestGetToolSchemasKeepsToolsWhenApprovalCallbackExists(t *testing.T) {
	orch := &Orchestrator{
		policyEngine:    policy.NewEngine(nil),
		integrationsReg: integrations.NewRegistry(),
		permissionCallback: func(req PermissionRequest) PermissionResponse {
			return PermissionResponse{Approved: false}
		},
	}

	tools := orch.getToolSchemas()
	names := toolNameSet(tools)

	assert.True(t, names["exec_command"])
	assert.True(t, names["web_search"])
}

func TestGetToolSchemasHidesExplicitlyDeniedToolsEvenWithCallback(t *testing.T) {
	orch := &Orchestrator{
		policyEngine: policy.NewEngine(&policy.Policy{
			Version: "1",
			Policies: []policy.PolicyRule{
				{
					Name:     "deny-exec",
					Action:   "exec.*",
					Resource: "**",
					Decision: policy.DecisionDeny,
				},
			},
		}),
		integrationsReg: integrations.NewRegistry(),
		permissionCallback: func(req PermissionRequest) PermissionResponse {
			return PermissionResponse{Approved: false}
		},
	}

	tools := orch.getToolSchemas()
	names := toolNameSet(tools)

	assert.False(t, names["exec_command"])
	assert.True(t, names["process_start"])
	assert.True(t, names["memory_get"])
}

func TestPermissionChecksForToolCallUsesFineGrainedActions(t *testing.T) {
	orch := &Orchestrator{}

	webCall := model.ToolCall{
		Name:  "web_fetch",
		Input: json.RawMessage(`{"url":"https://example.com"}`),
	}
	webChecks, err := orch.permissionChecksForToolCall(webCall)
	require.NoError(t, err)
	require.Len(t, webChecks, 1)
	assert.Equal(t, "web.fetch", webChecks[0].Action)
	assert.Contains(t, webChecks[0].FallbackActions, "net.request")

	processCall := model.ToolCall{
		Name:  "process_poll",
		Input: json.RawMessage(`{"id":"proc_1"}`),
	}
	processChecks, err := orch.permissionChecksForToolCall(processCall)
	require.NoError(t, err)
	require.Len(t, processChecks, 1)
	assert.Equal(t, "process.poll", processChecks[0].Action)
	assert.Contains(t, processChecks[0].FallbackActions, "exec.command")

	patchCall := model.ToolCall{
		Name:  "apply_patch",
		Input: json.RawMessage(`{"patch":"*** Begin Patch\n*** Add File: notes.txt\n+hello\n*** End Patch"}`),
	}
	patchChecks, err := orch.permissionChecksForToolCall(patchCall)
	require.NoError(t, err)
	require.Len(t, patchChecks, 1)
	assert.Equal(t, "patch.apply", patchChecks[0].Action)
	assert.Contains(t, patchChecks[0].FallbackActions, "files.write")
}

func toolNameSet(tools []model.ToolSchema) map[string]bool {
	out := make(map[string]bool, len(tools))
	for _, tool := range tools {
		out[tool.Name] = true
	}
	return out
}
