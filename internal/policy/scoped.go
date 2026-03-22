// Package policy provides the scoped policy engine that layers hierarchical
// access-control rules on top of the base policy engine.
//
// Scope hierarchy (most-specific wins):
//
//	global → team → user → agent
//
// A rule at a more-specific scope always overrides a rule at a less-specific
// scope, regardless of numeric priority.  Within the same scope the rule with
// the higher Priority value (larger integer) wins.
package policy

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ────────────────────────────────────────────────────────────────────────────
// Scope types
// ────────────────────────────────────────────────────────────────────────────

// PolicyScope identifies the level of a scoped rule.
type PolicyScope string

const (
	// ScopeGlobal rules apply to every request that has no more-specific match.
	ScopeGlobal PolicyScope = "global"

	// ScopeTeam rules apply to all members of a specific team.
	ScopeTeam PolicyScope = "team"

	// ScopeUser rules apply to a single user.
	ScopeUser PolicyScope = "user"

	// ScopeAgent rules apply to a single named agent.
	ScopeAgent PolicyScope = "agent"
)

// scopeSpecificity maps each scope to a numeric weight.  Higher = more
// specific.  A match at a higher weight always overrides a lower one.
var scopeSpecificity = map[PolicyScope]int{
	ScopeGlobal: 0,
	ScopeTeam:   1,
	ScopeUser:   2,
	ScopeAgent:  3,
}

// ────────────────────────────────────────────────────────────────────────────
// Restriction types
// ────────────────────────────────────────────────────────────────────────────

// TimeRestriction limits when a rule may grant access.
type TimeRestriction struct {
	// AllowedDays lists the short day names on which access is permitted, e.g.
	// ["mon","tue","wed","thu","fri"].  An empty slice means every day.
	AllowedDays []string `yaml:"allowed_days" json:"allowed_days"`

	// AllowedHours is an inclusive time window in "HH:MM-HH:MM" 24-hour format,
	// e.g. "09:00-18:00".  An empty string means any hour.
	AllowedHours string `yaml:"allowed_hours" json:"allowed_hours"`

	// Timezone is an IANA timezone name such as "America/New_York".
	// Defaults to UTC when empty or unrecognised.
	Timezone string `yaml:"timezone" json:"timezone"`
}

// CostLimit gates access once a cost threshold is reached.
type CostLimit struct {
	// MaxPerDay is the maximum USD spend allowed per calendar day.  Zero
	// disables the daily limit.
	MaxPerDay float64 `yaml:"max_per_day" json:"max_per_day"`

	// MaxPerMonth is the maximum USD spend allowed in the current calendar
	// month.  Zero disables the monthly limit.
	MaxPerMonth float64 `yaml:"max_per_month" json:"max_per_month"`

	// Action describes what happens when the limit is reached.
	// Supported values: "block" (default), "warn", "notify".
	Action string `yaml:"action" json:"action"`
}

// ────────────────────────────────────────────────────────────────────────────
// ScopedRule
// ────────────────────────────────────────────────────────────────────────────

// ScopedRule extends PolicyRule with scope metadata and additional conditions.
type ScopedRule struct {
	PolicyRule `yaml:",inline"`

	// Scope identifies which hierarchy level this rule belongs to.
	Scope PolicyScope `yaml:"scope" json:"scope"`

	// ScopeID is the identifier for the team, user, or agent that this rule
	// targets.  Empty for ScopeGlobal rules.
	ScopeID string `yaml:"scope_id" json:"scope_id"`

	// TimeRestriction, when set, enforces an allowed-hours window.
	TimeRestriction *TimeRestriction `yaml:"time_restriction,omitempty" json:"time_restriction,omitempty"`

	// CostLimit, when set, blocks or warns when the caller has exceeded a
	// budget threshold.
	CostLimit *CostLimit `yaml:"cost_limit,omitempty" json:"cost_limit,omitempty"`

	// ModelRestriction lists the only model names that may be used under this
	// rule.  An empty slice means any model is permitted.
	ModelRestriction []string `yaml:"model_restriction,omitempty" json:"model_restriction,omitempty"`

	// PIIAction describes how PII detected in the request should be handled.
	// Supported values: "block", "redact", "allow" (default).
	PIIAction string `yaml:"pii_action,omitempty" json:"pii_action,omitempty"`
}

// ────────────────────────────────────────────────────────────────────────────
// ScopedRequest / ScopedResult
// ────────────────────────────────────────────────────────────────────────────

// ScopedRequest extends PolicyRequest with the identity fields required for
// hierarchical evaluation.
type ScopedRequest struct {
	PolicyRequest

	// UserID identifies the human operator making the request.
	UserID string

	// TeamID identifies the team the user belongs to.
	TeamID string

	// AgentID overrides PolicyRequest.AgentID for clarity at the call site.
	AgentID string

	// ModelName is the model the caller intends to use.
	ModelName string

	// CostSoFar is the total USD spent by this caller in the current period;
	// the engine uses this to evaluate CostLimit rules.
	CostSoFar float64

	// Content is the raw text of the request; used for PII detection when a
	// PIIAction rule is in effect.
	Content string
}

// scopedAgentID returns the AgentID to use for scope matching, preferring the
// scoped field over the embedded PolicyRequest one.
func (r ScopedRequest) scopedAgentID() string {
	if r.AgentID != "" {
		return r.AgentID
	}
	return r.PolicyRequest.AgentID
}

// ScopedResult extends PolicyResult with the rule scope and any PII details.
type ScopedResult struct {
	PolicyResult

	// MatchedScope is the scope level of the winning rule (if any).
	MatchedScope PolicyScope

	// PIIDetected is true when PII was found in the request content.
	PIIDetected bool

	// PIIMatches contains the individual PII findings (non-empty when
	// PIIDetected is true and PIIAction is "block" or "redact").
	PIIMatches []PIIMatch

	// RedactedContent is populated when PIIAction is "redact" and PII was
	// found; callers should use this instead of the original content.
	RedactedContent string

	// CostLimitReached is true when a CostLimit caused a "block" decision.
	CostLimitReached bool

	// TimeRestrictionViolated is true when a TimeRestriction prevented access.
	TimeRestrictionViolated bool
}

// ────────────────────────────────────────────────────────────────────────────
// ScopedPolicy — the on-disk YAML document
// ────────────────────────────────────────────────────────────────────────────

// ScopedPolicy is the top-level YAML document loaded from disk.
type ScopedPolicy struct {
	Version string       `yaml:"version"`
	Rules   []ScopedRule `yaml:"rules"`
}

// ────────────────────────────────────────────────────────────────────────────
// ScopedEngine
// ────────────────────────────────────────────────────────────────────────────

// ScopedEngine evaluates hierarchical, multi-scope policy rules.  It is safe
// for concurrent use.  The engine does not replace the base Engine; it is
// consulted as an additional, more-specific layer in the permission flow.
type ScopedEngine struct {
	mu      sync.RWMutex
	rules   []ScopedRule
	path    string
	matcher *Matcher
}

// NewScopedEngine loads scoped rules from the YAML file at path.  If the file
// does not exist the engine starts empty (no rules) and is still usable; rules
// can be added later via AddRule.
func NewScopedEngine(path string) (*ScopedEngine, error) {
	e := &ScopedEngine{
		path:    path,
		matcher: NewMatcher(),
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// No file yet — empty engine, not an error.
		return e, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scoped policy: read %s: %w", path, err)
	}

	var doc ScopedPolicy
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("scoped policy: parse %s: %w", path, err)
	}

	if err := validateScopedRules(doc.Rules); err != nil {
		return nil, fmt.Errorf("scoped policy: validate: %w", err)
	}

	e.rules = doc.Rules
	return e, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Public API
// ────────────────────────────────────────────────────────────────────────────

// Evaluate applies the scoped rule cascade to req and returns a ScopedResult.
//
// Evaluation order:
//  1. Collect every rule whose (Scope, ScopeID, Action, Resource) match req.
//  2. Sort candidates: higher-specificity scope first; within the same scope
//     higher Priority first.
//  3. The first candidate in the sorted list governs the base decision.
//  4. Apply time-restriction, cost-limit, model-restriction, and PII checks
//     against the winning rule.  Any of these can upgrade a "allow" to "deny".
//
// If no rule matches, the engine returns DecisionAllow so that the base
// Engine remains authoritative for non-scoped traffic.  The caller in
// checkOrRequestPermission should chain them: base engine → scoped engine,
// and deny only when either engine denies.
func (e *ScopedEngine) Evaluate(ctx context.Context, req ScopedRequest) (*ScopedResult, error) {
	e.mu.RLock()
	rules := make([]ScopedRule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	// Filter to matching rules.
	var candidates []ScopedRule
	for _, r := range rules {
		ok, err := e.ruleMatches(r, req)
		if err != nil {
			return nil, fmt.Errorf("scoped policy: evaluate rule %q: %w", r.Name, err)
		}
		if ok {
			candidates = append(candidates, r)
		}
	}

	// No scoped rule matched — pass-through (allow).
	if len(candidates) == 0 {
		return &ScopedResult{
			PolicyResult: PolicyResult{
				Decision: DecisionAllow,
				Reason:   "no scoped rule matched (pass-through)",
			},
		}, nil
	}

	// Sort: scope specificity desc, then priority desc.
	sort.Slice(candidates, func(i, j int) bool {
		si := scopeSpecificity[candidates[i].Scope]
		sj := scopeSpecificity[candidates[j].Scope]
		if si != sj {
			return si > sj
		}
		return candidates[i].Priority > candidates[j].Priority
	})

	winning := candidates[0]
	result := &ScopedResult{
		PolicyResult: PolicyResult{
			Decision: winning.Decision,
			Rule:     &winning.PolicyRule,
			Reason:   fmt.Sprintf("matched scoped rule: %s (scope=%s)", winning.Name, winning.Scope),
		},
		MatchedScope: winning.Scope,
	}

	// ── Time restriction ───────────────────────────────────────────────────
	if winning.TimeRestriction != nil && result.Decision == DecisionAllow {
		if !isWithinTimeWindow(winning.TimeRestriction) {
			result.Decision = DecisionDeny
			result.Reason = fmt.Sprintf("rule %q: access denied outside allowed time window", winning.Name)
			result.TimeRestrictionViolated = true
			return result, nil
		}
	}

	// ── Cost limit ─────────────────────────────────────────────────────────
	if winning.CostLimit != nil && result.Decision == DecisionAllow {
		action, exceeded := evaluateCostLimit(winning.CostLimit, req.CostSoFar)
		if exceeded && action == "block" {
			result.Decision = DecisionDeny
			result.Reason = fmt.Sprintf("rule %q: cost limit exceeded (spent $%.4f)", winning.Name, req.CostSoFar)
			result.CostLimitReached = true
			return result, nil
		}
	}

	// ── Model restriction ─────────────────────────────────────────────────
	if len(winning.ModelRestriction) > 0 && req.ModelName != "" && result.Decision == DecisionAllow {
		if !modelAllowed(winning.ModelRestriction, req.ModelName) {
			result.Decision = DecisionDeny
			result.Reason = fmt.Sprintf("rule %q: model %q is not in the allowed list", winning.Name, req.ModelName)
			return result, nil
		}
	}

	// ── PII detection ──────────────────────────────────────────────────────
	if winning.PIIAction != "" && winning.PIIAction != "allow" && req.Content != "" && result.Decision == DecisionAllow {
		matches := DetectPII(req.Content)
		if len(matches) > 0 {
			result.PIIDetected = true
			result.PIIMatches = matches

			switch winning.PIIAction {
			case "block":
				result.Decision = DecisionDeny
				result.Reason = fmt.Sprintf("rule %q: PII detected in request content (%d match(es))", winning.Name, len(matches))
				return result, nil
			case "redact":
				result.RedactedContent = RedactPII(req.Content)
				// Decision stays allow; caller must use RedactedContent.
				result.Reason += " [PII redacted]"
			}
		}
	}

	return result, nil
}

// AddRule appends a rule to the engine's in-memory rule set.  The rule is not
// persisted to disk until Save is called.  Returns an error if the rule fails
// basic validation.
func (e *ScopedEngine) AddRule(rule ScopedRule) error {
	if err := validateScopedRules([]ScopedRule{rule}); err != nil {
		return fmt.Errorf("scoped policy: add rule: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
	return nil
}

// RemoveRule removes all rules with the given name.  It is not an error if no
// rule with that name exists.
func (e *ScopedEngine) RemoveRule(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	filtered := e.rules[:0]
	for _, r := range e.rules {
		if r.Name != name {
			filtered = append(filtered, r)
		}
	}
	e.rules = filtered
	return nil
}

// GetRulesForScope returns a copy of all rules that belong to the given scope
// and ScopeID.  For ScopeGlobal, scopeID is ignored.
func (e *ScopedEngine) GetRulesForScope(scope PolicyScope, scopeID string) []ScopedRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var out []ScopedRule
	for _, r := range e.rules {
		if r.Scope != scope {
			continue
		}
		if scope != ScopeGlobal && r.ScopeID != scopeID {
			continue
		}
		out = append(out, r)
	}
	return out
}

// GetAllRules returns a snapshot of every rule in the engine.
func (e *ScopedEngine) GetAllRules() []ScopedRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]ScopedRule, len(e.rules))
	copy(out, e.rules)
	return out
}

// Save persists the current in-memory rule set to the YAML file that was
// supplied to NewScopedEngine.  If no path was configured the call is a no-op.
func (e *ScopedEngine) Save() error {
	if e.path == "" {
		return nil
	}

	e.mu.RLock()
	rules := make([]ScopedRule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	doc := ScopedPolicy{
		Version: "1",
		Rules:   rules,
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("scoped policy: marshal: %w", err)
	}

	if err := os.WriteFile(e.path, data, 0644); err != nil {
		return fmt.Errorf("scoped policy: write %s: %w", e.path, err)
	}

	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ────────────────────────────────────────────────────────────────────────────

// ruleMatches returns true when the rule's scope/action/resource/identity
// fields all match the request.
func (e *ScopedEngine) ruleMatches(rule ScopedRule, req ScopedRequest) (bool, error) {
	// ── Scope identity checks ──────────────────────────────────────────────
	switch rule.Scope {
	case ScopeGlobal:
		// Global rules match everyone.
	case ScopeTeam:
		if rule.ScopeID != "" && rule.ScopeID != req.TeamID {
			return false, nil
		}
	case ScopeUser:
		if rule.ScopeID != "" && rule.ScopeID != req.UserID {
			return false, nil
		}
	case ScopeAgent:
		agentID := req.scopedAgentID()
		if rule.ScopeID != "" && rule.ScopeID != agentID {
			return false, nil
		}
	default:
		// Unknown scope — skip rule.
		return false, nil
	}

	// ── Action match ───────────────────────────────────────────────────────
	if !e.matcher.MatchAction(rule.Action, req.Action) {
		return false, nil
	}

	// ── Resource match ─────────────────────────────────────────────────────
	resourceMatches, err := e.matcher.MatchResource(rule.Resource, req.Resource)
	if err != nil {
		return false, fmt.Errorf("resource pattern %q: %w", rule.Resource, err)
	}
	if !resourceMatches {
		return false, nil
	}

	// ── Role / agent ID from the embedded PolicyRule ───────────────────────
	if rule.PolicyRule.Role != "" && rule.PolicyRule.Role != req.Role {
		return false, nil
	}
	if rule.PolicyRule.AgentID != "" && rule.PolicyRule.AgentID != req.scopedAgentID() {
		return false, nil
	}

	return true, nil
}

// validateScopedRules checks that each rule has the required fields and a
// valid scope value.
func validateScopedRules(rules []ScopedRule) error {
	for i, r := range rules {
		if r.Name == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}
		if r.Action == "" {
			return fmt.Errorf("rule %q: action is required", r.Name)
		}
		if r.Resource == "" {
			return fmt.Errorf("rule %q: resource is required", r.Name)
		}
		if r.Decision != DecisionAllow && r.Decision != DecisionDeny && r.Decision != DecisionRequireApproval {
			return fmt.Errorf("rule %q: decision must be allow, deny, or require_approval", r.Name)
		}
		switch r.Scope {
		case ScopeGlobal, ScopeTeam, ScopeUser, ScopeAgent:
			// valid
		case "":
			return fmt.Errorf("rule %q: scope is required", r.Name)
		default:
			return fmt.Errorf("rule %q: unknown scope %q", r.Name, r.Scope)
		}
		if r.Scope != ScopeGlobal && r.ScopeID == "" {
			return fmt.Errorf("rule %q: scope_id is required for scope %q", r.Name, r.Scope)
		}
		if r.TimeRestriction != nil {
			if err := validateTimeRestriction(r.Name, r.TimeRestriction); err != nil {
				return err
			}
		}
		if r.PIIAction != "" {
			switch r.PIIAction {
			case "block", "redact", "allow":
				// valid
			default:
				return fmt.Errorf("rule %q: pii_action must be block, redact, or allow", r.Name)
			}
		}
		if r.CostLimit != nil {
			action := r.CostLimit.Action
			if action != "" && action != "block" && action != "warn" && action != "notify" {
				return fmt.Errorf("rule %q: cost_limit.action must be block, warn, or notify", r.Name)
			}
		}
	}
	return nil
}

// validateTimeRestriction performs shallow validation of a TimeRestriction.
func validateTimeRestriction(ruleName string, tr *TimeRestriction) error {
	validDays := map[string]bool{
		"mon": true, "tue": true, "wed": true, "thu": true,
		"fri": true, "sat": true, "sun": true,
	}
	for _, d := range tr.AllowedDays {
		if !validDays[strings.ToLower(d)] {
			return fmt.Errorf("rule %q: invalid day %q in time_restriction.allowed_days", ruleName, d)
		}
	}

	if tr.AllowedHours != "" {
		parts := strings.SplitN(tr.AllowedHours, "-", 2)
		if len(parts) != 2 {
			return fmt.Errorf("rule %q: time_restriction.allowed_hours must be HH:MM-HH:MM", ruleName)
		}
		if _, err := time.Parse("15:04", parts[0]); err != nil {
			return fmt.Errorf("rule %q: time_restriction.allowed_hours start time invalid: %w", ruleName, err)
		}
		if _, err := time.Parse("15:04", parts[1]); err != nil {
			return fmt.Errorf("rule %q: time_restriction.allowed_hours end time invalid: %w", ruleName, err)
		}
	}

	if tr.Timezone != "" {
		if _, err := time.LoadLocation(tr.Timezone); err != nil {
			return fmt.Errorf("rule %q: time_restriction.timezone %q unrecognised: %w", ruleName, tr.Timezone, err)
		}
	}

	return nil
}

// isWithinTimeWindow returns true when the current wall-clock time satisfies
// the restrictions specified in tr.
func isWithinTimeWindow(tr *TimeRestriction) bool {
	loc := time.UTC
	if tr.Timezone != "" {
		if l, err := time.LoadLocation(tr.Timezone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)

	// Day check.
	if len(tr.AllowedDays) > 0 {
		weekday := strings.ToLower(now.Weekday().String()[:3])
		allowed := false
		for _, d := range tr.AllowedDays {
			if strings.ToLower(d) == weekday {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Hour window check.
	if tr.AllowedHours != "" {
		parts := strings.SplitN(tr.AllowedHours, "-", 2)
		if len(parts) == 2 {
			startT, err1 := time.Parse("15:04", parts[0])
			endT, err2 := time.Parse("15:04", parts[1])
			if err1 == nil && err2 == nil {
				nowMins := now.Hour()*60 + now.Minute()
				startMins := startT.Hour()*60 + startT.Minute()
				endMins := endT.Hour()*60 + endT.Minute()

				if startMins <= endMins {
					// Normal window: 09:00-18:00
					if nowMins < startMins || nowMins > endMins {
						return false
					}
				} else {
					// Overnight window: 22:00-06:00
					if nowMins < startMins && nowMins > endMins {
						return false
					}
				}
			}
		}
	}

	return true
}

// evaluateCostLimit returns (action, exceeded).  action defaults to "block"
// when CostLimit.Action is empty.
func evaluateCostLimit(cl *CostLimit, costSoFar float64) (string, bool) {
	action := cl.Action
	if action == "" {
		action = "block"
	}

	if cl.MaxPerDay > 0 && costSoFar >= cl.MaxPerDay {
		return action, true
	}
	if cl.MaxPerMonth > 0 && costSoFar >= cl.MaxPerMonth {
		return action, true
	}

	return action, false
}

// modelAllowed returns true when modelName is in the restriction list.
// The comparison is case-insensitive.
func modelAllowed(restriction []string, modelName string) bool {
	lower := strings.ToLower(modelName)
	for _, allowed := range restriction {
		if strings.ToLower(allowed) == lower {
			return true
		}
	}
	return false
}
