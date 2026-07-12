package cmd

import (
	"strings"
	"testing"
)

func TestSpawnConnectorProcessRequiresConfig(t *testing.T) {
	tests := []struct {
		name          string
		connectorType string
		cfg           map[string]string
		wantMissing   string
	}{
		{"signal without phone", "signal", map[string]string{}, "phone"},
		{"telegram without token", "telegram", map[string]string{}, "token"},
		{"slack without bot token", "slack", map[string]string{"app_token": "xapp-1"}, "bot_token"},
		{"matrix without homeserver", "matrix", map[string]string{}, "homeserver"},
		{"teams without app password", "teams", map[string]string{"app_id": "id"}, "app_password"},
		{"irc without nick", "irc", map[string]string{}, "nick"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := spawnConnectorProcess(tt.connectorType, tt.cfg, 8080)
			if err == nil {
				t.Fatalf("expected error for %s with cfg %v, got nil", tt.connectorType, tt.cfg)
			}
			if !strings.Contains(err.Error(), tt.wantMissing) {
				t.Errorf("error %q should name the missing key %q", err, tt.wantMissing)
			}
		})
	}
}

func TestSpawnConnectorProcessRejectsUnknownKeys(t *testing.T) {
	// A config entry must never smuggle an arbitrary flag (e.g. --gateway)
	// into the spawned connector process.
	_, err := spawnConnectorProcess("signal", map[string]string{
		"phone":   "+15551234567",
		"gateway": "https://evil.example",
	}, 8080)
	if err == nil {
		t.Fatal("expected unknown config key 'gateway' to be rejected")
	}
	if !strings.Contains(err.Error(), "gateway") {
		t.Errorf("error %q should name the rejected key", err)
	}
}

func TestConnectorRequiredKeysMatchSpawnEnv(t *testing.T) {
	// The env-var connectors validated inline before the table existed;
	// make sure the table still covers them so cfg[key] lookups in the
	// switch never silently pass empty credentials.
	for _, connectorType := range []string{"telegram", "discord", "slack"} {
		if len(connectorRequiredKeys[connectorType]) == 0 {
			t.Errorf("connectorRequiredKeys missing entry for %q", connectorType)
		}
	}
}
