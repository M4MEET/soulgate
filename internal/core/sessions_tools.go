package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// This file implements the OpenClaw-parity session/channel tools:
//
//   sessions_list      — list gateway chat threads and background agents
//   sessions_history   — read the transcript of a thread or agent
//   message            — send a message out through a connected channel
//   heartbeat_respond  — acknowledge or flag a heartbeat check
//
// Threads are produced by the gateway (state/threads.json); agents live in
// the orchestrator. Both are read-only here — introspection never mutates.

// gatewayThread mirrors the subset of the gateway's thread persistence format
// (.soulgate/state/threads.json) needed for introspection.
type gatewayThread struct {
	ID        string                 `json:"id"`
	CreatedAt string                 `json:"createdAt"`
	Archived  bool                   `json:"archived"`
	Messages  []gatewayThreadMessage `json:"messages"`
}

type gatewayThreadMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func (o *Orchestrator) loadGatewayThreads() ([]gatewayThread, error) {
	path := filepath.Join(o.workspace.ConfigDir, "state", "threads.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read threads state: %w", err)
	}
	var threads []gatewayThread
	if err := json.Unmarshal(data, &threads); err != nil {
		return nil, fmt.Errorf("parse threads state: %w", err)
	}
	return threads, nil
}

// handleSessionsList lists gateway chat threads and background agents.
func (o *Orchestrator) handleSessionsList(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Kind            string `json:"kind"` // "", "thread", "agent"
		IncludeArchived bool   `json:"include_archived"`
	}
	if len(input) > 0 {
		_ = json.Unmarshal(input, &params)
	}

	type sessionSummary struct {
		ID           string `json:"id"`
		Kind         string `json:"kind"`
		Name         string `json:"name,omitempty"`
		Status       string `json:"status,omitempty"`
		CreatedAt    string `json:"created_at,omitempty"`
		MessageCount int    `json:"message_count"`
		Archived     bool   `json:"archived,omitempty"`
	}

	var out []sessionSummary

	if params.Kind == "" || params.Kind == "thread" {
		threads, err := o.loadGatewayThreads()
		if err != nil {
			return "", err
		}
		for _, t := range threads {
			if t.Archived && !params.IncludeArchived {
				continue
			}
			out = append(out, sessionSummary{
				ID:           t.ID,
				Kind:         "thread",
				CreatedAt:    t.CreatedAt,
				MessageCount: len(t.Messages),
				Archived:     t.Archived,
			})
		}
	}

	if (params.Kind == "" || params.Kind == "agent") && o.agentManager != nil {
		for _, a := range o.agentManager.List() {
			out = append(out, sessionSummary{
				ID:           a.ID,
				Kind:         "agent",
				Name:         a.Name,
				Status:       string(a.Status),
				CreatedAt:    a.CreatedAt.Format(time.RFC3339),
				MessageCount: len(a.GetLog()),
			})
		}
	}

	if len(out) == 0 {
		return "No sessions found.", nil
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal sessions: %w", err)
	}
	return string(data), nil
}

// handleSessionsHistory returns the transcript of a gateway thread or the
// activity log of a background agent.
func (o *Orchestrator) handleSessionsHistory(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ID    string `json:"id"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.ID == "" {
		return "", fmt.Errorf("id is required (thread id or agent id — see sessions_list)")
	}
	if params.Limit <= 0 || params.Limit > 200 {
		params.Limit = 50
	}

	// Gateway thread transcript.
	if strings.HasPrefix(params.ID, "thread_") {
		threads, err := o.loadGatewayThreads()
		if err != nil {
			return "", err
		}
		for _, t := range threads {
			if t.ID != params.ID {
				continue
			}
			msgs := t.Messages
			if len(msgs) > params.Limit {
				msgs = msgs[len(msgs)-params.Limit:]
			}
			data, err := json.MarshalIndent(msgs, "", "  ")
			if err != nil {
				return "", fmt.Errorf("marshal transcript: %w", err)
			}
			return string(data), nil
		}
		return "", fmt.Errorf("thread %q not found", params.ID)
	}

	// Background agent log.
	agent := o.findSessionAgent(params.ID)
	if agent == nil {
		return "", fmt.Errorf("session %q not found (not a thread id or agent id)", params.ID)
	}
	entries := agent.GetLogTail(params.Limit)
	if len(entries) == 0 {
		return fmt.Sprintf("Agent %q (%s) has no activity yet.", agent.Name, agent.ID), nil
	}
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "[%s] %s: %s\n", e.Time.Format(time.RFC3339), e.Kind, e.Message)
	}
	return sb.String(), nil
}

// findSessionAgent resolves an agent by ID or (case-insensitive) name.
func (o *Orchestrator) findSessionAgent(idOrName string) *BackgroundAgent {
	if o.agentManager == nil {
		return nil
	}
	for _, a := range o.agentManager.List() {
		if a.ID == idOrName || strings.EqualFold(a.Name, idOrName) {
			return a
		}
	}
	return nil
}

// SetChannelMessenger registers the gateway hook used by the `message` tool
// to deliver text to a connected channel (telegram, slack, ...). Without a
// hook (plain CLI runs) the tool reports that a gateway is required.
func (o *Orchestrator) SetChannelMessenger(fn func(channel, conversationID, text string) error) {
	o.channelMessenger = fn
}

// handleMessage sends a message through a connected gateway channel.
// Policy action "messaging.send" is enforced by the permission registry
// before dispatch reaches this handler.
func (o *Orchestrator) handleMessage(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Channel        string `json:"channel"`
		ConversationID string `json:"conversation_id"`
		Text           string `json:"text"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if params.Text == "" {
		return "", fmt.Errorf("text is required")
	}
	if params.Channel == "" {
		return "", fmt.Errorf("channel is required (e.g. telegram, slack)")
	}
	if o.channelMessenger == nil {
		return "", fmt.Errorf("message tool requires a running gateway (start with: soulgate gateway start)")
	}

	if err := o.channelMessenger(params.Channel, params.ConversationID, params.Text); err != nil {
		return "", fmt.Errorf("send via %s failed: %w", params.Channel, err)
	}
	return fmt.Sprintf("Message sent via %s.", params.Channel), nil
}

// handleHeartbeatRespond records the model's explicit response to a heartbeat
// check. status "ok" keeps the heartbeat silent; "attention" forwards the
// message to the configured notification callback.
func (o *Orchestrator) handleHeartbeatRespond(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	switch params.Status {
	case "ok", "attention":
	default:
		return "", fmt.Errorf("status must be 'ok' or 'attention'")
	}
	if params.Status == "attention" && params.Message == "" {
		return "", fmt.Errorf("message is required when status is 'attention'")
	}

	if o.heartbeat == nil {
		return "", fmt.Errorf("heartbeat subsystem not initialized")
	}
	o.heartbeat.RecordResponse(params.Status, params.Message)

	if params.Status == "ok" {
		return "Heartbeat acknowledged (all clear).", nil
	}
	return "Heartbeat attention recorded and forwarded.", nil
}
