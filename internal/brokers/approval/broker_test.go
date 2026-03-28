package approval

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApprovalBroker_Approve(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir).WithTimeout(10 * time.Second)
	defer b.Close()

	var requestID string

	// Submit a request in the background, approve it asynchronously.
	done := make(chan bool, 1)
	go func() {
		approved, err := b.RequestApproval(context.Background(), "files.write", "./secret.yml", "policy requires approval", "agent-1")
		require.NoError(t, err)
		done <- approved
	}()

	// Give the goroutine time to enqueue.
	time.Sleep(50 * time.Millisecond)

	pending := b.ListPending()
	require.Len(t, pending, 1)
	requestID = pending[0].ID

	err := b.Approve(requestID, "admin")
	require.NoError(t, err)

	select {
	case approved := <-done:
		assert.True(t, approved, "request should be approved")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for approval")
	}
}

func TestApprovalBroker_Deny(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir).WithTimeout(10 * time.Second)
	defer b.Close()

	done := make(chan bool, 1)
	go func() {
		approved, err := b.RequestApproval(context.Background(), "exec.command", "rm -rf /", "dangerous command", "agent-2")
		require.NoError(t, err)
		done <- approved
	}()

	time.Sleep(50 * time.Millisecond)

	pending := b.ListPending()
	require.Len(t, pending, 1)

	err := b.Deny(pending[0].ID, "admin")
	require.NoError(t, err)

	select {
	case approved := <-done:
		assert.False(t, approved, "request should be denied")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for deny")
	}
}

func TestApprovalBroker_Expiry(t *testing.T) {
	dir := t.TempDir()
	// Very short timeout so expiry fires quickly.
	b := NewBroker(dir).WithTimeout(200 * time.Millisecond)
	defer b.Close()

	done := make(chan bool, 1)
	go func() {
		approved, err := b.RequestApproval(context.Background(), "files.delete", "./data.db", "flagged by policy", "agent-3")
		require.NoError(t, err)
		done <- approved
	}()

	// Wait longer than the broker timeout so the sweeper fires.
	select {
	case approved := <-done:
		assert.False(t, approved, "expired request should be denied")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for expiry")
	}
}

func TestApprovalBroker_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir).WithTimeout(30 * time.Second)
	defer b.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := b.RequestApproval(ctx, "net.request", "https://evil.example.com", "outbound request", "agent-4")
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err, "should return context error")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for context cancel")
	}
}

func TestApprovalBroker_DuplicateDecide(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir).WithTimeout(10 * time.Second)
	defer b.Close()

	go func() {
		b.RequestApproval(context.Background(), "files.write", "./x.txt", "test", "agent-5") //nolint:errcheck
	}()

	time.Sleep(50 * time.Millisecond)
	pending := b.ListPending()
	require.Len(t, pending, 1)
	id := pending[0].ID

	require.NoError(t, b.Approve(id, "admin"))
	// Second decision should fail.
	err := b.Approve(id, "admin")
	assert.Error(t, err, "duplicate approve should return error")
}

func TestApprovalBroker_NotFound(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir)
	defer b.Close()

	err := b.Approve("nonexistent-id", "admin")
	assert.Error(t, err, "approving non-existent request should fail")

	err = b.Deny("nonexistent-id", "admin")
	assert.Error(t, err, "denying non-existent request should fail")
}

func TestApprovalBroker_Persistence(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir).WithTimeout(10 * time.Second)
	defer b.Close()

	// Submit without deciding — request will be persisted as pending.
	go func() {
		b.RequestApproval(context.Background(), "files.write", "./persist.txt", "test persistence", "agent-6") //nolint:errcheck
	}()

	time.Sleep(100 * time.Millisecond)
	require.Len(t, b.ListPending(), 1)

	// Verify the file was written to security/approvals.json.
	_, err := os.Stat(dir + "/security/approvals.json")
	require.NoError(t, err, "security/approvals.json should exist")
}

func TestApprovalBroker_HandlerNotification(t *testing.T) {
	dir := t.TempDir()
	b := NewBroker(dir).WithTimeout(10 * time.Second)
	defer b.Close()

	notified := make(chan *ApprovalRequest, 1)
	b.AddHandler(&testHandler{ch: notified})

	go func() {
		b.RequestApproval(context.Background(), "files.write", "./notify.txt", "handler test", "agent-7") //nolint:errcheck
	}()

	select {
	case req := <-notified:
		assert.Equal(t, "files.write", req.Action)
		assert.Equal(t, "./notify.txt", req.Resource)
	case <-time.After(3 * time.Second):
		t.Fatal("handler was not notified")
	}
}

// testHandler is a simple ApprovalHandler for tests.
type testHandler struct {
	ch chan *ApprovalRequest
}

func (h *testHandler) OnApprovalRequired(req *ApprovalRequest) {
	h.ch <- req
}
