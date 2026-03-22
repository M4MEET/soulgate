package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "test-workspace")

	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)
	require.NotNil(t, workspace)

	// Verify workspace structure
	assert.Equal(t, workspaceDir, workspace.Root)
	assert.Equal(t, filepath.Join(workspaceDir, ".soulgate"), workspace.ConfigDir)

	// Verify .soulgate directory exists
	info, err := os.Stat(workspace.ConfigDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify config.yml exists
	configPath := filepath.Join(workspace.ConfigDir, "config.yml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Verify policy.yml exists
	policyPath := filepath.Join(workspace.ConfigDir, "policy.yml")
	_, err = os.Stat(policyPath)
	assert.NoError(t, err)

	// Verify plugins directory exists
	pluginsDir := filepath.Join(workspaceDir, "plugins")
	info, err = os.Stat(pluginsDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInitWorkspaceConfigPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "test-workspace")

	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Verify config file has secure permissions (0600)
	configPath := filepath.Join(workspace.ConfigDir, "config.yml")
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "config file should have 0600 permissions for security")
}

func TestInitWorkspacePolicyContent(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "test-workspace")

	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Read policy file
	policyPath := filepath.Join(workspace.ConfigDir, "policy.yml")
	content, err := os.ReadFile(policyPath)
	require.NoError(t, err)

	// Verify policy contains expected rules
	policyStr := string(content)
	assert.Contains(t, policyStr, "allow-workspace-reads")
	assert.Contains(t, policyStr, "allow-workspace-list")
	assert.Contains(t, policyStr, "deny-parent-access")
	assert.Contains(t, policyStr, "files.read")
	assert.Contains(t, policyStr, "./**")
}

func TestInitWorkspaceConfigValues(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "test-workspace")

	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Verify config has correct paths
	assert.Equal(t, workspaceDir, workspace.Config.Workspace.Root)
	assert.Equal(t, filepath.Join(workspaceDir, ".soulgate"), workspace.Config.Workspace.ConfigDir)
	assert.Equal(t, filepath.Join(workspaceDir, "plugins"), workspace.Config.Plugins.Dir)
	assert.Equal(t, filepath.Join(workspaceDir, ".soulgate", "audit.jsonl"), workspace.Config.Audit.DatabasePath)
	assert.Equal(t, filepath.Join(workspaceDir, ".soulgate", "policy.yml"), workspace.Config.Policy.FilePath)
}

func TestIsInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// Not initialized
	assert.False(t, IsInitialized(tmpDir))

	// Initialize workspace
	_, err := InitWorkspace(tmpDir)
	require.NoError(t, err)

	// Now it should be initialized
	assert.True(t, IsInitialized(tmpDir))
}

func TestLoadWorkspaceFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Initialize workspace first
	_, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Load workspace
	workspace, err := LoadWorkspaceFromPath(workspaceDir)
	require.NoError(t, err)
	require.NotNil(t, workspace)

	assert.Equal(t, workspaceDir, workspace.Root)
	assert.Equal(t, filepath.Join(workspaceDir, ".soulgate"), workspace.ConfigDir)
	assert.NotNil(t, workspace.Config)
}

func TestLoadWorkspaceFromPathNestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Initialize workspace
	_, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Create nested directory
	nestedDir := filepath.Join(workspaceDir, "subdir", "deep")
	err = os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	// Load workspace from nested directory - should find parent workspace
	workspace, err := LoadWorkspaceFromPath(nestedDir)
	require.NoError(t, err)
	assert.Equal(t, workspaceDir, workspace.Root, "should find workspace root from nested directory")
}

func TestLoadWorkspaceFromPathNotInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	workspace, err := LoadWorkspaceFromPath(tmpDir)
	assert.Error(t, err)
	assert.Nil(t, workspace)
	assert.Contains(t, err.Error(), "not a soulgate workspace")
}

func TestFindWorkspaceRoot(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Initialize workspace
	_, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Create nested structure
	nestedDir := filepath.Join(workspaceDir, "a", "b", "c")
	err = os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	// Find workspace root from nested directory
	root, err := findWorkspaceRoot(nestedDir)
	require.NoError(t, err)
	assert.Equal(t, workspaceDir, root)
}

func TestFindWorkspaceRootNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	root, err := findWorkspaceRoot(tmpDir)
	assert.Error(t, err)
	assert.Empty(t, root)
	assert.Contains(t, err.Error(), "not a soulgate workspace")
}

func TestSaveWorkspaceConfig(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Initialize workspace
	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Modify config
	workspace.Config.Model.DefaultProvider = "anthropic"
	workspace.Config.Plugins.Timeout = 120

	// Save config
	err = workspace.SaveConfig()
	require.NoError(t, err)

	// Load workspace again
	reloaded, err := LoadWorkspaceFromPath(workspaceDir)
	require.NoError(t, err)

	// Verify changes were saved
	assert.Equal(t, "anthropic", reloaded.Config.Model.DefaultProvider)
	assert.Equal(t, 120, reloaded.Config.Plugins.Timeout)
}

func TestLoadWorkspaceWithoutConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Create .soulgate directory without config file
	configDir := filepath.Join(workspaceDir, ".soulgate")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// Load workspace - should use default config
	workspace, err := LoadWorkspaceFromPath(workspaceDir)
	require.NoError(t, err)
	assert.NotNil(t, workspace.Config)
	assert.Equal(t, "openai", workspace.Config.Model.DefaultProvider) // Default value
}

func TestInitWorkspaceRelativePath(t *testing.T) {
	// Save current directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Initialize with relative path
	workspace, err := InitWorkspace("./test-workspace")
	require.NoError(t, err)

	// Verify it was converted to absolute path
	assert.True(t, filepath.IsAbs(workspace.Root))
	assert.Contains(t, workspace.Root, "test-workspace")
}

func TestCreateDefaultPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.yml")

	err := createDefaultPolicy(policyPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(policyPath)
	assert.NoError(t, err)

	// Read and verify content
	content, err := os.ReadFile(policyPath)
	require.NoError(t, err)

	policyStr := string(content)
	assert.Contains(t, policyStr, "version:")
	assert.Contains(t, policyStr, "policies:")
	assert.Contains(t, policyStr, "allow-workspace-reads")
	assert.Contains(t, policyStr, "deny-parent-access")
}

func TestWorkspacePathsAreAbsolute(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// All paths should be absolute
	assert.True(t, filepath.IsAbs(workspace.Root))
	assert.True(t, filepath.IsAbs(workspace.ConfigDir))
	assert.True(t, filepath.IsAbs(workspace.Config.Plugins.Dir))
	assert.True(t, filepath.IsAbs(workspace.Config.Audit.DatabasePath))
	assert.True(t, filepath.IsAbs(workspace.Config.Policy.FilePath))
}

func TestLoadWorkspaceRelativePathsConvertedToAbsolute(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Initialize workspace
	_, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Modify config file to have relative paths
	configPath := filepath.Join(workspaceDir, ".soulgate", "config.yml")
	relativeConfigYAML := `
workspace:
  root: .
  config_dir: .soulgate

model:
  default_provider: openai

plugins:
  dir: plugins
  timeout: 30

audit:
  database_path: .soulgate/audit.jsonl
  enabled: true

policy:
  file_path: .soulgate/policy.yml
`
	err = os.WriteFile(configPath, []byte(relativeConfigYAML), 0600)
	require.NoError(t, err)

	// Load workspace
	workspace, err := LoadWorkspaceFromPath(workspaceDir)
	require.NoError(t, err)

	// Relative paths should be converted to absolute
	assert.True(t, filepath.IsAbs(workspace.Config.Plugins.Dir))
	assert.True(t, filepath.IsAbs(workspace.Config.Audit.DatabasePath))
	assert.True(t, filepath.IsAbs(workspace.Config.Policy.FilePath))

	// Paths should be relative to workspace root
	assert.Contains(t, workspace.Config.Plugins.Dir, workspaceDir)
	assert.Contains(t, workspace.Config.Audit.DatabasePath, workspaceDir)
	assert.Contains(t, workspace.Config.Policy.FilePath, workspaceDir)
}

func TestMultipleWorkspaceInitialization(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize multiple workspaces
	workspace1Dir := filepath.Join(tmpDir, "workspace1")
	workspace2Dir := filepath.Join(tmpDir, "workspace2")

	ws1, err := InitWorkspace(workspace1Dir)
	require.NoError(t, err)

	ws2, err := InitWorkspace(workspace2Dir)
	require.NoError(t, err)

	// They should be separate
	assert.NotEqual(t, ws1.Root, ws2.Root)
	assert.NotEqual(t, ws1.ConfigDir, ws2.ConfigDir)
}

func TestWorkspaceConfigPreservesCustomValues(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	// Initialize workspace
	workspace, err := InitWorkspace(workspaceDir)
	require.NoError(t, err)

	// Modify config
	workspace.Config.Model.DefaultProvider = "anthropic"
	workspace.Config.Model.OpenAI.MaxTokens = 8000
	workspace.Config.Execution.MaxIterations = 20
	workspace.Config.HTTPClient.AllowPrivateIPs = true

	// Save
	err = workspace.SaveConfig()
	require.NoError(t, err)

	// Reload
	reloaded, err := LoadWorkspaceFromPath(workspaceDir)
	require.NoError(t, err)

	// Verify all custom values preserved
	assert.Equal(t, "anthropic", reloaded.Config.Model.DefaultProvider)
	assert.Equal(t, 8000, reloaded.Config.Model.OpenAI.MaxTokens)
	assert.Equal(t, 20, reloaded.Config.Execution.MaxIterations)
	assert.True(t, reloaded.Config.HTTPClient.AllowPrivateIPs)
}
