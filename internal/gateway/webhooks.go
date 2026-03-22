package gateway

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// WebhookFormat describes how to extract a message from an inbound payload.
type WebhookFormat string

const (
	WebhookFormatJSON   WebhookFormat = "json"
	WebhookFormatText   WebhookFormat = "text"
	WebhookFormatGitHub WebhookFormat = "github"
	WebhookFormatGitLab WebhookFormat = "gitlab"
)

// WebhookConfig describes a single inbound webhook endpoint.
type WebhookConfig struct {
	// Name is used as the URL path segment: POST /webhook/{name}
	Name string `json:"name"`
	// Secret is an optional HMAC-SHA256 shared secret (same convention as GitHub webhooks).
	// When non-empty, incoming requests must carry a valid X-Hub-Signature-256 header.
	Secret string `json:"secret"`
	// Format controls how the message body is decoded. Defaults to "json".
	Format WebhookFormat `json:"format"`
	// MessageKey is a dot-separated JSON path used when Format=="json" to locate the
	// text of the message to forward to the chat handler. Defaults to "message".
	MessageKey string `json:"message_key"`
	// Enabled allows individual webhooks to be toggled without removing them.
	Enabled bool `json:"enabled"`
}

// WebhookStore holds the persisted list of webhook configs and the path to the
// backing file. All exported mutating methods are safe for concurrent use.
type WebhookStore struct {
	mu       sync.RWMutex
	webhooks map[string]*WebhookConfig
	path     string
}

// NewWebhookStore creates a WebhookStore backed by the given JSON file path and
// loads any existing webhook configs from disk. A missing file is not an error.
func NewWebhookStore(path string) (*WebhookStore, error) {
	s := &WebhookStore{
		webhooks: make(map[string]*WebhookConfig),
		path:     path,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func newWebhookStore(path string) *WebhookStore {
	return &WebhookStore{
		webhooks: make(map[string]*WebhookConfig),
		path:     path,
	}
}

// EmptyWebhookStore returns a brand-new, empty WebhookStore that will persist
// to the given path on the first write. This is used when the backing file
// cannot be parsed, to allow callers to still add new webhooks.
func EmptyWebhookStore(path string) *WebhookStore {
	return &WebhookStore{
		webhooks: make(map[string]*WebhookConfig),
		path:     path,
	}
}

// load reads webhooks from the JSON file. Missing file is not an error.
func (s *WebhookStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read webhooks file: %w", err)
	}

	var list []*WebhookConfig
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse webhooks file: %w", err)
	}

	s.webhooks = make(map[string]*WebhookConfig, len(list))
	for _, wh := range list {
		s.webhooks[wh.Name] = wh
	}
	return nil
}

// save writes the current webhook list to disk atomically (write-then-rename).
func (s *WebhookStore) save() error {
	list := make([]*WebhookConfig, 0, len(s.webhooks))
	for _, wh := range s.webhooks {
		list = append(list, wh)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal webhooks: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write webhooks tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename webhooks file: %w", err)
	}
	return nil
}

// List returns a copy of all configured webhooks.
func (s *WebhookStore) List() []*WebhookConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*WebhookConfig, 0, len(s.webhooks))
	for _, wh := range s.webhooks {
		cp := *wh
		out = append(out, &cp)
	}
	return out
}

// Get returns the webhook with the given name, or nil if not found.
func (s *WebhookStore) Get(name string) *WebhookConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wh, ok := s.webhooks[name]
	if !ok {
		return nil
	}
	cp := *wh
	return &cp
}

// Add persists a new webhook config. Returns an error if the name is already taken.
func (s *WebhookStore) Add(wh *WebhookConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.webhooks[wh.Name]; exists {
		return fmt.Errorf("webhook %q already exists", wh.Name)
	}

	// Apply defaults.
	if wh.Format == "" {
		wh.Format = WebhookFormatJSON
	}
	if wh.MessageKey == "" {
		wh.MessageKey = "message"
	}

	s.webhooks[wh.Name] = wh
	return s.save()
}

// Remove deletes the webhook with the given name.
func (s *WebhookStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.webhooks[name]; !exists {
		return fmt.Errorf("webhook %q not found", name)
	}
	delete(s.webhooks, name)
	return s.save()
}

// handleWebhook is the HTTP handler for POST /webhook/{name}.
// It extracts the message from the payload, calls the configured ChatHandler,
// and returns the AI response as JSON.
func (g *Gateway) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeWebhookJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Extract webhook name from URL: /webhook/<name>
	name := strings.TrimPrefix(r.URL.Path, "/webhook/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		writeWebhookJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook name required"})
		return
	}

	if g.webhookStore == nil {
		writeWebhookJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "webhook system not initialised"})
		return
	}

	wh := g.webhookStore.Get(name)
	if wh == nil {
		writeWebhookJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("webhook %q not found", name)})
		return
	}

	if !wh.Enabled {
		writeWebhookJSON(w, http.StatusForbidden, map[string]string{"error": "webhook is disabled"})
		return
	}

	// Read body once — we may need it for both HMAC verification and parsing.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		writeWebhookJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
		return
	}

	// Verify HMAC signature when a secret is configured.
	if wh.Secret != "" {
		if err := verifyWebhookSignature(r, body, wh.Secret); err != nil {
			writeWebhookJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
	}

	// Extract message text based on format.
	message, err := extractWebhookMessage(body, r, wh)
	if err != nil {
		writeWebhookJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": fmt.Sprintf("could not extract message: %v", err)})
		return
	}

	if message == "" {
		writeWebhookJSON(w, http.StatusBadRequest, map[string]string{"error": "extracted message is empty"})
		return
	}

	// Forward to ChatHandler if available.
	if g.config.OnChat == nil {
		writeWebhookJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "chat handler not configured"})
		return
	}

	fmt.Printf("[webhook] %q -> %q\n", name, truncate(message, 80))

	// Fire message.received notification before calling the handler.
	g.Notify("message.received", map[string]interface{}{
		"webhook": name,
		"message": message,
	})

	response, err := g.config.OnChat(r.Context(), message)
	if err != nil {
		g.Notify("error", map[string]interface{}{
			"webhook": name,
			"error":   err.Error(),
		})
		writeWebhookJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("AI error: %v", err)})
		return
	}

	g.Notify("agent.completed", map[string]interface{}{
		"webhook":  name,
		"message":  message,
		"response": response,
	})

	writeWebhookJSON(w, http.StatusOK, map[string]string{"response": response})
}

// verifyWebhookSignature checks the X-Hub-Signature-256 header using HMAC-SHA256.
// This mirrors the GitHub webhook signature scheme.
func verifyWebhookSignature(r *http.Request, body []byte, secret string) error {
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig == "" {
		return fmt.Errorf("missing X-Hub-Signature-256 header")
	}

	// Header format: "sha256=<hex>"
	if !strings.HasPrefix(sig, "sha256=") {
		return fmt.Errorf("invalid signature format")
	}
	sigHex := strings.TrimPrefix(sig, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sigHex), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// extractWebhookMessage converts a raw HTTP payload into a human-readable message
// string based on the configured format.
func extractWebhookMessage(body []byte, r *http.Request, wh *WebhookConfig) (string, error) {
	switch wh.Format {
	case WebhookFormatText:
		return strings.TrimSpace(string(body)), nil

	case WebhookFormatGitHub:
		return extractGitHubMessage(body, r)

	case WebhookFormatGitLab:
		return extractGitLabMessage(body)

	default: // WebhookFormatJSON and anything else
		return extractJSONMessage(body, wh.MessageKey)
	}
}

// extractJSONMessage resolves a dot-separated key path inside an arbitrary JSON object.
func extractJSONMessage(body []byte, keyPath string) (string, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	parts := strings.Split(keyPath, ".")
	var cur interface{} = obj
	for _, part := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("key %q not found in payload", keyPath)
		}
		cur, ok = m[part]
		if !ok {
			return "", fmt.Errorf("key %q not found in payload", keyPath)
		}
	}

	if s, ok := cur.(string); ok {
		return s, nil
	}

	// If the value is not a string, marshal it back for transparency.
	raw, _ := json.Marshal(cur)
	return string(raw), nil
}

// extractGitHubMessage produces a readable summary from common GitHub webhook events.
func extractGitHubMessage(body []byte, r *http.Request) (string, error) {
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		// Fallback: try to treat as generic JSON with a "message" key.
		return extractJSONMessage(body, "message")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("invalid GitHub payload: %w", err)
	}

	repo := nestedString(payload, "repository", "full_name")

	switch event {
	case "push":
		ref := stringVal(payload, "ref")
		branch := strings.TrimPrefix(ref, "refs/heads/")
		pusher := nestedString(payload, "pusher", "name")
		commitCount := 0
		if commits, ok := payload["commits"].([]interface{}); ok {
			commitCount = len(commits)
		}
		headMsg := ""
		if commits, ok := payload["commits"].([]interface{}); ok && len(commits) > 0 {
			if c, ok := commits[len(commits)-1].(map[string]interface{}); ok {
				headMsg = stringVal(c, "message")
			}
		}
		return fmt.Sprintf("GitHub push to %s/%s by %s: %d commit(s). Latest: %s",
			repo, branch, pusher, commitCount, headMsg), nil

	case "pull_request":
		action := stringVal(payload, "action")
		number := 0
		if pr, ok := payload["pull_request"].(map[string]interface{}); ok {
			if n, ok := pr["number"].(float64); ok {
				number = int(n)
			}
		}
		title := ""
		if pr, ok := payload["pull_request"].(map[string]interface{}); ok {
			title = stringVal(pr, "title")
		}
		user := nestedString(payload, "pull_request", "user", "login")
		return fmt.Sprintf("GitHub PR #%d %s by %s on %s: %s", number, action, user, repo, title), nil

	case "issues":
		action := stringVal(payload, "action")
		title := nestedString(payload, "issue", "title")
		number := 0
		if issue, ok := payload["issue"].(map[string]interface{}); ok {
			if n, ok := issue["number"].(float64); ok {
				number = int(n)
			}
		}
		user := nestedString(payload, "issue", "user", "login")
		return fmt.Sprintf("GitHub issue #%d %s by %s on %s: %s", number, action, user, repo, title), nil

	case "issue_comment":
		body := nestedString(payload, "comment", "body")
		issueNum := 0
		if issue, ok := payload["issue"].(map[string]interface{}); ok {
			if n, ok := issue["number"].(float64); ok {
				issueNum = int(n)
			}
		}
		user := nestedString(payload, "comment", "user", "login")
		return fmt.Sprintf("GitHub comment on issue #%d by %s on %s: %s", issueNum, user, repo, body), nil

	case "create":
		refType := stringVal(payload, "ref_type")
		ref := stringVal(payload, "ref")
		sender := nestedString(payload, "sender", "login")
		return fmt.Sprintf("GitHub %s %q created by %s on %s", refType, ref, sender, repo), nil

	case "release":
		action := stringVal(payload, "action")
		tag := nestedString(payload, "release", "tag_name")
		name := nestedString(payload, "release", "name")
		return fmt.Sprintf("GitHub release %s: %s (%s) on %s", action, name, tag, repo), nil

	default:
		return fmt.Sprintf("GitHub %s event on %s", event, repo), nil
	}
}

// extractGitLabMessage produces a readable summary from common GitLab webhook events.
func extractGitLabMessage(body []byte) (string, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("invalid GitLab payload: %w", err)
	}

	kind := stringVal(payload, "object_kind")
	repo := nestedString(payload, "project", "path_with_namespace")

	switch kind {
	case "push":
		ref := stringVal(payload, "ref")
		branch := strings.TrimPrefix(ref, "refs/heads/")
		user := stringVal(payload, "user_name")
		commitCount := 0
		if commits, ok := payload["commits"].([]interface{}); ok {
			commitCount = len(commits)
		}
		headMsg := ""
		if commits, ok := payload["commits"].([]interface{}); ok && len(commits) > 0 {
			if c, ok := commits[len(commits)-1].(map[string]interface{}); ok {
				headMsg = stringVal(c, "message")
			}
		}
		return fmt.Sprintf("GitLab push to %s/%s by %s: %d commit(s). Latest: %s",
			repo, branch, user, commitCount, headMsg), nil

	case "merge_request":
		action := nestedString(payload, "object_attributes", "action")
		title := nestedString(payload, "object_attributes", "title")
		iid := 0
		if oa, ok := payload["object_attributes"].(map[string]interface{}); ok {
			if n, ok := oa["iid"].(float64); ok {
				iid = int(n)
			}
		}
		user := nestedString(payload, "user", "name")
		return fmt.Sprintf("GitLab MR !%d %s by %s on %s: %s", iid, action, user, repo, title), nil

	case "issue":
		action := nestedString(payload, "object_attributes", "action")
		title := nestedString(payload, "object_attributes", "title")
		iid := 0
		if oa, ok := payload["object_attributes"].(map[string]interface{}); ok {
			if n, ok := oa["iid"].(float64); ok {
				iid = int(n)
			}
		}
		user := nestedString(payload, "user", "name")
		return fmt.Sprintf("GitLab issue #%d %s by %s on %s: %s", iid, action, user, repo, title), nil

	case "note":
		noteType := nestedString(payload, "object_attributes", "noteable_type")
		body := nestedString(payload, "object_attributes", "note")
		user := nestedString(payload, "user", "name")
		return fmt.Sprintf("GitLab %s comment by %s on %s: %s", noteType, user, repo, body), nil

	case "tag_push":
		tag := strings.TrimPrefix(stringVal(payload, "ref"), "refs/tags/")
		user := stringVal(payload, "user_name")
		return fmt.Sprintf("GitLab tag %q pushed by %s on %s", tag, user, repo), nil

	case "pipeline":
		status := nestedString(payload, "object_attributes", "status")
		ref := nestedString(payload, "object_attributes", "ref")
		return fmt.Sprintf("GitLab pipeline %s on %s/%s", status, repo, ref), nil

	default:
		if kind == "" {
			// Fall back to JSON message extraction.
			return extractJSONMessage(body, "message")
		}
		return fmt.Sprintf("GitLab %s event on %s", kind, repo), nil
	}
}

// writeWebhookJSON writes a JSON response with the given status code.
func writeWebhookJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(v) //nolint:errcheck
}

// truncate shortens s to at most n bytes, appending "..." if trimmed.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// nestedString safely traverses nested map[string]interface{} values and returns
// the string at the end of the key path, or "" if any step is missing.
func nestedString(m map[string]interface{}, keys ...string) string {
	var cur interface{} = m
	for _, k := range keys {
		cm, ok := cur.(map[string]interface{})
		if !ok {
			return ""
		}
		cur = cm[k]
	}
	s, _ := cur.(string)
	return s
}

// stringVal returns the string value for a top-level key in a map.
func stringVal(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

// generateWebhookTestPayload builds a minimal test payload for a webhook.
// It is used by the `soulgate webhook test` CLI command.
func generateWebhookTestPayload(wh *WebhookConfig) ([]byte, map[string]string, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	var body []byte
	var err error

	switch wh.Format {
	case WebhookFormatText:
		body = []byte("This is a test message from SoulGate webhook test.")
		headers["Content-Type"] = "text/plain"

	case WebhookFormatGitHub:
		headers["X-GitHub-Event"] = "push"
		payload := map[string]interface{}{
			"ref": "refs/heads/main",
			"repository": map[string]interface{}{
				"full_name": "test-org/test-repo",
			},
			"pusher": map[string]interface{}{
				"name": "soulgate-test",
			},
			"commits": []interface{}{
				map[string]interface{}{
					"message": "Test commit from SoulGate webhook test",
				},
			},
		}
		body, err = json.Marshal(payload)

	case WebhookFormatGitLab:
		payload := map[string]interface{}{
			"object_kind": "push",
			"ref":         "refs/heads/main",
			"user_name":   "soulgate-test",
			"project": map[string]interface{}{
				"path_with_namespace": "test-group/test-project",
			},
			"commits": []interface{}{
				map[string]interface{}{
					"message": "Test commit from SoulGate webhook test",
				},
			},
		}
		body, err = json.Marshal(payload)

	default: // json
		key := wh.MessageKey
		if key == "" {
			key = "message"
		}
		// Build a nested object for dotted key paths.
		parts := strings.Split(key, ".")
		var build func(parts []string) interface{}
		build = func(parts []string) interface{} {
			if len(parts) == 1 {
				return "This is a test message from SoulGate webhook test."
			}
			return map[string]interface{}{parts[0]: build(parts[1:])}
		}
		root := map[string]interface{}{parts[0]: build(parts[1:])}
		// For single-part keys, build is the value directly.
		if len(parts) == 1 {
			root = map[string]interface{}{parts[0]: "This is a test message from SoulGate webhook test."}
		}
		body, err = json.Marshal(root)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("build test payload: %w", err)
	}

	// Sign with HMAC if a secret is configured.
	if wh.Secret != "" {
		mac := hmac.New(sha256.New, []byte(wh.Secret))
		mac.Write(body)
		headers["X-Hub-Signature-256"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	return body, headers, nil
}

// SendTestWebhook fires a local HTTP request against the gateway's own webhook
// endpoint for the given config and returns the response body.
func SendTestWebhook(port int, wh *WebhookConfig) (string, error) {
	body, headers, err := generateWebhookTestPayload(wh)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://localhost:%d/webhook/%s", port, wh.Name)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send test webhook: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return string(respBody), nil
}
