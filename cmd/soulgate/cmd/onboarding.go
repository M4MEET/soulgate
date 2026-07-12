package cmd

import "github.com/spf13/cobra"

var onboardingCmd = &cobra.Command{
	Use:   "onboarding",
	Short: "Launch onboarding wizard in the TUI",
	Long: `Launch the interactive onboarding wizard directly.

This is a shortcut for:
  soulgate tui --onboarding

Use this when you want to rerun first-time setup guidance.`,
	Args: cobra.NoArgs,
	RunE: runOnboarding,
}

func init() {
	rootCmd.AddCommand(onboardingCmd)
}

func runOnboarding(cmd *cobra.Command, args []string) error {
	prevInteractive := useInteractiveTUI
	prevForce := forceOnboarding

	useInteractiveTUI = true
	forceOnboarding = true
	defer func() {
		useInteractiveTUI = prevInteractive
		forceOnboarding = prevForce
	}()

	return runChat(cmd, args)
}
