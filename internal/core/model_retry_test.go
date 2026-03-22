package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/M4MEET/soulgate/internal/model"
)

type scriptedProvider struct {
	errs  []error
	resps []*model.CompletionResponse
	calls int
}

func (p *scriptedProvider) Complete(ctx context.Context, req model.CompletionRequest) (*model.CompletionResponse, error) {
	idx := p.calls
	p.calls++

	if idx < len(p.errs) && p.errs[idx] != nil {
		return nil, p.errs[idx]
	}
	if idx < len(p.resps) && p.resps[idx] != nil {
		return p.resps[idx], nil
	}

	return &model.CompletionResponse{
		Message: model.Message{
			Role:    model.RoleAssistant,
			Content: "ok",
		},
		StopReason: model.StopReasonEndTurn,
	}, nil
}

func (p *scriptedProvider) StreamComplete(ctx context.Context, req model.CompletionRequest) (<-chan model.StreamChunk, error) {
	return nil, fmt.Errorf("streaming not used in this test")
}

func (p *scriptedProvider) Name() string {
	return "test"
}

func (p *scriptedProvider) SupportedFeatures() model.FeatureSet {
	return model.FeatureSet{}
}

func TestCallModelWithRetry_RetriesTransientError(t *testing.T) {
	origSleep := modelCallSleep
	modelCallSleep = func(ctx context.Context, d time.Duration) error { return nil }
	t.Cleanup(func() { modelCallSleep = origSleep })

	provider := &scriptedProvider{
		errs: []error{
			fmt.Errorf("failed to send request: %w", context.DeadlineExceeded),
		},
		resps: []*model.CompletionResponse{
			nil,
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "recovered",
				},
				StopReason: model.StopReasonEndTurn,
			},
		},
	}

	orch := &Orchestrator{provider: provider}
	tracker := NewExecutionTracker(DefaultExecutionLimits())

	resp, err := orch.callModelWithRetry(context.Background(), tracker, model.CompletionRequest{})
	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}
	if provider.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", provider.calls)
	}
	if resp == nil || resp.Message.Content != "recovered" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestCallModelWithRetry_DoesNotRetryOnNonTransientError(t *testing.T) {
	origSleep := modelCallSleep
	modelCallSleep = func(ctx context.Context, d time.Duration) error { return nil }
	t.Cleanup(func() { modelCallSleep = origSleep })

	provider := &scriptedProvider{
		errs: []error{
			fmt.Errorf("API error (status 400): bad request"),
		},
		resps: []*model.CompletionResponse{
			nil,
			{
				Message: model.Message{
					Role:    model.RoleAssistant,
					Content: "should-not-happen",
				},
				StopReason: model.StopReasonEndTurn,
			},
		},
	}

	orch := &Orchestrator{provider: provider}
	tracker := NewExecutionTracker(DefaultExecutionLimits())

	_, err := orch.callModelWithRetry(context.Background(), tracker, model.CompletionRequest{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if provider.calls != 1 {
		t.Fatalf("expected 1 call, got %d", provider.calls)
	}
}

func TestIsRetryableModelError_ContextCanceledStopsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := fmt.Errorf("failed to send request: %w", context.DeadlineExceeded)
	if isRetryableModelError(ctx, err) {
		t.Fatalf("expected non-retryable when parent context is canceled")
	}
}

func TestParseAPIStatusCode(t *testing.T) {
	code, ok := parseAPIStatusCode("API error (status 503): overloaded")
	if !ok {
		t.Fatalf("expected status code parse success")
	}
	if code != 503 {
		t.Fatalf("expected 503, got %d", code)
	}
}
