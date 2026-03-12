package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadPolicy loads a policy from a YAML file
func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	var policy Policy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}

	// Validate policy
	if err := ValidatePolicy(&policy); err != nil {
		return nil, fmt.Errorf("invalid policy: %w", err)
	}

	return &policy, nil
}

// ValidatePolicy validates a policy structure
func ValidatePolicy(policy *Policy) error {
	if policy.Version == "" {
		return fmt.Errorf("policy version is required")
	}

	if len(policy.Policies) == 0 {
		return fmt.Errorf("policy must contain at least one rule")
	}

	for i, rule := range policy.Policies {
		if rule.Name == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}

		if rule.Action == "" {
			return fmt.Errorf("rule %s: action is required", rule.Name)
		}

		if rule.Resource == "" {
			return fmt.Errorf("rule %s: resource is required", rule.Name)
		}

		if rule.Decision != DecisionAllow && rule.Decision != DecisionDeny && rule.Decision != DecisionRequireApproval {
			return fmt.Errorf("rule %s: decision must be 'allow', 'deny', or 'require_approval'", rule.Name)
		}
	}

	return nil
}

// SavePolicy saves a policy to a YAML file
func SavePolicy(policy *Policy, path string) error {
	data, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to marshal policy: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write policy file: %w", err)
	}

	return nil
}
