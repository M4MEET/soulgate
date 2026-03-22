package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

// HealthStatus is the rich payload returned by the /health and /api/health endpoints.
type HealthStatus struct {
	Status    string         `json:"status"` // "healthy", "degraded", "unhealthy"
	Uptime    string         `json:"uptime"`
	StartedAt time.Time      `json:"started_at"`
	Clients   map[string]int `json:"clients"` // role -> count
	Sessions  int            `json:"sessions"`
	Provider  string         `json:"provider"`
	Model     string         `json:"model"`
	Memory    MemoryStats    `json:"memory"`
	Checks    []HealthCheck  `json:"checks"`
}

// MemoryStats captures Go runtime memory information.
type MemoryStats struct {
	AllocMB      uint64 `json:"alloc_mb"`
	TotalAllocMB uint64 `json:"total_alloc_mb"`
	SysMB        uint64 `json:"sys_mb"`
	NumGC        uint32 `json:"num_gc"`
	Goroutines   int    `json:"goroutines"`
}

// HealthCheck is the result of a single named health probe.
type HealthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "warn", "fail"
	Detail string `json:"detail,omitempty"`
}

// providerCache holds the most-recent result of the provider reachability probe
// so the gateway does not call the API on every health request.
type providerCache struct {
	mu        sync.Mutex
	status    string
	detail    string
	checkedAt time.Time
}

// healthMonitor runs background checks and builds HealthStatus snapshots.
type healthMonitor struct {
	gw       *Gateway
	provider providerCache
}

func newHealthMonitor(gw *Gateway) *healthMonitor {
	return &healthMonitor{gw: gw}
}

// buildStatus constructs a complete HealthStatus from live gateway state.
func (hm *healthMonitor) buildStatus() HealthStatus {
	gw := hm.gw

	// --- client counts by role ---
	gw.roleMux.RLock()
	clients := map[string]int{
		"agent":   len(gw.agents),
		"channel": len(gw.channels),
		"ui":      len(gw.uis),
		"node":    len(gw.nodes),
	}
	gw.roleMux.RUnlock()

	gw.sessionMux.RLock()
	sessions := len(gw.sessions)
	gw.sessionMux.RUnlock()

	// --- runtime memory ---
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	mem := MemoryStats{
		AllocMB:      ms.Alloc / (1024 * 1024),
		TotalAllocMB: ms.TotalAlloc / (1024 * 1024),
		SysMB:        ms.Sys / (1024 * 1024),
		NumGC:        ms.NumGC,
		Goroutines:   runtime.NumGoroutine(),
	}

	// --- individual health checks ---
	checks := []HealthCheck{
		hm.checkConfig(),
		hm.checkAuditLog(),
		hm.checkProvider(),
		hm.checkMemory(mem),
		hm.checkGoroutines(mem),
	}

	// --- aggregate status ---
	overall := aggregateStatus(checks)

	uptime := time.Since(gw.startedAt).Truncate(time.Second).String()

	return HealthStatus{
		Status:    overall,
		Uptime:    uptime,
		StartedAt: gw.startedAt.UTC(),
		Clients:   clients,
		Sessions:  sessions,
		Provider:  gw.config.Provider,
		Model:     gw.config.Model,
		Memory:    mem,
		Checks:    checks,
	}
}

// aggregateStatus derives the overall status from individual check results.
// Any "fail" → "unhealthy"; any "warn" → "degraded"; all "pass" → "healthy".
func aggregateStatus(checks []HealthCheck) string {
	hasFail := false
	hasWarn := false
	for _, c := range checks {
		switch c.Status {
		case "fail":
			hasFail = true
		case "warn":
			hasWarn = true
		}
	}
	switch {
	case hasFail:
		return "unhealthy"
	case hasWarn:
		return "degraded"
	default:
		return "healthy"
	}
}

// checkConfig verifies the workspace config file is readable.
func (hm *healthMonitor) checkConfig() HealthCheck {
	// The gateway does not hold a direct path to config.yml, so we check the
	// standard location relative to the cwd (same approach used by LoadWorkspace).
	candidates := []string{
		".soulgate/config.yml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return HealthCheck{Name: "config_readable", Status: "pass"}
		}
	}
	return HealthCheck{
		Name:   "config_readable",
		Status: "warn",
		Detail: "config.yml not found in .soulgate/; workspace may not be initialized",
	}
}

// checkAuditLog verifies the audit log directory is writable.
func (hm *healthMonitor) checkAuditLog() HealthCheck {
	dir := ".soulgate"
	if _, err := os.Stat(dir); err != nil {
		return HealthCheck{
			Name:   "audit_log_writable",
			Status: "warn",
			Detail: fmt.Sprintf("audit directory not found: %v", err),
		}
	}

	// Attempt a temp-file write to confirm write permission.
	tmp, err := os.CreateTemp(dir, ".health-probe-*")
	if err != nil {
		return HealthCheck{
			Name:   "audit_log_writable",
			Status: "fail",
			Detail: fmt.Sprintf("cannot write to audit directory: %v", err),
		}
	}
	tmp.Close()           //nolint:errcheck
	os.Remove(tmp.Name()) //nolint:errcheck

	return HealthCheck{Name: "audit_log_writable", Status: "pass"}
}

// checkProvider returns the cached provider reachability result,
// refreshing the cache if it is older than 60 seconds.
func (hm *healthMonitor) checkProvider() HealthCheck {
	hm.provider.mu.Lock()
	defer hm.provider.mu.Unlock()

	// Use cached result if still fresh.
	if time.Since(hm.provider.checkedAt) < 60*time.Second && hm.provider.status != "" {
		return HealthCheck{
			Name:   "provider_reachable",
			Status: hm.provider.status,
			Detail: hm.provider.detail,
		}
	}

	// Probe: a simple HTTP HEAD to the provider base URL (if identifiable).
	status, detail := hm.probeProvider()

	hm.provider.status = status
	hm.provider.detail = detail
	hm.provider.checkedAt = time.Now()

	return HealthCheck{Name: "provider_reachable", Status: status, Detail: detail}
}

// probeProvider performs the actual network probe.
// It resolves the provider's API base URL from the configured provider name
// and issues a HEAD request with a 5-second timeout.
func (hm *healthMonitor) probeProvider() (status, detail string) {
	provider := hm.gw.config.Provider

	baseURLs := map[string]string{
		"anthropic":  "https://api.anthropic.com",
		"openai":     "https://api.openai.com",
		"groq":       "https://api.groq.com",
		"google":     "https://generativelanguage.googleapis.com",
		"mistral":    "https://api.mistral.ai",
		"cohere":     "https://api.cohere.ai",
		"deepseek":   "https://api.deepseek.com",
		"openrouter": "https://openrouter.ai",
		"together":   "https://api.together.xyz",
		"perplexity": "https://api.perplexity.ai",
		"xai":        "https://api.x.ai",
		"ollama":     "http://localhost:11434",
	}

	targetURL, known := baseURLs[provider]
	if !known {
		// Unknown or empty provider; skip the network check.
		return "warn", fmt.Sprintf("no base URL known for provider %q; skipping probe", provider)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return "warn", fmt.Sprintf("could not build probe request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "fail", fmt.Sprintf("provider %s unreachable: %v", provider, err)
	}
	resp.Body.Close() //nolint:errcheck

	// Any HTTP response means the network path is open.
	return "pass", fmt.Sprintf("provider %s responded with %d", provider, resp.StatusCode)
}

// checkMemory warns at 500 MB allocated, fails at 1 GB.
func (hm *healthMonitor) checkMemory(mem MemoryStats) HealthCheck {
	const warnMB = 500
	const failMB = 1024

	switch {
	case mem.AllocMB >= failMB:
		return HealthCheck{
			Name:   "memory_usage",
			Status: "fail",
			Detail: fmt.Sprintf("allocated %d MB exceeds 1 GB limit", mem.AllocMB),
		}
	case mem.AllocMB >= warnMB:
		return HealthCheck{
			Name:   "memory_usage",
			Status: "warn",
			Detail: fmt.Sprintf("allocated %d MB exceeds 500 MB threshold", mem.AllocMB),
		}
	default:
		return HealthCheck{
			Name:   "memory_usage",
			Status: "pass",
			Detail: fmt.Sprintf("%d MB allocated", mem.AllocMB),
		}
	}
}

// checkGoroutines warns when the goroutine count exceeds 1000.
func (hm *healthMonitor) checkGoroutines(mem MemoryStats) HealthCheck {
	const warnCount = 1000

	if mem.Goroutines >= warnCount {
		return HealthCheck{
			Name:   "goroutine_count",
			Status: "warn",
			Detail: fmt.Sprintf("%d goroutines (threshold: %d)", mem.Goroutines, warnCount),
		}
	}
	return HealthCheck{
		Name:   "goroutine_count",
		Status: "pass",
		Detail: fmt.Sprintf("%d goroutines", mem.Goroutines),
	}
}

// handleHealth replaces the old stub and returns a rich HealthStatus payload.
func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	hs := g.monitor.buildStatus()

	code := http.StatusOK
	if hs.Status == "unhealthy" {
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(hs) //nolint:errcheck
}
