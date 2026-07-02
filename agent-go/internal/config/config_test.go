package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDiscboeingEnvVars(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("DISCBOEING_PORT", "4123")
	t.Setenv("DISCBOEING_SECRET", "secret-hash")
	t.Setenv("DISCBOEING_AGENT_CWD", "/tmp/workspace")
	t.Setenv("DISCBOEING_MODEL", "openai/gpt-5.4")
	t.Setenv("WORKSPACE_SOURCE", "https://example.com/repo.git")
	t.Setenv("DISCBOEING_DATA_DIR", "/tmp/discboeing-data")
	t.Setenv("DISCBOEING_THREADS_DIR", "/tmp/discboeing-threads")
	t.Setenv("DISCBOEING_HOOKS_ENABLED", "true")
	t.Setenv("DISCBOEING_SESSION_ID", "session-123")
	t.Setenv("DISCBOEING_MCP_OAUTH_REDIRECT_BASE", "http://127.0.0.1:9999")
	t.Setenv("DISCBOEING_SERVER_URL", "http://127.0.0.1:3001")
	t.Setenv("DISCBOEING_PROJECT_ID", "project-123")

	cfg := Load()

	if cfg.Port != 4123 {
		t.Fatalf("expected port 4123, got %d", cfg.Port)
	}
	if cfg.SecretHash != "secret-hash" {
		t.Fatalf("expected secret hash to load from DISCBOEING_SECRET, got %q", cfg.SecretHash)
	}
	if cfg.AgentCwd != "/tmp/workspace" {
		t.Fatalf("expected agent cwd to load from DISCBOEING_AGENT_CWD, got %q", cfg.AgentCwd)
	}
	if cfg.Model != "openai/gpt-5.4" {
		t.Fatalf("expected model to load from DISCBOEING_MODEL, got %q", cfg.Model)
	}
	if cfg.WorkspaceSource != "https://example.com/repo.git" {
		t.Fatalf("expected workspace source to load from WORKSPACE_SOURCE, got %q", cfg.WorkspaceSource)
	}
	if cfg.DataDir != "/tmp/discboeing-data" {
		t.Fatalf("expected data dir to load from DISCBOEING_DATA_DIR, got %q", cfg.DataDir)
	}
	if cfg.ThreadsDir != "/tmp/discboeing-threads" {
		t.Fatalf("expected threads dir to load from DISCBOEING_THREADS_DIR, got %q", cfg.ThreadsDir)
	}
	if !cfg.HooksEnabled {
		t.Fatal("expected hooks to be enabled from DISCBOEING_HOOKS_ENABLED")
	}
	if cfg.SessionID != "session-123" {
		t.Fatalf("expected session ID to load from DISCBOEING_SESSION_ID, got %q", cfg.SessionID)
	}
	if cfg.MCPOAuthRedirectBase != "http://127.0.0.1:9999" {
		t.Fatalf("expected MCP OAuth redirect base to load from DISCBOEING_MCP_OAUTH_REDIRECT_BASE, got %q", cfg.MCPOAuthRedirectBase)
	}
	if cfg.DiscboeingServerURL != "http://127.0.0.1:3001" {
		t.Fatalf("expected server URL to load from DISCBOEING_SERVER_URL, got %q", cfg.DiscboeingServerURL)
	}
	if cfg.DiscboeingProjectID != "project-123" {
		t.Fatalf("expected project ID to load from DISCBOEING_PROJECT_ID, got %q", cfg.DiscboeingProjectID)
	}
}

func TestLoadIgnoresLegacyUnprefixedConfigEnvVars(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	t.Setenv("PORT", "4123")
	t.Setenv("AGENT_CWD", "/tmp/workspace")
	t.Setenv("MODEL", "openai/gpt-5.4")
	t.Setenv("DATA_DIR", "/tmp/discboeing-data")
	t.Setenv("THREADS_DIR", "/tmp/discboeing-threads")
	t.Setenv("MCP_OAUTH_REDIRECT_BASE", "http://127.0.0.1:9999")

	t.Setenv("DISCBOEING_PORT", "")
	t.Setenv("DISCBOEING_AGENT_CWD", "")
	t.Setenv("DISCBOEING_MODEL", "")
	t.Setenv("DISCBOEING_DATA_DIR", "")
	t.Setenv("DISCBOEING_THREADS_DIR", "")
	t.Setenv("DISCBOEING_SESSION_ID", "")
	t.Setenv("DISCBOEING_MCP_OAUTH_REDIRECT_BASE", "")
	t.Setenv("WORKSPACE_PATH", "")

	cfg := Load()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Port != 3002 {
		t.Fatalf("expected default port 3002 when DISCBOEING_PORT is unset, got %d", cfg.Port)
	}
	if cfg.AgentCwd != cwd {
		t.Fatalf("expected default cwd %q when DISCBOEING_AGENT_CWD is unset, got %q", cwd, cfg.AgentCwd)
	}
	if cfg.Model != "" {
		t.Fatalf("expected empty model when DISCBOEING_MODEL is unset, got %q", cfg.Model)
	}
	if want := filepath.Join(homeDir, ".discboeing"); cfg.DataDir != want {
		t.Fatalf("expected default data dir %q, got %q", want, cfg.DataDir)
	}
	if want := filepath.Join(homeDir, ".discboeing", "threads"); cfg.ThreadsDir != want {
		t.Fatalf("expected default threads dir %q, got %q", want, cfg.ThreadsDir)
	}
	if cfg.SessionID != "default" {
		t.Fatalf("expected default session ID when DISCBOEING_SESSION_ID is unset, got %q", cfg.SessionID)
	}
	if cfg.MCPOAuthRedirectBase != "" {
		t.Fatalf("expected empty MCP OAuth redirect base when DISCBOEING_MCP_OAUTH_REDIRECT_BASE is unset, got %q", cfg.MCPOAuthRedirectBase)
	}
}

func TestLoadUsesWorkspacePathAsAgentCwdFallback(t *testing.T) {
	t.Setenv("DISCBOEING_AGENT_CWD", "")
	t.Setenv("WORKSPACE_PATH", "/home/discboeing/workspace")

	cfg := Load()

	if cfg.AgentCwd != "/home/discboeing/workspace" {
		t.Fatalf("expected agent cwd to fall back to WORKSPACE_PATH, got %q", cfg.AgentCwd)
	}
}
