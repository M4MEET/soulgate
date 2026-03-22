package core

import (
	"context"
	"testing"
	"time"

	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/stretchr/testify/assert"
)

func TestNormalizePolicyAction(t *testing.T) {
	tests := map[string]string{
		"files_read":   "files.read",
		"files_write":  "files.write",
		"files_list":   "files.list",
		"files_delete": "files.delete",
		"files_stat":   "files.stat",
		"exec_command": "exec.command",
		"net_request":  "net.request",
		"files.read":   "files.read",
	}

	for in, want := range tests {
		assert.Equal(t, want, normalizePolicyAction(in))
	}
}

func TestCheckOrRequestPermissionSupportsLegacyActionNames(t *testing.T) {
	orch := &Orchestrator{
		policyEngine: policy.NewEngine(&policy.Policy{
			Version: "1",
			Policies: []policy.PolicyRule{
				{
					Name:     "allow-read",
					Action:   "files.read",
					Resource: "./**",
					Decision: policy.DecisionAllow,
				},
			},
		}),
	}

	allowed, reason := orch.checkOrRequestPermission(context.Background(), "files_read", "./test.txt")
	assert.True(t, allowed)
	assert.Empty(t, reason)
}

func TestCheckOrRequestPermissionRespectsTrustModeExpiry(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	orch := &Orchestrator{
		policyEngine: policy.NewEngine(&policy.Policy{
			Version: "1",
			Policies: []policy.PolicyRule{
				{
					Name:     "deny-all",
					Action:   "*",
					Resource: "**",
					Decision: policy.DecisionDeny,
				},
			},
		}),
		trustMode:       true,
		trustModeExpiry: &past,
	}

	allowed, _ := orch.checkOrRequestPermission(context.Background(), "files.read", "./test.txt")
	assert.False(t, allowed)
	assert.False(t, orch.trustMode, "expired trust mode should be disabled during permission checks")
}

func TestCheckOrRequestPermissionWithFallbackSupportsLegacyPolicies(t *testing.T) {
	orch := &Orchestrator{
		policyEngine: policy.NewEngine(&policy.Policy{
			Version: "1",
			Policies: []policy.PolicyRule{
				{
					Name:     "allow-net",
					Action:   "net.request",
					Resource: "**",
					Decision: policy.DecisionAllow,
				},
			},
		}),
	}

	allowed, usedAction, reason := orch.checkOrRequestPermissionWithFallback(
		context.Background(),
		"web.search",
		[]string{"net.request"},
		"query:golang",
	)
	assert.True(t, allowed)
	assert.Equal(t, "net.request", usedAction)
	assert.Empty(t, reason)
}

func TestSetTrustModeBypassesPolicyEngineEvaluations(t *testing.T) {
	orch := &Orchestrator{
		policyEngine: policy.NewEngine(&policy.Policy{
			Version: "1",
			Policies: []policy.PolicyRule{
				{
					Name:     "deny-all",
					Action:   "*",
					Resource: "**",
					Decision: policy.DecisionDeny,
				},
			},
		}),
	}

	orch.SetTrustMode(true)
	allowedResult, err := orch.policyEngine.Evaluate(context.Background(), policy.PolicyRequest{
		Action:   "files.read",
		Resource: "./foo.txt",
	})
	assert.NoError(t, err)
	assert.Equal(t, policy.DecisionAllow, allowedResult.Decision)

	orch.SetTrustMode(false)
	deniedResult, err := orch.policyEngine.Evaluate(context.Background(), policy.PolicyRequest{
		Action:   "files.read",
		Resource: "./foo.txt",
	})
	assert.NoError(t, err)
	assert.Equal(t, policy.DecisionDeny, deniedResult.Decision)
}
