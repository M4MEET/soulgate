package config

import (
	"fmt"
	"os"
	"path/filepath"
)

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
			_ = os.WriteFile(markerPath, []byte("migrated from existing workspace\n"), 0644)
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
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .soulgate directory: %w", err)
	}

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
	config.Audit.DatabasePath = filepath.Join(configDir, "audit.db")
	config.Policy.FilePath = filepath.Join(configDir, "policy.yml")

	// Save configuration
	configPath := filepath.Join(configDir, "config.yml")
	if err := config.Save(configPath); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Create default policy file
	if err := createDefaultPolicy(config.Policy.FilePath); err != nil {
		return nil, fmt.Errorf("failed to create policy file: %w", err)
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

// createDefaultPolicy creates a default policy file
func createDefaultPolicy(path string) error {
	defaultPolicy := `version: "1"

# Default policy: allow workspace reads, deny everything else
policies:
  - name: "allow-workspace-reads"
    description: "Allow reading files within the workspace"
    action: "files.read"
    resource: "./**"
    decision: allow

  - name: "allow-workspace-list"
    description: "Allow listing directories within the workspace"
    action: "files.list"
    resource: "./**"
    decision: allow

  - name: "allow-workspace-stat"
    description: "Allow stat operations within the workspace"
    action: "files.stat"
    resource: "./**"
    decision: allow

  - name: "deny-parent-access"
    description: "Deny access to parent directories"
    action: "files.*"
    resource: "../**"
    decision: deny

  - name: "deny-absolute-paths"
    description: "Deny absolute paths outside workspace"
    action: "files.*"
    resource: "/**"
    decision: deny
`

	return os.WriteFile(path, []byte(defaultPolicy), 0644)
}

// SaveConfig saves the workspace configuration to disk
func (w *Workspace) SaveConfig() error {
	configPath := filepath.Join(w.ConfigDir, "config.yml")
	return w.Config.Save(configPath)
}
