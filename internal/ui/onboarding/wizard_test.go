package onboarding

import (
	"path/filepath"
	"testing"

	"github.com/M4MEET/soulgate/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSaveAPIKeysAnthropicSelection(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Workspace.Root = tmp
	cfg.Workspace.ConfigDir = tmp

	workspace := &config.Workspace{
		Root:      tmp,
		ConfigDir: tmp,
		Config:    cfg,
	}

	state := NewOnboardingState(workspace)
	state.SelectedProvider = "anthropic"
	state.SelectedModel = "claude-sonnet-5"
	state.AnthropicKey = "sk-ant-test-key-1234567890"

	require.NoError(t, state.SaveAPIKeys())
	require.Equal(t, "anthropic", workspace.Config.Model.DefaultProvider)
	require.Equal(t, "claude-sonnet-5", workspace.Config.Model.Anthropic.Model)
	require.Equal(t, "sk-ant-test-key-1234567890", workspace.Config.Model.Anthropic.APIKey)

	// Ensure save path exists.
	require.FileExists(t, filepath.Join(tmp, "config.yml"))
}

func TestSaveAPIKeysOpenAICompatibleSelection(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Workspace.Root = tmp
	cfg.Workspace.ConfigDir = tmp

	workspace := &config.Workspace{
		Root:      tmp,
		ConfigDir: tmp,
		Config:    cfg,
	}

	state := NewOnboardingState(workspace)
	state.SelectedProvider = "groq"
	state.SelectedModel = "llama-3.3-70b"
	state.ProviderAPIKey = "gsk_test_key_1234567890"

	require.NoError(t, state.SaveAPIKeys())
	require.Equal(t, "groq", workspace.Config.Model.DefaultProvider)
	require.Equal(t, "llama-3.3-70b-versatile", workspace.Config.Model.OpenAI.Model)
	require.Equal(t, "gsk_test_key_1234567890", workspace.Config.Model.OpenAI.APIKey)
}

func TestSetAPIKeyAndHasSavedAPIKey(t *testing.T) {
	state := NewOnboardingState(&config.Workspace{Config: config.DefaultConfig()})

	state.SetAPIKey("openai", "sk-test")
	require.True(t, state.HasSavedAPIKey("openai"))
	require.Equal(t, "sk-test", state.OpenAIKey)

	state.SetAPIKey("anthropic", "sk-ant-test")
	require.True(t, state.HasSavedAPIKey("anthropic"))
	require.Equal(t, "sk-ant-test", state.AnthropicKey)

	state.SetAPIKey("groq", "gsk-test")
	require.True(t, state.HasSavedAPIKey("groq"))
	require.Equal(t, "gsk-test", state.ProviderAPIKey)
}

func TestApplyRecommendedModel(t *testing.T) {
	state := NewOnboardingState(&config.Workspace{Config: config.DefaultConfig()})

	idx, ok := state.ApplyRecommendedModel()
	require.True(t, ok)
	require.GreaterOrEqual(t, idx, 0)
	require.NotEmpty(t, state.SelectedProvider)
	require.NotEmpty(t, state.SelectedModel)

	preset, found := state.SelectedPreset()
	require.True(t, found)
	require.Equal(t, preset.Provider, state.SelectedProvider)
	require.Equal(t, preset.ID, state.SelectedModel)
}

func TestSetStepByName(t *testing.T) {
	state := NewOnboardingState(&config.Workspace{Config: config.DefaultConfig()})

	require.True(t, state.SetStepByName("integrations"))
	require.Equal(t, "integrations", state.GetCurrentStep().Name)

	require.False(t, state.SetStepByName("missing-step"))
	require.Equal(t, "integrations", state.GetCurrentStep().Name)
}
