package core

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/stretchr/testify/require"
)

type tokenUsageTestProvider struct{}

func (p *tokenUsageTestProvider) Complete(ctx context.Context, req model.CompletionRequest) (*model.CompletionResponse, error) {
	return &model.CompletionResponse{
		Message: model.Message{
			Role:    model.RoleAssistant,
			Content: "done",
		},
		StopReason: model.StopReasonEndTurn,
		Model:      "token-test-model",
		Usage: model.TokenUsage{
			TotalTokens: 123,
		},
	}, nil
}

func (p *tokenUsageTestProvider) StreamComplete(ctx context.Context, req model.CompletionRequest) (<-chan model.StreamChunk, error) {
	ch := make(chan model.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *tokenUsageTestProvider) Name() string {
	return "token-test-provider"
}

func (p *tokenUsageTestProvider) SupportedFeatures() model.FeatureSet {
	return model.FeatureSet{
		ToolCalling:     true,
		SystemMessages:  true,
		StreamingOutput: true,
	}
}

func TestExecuteAgenticLoopEmitsThinkingTokenUsage(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Workspace.Root = tmpDir
	cfg.Workspace.ConfigDir = filepath.Join(tmpDir, ".soulgate")

	auditLogger, err := audit.NewJSONLLogger(filepath.Join(tmpDir, "audit.jsonl"))
	require.NoError(t, err)
	defer auditLogger.Close()

	orch := &Orchestrator{
		workspace:           &config.Workspace{Root: tmpDir, ConfigDir: cfg.Workspace.ConfigDir, Config: cfg},
		audit:               auditLogger,
		session:             NewSession(tmpDir),
		provider:            &tokenUsageTestProvider{},
		integrationsReg:     integrations.NewRegistry(),
		toolRegistry:        NewToolRegistry(),
		directives:          DefaultDirectives(),
		loopDetector:        NewLoopDetector(),
		conversationHistory: []model.Message{},
	}

	var events []ThinkingEvent
	orch.SetThinkingCallback(func(event ThinkingEvent) {
		events = append(events, event)
	})

	resp, err := orch.executeAgenticLoop(context.Background(), "hello", "run-1")
	require.NoError(t, err)
	require.Equal(t, "done", resp)

	var tokenEvent *ThinkingEvent
	for i := range events {
		if events[i].Kind == ThinkingTokenUsage {
			tokenEvent = &events[i]
			break
		}
	}

	require.NotNil(t, tokenEvent, "expected ThinkingTokenUsage event")
	require.Equal(t, 123, tokenEvent.TokensUsed)
}
