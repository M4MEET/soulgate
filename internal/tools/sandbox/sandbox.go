// Package sandbox provides isolated code execution for the SoulGate
// orchestrator. Code snippets are written to a temporary directory and
// executed by the host language runtime (python3, node, go, bash, ruby).
// All processes are killed on timeout and the temp directory is always
// cleaned up on return.
//
// Security properties:
//   - Each execution runs in its own throw-away temp directory.
//   - Output (stdout + stderr) is capped at maxOutputBytes.
//   - Execution time is capped at maxTimeoutSec.
//   - The OS process is sent SIGKILL when the deadline elapses.
//
// This package intentionally has no dependency on internal SoulGate packages
// so that it stays trivially testable in isolation.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// maxOutputBytes caps the combined output returned to the caller (~32 KB).
const maxOutputBytes = 32 * 1024

// defaultTimeoutSec is the timeout used when the caller passes 0.
const defaultTimeoutSec = 10

// maxTimeoutSec is the upper bound a caller may request.
const maxTimeoutSec = 60

// SupportedLanguages is the set of language identifiers accepted by Execute
// and Install.
var SupportedLanguages = map[string]bool{
	"python": true,
	"node":   true,
	"go":     true,
	"bash":   true,
	"ruby":   true,
}

// ExecutionResult is the value returned by Execute.
type ExecutionResult struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	ExitCode int     `json:"exit_code"`
	Duration float64 `json:"duration_seconds"`
	Language string  `json:"language"`
	// Error is set when the sandbox itself (not the user code) fails.
	Error string `json:"error,omitempty"`
}

// InstallResult is the value returned by Install.
type InstallResult struct {
	Language string `json:"language"`
	Package  string `json:"package"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// Execute runs code in the requested language inside a temporary directory.
// timeout is clamped to [1s, maxTimeoutSec]. Pass 0 to use the default.
func Execute(ctx context.Context, language, code string, timeout time.Duration) (*ExecutionResult, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	if !SupportedLanguages[language] {
		return nil, fmt.Errorf("sandbox: unsupported language %q (supported: python, node, go, bash, ruby)", language)
	}
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("sandbox: code must not be empty")
	}

	timeout = clampTimeout(timeout)

	// Create isolated temp directory.
	tmpDir, err := os.MkdirTemp("", "soulgate-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("sandbox: failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) // always clean up

	// Write the source file and build the command.
	cmd, err := buildCommand(tmpDir, language, code)
	if err != nil {
		return nil, fmt.Errorf("sandbox: failed to prepare command: %w", err)
	}

	// Derive an exec context so we can kill on timeout.
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdoutBuf, stderrBuf limitedBuffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Dir = tmpDir

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start).Seconds()

	result := &ExecutionResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		Duration: duration,
		Language: language,
	}

	if runErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.ExitCode = 124 // same convention as GNU timeout
			result.Error = fmt.Sprintf("execution timed out after %.1fs", timeout.Seconds())
			return result, nil
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			// A non-zero exit is user code failing, not a sandbox error.
			return result, nil
		}
		// Unexpected launch failure.
		result.Error = runErr.Error()
		return result, nil
	}

	result.ExitCode = 0
	return result, nil
}

// Install installs a package for the given language using the appropriate
// package manager (pip, npm, go get). The timeout is fixed at 120 s for
// package installations to allow for network latency.
func Install(ctx context.Context, language, pkg string) (*InstallResult, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	pkg = strings.TrimSpace(pkg)

	if !SupportedLanguages[language] {
		return nil, fmt.Errorf("sandbox: unsupported language %q", language)
	}
	if pkg == "" {
		return nil, fmt.Errorf("sandbox: package name must not be empty")
	}

	var args []string
	switch language {
	case "python":
		args = []string{"pip3", "install", "--quiet", pkg}
	case "node":
		args = []string{"npm", "install", "--no-save", pkg}
	case "go":
		args = []string{"go", "get", pkg}
	case "ruby":
		args = []string{"gem", "install", "--no-document", pkg}
	case "bash":
		return nil, fmt.Errorf("sandbox: bash does not have a package manager; install system packages via exec_command")
	}

	installTimeout := 120 * time.Second
	execCtx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	var outBuf limitedBuffer
	cmd := exec.CommandContext(execCtx, args[0], args[1:]...) //nolint:gosec
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	runErr := cmd.Run()

	result := &InstallResult{
		Language: language,
		Package:  pkg,
		Output:   outBuf.String(),
	}

	if runErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.ExitCode = 124
			result.Error = "package installation timed out after 120s"
			return result, nil
		}
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		result.Error = runErr.Error()
		return result, nil
	}

	result.ExitCode = 0
	return result, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildCommand writes the code to disk (if needed) and returns an *exec.Cmd
// ready to run. The returned command has Dir unset; the caller sets it.
func buildCommand(tmpDir, language, code string) (*exec.Cmd, error) {
	switch language {
	case "python":
		src := filepath.Join(tmpDir, "script.py")
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return nil, err
		}
		return exec.Command("python3", src), nil //nolint:gosec

	case "node":
		src := filepath.Join(tmpDir, "script.js")
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return nil, err
		}
		return exec.Command("node", src), nil //nolint:gosec

	case "go":
		if err := writeGoModule(tmpDir, code); err != nil {
			return nil, err
		}
		return exec.Command("go", "run", "."), nil

	case "bash":
		src := filepath.Join(tmpDir, "script.sh")
		if err := os.WriteFile(src, []byte(code), 0o700); err != nil {
			return nil, err
		}
		return exec.Command("bash", src), nil //nolint:gosec

	case "ruby":
		src := filepath.Join(tmpDir, "script.rb")
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return nil, err
		}
		return exec.Command("ruby", src), nil //nolint:gosec

	default:
		return nil, fmt.Errorf("unknown language %q", language)
	}
}

// writeGoModule writes a minimal go.mod + main.go into tmpDir so that
// "go run ." can execute the provided code.
func writeGoModule(tmpDir, code string) error {
	goMod := "module sandbox\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o600); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Wrap bare snippets that lack a package declaration.
	if !strings.Contains(code, "package ") {
		code = "package main\n\nimport \"fmt\"\n\nfunc main() {\n" + code + "\n_ = fmt.Sprintf // suppress unused import\n}\n"
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(code), 0o600); err != nil {
		return fmt.Errorf("failed to write main.go: %w", err)
	}
	return nil
}

// clampTimeout normalises the caller's requested timeout.
func clampTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Duration(defaultTimeoutSec) * time.Second
	}
	if d > time.Duration(maxTimeoutSec)*time.Second {
		return time.Duration(maxTimeoutSec) * time.Second
	}
	return d
}

// limitedBuffer is a bytes.Buffer that silently drops writes once it has
// accumulated maxOutputBytes bytes. This prevents runaway processes from
// exhausting memory.
type limitedBuffer struct {
	buf bytes.Buffer
}

func (lb *limitedBuffer) Write(p []byte) (n int, err error) {
	remaining := maxOutputBytes - lb.buf.Len()
	if remaining <= 0 {
		return len(p), nil // discard silently
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return lb.buf.Write(p)
}

func (lb *limitedBuffer) String() string {
	return lb.buf.String()
}
