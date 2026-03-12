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
	Action      string // e.g., "files_list", "exec_command"
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
	// For file operations, generalize to directory level
	if strings.HasPrefix(action, "files_") {
		// If it's a specific file, allow the whole directory
		if !strings.HasSuffix(resource, "/") {
			dir := filepath.Dir(resource)
			return dir + "/**"
		}
		// If it's already a directory, use it
		return resource + "**"
	}

	// For exec commands, generalize the command
	if action == "exec_command" {
		// Extract command name
		parts := strings.Fields(resource)
		if len(parts) > 0 {
			// Allow all uses of this command
			return parts[0] + " *"
		}
	}

	// For network requests, generalize domain
	if action == "net_request" {
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
	pattern := GenerateSmartPattern(action, resource)

	// Convert action to policy format
	policyAction := action
	if strings.HasPrefix(action, "files_") {
		policyAction = "files.*" // Allow all file operations on this resource
	} else if action == "exec_command" {
		policyAction = "exec.*"
	} else if action == "net_request" {
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
	switch action {
	case "files_read":
		return fmt.Sprintf("Read file: %s", resource)
	case "files_write":
		return fmt.Sprintf("Write file: %s", resource)
	case "files_list":
		return fmt.Sprintf("List directory: %s", resource)
	case "files_delete":
		return fmt.Sprintf("Delete: %s", resource)
	case "exec_command":
		return fmt.Sprintf("Execute command: %s", resource)
	case "net_request":
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
	// Create learned rule
	rule := CreateLearnedRule(action, resource)

	// Add to policy engine
	o.policyEngine.AddRule(rule)

	// Save to policy file
	policyPath := o.workspace.ConfigDir + "/policy.yml"
	return policy.SavePolicy(o.policyEngine.GetPolicy(), policyPath)
}

// checkOrRequestPermission checks policy and requests user permission if denied
func (o *Orchestrator) checkOrRequestPermission(ctx context.Context, action string, resource string) (bool, string) {
	// Evaluate policy
	result, err := o.policyEngine.Evaluate(ctx, policy.PolicyRequest{
		Action:   action,
		Resource: resource,
	})

	if err != nil {
		return false, fmt.Sprintf("policy evaluation error: %v", err)
	}

	// If already allowed, return immediately
	if result.Decision.IsAllow() {
		return true, ""
	}

	// Policy denied - request permission from user
	approved, learn := o.RequestPermission(action, resource, result.Reason)

	if !approved {
		return false, result.Reason
	}

	// User approved!
	if learn {
		// Learn this pattern for future
		if err := o.LearnPermission(action, resource); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to save learned permission: %v\n", err)
		}
	}

	return true, ""
}
