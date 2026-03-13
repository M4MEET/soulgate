package onboarding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/dependencies"
	"github.com/M4MEET/soulgate/internal/ui/setup"
	"gopkg.in/yaml.v3"
)

// OnboardingState tracks the current onboarding step
type OnboardingState struct {
	Step                   int
	SelectedProvider       string
	SelectedModel          string
	OpenAIKey              string
	AnthropicKey           string
	IntegrationsToSetup    []string
	CurrentIntegration     int
	IntegrationWizard      *setup.Wizard
	Workspace              *config.Workspace
	Complete               bool
	InstallingDependencies bool
	DependencyStatus       map[string]string // integrationID -> status message
	DependencyErrors       []string
}

// OnboardingStep represents a step in the onboarding process
type OnboardingStep struct {
	Name        string
	Title       string
	Description string
	CanSkip     bool
}

// GetOnboardingSteps returns all onboarding steps
func GetOnboardingSteps() []OnboardingStep {
	return []OnboardingStep{
		{
			Name:        "welcome",
			Title:       "Welcome to SoulGate!",
			Description: "Your secure AI agent gateway",
			CanSkip:     false,
		},
		{
			Name:        "model_selection",
			Title:       "Choose Your AI Model",
			Description: "Select your default AI provider",
			CanSkip:     false,
		},
		{
			Name:        "api_keys",
			Title:       "Configure API Keys",
			Description: "Add your AI provider API keys",
			CanSkip:     false,
		},
		{
			Name:        "test_connection",
			Title:       "Test Connection",
			Description: "Verify your API keys work",
			CanSkip:     false,
		},
		{
			Name:        "integrations",
			Title:       "Setup Integrations",
			Description: "Add Slack, GitHub, and more (optional)",
			CanSkip:     true,
		},
		{
			Name:        "dependencies",
			Title:       "Installing Dependencies",
			Description: "Setting up required tools and SDKs",
			CanSkip:     false,
		},
		{
			Name:        "tutorial",
			Title:       "Quick Start Guide",
			Description: "Learn the basics",
			CanSkip:     true,
		},
		{
			Name:        "complete",
			Title:       "You're All Set!",
			Description: "Ready to start using SoulGate",
			CanSkip:     false,
		},
	}
}

// NewOnboardingState creates a new onboarding state
func NewOnboardingState(workspace *config.Workspace) *OnboardingState {
	return &OnboardingState{
		Step:                   0,
		IntegrationsToSetup:    []string{},
		IntegrationWizard:      setup.NewWizard(workspace),
		Workspace:              workspace,
		Complete:               false,
		InstallingDependencies: false,
		DependencyStatus:       make(map[string]string),
		DependencyErrors:       []string{},
	}
}

// NextStep advances to the next step
func (s *OnboardingState) NextStep() {
	s.Step++
}

// PreviousStep goes back to the previous step
func (s *OnboardingState) PreviousStep() {
	if s.Step > 0 {
		s.Step--
	}
}

// GetCurrentStep returns the current step
func (s *OnboardingState) GetCurrentStep() OnboardingStep {
	steps := GetOnboardingSteps()
	if s.Step >= 0 && s.Step < len(steps) {
		return steps[s.Step]
	}
	return steps[len(steps)-1] // Return complete step if beyond
}

// GetProgress returns the progress percentage
func (s *OnboardingState) GetProgress() int {
	steps := GetOnboardingSteps()
	return int(float64(s.Step) / float64(len(steps)-1) * 100)
}

// SaveAPIKeys saves the configured API keys to config
func (s *OnboardingState) SaveAPIKeys() error {
	cfg := s.Workspace.Config

	// Update OpenAI config
	if s.OpenAIKey != "" {
		cfg.Model.OpenAI.APIKey = s.OpenAIKey
		// Also set in environment for immediate use
		os.Setenv("OPENAI_API_KEY", s.OpenAIKey)
	}

	// Update Anthropic config
	if s.AnthropicKey != "" {
		cfg.Model.Anthropic.APIKey = s.AnthropicKey
		// Also set in environment for immediate use
		os.Setenv("ANTHROPIC_API_KEY", s.AnthropicKey)
	}

	// Set default provider
	cfg.Model.DefaultProvider = s.SelectedProvider

	// Set default model based on selection
	switch s.SelectedModel {
	case "gpt-4.1":
		cfg.Model.OpenAI.Model = "gpt-4.1"
	case "gpt-4.1-mini":
		cfg.Model.OpenAI.Model = "gpt-4.1-mini"
	case "claude-opus":
		cfg.Model.Anthropic.Model = "claude-opus-4-20250514"
	case "claude-sonnet":
		cfg.Model.Anthropic.Model = "claude-sonnet-4-20250514"
	}

	// Save config
	configPath := filepath.Join(s.Workspace.ConfigDir, "config.yml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// MarkComplete marks onboarding as complete
func (s *OnboardingState) MarkComplete() error {
	s.Complete = true

	// Create a marker file to indicate onboarding is complete
	markerPath := filepath.Join(s.Workspace.ConfigDir, ".onboarding_complete")
	if err := os.WriteFile(markerPath, []byte("complete"), 0644); err != nil {
		return fmt.Errorf("failed to create onboarding marker: %w", err)
	}

	return nil
}

// IsOnboardingComplete checks if onboarding has been completed
func IsOnboardingComplete(workspace *config.Workspace) bool {
	markerPath := filepath.Join(workspace.ConfigDir, ".onboarding_complete")
	_, err := os.Stat(markerPath)
	return err == nil
}

// GetModelOptions returns available model options based on onboarding flow
// This now wraps the shared config.GetModelOptions() for consistency
func GetModelOptions() []ModelOption {
	// Use shared presets from config package
	sharedOptions := config.GetModelOptions()

	// Convert to local ModelOption type for UI rendering
	options := make([]ModelOption, len(sharedOptions))
	for i, opt := range sharedOptions {
		options[i] = ModelOption{
			ID:          opt.ID,
			Name:        opt.Name,
			Provider:    opt.Provider,
			Description: opt.Description,
			Icon:        opt.Icon,
			Recommended: opt.Recommended,
		}
	}

	return options
}

// ModelOption represents a selectable model option
type ModelOption struct {
	ID          string
	Name        string
	Provider    string
	Description string
	Icon        string
	Recommended bool
}

// ValidateOpenAIKey validates an OpenAI API key format
// DEPRECATED: Use config.ValidateAPIKey("openai", key) instead
func ValidateOpenAIKey(key string) error {
	return config.ValidateAPIKey("openai", key)
}

// ValidateAnthropicKey validates an Anthropic API key format
// DEPRECATED: Use config.ValidateAPIKey("anthropic", key) instead
func ValidateAnthropicKey(key string) error {
	return config.ValidateAPIKey("anthropic", key)
}

// GetIntegrationRecommendations returns recommended integrations for quick setup
func GetIntegrationRecommendations() []IntegrationRecommendation {
	return []IntegrationRecommendation{
		{
			ID:          "slack",
			Name:        "Slack",
			Icon:        "💬",
			Description: "Team communication and notifications",
			Popular:     true,
		},
		{
			ID:          "github",
			Name:        "GitHub",
			Icon:        "🐙",
			Description: "Code repositories and collaboration",
			Popular:     true,
		},
		{
			ID:          "notion",
			Name:        "Notion",
			Icon:        "📝",
			Description: "Notes and knowledge management",
			Popular:     true,
		},
		{
			ID:          "linear",
			Name:        "Linear",
			Icon:        "🎯",
			Description: "Issue tracking and project management",
			Popular:     false,
		},
	}
}

// IntegrationRecommendation represents a recommended integration
type IntegrationRecommendation struct {
	ID          string
	Name        string
	Icon        string
	Description string
	Popular     bool
}

// GetTutorialSteps returns quick start tutorial steps
func GetTutorialSteps() []TutorialStep {
	return []TutorialStep{
		{
			Title:   "Ask Questions",
			Command: "What files are in this directory?",
			Desc:    "The AI has full access to your system",
		},
		{
			Title:   "Execute Commands",
			Command: "!git status",
			Desc:    "Run shell commands with !",
		},
		{
			Title:   "Use Integrations",
			Command: "Send a message to #general",
			Desc:    "The AI uses your configured integrations",
		},
		{
			Title:   "Switch Models",
			Command: "/model",
			Desc:    "Choose different AI models on the fly",
		},
		{
			Title:   "View Status",
			Command: "/status",
			Desc:    "See current configuration and stats",
		},
	}
}

// TutorialStep represents a tutorial step
type TutorialStep struct {
	Title   string
	Command string
	Desc    string
}

// InstallDependencies installs dependencies for all configured integrations
func (s *OnboardingState) InstallDependencies(ctx context.Context) error {
	if len(s.IntegrationsToSetup) == 0 {
		return nil // No integrations configured, nothing to install
	}

	s.InstallingDependencies = true

	// Use workspace config directory for dependencies
	soulGateDir := s.Workspace.ConfigDir
	installer := dependencies.NewDependencyInstaller(soulGateDir, true) // verbose mode

	// First, check system prerequisites
	prereqs := installer.CheckSystemPrerequisites()
	missingPrereqs := []string{}
	if !prereqs["npm"] {
		missingPrereqs = append(missingPrereqs, "npm (Node.js package manager)")
	}
	if !prereqs["node"] {
		missingPrereqs = append(missingPrereqs, "node (Node.js runtime)")
	}

	if len(missingPrereqs) > 0 {
		s.DependencyErrors = append(s.DependencyErrors,
			fmt.Sprintf("Missing prerequisites: %s. Please install Node.js first.",
				strings.Join(missingPrereqs, ", ")))
	}

	// Install dependencies for each integration
	for _, integrationID := range s.IntegrationsToSetup {
		s.DependencyStatus[integrationID] = "checking..."

		// Check what's missing
		missing, err := installer.GetMissingDependencies(ctx, integrationID)
		if err != nil {
			s.DependencyStatus[integrationID] = "error checking"
			s.DependencyErrors = append(s.DependencyErrors,
				fmt.Sprintf("%s: %v", integrationID, err))
			continue
		}

		if len(missing) == 0 {
			s.DependencyStatus[integrationID] = "✓ already installed"
			continue
		}

		// Install missing dependencies
		s.DependencyStatus[integrationID] = fmt.Sprintf("installing %d dependencies...", len(missing))

		installed, err := installer.InstallAll(ctx, integrationID)
		if err != nil {
			s.DependencyStatus[integrationID] = fmt.Sprintf("✗ %d/%d installed", len(installed), len(missing))
			s.DependencyErrors = append(s.DependencyErrors,
				fmt.Sprintf("%s: %v", integrationID, err))
			continue
		}

		s.DependencyStatus[integrationID] = fmt.Sprintf("✓ %d dependencies installed", len(installed))
	}

	s.InstallingDependencies = false
	return nil
}

// GetDependencyInstructions returns manual installation instructions for dependencies
func (s *OnboardingState) GetDependencyInstructions() map[string][]string {
	instructions := make(map[string][]string)

	for _, integrationID := range s.IntegrationsToSetup {
		instrs := dependencies.GetInstallInstructions(integrationID)
		if len(instrs) > 0 {
			instructions[integrationID] = instrs
		}
	}

	return instructions
}
