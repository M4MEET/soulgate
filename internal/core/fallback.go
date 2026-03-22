package core

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
)

// FallbackProvider describes one entry in the fallback chain as understood by
// the core package.  It mirrors config.FallbackProviderConfig but is kept
// separate so the core layer does not depend on YAML tags.
type FallbackProvider struct {
	// Name is the canonical provider name (e.g. "groq", "ollama").
	Name string

	// Model is the model ID to use.  Empty means the provider registry default.
	Model string

	// Priority controls evaluation order — lower values are tried first.
	Priority int
}

// FallbackChain holds an ordered set of backup providers and coordinates
// thread-safe access.
type FallbackChain struct {
	providers []FallbackProvider
	mu        sync.RWMutex
}

// NewFallbackChain builds a FallbackChain from the workspace configuration.
// The slice is sorted by Priority ascending so that callers can iterate in
// order without knowing the original config ordering.
func NewFallbackChain(cfgEntries []config.FallbackProviderConfig) *FallbackChain {
	fc := &FallbackChain{}
	for _, e := range cfgEntries {
		fc.providers = append(fc.providers, FallbackProvider{
			Name:     e.Provider,
			Model:    e.Model,
			Priority: e.Priority,
		})
	}
	sort.Slice(fc.providers, func(i, j int) bool {
		return fc.providers[i].Priority < fc.providers[j].Priority
	})
	return fc
}

// Providers returns a snapshot of the ordered fallback providers.
func (fc *FallbackChain) Providers() []FallbackProvider {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	out := make([]FallbackProvider, len(fc.providers))
	copy(out, fc.providers)
	return out
}

// Len returns the number of entries in the chain.
func (fc *FallbackChain) Len() int {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return len(fc.providers)
}

// buildFallbackProvider constructs a model.Provider from a FallbackProvider
// descriptor using the same logic as initializeProvider.
func buildFallbackProvider(fp FallbackProvider) (model.Provider, error) {
	def, err := model.LookupProvider(fp.Name)
	if err != nil {
		return nil, fmt.Errorf("fallback provider %q: %w", fp.Name, err)
	}

	apiKey, err := model.ResolveAPIKey(def)
	if err != nil {
		return nil, fmt.Errorf("fallback provider %q: %w", fp.Name, err)
	}

	modelName := fp.Model
	if modelName == "" {
		modelName = def.DefaultModel
	}

	baseURL := model.ResolveBaseURL(def, "")

	switch def.Protocol {
	case "anthropic":
		return anthropic.NewProvider(apiKey, modelName, baseURL), nil
	default:
		return openai.NewProvider(apiKey, modelName, baseURL), nil
	}
}

// callModelWithFallback attempts the primary provider first, then walks the
// fallback chain on retryable errors.  It restores the original provider after
// the call regardless of outcome so the Orchestrator's state is not mutated.
//
// The method is intentionally kept separate from callModelWithRetry: the retry
// loop handles transient failures against a single provider; the fallback chain
// handles provider-level failures where the entire endpoint is unavailable or
// rate-limited.
func (o *Orchestrator) callModelWithFallback(
	ctx context.Context,
	tracker *ExecutionTracker,
	req model.CompletionRequest,
) (*model.CompletionResponse, error) {
	// Fast path: no fallback chain configured.
	if o.fallbackChain == nil || o.fallbackChain.Len() == 0 {
		return o.callModelWithRetry(ctx, tracker, req)
	}

	// Try primary provider first (with its own retry budget).
	primaryProvider := o.provider
	resp, primaryErr := o.callModelWithRetry(ctx, tracker, req)
	if primaryErr == nil {
		return resp, nil
	}

	// Only walk the fallback chain for retryable/transient errors.  Hard
	// failures (auth errors, invalid requests) should surface immediately.
	if !isRetryableModelError(ctx, primaryErr) {
		return nil, primaryErr
	}

	// Walk the chain.
	for _, fp := range o.fallbackChain.Providers() {
		fallbackProv, err := buildFallbackProvider(fp)
		if err != nil {
			// Log the build failure but keep trying other providers.
			o.emitThinking(ThinkingEvent{
				Kind:    ThinkingStatus,
				Message: fmt.Sprintf("fallback provider %q unavailable: %v", fp.Name, err),
			})
			continue
		}

		o.emitThinking(ThinkingEvent{
			Kind:     ThinkingStatus,
			Provider: fp.Name,
			Message: fmt.Sprintf(
				"primary provider failed (%v), falling back to %s/%s",
				primaryErr, fp.Name, fp.Model,
			),
		})

		// Audit the fallback decision.
		auditEvent := audit.NewEvent(audit.EventModelCall, audit.CategoryModel).
			WithSessionID(o.session.ID).
			WithMetadata("fallback_from", primaryProvider.Name()).
			WithMetadata("fallback_to", fp.Name).
			WithMetadata("fallback_model", fp.Model).
			WithMetadata("primary_error", primaryErr.Error())
		if logErr := o.audit.Log(ctx, auditEvent); logErr != nil {
			// Non-fatal; proceed.
			o.emitThinking(ThinkingEvent{
				Kind:    ThinkingStatus,
				Message: fmt.Sprintf("audit log failed during fallback: %v", logErr),
			})
		}

		// Temporarily switch the provider in the orchestrator so that
		// callModelWithRetry (and the streaming path inside it) uses it.
		o.provider = fallbackProv
		resp, err = o.callModelWithRetry(ctx, tracker, req)

		// Restore the original provider unconditionally.
		o.provider = primaryProvider

		if err == nil {
			return resp, nil
		}

		// If this fallback also failed with a retryable error, try the next.
		if !isRetryableModelError(ctx, err) {
			return nil, fmt.Errorf("fallback provider %q failed (non-retryable): %w", fp.Name, err)
		}

		o.emitThinking(ThinkingEvent{
			Kind:    ThinkingStatus,
			Message: fmt.Sprintf("fallback provider %q also failed: %v", fp.Name, err),
		})
	}

	// All providers exhausted; return the original primary error so the caller
	// sees the root cause rather than the last fallback error.
	return nil, fmt.Errorf("all providers failed; primary error: %w", primaryErr)
}
