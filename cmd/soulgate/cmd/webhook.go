package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/gateway"
	"github.com/spf13/cobra"
)

// webhookCmd is the parent command: soulgate webhook
var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage inbound webhooks",
	Long: `Manage inbound webhooks that allow external services to send messages
to SoulGate via HTTP POST.

Each webhook creates an endpoint at POST /webhook/{name} on the gateway server.
Payloads are extracted based on the configured format and forwarded to the
configured chat handler.`,
}

// webhook list

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured webhooks",
	Long:  `Display all configured inbound webhooks with their settings.`,
	RunE:  runWebhookList,
}

// webhook add flags
var (
	webhookAddSecret     string
	webhookAddFormat     string
	webhookAddMessageKey string
)

// webhook add

var webhookAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add an inbound webhook",
	Long: `Add a new inbound webhook endpoint.

The webhook will be reachable at POST /webhook/<name> when the gateway is running.

Formats:
  json    (default) Extract message from a JSON field (see --key)
  text    Use raw request body as message
  github  Parse GitHub webhook events (push, PR, issues, etc.)
  gitlab  Parse GitLab webhook events (push, MR, issues, etc.)

Examples:
  soulgate webhook add ci-alerts --format json --key alert.message
  soulgate webhook add github-events --format github --secret $WEBHOOK_SECRET
  soulgate webhook add plaintext-feed --format text`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhookAdd,
}

// webhook remove

var webhookRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an inbound webhook",
	Long:  `Delete a configured inbound webhook by name.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runWebhookRemove,
}

// webhook test

var webhookTestPort int

var webhookTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Send a test message through a webhook",
	Long: `Send a synthetic test payload to the local gateway via the named webhook.

The gateway must be running (soulgate gateway start) for this command to work.
The test payload is constructed to match the webhook's configured format.`,
	Args: cobra.ExactArgs(1),
	RunE: runWebhookTest,
}

func init() {
	rootCmd.AddCommand(webhookCmd)
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookAddCmd)
	webhookCmd.AddCommand(webhookRemoveCmd)
	webhookCmd.AddCommand(webhookTestCmd)

	webhookAddCmd.Flags().StringVar(&webhookAddSecret, "secret", "", "HMAC-SHA256 secret for signature verification")
	webhookAddCmd.Flags().StringVar(&webhookAddFormat, "format", "json", "Payload format: json, text, github, gitlab")
	webhookAddCmd.Flags().StringVar(&webhookAddMessageKey, "key", "message", "JSON key path to extract message (only used with --format json)")

	webhookTestCmd.Flags().IntVar(&webhookTestPort, "port", 8080, "Port the gateway is listening on")
}

// webhooksFilePath returns the absolute path to the webhooks JSON file inside
// the workspace .soulgate directory.
func webhooksFilePath() (string, error) {
	workspace, err := config.LoadWorkspace()
	if err != nil {
		return "", fmt.Errorf("failed to load workspace: %w", err)
	}
	return filepath.Join(workspace.ConfigDir, "webhooks.json"), nil
}

func runWebhookList(cmd *cobra.Command, args []string) error {
	path, err := webhooksFilePath()
	if err != nil {
		return err
	}

	// Read the file directly — no running gateway required.
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Println("No webhooks configured.")
		fmt.Println()
		fmt.Println("Add one with: soulgate webhook add <name>")
		return nil
	}
	if err != nil {
		return fmt.Errorf("read webhooks file: %w", err)
	}

	var list []*gateway.WebhookConfig
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse webhooks file: %w", err)
	}

	if len(list) == 0 {
		fmt.Println("No webhooks configured.")
		fmt.Println()
		fmt.Println("Add one with: soulgate webhook add <name>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tFORMAT\tKEY\tSECRET\tENABLED")
	fmt.Fprintln(w, "----\t------\t---\t------\t-------")

	for _, wh := range list {
		secret := "none"
		if wh.Secret != "" {
			secret = "set"
		}
		key := wh.MessageKey
		if wh.Format != gateway.WebhookFormatJSON {
			key = "-"
		}
		enabled := "yes"
		if !wh.Enabled {
			enabled = "no"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			wh.Name, wh.Format, key, secret, enabled)
	}

	return nil
}

func runWebhookAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	path, err := webhooksFilePath()
	if err != nil {
		return err
	}

	store := loadWebhookStore(path)

	format := gateway.WebhookFormat(webhookAddFormat)
	switch format {
	case gateway.WebhookFormatJSON, gateway.WebhookFormatText,
		gateway.WebhookFormatGitHub, gateway.WebhookFormatGitLab:
		// valid
	default:
		return fmt.Errorf("unknown format %q; valid values: json, text, github, gitlab", webhookAddFormat)
	}

	wh := &gateway.WebhookConfig{
		Name:       name,
		Secret:     webhookAddSecret,
		Format:     format,
		MessageKey: webhookAddMessageKey,
		Enabled:    true,
	}

	if err := store.Add(wh); err != nil {
		return fmt.Errorf("add webhook: %w", err)
	}

	fmt.Printf("Webhook %q added.\n", name)
	fmt.Printf("  Endpoint : POST /webhook/%s\n", name)
	fmt.Printf("  Format   : %s\n", format)
	if format == gateway.WebhookFormatJSON {
		fmt.Printf("  Key      : %s\n", webhookAddMessageKey)
	}
	if webhookAddSecret != "" {
		fmt.Printf("  Secret   : set (HMAC-SHA256 verification enabled)\n")
	}
	fmt.Println()
	fmt.Println("Start the gateway with: soulgate gateway start")
	return nil
}

func runWebhookRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	path, err := webhooksFilePath()
	if err != nil {
		return err
	}

	store := loadWebhookStore(path)

	if err := store.Remove(name); err != nil {
		return fmt.Errorf("remove webhook: %w", err)
	}

	fmt.Printf("Webhook %q removed.\n", name)
	return nil
}

func runWebhookTest(cmd *cobra.Command, args []string) error {
	name := args[0]

	path, err := webhooksFilePath()
	if err != nil {
		return err
	}

	store := loadWebhookStore(path)

	wh := store.Get(name)
	if wh == nil {
		return fmt.Errorf("webhook %q not found", name)
	}

	fmt.Printf("Sending test payload to webhook %q (gateway port %d)...\n", name, webhookTestPort)

	response, err := gateway.SendTestWebhook(webhookTestPort, wh)
	if err != nil {
		return fmt.Errorf("test failed: %w", err)
	}

	fmt.Println("Response:")
	fmt.Println(response)
	return nil
}

// loadWebhookStore creates a WebhookStore backed by the file at path, loading
// existing data from disk. A missing file is not an error; it will be created
// on the first write. Errors (e.g. corrupted JSON) are printed and a fresh
// empty store is returned so that other commands can still proceed.
func loadWebhookStore(path string) *gateway.WebhookStore {
	s, err := gateway.NewWebhookStore(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load webhooks file: %v\n", err)
		// Construct an empty store at the same path so writes go to the right place.
		s = gateway.EmptyWebhookStore(path)
	}
	return s
}
