package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/mcp"
	"github.com/M4MEET/soulgate/internal/model"
)

type permissionCheck struct {
	Action          string
	Resource        string
	FallbackActions []string
}

func (o *Orchestrator) authorizeToolCall(ctx context.Context, runID string, toolCall model.ToolCall) error {
	checks, err := o.permissionChecksForToolCall(toolCall)
	if err != nil {
		return err
	}
	if len(checks) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(checks))
	for _, check := range checks {
		key := check.Action + "|" + check.Resource
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		allowed, resolvedAction, reason := o.checkOrRequestPermissionWithFallback(
			ctx,
			check.Action,
			check.FallbackActions,
			check.Resource,
		)
		if !allowed {
			deniedAction := normalizePolicyAction(check.Action)
			if resolvedAction != "" {
				deniedAction = resolvedAction
			}
			if o.audit != nil {
				o.audit.Log(ctx, audit.NewEvent(audit.EventPolicyDeny, audit.CategoryPolicy).
					WithSessionID(o.session.ID).
					WithRunID(runID).
					WithAction(deniedAction).
					WithResource(check.Resource).
					WithMetadata("tool", toolCall.Name).
					WithStatus(audit.StatusDenied))
			}
			return fmt.Errorf("permission denied for %s: %s", toolCall.Name, reason)
		}

		allowedAction := normalizePolicyAction(check.Action)
		if resolvedAction != "" {
			allowedAction = resolvedAction
		}
		if o.audit != nil {
			o.audit.Log(ctx, audit.NewEvent(audit.EventPolicyAllow, audit.CategoryPolicy).
				WithSessionID(o.session.ID).
				WithRunID(runID).
				WithAction(allowedAction).
				WithResource(check.Resource).
				WithMetadata("tool", toolCall.Name).
				WithStatus(audit.StatusSuccess))
		}
	}

	return nil
}

func (o *Orchestrator) permissionChecksForToolCall(toolCall model.ToolCall) ([]permissionCheck, error) {
	// Try the data-driven registry first.
	checks, err := runtimeChecksFromRegistry(toolCall)
	if err != nil {
		return nil, err
	}
	if checks != nil {
		return checks, nil
	}

	// Tool is in the registry but needs no checks (broker-managed or no-permission).
	if _, ok := toolPermissionDefs[toolCall.Name]; ok {
		return nil, nil
	}

	// MCP tools are namespaced as server__tool.
	if mcp.IsMCPTool(toolCall.Name) {
		return []permissionCheck{{
			Action:   "mcp.tool_call",
			Resource: toolCall.Name,
		}}, nil
	}

	// Integration tools default to integration.call with net.request fallback.
	return []permissionCheck{{
		Action:          "integration.call",
		Resource:        "integration:" + toolCall.Name,
		FallbackActions: []string{"net.request"},
	}}, nil
}

func extractApplyPatchPermissionChecks(patchText string) ([]permissionCheck, error) {
	patchText = strings.TrimSpace(patchText)
	if patchText == "" {
		return nil, fmt.Errorf("apply_patch requires non-empty patch")
	}

	lines := strings.Split(patchText, "\n")
	checks := make([]permissionCheck, 0, 8)
	found := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "*** Add File:"):
			found = true
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Add File:"))
			checks = append(checks, permissionCheck{
				Action:          "patch.apply",
				Resource:        normalizePolicyFileResource(path),
				FallbackActions: []string{"files.write"},
			})
		case strings.HasPrefix(trimmed, "*** Update File:"):
			found = true
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Update File:"))
			checks = append(checks, permissionCheck{
				Action:          "patch.apply",
				Resource:        normalizePolicyFileResource(path),
				FallbackActions: []string{"files.write"},
			})
		case strings.HasPrefix(trimmed, "*** Delete File:"):
			found = true
			path := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Delete File:"))
			checks = append(checks, permissionCheck{
				Action:          "patch.apply",
				Resource:        normalizePolicyFileResource(path),
				FallbackActions: []string{"files.delete"},
			})
		case strings.HasPrefix(trimmed, "*** Move File:"):
			found = true
			moveSpec := strings.TrimSpace(strings.TrimPrefix(trimmed, "*** Move File:"))
			parts := strings.SplitN(moveSpec, "->", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid apply_patch move directive: %s", trimmed)
			}
			src := strings.TrimSpace(parts[0])
			dst := strings.TrimSpace(parts[1])
			checks = append(checks,
				permissionCheck{
					Action:          "patch.apply",
					Resource:        normalizePolicyFileResource(src),
					FallbackActions: []string{"files.delete"},
				},
				permissionCheck{
					Action:          "patch.apply",
					Resource:        normalizePolicyFileResource(dst),
					FallbackActions: []string{"files.write"},
				},
			)
		}
	}

	if !found {
		return nil, fmt.Errorf("apply_patch: no file directives found")
	}

	return checks, nil
}

func normalizePolicyFileResource(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" || cleaned == "." {
		return "./."
	}
	if filepath.IsAbs(cleaned) {
		return cleaned
	}
	return "./" + cleaned
}
