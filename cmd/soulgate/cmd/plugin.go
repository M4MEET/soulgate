package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/plugins/loader"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Plugin management",
	Long:  `Manage SoulGate plugins - list, install, and configure plugins.`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Long:  `Display all installed plugins with their capabilities.`,
	RunE:  runPluginList,
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd)
}

func runPluginList(cmd *cobra.Command, args []string) error {
	// Load workspace
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return fmt.Errorf("failed to load workspace: %w", err)
	}

	// Create loader
	pluginLoader := loader.NewLoader(workspace.Config.Plugins.Dir)

	// Load all plugins
	plugins, err := pluginLoader.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	if len(plugins) == 0 {
		fmt.Println("No plugins installed")
		fmt.Println()
		fmt.Println("To install plugins, place them in:", workspace.Config.Plugins.Dir)
		return nil
	}

	// Display plugins
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "NAME\tVERSION\tTOOLS\tRUNTIME\n")
	fmt.Fprintf(w, "----\t-------\t-----\t-------\n")

	for _, plugin := range plugins {
		toolCount := len(plugin.Manifest.Tools)
		toolNames := ""
		if toolCount > 0 {
			toolNames = plugin.Manifest.Tools[0].Name
			if toolCount > 1 {
				toolNames += fmt.Sprintf(" (+%d more)", toolCount-1)
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			plugin.Manifest.Name,
			plugin.Manifest.Version,
			toolNames,
			plugin.Manifest.Runtime,
		)
	}

	return nil
}
