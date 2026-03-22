package tui

import (
	"testing"

	"github.com/M4MEET/soulgate/internal/ui/onboarding"
	"github.com/stretchr/testify/require"
)

func TestMoveOnboardingSelectionWraps(t *testing.T) {
	require.Equal(t, 0, moveOnboardingSelection(0, 1, 1))
	require.Equal(t, 2, moveOnboardingSelection(0, -1, 3))
	require.Equal(t, 0, moveOnboardingSelection(2, 1, 3))
	require.Equal(t, 1, moveOnboardingSelection(0, 4, 3))
}

func TestProceedFromIntegrationsStepSkipsDependenciesWhenEmpty(t *testing.T) {
	m := InteractiveChatModel{
		OnboardingState: &onboarding.OnboardingState{},
	}

	require.True(t, m.OnboardingState.SetStepByName("integrations"))

	cmd := m.proceedFromIntegrationsStep()
	require.Nil(t, cmd)
	require.Equal(t, "tutorial", m.OnboardingState.GetCurrentStep().Name)

	m.OnboardingState.QuickMode = true
	require.True(t, m.OnboardingState.SetStepByName("integrations"))

	cmd = m.proceedFromIntegrationsStep()
	require.Nil(t, cmd)
	require.Equal(t, "complete", m.OnboardingState.GetCurrentStep().Name)
}
