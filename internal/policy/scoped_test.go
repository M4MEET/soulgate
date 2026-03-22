package policy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newRule(name string, scope PolicyScope, scopeID, action, resource string, decision Decision) ScopedRule {
	return ScopedRule{
		PolicyRule: PolicyRule{
			Name:     name,
			Action:   action,
			Resource: resource,
			Decision: decision,
		},
		Scope:   scope,
		ScopeID: scopeID,
	}
}

func newEngine(t *testing.T, rules ...ScopedRule) *ScopedEngine {
	t.Helper()
	e := &ScopedEngine{matcher: NewMatcher()}
	for _, r := range rules {
		require.NoError(t, e.AddRule(r))
	}
	return e
}

// ── ScopedEngine construction ─────────────────────────────────────────────────

func TestNewScopedEngine_MissingFile(t *testing.T) {
	e, err := NewScopedEngine("/nonexistent/scoped_policy.yml")
	require.NoError(t, err, "missing file should not be an error")
	assert.NotNil(t, e)
	assert.Empty(t, e.GetAllRules())
}

func TestNewScopedEngine_LoadYAML(t *testing.T) {
	yaml := `
version: "1"
rules:
  - name: global-deny-exec
    scope: global
    scope_id: ""
    action: "exec.*"
    resource: "**"
    decision: deny
    priority: 50
`
	dir := t.TempDir()
	path := filepath.Join(dir, "scoped.yml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0644))

	e, err := NewScopedEngine(path)
	require.NoError(t, err)
	rules := e.GetAllRules()
	require.Len(t, rules, 1)
	assert.Equal(t, "global-deny-exec", rules[0].Name)
	assert.Equal(t, ScopeGlobal, rules[0].Scope)
}

// ── AddRule validation ────────────────────────────────────────────────────────

func TestAddRule_Valid(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(newRule("r1", ScopeGlobal, "", "files.*", "**", DecisionAllow))
	require.NoError(t, err)
	assert.Len(t, e.GetAllRules(), 1)
}

func TestAddRule_MissingName(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Action: "files.read", Resource: "**", Decision: DecisionAllow},
		Scope:      ScopeGlobal,
	})
	assert.Error(t, err)
}

func TestAddRule_UnknownScope(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Name: "r", Action: "files.read", Resource: "**", Decision: DecisionAllow},
		Scope:      PolicyScope("org"),
		ScopeID:    "acme",
	})
	assert.Error(t, err)
}

func TestAddRule_NonGlobalRequiresScopeID(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Name: "r", Action: "files.read", Resource: "**", Decision: DecisionAllow},
		Scope:      ScopeTeam,
		ScopeID:    "", // missing
	})
	assert.Error(t, err)
}

// ── RemoveRule ────────────────────────────────────────────────────────────────

func TestRemoveRule(t *testing.T) {
	e := newEngine(t,
		newRule("keep", ScopeGlobal, "", "files.read", "**", DecisionAllow),
		newRule("remove", ScopeGlobal, "", "files.write", "**", DecisionDeny),
	)
	require.NoError(t, e.RemoveRule("remove"))
	rules := e.GetAllRules()
	require.Len(t, rules, 1)
	assert.Equal(t, "keep", rules[0].Name)
}

func TestRemoveRule_NotFound_NoError(t *testing.T) {
	e := newEngine(t, newRule("r", ScopeGlobal, "", "files.read", "**", DecisionAllow))
	assert.NoError(t, e.RemoveRule("nonexistent"))
	assert.Len(t, e.GetAllRules(), 1)
}

// ── GetRulesForScope ──────────────────────────────────────────────────────────

func TestGetRulesForScope(t *testing.T) {
	e := newEngine(t,
		newRule("g1", ScopeGlobal, "", "files.read", "**", DecisionAllow),
		newRule("t1", ScopeTeam, "eng", "files.write", "**", DecisionAllow),
		newRule("t2", ScopeTeam, "ops", "files.write", "**", DecisionDeny),
		newRule("u1", ScopeUser, "alice", "exec.*", "**", DecisionDeny),
	)

	global := e.GetRulesForScope(ScopeGlobal, "")
	assert.Len(t, global, 1)
	assert.Equal(t, "g1", global[0].Name)

	eng := e.GetRulesForScope(ScopeTeam, "eng")
	assert.Len(t, eng, 1)
	assert.Equal(t, "t1", eng[0].Name)

	user := e.GetRulesForScope(ScopeUser, "alice")
	assert.Len(t, user, 1)
	assert.Equal(t, "u1", user[0].Name)
}

// ── Basic evaluation ──────────────────────────────────────────────────────────

func TestEvaluate_NoRules_PassThrough(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./test.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.Contains(t, result.Reason, "pass-through")
}

func TestEvaluate_GlobalRule_Deny(t *testing.T) {
	e := newEngine(t, newRule("deny-exec", ScopeGlobal, "", "exec.*", "**", DecisionDeny))
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "exec.command", Resource: "ls"},
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
}

func TestEvaluate_GlobalRule_Allow(t *testing.T) {
	e := newEngine(t, newRule("allow-files", ScopeGlobal, "", "files.*", "./**", DecisionAllow))
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./readme.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
}

// ── Scope cascade / override ──────────────────────────────────────────────────

func TestEvaluate_AgentScopeOverridesGlobal(t *testing.T) {
	// Global: deny exec; Agent "bot-1": allow exec
	e := newEngine(t,
		newRule("global-deny-exec", ScopeGlobal, "", "exec.*", "**", DecisionDeny),
		newRule("agent-allow-exec", ScopeAgent, "bot-1", "exec.*", "**", DecisionAllow),
	)

	// Request from bot-1 should be allowed (agent overrides global).
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "exec.command", Resource: "ls"},
		AgentID:       "bot-1",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.Equal(t, ScopeAgent, result.MatchedScope)
}

func TestEvaluate_UserScopeOverridesTeam(t *testing.T) {
	// Team "eng": allow files.write; User "bob" in "eng": deny files.write
	e := newEngine(t,
		newRule("team-allow-write", ScopeTeam, "eng", "files.write", "**", DecisionAllow),
		newRule("user-deny-write", ScopeUser, "bob", "files.write", "**", DecisionDeny),
	)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.write", Resource: "./report.txt"},
		TeamID:        "eng",
		UserID:        "bob",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
	assert.Equal(t, ScopeUser, result.MatchedScope)
}

func TestEvaluate_TeamScopeMatchesTeamID(t *testing.T) {
	e := newEngine(t, newRule("team-deny-net", ScopeTeam, "finance", "net.*", "**", DecisionDeny))

	// "eng" team should not be affected.
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "net.request", Resource: "https://example.com"},
		TeamID:        "eng",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision, "eng team should pass through")

	// "finance" team should be denied.
	result, err = e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "net.request", Resource: "https://example.com"},
		TeamID:        "finance",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
}

// ── Priority within the same scope ───────────────────────────────────────────

func TestEvaluate_PriorityWithinScope(t *testing.T) {
	e := newEngine(t,
		ScopedRule{
			PolicyRule: PolicyRule{Name: "low-deny", Action: "files.read", Resource: "**", Decision: DecisionDeny, Priority: 1},
			Scope:      ScopeGlobal,
		},
		ScopedRule{
			PolicyRule: PolicyRule{Name: "high-allow", Action: "files.read", Resource: "**", Decision: DecisionAllow, Priority: 10},
			Scope:      ScopeGlobal,
		},
	)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./file.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.Equal(t, "high-allow", result.Rule.Name)
}

// ── Model restriction ─────────────────────────────────────────────────────────

func TestEvaluate_ModelRestriction_Allowed(t *testing.T) {
	r := newRule("team-models", ScopeTeam, "eng", "files.*", "**", DecisionAllow)
	r.ModelRestriction = []string{"gpt-4.1", "claude-3-5-sonnet-20241022"}

	e := newEngine(t, r)
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
		TeamID:        "eng",
		ModelName:     "gpt-4.1",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
}

func TestEvaluate_ModelRestriction_Blocked(t *testing.T) {
	r := newRule("team-models", ScopeTeam, "eng", "files.*", "**", DecisionAllow)
	r.ModelRestriction = []string{"gpt-4.1"}

	e := newEngine(t, r)
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
		TeamID:        "eng",
		ModelName:     "gpt-3.5-turbo",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
}

func TestEvaluate_ModelRestriction_CaseInsensitive(t *testing.T) {
	r := newRule("r", ScopeGlobal, "", "files.*", "**", DecisionAllow)
	r.ModelRestriction = []string{"GPT-4.1"}
	e := newEngine(t, r)
	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
		ModelName:     "gpt-4.1",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
}

// ── Cost limits ───────────────────────────────────────────────────────────────

func TestEvaluate_CostLimit_BelowLimit(t *testing.T) {
	r := newRule("cost-gate", ScopeUser, "alice", "files.*", "**", DecisionAllow)
	r.CostLimit = &CostLimit{MaxPerDay: 10.0, Action: "block"}
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
		UserID:        "alice",
		CostSoFar:     5.0,
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.False(t, result.CostLimitReached)
}

func TestEvaluate_CostLimit_Exceeded(t *testing.T) {
	r := newRule("cost-gate", ScopeUser, "alice", "files.*", "**", DecisionAllow)
	r.CostLimit = &CostLimit{MaxPerDay: 10.0, Action: "block"}
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
		UserID:        "alice",
		CostSoFar:     12.0,
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
	assert.True(t, result.CostLimitReached)
}

func TestEvaluate_CostLimit_WarnActionDoesNotBlock(t *testing.T) {
	r := newRule("cost-warn", ScopeUser, "alice", "files.*", "**", DecisionAllow)
	r.CostLimit = &CostLimit{MaxPerDay: 5.0, Action: "warn"}
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
		UserID:        "alice",
		CostSoFar:     10.0,
	})
	require.NoError(t, err)
	// "warn" action does not produce a Deny decision.
	assert.Equal(t, DecisionAllow, result.Decision)
}

// ── PII detection ─────────────────────────────────────────────────────────────

func TestEvaluate_PIIAction_Block(t *testing.T) {
	r := newRule("pii-block", ScopeGlobal, "", "files.write", "**", DecisionAllow)
	r.PIIAction = "block"
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.write", Resource: "./out.txt"},
		Content:       "Please send invoice to user@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
	assert.True(t, result.PIIDetected)
	assert.NotEmpty(t, result.PIIMatches)
}

func TestEvaluate_PIIAction_Redact(t *testing.T) {
	r := newRule("pii-redact", ScopeGlobal, "", "files.write", "**", DecisionAllow)
	r.PIIAction = "redact"
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.write", Resource: "./out.txt"},
		Content:       "Contact alice@company.org for details",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.True(t, result.PIIDetected)
	assert.Contains(t, result.RedactedContent, "[EMAIL]")
	assert.NotContains(t, result.RedactedContent, "alice@company.org")
}

func TestEvaluate_PIIAction_Allow(t *testing.T) {
	r := newRule("pii-allow", ScopeGlobal, "", "files.write", "**", DecisionAllow)
	r.PIIAction = "allow"
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.write", Resource: "./out.txt"},
		Content:       "Email: user@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.False(t, result.PIIDetected)
}

func TestEvaluate_NoPII_NoRedaction(t *testing.T) {
	r := newRule("pii-redact", ScopeGlobal, "", "files.write", "**", DecisionAllow)
	r.PIIAction = "redact"
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.write", Resource: "./out.txt"},
		Content:       "No sensitive data here",
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.False(t, result.PIIDetected)
	assert.Empty(t, result.RedactedContent)
}

// ── Time restriction ──────────────────────────────────────────────────────────

func TestEvaluate_TimeRestriction_CurrentDayAllowed(t *testing.T) {
	// Build a restriction that always allows today and any hour.
	allDays := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	r := newRule("time-gate", ScopeGlobal, "", "files.*", "**", DecisionAllow)
	r.TimeRestriction = &TimeRestriction{
		AllowedDays:  allDays,
		AllowedHours: "00:00-23:59",
		Timezone:     "UTC",
	}
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionAllow, result.Decision)
	assert.False(t, result.TimeRestrictionViolated)
}

func TestEvaluate_TimeRestriction_InvalidHourWindow_Denied(t *testing.T) {
	// Use a past-midnight window that never overlaps with now.
	// "23:59-00:01" is an overnight window; current time (UTC) is very
	// unlikely to be exactly in that 2-minute window in CI.
	// Instead we set an impossible window: allow only 00:00-00:00 (zero duration).
	r := newRule("time-gate", ScopeGlobal, "", "files.*", "**", DecisionAllow)

	// Use a day list that does NOT include today.
	now := time.Now().UTC()
	todayShort := strings.ToLower(now.Weekday().String()[:3])
	// Pick a day that is definitely not today.
	restrictedDays := []string{}
	for _, d := range []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"} {
		if d != todayShort {
			restrictedDays = append(restrictedDays, d)
		}
	}
	if len(restrictedDays) == 0 {
		t.Skip("could not construct a day restriction that excludes today")
	}

	r.TimeRestriction = &TimeRestriction{
		AllowedDays:  restrictedDays,
		AllowedHours: "00:00-23:59",
		Timezone:     "UTC",
	}
	e := newEngine(t, r)

	result, err := e.Evaluate(context.Background(), ScopedRequest{
		PolicyRequest: PolicyRequest{Action: "files.read", Resource: "./x.txt"},
	})
	require.NoError(t, err)
	assert.Equal(t, DecisionDeny, result.Decision)
	assert.True(t, result.TimeRestrictionViolated)
}

// ── Save / Load round-trip ────────────────────────────────────────────────────

func TestSave_Load_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scoped.yml")

	e1 := &ScopedEngine{path: path, matcher: NewMatcher()}
	require.NoError(t, e1.AddRule(newRule("r1", ScopeGlobal, "", "files.*", "**", DecisionAllow)))
	require.NoError(t, e1.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Name: "r2", Action: "exec.*", Resource: "**", Decision: DecisionDeny},
		Scope:      ScopeTeam, ScopeID: "eng",
	}))
	require.NoError(t, e1.Save())

	e2, err := NewScopedEngine(path)
	require.NoError(t, err)
	rules := e2.GetAllRules()
	require.Len(t, rules, 2)

	names := map[string]bool{}
	for _, r := range rules {
		names[r.Name] = true
	}
	assert.True(t, names["r1"])
	assert.True(t, names["r2"])
}

// ── Validation edge cases ─────────────────────────────────────────────────────

func TestValidateScopedRules_InvalidDecision(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Name: "r", Action: "files.read", Resource: "**", Decision: "maybe"},
		Scope:      ScopeGlobal,
	})
	assert.Error(t, err)
}

func TestValidateScopedRules_InvalidPIIAction(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Name: "r", Action: "files.read", Resource: "**", Decision: DecisionAllow},
		Scope:      ScopeGlobal,
		PIIAction:  "scramble",
	})
	assert.Error(t, err)
}

func TestValidateScopedRules_InvalidCostLimitAction(t *testing.T) {
	e := &ScopedEngine{matcher: NewMatcher()}
	err := e.AddRule(ScopedRule{
		PolicyRule: PolicyRule{Name: "r", Action: "files.read", Resource: "**", Decision: DecisionAllow},
		Scope:      ScopeGlobal,
		CostLimit:  &CostLimit{MaxPerDay: 10, Action: "explode"},
	})
	assert.Error(t, err)
}

func TestValidateTimeRestriction_BadDay(t *testing.T) {
	err := validateTimeRestriction("r", &TimeRestriction{AllowedDays: []string{"funday"}})
	assert.Error(t, err)
}

func TestValidateTimeRestriction_BadHourFormat(t *testing.T) {
	err := validateTimeRestriction("r", &TimeRestriction{AllowedHours: "9am-5pm"})
	assert.Error(t, err)
}

func TestValidateTimeRestriction_BadTimezone(t *testing.T) {
	err := validateTimeRestriction("r", &TimeRestriction{Timezone: "Mars/Olympus"})
	assert.Error(t, err)
}

// ── isWithinTimeWindow helper ─────────────────────────────────────────────────

func TestIsWithinTimeWindow_AllDaysAllHours(t *testing.T) {
	assert.True(t, isWithinTimeWindow(&TimeRestriction{
		AllowedDays:  []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"},
		AllowedHours: "00:00-23:59",
		Timezone:     "UTC",
	}))
}

// ── modelAllowed helper ───────────────────────────────────────────────────────

func TestModelAllowed(t *testing.T) {
	list := []string{"GPT-4.1", "claude-3-5-sonnet-20241022"}
	assert.True(t, modelAllowed(list, "gpt-4.1"))
	assert.True(t, modelAllowed(list, "Claude-3-5-Sonnet-20241022"))
	assert.False(t, modelAllowed(list, "gpt-3.5-turbo"))
}
