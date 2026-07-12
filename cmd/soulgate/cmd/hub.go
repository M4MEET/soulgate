package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/hub"
	"github.com/spf13/cobra"
)

// hubCmd is the root hub command.
var hubCmd = &cobra.Command{
	Use:   "hub",
	Short: "Manage skills, tools, and agents",
	Long: `SoulHub is the community package manager for SoulGate.

Browse, install, and manage skills, tools, and agents from the community registry.

Categories:
  skill  — Behavioral instructions (SKILL.md)
  tool   — Capabilities: plugins, MCP servers, connectors, scripts
  agent  — Pre-configured agent templates (agent.yml)

Legacy type names (plugin, mcp, connector, extension) are still accepted
and mapped to tool with the appropriate kind.`,
	Run: func(cmd *cobra.Command, args []string) {
		showHubOverview()
	},
}

var hubSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the registry",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHubSearch(strings.Join(args, " "))
	},
}

var hubInstallCmd = &cobra.Command{
	Use:   "install <type:name>",
	Short: "Install a package from the hub",
	Long: `Install a package from the SoulHub registry.

Package type must be one of: skill, tool, agent
(Legacy names plugin, mcp, connector, extension are also accepted)

Examples:
  soulgate hub install skill:kubernetes-ops
  soulgate hub install tool:git-tools
  soulgate hub install agent:code-reviewer
  soulgate hub install plugin:example     (backward compat → tool with kind=plugin)
  soulgate hub install mcp:github         (backward compat → tool with kind=mcp)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHubInstall(args[0])
	},
}

var hubUninstallCmd = &cobra.Command{
	Use:   "uninstall <type:name>",
	Short: "Remove an installed package",
	Long: `Remove an installed package from the workspace.

Examples:
  soulgate hub uninstall skill:kubernetes-ops
  soulgate hub uninstall tool:github
  soulgate hub uninstall mcp:github       (backward compat)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHubUninstall(args[0])
	},
}

var hubListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHubList()
	},
}

var hubUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all installed packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHubUpdate()
	},
}

var hubInfoCmd = &cobra.Command{
	Use:   "info <type:name>",
	Short: "Show details about a package",
	Long: `Display registry information about a specific package.

Examples:
  soulgate hub info skill:kubernetes-ops
  soulgate hub info tool:github`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHubInfo(args[0])
	},
}

func init() {
	rootCmd.AddCommand(hubCmd)
	hubCmd.AddCommand(hubSearchCmd)
	hubCmd.AddCommand(hubInstallCmd)
	hubCmd.AddCommand(hubUninstallCmd)
	hubCmd.AddCommand(hubListCmd)
	hubCmd.AddCommand(hubUpdateCmd)
	hubCmd.AddCommand(hubInfoCmd)
}

// showHubOverview prints the hub help banner.
func showHubOverview() {
	fmt.Println(colorAccentBright("SoulHub — Community Package Manager"))
	fmt.Println()
	fmt.Println(colorBold("Categories:"))
	fmt.Println("  " + colorAccent("skill") + "  - Behavioral instructions (SKILL.md)")
	fmt.Println("  " + colorAccent("tool") + "   - Capabilities: plugins, MCP servers, connectors, scripts")
	fmt.Println("  " + colorAccent("agent") + "  - Pre-configured agent templates (agent.yml)")
	fmt.Println()
	fmt.Println(colorMuted("  Legacy names (plugin, mcp, connector, extension) are still accepted."))
	fmt.Println()
	fmt.Println(colorBold("Commands:"))
	fmt.Println("  " + colorMuted("soulgate hub search <query>        ") + "Search registry")
	fmt.Println("  " + colorMuted("soulgate hub install <type:name>   ") + "Install a package")
	fmt.Println("  " + colorMuted("soulgate hub uninstall <type:name> ") + "Remove a package")
	fmt.Println("  " + colorMuted("soulgate hub list                  ") + "List installed packages")
	fmt.Println("  " + colorMuted("soulgate hub update                ") + "Update all packages")
	fmt.Println("  " + colorMuted("soulgate hub info <type:name>      ") + "Show package details")
	fmt.Println()
}

// formatTypeKind returns a display label like "tool (mcp)" or just "skill".
func formatTypeKind(pkgType hub.PackageType, kind hub.ToolKind) string {
	if pkgType == hub.TypeTool && kind != "" {
		return fmt.Sprintf("%s (%s)", pkgType, kind)
	}
	return string(pkgType)
}

// runHubSearch searches the registry and prints results.
func runHubSearch(query string) error {
	h := newHub()
	results, err := h.Search(query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println(colorMuted("No packages found matching: " + query))
		return nil
	}

	fmt.Printf(colorAccentBright("Search results for %q\n\n"), query)
	for _, pkg := range results {
		label := formatTypeKind(pkg.Type, pkg.Kind)
		fmt.Printf("  %s %s\n", colorAccent(fmt.Sprintf("%-16s", label)), colorBold(pkg.Name))
		if pkg.Description != "" {
			fmt.Printf("                   %s\n", colorMuted(pkg.Description))
		}
		if pkg.Version != "" && pkg.Version != "unknown" && pkg.Version != "local" {
			fmt.Printf("                   %s\n", colorMuted("v"+pkg.Version))
		} else if pkg.Version == "local" {
			fmt.Printf("                   %s\n", colorMuted("workspace skill"))
		}
		fmt.Println()
	}
	fmt.Printf(colorMuted("Found %d package(s)\n"), len(results))
	return nil
}

// runHubInstall installs a single package.
func runHubInstall(typeAndName string) error {
	h := newHub()
	fmt.Printf("Installing %s...\n", colorAccent(typeAndName))

	if err := h.Install(typeAndName); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Println(colorSuccess("Installed " + typeAndName))
	printPostInstallHint(typeAndName)
	return nil
}

// runHubUninstall removes an installed package.
func runHubUninstall(typeAndName string) error {
	h := newHub()
	fmt.Printf("Removing %s...\n", colorAccent(typeAndName))

	if err := h.Uninstall(typeAndName); err != nil {
		return fmt.Errorf("uninstall failed: %w", err)
	}

	fmt.Println(colorSuccess("Removed " + typeAndName))
	return nil
}

// runHubList prints all installed packages.
func runHubList() error {
	h := newHub()
	pkgs, err := h.List()
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}

	if len(pkgs) == 0 {
		fmt.Println(colorMuted("No packages installed."))
		fmt.Println(colorMuted("Install one with: soulgate hub install <type:name>"))
		return nil
	}

	fmt.Println(colorAccentBright("Installed packages\n"))
	for _, p := range pkgs {
		label := formatTypeKind(p.Type, p.Kind)
		fmt.Printf("  %s %s / %s\n",
			colorSuccess("*"),
			colorAccent(label),
			colorBold(p.Name),
		)
		if p.Version != "" && p.Version != "unknown" {
			fmt.Printf("    v%s", p.Version)
		}
		fmt.Printf("  installed %s\n", p.InstalledAt.Format("2006-01-02"))
	}
	fmt.Printf("\n%s\n", colorMuted(fmt.Sprintf("Total: %d package(s)", len(pkgs))))
	return nil
}

// runHubUpdate updates all installed packages.
func runHubUpdate() error {
	h := newHub()

	pkgs, err := h.List()
	if err != nil {
		return fmt.Errorf("failed to read installed packages: %w", err)
	}
	if len(pkgs) == 0 {
		fmt.Println(colorMuted("No packages installed — nothing to update."))
		return nil
	}

	fmt.Println(colorAccent("Checking for updates..."))
	if err := h.Update(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println(colorSuccess(fmt.Sprintf("Updated %d package(s)", len(pkgs))))
	return nil
}

// runHubInfo shows registry details for a package.
func runHubInfo(typeAndName string) error {
	h := newHub()
	pkg, err := h.Info(typeAndName)
	if err != nil {
		return fmt.Errorf("info failed: %w", err)
	}

	label := formatTypeKind(pkg.Type, pkg.Kind)
	fmt.Printf("%s %s\n\n", colorAccentBright(label), colorBold(pkg.Name))
	if pkg.Description != "" {
		fmt.Printf("  %s\n\n", pkg.Description)
	}
	if pkg.Version != "" {
		fmt.Printf("  Version:    %s\n", colorMuted(pkg.Version))
	}
	if pkg.Author != "" {
		fmt.Printf("  Author:     %s\n", colorMuted(pkg.Author))
	}
	if pkg.Repository != "" {
		fmt.Printf("  Repository: %s\n", colorMuted(pkg.Repository))
	}
	if len(pkg.Tags) > 0 {
		fmt.Printf("  Tags:       %s\n", colorMuted(strings.Join(pkg.Tags, ", ")))
	}
	if len(pkg.Files) > 0 {
		fmt.Printf("  Files:      %s\n", colorMuted(strings.Join(pkg.Files, ", ")))
	}
	fmt.Println()
	fmt.Printf("  Install: %s\n", colorMuted(fmt.Sprintf("soulgate hub install %s:%s", pkg.Type, pkg.Name)))
	return nil
}

// ---- helpers ----

func newHub() *hub.Hub {
	workDir := resolveWorkDir()
	return hub.NewHub(workDir)
}

func resolveWorkDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	current := cwd
	for {
		if _, err := os.Stat(filepath.Join(current, ".soulgate")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return cwd
}

func printPostInstallHint(typeAndName string) {
	if !strings.Contains(typeAndName, ":") && !strings.Contains(typeAndName, "/") {
		return
	}
	var pkgType string
	if idx := strings.IndexAny(typeAndName, ":/"); idx != -1 {
		pkgType = typeAndName[:idx]
	}
	switch pkgType {
	case "mcp":
		fmt.Println(colorMuted("  MCP server entry added to .soulgate/config.yml"))
		fmt.Println(colorMuted("  Restart soulgate for changes to take effect."))
	case "skill":
		fmt.Println(colorMuted("  Skill installed — it will be available on next run."))
	case "tool", "plugin":
		fmt.Println(colorMuted("  Tool installed — restart soulgate to load it."))
	case "agent":
		fmt.Println(colorMuted("  Agent template installed — use it with soulgate agent."))
	}
}

func renderStars(rating float64) string {
	n := int(rating)
	stars := strings.Repeat("*", n)
	return fmt.Sprintf("%s %.1f", stars, rating)
}
