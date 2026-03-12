package exec

import (
	"context"
	"fmt"
	"os/exec"

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

	// Execute command in workspace directory
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = b.workspaceRoot

	// Capture output
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
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
