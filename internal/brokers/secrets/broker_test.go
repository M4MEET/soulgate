package secrets

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "sk-super-secret-value-12345"

func newTestBroker(t *testing.T) (*SecretBroker, string) {
	t.Helper()
	dir := t.TempDir()
	sb, err := NewBroker(dir, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Close() })
	return sb, dir
}

func TestSecretBrokerSetGetRoundTrip(t *testing.T) {
	sb, _ := newTestBroker(t)

	require.NoError(t, sb.Set("github_token", testSecret, "github", "CI token"))

	val, err := sb.Get("github_token")
	require.NoError(t, err)
	assert.Equal(t, testSecret, val)
}

func TestSecretBrokerEncryptedAtRest(t *testing.T) {
	sb, dir := newTestBroker(t)
	require.NoError(t, sb.Set("api_key", testSecret, "openai", ""))

	data, err := os.ReadFile(filepath.Join(dir, "security", "secrets.json"))
	require.NoError(t, err)

	assert.NotContains(t, string(data), testSecret,
		"plaintext secret must never appear in the on-disk store")
	assert.Contains(t, string(data), "api_key", "handle metadata should be stored")
}

func TestSecretBrokerFilePermissions(t *testing.T) {
	sb, dir := newTestBroker(t)
	require.NoError(t, sb.Set("k", testSecret, "", ""))

	info, err := os.Stat(filepath.Join(dir, "security", "secrets.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(),
		"secrets file must be owner-only")
}

func TestSecretBrokerPersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()

	sb1, err := NewBroker(dir, nil)
	require.NoError(t, err)
	require.NoError(t, sb1.Set("k", testSecret, "p", "d"))
	require.NoError(t, sb1.Close())

	// Same machine + same config dir derives the same key.
	sb2, err := NewBroker(dir, nil)
	require.NoError(t, err)
	defer sb2.Close()

	val, err := sb2.Get("k")
	require.NoError(t, err)
	assert.Equal(t, testSecret, val)
}

func TestSecretBrokerMachineBinding(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	sbA, err := NewBroker(dirA, nil)
	require.NoError(t, err)
	require.NoError(t, sbA.Set("k", testSecret, "", ""))
	require.NoError(t, sbA.Close())

	// Simulate copying the secrets file to a different install location:
	// the derived key differs, so decryption must fail.
	require.NoError(t, os.MkdirAll(filepath.Join(dirB, "security"), 0o700))
	data, err := os.ReadFile(filepath.Join(dirA, "security", "secrets.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dirB, "security", "secrets.json"), data, 0o600))

	sbB, err := NewBroker(dirB, nil)
	require.NoError(t, err)
	defer sbB.Close()

	_, err = sbB.Get("k")
	assert.Error(t, err, "copied secrets file must not decrypt under a different derived key")
}

func TestSecretBrokerListNeverExposesValues(t *testing.T) {
	sb, _ := newTestBroker(t)
	require.NoError(t, sb.Set("k1", testSecret, "github", "desc"))

	infos := sb.List()
	require.Len(t, infos, 1)
	assert.Equal(t, "k1", infos[0].Handle)

	// The tool-facing list output must not leak plaintext either.
	out, err := sb.ExecuteTool(context.Background(), "secret_list", []byte(`{}`))
	require.NoError(t, err)
	assert.NotContains(t, out, testSecret)
}

func TestSecretBrokerDelete(t *testing.T) {
	sb, _ := newTestBroker(t)
	ctx := context.Background()

	require.NoError(t, sb.Set("k", testSecret, "", ""))
	require.NoError(t, sb.Delete(ctx, "k"))

	_, err := sb.Get("k")
	assert.Error(t, err)

	assert.Error(t, sb.Delete(ctx, "k"), "double delete should error")
}

func TestSecretBrokerRotate(t *testing.T) {
	sb, dir := newTestBroker(t)
	ctx := context.Background()

	require.NoError(t, sb.Set("k", "old-value", "", ""))
	require.NoError(t, sb.Rotate(ctx, "k", "new-value"))

	val, err := sb.Get("k")
	require.NoError(t, err)
	assert.Equal(t, "new-value", val)

	data, err := os.ReadFile(filepath.Join(dir, "security", "secrets.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "old-value")
	assert.NotContains(t, string(data), "new-value")

	assert.Error(t, sb.Rotate(ctx, "missing", "v"))
	assert.Error(t, sb.Rotate(ctx, "k", ""), "empty rotation value must be rejected")
}

func TestSecretBrokerValidation(t *testing.T) {
	sb, _ := newTestBroker(t)

	assert.Error(t, sb.Set("", "v", "", ""), "empty handle rejected")
	assert.Error(t, sb.Set("h", "", "", ""), "empty value rejected")

	_, err := sb.Get("nope")
	assert.Error(t, err)
}

func TestSecretBrokerInjectHeader(t *testing.T) {
	sb, _ := newTestBroker(t)
	require.NoError(t, sb.Set("token", testSecret, "", ""))

	req, err := http.NewRequest(http.MethodGet, "https://api.example.com", nil)
	require.NoError(t, err)

	require.NoError(t, sb.InjectHeader(context.Background(), "token", req))
	assert.Equal(t, "Bearer "+testSecret, req.Header.Get("Authorization"))
}

func TestSecretBrokerInjectEnv(t *testing.T) {
	sb, _ := newTestBroker(t)
	require.NoError(t, sb.Set("github-token", testSecret, "", ""))

	env, err := sb.InjectEnv(context.Background(), "github-token")
	require.NoError(t, err)
	assert.Equal(t, "GITHUB_TOKEN="+testSecret, env)
}

func TestSecretBrokerToolSurfaceHasNoGet(t *testing.T) {
	// The model must never be able to read a secret back.
	for _, schema := range ToolSchemas() {
		name := schema["name"].(string)
		assert.NotContains(t, name, "get")
		assert.NotContains(t, name, "retrieve")
	}

	sb, _ := newTestBroker(t)
	require.NoError(t, sb.Set("k", testSecret, "", ""))

	// secret_inject acknowledges without returning the value.
	out, err := sb.ExecuteTool(context.Background(), "secret_inject",
		[]byte(`{"handle":"k","url":"https://api.example.com"}`))
	require.NoError(t, err)
	assert.NotContains(t, out, testSecret)

	_, err = sb.ExecuteTool(context.Background(), "secret_get", []byte(`{"handle":"k"}`))
	assert.Error(t, err, "unknown tools must be rejected")
}

func TestToEnvKey(t *testing.T) {
	cases := map[string]string{
		"github_token": "GITHUB_TOKEN",
		"github-token": "GITHUB_TOKEN",
		"aws key":      "AWS_KEY",
		"ALREADY":      "ALREADY",
	}
	for in, want := range cases {
		if got := toEnvKey(in); got != want {
			t.Errorf("toEnvKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSecretBrokerConcurrentAccess(t *testing.T) {
	sb, _ := newTestBroker(t)
	require.NoError(t, sb.Set("k", testSecret, "", ""))

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 25; j++ {
				_, _ = sb.Get("k")
				_ = sb.List()
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}

	val, err := sb.Get("k")
	require.NoError(t, err)
	assert.Equal(t, testSecret, val)
}
