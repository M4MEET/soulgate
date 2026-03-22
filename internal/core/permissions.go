package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/policy"
)

// PermissionRequest represents a request for user permission
type PermissionRequest struct {
	Action      string // e.g., "files.list", "exec.command"
	Resource    string // e.g., "/Users/demon", "git status"
	Description string // Human-readable description
	Reason      string // Why it was denied
}

// PermissionResponse represents the user's decision
type PermissionResponse struct {
	Approved     bool
	LearnPattern bool // If true, add to policy permanently
}

// PermissionCallback is called when permission is needed
type PermissionCallback func(req PermissionRequest) PermissionResponse

// GenerateSmartPattern generates an intelligent pattern from a specific resource
func GenerateSmartPattern(action string, resource string) string {
	action = normalizePolicyAction(action)

	// For file operations, generalize to directory level
	if strings.HasPrefix(action, "files.") {
		// If it's a specific file, allow the whole directory
		if !strings.HasSuffix(resource, "/") {
			dir := filepath.Dir(resource)
			return dir + "/**"
		}
		// If it's already a directory, use it
		return resource + "**"
	}

	// For exec commands, generalize the command
	if action == "exec.command" {
		// Extract command name
		parts := strings.Fields(resource)
		if len(parts) > 0 {
			// Allow all uses of this command
			return parts[0] + "*"
		}
	}

	// For network requests, generalize domain
	if action == "net.request" {
		// Extract domain from URL
		if strings.HasPrefix(resource, "http") {
			parts := strings.Split(resource, "/")
			if len(parts) >= 3 {
				// Allow all requests to this domain
				return parts[0] + "//" + parts[2] + "/**"
			}
		}
	}

	// Default: exact match
	return resource
}

// CreateLearnedRule creates a policy rule from a permission approval
func CreateLearnedRule(action string, resource string) policy.PolicyRule {
	action = normalizePolicyAction(action)
	pattern := GenerateSmartPattern(action, resource)

	// Convert action to policy format
	policyAction := action
	if strings.HasPrefix(action, "files.") {
		policyAction = "files.*" // Allow all file operations on this resource
	} else if action == "exec.command" {
		policyAction = "exec.*"
	} else if action == "net.request" {
		policyAction = "net.*"
	}

	// Generate descriptive name
	timestamp := time.Now().Format("20060102-150405")
	ruleName := fmt.Sprintf("auto-learned-%s-%s", action, timestamp)

	return policy.PolicyRule{
		Name:        ruleName,
		Description: fmt.Sprintf("Auto-learned from user approval on %s", time.Now().Format("2006-01-02 15:04:05")),
		Action:      policyAction,
		Resource:    pattern,
		Decision:    policy.DecisionAllow,
		Priority:    100, // High priority so learned rules take precedence
	}
}

// FormatPermissionDescription creates a human-readable description
func FormatPermissionDescription(action string, resource string) string {
	action = normalizePolicyAction(action)

	switch action {
	case "files.read":
		return fmt.Sprintf("Read file: %s", resource)
	case "files.write":
		return fmt.Sprintf("Write file: %s", resource)
	case "files.list":
		return fmt.Sprintf("List directory: %s", resource)
	case "files.delete":
		return fmt.Sprintf("Delete: %s", resource)
	case "exec.command":
		return fmt.Sprintf("Execute command: %s", resource)
	case "net.request":
		return fmt.Sprintf("HTTP request to: %s", resource)
	default:
		return fmt.Sprintf("Access: %s on %s", action, resource)
	}
}

// SetPermissionCallback sets a callback for permission requests
func (o *Orchestrator) SetPermissionCallback(callback PermissionCallback) {
	o.permissionCallback = callback
}

// RequestPermission requests permission from the user via callback
func (o *Orchestrator) RequestPermission(action string, resource string, reason string) (bool, bool) {
	action = normalizePolicyAction(action)

	if o.permissionCallback == nil {
		// No callback set - deny by default
		return false, false
	}

	req := PermissionRequest{
		Action:      action,
		Resource:    resource,
		Description: FormatPermissionDescription(action, resource),
		Reason:      reason,
	}

	resp := o.permissionCallback(req)
	return resp.Approved, resp.LearnPattern
}

// LearnPermission adds a learned permission rule and saves to policy file
func (o *Orchestrator) LearnPermission(action string, resource string) error {
	action = normalizePolicyAction(action)

	// Create learned rule
	rule := CreateLearnedRule(action, resource)

	// Add to policy engine
	o.policyEngine.AddRule(rule)

	// Save to policy file
	policyPath := o.workspace.ConfigDir + "/policy.yml"
	return policy.SavePolicy(o.policyEngine.GetPolicy(), policyPath)
}

// SetTrustMode enables or disables trust mode (bypass all permission checks).
// When enabled, trust mode auto-expires after 30 minutes.
func (o *Orchestrator) SetTrustMode(enabled bool) {
	o.trustMu.Lock()
	o.trustMode = enabled
	if enabled {
		expiry := time.Now().Add(30 * time.Minute)
		o.trustModeExpiry = &expiry
	} else {
		o.trustModeExpiry = nil
	}
	o.trustMu.Unlock()

	if o.policyEngine != nil {
		o.policyEngine.SetBypassChecker(o.IsTrustMode)
	}
}

// IsTrustMode returns whether trust mode is currently active.
// Returns false if trust mode has expired.
func (o *Orchestrator) IsTrustMode() bool {
	o.trustMu.Lock()
	defer o.trustMu.Unlock()

	if !o.trustMode {
		return false
	}
	if o.trustModeExpiry != nil && time.Now().After(*o.trustModeExpiry) {
		o.trustMode = false
		o.trustModeExpiry = nil
		return false
	}
	return true
}

// TrustModeRemaining returns the time remaining until trust mode expires.
// Returns zero if trust mode is not active.
func (o *Orchestrator) TrustModeRemaining() time.Duration {
	if !o.IsTrustMode() {
		return 0
	}

	o.trustMu.RLock()
	expiry := o.trustModeExpiry
	o.trustMu.RUnlock()
	if expiry == nil {
		return 0
	}

	remaining := time.Until(*expiry)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// checkOrRequestPermission checks policy and requests user permission if denied
func (o *Orchestrator) checkOrRequestPermission(ctx context.Context, action string, resource string) (bool, string) {
	allowed, _, reason := o.checkOrRequestPermissionWithFallback(ctx, action, nil, resource)
	return allowed, reason
}

// checkOrRequestPermissionWithFallback evaluates the primary action first, then
// tries fallback actions without user prompts. If none are allowed, it requests
// approval using the primary action.
func (o *Orchestrator) checkOrRequestPermissionWithFallback(
	ctx context.Context,
	action string,
	fallbackActions []string,
	resource string,
) (bool, string, string) {
	primaryAction := normalizePolicyAction(action)

	// Trust mode: auto-approve everything.
	if o.IsTrustMode() {
		return true, primaryAction, ""
	}

	if o.policyEngine == nil {
		return false, primaryAction, "policy engine not configured"
	}

	candidates := make([]string, 0, 1+len(fallbackActions))
	candidates = append(candidates, primaryAction)
	for _, raw := range fallbackActions {
		candidate := normalizePolicyAction(strings.TrimSpace(raw))
		if candidate == "" {
			continue
		}
		duplicate := false
		for _, existing := range candidates {
			if existing == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			candidates = append(candidates, candidate)
		}
	}

	primaryReason := "no matching rule (default deny)"
	for i, candidate := range candidates {
		result, err := o.policyEngine.Evaluate(ctx, policy.PolicyRequest{
			Action:   candidate,
			Resource: resource,
		})
		if err != nil {
			reason := fmt.Sprintf("policy evaluation error: %v", err)
			if i == 0 {
				primaryReason = reason
			}
			return false, candidate, reason
		}
		if i == 0 {
			primaryReason = result.Reason
		}
		if result.Decision.IsAllow() {
			return true, candidate, ""
		}
	}

	// Policy denied - request permission from user using the primary action.
	approved, learn := o.RequestPermission(primaryAction, resource, primaryReason)
	if !approved {
		return false, primaryAction, primaryReason
	}

	if learn {
		// Learn this pattern for future.
		if err := o.LearnPermission(primaryAction, resource); err != nil {
			// Log error but don't fail the operation.
			fmt.Printf("Warning: failed to save learned permission: %v\n", err)
		}
	}

	return true, primaryAction, ""
}

func normalizePolicyAction(action string) string {
	switch action {
	case "files_read":
		return "files.read"
	case "files_write":
		return "files.write"
	case "files_list":
		return "files.list"
	case "files_delete":
		return "files.delete"
	case "files_stat":
		return "files.stat"
	case "exec_command":
		return "exec.command"
	case "net_request":
		return "net.request"
	case "apply_patch":
		return "patch.apply"
	default:
		return action
	}
}
