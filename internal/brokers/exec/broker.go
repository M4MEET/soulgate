package exec

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/policy"
)

// Broker provides secure command execution through policy enforcement
type Broker struct {
	workspaceRoot string
	policyEngine  *policy.Engine
	auditLogger   audit.Logger
}

var defaultExecCommandTimeout = 45 * time.Second

// NewBroker creates a new exec broker
func NewBroker(workspaceRoot string, policyEngine *policy.Engine, auditLogger audit.Logger) (*Broker, error) {
	return &Broker{
		workspaceRoot: workspaceRoot,
		policyEngine:  policyEngine,
		auditLogger:   auditLogger,
	}, nil
}

// Name returns the broker name
func (b *Broker) Name() string {
	return "exec"
}

// Close closes the broker
func (b *Broker) Close() error {
	return nil
}

// Execute runs a shell command
func (b *Broker) Execute(ctx context.Context, brokerCtx brokers.BrokerContext, command string) (*ExecResult, error) {
	if b.policyEngine == nil {
		err := fmt.Errorf("policy engine not configured")
		b.logAuditEvent(ctx, brokerCtx, command, "", 1, audit.StatusError, err)
		return nil, err
	}

	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "exec.command",
		Resource: command,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, command, "", 1, audit.StatusError, err)
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, command, "", 1, audit.StatusDenied, err)
		return nil, err
	}

	execCtx := ctx
	cancel := func() {}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && defaultExecCommandTimeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, defaultExecCommandTimeout)
	}
	defer cancel()

	// Execute command in workspace directory
	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = b.workspaceRoot

	// Capture output
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			timeoutErr := fmt.Errorf(
				"command timed out after %s; use process_start for long-running commands",
				defaultExecCommandTimeout.Round(time.Second),
			)
			b.logAuditEvent(ctx, brokerCtx, command, string(output), 124, audit.StatusError, timeoutErr)
			return nil, timeoutErr
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			b.logAuditEvent(ctx, brokerCtx, command, string(output), 1, audit.StatusError, err)
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	execResult := &ExecResult{
		Command:  command,
		Output:   string(output),
		ExitCode: exitCode,
	}

	// Log success
	b.logAuditEvent(ctx, brokerCtx, command, string(output), exitCode, audit.StatusSuccess, nil)

	return execResult, nil
}

// logAuditEvent logs an audit event
func (b *Broker) logAuditEvent(ctx context.Context, brokerCtx brokers.BrokerContext, command, output string, exitCode int, status audit.EventStatus, err error) {
	if b.auditLogger == nil {
		return
	}

	event := audit.NewEvent(audit.EventExecCommand, audit.CategoryBroker).
		WithSessionID(brokerCtx.SessionID).
		WithRunID(brokerCtx.RunID).
		WithPlugin(brokerCtx.PluginID).
		WithResource(command).
		WithStatus(status).
		WithMetadata("exit_code", exitCode)

	if output != "" {
		// Truncate output if too long
		if len(output) > 1000 {
			output = output[:1000] + "... (truncated)"
		}
		event.WithMetadata("output", output)
	}

	if err != nil {
		event.WithError(err)
	}

	// Best effort logging
	b.auditLogger.Log(ctx, event)
}

// ExecResult represents the result of command execution
type ExecResult struct {
	Command  string `json:"command"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
}
