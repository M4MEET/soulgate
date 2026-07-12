package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/M4MEET/soulgate/internal/config"
)

// newSessionsTestOrchestrator builds a minimal orchestrator with a temp
// workspace — enough for the introspection tools, no model provider needed.
func newSessionsTestOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, ".soulgate")
	if err := os.MkdirAll(filepath.Join(configDir, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	return &Orchestrator{
		workspace: &config.Workspace{
			Root:      root,
			ConfigDir: configDir,
			Config:    &config.Config{},
		},
		agentManager: NewAgentManager(""),
	}
}

func writeThreadsState(t *testing.T, o *Orchestrator, payload string) {
	t.Helper()
	path := filepath.Join(o.workspace.ConfigDir, "state", "threads.json")
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSessionsListEmpty(t *testing.T) {
	o := newSessionsTestOrchestrator(t)
	out, err := o.handleSessionsList(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "No sessions found." {
		t.Errorf("got %q", out)
	}
}

func TestSessionsListThreads(t *testing.T) {
	o := newSessionsTestOrchestrator(t)
	writeThreadsState(t, o, `[
		{"id":"thread_1","createdAt":"2026-07-12T10:00:00Z","archived":false,
		 "messages":[{"role":"user","content":"hi","timestamp":"2026-07-12T10:00:01Z"}]},
		{"id":"thread_2","createdAt":"2026-07-12T11:00:00Z","archived":true,"messages":[]}
	]`)

	out, err := o.handleSessionsList(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "thread_1") {
		t.Errorf("active thread missing from output: %s", out)
	}
	if strings.Contains(out, "thread_2") {
		t.Errorf("archived thread should be hidden by default: %s", out)
	}

	out, err = o.handleSessionsList(context.Background(), json.RawMessage(`{"include_archived":true}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "thread_2") {
		t.Errorf("archived thread missing with include_archived: %s", out)
	}
}

func TestSessionsHistoryThread(t *testing.T) {
	o := newSessionsTestOrchestrator(t)
	writeThreadsState(t, o, `[
		{"id":"thread_1","createdAt":"2026-07-12T10:00:00Z","archived":false,
		 "messages":[
			{"role":"user","content":"first","timestamp":"t1"},
			{"role":"assistant","content":"second","timestamp":"t2"}
		]}
	]`)

	out, err := o.handleSessionsHistory(context.Background(), json.RawMessage(`{"id":"thread_1"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Errorf("transcript incomplete: %s", out)
	}

	if _, err := o.handleSessionsHistory(context.Background(), json.RawMessage(`{"id":"thread_missing"}`)); err == nil {
		t.Error("expected error for unknown thread")
	}
	if _, err := o.handleSessionsHistory(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Error("expected error for missing id")
	}
}

func TestMessageToolRequiresGateway(t *testing.T) {
	o := newSessionsTestOrchestrator(t)

	_, err := o.handleMessage(context.Background(), json.RawMessage(`{"channel":"telegram","text":"hi"}`))
	if err == nil || !strings.Contains(err.Error(), "gateway") {
		t.Errorf("expected gateway-required error, got %v", err)
	}

	var got [3]string
	o.SetChannelMessenger(func(channel, conversationID, text string) error {
		got = [3]string{channel, conversationID, text}
		return nil
	})
	out, err := o.handleMessage(context.Background(), json.RawMessage(`{"channel":"telegram","conversation_id":"42","text":"hi"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != [3]string{"telegram", "42", "hi"} {
		t.Errorf("messenger got %v", got)
	}
	if !strings.Contains(out, "telegram") {
		t.Errorf("confirmation should name the channel: %s", out)
	}

	if _, err := o.handleMessage(context.Background(), json.RawMessage(`{"channel":"telegram"}`)); err == nil {
		t.Error("expected error for missing text")
	}
}

func TestHeartbeatRespond(t *testing.T) {
	o := newSessionsTestOrchestrator(t)
	o.heartbeat = NewHeartbeat(o, config.HeartbeatConfig{})

	var notified string
	o.heartbeat.SetCallback(func(msg string) { notified = msg })

	out, err := o.handleHeartbeatRespond(context.Background(), json.RawMessage(`{"status":"ok"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notified != "" {
		t.Errorf("ok status must not notify, got %q", notified)
	}
	if !strings.Contains(out, "all clear") {
		t.Errorf("unexpected output: %s", out)
	}

	if _, err := o.handleHeartbeatRespond(context.Background(), json.RawMessage(`{"status":"attention"}`)); err == nil {
		t.Error("attention without message must error")
	}

	_, err = o.handleHeartbeatRespond(context.Background(), json.RawMessage(`{"status":"attention","message":"disk almost full"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notified != "disk almost full" {
		t.Errorf("callback got %q", notified)
	}

	if _, err := o.handleHeartbeatRespond(context.Background(), json.RawMessage(`{"status":"maybe"}`)); err == nil {
		t.Error("invalid status must error")
	}
}
