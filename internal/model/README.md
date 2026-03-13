# Model Integration Agent

**Owner**: Agent 6 - Model Integration Specialist
**Independence Level**: HIGH
**Status**: Active (Under Development)

## Overview

LLM provider adapters and tool calling integration. Supports multiple providers (OpenAI, Anthropic, local models) with unified interface.

## Responsibilities

- LLM provider interface
- OpenAI adapter (function calling)
- Anthropic adapter (tool use)
- Tool schema conversion
- Streaming support (planned)
- Token counting and cost tracking (planned)

## Package Structure

```
internal/model/
├── README.md
├── provider.go        # Provider interface
├── schema.go          # Tool schema (CRITICAL INTERFACE)
├── openai/
│   └── client.go      # OpenAI adapter
└── anthropic/
    └── client.go      # Anthropic adapter
```

## Core Interfaces

```go
type Provider interface {
    Complete(ctx, req CompletionRequest) (*CompletionResponse, error)
    Name() string
    SupportedFeatures() FeatureSet
}
```

```go
type ToolSchema struct {
    Name        string
    Description string
    InputSchema interface{} // JSON Schema
}
```

**WARNING**: ToolSchema format must be compatible with OpenAI/Anthropic specs.

## Dependencies

- External: OpenAI API, Anthropic API
- No internal SoulGate dependencies

## Usage

```go
// OpenAI provider
provider := openai.NewProvider(apiKey)
resp, err := provider.Complete(ctx, req)

// Anthropic provider
provider := anthropic.NewProvider(apiKey)
resp, err := provider.Complete(ctx, req)
```

## Testing

**Coverage Target**: 80%+
**Current**: No tests

### Test Requirements
- Unit tests with mocks
- Schema conversion tests
- Integration tests with real APIs (API key required)
- Error handling tests

## Planned Work

- [ ] Complete OpenAI adapter (function calling)
- [ ] Complete Anthropic adapter (tool use)
- [ ] Add streaming response support
- [ ] Token counting and cost tracking
- [ ] Provider fallback/retry logic
- [ ] Local model support (Ollama, LM Studio)

## Coordination

- ToolSchema changes → coordinate with Plugin and Orchestration agents
- Provider config → coordinate with Config agent

## Contact

**Owner**: @model-agent
