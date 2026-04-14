package workspaceenv

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSnapshotReloadsUpdatedValues(t *testing.T) {
	workspaceRoot := t.TempDir()
	envDir := filepath.Join(workspaceRoot, ".discobot")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	envPath := filepath.Join(envDir, "env")

	if err := os.WriteFile(envPath, []byte("ALPHA=first\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(first): %v", err)
	}
	if got := FileSnapshot(workspaceRoot)["ALPHA"]; got != "first" {
		t.Fatalf("ALPHA = %q, want first", got)
	}

	if err := os.WriteFile(envPath, []byte("ALPHA=second\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(second): %v", err)
	}
	if got := FileSnapshot(workspaceRoot)["ALPHA"]; got != "second" {
		t.Fatalf("ALPHA = %q, want second", got)
	}

	if err := os.WriteFile(envPath, []byte("BETA=only\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(third): %v", err)
	}
	env := FileSnapshot(workspaceRoot)
	if _, ok := env["ALPHA"]; ok {
		t.Fatalf("ALPHA should be absent after removal, env=%v", env)
	}
	if env["BETA"] != "only" {
		t.Fatalf("BETA = %q, want only", env["BETA"])
	}
}

func TestFileSnapshotIgnoresInvalidLinesAndPreservesLiterals(t *testing.T) {
	workspaceRoot := t.TempDir()
	envDir := filepath.Join(workspaceRoot, ".discobot")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	envPath := filepath.Join(envDir, "env")
	secret := "TOP_SECRET_SHOULD_NOT_APPEAR"
	content := strings.Join([]string{
		"VALID=ok",
		"export QUOTED='quoted value'",
		"MALICIOUS=$(touch /tmp/should-not-run)",
		"not an assignment " + secret,
		`BROKEN_QUOTE="` + secret,
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var logs strings.Builder
	restore := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(restore)

	env := FileSnapshot(workspaceRoot)
	if env["VALID"] != "ok" {
		t.Fatalf("VALID = %q, want ok", env["VALID"])
	}
	if env["QUOTED"] != "quoted value" {
		t.Fatalf("QUOTED = %q, want quoted value", env["QUOTED"])
	}
	if env["MALICIOUS"] != "$(touch /tmp/should-not-run)" {
		t.Fatalf("MALICIOUS = %q, want literal value", env["MALICIOUS"])
	}

	logText := logs.String()
	if got := strings.Count(logText, "ignoring invalid env line"); got != 2 {
		t.Fatalf("warning count = %d, want 2\nlogs:\n%s", got, logText)
	}
	if !strings.Contains(logText, envPath) {
		t.Fatalf("expected warnings to include env path, got:\n%s", logText)
	}
	if strings.Contains(logText, secret) {
		t.Fatalf("warnings leaked invalid line content:\n%s", logText)
	}
}
