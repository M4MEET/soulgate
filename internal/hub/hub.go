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

// PackageType identifies the user-facing category: skill, tool, or agent.
type PackageType string

const (
	TypeSkill PackageType = "skill"
	TypeTool  PackageType = "tool"
	TypeAgent PackageType = "agent"
)

// ToolKind identifies the implementation kind within the "tool" category.
type ToolKind string

const (
	KindPlugin    ToolKind = "plugin"
	KindMCP       ToolKind = "mcp"
	KindConnector ToolKind = "connector"
	KindScript    ToolKind = "script"
)

const (
	defaultRegistryURL = "https://raw.githubusercontent.com/M4MEET/soulgate-hub/main/registry.json"
	hubCacheTTL        = 30 * time.Minute
	hubInstalledFile   = "hub/installed.json"
	rawBaseURL         = "https://raw.githubusercontent.com/M4MEET/soulgate-hub/main/%ss/%s/"
)

// Package describes a single package in the remote registry.
type Package struct {
	Name        string      `json:"name"        yaml:"name"`
	Type        PackageType `json:"type"        yaml:"type"`
	Kind        ToolKind    `json:"kind,omitempty" yaml:"kind,omitempty"`
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
	Kind        ToolKind    `json:"kind,omitempty"`
	Version     string      `json:"version"`
	InstalledAt time.Time   `json:"installed_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Hub is the package manager for skills, tools, and agents.
type Hub struct {
	registryURL string
	cacheDir    string // ~/.soulgate/hub-cache/
	workDir     string // workspace root
	httpClient  *http.Client
}

// NewHub creates a Hub for the given workspace root directory.
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
	cached, err := h.readCache("registry.json")
	if err == nil {
		var reg PackageRegistry
		if json.Unmarshal(cached, &reg) == nil {
			h.migrateRegistryPackages(&reg)
			return &reg, nil
		}
	}

	req, err := http.NewRequest(http.MethodGet, h.registryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("hub: create request: %w", err)
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		if cached != nil {
			var reg PackageRegistry
			if json.Unmarshal(cached, &reg) == nil {
				h.migrateRegistryPackages(&reg)
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

	_ = h.writeCache("registry.json", data)

	h.migrateRegistryPackages(&reg)
	return &reg, nil
}

// migrateRegistryPackages converts old 6-type registry entries to the new 3-type system.
func (h *Hub) migrateRegistryPackages(reg *PackageRegistry) {
	for i := range reg.Packages {
		pkg := &reg.Packages[i]
		switch PackageType(pkg.Type) {
		case "plugin":
			pkg.Type = TypeTool
			pkg.Kind = KindPlugin
		case "mcp":
			pkg.Type = TypeTool
			pkg.Kind = KindMCP
		case "connector":
			pkg.Type = TypeTool
			pkg.Kind = KindConnector
		case "extension":
			pkg.Type = TypeTool
			pkg.Kind = KindScript
		}
	}
}

// Search returns packages whose name, description, or tags contain query.
// Results merge the remote registry with locally installed packages and
// workspace skills, de-duplicated by type:name with remote entries winning
// (they carry richer metadata). If the remote registry is unreachable, local
// results are returned instead of an error. An empty query matches everything.
func (h *Hub) Search(query string) ([]Package, error) {
	q := strings.ToLower(query)

	var results []Package
	seen := make(map[string]bool)

	if reg, err := h.FetchRegistry(); err == nil {
		for _, pkg := range reg.Packages {
			if matchesQuery(pkg, q) {
				results = append(results, pkg)
				seen[fmt.Sprintf("%s:%s", pkg.Type, pkg.Name)] = true
			}
		}
	}

	for _, pkg := range h.localPackages() {
		key := fmt.Sprintf("%s:%s", pkg.Type, pkg.Name)
		if !seen[key] && matchesQuery(pkg, q) {
			results = append(results, pkg)
			seen[key] = true
		}
	}

	return results, nil
}

// matchesQuery reports whether pkg matches the lowercased query.
// An empty query matches everything.
func matchesQuery(pkg Package, q string) bool {
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(pkg.Name), q) ||
		strings.Contains(strings.ToLower(pkg.Description), q) ||
		containsTag(pkg.Tags, q)
}

// Info returns details for a package identified by "type:name" or "type/name".
// It consults the remote registry first; if the package isn't there (or the
// registry is unreachable) it falls back to locally installed metadata.
func (h *Hub) Info(typeAndName string) (*Package, error) {
	pkgType, kind, name, err := parseTypeAndName(typeAndName)
	if err != nil {
		return nil, err
	}

	reg, regErr := h.FetchRegistry()
	if regErr == nil {
		for i := range reg.Packages {
			p := &reg.Packages[i]
			if p.Name == name && p.Type == pkgType {
				if pkgType == TypeTool && kind != "" && p.Kind != kind {
					continue
				}
				return p, nil
			}
		}
	}

	if pkg := h.localPackage(pkgType, kind, name); pkg != nil {
		return pkg, nil
	}

	if regErr != nil {
		return nil, fmt.Errorf("hub: package %q not installed locally and registry unavailable: %w", typeAndName, regErr)
	}
	return nil, fmt.Errorf("hub: package %q not found in registry or installed locally", typeAndName)
}

// Install downloads and installs a package identified by "type:name" or "type/name".
func (h *Hub) Install(typeAndName string) error {
	pkgType, kind, name, err := parseTypeAndName(typeAndName)
	if err != nil {
		return err
	}

	// Try to find in registry for metadata.
	var pkg *Package
	if reg, regErr := h.FetchRegistry(); regErr == nil {
		for i := range reg.Packages {
			p := &reg.Packages[i]
			if p.Name == name && p.Type == pkgType {
				pkg = p
				if kind != "" {
					kind = p.Kind // trust registry kind
				}
				break
			}
		}
	}

	version := "latest"
	if pkg != nil {
		version = pkg.Version
	}

	// If kind wasn't resolved from registry or input, default for tools.
	if pkgType == TypeTool && kind == "" {
		kind = KindPlugin
	}

	baseRaw := h.buildRawURL(pkgType, kind, name)

	switch pkgType {
	case TypeSkill:
		if err := h.installSkill(name, baseRaw, pkg); err != nil {
			return err
		}
	case TypeTool:
		if err := h.installTool(name, kind, baseRaw, pkg); err != nil {
			return err
		}
	case TypeAgent:
		if err := h.installAgent(name, baseRaw, pkg); err != nil {
			return err
		}
	default:
		return fmt.Errorf("hub: unknown package type %q", pkgType)
	}

	err = h.recordInstalled(InstalledPackage{
		Name:        name,
		Type:        pkgType,
		Kind:        kind,
		Version:     version,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	})
	h.logActivity("install", name, pkgType, kind, version, err)
	return err
}

// Uninstall removes a package identified by "type:name" or "type/name".
func (h *Hub) Uninstall(typeAndName string) error {
	pkgType, kind, name, err := parseTypeAndName(typeAndName)
	if err != nil {
		return err
	}

	// Look up installed record for kind if not specified.
	if pkgType == TypeTool && kind == "" {
		if pkgs, loadErr := h.loadInstalled(); loadErr == nil {
			for _, p := range pkgs {
				if p.Type == TypeTool && p.Name == name {
					kind = p.Kind
					break
				}
			}
		}
	}

	destDir := h.packageDir(pkgType, name)
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Errorf("hub: %s %q is not installed", pkgType, name)
	}

	if kind == KindMCP {
		if err := h.removeMCPEntry(name); err != nil {
			return fmt.Errorf("hub: remove MCP entry from config: %w", err)
		}
	}

	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("hub: remove directory %s: %w", destDir, err)
	}

	err = h.removeInstalled(pkgType, name)
	h.logActivity("uninstall", name, pkgType, kind, "", err)
	return err
}

// List returns all installed packages, reconciling the installed manifest
// with the package directories actually present on disk. Directories missing
// from the manifest are added; manifest entries whose directory no longer
// exists are dropped. The healed manifest is written back when it changed.
func (h *Hub) List() ([]InstalledPackage, error) {
	pkgs, err := h.loadInstalled()
	if err != nil {
		return nil, err
	}
	return h.reconcileInstalled(pkgs), nil
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

func (h *Hub) installAgent(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeAgent, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create agent dir: %w", err)
	}
	return h.downloadFile(baseRaw+"agent.yml", filepath.Join(destDir, "agent.yml"))
}

// installTool dispatches to the correct installer based on kind.
func (h *Hub) installTool(name string, kind ToolKind, baseRaw string, pkg *Package) error {
	switch kind {
	case KindPlugin:
		return h.installToolPlugin(name, baseRaw, pkg)
	case KindMCP:
		return h.installToolMCP(name, baseRaw, pkg)
	case KindConnector:
		return h.installToolConnector(name, baseRaw, pkg)
	case KindScript:
		return h.installToolScript(name, baseRaw, pkg)
	default:
		return fmt.Errorf("hub: unknown tool kind %q", kind)
	}
}

func (h *Hub) installToolPlugin(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeTool, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create tool dir: %w", err)
	}
	if err := h.downloadFile(baseRaw+"manifest.yml", filepath.Join(destDir, "manifest.yml")); err != nil {
		return err
	}
	if pkg != nil {
		for _, f := range pkg.Files {
			dest := filepath.Join(destDir, filepath.Base(f))
			_ = h.downloadFile(baseRaw+f, dest)
		}
	}
	return nil
}

func (h *Hub) installToolMCP(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeTool, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create tool dir: %w", err)
	}

	_ = h.downloadFile(baseRaw+"README.md", filepath.Join(destDir, "README.md"))

	mcpYMLPath := filepath.Join(destDir, "mcp.yml")
	_ = h.downloadFile(baseRaw+"mcp.yml", mcpYMLPath)

	return h.addMCPEntry(name, mcpYMLPath)
}

func (h *Hub) installToolConnector(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeTool, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create tool dir: %w", err)
	}
	_ = h.downloadFile(baseRaw+"README.md", filepath.Join(destDir, "README.md"))
	return h.downloadFile(baseRaw+"setup.md", filepath.Join(destDir, "setup.md"))
}

func (h *Hub) installToolScript(name, baseRaw string, pkg *Package) error {
	destDir := h.packageDir(TypeTool, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("hub: create tool dir: %w", err)
	}
	return h.downloadFile(baseRaw+"extension.sh", filepath.Join(destDir, "extension.sh"))
}

// ---- MCP config helpers ----

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

	entry := map[string]interface{}{
		"name":    name,
		"command": "npx",
		"args":    []string{"-y", fmt.Sprintf("@modelcontextprotocol/server-%s", name)},
	}

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

	mcp, _ := cfg["mcp"].(map[string]interface{})
	if mcp == nil {
		mcp = map[string]interface{}{}
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

	// Migrate old 6-type entries to 3-type system.
	migrated := false
	for i := range pkgs {
		p := &pkgs[i]
		switch PackageType(p.Type) {
		case "plugin":
			p.Type = TypeTool
			p.Kind = KindPlugin
			migrated = true
		case "mcp":
			p.Type = TypeTool
			p.Kind = KindMCP
			migrated = true
		case "connector":
			p.Type = TypeTool
			p.Kind = KindConnector
			migrated = true
		case "extension":
			p.Type = TypeTool
			p.Kind = KindScript
			migrated = true
		}
	}
	if migrated {
		_ = h.saveInstalled(pkgs)
	}

	return pkgs, nil
}

func (h *Hub) recordInstalled(ip InstalledPackage) error {
	pkgs, err := h.loadInstalled()
	if err != nil {
		pkgs = nil
	}

	found := false
	for i, p := range pkgs {
		if p.Type == ip.Type && p.Name == ip.Name {
			ip.InstalledAt = p.InstalledAt
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

// ---- local state reconciliation ----

// reconcileInstalled syncs the installed manifest with the package
// directories on disk under .soulgate/hub/{skills,tools,agents}/. Packages
// found on disk but missing from the manifest are added; manifest entries
// whose directory is gone are dropped. When anything changed, the healed
// manifest is saved (best effort, like the migration in loadInstalled).
func (h *Hub) reconcileInstalled(pkgs []InstalledPackage) []InstalledPackage {
	changed := false

	// Drop manifest entries whose directory no longer exists.
	kept := make([]InstalledPackage, 0, len(pkgs))
	seen := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		if _, err := os.Stat(h.packageDir(p.Type, p.Name)); os.IsNotExist(err) {
			changed = true
			continue
		}
		kept = append(kept, p)
		seen[fmt.Sprintf("%s:%s", p.Type, p.Name)] = true
	}

	// Add package directories on disk that are missing from the manifest.
	for _, pkgType := range []PackageType{TypeSkill, TypeTool, TypeAgent} {
		typeDir := filepath.Join(h.workDir, ".soulgate", "hub", string(pkgType)+"s")
		entries, err := os.ReadDir(typeDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			key := fmt.Sprintf("%s:%s", pkgType, e.Name())
			if seen[key] {
				continue
			}
			dir := filepath.Join(typeDir, e.Name())
			installedAt := time.Now()
			if info, infoErr := e.Info(); infoErr == nil {
				installedAt = info.ModTime()
			}
			var kind ToolKind
			if pkgType == TypeTool {
				kind = detectToolKind(dir)
			}
			kept = append(kept, InstalledPackage{
				Name:        e.Name(),
				Type:        pkgType,
				Kind:        kind,
				Version:     localVersion(dir),
				InstalledAt: installedAt,
				UpdatedAt:   installedAt,
			})
			seen[key] = true
			changed = true
		}
	}

	if changed {
		_ = h.saveInstalled(kept)
	}
	return kept
}

// detectToolKind infers a tool's kind from the marker files its installer
// would have written into the package directory.
func detectToolKind(dir string) ToolKind {
	markers := []struct {
		file string
		kind ToolKind
	}{
		{"manifest.yml", KindPlugin},
		{"mcp.yml", KindMCP},
		{"setup.md", KindConnector},
		{"extension.sh", KindScript},
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			return m.kind
		}
	}
	return ""
}

// localVersion reads a package version from manifest.yml or agent.yml in the
// package directory, or returns "unknown" when none is available.
func localVersion(dir string) string {
	for _, name := range []string{"manifest.yml", "agent.yml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var meta struct {
			Version string `yaml:"version"`
		}
		if yaml.Unmarshal(data, &meta) == nil && meta.Version != "" {
			return meta.Version
		}
	}
	return "unknown"
}

// localDescription reads a package description from the package directory:
// the first non-heading line of SKILL.md, or the description field of
// manifest.yml/agent.yml.
func (h *Hub) localDescription(pkgType PackageType, name string) string {
	dir := h.packageDir(pkgType, name)
	if desc := skillDescription(filepath.Join(dir, "SKILL.md")); desc != "" {
		return desc
	}
	for _, f := range []string{"manifest.yml", "agent.yml"} {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		var meta struct {
			Description string `yaml:"description"`
		}
		if yaml.Unmarshal(data, &meta) == nil && meta.Description != "" {
			return meta.Description
		}
	}
	return ""
}

// skillDescription returns the first non-empty, non-heading line of a
// SKILL.md file, or "" when the file is missing or has no such line.
func skillDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

// localPackage builds a registry-style Package from local install metadata,
// or nil when the package is not installed locally.
func (h *Hub) localPackage(pkgType PackageType, kind ToolKind, name string) *Package {
	installed, err := h.List()
	if err != nil {
		return nil
	}
	for _, ip := range installed {
		if ip.Type != pkgType || ip.Name != name {
			continue
		}
		if pkgType == TypeTool && kind != "" && ip.Kind != kind {
			continue
		}
		return &Package{
			Name:        ip.Name,
			Type:        ip.Type,
			Kind:        ip.Kind,
			Version:     ip.Version,
			Description: h.localDescription(ip.Type, ip.Name),
		}
	}
	return nil
}

// localPackages returns all locally installed packages (post-reconcile) plus
// workspace skills, as registry-style packages.
func (h *Hub) localPackages() []Package {
	var pkgs []Package
	if installed, err := h.List(); err == nil {
		for _, ip := range installed {
			pkgs = append(pkgs, Package{
				Name:        ip.Name,
				Type:        ip.Type,
				Kind:        ip.Kind,
				Version:     ip.Version,
				Description: h.localDescription(ip.Type, ip.Name),
			})
		}
	}
	pkgs = append(pkgs, h.workspaceSkills()...)
	return pkgs
}

// workspaceSkills returns skills defined directly in the workspace skills/
// directory (directories containing a SKILL.md).
func (h *Hub) workspaceSkills() []Package {
	skillsDir := filepath.Join(h.workDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	var pkgs []Package
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillMD := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:        e.Name(),
			Type:        TypeSkill,
			Version:     "local",
			Description: skillDescription(skillMD),
		})
	}
	return pkgs
}

// ---- path helpers ----

// packageDir returns the local directory for a given package.
//   - skill:X → .soulgate/hub/skills/X/
//   - tool:X  → .soulgate/hub/tools/X/
//   - agent:X → .soulgate/hub/agents/X/
func (h *Hub) packageDir(pkgType PackageType, name string) string {
	return filepath.Join(h.workDir, ".soulgate", "hub", string(pkgType)+"s", name)
}

// buildRawURL maps the new type+kind to the remote repo directory structure.
// The remote repo keeps its old layout: plugins/, mcps/, connectors/, extensions/, skills/, agents/
func (h *Hub) buildRawURL(pkgType PackageType, kind ToolKind, name string) string {
	var remoteDir string
	switch pkgType {
	case TypeSkill:
		remoteDir = "skill"
	case TypeAgent:
		remoteDir = "agent"
	case TypeTool:
		switch kind {
		case KindPlugin:
			remoteDir = "plugin"
		case KindMCP:
			remoteDir = "mcp"
		case KindConnector:
			remoteDir = "connector"
		case KindScript:
			remoteDir = "extension"
		default:
			remoteDir = "plugin"
		}
	default:
		remoteDir = string(pkgType)
	}
	return fmt.Sprintf(rawBaseURL, remoteDir, name)
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

// oldTypeMapping maps legacy 6-type names to new type + kind.
var oldTypeMapping = map[string]struct {
	Type PackageType
	Kind ToolKind
}{
	"plugin":    {TypeTool, KindPlugin},
	"mcp":       {TypeTool, KindMCP},
	"connector": {TypeTool, KindConnector},
	"extension": {TypeTool, KindScript},
}

// parseTypeAndName parses "type:name" or "type/name" into (PackageType, ToolKind, name).
// Old type names (plugin, mcp, connector, extension) are accepted for backward compat
// and mapped to TypeTool + appropriate kind.
func parseTypeAndName(s string) (PackageType, ToolKind, string, error) {
	var parts []string
	if strings.Contains(s, ":") {
		parts = strings.SplitN(s, ":", 2)
	} else if strings.Contains(s, "/") {
		parts = strings.SplitN(s, "/", 2)
	} else {
		return "", "", "", fmt.Errorf("hub: invalid package identifier %q — use type:name or type/name", s)
	}

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("hub: invalid package identifier %q", s)
	}

	typeName := parts[0]
	name := parts[1]

	// Check for backward-compat old type names.
	if mapping, ok := oldTypeMapping[typeName]; ok {
		return mapping.Type, mapping.Kind, name, nil
	}

	// Check new types.
	pkgType := PackageType(typeName)
	switch pkgType {
	case TypeSkill, TypeTool, TypeAgent:
		return pkgType, "", name, nil
	}

	return "", "", "", fmt.Errorf("hub: unknown package type %q (valid: skill, tool, agent)", typeName)
}

func containsTag(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
// Activity Log
// --------------------------------------------------------------------------

const activityLogFile = "hub/activity.jsonl"

// ActivityEntry records a hub action for auditing.
type ActivityEntry struct {
	Action    string    `json:"action"`
	Package   string    `json:"package"`
	Type      string    `json:"type"`
	Kind      string    `json:"kind,omitempty"`
	Version   string    `json:"version,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

func (h *Hub) logActivity(action, name string, pkgType PackageType, kind ToolKind, version string, err error) {
	path := filepath.Join(h.workDir, ".soulgate", activityLogFile)
	_ = os.MkdirAll(filepath.Dir(path), 0700)

	entry := ActivityEntry{
		Action:    action,
		Package:   name,
		Type:      string(pkgType),
		Kind:      string(kind),
		Version:   version,
		Timestamp: time.Now(),
	}
	if err != nil {
		entry.Error = err.Error()
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return
	}
	data = append(data, '\n')

	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if openErr != nil {
		return
	}
	f.Write(data)
	f.Close()
}

// ActivityLog returns the last n hub activity entries.
func (h *Hub) ActivityLog(limit int) []ActivityEntry {
	path := filepath.Join(h.workDir, ".soulgate", activityLogFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entries []ActivityEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var e ActivityEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries
}
