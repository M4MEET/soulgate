package hub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PackageType identifies the kind of hub package.
type PackageType string

const (
	TypeSkill     PackageType = "skill"
	TypePlugin    PackageType = "plugin"
	TypeAgent     PackageType = "agent"
	TypeMCP       PackageType = "mcp"
	TypeConnector PackageType = "connector"
	TypeExtension PackageType = "extension"
)

const (
	defaultRegistryURL = "https://raw.githubusercontent.com/M4MEET/soulgate-hub/main/registry.json"
	hubCacheTTL        = 30 * time.Minute
	hubInstalledFile   = "hub-installed.json"
	rawBaseURL         = "https://raw.githubusercontent.com/M4MEET/soulgate-hub/main/%ss/%s/"
)

// Package describes a single package in the remote registry.
type Package struct {
	Name        string      `json:"name"        yaml:"name"`
	Type        PackageType `json:"type"        yaml:"type"`
	Version     string      `json:"version"     yaml:"version"`
	Description string      `json:"description" yaml:"description"`
	Author      string      `json:"author"      yaml:"author"`
	Tags        []string    `json:"tags"        yaml:"tags"`
	Repository  string      `json:"repository"  yaml:"repository"`
	Files       []string    `json:"files"       yaml:"files"`
}

// PackageRegistry is the remote registry.json payload.
type PackageRegistry struct {
	Packages []Package `json:"packages"`
	Updated  string    `json:"updated"`
}

// InstalledPackage records a locally installed package.
type InstalledPackage struct {
	Name        string      `json:"name"`
	Type        PackageType `json:"type"`
	Version     string      `json:"version"`
	InstalledAt time.Time   `json:"installed_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Hub is the package manager for skills, plugins, agents, MCP servers,
// connectors, and extensions.
type Hub struct {
	registryURL string
	cacheDir    string // ~/.soulgate/hub-cache/
	workDir     string // workspace root
	httpClient  *http.Client
}

// NewHub creates a Hub for the given workspace root directory.
// cacheDir defaults to ~/.soulgate/hub-cache/, registryURL to the GitHub raw URL.
func NewHub(workDir string) *Hub {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".soulgate", "hub-cache")
	return &Hub{
		registryURL: defaultRegistryURL,
		cacheDir:    cacheDir,
		workDir:     workDir,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchRegistry downloads (or loads from cache) the remote registry.json.
func (h *Hub) FetchRegistry() (*PackageRegistry, error) {
	// Try cache first.
	cached, err := h.readCache("registry.json")
	if err == nil {
		var reg PackageRegistry
		if json.Unmarshal(cached, &reg) == nil {
			return &reg, nil
		}
	}

	// Fetch from network.
	req, err := http.NewRequest(http.MethodGet, h.registryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hub: create request: %w", err)
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		// Offline: return cached data even if stale.
		if cached != nil {
			var reg PackageRegistry
			if json.Unmarshal(cached, &reg) == nil {
				return &reg, nil
			}
		}
		return nil, fmt.Errorf("hub: fetch registry (offline?): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub: registry returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hub: read registry response: %w", err)
	}

	var reg PackageRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("hub: parse registry: %w", err)
	}

	// Persist to cache.
	_ = h.writeCache("registry.json", data)

	return &reg, nil
}

// Search returns packages whose name, description, or tags contain query
// (case-insensitive). An empty query returns all packages.
func (h *Hub) Search(query string) ([]Package, error) {
	reg, err := h.FetchRegistry()
	if err != nil {
		return nil, err
	}

	if query == "" {
		return reg.Packages, nil
	}

	q := strings.ToLower(query)
	var results []Package
	for _, pkg := range reg.Packages {
		if strings.Contains(strings.ToLower(pkg.Name), q) ||
			strings.Contains(strings.ToLower(pkg.Description), q) ||
			containsTag(pkg.Tags, q) {
			results = append(results, pkg)
		}
	}
	return results, nil
}

// Info returns details for a package identified by "type:name" or "type/name".
func (h *Hub) Info(typeAndName string) (*Package, error) {
	pkgType, name, err := parseTypeAndName(typeAndName)
	if err != nil {
		return nil, err
	}

	reg, err := h.FetchRegistry()
	if err != nil {
		return nil, err
	}

	for i := range reg.Packages {
		if reg.Packages[i].Type == pkgType && reg.Packages[i].Name == name {
			return &reg.Packages[i], nil
		}
	}

	return nil, fmt.Errorf("hub: package %q not found in registry", typeAndName)
}

// Install downloads and installs a package identified by "type:name" or "type/name".
func (h *Hub) Install(typeAndName string) error {
	pkgType, name, err := parseTypeAndName(typeAndName)
	if err != nil {
		return err
	}

	// Try to find in registry for metadata.  Non-fatal if registry is unreachable.
	var pkg *Package
	if reg, regErr := h.FetchRegistry(); regErr == nil {
		for i := range reg.Packages {
			if reg.Packages[i].Type == pkgType && reg.Packages[i].Name == name {
				pkg = &reg.Packages[i]
				break
			}
		}
	}

	version := "latest"
	if pkg != nil {
		version = pkg.Version
	}

	baseRaw := fmt.Sprintf(rawBaseURL, string(pkgType), name)

	switch pkgType {
	case TypeSkill:
		if err := h.installSkill(name, baseRaw, pkg); err != nil {
			return err
		}
	case TypePlugin:
		if err := h.installPlugin(name, baseRaw, pkg); err != nil {
			return err
		}
	case TypeAgent:
		if err := h.installAgent(name, baseRaw, pkg); err != nil {
			return err
		}
	case TypeMCP:
		if err := h.installMCP(name, baseRaw, pkg); err != nil {
			return err
		}
	case TypeConnector:
		if err := h.installConnector(name, baseRaw, pkg); err != nil {
			return err
		}
	case TypeExtension:
		if err := h.installExtension(name, baseRaw, pkg); err != nil {
			return err
		}
	default:
		return fmt.Errorf("hub: unknown package type %q", pkgType)
	}

	// Record in installed manifest.
	return h.recordInstalled(InstalledPackage{
		Name:        name,
		Type:        pkgType,
		Version:     version,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	})
}

// Uninstall removes a package identified by "type:name" or "type/name".
func (h *Hub) Uninstall(typeAndName string) error {
	pkgType, name, err := parseTypeAndName(typeAndName)
	if err != nil {
		return err
	}

	destDir := h.packageDir(pkgType, name)
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Errorf("hub: %s %q is not installed", pkgType, name)
	}

	if pkgType == TypeMCP {
		if err := h.removeMCPEntry(name); err != nil {
			return fmt.Errorf("hub: remove MCP entry from config: %w", err)
		}
	}

	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("hub: remove directory %s: %w", destDir, err)
	}

	return h.removeInstalled(pkgType, name)
}

// List returns all packages recorded in the installed manifest.
func (h *Hub) List() ([]InstalledPackage, error) {
	return h.loadInstalled()
}

// Update re-installs all packages that appear in the remote registry.
func (h *Hub) Update() error {
	installed, err := h.loadInstalled()
	if err != nil {
		return err
	}

	var errs []string
	for _, ip := range installed {
		key := fmt.Sprintf("%s:%s", ip.Type, ip.Name)
		if err := h.Install(key); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", key, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("hub: update errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// ---- type-specific install helpers ----

func (h *Hub) installSkill(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeSkill, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create skills dir: %w", err)
	}
	dest := filepath.Join(destDir, "SKILL.md")
	return h.downloadFile(baseRaw+"SKILL.md", dest)
}

func (h *Hub) installPlugin(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypePlugin, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create plugin dir: %w", err)
	}
	// Always try manifest.yml.
	if err := h.downloadFile(baseRaw+"manifest.yml", filepath.Join(destDir, "manifest.yml")); err != nil {
		return err
	}
	// Download declared files if available.
	if pkg != nil {
		for _, f := range pkg.Files {
			dest := filepath.Join(destDir, filepath.Base(f))
			_ = h.downloadFile(baseRaw+f, dest) // best-effort for extra files
		}
	}
	return nil
}

func (h *Hub) installAgent(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeAgent, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create agent dir: %w", err)
	}
	return h.downloadFile(baseRaw+"agent.yml", filepath.Join(destDir, "agent.yml"))
}

func (h *Hub) installMCP(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeMCP, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create mcp dir: %w", err)
	}

	// Download the setup guide for reference.
	_ = h.downloadFile(baseRaw+"README.md", filepath.Join(destDir, "README.md"))

	// Download mcp.yml for config entry data.
	mcpYMLPath := filepath.Join(destDir, "mcp.yml")
	_ = h.downloadFile(baseRaw+"mcp.yml", mcpYMLPath)

	// Inject into .soulgate/config.yml.
	return h.addMCPEntry(name, mcpYMLPath)
}

func (h *Hub) installConnector(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeConnector, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create connector dir: %w", err)
	}
	_ = h.downloadFile(baseRaw+"README.md", filepath.Join(destDir, "README.md"))
	return h.downloadFile(baseRaw+"setup.md", filepath.Join(destDir, "setup.md"))
}

func (h *Hub) installExtension(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeExtension, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create extension dir: %w", err)
	}
	return h.downloadFile(baseRaw+"extension.sh", filepath.Join(destDir, "extension.sh"))
}

// ---- MCP config helpers ----

// mcpYMLEntry is used to unmarshal a downloaded mcp.yml file.
type mcpYMLEntry struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
	WorkDir string            `yaml:"work_dir"`
}

func (h *Hub) addMCPEntry(name, mcpYMLPath string) error {
	configPath := filepath.Join(h.workDir, ".soulgate", "config.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("hub: read config.yml: %w", err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("hub: parse config.yml: %w", err)
	}

	// Build the new MCP server entry.
	entry := map[string]interface{}{
		"name":    name,
		"command": "npx",
		"args":    []string{"-y", fmt.Sprintf("@modelcontextprotocol/server-%s", name)},
	}

	// If we have a downloaded mcp.yml, use it to override defaults.
	if ymlData, err := os.ReadFile(mcpYMLPath); err == nil {
		var mcpEntry mcpYMLEntry
		if yaml.Unmarshal(ymlData, &mcpEntry) == nil {
			if mcpEntry.Command != "" {
				entry["command"] = mcpEntry.Command
			}
			if len(mcpEntry.Args) > 0 {
				entry["args"] = mcpEntry.Args
			}
			if len(mcpEntry.Env) > 0 {
				entry["env"] = mcpEntry.Env
			}
			if mcpEntry.WorkDir != "" {
				entry["work_dir"] = mcpEntry.WorkDir
			}
		}
	}

	// Ensure mcp.servers exists.
	mcp, _ := cfg["mcp"].(map[string]interface{})
	if mcp == nil {
		mcp = map[string]interface{}{}
	}

	servers, _ := mcp["servers"].([]interface{})
	// Remove existing entry with same name if present.
	filtered := make([]interface{}, 0, len(servers))
	for _, s := range servers {
		if sm, ok := s.(map[string]interface{}); ok {
			if sm["name"] != name {
				filtered = append(filtered, s)
			}
		}
	}
	filtered = append(filtered, entry)
	mcp["servers"] = filtered
	cfg["mcp"] = mcp

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("hub: marshal config.yml: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0600); err != nil {
		return fmt.Errorf("hub: write config.yml: %w", err)
	}

	return nil
}

func (h *Hub) removeMCPEntry(name string) error {
	configPath := filepath.Join(h.workDir, ".soulgate", "config.yml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config may not exist; nothing to remove.
		return nil
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("hub: parse config.yml: %w", err)
	}

	mcp, ok := cfg["mcp"].(map[string]interface{})
	if !ok {
		return nil
	}

	servers, _ := mcp["servers"].([]interface{})
	filtered := make([]interface{}, 0, len(servers))
	for _, s := range servers {
		if sm, ok := s.(map[string]interface{}); ok {
			if sm["name"] != name {
				filtered = append(filtered, s)
			}
		}
	}
	mcp["servers"] = filtered
	cfg["mcp"] = mcp

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("hub: marshal config.yml: %w", err)
	}

	return os.WriteFile(configPath, out, 0600)
}

// ---- installed manifest ----

func (h *Hub) installedPath() string {
	return filepath.Join(h.workDir, ".soulgate", hubInstalledFile)
}

func (h *Hub) loadInstalled() ([]InstalledPackage, error) {
	data, err := os.ReadFile(h.installedPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("hub: read installed manifest: %w", err)
	}
	var pkgs []InstalledPackage
	if err := json.Unmarshal(data, &pkgs); err != nil {
		return nil, fmt.Errorf("hub: parse installed manifest: %w", err)
	}
	return pkgs, nil
}

func (h *Hub) recordInstalled(ip InstalledPackage) error {
	pkgs, err := h.loadInstalled()
	if err != nil {
		pkgs = nil
	}

	// Replace or append.
	found := false
	for i, p := range pkgs {
		if p.Type == ip.Type && p.Name == ip.Name {
			ip.InstalledAt = p.InstalledAt // preserve original install time
			pkgs[i] = ip
			found = true
			break
		}
	}
	if !found {
		pkgs = append(pkgs, ip)
	}

	return h.saveInstalled(pkgs)
}

func (h *Hub) removeInstalled(pkgType PackageType, name string) error {
	pkgs, err := h.loadInstalled()
	if err != nil {
		return err
	}

	filtered := pkgs[:0]
	for _, p := range pkgs {
		if !(p.Type == pkgType && p.Name == name) {
			filtered = append(filtered, p)
		}
	}

	return h.saveInstalled(filtered)
}

func (h *Hub) saveInstalled(pkgs []InstalledPackage) error {
	dir := filepath.Dir(h.installedPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("hub: create .soulgate dir: %w", err)
	}
	data, err := json.MarshalIndent(pkgs, "", "  ")
	if err != nil {
		return fmt.Errorf("hub: marshal installed manifest: %w", err)
	}
	return os.WriteFile(h.installedPath(), data, 0600)
}

// ---- path helpers ----

// packageDir returns the local directory for a given package.
// The directory layout mirrors how each type is stored in the workspace:
//   - skill:X     → skills/X/
//   - plugin:X    → plugins/X/
//   - agent:X     → agents/X/
//   - mcp:X       → mcp/X/          (also patched into config.yml)
//   - connector:X → connectors/X/
//   - extension:X → extensions/X/
func (h *Hub) packageDir(pkgType PackageType, name string) string {
	return filepath.Join(h.workDir, string(pkgType)+"s", name)
}

// ---- download helpers ----

func (h *Hub) downloadFile(url, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("hub: create download request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hub: download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hub: download %s: HTTP %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("hub: create directory for %s: %w", destPath, err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("hub: create file %s: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("hub: write file %s: %w", destPath, err)
	}

	return nil
}

// ---- cache helpers ----

func (h *Hub) cacheFile(name string) string {
	return filepath.Join(h.cacheDir, name)
}

func (h *Hub) readCache(name string) ([]byte, error) {
	path := h.cacheFile(name)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > hubCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	return os.ReadFile(path)
}

func (h *Hub) writeCache(name string, data []byte) error {
	if err := os.MkdirAll(h.cacheDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(h.cacheFile(name), data, 0644)
}

// ---- parsing helpers ----

// parseTypeAndName parses "type:name" or "type/name" into its components.
func parseTypeAndName(s string) (PackageType, string, error) {
	var parts []string
	if strings.Contains(s, ":") {
		parts = strings.SplitN(s, ":", 2)
	} else if strings.Contains(s, "/") {
		parts = strings.SplitN(s, "/", 2)
	} else {
		return "", "", fmt.Errorf("hub: invalid package identifier %q — use type:name or type/name", s)
	}

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("hub: invalid package identifier %q", s)
	}

	pkgType := PackageType(parts[0])
	switch pkgType {
	case TypeSkill, TypePlugin, TypeAgent, TypeMCP, TypeConnector, TypeExtension:
	default:
		return "", "", fmt.Errorf("hub: unknown package type %q", parts[0])
	}

	return pkgType, parts[1], nil
}

func containsTag(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}
