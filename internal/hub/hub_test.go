package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unreachableURL guarantees the remote registry fetch fails fast.
const unreachableURL = "http://127.0.0.1:1/registry.json"

// newTestHub returns a Hub rooted in a temp workspace with an isolated cache
// directory and an unreachable registry (tests opt in to a fake remote via
// httptest where needed).
func newTestHub(t *testing.T) *Hub {
	t.Helper()
	workDir := t.TempDir()
	h := NewHub(workDir)
	h.cacheDir = filepath.Join(workDir, "hub-cache")
	h.registryURL = unreachableURL
	return h
}

// writeHubSkill creates an installed skill directory with a SKILL.md.
func writeHubSkill(t *testing.T, h *Hub, name, content string) {
	t.Helper()
	dir := h.packageDir(TypeSkill, name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644))
}

// writeHubTool creates an installed tool directory with a manifest.yml.
func writeHubTool(t *testing.T, h *Hub, name, manifest string) {
	t.Helper()
	dir := h.packageDir(TypeTool, name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(manifest), 0644))
}

// writeWorkspaceSkill creates a skill under <workDir>/skills/.
func writeWorkspaceSkill(t *testing.T, h *Hub, name, content string) {
	t.Helper()
	dir := filepath.Join(h.workDir, "skills", name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644))
}

// fakeRegistry starts an httptest server serving the given registry payload
// and points the hub at it.
func fakeRegistry(t *testing.T, h *Hub, reg PackageRegistry) {
	t.Helper()
	data, err := json.Marshal(reg)
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	h.registryURL = srv.URL
}

func findInstalled(pkgs []InstalledPackage, pkgType PackageType, name string) *InstalledPackage {
	for i := range pkgs {
		if pkgs[i].Type == pkgType && pkgs[i].Name == name {
			return &pkgs[i]
		}
	}
	return nil
}

func findPackage(pkgs []Package, pkgType PackageType, name string) *Package {
	for i := range pkgs {
		if pkgs[i].Type == pkgType && pkgs[i].Name == name {
			return &pkgs[i]
		}
	}
	return nil
}

func TestListHealsManifestFromDisk(t *testing.T) {
	h := newTestHub(t)

	// Packages on disk, but no installed.json at all.
	writeHubSkill(t, h, "self-improve", "# Self-Improvement Agent\n\nWhen asked to improve yourself, do so.\n")
	writeHubTool(t, h, "git-tools", "name: git-tools\nversion: 1.2.3\ndescription: Git helpers\n")

	pkgs, err := h.List()
	require.NoError(t, err)
	require.Len(t, pkgs, 2)

	skill := findInstalled(pkgs, TypeSkill, "self-improve")
	require.NotNil(t, skill)
	assert.Equal(t, "unknown", skill.Version)
	assert.False(t, skill.InstalledAt.IsZero())

	tool := findInstalled(pkgs, TypeTool, "git-tools")
	require.NotNil(t, tool)
	assert.Equal(t, KindPlugin, tool.Kind)
	assert.Equal(t, "1.2.3", tool.Version)

	// The manifest was healed on disk.
	data, err := os.ReadFile(h.installedPath())
	require.NoError(t, err)
	var saved []InstalledPackage
	require.NoError(t, json.Unmarshal(data, &saved))
	require.Len(t, saved, 2)
	assert.NotNil(t, findInstalled(saved, TypeSkill, "self-improve"))
	assert.NotNil(t, findInstalled(saved, TypeTool, "git-tools"))
}

func TestListDropsStaleManifestEntries(t *testing.T) {
	h := newTestHub(t)

	writeHubSkill(t, h, "debugging", "# Debugging\n\nDebug things.\n")

	// Manifest records a package whose directory no longer exists.
	require.NoError(t, h.saveInstalled([]InstalledPackage{
		{Name: "debugging", Type: TypeSkill, Version: "1.0.0", InstalledAt: time.Now(), UpdatedAt: time.Now()},
		{Name: "ghost", Type: TypeSkill, Version: "0.1.0", InstalledAt: time.Now(), UpdatedAt: time.Now()},
	}))

	pkgs, err := h.List()
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "debugging", pkgs[0].Name)
	assert.Equal(t, "1.0.0", pkgs[0].Version) // manifest entry preserved

	// The manifest was healed on disk.
	data, err := os.ReadFile(h.installedPath())
	require.NoError(t, err)
	var saved []InstalledPackage
	require.NoError(t, json.Unmarshal(data, &saved))
	require.Len(t, saved, 1)
	assert.Equal(t, "debugging", saved[0].Name)
}

func TestInfoLocalFallback(t *testing.T) {
	h := newTestHub(t) // registry unreachable

	writeHubSkill(t, h, "self-improve",
		"# Self-Improvement Agent\n\nWhen asked to improve yourself, run soulgate doctor.\n")

	pkg, err := h.Info("skill:self-improve")
	require.NoError(t, err)
	assert.Equal(t, "self-improve", pkg.Name)
	assert.Equal(t, TypeSkill, pkg.Type)
	assert.Equal(t, "unknown", pkg.Version)
	assert.Equal(t, "When asked to improve yourself, run soulgate doctor.", pkg.Description)

	// Neither remote nor local still errors.
	_, err = h.Info("skill:does-not-exist")
	require.Error(t, err)
}

func TestSearchOfflineLocalOnly(t *testing.T) {
	h := newTestHub(t) // registry unreachable

	writeHubSkill(t, h, "debugging", "# Debugging\n\nSystematic debugging workflow.\n")
	writeWorkspaceSkill(t, h, "spotify-management", "# Skill: spotify-management\n\nControl Spotify on macOS.\n")

	// Empty query returns everything local instead of an error.
	all, err := h.Search("")
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.NotNil(t, findPackage(all, TypeSkill, "debugging"))

	ws := findPackage(all, TypeSkill, "spotify-management")
	require.NotNil(t, ws)
	assert.Equal(t, "Control Spotify on macOS.", ws.Description)

	// Query filters local results.
	results, err := h.Search("spotify")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "spotify-management", results[0].Name)

	results, err = h.Search("no-such-package")
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchDedupPrefersRemote(t *testing.T) {
	h := newTestHub(t)
	fakeRegistry(t, h, PackageRegistry{
		Packages: []Package{
			{
				Name:        "self-improve",
				Type:        TypeSkill,
				Version:     "2.0.0",
				Description: "Remote description with richer metadata",
				Author:      "soulgate",
			},
			{
				Name:        "kubernetes-ops",
				Type:        TypeSkill,
				Version:     "1.0.0",
				Description: "Manage Kubernetes clusters",
			},
		},
	})

	// self-improve is both remote and locally installed.
	writeHubSkill(t, h, "self-improve", "# Self-Improvement Agent\n\nLocal description.\n")
	// debugging is local-only.
	writeHubSkill(t, h, "debugging", "# Debugging\n\nSystematic debugging workflow.\n")

	all, err := h.Search("")
	require.NoError(t, err)
	require.Len(t, all, 3) // self-improve deduped, kubernetes-ops + debugging

	si := findPackage(all, TypeSkill, "self-improve")
	require.NotNil(t, si)
	assert.Equal(t, "2.0.0", si.Version)
	assert.Equal(t, "Remote description with richer metadata", si.Description)
	assert.Equal(t, "soulgate", si.Author)

	assert.NotNil(t, findPackage(all, TypeSkill, "kubernetes-ops"))
	assert.NotNil(t, findPackage(all, TypeSkill, "debugging"))
}
