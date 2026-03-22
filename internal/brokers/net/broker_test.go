package net

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAuditLogger(t *testing.T) audit.Logger {
	t.Helper()
	auditPath := filepath.Join(t.TempDir(), "audit.jsonl")
	logger, err := audit.NewJSONLLogger(auditPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = logger.Close()
	})
	return logger
}

func TestRequestPolicyDeny(t *testing.T) {
	engine := policy.NewEngine(&policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "deny-net",
				Action:   "net.*",
				Resource: "**",
				Decision: policy.DecisionDeny,
				Priority: 100,
			},
		},
	})

	broker, err := NewBroker(engine, newTestAuditLogger(t))
	require.NoError(t, err)

	_, err = broker.Request(context.Background(), brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}, "GET", "https://example.com", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied by policy")
}

func TestRequestNoPolicyEngine(t *testing.T) {
	broker, err := NewBroker(nil, newTestAuditLogger(t))
	require.NoError(t, err)

	_, err = broker.Request(context.Background(), brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}, "GET", "https://example.com", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy engine not configured")
}

func TestRequestBlocksPrivateIPTargets(t *testing.T) {
	engine := policy.NewEngine(&policy.Policy{
		Version: "1",
		Policies: []policy.PolicyRule{
			{
				Name:     "allow-net",
				Action:   "net.*",
				Resource: "**",
				Decision: policy.DecisionAllow,
				Priority: 10,
			},
		},
	})

	broker, err := NewBroker(engine, newTestAuditLogger(t))
	require.NoError(t, err)

	_, err = broker.Request(context.Background(), brokers.BrokerContext{
		PluginID: "test-plugin",
		RunID:    "test-run",
	}, "GET", "http://127.0.0.1:12345", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private IP")
}
