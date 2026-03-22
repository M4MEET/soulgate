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
	Short: "Manage skills, plugins, agents, MCP servers, and extensions",
	Long: `SoulHub is the community package manager for SoulGate.

Browse, install, and manage skills, plugins, agents, MCP servers,
connectors, and extensions from the community registry.`,
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

Package type must be one of: skill, plugin, agent, mcp, connector, extension

Examples:
  soulgate hub install skill:kubernetes-ops
  soulgate hub install plugin:git-tools
  soulgate hub install mcp:github
  soulgate hub install agent:code-reviewer`,
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
  soulgate hub uninstall mcp:github`,
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
  soulgate hub info mcp:github`,
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
	fmt.Println(colorBold("Package types:"))
	fmt.Println("  " + colorAccent("skill") + "      - Instruction sets (SKILL.md)")
	fmt.Println("  " + colorAccent("plugin") + "     - WASM extensions (manifest.yml)")
	fmt.Println("  " + colorAccent("agent") + "      - Agent definitions (agent.yml)")
	fmt.Println("  " + colorAccent("mcp") + "        - MCP servers (patched into config.yml)")
	fmt.Println("  " + colorAccent("connector") + "  - Integration setup guides")
	fmt.Println("  " + colorAccent("extension") + "  - Shell scripts and hooks")
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
		fmt.Printf("  %s %s\n", colorAccent(fmt.Sprintf("%-10s", string(pkg.Type))), colorBold(pkg.Name))
		if pkg.Description != "" {
			fmt.Printf("           %s\n", colorMuted(pkg.Description))
		}
		if pkg.Version != "" {
			fmt.Printf("           %s\n", colorMuted("v"+pkg.Version))
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
		fmt.Printf("  %s %s/%s\n",
			colorSuccess("*"),
			colorAccent(string(p.Type)),
			colorBold(p.Name),
		)
		if p.Version != "" {
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

	fmt.Printf("%s %s\n\n", colorAccentBright(string(pkg.Type)), colorBold(pkg.Name))
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

// newHub creates a Hub for the current workspace.
// Falls back to the current working directory when no workspace is found.
func newHub() *hub.Hub {
	workDir := resolveWorkDir()
	return hub.NewHub(workDir)
}

// resolveWorkDir returns the workspace root, walking up from cwd.
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

// printPostInstallHint prints a type-specific hint after successful install.
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
	case "plugin":
		fmt.Println(colorMuted("  Plugin installed — restart soulgate to load it."))
	}
}

// renderStars renders a float rating as star characters.
func renderStars(rating float64) string {
	n := int(rating)
	stars := strings.Repeat("*", n)
	return fmt.Sprintf("%s %.1f", stars, rating)
}
