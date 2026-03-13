package dependencies

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Dependency represents a required dependency for an integration
type Dependency struct {
	Name        string
	Type        string // "npm", "binary"
	Package     string
	Version     string
	Description string
}

// IntegrationDependencies maps integration IDs to their dependencies
var IntegrationDependencies = map[string][]Dependency{
	"slack": {
		{
			Name:        "Slack SDK",
			Type:        "npm",
			Package:     "@slack/web-api",
			Version:     "latest",
			Description: "Slack Web API client",
		},
	},
	"github": {
		{
			Name:        "GitHub API",
			Type:        "npm",
			Package:     "@octokit/rest",
			Version:     "latest",
			Description: "GitHub REST API client",
		},
	},
	"notion": {
		{
			Name:        "Notion SDK",
			Type:        "npm",
			Package:     "@notionhq/client",
			Version:     "latest",
			Description: "Official Notion SDK",
		},
	},
	"jira": {
		{
			Name:        "Jira Client",
			Type:        "npm",
			Package:     "jira-client",
			Version:     "latest",
			Description: "Jira API client",
		},
	},
	"linear": {
		{
			Name:        "Linear SDK",
			Type:        "npm",
			Package:     "@linear/sdk",
			Version:     "latest",
			Description: "Linear GraphQL SDK",
		},
	},
	"aws": {
		{
			Name:        "AWS SDK",
			Type:        "npm",
			Package:     "aws-sdk",
			Version:     "latest",
			Description: "AWS SDK for JavaScript",
		},
	},
	"google": {
		{
			Name:        "Google APIs",
			Type:        "npm",
			Package:     "googleapis",
			Version:     "latest",
			Description: "Google APIs client library",
		},
	},
	"docker": {
		{
			Name:        "Docker API",
			Type:        "npm",
			Package:     "dockerode",
			Version:     "latest",
			Description: "Docker API client",
		},
	},
}

// DependencyInstaller handles installing integration dependencies
type DependencyInstaller struct {
	soulGateDir string
	nodeModules string
	binDir      string
	depsFile    string
	verbose     bool
}

// InstalledDependencies tracks what's been installed
type InstalledDependencies struct {
	Dependencies map[string][]string `yaml:"dependencies"` // integrationID -> list of packages
}

// NewDependencyInstaller creates a new dependency installer
func NewDependencyInstaller(soulGateDir string, verbose bool) *DependencyInstaller {
	return &DependencyInstaller{
		soulGateDir: soulGateDir,
		nodeModules: filepath.Join(soulGateDir, "node_modules"),
		binDir:      filepath.Join(soulGateDir, "bin"),
		depsFile:    filepath.Join(soulGateDir, "dependencies.yml"),
		verbose:     verbose,
	}
}

// EnsureDirectories creates necessary directories
func (di *DependencyInstaller) EnsureDirectories() error {
	dirs := []string{
		di.nodeModules,
		di.binDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// CheckDependency checks if a dependency is installed
func (di *DependencyInstaller) CheckDependency(ctx context.Context, dep Dependency) (bool, error) {
	switch dep.Type {
	case "npm":
		// Check if package exists in local node_modules
		packagePath := filepath.Join(di.nodeModules, dep.Package)
		_, err := os.Stat(packagePath)
		return err == nil, nil

	case "binary":
		// Check if binary exists in .soulgate/bin
		binaryPath := filepath.Join(di.binDir, dep.Package)
		_, err := os.Stat(binaryPath)
		return err == nil, nil

	default:
		return false, fmt.Errorf("unsupported dependency type: %s", dep.Type)
	}
}

// InstallDependency installs a dependency locally
func (di *DependencyInstaller) InstallDependency(ctx context.Context, dep Dependency) error {
	switch dep.Type {
	case "npm":
		return di.installNpmPackage(ctx, dep)
	case "binary":
		return fmt.Errorf("binary installation not yet supported for %s", dep.Name)
	default:
		return fmt.Errorf("unsupported dependency type: %s", dep.Type)
	}
}

// installNpmPackage installs an npm package locally
func (di *DependencyInstaller) installNpmPackage(ctx context.Context, dep Dependency) error {
	// Ensure node_modules directory exists
	if err := os.MkdirAll(di.nodeModules, 0755); err != nil {
		return fmt.Errorf("failed to create node_modules: %w", err)
	}

	// Check if npm is available
	if !commandExists("npm") {
		return fmt.Errorf("npm not found - please install Node.js")
	}

	// Create package.json if it doesn't exist
	packageJSON := filepath.Join(di.soulGateDir, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		content := `{
  "name": "soulgate-dependencies",
  "version": "1.0.0",
  "description": "SoulGate integration dependencies",
  "private": true,
  "dependencies": {}
}
`
		if err := os.WriteFile(packageJSON, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create package.json: %w", err)
		}
	}

	// Install package locally using npm install
	packageSpec := dep.Package
	if dep.Version != "" && dep.Version != "latest" {
		packageSpec = fmt.Sprintf("%s@%s", dep.Package, dep.Version)
	}

	cmd := exec.CommandContext(ctx, "npm", "install", packageSpec, "--prefix", di.soulGateDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if di.verbose {
		fmt.Printf("Installing %s locally...\n", dep.Name)
		fmt.Printf("Running: npm install %s --prefix %s\n", packageSpec, di.soulGateDir)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}

// GetMissingDependencies returns dependencies that are not installed
func (di *DependencyInstaller) GetMissingDependencies(ctx context.Context, integrationID string) ([]Dependency, error) {
	deps, ok := IntegrationDependencies[integrationID]
	if !ok {
		// No dependencies required
		return []Dependency{}, nil
	}

	var missing []Dependency
	for _, dep := range deps {
		installed, err := di.CheckDependency(ctx, dep)
		if err != nil {
			return nil, fmt.Errorf("failed to check %s: %w", dep.Name, err)
		}
		if !installed {
			missing = append(missing, dep)
		}
	}

	return missing, nil
}

// InstallAll installs all missing dependencies for an integration
func (di *DependencyInstaller) InstallAll(ctx context.Context, integrationID string) ([]string, error) {
	// Ensure directories exist
	if err := di.EnsureDirectories(); err != nil {
		return nil, err
	}

	missing, err := di.GetMissingDependencies(ctx, integrationID)
	if err != nil {
		return nil, err
	}

	if len(missing) == 0 {
		return []string{}, nil
	}

	var installed []string
	var errors []string

	for _, dep := range missing {
		if err := di.InstallDependency(ctx, dep); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", dep.Name, err))
			continue
		}

		// Verify installation
		ok, err := di.CheckDependency(ctx, dep)
		if err != nil || !ok {
			errors = append(errors, fmt.Sprintf("%s: verification failed", dep.Name))
			continue
		}

		installed = append(installed, dep.Name)
	}

	// Track installed dependencies
	if err := di.trackInstalled(integrationID, installed); err != nil {
		// Non-fatal - just log
		if di.verbose {
			fmt.Printf("Warning: failed to track installation: %v\n", err)
		}
	}

	if len(errors) > 0 {
		return installed, fmt.Errorf("some installations failed: %s", strings.Join(errors, "; "))
	}

	return installed, nil
}

// trackInstalled records which dependencies were installed
func (di *DependencyInstaller) trackInstalled(integrationID string, packages []string) error {
	// Load existing dependencies
	var deps InstalledDependencies
	data, err := os.ReadFile(di.depsFile)
	if err == nil {
		yaml.Unmarshal(data, &deps)
	}

	if deps.Dependencies == nil {
		deps.Dependencies = make(map[string][]string)
	}

	// Add newly installed packages
	existing := deps.Dependencies[integrationID]
	for _, pkg := range packages {
		// Check if already tracked
		found := false
		for _, e := range existing {
			if e == pkg {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, pkg)
		}
	}
	deps.Dependencies[integrationID] = existing

	// Save
	data, err = yaml.Marshal(deps)
	if err != nil {
		return err
	}

	return os.WriteFile(di.depsFile, data, 0644)
}

// CheckSystemPrerequisites checks if system has necessary package managers
func (di *DependencyInstaller) CheckSystemPrerequisites() map[string]bool {
	prereqs := map[string]bool{
		"npm":  commandExists("npm"),
		"node": commandExists("node"),
	}
	return prereqs
}

// commandExists checks if a command is available
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// GetInstallInstructions returns manual installation instructions
func GetInstallInstructions(integrationID string) []string {
	deps, ok := IntegrationDependencies[integrationID]
	if !ok {
		return []string{}
	}

	var instructions []string
	instructions = append(instructions, "# Dependencies will be installed locally to .soulgate/")
	instructions = append(instructions, "")

	for _, dep := range deps {
		switch dep.Type {
		case "npm":
			instructions = append(instructions, fmt.Sprintf("npm install %s --prefix ~/.soulgate", dep.Package))
		case "binary":
			instructions = append(instructions, fmt.Sprintf("# Download %s to ~/.soulgate/bin/", dep.Package))
		}
	}

	return instructions
}

// GetNodeModulesPath returns the path to node_modules for use in NODE_PATH
func (di *DependencyInstaller) GetNodeModulesPath() string {
	return di.nodeModules
}

// GetBinPath returns the path to bin directory for use in PATH
func (di *DependencyInstaller) GetBinPath() string {
	return di.binDir
}
