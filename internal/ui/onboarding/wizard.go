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
)

// OnboardingState tracks the current onboarding step and all configuration
// collected during the wizard flow.
type OnboardingState struct {
	Step int
	Flow string // "quickstart" or "advanced"

	// QuickMode mirrors Flow == "quickstart" for backward compatibility
	// with the TUI layer which reads this field directly.
	QuickMode bool

	Workspace *config.Workspace

	// Security acknowledgement
	RiskAccepted bool

	// Provider selection (two-level: group → provider)
	ProviderGroup    string // "cloud", "local", "custom"
	SelectedProvider string
	SelectedModel    string
	APIKey           string
	APIKeyError      string
	BaseURL          string // for custom providers

	// Legacy per-provider key fields kept for backward compatibility
	// with the TUI layer and existing tests.
	OpenAIKey      string
	AnthropicKey   string
	ProviderAPIKey string

	// Gateway config
	GatewayPort int    // default 8080
	GatewayBind string // "loopback", "lan", "custom"

	// Channel selection (messaging platforms)
	ChannelsToSetup []string

	// Integration selection (legacy; kept for TUI compatibility)
	IntegrationsToSetup []string

	// Completion
	Complete bool
	Error    string

	// Existing config detection
	HasExistingConfig bool
	ConfigAction      string // "use", "update", "reset"

	// Integration wizard (legacy; kept for TUI compatibility)
	CurrentIntegration int
	IntegrationWizard  *setup.Wizard

	// Dependency installation state (legacy; kept for TUI compatibility)
	InstallingDependencies bool
	DependencyStatus       map[string]string // integrationID -> status message
	DependencyErrors       []string
}

// OnboardingStep represents a step in the onboarding process.
type OnboardingStep struct {
	Name        string
	Title       string
	Description string
	CanSkip     bool
}

// GetOnboardingSteps returns all onboarding steps in display order.
//
// The canonical step sequence is:
//
//	welcome → security → flow_select → existing_config →
//	provider_group → provider_select → model_select → api_key →
//	channels → gateway → summary → complete
//
// The legacy steps (model_selection, api_keys, test_connection,
// integrations, dependencies, tutorial) are retained so that the existing
// TUI rendering and tests continue to work without modification.
func GetOnboardingSteps() []OnboardingStep {
	return []OnboardingStep{
		{
			Name:        "welcome",
			Title:       "Welcome to SoulGate",
			Description: "Secure AI agent gateway",
			CanSkip:     false,
		},
		{
			Name:        "security",
			Title:       "Security Notice",
			Description: "Please read before continuing",
			CanSkip:     false,
		},
		{
			Name:        "flow_select",
			Title:       "Choose Setup Style",
			Description: "QuickStart or Advanced configuration",
			CanSkip:     false,
		},
		{
			Name:        "existing_config",
			Title:       "Existing Configuration",
			Description: "A previous setup was found",
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
			Name:        "channels",
			Title:       "Messaging Channels",
			Description: "Choose platforms to connect",
			CanSkip:     true,
		},
		{
			Name:        "integrations",
			Title:       "Setup Integrations",
			Description: "Add Slack, GitHub, and more (optional)",
			CanSkip:     true,
		},
		{
			Name:        "gateway",
			Title:       "Gateway Configuration",
			Description: "Port and bind address (Advanced)",
			CanSkip:     true,
		},
		{
			Name:        "dependencies",
			Title:       "Installing Dependencies",
			Description: "Setting up required tools and SDKs",
			CanSkip:     false,
		},
		{
			Name:        "summary",
			Title:       "Configuration Summary",
			Description: "Review your settings before finishing",
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
			Title:       "Setup Complete",
			Description: "Ready to start using SoulGate",
			CanSkip:     false,
		},
	}
}

// NewOnboardingState creates a new onboarding state with sensible defaults.
func NewOnboardingState(workspace *config.Workspace) *OnboardingState {
	configDir := ""
	if workspace != nil {
		configDir = workspace.ConfigDir
	}

	return &OnboardingState{
		Step:                   0,
		Flow:                   "quickstart",
		GatewayPort:            8080,
		GatewayBind:            "loopback",
		ChannelsToSetup:        []string{},
		IntegrationsToSetup:    []string{},
		IntegrationWizard:      setup.NewWizard(workspace),
		Workspace:              workspace,
		Complete:               false,
		InstallingDependencies: false,
		DependencyStatus:       make(map[string]string),
		DependencyErrors:       []string{},
		HasExistingConfig:      DetectExistingConfig(configDir),
	}
}

// NextStep advances to the next step.
func (s *OnboardingState) NextStep() {
	s.Step++
}

// PreviousStep goes back to the previous step.
func (s *OnboardingState) PreviousStep() {
	if s.Step > 0 {
		s.Step--
	}
}

// GetCurrentStep returns the current step descriptor.
func (s *OnboardingState) GetCurrentStep() OnboardingStep {
	steps := GetOnboardingSteps()
	if s.Step >= 0 && s.Step < len(steps) {
		return steps[s.Step]
	}
	return steps[len(steps)-1]
}

// GetProgress returns the progress percentage (0–100).
func (s *OnboardingState) GetProgress() int {
	steps := GetOnboardingSteps()
	if len(steps) <= 1 {
		return 100
	}
	return int(float64(s.Step) / float64(len(steps)-1) * 100)
}

// GetStepTitle returns the display title for the given step index.
func GetStepTitle(step int) string {
	steps := GetOnboardingSteps()
	if step >= 0 && step < len(steps) {
		return steps[step].Title
	}
	return ""
}

// GetStepHint returns a short contextual hint for the given step index.
func GetStepHint(step int) string {
	steps := GetOnboardingSteps()
	if step < 0 || step >= len(steps) {
		return ""
	}
	hints := map[string]string{
		"welcome":         "press enter to continue, q to skip",
		"security":        "type \"I understand\" to acknowledge",
		"flow_select":     "enter for QuickStart, a for Advanced",
		"existing_config": "choose how to handle your existing setup",
		"model_selection": "use arrow keys to select, enter to confirm",
		"api_keys":        "paste your key and press enter, s to skip",
		"test_connection": "press enter to continue",
		"channels":        "space to toggle, enter to confirm",
		"integrations":    "space to toggle, enter to confirm",
		"gateway":         "configure port and bind address",
		"dependencies":    "wait for installation to complete",
		"summary":         "review your settings, enter to confirm",
		"tutorial":        "press enter or s to finish",
		"complete":        "press enter to start chatting",
	}
	if hint, ok := hints[steps[step].Name]; ok {
		return hint
	}
	return "press enter to continue"
}

// IsFirstRun returns true when the onboarding marker file does not yet exist
// in the given configDir, indicating the user has not completed setup before.
func IsFirstRun(configDir string) bool {
	if configDir == "" {
		return true
	}
	markerPath := filepath.Join(configDir, ".onboarding_complete")
	_, err := os.Stat(markerPath)
	return os.IsNotExist(err)
}

// DetectExistingConfig returns true when a config.yml file already exists
// inside configDir, indicating a prior workspace initialisation.
func DetectExistingConfig(configDir string) bool {
	if configDir == "" {
		return false
	}
	configPath := filepath.Join(configDir, "config.yml")
	_, err := os.Stat(configPath)
	return err == nil
}

// ValidateAPIKey validates the format of a provider API key.
// It delegates to config.ValidateAPIKey so all format rules are centralised.
func ValidateAPIKey(provider, key string) error {
	return config.ValidateAPIKey(provider, key)
}

// ProviderGroup represents a category of AI providers shown in the wizard.
type ProviderGroup struct {
	ID          string
	Name        string
	Description string
	Icon        string
}

// GetProviderGroups returns the top-level provider categories.
func GetProviderGroups() []ProviderGroup {
	return []ProviderGroup{
		{
			ID:          "cloud",
			Name:        "Cloud AI",
			Description: "OpenAI, Anthropic, Google, Groq, and more",
			Icon:        "☁",
		},
		{
			ID:          "local",
			Name:        "Local (Ollama)",
			Description: "Run models on your own machine — no API costs",
			Icon:        "🖥",
		},
		{
			ID:          "custom",
			Name:        "Custom Endpoint",
			Description: "OpenAI-compatible endpoint with a custom base URL",
			Icon:        "⚙",
		},
	}
}

// ProviderOption represents a specific AI provider within a group.
type ProviderOption struct {
	ID          string
	Name        string
	Description string
	Icon        string
	Group       string
	NeedsKey    bool
}

// GetProvidersInGroup returns the providers available within the given group ID.
func GetProvidersInGroup(group string) []ProviderOption {
	switch strings.ToLower(group) {
	case "cloud":
		return []ProviderOption{
			{ID: "openai", Name: "OpenAI", Description: "GPT-4.1 and GPT-4.1-mini", Icon: "🧠", Group: "cloud", NeedsKey: true},
			{ID: "anthropic", Name: "Anthropic", Description: "Claude Sonnet 4 and Opus 4", Icon: "🎭", Group: "cloud", NeedsKey: true},
			{ID: "google", Name: "Google (Gemini)", Description: "Gemini 2.5 Pro", Icon: "🔮", Group: "cloud", NeedsKey: true},
			{ID: "groq", Name: "Groq", Description: "Lightning-fast Llama inference", Icon: "⚡", Group: "cloud", NeedsKey: true},
			{ID: "mistral", Name: "Mistral AI", Description: "Mistral and Mixtral models", Icon: "💨", Group: "cloud", NeedsKey: true},
			{ID: "deepseek", Name: "DeepSeek", Description: "DeepSeek R1 and V3", Icon: "🔍", Group: "cloud", NeedsKey: true},
			{ID: "xai", Name: "xAI (Grok)", Description: "Grok models", Icon: "🤖", Group: "cloud", NeedsKey: true},
			{ID: "openrouter", Name: "OpenRouter", Description: "Access many models via one API", Icon: "🔀", Group: "cloud", NeedsKey: true},
			{ID: "together", Name: "Together AI", Description: "Open-source model hosting", Icon: "🤝", Group: "cloud", NeedsKey: true},
			{ID: "perplexity", Name: "Perplexity", Description: "Search-augmented generation", Icon: "🔭", Group: "cloud", NeedsKey: true},
			{ID: "cohere", Name: "Cohere", Description: "Command and Embed models", Icon: "🧬", Group: "cloud", NeedsKey: true},
		}
	case "local":
		return []ProviderOption{
			{ID: "ollama", Name: "Ollama", Description: "Run any open model locally", Icon: "🏠", Group: "local", NeedsKey: false},
		}
	case "custom":
		return []ProviderOption{
			{ID: "custom", Name: "Custom Endpoint", Description: "OpenAI-compatible base URL", Icon: "⚙", Group: "custom", NeedsKey: false},
		}
	default:
		return nil
	}
}

// ModelOption represents a selectable model for a specific provider.
type ModelOption struct {
	ID          string
	Name        string
	Provider    string
	Model       string
	Description string
	Icon        string
	Recommended bool
}

// GetModelsForProvider returns the curated model list for a given provider.
func GetModelsForProvider(provider string) []ModelOption {
	switch strings.ToLower(provider) {
	case "openai":
		return []ModelOption{
			{ID: "gpt-5.2", Name: "GPT-5.2", Provider: "openai", Model: "gpt-5.2", Description: "Latest flagship — complex coding & analysis", Icon: "🧠", Recommended: true},
			{ID: "gpt-5-mini", Name: "GPT-5 Mini", Provider: "openai", Model: "gpt-5-mini", Description: "Fast & economical", Icon: "⚡", Recommended: false},
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Model: "gpt-4o", Description: "Multimodal reasoning", Icon: "👁", Recommended: false},
		}
	case "anthropic":
		return []ModelOption{
			{ID: "claude-sonnet-5", Name: "Claude Sonnet 5", Provider: "anthropic", Model: "claude-sonnet-5", Description: "Balanced — great for most tasks", Icon: "🎭", Recommended: true},
			{ID: "claude-opus-4-8", Name: "Claude Opus 4.8", Provider: "anthropic", Model: "claude-opus-4-8", Description: "Most capable — deep reasoning", Icon: "🎪", Recommended: false},
		}
	case "google":
		return []ModelOption{
			{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Provider: "google", Model: "gemini-2.5-pro", Description: "Multimodal understanding", Icon: "🔮", Recommended: true},
			{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash", Provider: "google", Model: "gemini-2.0-flash", Description: "Fast and efficient", Icon: "⚡", Recommended: false},
		}
	case "groq":
		return []ModelOption{
			{ID: "llama-3.3-70b", Name: "Llama 3.3 70B", Provider: "groq", Model: "llama-3.3-70b-versatile", Description: "Lightning-fast open-source inference", Icon: "🦙", Recommended: true},
			{ID: "llama-3.1-8b", Name: "Llama 3.1 8B", Provider: "groq", Model: "llama-3.1-8b-instant", Description: "Ultra-fast instant responses", Icon: "⚡", Recommended: false},
		}
	case "mistral":
		return []ModelOption{
			{ID: "mistral-large", Name: "Mistral Large", Provider: "mistral", Model: "mistral-large-latest", Description: "Most capable Mistral model", Icon: "💨", Recommended: true},
			{ID: "mistral-small", Name: "Mistral Small", Provider: "mistral", Model: "mistral-small-latest", Description: "Fast and cost-effective", Icon: "⚡", Recommended: false},
		}
	case "deepseek":
		return []ModelOption{
			{ID: "deepseek-r1", Name: "DeepSeek R1", Provider: "deepseek", Model: "deepseek-r1", Description: "Advanced reasoning model", Icon: "🔍", Recommended: true},
			{ID: "deepseek-v3", Name: "DeepSeek V3", Provider: "deepseek", Model: "deepseek-chat", Description: "Fast general-purpose model", Icon: "⚡", Recommended: false},
		}
	case "xai":
		return []ModelOption{
			{ID: "grok-3", Name: "Grok 3", Provider: "xai", Model: "grok-3-latest", Description: "xAI's latest model", Icon: "🤖", Recommended: true},
		}
	case "openrouter":
		return []ModelOption{
			{ID: "openrouter-auto", Name: "Auto (best available)", Provider: "openrouter", Model: "openrouter/auto", Description: "Automatically routes to the best model", Icon: "🔀", Recommended: true},
		}
	case "together":
		return []ModelOption{
			{ID: "together-llama3", Name: "Llama 3.3 70B (Together)", Provider: "together", Model: "meta-llama/Llama-3.3-70B-Instruct-Turbo", Description: "Open-source inference", Icon: "🤝", Recommended: true},
		}
	case "perplexity":
		return []ModelOption{
			{ID: "sonar-pro", Name: "Sonar Pro", Provider: "perplexity", Model: "sonar-pro", Description: "Search-augmented answers", Icon: "🔭", Recommended: true},
		}
	case "cohere":
		return []ModelOption{
			{ID: "command-r-plus", Name: "Command R+", Provider: "cohere", Model: "command-r-plus", Description: "Best for RAG and complex tasks", Icon: "🧬", Recommended: true},
		}
	case "ollama":
		return []ModelOption{
			{ID: "ollama-llama3", Name: "Llama 3.2 (local)", Provider: "ollama", Model: "llama3.2", Description: "Run locally — no API costs", Icon: "🏠", Recommended: true},
			{ID: "ollama-mistral", Name: "Mistral 7B (local)", Provider: "ollama", Model: "mistral", Description: "Compact and capable", Icon: "💨", Recommended: false},
		}
	default:
		return nil
	}
}

// GetModelOptions returns the flat list of curated model presets used during
// the legacy onboarding flow (model_selection step). It wraps GetModelOptions
// from config for consistent preset definitions.
func GetModelOptions() []ModelOption {
	sharedOptions := config.GetModelOptions()
	options := make([]ModelOption, len(sharedOptions))
	for i, opt := range sharedOptions {
		options[i] = ModelOption{
			ID:          opt.ID,
			Name:        opt.Name,
			Provider:    opt.Provider,
			Model:       opt.Model,
			Description: opt.Description,
			Icon:        opt.Icon,
			Recommended: opt.Recommended,
		}
	}
	return options
}

// ModelOptions returns the currently available model presets.
func (s *OnboardingState) ModelOptions() []ModelOption {
	return GetModelOptions()
}

// SelectedPreset resolves the selected model ID to its full preset.
func (s *OnboardingState) SelectedPreset() (ModelOption, bool) {
	for _, option := range s.ModelOptions() {
		if option.ID == s.SelectedModel {
			return option, true
		}
	}
	return ModelOption{}, false
}

// SelectedModelID returns the full provider model name for the selected preset.
func (s *OnboardingState) SelectedModelID() string {
	if preset, ok := s.SelectedPreset(); ok {
		return preset.Model
	}
	return ""
}

// RecommendedPreset returns the first recommended model preset (or first option).
func (s *OnboardingState) RecommendedPreset() (ModelOption, int, bool) {
	options := s.ModelOptions()
	if len(options) == 0 {
		return ModelOption{}, -1, false
	}
	for i, option := range options {
		if option.Recommended {
			return option, i, true
		}
	}
	return options[0], 0, true
}

// ApplyRecommendedModel sets the selected provider/model to a recommended preset.
func (s *OnboardingState) ApplyRecommendedModel() (int, bool) {
	preset, idx, ok := s.RecommendedPreset()
	if !ok {
		return -1, false
	}
	s.SelectedModel = preset.ID
	s.SelectedProvider = preset.Provider
	return idx, true
}

// SetStepByName jumps the onboarding flow to a named step.
func (s *OnboardingState) SetStepByName(name string) bool {
	for i, step := range GetOnboardingSteps() {
		if step.Name == name {
			s.Step = i
			return true
		}
	}
	return false
}

// SetAPIKey stores a provider key in the correct state field.
func (s *OnboardingState) SetAPIKey(provider, key string) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	// Always update the unified APIKey field.
	s.APIKey = key
	// Also update legacy per-provider fields for TUI compatibility.
	switch provider {
	case "openai":
		s.OpenAIKey = key
	case "anthropic":
		s.AnthropicKey = key
	default:
		s.ProviderAPIKey = key
	}
}

// HasSavedAPIKey reports whether onboarding captured a key for the provider.
func (s *OnboardingState) HasSavedAPIKey(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "openai":
		return strings.TrimSpace(s.OpenAIKey) != ""
	case "anthropic":
		return strings.TrimSpace(s.AnthropicKey) != ""
	default:
		return strings.TrimSpace(s.ProviderAPIKey) != ""
	}
}

// ChannelOption represents a messaging platform that SoulGate can connect to.
type ChannelOption struct {
	ID          string
	Name        string
	Description string
	Icon        string
	Popular     bool
}

// GetAvailableChannels returns all messaging platforms available for setup.
func GetAvailableChannels() []ChannelOption {
	return []ChannelOption{
		{ID: "telegram", Name: "Telegram", Description: "Chat via Telegram bot", Icon: "✈", Popular: true},
		{ID: "discord", Name: "Discord", Description: "Connect to a Discord server", Icon: "🎮", Popular: true},
		{ID: "slack", Name: "Slack", Description: "Team communication and notifications", Icon: "💬", Popular: true},
		{ID: "whatsapp", Name: "WhatsApp", Description: "WhatsApp Business API", Icon: "📱", Popular: false},
		{ID: "matrix", Name: "Matrix", Description: "Open-source federated chat", Icon: "🔷", Popular: false},
		{ID: "irc", Name: "IRC", Description: "Classic internet relay chat", Icon: "💻", Popular: false},
	}
}

// ToggleChannel adds or removes a channel from the ChannelsToSetup list.
func (s *OnboardingState) ToggleChannel(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	for i, existing := range s.ChannelsToSetup {
		if existing == id {
			s.ChannelsToSetup = append(s.ChannelsToSetup[:i], s.ChannelsToSetup[i+1:]...)
			return
		}
	}
	s.ChannelsToSetup = append(s.ChannelsToSetup, id)
}

// SaveAPIKeys saves the configured API keys to config.
func (s *OnboardingState) SaveAPIKeys() error {
	cfg := s.Workspace.Config

	// Set default provider and selected model first.
	cfg.Model.DefaultProvider = s.SelectedProvider
	if preset, ok := s.SelectedPreset(); ok {
		switch preset.Provider {
		case "anthropic":
			cfg.Model.Anthropic.Model = preset.Model
		default:
			cfg.Model.OpenAI.Model = preset.Model
		}
	}

	// Update API keys.
	if s.OpenAIKey != "" {
		cfg.Model.OpenAI.APIKey = s.OpenAIKey
		os.Setenv("OPENAI_API_KEY", s.OpenAIKey)
	}
	if s.AnthropicKey != "" {
		cfg.Model.Anthropic.APIKey = s.AnthropicKey
		os.Setenv("ANTHROPIC_API_KEY", s.AnthropicKey)
	}

	// OpenAI-compatible providers store key in OpenAI config.
	if s.ProviderAPIKey != "" && s.SelectedProvider != "openai" && s.SelectedProvider != "anthropic" {
		cfg.Model.OpenAI.APIKey = s.ProviderAPIKey
		if env := providerEnvVar(s.SelectedProvider); env != "" {
			os.Setenv(env, s.ProviderAPIKey)
		}
	}

	// Apply custom base URL if set.
	if s.BaseURL != "" {
		cfg.Model.OpenAI.BaseURL = s.BaseURL
	}

	return s.Workspace.SaveConfig()
}

// MarkComplete marks onboarding as complete and writes the marker file.
func (s *OnboardingState) MarkComplete() error {
	s.Complete = true
	markerPath := filepath.Join(s.Workspace.ConfigDir, ".onboarding_complete")
	if err := os.WriteFile(markerPath, []byte("complete"), 0600); err != nil {
		return fmt.Errorf("failed to create onboarding marker: %w", err)
	}
	return nil
}

// IsOnboardingComplete reports whether onboarding has been completed for the
// given workspace by checking for the marker file.
func IsOnboardingComplete(workspace *config.Workspace) bool {
	markerPath := filepath.Join(workspace.ConfigDir, ".onboarding_complete")
	_, err := os.Stat(markerPath)
	return err == nil
}

// GetIntegrationRecommendations returns recommended integrations for quick setup.
func GetIntegrationRecommendations() []IntegrationRecommendation {
	return []IntegrationRecommendation{
		{ID: "slack", Name: "Slack", Icon: "💬", Description: "Team communication and notifications", Popular: true},
		{ID: "github", Name: "GitHub", Icon: "🐙", Description: "Code repositories and collaboration", Popular: true},
		{ID: "notion", Name: "Notion", Icon: "📝", Description: "Notes and knowledge management", Popular: true},
		{ID: "linear", Name: "Linear", Icon: "🎯", Description: "Issue tracking and project management", Popular: false},
	}
}

// IntegrationRecommendation represents a recommended integration.
type IntegrationRecommendation struct {
	ID          string
	Name        string
	Icon        string
	Description string
	Popular     bool
}

// GetTutorialSteps returns quick start tutorial steps.
func GetTutorialSteps() []TutorialStep {
	return []TutorialStep{
		{Title: "Ask Questions", Command: "What files are in this directory?", Desc: "The AI has full access to your system"},
		{Title: "Execute Commands", Command: "!git status", Desc: "Run shell commands with !"},
		{Title: "Use Integrations", Command: "Send a message to #general", Desc: "The AI uses your configured integrations"},
		{Title: "Switch Models", Command: "/model", Desc: "Choose different AI models on the fly"},
		{Title: "View Status", Command: "/status", Desc: "See current configuration and stats"},
	}
}

// TutorialStep represents a tutorial step.
type TutorialStep struct {
	Title   string
	Command string
	Desc    string
}

// ValidateOpenAIKey validates an OpenAI API key format.
// Deprecated: Use config.ValidateAPIKey("openai", key) instead.
func ValidateOpenAIKey(key string) error {
	return config.ValidateAPIKey("openai", key)
}

// ValidateAnthropicKey validates an Anthropic API key format.
// Deprecated: Use config.ValidateAPIKey("anthropic", key) instead.
func ValidateAnthropicKey(key string) error {
	return config.ValidateAPIKey("anthropic", key)
}

// InstallDependencies installs dependencies for all configured integrations.
func (s *OnboardingState) InstallDependencies(ctx context.Context) error {
	if len(s.IntegrationsToSetup) == 0 {
		return nil
	}

	s.InstallingDependencies = true

	soulGateDir := s.Workspace.ConfigDir
	installer := dependencies.NewDependencyInstaller(soulGateDir, true)

	// Check system prerequisites.
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

	// Install dependencies for each integration.
	for _, integrationID := range s.IntegrationsToSetup {
		s.DependencyStatus[integrationID] = "checking..."

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

// GetDependencyInstructions returns manual installation instructions for dependencies.
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

// providerEnvVar returns the environment variable name for a provider's API key.
func providerEnvVar(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "OPENAI_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "google":
		return "GOOGLE_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "mistral":
		return "MISTRAL_API_KEY"
	case "cohere":
		return "COHERE_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "xai":
		return "XAI_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "together":
		return "TOGETHER_API_KEY"
	case "perplexity":
		return "PERPLEXITY_API_KEY"
	case "ollama":
		return ""
	default:
		return strings.ToUpper(provider) + "_API_KEY"
	}
}
