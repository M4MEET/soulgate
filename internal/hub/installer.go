package hub

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Installer handles installation of hub items
type Installer struct {
	client    *HubClient
	registry  *Registry
	pluginDir string
	skillDir  string
}

// NewInstaller creates a new installer
func NewInstaller(client *HubClient, registry *Registry, configDir string) *Installer {
	return &Installer{
		client:    client,
		registry:  registry,
		pluginDir: filepath.Join(configDir, "plugins"),
		skillDir:  filepath.Join(configDir, "skills"),
	}
}

// InstallPlugin installs a plugin from the hub
func (i *Installer) InstallPlugin(ctx context.Context, name string, opts InstallOptions) error {
	// Check if already installed
	if !opts.Force && i.registry.IsInstalled("plugin", name) {
		return fmt.Errorf("plugin '%s' is already installed (use --force to reinstall)", name)
	}

	// Get plugin details
	details, err := i.client.GetPlugin(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get plugin details: %w", err)
	}

	// Create plugin directory
	destDir := filepath.Join(i.pluginDir, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download plugin binary
	binaryPath := filepath.Join(destDir, "plugin.wasm")
	if err := i.client.DownloadPlugin(ctx, name, binaryPath); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	// Save manifest
	manifestPath := filepath.Join(destDir, "manifest.yml")
	if err := i.saveManifest(manifestPath, details); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	// Register installation
	if err := i.registry.Add(InstalledItem{
		Type:    "plugin",
		Name:    name,
		Version: details.Version,
	}); err != nil {
		return fmt.Errorf("failed to register installation: %w", err)
	}

	return nil
}

// InstallSkill installs a skill from the hub
func (i *Installer) InstallSkill(ctx context.Context, name string, opts InstallOptions) error {
	// Check if already installed
	if !opts.Force && i.registry.IsInstalled("skill", name) {
		return fmt.Errorf("skill '%s' is already installed (use --force to reinstall)", name)
	}

	// TODO: Implement skill installation
	// For now, just register it
	if err := i.registry.Add(InstalledItem{
		Type:    "skill",
		Name:    name,
		Version: "1.0.0",
	}); err != nil {
		return fmt.Errorf("failed to register installation: %w", err)
	}

	return nil
}

// Uninstall removes an installed item
func (i *Installer) Uninstall(itemType, name string) error {
	// Check if installed
	if !i.registry.IsInstalled(itemType, name) {
		return fmt.Errorf("%s '%s' is not installed", itemType, name)
	}

	// Remove directory
	var dir string
	switch itemType {
	case "plugin":
		dir = filepath.Join(i.pluginDir, name)
	case "skill":
		dir = filepath.Join(i.skillDir, name)
	default:
		return fmt.Errorf("unsupported item type: %s", itemType)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove directory: %w", err)
	}

	// Unregister
	if err := i.registry.Remove(itemType, name); err != nil {
		return fmt.Errorf("failed to unregister: %w", err)
	}

	return nil
}

// UpdateAll updates all installed items
func (i *Installer) UpdateAll(ctx context.Context) ([]string, error) {
	updated := []string{}

	for _, item := range i.registry.List() {
		// Check for updates
		// TODO: Implement version comparison
		// For now, skip
		_ = item
	}

	return updated, nil
}

// saveManifest saves plugin manifest
func (i *Installer) saveManifest(path string, details *PluginDetails) error {
	data, err := yaml.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// ValidatePermissions checks if user approves required permissions
func (i *Installer) ValidatePermissions(permissions []string) (bool, error) {
	if len(permissions) == 0 {
		return true, nil
	}

	fmt.Println("\nThis plugin requires the following permissions:")
	for _, perm := range permissions {
		fmt.Printf("  - %s\n", perm)
	}

	fmt.Print("\nGrant these permissions? (Y/n): ")
	var response string
	fmt.Scanln(&response)

	if response == "" || response == "y" || response == "Y" {
		return true, nil
	}

	return false, nil
}
