package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyEngineBasic(t *testing.T) {
	policy := &Policy{
		Version: "1",
		Policies: []PolicyRule{
			{
				Name:     "allow-workspace-reads",
				Action:   "files.read",
				Resource: "./**",
				Decision: DecisionAllow,
			},
			{
				Name:     "deny-parent-reads",
				Action:   "files.read",
				Resource: "../**",
				Decision: DecisionDeny,
			},
		},
	}

	engine := NewEngine(policy)

	testCases := []struct {
		name     string
		request  PolicyRequest
		expected Decision
	}{
		{
			name: "allow workspace read",
			request: PolicyRequest{
				Action:   "files.read",
				Resource: "./test.txt",
			},
			expected: DecisionAllow,
		},
		{
			name: "deny parent read",
			request: PolicyRequest{
				Action:   "files.read",
				Resource: "../secret.txt",
			},
			expected: DecisionDeny,
		},
		{
			name: "deny unmached action",
			request: PolicyRequest{
				Action:   "files.write",
				Resource: "./test.txt",
			},
			expected: DecisionDeny,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Evaluate(context.Background(), tc.request)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result.Decision)
		})
	}
}

func TestPolicyEnginePriority(t *testing.T) {
	policy := &Policy{
		Version: "1",
		Policies: []PolicyRule{
			{
				Name:     "low-priority-allow",
				Action:   "files.read",
				Resource: "**",
				Decision: DecisionAllow,
				Priority: 1,
			},
			{
				Name:     "high-priority-deny",
				Action:   "files.read",
				Resource: "secret/**",
				Decision: DecisionDeny,
				Priority: 10,
			},
		},
	}

	engine := NewEngine(policy)

	// High priority deny should override low priority allow
	result, err := engine.Evaluate(context.Background(), PolicyRequest{
		Action:   "files.read",
		Resource: "secret/key.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
	assert.Equal(t, "high-priority-deny", result.Rule.Name)
}

func TestPolicyEngineWildcard(t *testing.T) {
	policy := &Policy{
		Version: "1",
		Policies: []PolicyRule{
			{
				Name:     "allow-all-file-ops",
				Action:   "files.*",
				Resource: "./**",
				Decision: DecisionAllow,
			},
		},
	}

	engine := NewEngine(policy)

	actions := []string{"files.read", "files.write", "files.list", "files.stat"}

	for _, action := range actions {
		result, err := engine.Evaluate(context.Background(), PolicyRequest{
			Action:   action,
			Resource: "./test.txt",
		})
		require.NoError(t, err)
		assert.Equal(t, DecisionAllow, result.Decision, "action %s should be allowed", action)
	}
}

func TestPolicyEngineDefaultDeny(t *testing.T) {
	policy := &Policy{
		Version: "1",
		Policies: []PolicyRule{
			{
				Name:     "allow-specific",
				Action:   "files.read",
				Resource: "./allowed.txt",
				Decision: DecisionAllow,
			},
		},
	}

	engine := NewEngine(policy)

	// Request that doesn't match any rule should be denied
	result, err := engine.Evaluate(context.Background(), PolicyRequest{
		Action:   "files.read",
		Resource: "./other.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
	assert.Contains(t, result.Reason, "default deny")
}
