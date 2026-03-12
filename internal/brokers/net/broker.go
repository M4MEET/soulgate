package net

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
	"github.com/M4MEET/soulgate/internal/brokers"
	"github.com/M4MEET/soulgate/internal/policy"
)

// Broker provides secure network access through policy enforcement
type Broker struct {
	policyEngine *policy.Engine
	auditLogger  audit.Logger
	client       *http.Client
}

// NewBroker creates a new network broker
func NewBroker(policyEngine *policy.Engine, auditLogger audit.Logger) (*Broker, error) {
	return &Broker{
		policyEngine: policyEngine,
		auditLogger:  auditLogger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Name returns the broker name
func (b *Broker) Name() string {
	return "net"
}

// Close closes the broker
func (b *Broker) Close() error {
	b.client.CloseIdleConnections()
	return nil
}

// Request makes an HTTP request
func (b *Broker) Request(ctx context.Context, brokerCtx brokers.BrokerContext, method, url string, body string, headers map[string]string) (*HTTPResponse, error) {
	// Check policy
	policyReq := policy.PolicyRequest{
		Action:   "net.request",
		Resource: url,
		PluginID: brokerCtx.PluginID,
		RunID:    brokerCtx.RunID,
	}

	result, err := b.policyEngine.Evaluate(ctx, policyReq)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, method, url, 0, audit.StatusError, err)
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	if result.Decision != policy.DecisionAllow {
		err := fmt.Errorf("access denied by policy: %s", result.Reason)
		b.logAuditEvent(ctx, brokerCtx, method, url, 0, audit.StatusDenied, err)
		return nil, err
	}

	// Create request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, method, url, 0, audit.StatusError, err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := b.client.Do(req)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, method, url, 0, audit.StatusError, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		b.logAuditEvent(ctx, brokerCtx, method, url, resp.StatusCode, audit.StatusError, err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	httpResponse := &HTTPResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    make(map[string]string),
		Body:       string(responseBody),
	}

	// Copy response headers
	for key := range resp.Header {
		httpResponse.Headers[key] = resp.Header.Get(key)
	}

	// Log success
	b.logAuditEvent(ctx, brokerCtx, method, url, resp.StatusCode, audit.StatusSuccess, nil)

	return httpResponse, nil
}

// logAuditEvent logs an audit event
func (b *Broker) logAuditEvent(ctx context.Context, brokerCtx brokers.BrokerContext, method, url string, statusCode int, status audit.EventStatus, err error) {
	event := audit.NewEvent(audit.EventNetRequest, audit.CategoryBroker).
		WithSessionID(brokerCtx.SessionID).
		WithRunID(brokerCtx.RunID).
		WithPlugin(brokerCtx.PluginID).
		WithResource(url).
		WithStatus(status).
		WithMetadata("method", method).
		WithMetadata("status_code", statusCode)

	if err != nil {
		event.WithError(err)
	}

	// Best effort logging
	b.auditLogger.Log(ctx, event)
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int               `json:"status_code"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}
