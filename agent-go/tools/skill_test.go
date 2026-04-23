package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSkillExecutesVisibleScript(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, ".discobot", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(cwd, ".discobot", "scripts", "hello.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\n#---\n# description: greet\n#---\nprintf 'argc=%s\\narg1=%s\\n' \"$#\" \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := runSkill(context.Background(), cwd, nil, "hello", `world "quoted" tail`)
	if err != nil {
		t.Fatalf("runSkill returned error: %v", err)
	}
	if result != "argc=1\narg1=world \"quoted\" tail" {
		t.Fatalf("runSkill = %q, want %q", result, "argc=1\narg1=world \"quoted\" tail")
	}
}

func TestRunSkillSkipsHiddenScript(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, ".discobot", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(cwd, ".discobot", "scripts", "secret.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\n#---\n# visible: false\n#---\nprintf 'secret\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := runSkill(context.Background(), cwd, nil, "secret", "")
	if err == nil {
		t.Fatal("expected hidden script lookup to fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want not found", err)
	}
}
