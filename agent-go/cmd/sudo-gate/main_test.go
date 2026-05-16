//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGateConfigRejectsNonRootAccessibleFile(t *testing.T) {
	path := writeGateConfig(t, 0o644)

	_, err := loadGateConfig(path)
	if err == nil {
		t.Fatal("expected non-root-accessible config to be rejected")
	}
}

func TestParseGateConfigIgnoresEnvironmentOverrides(t *testing.T) {
	t.Setenv("DISCOBOT_REAL_SUDO", "/tmp/malicious-sudo")
	t.Setenv("DISCOBOT_PORT", "1")

	cfg, err := parseGateConfig(fmt.Appendf(nil, `{"realSudo":%q,"agentAPIURL":%q}`, "/usr/lib/discobot/sudo.real", "http://127.0.0.1:3002/sudo/authorize"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RealSudo != "/usr/lib/discobot/sudo.real" {
		t.Fatalf("real sudo path should come from config, got %q", cfg.RealSudo)
	}
	if cfg.AgentAPIURL != "http://127.0.0.1:3002/sudo/authorize" {
		t.Fatalf("agent API URL should come from config, got %q", cfg.AgentAPIURL)
	}
}

func TestValidateGateConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  gateConfig
		want bool
	}{
		{
			name: "valid loopback URL",
			cfg: gateConfig{
				RealSudo:    "/usr/lib/discobot/sudo.real",
				AgentAPIURL: "http://127.0.0.1:3002/sudo/authorize",
			},
			want: true,
		},
		{
			name: "rejects relative real sudo path",
			cfg: gateConfig{
				RealSudo:    "sudo.real",
				AgentAPIURL: "http://127.0.0.1:3002/sudo/authorize",
			},
		},
		{
			name: "rejects gate recursion",
			cfg: gateConfig{
				RealSudo:    "/usr/bin/sudo",
				AgentAPIURL: "http://127.0.0.1:3002/sudo/authorize",
			},
		},
		{
			name: "rejects non-loopback URL",
			cfg: gateConfig{
				RealSudo:    "/usr/lib/discobot/sudo.real",
				AgentAPIURL: "http://192.0.2.10:3002/sudo/authorize",
			},
		},
		{
			name: "rejects wrong path",
			cfg: gateConfig{
				RealSudo:    "/usr/lib/discobot/sudo.real",
				AgentAPIURL: "http://127.0.0.1:3002/health",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGateConfig(tt.cfg)
			if tt.want && err != nil {
				t.Fatalf("expected config to be valid: %v", err)
			}
			if !tt.want && err == nil {
				t.Fatal("expected config to be rejected")
			}
		})
	}
}

func TestAuthorizeDoesNotSendBearerAuth(t *testing.T) {
	t.Setenv("DISCOBOT_SECRET", "legacy-secret")
	t.Setenv("DISCOBOT_SUDO_RUNTIME", "agent")
	t.Setenv("DISCOBOT_SUDO_TOKEN", "sudo-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q, want empty", got)
		}
		_ = json.NewEncoder(w).Encode(authorizeResponse{Allow: true})
	}))
	defer server.Close()

	resp, err := authorize(gateConfig{AgentAPIURL: server.URL, RealSudo: "/usr/lib/discobot/sudo.real"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Allow {
		t.Fatalf("expected authorize response to allow, got %#v", resp)
	}
}

func writeGateConfig(t *testing.T, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sudo-gate.json")
	content := fmt.Sprintf(`{"realSudo":%q,"agentAPIURL":%q}`, "/usr/lib/discobot/sudo.real", "http://127.0.0.1:3002/sudo/authorize")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return path
}
