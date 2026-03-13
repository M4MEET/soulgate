package core

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers/exec"
	"github.com/M4MEET/soulgate/internal/brokers/files"
	"github.com/M4MEET/soulgate/internal/brokers/net"
	"github.com/M4MEET/soulgate/internal/config"
	"github.com/M4MEET/soulgate/internal/integrations"
	"github.com/M4MEET/soulgate/internal/integrations/analytics"
	"github.com/M4MEET/soulgate/internal/integrations/aws"
	"github.com/M4MEET/soulgate/internal/integrations/database"
	"github.com/M4MEET/soulgate/internal/integrations/docker"
	"github.com/M4MEET/soulgate/internal/integrations/github"
	"github.com/M4MEET/soulgate/internal/integrations/google"
	"github.com/M4MEET/soulgate/internal/integrations/notion"
	"github.com/M4MEET/soulgate/internal/integrations/slack"
	"github.com/M4MEET/soulgate/internal/model"
	"github.com/M4MEET/soulgate/internal/model/anthropic"
	"github.com/M4MEET/soulgate/internal/model/openai"
	"github.com/M4MEET/soulgate/internal/policy"
	"github.com/M4MEET/soulgate/internal/tools/cron"
	"github.com/M4MEET/soulgate/internal/tools/process"
)

// Orchestrator coordinates model calls, plugin execution, and broker access
type Orchestrator struct {
	workspace          *config.Workspace
	audit              audit.Logger
	session            *Session
	provider           model.Provider
	policyEngine       *policy.Engine
	fileBroker         *files.Broker
	execBroker         *exec.Broker
	netBroker          *net.Broker
	integrationsReg    *integrations.Registry
	integrationsStore  *integrations.Store
	memoryStore        *MemoryStore
	agentManager       *AgentManager
	processManager     *process.Manager
	cronScheduler      *cron.Scheduler
	directives         *Directives
	loopDetector       *LoopDetector
	streaming          bool
	streamCallback     func(chunk string)        // Called for each streamed token
	thinkingCallback   func(event ThinkingEvent) // Called for live thinking output
	permissionCallback PermissionCallback        // Optional: called when permission is needed
	actualModelName    string                    // Actual model name from last API response
}

// RunResult represents the result of a run
type RunResult struct {
	RunID    string
	Response string
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(workspace *config.Workspace) (*Orchestrator, error) {
	// Initialize audit logger
	auditLogger, err := audit.NewSQLiteLogger(workspace.Config.Audit.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	// Initialize policy engine
	policyEngine, err := initializePolicyEngine(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize policy engine: %w", err)
	}

	// Initialize file broker
	fileBroker, err := files.NewBroker(workspace.Root, policyEngine, auditLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create file broker: %w", err)
	}

	// Initialize exec broker
	execBroker, err := exec.NewBroker(workspace.Root, policyEngine, auditLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec broker: %w", err)
	}

	// Initialize network broker
	netBroker, err := net.NewBroker(policyEngine, auditLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create network broker: %w", err)
	}

	// Initialize integrations
	integrationsStore, err := integrations.NewStore(workspace.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create integrations store: %w", err)
	}

	integrationsReg := integrations.NewRegistry()

	// Register available integrations
	if err := integrationsReg.Register(github.New()); err != nil {
		return nil, fmt.Errorf("failed to register github integration: %w", err)
	}
	if err := integrationsReg.Register(slack.New()); err != nil {
		return nil, fmt.Errorf("failed to register slack integration: %w", err)
	}
	if err := integrationsReg.Register(database.NewPostgres()); err != nil {
		return nil, fmt.Errorf("failed to register postgres integration: %w", err)
	}
	if err := integrationsReg.Register(google.NewDrive()); err != nil {
		return nil, fmt.Errorf("failed to register google_drive integration: %w", err)
	}
	if err := integrationsReg.Register(google.NewGmail()); err != nil {
		return nil, fmt.Errorf("failed to register gmail integration: %w", err)
	}
	if err := integrationsReg.Register(docker.New()); err != nil {
		return nil, fmt.Errorf("failed to register docker integration: %w", err)
	}
	if err := integrationsReg.Register(aws.NewS3()); err != nil {
		return nil, fmt.Errorf("failed to register aws_s3 integration: %w", err)
	}
	if err := integrationsReg.Register(notion.New()); err != nil {
		return nil, fmt.Errorf("failed to register notion integration: %w", err)
	}
	if err := integrationsReg.Register(analytics.New()); err != nil {
		return nil, fmt.Errorf("failed to register analytics integration: %w", err)
	}

	// Load and configure integrations from store
	for _, name := range integrationsStore.List() {
		config, _ := integrationsStore.Get(name)
		integration, err := integrationsReg.Get(name)
		if err == nil {
			integration.Setup(context.Background(), config)
		}
	}

	// Initialize memory store
	memoryStore, err := NewMemoryStore(workspace.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create memory store: %w", err)
	}

	// Initialize model provider
	provider, err := initializeProvider(workspace.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model provider: %w", err)
	}

	// Create session
	session := NewSession(workspace.Root)

	// Log session start
	event := audit.NewEvent(audit.EventSessionStart, audit.CategorySession).
		WithSessionID(session.ID).
		WithMetadata("workspace", workspace.Root).
		WithMetadata("provider", provider.Name())

	if err := auditLogger.Log(context.Background(), event); err != nil {
		return nil, fmt.Errorf("failed to log session start: %w", err)
	}

	return &Orchestrator{
		workspace:         workspace,
		audit:             auditLogger,
		session:           session,
		provider:          provider,
		policyEngine:      policyEngine,
		fileBroker:        fileBroker,
		execBroker:        execBroker,
		netBroker:         netBroker,
		integrationsReg:   integrationsReg,
		integrationsStore: integrationsStore,
		memoryStore:       memoryStore,
		agentManager:      NewAgentManager(),
		processManager:    process.NewManager(),
		cronScheduler:     cron.NewScheduler(workspace.ConfigDir),
		directives:        DefaultDirectives(),
		loopDetector:      NewLoopDetector(),
	}, nil
}

// initializePolicyEngine initializes the policy engine from workspace config
func initializePolicyEngine(workspace *config.Workspace) (*policy.Engine, error) {
	policyPath := workspace.Config.Policy.FilePath

	// Check if policy file exists
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		// No policy file - use default deny
		return policy.NewEngine(nil), nil
	}

	// Load policy
	pol, err := policy.LoadPolicy(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	return policy.NewEngine(pol), nil
}

// initializeProvider initializes the model provider based on config
func initializeProvider(cfg *config.Config) (model.Provider, error) {
	switch cfg.Model.DefaultProvider {
	case "openai":
		apiKey := cfg.Model.OpenAI.APIKey
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured (set OPENAI_API_KEY environment variable)")
		}
		return openai.NewProvider(
			apiKey,
			cfg.Model.OpenAI.Model,
			cfg.Model.OpenAI.BaseURL,
		), nil

	case "anthropic":
		apiKey := cfg.Model.Anthropic.APIKey
		if apiKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured (set ANTHROPIC_API_KEY environment variable)")
		}
		return anthropic.NewProvider(
			apiKey,
			cfg.Model.Anthropic.Model,
			cfg.Model.Anthropic.BaseURL,
		), nil

	// OpenAI-compatible providers (use OpenAI client with custom base URL)
	case "groq", "google", "mistral", "cohere", "deepseek", "openrouter", "together", "perplexity", "xai", "azure", "custom":
		apiKey := cfg.Model.OpenAI.APIKey
		if apiKey == "" {
			return nil, fmt.Errorf("%s API key not configured", cfg.Model.DefaultProvider)
		}
		return openai.NewProvider(
			apiKey,
			cfg.Model.OpenAI.Model,
			cfg.Model.OpenAI.BaseURL,
		), nil

	// Local providers (no API key required)
	case "ollama":
		// Ollama doesn't require an API key
		return openai.NewProvider(
			"ollama-no-key-needed",
			cfg.Model.OpenAI.Model,
			cfg.Model.OpenAI.BaseURL,
		), nil

	default:
		return nil, fmt.Errorf("unsupported model provider: %s\n\nSupported providers:\n  - openai, anthropic (native support)\n  - groq, google, mistral, cohere, deepseek, openrouter, together, perplexity, xai (OpenAI-compatible)\n  - ollama (local), azure, custom (advanced)\n\nRun 'soulgate chat' to configure a provider", cfg.Model.DefaultProvider)
	}
}

// Run executes a run with the given prompt
func (o *Orchestrator) Run(ctx context.Context, prompt string) (*RunResult, error) {
	// Create run
	run := o.session.CreateRun(prompt)
	run.Start()

	// Log run start
	event := audit.NewEvent(audit.EventRunStart, audit.CategoryRun).
		WithSessionID(o.session.ID).
		WithRunID(run.ID).
		WithMetadata("prompt", prompt)

	if err := o.audit.Log(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to log run start: %w", err)
	}

	// Execute agentic loop
	response, err := o.executeAgenticLoop(ctx, prompt, run.ID)
	if err != nil {
		run.SetResult(fmt.Sprintf("Error: %v", err))

		// Log run error with fresh context (in case parent context timed out)
		auditCtx, cancel := auditContext()
		defer cancel()

		errorEvent := audit.NewEvent(audit.EventRunComplete, audit.CategoryRun).
			WithSessionID(o.session.ID).
			WithRunID(run.ID).
			WithMetadata("error", err.Error()).
			WithStatus(audit.StatusError)

		if logErr := o.audit.Log(auditCtx, errorEvent); logErr != nil {
			// Don't fail the whole operation if audit logging fails
			// Just log to stderr for debugging
			fmt.Fprintf(os.Stderr, "Warning: failed to log run error to audit: %v\n", logErr)
		}

		return nil, err
	}

	result := &RunResult{
		RunID:    run.ID,
		Response: response,
	}

	run.SetResult(result.Response)

	// Log run complete with fresh context
	auditCtx, cancel := auditContext()
	defer cancel()

	completeEvent := audit.NewEvent(audit.EventRunComplete, audit.CategoryRun).
		WithSessionID(o.session.ID).
		WithRunID(run.ID).
		WithMetadata("result", result.Response)

	if err := o.audit.Log(auditCtx, completeEvent); err != nil {
		// Don't fail the operation if audit logging fails
		fmt.Fprintf(os.Stderr, "Warning: failed to log run complete to audit: %v\n", err)
	}

	return result, nil
}

// GetSession returns the current session
func (o *Orchestrator) GetSession() *Session {
	return o.session
}

// GetAuditLogger returns the audit logger
func (o *Orchestrator) GetAuditLogger() audit.Logger {
	return o.audit
}

// GetWorkspace returns the current workspace
func (o *Orchestrator) GetWorkspace() *config.Workspace {
	return o.workspace
}

// GetMemoryStore returns the memory store
func (o *Orchestrator) GetMemoryStore() *MemoryStore {
	return o.memoryStore
}

// SetStreaming enables or disables streaming mode
func (o *Orchestrator) SetStreaming(enabled bool, callback func(chunk string)) {
	o.streaming = enabled
	o.streamCallback = callback
}

// IsStreaming returns whether streaming mode is enabled
func (o *Orchestrator) IsStreaming() bool {
	return o.streaming
}

// GetAgentManager returns the agent manager
func (o *Orchestrator) GetAgentManager() *AgentManager {
	return o.agentManager
}

// GetProcessManager returns the process manager
func (o *Orchestrator) GetProcessManager() *process.Manager {
	return o.processManager
}

// GetCronScheduler returns the cron scheduler
func (o *Orchestrator) GetCronScheduler() *cron.Scheduler {
	return o.cronScheduler
}

// GetDirectives returns the current session directives
func (o *Orchestrator) GetDirectives() *Directives {
	return o.directives
}

// GetLoopDetector returns the loop detector
func (o *Orchestrator) GetLoopDetector() *LoopDetector {
	return o.loopDetector
}

// Close cleans up resources
func (o *Orchestrator) Close() error {
	// Stop cron scheduler
	o.cronScheduler.Stop()

	// Log session end
	event := audit.NewEvent(audit.EventSessionEnd, audit.CategorySession).
		WithSessionID(o.session.ID)

	if err := o.audit.Log(context.Background(), event); err != nil {
		return fmt.Errorf("failed to log session end: %w", err)
	}

	// Close audit logger
	if err := o.audit.Close(); err != nil {
		return fmt.Errorf("failed to close audit logger: %w", err)
	}

	return nil
}

// auditContext creates a fresh context for audit logging with a reasonable timeout.
// This ensures audit logging doesn't fail even if the parent context is cancelled.
func auditContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
