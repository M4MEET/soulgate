package policy

import (
	"context"
	"fmt"
	"sort"
)

// Engine evaluates policy decisions
type Engine struct {
	policy  *Policy
	matcher *Matcher
}

// NewEngine creates a new policy engine
func NewEngine(policy *Policy) *Engine {
	return &Engine{
		policy:  policy,
		matcher: NewMatcher(),
	}
}

// Evaluate evaluates a policy request and returns a decision
func (e *Engine) Evaluate(ctx context.Context, req PolicyRequest) (*PolicyResult, error) {
	if e.policy == nil {
		return &PolicyResult{
			Decision: DecisionDeny,
			Reason:   "no policy configured (default deny)",
		}, nil
	}

	// Sort rules by priority (higher priority first)
	rules := e.getSortedRules()

	// Evaluate each rule in priority order
	for _, rule := range rules {
		matches, err := e.ruleMatches(rule, req)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate rule %s: %w", rule.Name, err)
		}

		if matches {
			return &PolicyResult{
				Decision: rule.Decision,
				Rule:     &rule,
				Reason:   fmt.Sprintf("matched rule: %s", rule.Name),
			}, nil
		}
	}

	// Default deny if no rule matched
	return &PolicyResult{
		Decision: DecisionDeny,
		Reason:   "no matching rule (default deny)",
	}, nil
}

// ruleMatches checks if a rule matches a request
func (e *Engine) ruleMatches(rule PolicyRule, req PolicyRequest) (bool, error) {
	// Check action match
	if !e.matcher.MatchAction(rule.Action, req.Action) {
		return false, nil
	}

	// Check resource match
	resourceMatches, err := e.matcher.MatchResource(rule.Resource, req.Resource)
	if err != nil {
		return false, fmt.Errorf("failed to match resource pattern: %w", err)
	}
	if !resourceMatches {
		return false, nil
	}

	// If the rule scopes to a specific role, require the request to carry that role
	if rule.Role != "" && rule.Role != req.Role {
		return false, nil
	}

	// If the rule scopes to a specific agent, require the request to carry that agent ID
	if rule.AgentID != "" && rule.AgentID != req.AgentID {
		return false, nil
	}

	return true, nil
}

// getSortedRules returns rules sorted by priority (highest first)
func (e *Engine) getSortedRules() []PolicyRule {
	rules := make([]PolicyRule, len(e.policy.Policies))
	copy(rules, e.policy.Policies)

	sort.Slice(rules, func(i, j int) bool {
		// Higher priority comes first
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		// If priorities are equal, maintain original order
		return i < j
	})

	return rules
}

// GetPolicy returns the current policy
func (e *Engine) GetPolicy() *Policy {
	return e.policy
}

// AddRule adds a new rule to the policy (in-memory)
func (e *Engine) AddRule(rule PolicyRule) {
	if e.policy == nil {
		e.policy = &Policy{
			Version:  "1",
			Policies: []PolicyRule{},
		}
	}
	e.policy.Policies = append(e.policy.Policies, rule)
}

// RemoveRule removes a rule by name
func (e *Engine) RemoveRule(name string) {
	if e.policy == nil {
		return
	}

	filtered := []PolicyRule{}
	for _, rule := range e.policy.Policies {
		if rule.Name != name {
			filtered = append(filtered, rule)
		}
	}
	e.policy.Policies = filtered
}
