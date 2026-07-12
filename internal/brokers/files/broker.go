package files

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/policy"
)

// Broker provides secure file system access through policy enforcement
type Broker struct {
	workspaceRoot string
	policyEngine  *policy.Engine
	auditLogger   audit.Logger
}

// NewBroker creates a new file broker
func NewBroker(workspaceRoot string, policyEngine *policy.Engine, auditLogger audit.Logger) (*Broker, error) {
	// Ensure workspace root is absolute
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
	}

	// Verify workspace root exists
	if _, err := os.Stat(absRoot); err != nil {
		return nil, fmt.Errorf("workspace root does not exist: %w", err)
	}

	// Resolve symlinks in workspace root for consistent path comparison
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve symlinks in workspace root: %w", err)
	}

	return &Broker{
		workspaceRoot: resolvedRoot,
		policyEngine:  policyEngine,
		auditLogger:   auditLogger,
	}, nil
}

// Name returns the broker name
func (b *Broker) Name() string {
	return "files"
}

// Close closes the broker
func (b *Broker) Close() error {
	return nil
}

// policyOpts controls optional behaviour in withPolicyCheck.
type policyOpts struct {
	coreProtect  bool                        // Whether to check isCoreProtected (write/delete only)
	protectMsgFn func(relPath string) string // Returns the full error message when core protection triggers
}

// withPolicyCheck encapsulates the common guard sequence shared by every broker
// operation:
//  1. Ensure a policy engine is present.
//  2. Validate and resolve the requested path.
//  3. Compute the workspace-relative path.
//  4. Optionally enforce core-protection (write/delete only).
//  5. Evaluate the policy rule.
//  6. Invoke fn with the resolved paths.
//  7. Emit an audit event reflecting the final outcome.
//
// fn receives the resolved absolute path and the workspace-relative path.
// It is responsible for the actual I/O.  Any error returned by fn is wrapped
// and passed back to the caller.
func (b *Broker) withPolicyCheck(
	ctx context.Context,
	brokerCtx brokers.BrokerContext,
	eventType audit.EventType,
	action string,
	path string,
	opts policyOpts,
	fn func(validPath, relPath string) error,
) error {
	// Step 1 – policy engine must be present.
	if b.policyEngine == nil {
		err := fmt.Errorf("policy engine not configured")
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusError, err)
		return err
	}

	// Step 2 – validate and resolve path (enforces workspace boundary).
	validPath, err := validatePath(b.workspaceRoot, path)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusDenied, err)
		return err
	}

	// Step 3 – derive workspace-relative path for policy evaluation.
	relPath, err := getRelativePath(b.workspaceRoot, validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusDenied, err)
		return err
	}

	// Step 4 – core protection (write/delete only).
	if opts.coreProtect && isCoreProtected(relPath) {
		msg := relPath // default fallback
		if opts.protectMsgFn != nil {
			msg = opts.protectMsgFn(relPath)
		}
		err := fmt.Errorf("%s", msg)
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusDenied, err)
		return err
	}

	// Step 5 – evaluate policy.
	policyReq := policy.PolicyRequest{
		Action:   action,
		Resource: "./" + relPath,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusError, err)
		return fmt.Errorf("policy evaluation failed: %w", err)
	}

	// Step 6 – enforce the decision.
	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusDenied, err)
		return err
	}

	// Step 7 – perform the actual I/O operation.
	if err := fn(validPath, relPath); err != nil {
		b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusError, err)
		return err
	}

	// Step 8 – record success.
	b.logAuditEvent(ctx, brokerCtx, eventType, path, audit.StatusSuccess, nil)
	return nil
}

// ReadFile reads a file's contents
func (b *Broker) ReadFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string) ([]byte, error) {
	var content []byte
	err := b.withPolicyCheck(ctx, brokerCtx, audit.EventFileRead, "files.read", path, policyOpts{}, func(validPath, _ string) error {
		var readErr error
		content, readErr = os.ReadFile(validPath)
		if readErr != nil {
			return fmt.Errorf("failed to read file: %w", readErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return content, nil
}

// ListDir lists directory contents
func (b *Broker) ListDir(ctx context.Context, brokerCtx brokers.BrokerContext, path string) ([]FileInfo, error) {
	var files []FileInfo
	err := b.withPolicyCheck(ctx, brokerCtx, audit.EventFileList, "files.list", path, policyOpts{}, func(validPath, _ string) error {
		entries, readErr := os.ReadDir(validPath)
		if readErr != nil {
			return fmt.Errorf("failed to read directory: %w", readErr)
		}

		files = make([]FileInfo, 0, len(entries))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue // Skip entries we can't stat
			}
			files = append(files, FileInfo{
				Name:  entry.Name(),
				IsDir: entry.IsDir(),
				Size:  info.Size(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// WriteFile writes content to a file
func (b *Broker) WriteFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string, content []byte) error {
	opts := policyOpts{
		coreProtect: true,
		protectMsgFn: func(relPath string) string {
			return fmt.Sprintf("core protection: cannot write to '%s' — this is part of SoulGate's protected core. Use skills/, extensions/, plugins/, or .soulgate/ to extend capabilities", relPath)
		},
	}
	return b.withPolicyCheck(ctx, brokerCtx, audit.EventFileWrite, "files.write", path, opts, func(validPath, _ string) error {
		if err := os.WriteFile(validPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		return nil
	})
}

// DeleteFile deletes a file or directory
func (b *Broker) DeleteFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string) error {
	opts := policyOpts{
		coreProtect: true,
		protectMsgFn: func(relPath string) string {
			return fmt.Sprintf("core protection: cannot delete '%s' — this is part of SoulGate's protected core", relPath)
		},
	}
	return b.withPolicyCheck(ctx, brokerCtx, audit.EventFileDelete, "files.delete", path, opts, func(validPath, _ string) error {
		if err := os.RemoveAll(validPath); err != nil {
			return fmt.Errorf("failed to delete: %w", err)
		}
		return nil
	})
}

// Stat returns file information
func (b *Broker) Stat(ctx context.Context, brokerCtx brokers.BrokerContext, path string) (*FileInfo, error) {
	var fileInfo *FileInfo
	err := b.withPolicyCheck(ctx, brokerCtx, audit.EventFileStat, "files.stat", path, policyOpts{}, func(validPath, _ string) error {
		info, statErr := os.Stat(validPath)
		if statErr != nil {
			return fmt.Errorf("failed to stat file: %w", statErr)
		}
		fileInfo = &FileInfo{
			Name:  info.Name(),
			IsDir: info.IsDir(),
			Size:  info.Size(),
			Mode:  info.Mode(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fileInfo, nil
}

// logAuditEvent logs an audit event
func (b *Broker) logAuditEvent(ctx context.Context, brokerCtx brokers.BrokerContext, eventType audit.EventType, resource string, status audit.EventStatus, err error) {
	if b.auditLogger == nil {
		return
	}

	event := audit.NewEvent(eventType, audit.CategoryBroker).
		WithSessionID(brokerCtx.SessionID).
		WithRunID(brokerCtx.RunID).
		WithPlugin(brokerCtx.PluginID).
		WithResource(resource).
		WithStatus(status)

	if err != nil {
		event.WithError(err)
	}

	// Best effort logging - don't fail the operation if audit logging fails
	b.auditLogger.Log(ctx, event)
}

// FileInfo represents file information
type FileInfo struct {
	Name  string      `json:"name"`
	IsDir bool        `json:"is_dir"`
	Size  int64       `json:"size"`
	Mode  fs.FileMode `json:"mode,omitempty"`
}
