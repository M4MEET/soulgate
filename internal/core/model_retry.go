package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/model"
)

const (
	modelCallMaxAttempts   = 3
	modelCallRetryMinDelay = 500 * time.Millisecond
	modelCallRetryMaxDelay = 2 * time.Second
)

var modelCallSleep = sleepWithContext

// callModelWithRetry executes a model call with bounded retries for transient
// transport/API failures.
func (o *Orchestrator) callModelWithRetry(
	ctx context.Context,
	tracker *ExecutionTracker,
	req model.CompletionRequest,
) (*model.CompletionResponse, error) {
	attempts := modelCallMaxAttempts
	if o.streaming && o.streamCallback != nil {
		// Streaming retries are only safe before any output is emitted.
		attempts = 2
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		apiCtx, apiCancel := tracker.APICallContext(ctx)
		resp, streamedOutput, err := o.callModelOnce(apiCtx, req)
		apiCancel()
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Do not retry if:
		// - partial stream already emitted
		// - error is non-transient
		// - retries exhausted
		if streamedOutput || attempt == attempts || !isRetryableModelError(ctx, err) {
			return nil, err
		}

		o.emitThinking(ThinkingEvent{
			Kind:     ThinkingStatus,
			Provider: o.provider.Name(),
			Message:  fmt.Sprintf("transient model error, retrying (%d/%d)", attempt, attempts-1),
		})

		if err := modelCallSleep(ctx, modelCallRetryDelay(attempt)); err != nil {
			return nil, fmt.Errorf("%w (retry interrupted: %v)", lastErr, err)
		}
	}

	return nil, lastErr
}

func (o *Orchestrator) callModelOnce(
	ctx context.Context,
	req model.CompletionRequest,
) (*model.CompletionResponse, bool, error) {
	if o.streaming && o.streamCallback != nil {
		streamCh, err := o.provider.StreamComplete(ctx, req)
		if err != nil {
			return nil, false, err
		}

		var streamedOutput bool
		var resp *model.CompletionResponse
		for chunk := range streamCh {
			if chunk.Error != nil {
				return nil, streamedOutput, fmt.Errorf("streaming error: %w", chunk.Error)
			}
			if chunk.Delta != "" {
				streamedOutput = true
				o.streamCallback(chunk.Delta)
			}
			if chunk.Done && chunk.Response != nil {
				resp = chunk.Response
			}
		}

		if resp == nil {
			return nil, streamedOutput, fmt.Errorf("streaming completed without final response")
		}
		return resp, streamedOutput, nil
	}

	resp, err := o.provider.Complete(ctx, req)
	if err != nil {
		return nil, false, err
	}
	return resp, false, nil
}

func modelCallRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := modelCallRetryMinDelay * time.Duration(1<<(attempt-1))
	if delay > modelCallRetryMaxDelay {
		return modelCallRetryMaxDelay
	}
	return delay
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableModelError(parentCtx context.Context, err error) bool {
	if err == nil {
		return false
	}

	// Parent run context cancellation/deadline means stop immediately.
	if parentCtx.Err() != nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	if status, ok := parseAPIStatusCode(err.Error()); ok {
		switch status {
		case 408, 425, 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}

	msg := strings.ToLower(err.Error())
	transientMarkers := []string{
		"failed to send request: eof",
		"context deadline exceeded",
		"timeout",
		"temporarily unavailable",
		"temporary failure",
		"connection reset by peer",
		"connection refused",
		"broken pipe",
		"unexpected eof",
		"http2: client connection lost",
		"server closed idle connection",
		"tls handshake timeout",
	}
	for _, marker := range transientMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}

	return false
}

func parseAPIStatusCode(errMsg string) (int, bool) {
	lower := strings.ToLower(errMsg)
	const marker = "api error (status "
	start := strings.Index(lower, marker)
	if start == -1 {
		return 0, false
	}
	start += len(marker)
	end := start
	for end < len(lower) && lower[end] >= '0' && lower[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}

	code, err := strconv.Atoi(lower[start:end])
	if err != nil {
		return 0, false
	}
	return code, true
}
