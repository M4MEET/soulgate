package gateway

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
)

// MetricsCollector tracks gateway-level counters and gauges used to populate
// the /metrics endpoint. It is safe for concurrent use.
//
// All fields use atomic operations or are protected by mu so that HTTP handlers
// and WebSocket goroutines can update them without coordination.
type MetricsCollector struct {
	// Request counters per endpoint path, e.g. "/api/chat" -> 42.
	// Protected by endpointMu so that new paths can be lazily registered.
	endpointCounts map[string]*int64
	endpointMu     sync.RWMutex

	// Token counter: cumulative tokens across all model calls.
	tokensTotal int64

	// Cost accumulator in micro-USD (1 unit == $0.000001) to avoid float races.
	costMicroUSD int64
}

// newMetricsCollector creates an initialised MetricsCollector.
func newMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		endpointCounts: make(map[string]*int64),
	}
}

// IncrEndpoint atomically increments the request counter for path.
func (m *MetricsCollector) IncrEndpoint(path string) {
	m.endpointMu.RLock()
	ctr, ok := m.endpointCounts[path]
	m.endpointMu.RUnlock()

	if ok {
		atomic.AddInt64(ctr, 1)
		return
	}

	// First time we see this path — promote to write lock and initialise.
	m.endpointMu.Lock()
	// Re-check after acquiring write lock (another goroutine may have beaten us).
	if ctr, ok = m.endpointCounts[path]; !ok {
		var zero int64
		m.endpointCounts[path] = &zero
		ctr = m.endpointCounts[path]
	}
	m.endpointMu.Unlock()

	atomic.AddInt64(ctr, 1)
}

// AddTokens adds n to the cumulative token counter.
func (m *MetricsCollector) AddTokens(n int64) {
	atomic.AddInt64(&m.tokensTotal, n)
}

// AddCostUSD adds a cost value (in USD) to the cumulative cost gauge.
// The value is stored as integer micro-USD to avoid float atomicity issues.
func (m *MetricsCollector) AddCostUSD(usd float64) {
	atomic.AddInt64(&m.costMicroUSD, int64(usd*1_000_000))
}

// snapshot returns a consistent point-in-time copy of all metric values.
func (m *MetricsCollector) snapshot(gw *Gateway) metricsSnapshot {
	// Endpoint counts
	m.endpointMu.RLock()
	endpoints := make(map[string]int64, len(m.endpointCounts))
	for path, ctr := range m.endpointCounts {
		endpoints[path] = atomic.LoadInt64(ctr)
	}
	m.endpointMu.RUnlock()

	// Client counts by role
	gw.roleMux.RLock()
	agentCount := int64(len(gw.agents))
	channelCount := int64(len(gw.channels))
	gw.roleMux.RUnlock()

	// Runtime memory
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return metricsSnapshot{
		endpoints:      endpoints,
		tokensTotal:    atomic.LoadInt64(&m.tokensTotal),
		costMicroUSD:   atomic.LoadInt64(&m.costMicroUSD),
		agentClients:   agentCount,
		channelClients: channelCount,
		goroutines:     int64(runtime.NumGoroutine()),
		memoryBytes:    int64(ms.Alloc),
	}
}

// metricsSnapshot is an immutable point-in-time copy of all metric values,
// used to render a single consistent /metrics response without holding locks.
type metricsSnapshot struct {
	endpoints      map[string]int64
	tokensTotal    int64
	costMicroUSD   int64
	agentClients   int64
	channelClients int64
	goroutines     int64
	memoryBytes    int64
}

// handleMetrics serves the /metrics endpoint in Prometheus text exposition
// format (version 0.0.4). No external dependency is required — the format
// is straightforward plain text.
//
// Reference: https://prometheus.io/docs/instrumenting/exposition_formats/
func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if g.metrics == nil {
		http.Error(w, "metrics not initialised", http.StatusInternalServerError)
		return
	}

	snap := g.metrics.snapshot(g)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	fmt.Fprint(w, renderMetrics(snap))
}

// renderMetrics formats a metricsSnapshot into the Prometheus text format.
// Keeping the rendering logic separate makes it trivially testable.
func renderMetrics(snap metricsSnapshot) string {
	var b []byte

	// --- soulgate_requests_total ---
	b = append(b, "# HELP soulgate_requests_total Total API requests\n"...)
	b = append(b, "# TYPE soulgate_requests_total counter\n"...)
	// Emit one labelled line per tracked endpoint.  If no requests have been
	// recorded yet, emit the sentinel endpoints with zero to keep dashboards happy.
	if len(snap.endpoints) == 0 {
		b = appendMetricLine(b, "soulgate_requests_total", `endpoint="/api/chat"`, 0)
		b = appendMetricLine(b, "soulgate_requests_total", `endpoint="/api/health"`, 0)
	} else {
		for endpoint, count := range snap.endpoints {
			label := fmt.Sprintf("endpoint=%q", endpoint)
			b = appendMetricLine(b, "soulgate_requests_total", label, count)
		}
	}
	b = append(b, '\n')

	// --- soulgate_tokens_total ---
	b = append(b, "# HELP soulgate_tokens_total Total tokens used\n"...)
	b = append(b, "# TYPE soulgate_tokens_total counter\n"...)
	b = appendMetricLine(b, "soulgate_tokens_total", "", snap.tokensTotal)
	b = append(b, '\n')

	// --- soulgate_cost_total_usd ---
	costUSD := float64(snap.costMicroUSD) / 1_000_000
	b = append(b, "# HELP soulgate_cost_total_usd Total cost in USD\n"...)
	b = append(b, "# TYPE soulgate_cost_total_usd gauge\n"...)
	b = append(b, fmt.Sprintf("soulgate_cost_total_usd %.6f\n", costUSD)...)
	b = append(b, '\n')

	// --- soulgate_clients_connected ---
	b = append(b, "# HELP soulgate_clients_connected Connected WebSocket clients\n"...)
	b = append(b, "# TYPE soulgate_clients_connected gauge\n"...)
	b = appendMetricLine(b, "soulgate_clients_connected", `role="agent"`, snap.agentClients)
	b = appendMetricLine(b, "soulgate_clients_connected", `role="channel"`, snap.channelClients)
	b = append(b, '\n')

	// --- soulgate_goroutines ---
	b = append(b, "# HELP soulgate_goroutines Number of goroutines\n"...)
	b = append(b, "# TYPE soulgate_goroutines gauge\n"...)
	b = appendMetricLine(b, "soulgate_goroutines", "", snap.goroutines)
	b = append(b, '\n')

	// --- soulgate_memory_bytes ---
	b = append(b, "# HELP soulgate_memory_bytes Memory allocated in bytes\n"...)
	b = append(b, "# TYPE soulgate_memory_bytes gauge\n"...)
	b = appendMetricLine(b, "soulgate_memory_bytes", "", snap.memoryBytes)

	return string(b)
}

// appendMetricLine appends a single Prometheus metric line to buf.
// When label is empty the metric is written without braces: "name value\n".
// When label is non-empty it is wrapped in braces:          "name{label} value\n".
func appendMetricLine(buf []byte, name, label string, value int64) []byte {
	if label == "" {
		return append(buf, fmt.Sprintf("%s %d\n", name, value)...)
	}
	return append(buf, fmt.Sprintf("%s{%s} %d\n", name, label, value)...)
}

// metricsMiddleware wraps an http.Handler and counts each request against the
// provided MetricsCollector, keyed by the request's URL path.
func metricsMiddleware(next http.Handler, mc *MetricsCollector) http.Handler {
	if mc == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mc.IncrEndpoint(r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
