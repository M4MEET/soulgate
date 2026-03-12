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

// ReadFile reads a file's contents
func (b *Broker) ReadFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string) ([]byte, error) {
	// Validate and resolve path
	validPath, err := validatePath(b.workspaceRoot, path)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileRead, path, audit.StatusDenied, err)
		return nil, err
	}

	// Get relative path for policy evaluation
	relPath, err := getRelativePath(b.workspaceRoot, validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileRead, path, audit.StatusDenied, err)
		return nil, err
	}

	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "files.read",
		Resource: "./" + relPath,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileRead, path, audit.StatusError, err)
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileRead, path, audit.StatusDenied, err)
		return nil, err
	}

	// Read file
	content, err := os.ReadFile(validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileRead, path, audit.StatusError, err)
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Log success
	b.logAuditEvent(ctx, brokerCtx, audit.EventFileRead, path, audit.StatusSuccess, nil)

	return content, nil
}

// ListDir lists directory contents
func (b *Broker) ListDir(ctx context.Context, brokerCtx brokers.BrokerContext, path string) ([]FileInfo, error) {
	// Validate and resolve path
	validPath, err := validatePath(b.workspaceRoot, path)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileList, path, audit.StatusDenied, err)
		return nil, err
	}

	// Get relative path for policy evaluation
	relPath, err := getRelativePath(b.workspaceRoot, validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileList, path, audit.StatusDenied, err)
		return nil, err
	}

	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "files.list",
		Resource: "./" + relPath,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileList, path, audit.StatusError, err)
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileList, path, audit.StatusDenied, err)
		return nil, err
	}

	// Read directory
	entries, err := os.ReadDir(validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileList, path, audit.StatusError, err)
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Convert to FileInfo
	files := make([]FileInfo, 0, len(entries))
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

	// Log success
	b.logAuditEvent(ctx, brokerCtx, audit.EventFileList, path, audit.StatusSuccess, nil)

	return files, nil
}

// WriteFile writes content to a file
func (b *Broker) WriteFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string, content []byte) error {
	// Validate and resolve path
	validPath, err := validatePath(b.workspaceRoot, path)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileWrite, path, audit.StatusDenied, err)
		return err
	}

	// Get relative path for policy evaluation
	relPath, err := getRelativePath(b.workspaceRoot, validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileWrite, path, audit.StatusDenied, err)
		return err
	}

	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "files.write",
		Resource: "./" + relPath,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileWrite, path, audit.StatusError, err)
		return fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileWrite, path, audit.StatusDenied, err)
		return err
	}

	// Write file
	err = os.WriteFile(validPath, content, 0644)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileWrite, path, audit.StatusError, err)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Log success
	b.logAuditEvent(ctx, brokerCtx, audit.EventFileWrite, path, audit.StatusSuccess, nil)

	return nil
}

// DeleteFile deletes a file or directory
func (b *Broker) DeleteFile(ctx context.Context, brokerCtx brokers.BrokerContext, path string) error {
	// Validate and resolve path
	validPath, err := validatePath(b.workspaceRoot, path)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileDelete, path, audit.StatusDenied, err)
		return err
	}

	// Get relative path for policy evaluation
	relPath, err := getRelativePath(b.workspaceRoot, validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileDelete, path, audit.StatusDenied, err)
		return err
	}

	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "files.delete",
		Resource: "./" + relPath,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileDelete, path, audit.StatusError, err)
		return fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileDelete, path, audit.StatusDenied, err)
		return err
	}

	// Delete file or directory
	err = os.RemoveAll(validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileDelete, path, audit.StatusError, err)
		return fmt.Errorf("failed to delete: %w", err)
	}

	// Log success
	b.logAuditEvent(ctx, brokerCtx, audit.EventFileDelete, path, audit.StatusSuccess, nil)

	return nil
}

// Stat returns file information
func (b *Broker) Stat(ctx context.Context, brokerCtx brokers.BrokerContext, path string) (*FileInfo, error) {
	// Validate and resolve path
	validPath, err := validatePath(b.workspaceRoot, path)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileStat, path, audit.StatusDenied, err)
		return nil, err
	}

	// Get relative path for policy evaluation
	relPath, err := getRelativePath(b.workspaceRoot, validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileStat, path, audit.StatusDenied, err)
		return nil, err
	}

	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "files.stat",
		Resource: "./" + relPath,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileStat, path, audit.StatusError, err)
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileStat, path, audit.StatusDenied, err)
		return nil, err
	}

	// Stat file
	info, err := os.Stat(validPath)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, audit.EventFileStat, path, audit.StatusError, err)
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	fileInfo := &FileInfo{
		Name:  info.Name(),
		IsDir: info.IsDir(),
		Size:  info.Size(),
		Mode:  info.Mode(),
	}

	// Log success
	b.logAuditEvent(ctx, brokerCtx, audit.EventFileStat, path, audit.StatusSuccess, nil)

	return fileInfo, nil
}

// logAuditEvent logs an audit event
func (b *Broker) logAuditEvent(ctx context.Context, brokerCtx brokers.BrokerContext, eventType audit.EventType, resource string, status audit.EventStatus, err error) {
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
