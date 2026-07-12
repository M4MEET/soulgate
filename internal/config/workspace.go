package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// subDirs lists all subdirectories that must exist inside .soulgate/.
// Created with 0700 permissions so only the owner can read them.
var subDirs = []string{
	"hub",
	"hub/skills",
	"hub/agents",
	"hub/tools",
	"hub/connectors",
	"hub/mcp",
	"hub/plugins",
	"state",
	"state/vectors",
	"security",
	"logs",
	"logs/sessions",
	"canvas",
}

// MigrateFilesystem moves files from the old flat .soulgate/ layout to the
// new organised subdirectory layout.  It is safe to run on a workspace that
// has already been migrated — moves are skipped when the source file does not
// exist or the destination already exists.
//
// Callers outside this package (e.g. cmd/soulgate/cmd/chat.go) may invoke
// this directly when they manage a global config directory rather than a
// per-workspace one.
func MigrateFilesystem(configDir string) {
	// Ensure new directories exist before moving files into them.
	for _, dir := range subDirs {
		_ = os.MkdirAll(filepath.Join(configDir, dir), 0700)
	}

	// Simple 1-to-1 file renames.
	moves := map[string]string{
		"memory.json":            "state/memory.json",
		"agents_state.json":      "state/agents.json",
		"branches.json":          "state/branches.json",
		"web_threads.json":       "state/threads.json",
		"cron_jobs.json":         "state/cron.json",
		"heartbeat_state.json":   "state/heartbeat.json",
		"session_state.json":     "state/session.json",
		"policy.yml":             "security/policy.yml",
		"scoped_policy.yml":      "security/scoped_policy.yml",
		"secrets.json":           "security/secrets.json",
		"users.json":             "security/users.json",
		"api_tokens.json":        "security/tokens.json",
		"approval_requests.json": "security/approvals.json",
		"costs.jsonl":            "logs/costs.jsonl",
		"hub-installed.json":     "hub/installed.json",
		// Legacy audit DB
		"audit.jsonl": "logs/audit.jsonl",
	}
	for src, dst := range moves {
		srcPath := filepath.Join(configDir, src)
		dstPath := filepath.Join(configDir, dst)
		// Skip if source does not exist.
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			continue
		}
		// Skip if destination already exists (idempotent).
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}
		// Best-effort rename — ignore errors so a partial migration does
		// not prevent the runtime from starting.
		_ = os.Rename(srcPath, dstPath)
	}

	// Move audit-YYYY-MM-DD.jsonl files to logs/
	entries, err := os.ReadDir(configDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// Match audit-YYYY-MM-DD.jsonl pattern (length 22, starts with "audit-")
			if len(name) == 22 && name[:6] == "audit-" && name[len(name)-6:] == ".jsonl" {
				src := filepath.Join(configDir, name)
				dst := filepath.Join(configDir, "logs", name)
				if _, err := os.Stat(dst); os.IsNotExist(err) {
					_ = os.Rename(src, dst)
				}
			}
		}
	}

	// Move vectors/ subdirectory to state/vectors/
	oldVectors := filepath.Join(configDir, "vectors")
	newVectors := filepath.Join(configDir, "state", "vectors")
	if info, err := os.Stat(oldVectors); err == nil && info.IsDir() {
		if _, err := os.Stat(newVectors); os.IsNotExist(err) {
			_ = os.Rename(oldVectors, newVectors)
		}
	}

	// Move canvas/ contents to canvas/ (already at the right relative path,
	// nothing to move — directory was already created above).
}

// Workspace represents a SoulGate workspace
type Workspace struct {
	// Root directory of the workspace
	Root string

	// Configuration directory (.soulgate)
	ConfigDir string

	// Configuration
	Config *Config
}

// LoadWorkspace loads the workspace from the current directory
func LoadWorkspace() (*Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	return LoadWorkspaceFromPath(cwd)
}

// LoadWorkspaceFromPath loads the workspace from a specific path
func LoadWorkspaceFromPath(path string) (*Workspace, error) {
	// Find the workspace root by looking for .soulgate directory
	root, err := findWorkspaceRoot(path)
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(root, ".soulgate")

	// Load configuration
	configPath := filepath.Join(configDir, "config.yml")
	var config *Config

	if _, err := os.Stat(configPath); err == nil {
		// Config file exists, load it
		config, err = LoadConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		// Use default config
		config = DefaultConfig()
	}

	// Set workspace-specific paths
	config.Workspace.Root = root
	config.Workspace.ConfigDir = configDir

	// Make paths absolute
	if !filepath.IsAbs(config.Plugins.Dir) {
		config.Plugins.Dir = filepath.Join(root, config.Plugins.Dir)
	}
	if !filepath.IsAbs(config.Audit.DatabasePath) {
		config.Audit.DatabasePath = filepath.Join(root, config.Audit.DatabasePath)
	}
	if !filepath.IsAbs(config.Policy.FilePath) {
		config.Policy.FilePath = filepath.Join(root, config.Policy.FilePath)
	}

	// Migrate any pre-existing flat files to the organised subdirectory layout.
	MigrateFilesystem(configDir)

	workspace := &Workspace{
		Root:      root,
		ConfigDir: configDir,
		Config:    config,
	}

	// Auto-migrate existing workspaces (mark onboarding complete if API keys are configured)
	markerPath := filepath.Join(configDir, ".onboarding_complete")
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		// Marker doesn't exist - check if this is an existing configured workspace
		hasAPIKey := false
		if config.Model.OpenAI.APIKey != "" || os.Getenv("OPENAI_API_KEY") != "" {
			hasAPIKey = true
		}
		if config.Model.Anthropic.APIKey != "" || os.Getenv("ANTHROPIC_API_KEY") != "" {
			hasAPIKey = true
		}

		// If user has already configured API keys, mark onboarding as complete
		// This prevents forcing existing users through onboarding on upgrade
		if hasAPIKey {
			// Create marker file (ignore errors - worst case: user goes through onboarding again)
			_ = os.WriteFile(markerPath, []byte("migrated from existing workspace\n"), 0600)
		}
	}

	return workspace, nil
}

// InitWorkspace initializes a new workspace in the given directory
func InitWorkspace(path string) (*Workspace, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	configDir := filepath.Join(absPath, ".soulgate")

	// Create .soulgate directory
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create .soulgate directory: %w", err)
	}

	// Migrate any pre-existing flat files to the organised subdirectory layout.
	// This is idempotent — safe to call on a fresh or already-migrated workspace.
	MigrateFilesystem(configDir)

	// Create plugins directory
	pluginsDir := filepath.Join(absPath, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Create default configuration
	config := DefaultConfig()
	config.Workspace.Root = absPath
	config.Workspace.ConfigDir = configDir
	config.Plugins.Dir = pluginsDir
	config.Audit.DatabasePath = filepath.Join(configDir, "logs", "audit.jsonl")
	config.Policy.FilePath = filepath.Join(configDir, "security", "policy.yml")
	config.Policy.ScopedFilePath = filepath.Join(configDir, "security", "scoped_policy.yml")

	// Save configuration
	configPath := filepath.Join(configDir, "config.yml")
	if err := config.Save(configPath); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Create default policy file
	if err := createDefaultPolicy(config.Policy.FilePath); err != nil {
		return nil, fmt.Errorf("failed to create policy file: %w", err)
	}

	// Create default HEARTBEAT.md if it does not exist yet.
	heartbeatPath := filepath.Join(configDir, "HEARTBEAT.md")
	if _, err := os.Stat(heartbeatPath); os.IsNotExist(err) {
		if err := createDefaultHeartbeatFile(heartbeatPath); err != nil {
			// Non-fatal — heartbeat falls back to built-in instructions.
			fmt.Fprintf(os.Stderr, "warning: could not create HEARTBEAT.md: %v\n", err)
		}
	}

	workspace := &Workspace{
		Root:      absPath,
		ConfigDir: configDir,
		Config:    config,
	}

	return workspace, nil
}

// IsInitialized checks if a directory is a SoulGate workspace
func IsInitialized(path string) bool {
	configDir := filepath.Join(path, ".soulgate")
	info, err := os.Stat(configDir)
	return err == nil && info.IsDir()
}

// findWorkspaceRoot searches for the workspace root by looking for .soulgate directory
func findWorkspaceRoot(startPath string) (string, error) {
	current := startPath

	for {
		if IsInitialized(current) {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding workspace
			return "", fmt.Errorf("not a soulgate workspace (no .soulgate directory found)")
		}

		current = parent
	}
}

// createDefaultHeartbeatFile writes the default HEARTBEAT.md instructions.
func createDefaultHeartbeatFile(path string) error {
	content := `# Heartbeat Check

On each heartbeat, check:
1. Any running background agents that completed or failed
2. Any file watchers that triggered
3. System health (disk space, memory)
4. Any pending approval requests
5. Cron jobs that failed

If everything is fine, respond with just "OK".
If something needs attention, describe it briefly.
`
	return os.WriteFile(path, []byte(content), 0600)
}

// createDefaultPolicy creates a default policy file
func createDefaultPolicy(path string) error {
	defaultPolicy := `version: "1"

# Default policy: allow all operations
policies:
  - name: "allow-all"
    action: "*"
    resource: "*"
    decision: allow
`

	return os.WriteFile(path, []byte(defaultPolicy), 0600)
}

// SaveConfig saves the workspace configuration to disk
func (w *Workspace) SaveConfig() error {
	configPath := filepath.Join(w.ConfigDir, "config.yml")
	return w.Config.Save(configPath)
}
