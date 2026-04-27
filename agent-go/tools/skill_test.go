package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeVisibleSkillScript(t *testing.T, dir, name string) {
	t.Helper()

	if runtime.GOOS != "windows" {
		scriptPath := filepath.Join(dir, name+".sh")
		content := "#!/bin/sh\n#---\n# description: greet\n#---\nprintf 'argc=%s\\narg1=%s\\n' \"$#\" \"$1\"\n"
		if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		return
	}

	scriptPath := filepath.Join(dir, name+".ps1")
	content := "#!/usr/bin/env pwsh\n#---\n# description: greet\n#---\nWrite-Output (\"argc=$($args.Count)`narg1=$($args[0])\")\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeHiddenSkillScript(t *testing.T, dir, name string) {
	t.Helper()

	if runtime.GOOS != "windows" {
		scriptPath := filepath.Join(dir, name+".sh")
		content := "#!/bin/sh\n#---\n# visible: false\n#---\nprintf 'secret\\n'\n"
		if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		return
	}

	scriptPath := filepath.Join(dir, name+".ps1")
	content := "#!/usr/bin/env pwsh\n#---\n# visible: false\n#---\nWrite-Output 'secret'\n"
	if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunSkillExecutesVisibleScript(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, ".discobot", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptDir := filepath.Join(cwd, ".discobot", "scripts")
	writeVisibleSkillScript(t, scriptDir, "hello")

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

	writeHiddenSkillScript(t, filepath.Join(cwd, ".discobot", "scripts"), "secret")

	_, err := runSkill(context.Background(), cwd, nil, "secret", "")
	if err == nil {
		t.Fatal("expected hidden script lookup to fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want not found", err)
	}
}
