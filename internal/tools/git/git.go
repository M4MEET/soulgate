// Package git provides git operation tools for the SoulGate agentic loop.
// All commands are executed as subprocesses of `git` with the workspace root
// as the working directory. The workspace boundary is never escaped.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// runGit executes a git subcommand in workDir and returns combined output.
// An error is returned for both subprocess failures and non-zero exit codes.
func runGit(ctx context.Context, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		combined := strings.TrimSpace(stderr.String())
		if combined == "" {
			combined = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %w\n%s", args[0], err, combined)
	}

	return strings.TrimRight(stdout.String(), "\n"), nil
}

// Status returns the output of `git status --short` in workDir.
func Status(ctx context.Context, workDir string) (string, error) {
	out, err := runGit(ctx, workDir, "status", "--short")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "working tree clean", nil
	}
	return out, nil
}

// Diff returns the unstaged diff or the staged diff when staged is true.
func Diff(ctx context.Context, workDir string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	out, err := runGit(ctx, workDir, args...)
	if err != nil {
		return "", err
	}
	if out == "" {
		if staged {
			return "no staged changes", nil
		}
		return "no unstaged changes", nil
	}
	return out, nil
}

// Log returns the last n one-line log entries (default 20).
func Log(ctx context.Context, workDir string, n int) (string, error) {
	if n <= 0 {
		n = 20
	}
	out, err := runGit(ctx, workDir, "log", fmt.Sprintf("--oneline"), fmt.Sprintf("-n%d", n))
	if err != nil {
		return "", err
	}
	if out == "" {
		return "no commits yet", nil
	}
	return out, nil
}

// Commit stages the given files (or all changes when files is empty) and
// creates a commit with msg. Returns the commit hash line from git output.
func Commit(ctx context.Context, workDir string, files []string, msg string) (string, error) {
	if strings.TrimSpace(msg) == "" {
		return "", fmt.Errorf("commit message must not be empty")
	}

	// Stage files
	if len(files) == 0 {
		if _, err := runGit(ctx, workDir, "add", "--all"); err != nil {
			return "", fmt.Errorf("stage all: %w", err)
		}
	} else {
		addArgs := append([]string{"add", "--"}, files...)
		if _, err := runGit(ctx, workDir, addArgs...); err != nil {
			return "", fmt.Errorf("stage files: %w", err)
		}
	}

	out, err := runGit(ctx, workDir, "commit", "-m", msg)
	if err != nil {
		return "", err
	}
	return out, nil
}

// Branch lists all branches, creates a new branch, or switches to an existing
// branch depending on the arguments provided.
//
//   - action "list"   -> list branches (local by default; set remote=true for remotes)
//   - action "create" -> create a new branch named name
//   - action "switch" -> switch to branch name
func Branch(ctx context.Context, workDir, action, name string, remote bool) (string, error) {
	switch action {
	case "list":
		args := []string{"branch"}
		if remote {
			args = append(args, "-r")
		}
		return runGit(ctx, workDir, args...)

	case "create":
		if strings.TrimSpace(name) == "" {
			return "", fmt.Errorf("branch name required for create")
		}
		return runGit(ctx, workDir, "branch", name)

	case "switch":
		if strings.TrimSpace(name) == "" {
			return "", fmt.Errorf("branch name required for switch")
		}
		return runGit(ctx, workDir, "checkout", name)

	default:
		return "", fmt.Errorf("unknown branch action %q; valid: list, create, switch", action)
	}
}

// Stash pops or stashes working-tree changes.
//
//   - action "save"  -> stash with an optional message
//   - action "pop"   -> apply the most recent stash entry
//   - action "list"  -> list stash entries
func Stash(ctx context.Context, workDir, action, message string) (string, error) {
	switch action {
	case "save":
		args := []string{"stash", "push"}
		if strings.TrimSpace(message) != "" {
			args = append(args, "-m", message)
		}
		return runGit(ctx, workDir, args...)

	case "pop":
		return runGit(ctx, workDir, "stash", "pop")

	case "list":
		return runGit(ctx, workDir, "stash", "list")

	default:
		return "", fmt.Errorf("unknown stash action %q; valid: save, pop, list", action)
	}
}
