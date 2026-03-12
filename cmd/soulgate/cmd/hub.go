package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/hub"
	"github.com/spf13/cobra"
)

var hubCmd = &cobra.Command{
	Use:   "hub",
	Short: "Access SoulHub community marketplace",
	Long: `SoulHub is the community marketplace for plugins, skills, and automations.

Browse, install, and manage community contributions.`,
	Run: func(cmd *cobra.Command, args []string) {
		showHubOverview()
	},
}

var hubPluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Browse available plugins",
	Run: func(cmd *cobra.Command, args []string) {
		listPlugins(cmd.Context())
	},
}

var hubSkillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Browse available skills",
	Run: func(cmd *cobra.Command, args []string) {
		listSkills(cmd.Context())
	},
}

var hubSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the hub",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		searchHub(cmd.Context(), strings.Join(args, " "))
	},
}

var hubInstallCmd = &cobra.Command{
	Use:   "install [type/name]",
	Short: "Install an item from the hub",
	Long: `Install a plugin, skill, or integration from SoulHub.

Examples:
  soulgate hub install plugin/awesome-github
  soulgate hub install skill/auto-commit
  soulgate hub install whatsapp (auto-detects type)`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")
		installItem(cmd.Context(), args[0], force)
	},
}

var hubUninstallCmd = &cobra.Command{
	Use:   "uninstall [type/name]",
	Short: "Uninstall an installed item",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		uninstallItem(cmd.Context(), args[0])
	},
}

var hubInstalledCmd = &cobra.Command{
	Use:   "installed",
	Short: "List installed items",
	Run: func(cmd *cobra.Command, args []string) {
		listInstalled(cmd.Context())
	},
}

var hubInfoCmd = &cobra.Command{
	Use:   "info [type/name]",
	Short: "Show detailed information about an item",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		showInfo(cmd.Context(), args[0])
	},
}

var hubUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update installed items",
	Run: func(cmd *cobra.Command, args []string) {
		updateItems(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(hubCmd)

	hubCmd.AddCommand(hubPluginsCmd)
	hubCmd.AddCommand(hubSkillsCmd)
	hubCmd.AddCommand(hubSearchCmd)
	hubCmd.AddCommand(hubInstallCmd)
	hubCmd.AddCommand(hubUninstallCmd)
	hubCmd.AddCommand(hubInstalledCmd)
	hubCmd.AddCommand(hubInfoCmd)
	hubCmd.AddCommand(hubUpdateCmd)

	// Flags
	hubInstallCmd.Flags().BoolP("force", "f", false, "Force reinstall")
}

// showHubOverview shows the main hub overview
func showHubOverview() {
	banner := colorAccent(`
  ╭─ 🐙 SoulHub ─────────────────────────────────────────╮
  │                                                       │
  │  Community Marketplace for Plugins, Skills & More    │
  │                                                       │
  ╰───────────────────────────────────────────────────────╯
`)

	fmt.Println(banner)
	fmt.Println(colorBold("\nCategories:"))
	fmt.Println(colorAccent("  🔌 Plugins") + "       - WASM extensions and integrations")
	fmt.Println(colorAccent("  ⚡ Skills") + "        - Automation workflows")
	fmt.Println(colorAccent("  🔗 Integrations") + " - Ready-to-use integrations")
	fmt.Println(colorAccent("  📦 Recipes") + "       - Complete solutions")

	fmt.Println(colorBold("\nQuick Commands:"))
	fmt.Println("  " + colorMuted("soulgate hub plugins") + "           - Browse plugins")
	fmt.Println("  " + colorMuted("soulgate hub search <query>") + "    - Search hub")
	fmt.Println("  " + colorMuted("soulgate hub install <name>") + "    - Install item")
	fmt.Println("  " + colorMuted("soulgate hub installed") + "         - Show installed")

	fmt.Println(colorBold("\nIn Chat:"))
	fmt.Println("  " + colorMuted(">>> /hub") + "                       - Open hub browser")
	fmt.Println("  " + colorMuted(">>> /hub plugins") + "               - Browse plugins")
	fmt.Println("  " + colorMuted(">>> /hub install <name>") + "        - Install item")

	fmt.Println()
}

// listPlugins lists all available plugins
func listPlugins(ctx context.Context) {
	client := getHubClient()

	fmt.Println(colorAccentBright("🔌 Available Plugins"))
	fmt.Println()

	plugins, err := client.ListPlugins(ctx)
	if err != nil {
		fmt.Println(colorError("Error: " + err.Error()))
		return
	}

	if len(plugins) == 0 {
		fmt.Println(colorMuted("No plugins available yet."))
		return
	}

	for _, plugin := range plugins {
		stars := renderStars(plugin.Rating)
		fmt.Printf("%s %s\n", colorAccentBright(plugin.Name), stars)
		fmt.Printf("  %s\n", colorMuted(plugin.Description))
		fmt.Printf("  %s • %d downloads\n", colorMuted(plugin.Category), plugin.Downloads)
		fmt.Println()
	}

	fmt.Printf(colorMuted("Total: %d plugins\n"), len(plugins))
}

// listSkills lists all available skills
func listSkills(ctx context.Context) {
	client := getHubClient()

	fmt.Println(colorAccentBright("⚡ Available Skills"))
	fmt.Println()

	skills, err := client.ListSkills(ctx)
	if err != nil {
		fmt.Println(colorError("Error: " + err.Error()))
		return
	}

	if len(skills) == 0 {
		fmt.Println(colorMuted("No skills available yet."))
		return
	}

	for _, skill := range skills {
		stars := renderStars(skill.Rating)
		fmt.Printf("%s %s\n", colorAccentBright(skill.Name), stars)
		fmt.Printf("  %s\n", colorMuted(skill.Description))
		fmt.Printf("  %s • %d downloads\n", colorMuted(skill.Category), skill.Downloads)
		fmt.Println()
	}

	fmt.Printf(colorMuted("Total: %d skills\n"), len(skills))
}

// searchHub searches the hub
func searchHub(ctx context.Context, query string) {
	client := getHubClient()

	fmt.Printf(colorAccentBright("🔍 Searching for: %s\n\n"), query)

	plugins, err := client.SearchPlugins(ctx, query)
	if err != nil {
		fmt.Println(colorError("Error: " + err.Error()))
		return
	}

	if len(plugins) == 0 {
		fmt.Println(colorMuted("No results found."))
		return
	}

	for _, plugin := range plugins {
		stars := renderStars(plugin.Rating)
		fmt.Printf("%s %s\n", colorAccentBright(plugin.Name), stars)
		fmt.Printf("  %s\n", colorMuted(plugin.Description))
		fmt.Println()
	}

	fmt.Printf(colorMuted("Found %d results\n"), len(plugins))
}

// installItem installs an item from the hub
func installItem(ctx context.Context, itemPath string, force bool) {
	itemType, name := parseItemPath(itemPath)

	fmt.Printf(colorAccent("Installing %s/%s...\n"), itemType, name)

	installer := getInstaller()

	opts := hub.InstallOptions{
		Force: force,
	}

	var err error
	switch itemType {
	case "plugin":
		err = installer.InstallPlugin(ctx, name, opts)
	case "skill":
		err = installer.InstallSkill(ctx, name, opts)
	default:
		fmt.Println(colorError("Unknown type: " + itemType))
		return
	}

	if err != nil {
		fmt.Println(colorError("✗ Installation failed: " + err.Error()))
		return
	}

	fmt.Println(colorSuccess("✓ Successfully installed " + name))
}

// uninstallItem uninstalls an installed item
func uninstallItem(ctx context.Context, itemPath string) {
	itemType, name := parseItemPath(itemPath)

	fmt.Printf(colorAccent("Uninstalling %s/%s...\n"), itemType, name)

	installer := getInstaller()

	if err := installer.Uninstall(itemType, name); err != nil {
		fmt.Println(colorError("✗ Uninstallation failed: " + err.Error()))
		return
	}

	fmt.Println(colorSuccess("✓ Successfully uninstalled " + name))
}

// listInstalled lists installed items
func listInstalled(ctx context.Context) {
	registry := getRegistry()

	items := registry.List()

	if len(items) == 0 {
		fmt.Println(colorMuted("No items installed yet."))
		fmt.Println(colorMuted("\nInstall something with: soulgate hub install <name>"))
		return
	}

	fmt.Println(colorAccentBright("📦 Installed Items"))
	fmt.Println()

	for _, item := range items {
		fmt.Printf("%s %s/%s\n", colorSuccess("✓"), colorAccent(item.Type), colorBold(item.Name))
		fmt.Printf("  Version: %s\n", colorMuted(item.Version))
		fmt.Printf("  Installed: %s\n", colorMuted(item.InstalledAt.Format("2006-01-02 15:04")))
		fmt.Println()
	}

	fmt.Printf(colorMuted("Total: %d items installed\n"), len(items))
}

// showInfo shows detailed information about an item
func showInfo(ctx context.Context, itemPath string) {
	itemType, name := parseItemPath(itemPath)
	client := getHubClient()

	if itemType == "plugin" {
		details, err := client.GetPlugin(ctx, name)
		if err != nil {
			fmt.Println(colorError("Error: " + err.Error()))
			return
		}

		// Display plugin details
		fmt.Println(colorAccentBright("🔌 Plugin: " + details.Name))
		fmt.Println()
		fmt.Printf("  %s\n", renderStars(details.Rating))
		fmt.Printf("  Version: %s\n", colorMuted(details.Version))
		fmt.Printf("  Author: %s\n", colorMuted(details.Author))
		fmt.Printf("  Downloads: %s\n", colorMuted(fmt.Sprintf("%d", details.Downloads)))
		fmt.Println()
		fmt.Printf("  %s\n", details.Description)
		fmt.Println()

		if len(details.Permissions) > 0 {
			fmt.Println(colorBold("  Permissions:"))
			for _, perm := range details.Permissions {
				fmt.Printf("    • %s\n", colorMuted(perm))
			}
			fmt.Println()
		}

		if len(details.Tools) > 0 {
			fmt.Println(colorBold("  Tools Provided:"))
			for _, tool := range details.Tools {
				fmt.Printf("    • %s - %s\n", colorAccent(tool.Name), colorMuted(tool.Description))
			}
			fmt.Println()
		}

		fmt.Printf("  Install: %s\n", colorMuted("soulgate hub install plugin/"+name))
	}
}

// updateItems updates all installed items
func updateItems(ctx context.Context) {
	installer := getInstaller()

	fmt.Println(colorAccent("Checking for updates..."))

	updated, err := installer.UpdateAll(ctx)
	if err != nil {
		fmt.Println(colorError("Error: " + err.Error()))
		return
	}

	if len(updated) == 0 {
		fmt.Println(colorSuccess("✓ All items are up to date"))
		return
	}

	fmt.Printf(colorSuccess("✓ Updated %d items:\n"), len(updated))
	for _, item := range updated {
		fmt.Printf("  - %s\n", item)
	}
}

// Helper functions

func getHubClient() *hub.HubClient {
	configDir := getGlobalConfigDir()
	cacheDir := filepath.Join(configDir, "hub", "cache")
	return hub.NewHubClient("", cacheDir)
}

func getRegistry() *hub.Registry {
	configDir := getGlobalConfigDir()
	registry, err := hub.NewRegistry(configDir)
	if err != nil {
		fmt.Println(colorError("Error creating registry: " + err.Error()))
		os.Exit(1)
	}
	return registry
}

func getInstaller() *hub.Installer {
	configDir := getGlobalConfigDir()
	client := getHubClient()
	registry := getRegistry()
	return hub.NewInstaller(client, registry, configDir)
}

func getGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println(colorError("Error getting home directory: " + err.Error()))
		os.Exit(1)
	}
	return filepath.Join(home, ".soulgate")
}

func parseItemPath(path string) (string, string) {
	parts := strings.Split(path, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Default to plugin
	return "plugin", path
}

func renderStars(rating float64) string {
	fullStars := int(rating)
	stars := ""
	for i := 0; i < fullStars; i++ {
		stars += "⭐"
	}
	return stars + fmt.Sprintf(" %.1f", rating)
}
