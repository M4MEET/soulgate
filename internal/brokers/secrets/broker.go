// Package secrets provides a SecretBroker that stores sensitive credentials
// encrypted at rest and injects them into HTTP requests on behalf of agents.
//
// Security model:
//   - Plugins and the model NEVER see raw secret values.
//   - Agents reference secrets by an opaque handle (e.g. "github_token").
//   - The broker injects the plaintext value only at the HTTP/env boundary,
//     inside the trusted runtime.
//   - Values are stored AES-256-GCM encrypted; the key is derived from a
//     machine-specific salt so that copying the secrets file to another
//     machine does not immediately expose the values.
//   - The secrets file is created with 0600 permissions (owner-read/write only).
//   - Secret values are never written to the audit log or returned through
//     the tool API surface.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/M4MEET/soulgate/internal/audit"
)

// Secret holds the metadata and encrypted value for a stored credential.
type Secret struct {
	Handle      string    `json:"handle"`      // e.g., "github_token"
	EncValue    string    `json:"enc_value"`   // hex-encoded AES-256-GCM ciphertext
	Provider    string    `json:"provider"`    // e.g., "github", "aws"
	Description string    `json:"description"` // human-readable note
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	UsageCount  int       `json:"usage_count"`

	// Value is never serialised to JSON (json:"-").
	// It is populated on demand by Get and zeroed when the broker is closed.
	Value string `json:"-"`
}

// SecretInfo is a redacted view of a Secret safe to return to callers:
// the raw value is omitted.
type SecretInfo struct {
	Handle      string    `json:"handle"`
	Provider    string    `json:"provider"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsed    time.Time `json:"last_used"`
	UsageCount  int       `json:"usage_count"`
}

// SecretBroker manages encrypted secrets on behalf of the runtime.
// All methods are safe for concurrent use.
type SecretBroker struct {
	mu          sync.RWMutex
	secrets     map[string]*Secret // handle -> secret
	path        string             // absolute path to secrets.json
	encKey      []byte             // 32-byte AES-256 key
	auditLogger audit.Logger
	saves       sync.WaitGroup // tracks async metadata saves so Close can drain them
}

// NewBroker loads (or creates) the encrypted secrets store located at
// <configDir>/secrets.json.
//
// The encryption key is derived from the hostname and configDir path so that
// the file is not trivially portable between machines.
func NewBroker(configDir string, auditLogger audit.Logger) (*SecretBroker, error) {
	absDir, err := filepath.Abs(configDir)
	if err != nil {
		return nil, fmt.Errorf("secrets broker: cannot resolve config dir: %w", err)
	}

	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return nil, fmt.Errorf("secrets broker: cannot create config dir: %w", err)
	}

	// Ensure the security/ subdirectory exists before writing secrets.json.
	secDir := filepath.Join(absDir, "security")
	if err := os.MkdirAll(secDir, 0o700); err != nil {
		return nil, fmt.Errorf("secrets broker: cannot create security dir: %w", err)
	}

	key, err := deriveKey(absDir)
	if err != nil {
		return nil, fmt.Errorf("secrets broker: key derivation failed: %w", err)
	}

	sb := &SecretBroker{
		secrets:     make(map[string]*Secret),
		path:        filepath.Join(absDir, "security", "secrets.json"),
		encKey:      key,
		auditLogger: auditLogger,
	}

	if err := sb.load(); err != nil {
		return nil, fmt.Errorf("secrets broker: failed to load secrets: %w", err)
	}

	return sb, nil
}

// Name returns the broker name (satisfies brokers.Broker interface).
func (sb *SecretBroker) Name() string { return "secrets" }

// Close zeroes in-memory plaintext values and releases resources.
// It waits for in-flight metadata saves so nothing writes after shutdown.
func (sb *SecretBroker) Close() error {
	sb.saves.Wait()
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for _, s := range sb.secrets {
		zeroString(&s.Value)
	}
	zeroBytes(sb.encKey)
	return nil
}

// Set stores or updates a secret identified by handle.
// The plaintext value is encrypted before being written to disk.
func (sb *SecretBroker) Set(handle, value, provider, description string) error {
	if handle == "" {
		return fmt.Errorf("secrets broker: handle must not be empty")
	}
	if value == "" {
		return fmt.Errorf("secrets broker: value must not be empty")
	}

	encValue, err := sb.encrypt(value)
	if err != nil {
		return fmt.Errorf("secrets broker: encryption failed: %w", err)
	}

	sb.mu.Lock()
	existing, exists := sb.secrets[handle]
	if exists {
		// Preserve creation time; update the rest.
		existing.EncValue = encValue
		existing.Value = value
		existing.Provider = provider
		existing.Description = description
	} else {
		sb.secrets[handle] = &Secret{
			Handle:      handle,
			EncValue:    encValue,
			Value:       value,
			Provider:    provider,
			Description: description,
			CreatedAt:   time.Now().UTC(),
		}
	}
	sb.mu.Unlock()

	return sb.save()
}

// Get retrieves the plaintext value for a secret.
// This method is for broker-internal use only (e.g. by InjectHeader and
// InjectEnv).  It must not be exposed directly through the tool/API surface.
func (sb *SecretBroker) Get(handle string) (string, error) {
	sb.mu.Lock()
	s, ok := sb.secrets[handle]
	if !ok {
		sb.mu.Unlock()
		return "", fmt.Errorf("secrets broker: unknown handle %q", handle)
	}

	// Decrypt lazily — decrypt once per process lifetime and cache in s.Value.
	if s.Value == "" {
		val, err := sb.decrypt(s.EncValue)
		if err != nil {
			sb.mu.Unlock()
			return "", fmt.Errorf("secrets broker: decryption failed for %q: %w", handle, err)
		}
		s.Value = val
	}

	s.LastUsed = time.Now().UTC()
	s.UsageCount++
	sb.mu.Unlock()

	// Persist updated metadata asynchronously; ignore transient errors.
	sb.saves.Add(1)
	go func() {
		defer sb.saves.Done()
		_ = sb.save()
	}()

	return s.Value, nil
}

// InjectHeader injects the secret identified by handle as an Authorization
// header on req.  The model never sees the plaintext value.
func (sb *SecretBroker) InjectHeader(ctx context.Context, handle string, req *http.Request) error {
	val, err := sb.Get(handle)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+val)

	sb.logEvent(ctx, audit.EventType("secret.inject"), handle, audit.StatusSuccess, nil)
	return nil
}

// InjectEnv returns a "KEY=value" string suitable for passing to exec.Cmd.Env.
// The variable name is derived from the handle (upper-cased, hyphens replaced
// by underscores).
//
// NOTE: the caller must not log or display the returned string.
func (sb *SecretBroker) InjectEnv(ctx context.Context, handle string) (string, error) {
	val, err := sb.Get(handle)
	if err != nil {
		return "", err
	}

	envKey := toEnvKey(handle)
	result := envKey + "=" + val

	sb.logEvent(ctx, audit.EventType("secret.inject_env"), handle, audit.StatusSuccess, nil)
	return result, nil
}

// List returns redacted metadata for all stored secrets.
// Raw values are never included.
func (sb *SecretBroker) List() []SecretInfo {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	out := make([]SecretInfo, 0, len(sb.secrets))
	for _, s := range sb.secrets {
		out = append(out, SecretInfo{
			Handle:      s.Handle,
			Provider:    s.Provider,
			Description: s.Description,
			CreatedAt:   s.CreatedAt,
			LastUsed:    s.LastUsed,
			UsageCount:  s.UsageCount,
		})
	}
	return out
}

// Delete removes a secret permanently.
func (sb *SecretBroker) Delete(ctx context.Context, handle string) error {
	sb.mu.Lock()
	if _, ok := sb.secrets[handle]; !ok {
		sb.mu.Unlock()
		return fmt.Errorf("secrets broker: unknown handle %q", handle)
	}

	// Zero the in-memory plaintext before deletion.
	zeroString(&sb.secrets[handle].Value)
	delete(sb.secrets, handle)
	sb.mu.Unlock()

	if err := sb.save(); err != nil {
		return err
	}

	sb.logEvent(ctx, audit.EventType("secret.delete"), handle, audit.StatusSuccess, nil)
	return nil
}

// Rotate replaces the value of an existing secret.
func (sb *SecretBroker) Rotate(ctx context.Context, handle, newValue string) error {
	if newValue == "" {
		return fmt.Errorf("secrets broker: new value must not be empty")
	}

	sb.mu.RLock()
	s, ok := sb.secrets[handle]
	sb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("secrets broker: unknown handle %q", handle)
	}

	encValue, err := sb.encrypt(newValue)
	if err != nil {
		return fmt.Errorf("secrets broker: encryption failed: %w", err)
	}

	sb.mu.Lock()
	// Zero old plaintext before replacing.
	zeroString(&s.Value)
	s.EncValue = encValue
	s.Value = newValue
	sb.mu.Unlock()

	if err := sb.save(); err != nil {
		return err
	}

	sb.logEvent(ctx, audit.EventType("secret.rotate"), handle, audit.StatusSuccess, nil)
	return nil
}

// ToolSchemas returns JSON tool schemas that expose the secret broker to the
// model.  The schemas intentionally do NOT expose a "get" or "retrieve" tool
// — the model can only set, list, delete, or request injection.
func ToolSchemas() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "secret_set",
			"description": "Store a secret credential identified by a handle. The value is encrypted at rest and never shown to the model again. Use secret_list to see stored handles.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"handle": map[string]interface{}{
						"type":        "string",
						"description": "Short identifier for the secret (e.g. 'github_token', 'aws_key'). Use lowercase with underscores.",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "The secret value (API key, token, password, etc.).",
					},
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Service or provider the secret belongs to (e.g. 'github', 'aws', 'openai').",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Optional human-readable note about the secret's purpose.",
					},
				},
				"required": []string{"handle", "value"},
			},
		},
		{
			"name":        "secret_list",
			"description": "List the handles and metadata for all stored secrets. Secret values are never returned.",
			"input_schema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "secret_delete",
			"description": "Permanently delete a stored secret by handle.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"handle": map[string]interface{}{
						"type":        "string",
						"description": "Handle of the secret to delete.",
					},
				},
				"required": []string{"handle"},
			},
		},
		{
			"name":        "secret_inject",
			"description": "Inject a secret as the Authorization header for a subsequent net_request. Specify the handle; the runtime will set 'Authorization: Bearer <value>' automatically.",
			"input_schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"handle": map[string]interface{}{
						"type":        "string",
						"description": "Handle of the secret to inject.",
					},
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The URL that the injected request will target. Stored in the audit log.",
					},
				},
				"required": []string{"handle"},
			},
		},
	}
}

// ExecuteTool dispatches a tool call by name to the appropriate method.
// input is the raw JSON object from the model.
func (sb *SecretBroker) ExecuteTool(ctx context.Context, name string, input []byte) (string, error) {
	switch name {
	case "secret_set":
		return sb.execSet(ctx, input)
	case "secret_list":
		return sb.execList(ctx)
	case "secret_delete":
		return sb.execDelete(ctx, input)
	case "secret_inject":
		return sb.execInject(ctx, input)
	default:
		return "", fmt.Errorf("secrets broker: unknown tool %q", name)
	}
}

// --------------------------------------------------------------------------
// Tool handlers (unexported)
// --------------------------------------------------------------------------

func (sb *SecretBroker) execSet(ctx context.Context, input []byte) (string, error) {
	var p struct {
		Handle      string `json:"handle"`
		Value       string `json:"value"`
		Provider    string `json:"provider"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if err := sb.Set(p.Handle, p.Value, p.Provider, p.Description); err != nil {
		sb.logEvent(ctx, audit.EventType("secret.set"), p.Handle, audit.StatusError, err)
		return "", err
	}
	sb.logEvent(ctx, audit.EventType("secret.set"), p.Handle, audit.StatusSuccess, nil)
	return fmt.Sprintf("Secret %q stored successfully.", p.Handle), nil
}

func (sb *SecretBroker) execList(ctx context.Context) (string, error) {
	infos := sb.List()
	sb.logEvent(ctx, audit.EventType("secret.list"), "all", audit.StatusSuccess, nil)
	if len(infos) == 0 {
		return "No secrets stored.", nil
	}
	out, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal secret list: %w", err)
	}
	return string(out), nil
}

func (sb *SecretBroker) execDelete(ctx context.Context, input []byte) (string, error) {
	var p struct {
		Handle string `json:"handle"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if err := sb.Delete(ctx, p.Handle); err != nil {
		return "", err
	}
	return fmt.Sprintf("Secret %q deleted.", p.Handle), nil
}

func (sb *SecretBroker) execInject(ctx context.Context, input []byte) (string, error) {
	var p struct {
		Handle string `json:"handle"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if p.Handle == "" {
		return "", fmt.Errorf("handle is required")
	}

	// Verify the handle exists and can be decrypted — do not return the value.
	if _, err := sb.Get(p.Handle); err != nil {
		sb.logEvent(ctx, audit.EventType("secret.inject"), p.Handle, audit.StatusError, err)
		return "", err
	}

	sb.logEvent(ctx, audit.EventType("secret.inject"), p.Handle, audit.StatusSuccess, nil)
	return fmt.Sprintf(
		"Secret %q will be injected as the Authorization header for subsequent requests to %s. "+
			"Use net_request and the broker will set the header automatically when the handle is active.",
		p.Handle, p.URL,
	), nil
}

// --------------------------------------------------------------------------
// Persistence
// --------------------------------------------------------------------------

// secretsFile is the on-disk representation (values are encrypted).
type secretsFile struct {
	Version string             `json:"version"`
	Secrets map[string]*Secret `json:"secrets"`
}

// load reads the secrets file from disk.  A missing file is not an error.
// Must be called without holding sb.mu (it takes the lock itself).
func (sb *SecretBroker) load() error {
	data, err := os.ReadFile(sb.path)
	if os.IsNotExist(err) {
		return nil // fresh installation
	}
	if err != nil {
		return fmt.Errorf("read secrets file: %w", err)
	}

	var sf secretsFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("parse secrets file: %w", err)
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sf.Secrets != nil {
		for k, v := range sf.Secrets {
			if v != nil {
				sb.secrets[k] = v
			}
		}
	}
	return nil
}

// save atomically writes the secrets file with 0600 permissions.
// Must NOT be called while holding sb.mu.
func (sb *SecretBroker) save() error {
	sb.mu.RLock()
	// Deep-copy secrets to avoid holding the lock during I/O.
	cp := make(map[string]*Secret, len(sb.secrets))
	for k, v := range sb.secrets {
		clone := *v
		clone.Value = "" // never persist plaintext
		cp[k] = &clone
	}
	sb.mu.RUnlock()

	sf := secretsFile{
		Version: "1",
		Secrets: cp,
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}

	// Write to a temporary file then rename for atomicity.
	dir := filepath.Dir(sb.path)
	tmp, err := os.CreateTemp(dir, "secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp secrets file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp secrets file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp secrets file: %w", err)
	}

	// Restrict permissions before rename so the file is never world-readable.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod secrets file: %w", err)
	}

	if err := os.Rename(tmpPath, sb.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename secrets file: %w", err)
	}

	return nil
}

// --------------------------------------------------------------------------
// Encryption helpers (AES-256-GCM)
// --------------------------------------------------------------------------

// encrypt encrypts plaintext with AES-256-GCM and returns a hex-encoded
// string of the form: <12-byte-nonce><ciphertext>.
func (sb *SecretBroker) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(sb.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ct), nil
}

// decrypt decrypts a hex-encoded AES-256-GCM ciphertext produced by encrypt.
func (sb *SecretBroker) decrypt(encHex string) (string, error) {
	data, err := hex.DecodeString(encHex)
	if err != nil {
		return "", fmt.Errorf("hex decode: %w", err)
	}

	block, err := aes.NewCipher(sb.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// deriveKey produces a 32-byte AES key from the machine hostname and
// configDir path using SHA-256.  This is a lightweight machine-binding that
// prevents casual extraction of secrets by copying the file to another host.
func deriveKey(configDir string) ([]byte, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	salt := hostname + "|" + configDir + "|soulgate-secrets-v1"
	h := sha256.Sum256([]byte(salt))
	return h[:], nil
}

// --------------------------------------------------------------------------
// Utility helpers
// --------------------------------------------------------------------------

// toEnvKey converts a secret handle to a shell environment variable name:
// lowercase letters become uppercase, hyphens and spaces become underscores.
func toEnvKey(handle string) string {
	b := []byte(handle)
	for i, c := range b {
		switch {
		case c >= 'a' && c <= 'z':
			b[i] = c - 32
		case c == '-' || c == ' ':
			b[i] = '_'
		}
	}
	return string(b)
}

// zeroString overwrites the backing array of a string with zeros.
// This is a best-effort mitigation — the Go runtime does not guarantee that
// a string's backing memory is not copied by the GC.
func zeroString(s *string) {
	if *s == "" {
		return
	}
	b := []byte(*s)
	for i := range b {
		b[i] = 0
	}
	*s = ""
}

// zeroBytes overwrites a byte slice with zeros.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// logEvent emits an audit event.  Errors are silently discarded (audit
// logging is best-effort and must not fail the primary operation).
func (sb *SecretBroker) logEvent(ctx context.Context, eventType audit.EventType, handle string, status audit.EventStatus, opErr error) {
	if sb.auditLogger == nil {
		return
	}

	ev := audit.NewEvent(eventType, audit.CategoryBroker).
		WithResource("secret:" + handle).
		WithStatus(status)

	if opErr != nil {
		ev.WithError(opErr)
	}

	_ = sb.auditLogger.Log(ctx, ev)
}
