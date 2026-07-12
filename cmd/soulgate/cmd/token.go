package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/gateway"
	"github.com/spf13/cobra"
)

// tokenCmd is the parent: soulgate token
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage gateway API tokens",
	Long: `Manage Bearer tokens for the SoulGate gateway HTTP API.

Tokens are stored in .soulgate/security/tokens.json in the current workspace.
When the gateway is started with --auth, every /api/* request must include:

  Authorization: Bearer sg_<value>

Requests from localhost are always allowed in --dev-mode (the default) so
local development does not require a token.`,
}

var (
	tokenCreateName string
)

// token create

var tokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Generate a new API token",
	Long: `Generate a new gateway API token and print it to stdout.

The token is stored in .soulgate/security/tokens.json and can be used immediately
with a running gateway that has --auth enabled.

Example:
  soulgate token create --name ci-pipeline`,
	Args: cobra.NoArgs,
	RunE: runTokenCreate,
}

func runTokenCreate(cmd *cobra.Command, _ []string) error {
	tokensFile, err := resolveTokensFile()
	if err != nil {
		return err
	}

	apiAuth, err := gateway.NewAPIAuthFromFile(tokensFile, 0)
	if err != nil {
		return fmt.Errorf("load token store: %w", err)
	}

	tok, err := apiAuth.CreateToken(tokenCreateName)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}

	fmt.Printf("Token created successfully.\n\n")
	fmt.Printf("  Name:    %s\n", labelOrEmpty(tok.Name))
	fmt.Printf("  Value:   %s\n", tok.Value)
	fmt.Printf("  Created: %s\n\n", tok.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Add to requests as:\n  Authorization: Bearer %s\n", tok.Value)
	return nil
}

// token list

var tokenListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active API tokens",
	Long:  `Display all API tokens stored in the current workspace.`,
	RunE:  runTokenList,
}

func runTokenList(_ *cobra.Command, _ []string) error {
	tokensFile, err := resolveTokensFile()
	if err != nil {
		return err
	}

	apiAuth, err := gateway.NewAPIAuthFromFile(tokensFile, 0)
	if err != nil {
		return fmt.Errorf("load token store: %w", err)
	}

	tokens := apiAuth.ListTokens()
	if len(tokens) == 0 {
		fmt.Println("No API tokens found. Create one with: soulgate token create")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "VALUE\tNAME\tSTATUS\tCREATED")
	fmt.Fprintln(w, "-----\t----\t------\t-------")
	for _, tok := range tokens {
		status := "active"
		if tok.Revoked {
			status = "revoked"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			tok.Value,
			labelOrEmpty(tok.Name),
			status,
			tok.CreatedAt.Format("2006-01-02 15:04:05"),
		)
	}
	w.Flush()
	return nil
}

// token revoke

var tokenRevokeCmd = &cobra.Command{
	Use:   "revoke <token>",
	Short: "Revoke an API token",
	Long: `Mark an API token as revoked. Revoked tokens are rejected immediately
by a running gateway (no restart needed).`,
	Args: cobra.ExactArgs(1),
	RunE: runTokenRevoke,
}

func runTokenRevoke(_ *cobra.Command, args []string) error {
	tokensFile, err := resolveTokensFile()
	if err != nil {
		return err
	}

	apiAuth, err := gateway.NewAPIAuthFromFile(tokensFile, 0)
	if err != nil {
		return fmt.Errorf("load token store: %w", err)
	}

	if err := apiAuth.RevokeToken(args[0]); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}

	fmt.Printf("Token revoked: %s\n", args[0])
	return nil
}

// resolveTokensFile returns the path to security/tokens.json inside the
// workspace config directory, creating the directory if it does not exist.
func resolveTokensFile() (string, error) {
	ws, err := config.LoadWorkspace()
	if err != nil {
		return "", fmt.Errorf("load workspace: %w", err)
	}
	secDir := filepath.Join(ws.ConfigDir, "security")
	if err := os.MkdirAll(secDir, 0o700); err != nil {
		return "", fmt.Errorf("ensure security dir: %w", err)
	}
	return filepath.Join(secDir, "tokens.json"), nil
}

func labelOrEmpty(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func init() {
	rootCmd.AddCommand(tokenCmd)
	tokenCmd.AddCommand(tokenCreateCmd)
	tokenCmd.AddCommand(tokenListCmd)
	tokenCmd.AddCommand(tokenRevokeCmd)

	tokenCreateCmd.Flags().StringVar(&tokenCreateName, "name", "", "Optional label for the token (e.g. ci-pipeline)")
}
