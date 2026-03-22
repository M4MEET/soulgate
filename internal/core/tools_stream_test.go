package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/stretchr/testify/require"
)

type streamingTestProvider struct{}

func (p *streamingTestProvider) Complete(ctx context.Context, req model.CompletionRequest) (*model.CompletionResponse, error) {
	return nil, context.Canceled
}

func (p *streamingTestProvider) StreamComplete(ctx context.Context, req model.CompletionRequest) (<-chan model.StreamChunk, error) {
	ch := make(chan model.StreamChunk, 4)
	go func() {
		defer close(ch)

		select {
		case <-ctx.Done():
			ch <- model.StreamChunk{Error: ctx.Err()}
			return
		case <-time.After(20 * time.Millisecond):
		}

		ch <- model.StreamChunk{Delta: "ok"}
		ch <- model.StreamChunk{
			Done: true,
			Response: &model.CompletionResponse{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "done",
				},
				StopReason: model.StopReasonEndTurn,
				Model:      "test-model",
				Usage: model.TokenUsage{
					TotalTokens: 1,
				},
			},
		}
	}()
	return ch, nil
}

func (p *streamingTestProvider) Name() string {
	return "test-provider"
}

func (p *streamingTestProvider) SupportedFeatures() model.FeatureSet {
	return model.FeatureSet{
		ToolCalling:     true,
		SystemMessages:  true,
		StreamingOutput: true,
	}
}

func TestExecuteAgenticLoopStreamingDoesNotCancelAPICallImmediately(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Workspace.Root = tmpDir
	cfg.Workspace.ConfigDir = filepath.Join(tmpDir, ".soulgate")

	auditLogger, err := audit.NewJSONLLogger(filepath.Join(tmpDir, "audit.jsonl"))
	require.NoError(t, err)
	defer auditLogger.Close()

	orch := &Orchestrator{
		workspace:       &config.Workspace{Root: tmpDir, ConfigDir: cfg.Workspace.ConfigDir, Config: cfg},
		audit:           auditLogger,
		session:         NewSession(tmpDir),
		provider:        &streamingTestProvider{},
		integrationsReg: integrations.NewRegistry(),
		toolRegistry:    NewToolRegistry(),
		directives:      DefaultDirectives(),
		loopDetector:    NewLoopDetector(),
		streaming:       true,
		streamCallback:  func(_ string) {},
	}

	resp, err := orch.executeAgenticLoop(context.Background(), "hello", "run-1")
	require.NoError(t, err)
	require.Equal(t, "done", resp)
}
