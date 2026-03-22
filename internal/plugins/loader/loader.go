package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/M4MEET/soulgate/internal/plugins/sdk"
	"gopkg.in/yaml.v3"
)

// Loader discovers and loads plugins
type Loader struct {
	pluginDir string
}

// NewLoader creates a new plugin loader
func NewLoader(pluginDir string) *Loader {
	return &Loader{
		pluginDir: pluginDir,
	}
}

// LoadAll loads all plugins from the plugin directory
func (l *Loader) LoadAll() ([]*Plugin, error) {
	// Check if plugin directory exists
	if _, err := os.Stat(l.pluginDir); os.IsNotExist(err) {
		return []*Plugin{}, nil // No plugins directory, return empty list
	}

	// Read plugin directory
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	plugins := make([]*Plugin, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(l.pluginDir, entry.Name())
		plugin, err := l.LoadPlugin(pluginPath)
		if err != nil {
			// Log error but continue loading other plugins
			fmt.Fprintf(os.Stderr, "warning: failed to load plugin %s: %v\n", entry.Name(), err)
			continue
		}

		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// LoadPlugin loads a single plugin from a directory
func (l *Loader) LoadPlugin(pluginPath string) (*Plugin, error) {
	// Load manifest
	manifestPath := filepath.Join(pluginPath, "manifest.yml")
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Validate manifest
	if err := ValidateManifest(manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}

	// Determine WASM file path
	wasmPath := filepath.Join(pluginPath, "plugin.wasm")
	if manifest.Entrypoint != "" {
		wasmPath = filepath.Join(pluginPath, manifest.Entrypoint)
	}

	// Check if WASM file exists
	if _, err := os.Stat(wasmPath); err != nil {
		return nil, fmt.Errorf("WASM file not found: %s", wasmPath)
	}

	plugin := &Plugin{
		Path:     pluginPath,
		Manifest: manifest,
		WASMPath: wasmPath,
	}

	return plugin, nil
}

// Plugin represents a loaded plugin
type Plugin struct {
	Path     string
	Manifest *sdk.Manifest
	WASMPath string
}

// GetID returns a unique identifier for the plugin
func (p *Plugin) GetID() string {
	return p.Manifest.Name
}

// loadManifest loads a plugin manifest from a YAML file
func loadManifest(path string) (*sdk.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest sdk.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Resolve YAML input_schema maps to JSON
	if err := sdk.ResolveInputSchemas(manifest.Tools); err != nil {
		return nil, fmt.Errorf("failed to resolve input schemas: %w", err)
	}

	return &manifest, nil
}
