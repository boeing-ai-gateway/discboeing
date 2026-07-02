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

func TestRunSkillPrefixesSkillDirectory(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(cwd, ".claude", "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
description: Commit pending changes.
---

Review the pending changes and commit them.`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := runSkill(context.Background(), cwd, nil, "commit", "")
	if err != nil {
		t.Fatalf("runSkill returned error: %v", err)
	}
	want := "Skill directory: " + skillDir + "\n\n<commit>\nReview the pending changes and commit them.\n</commit>"
	if result != want {
		t.Fatalf("runSkill = %q, want %q", result, want)
	}
}

func TestRunSkillPrefixesCommandDirectory(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	commandDir := filepath.Join(cwd, ".claude", "commands")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "release.md"), []byte(`---
description: Tag a release.
---

Create and push the release tag.`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := runSkill(context.Background(), cwd, nil, "release", "")
	if err != nil {
		t.Fatalf("runSkill returned error: %v", err)
	}
	want := "Command directory: " + commandDir + "\n\n<release>\nCreate and push the release tag.\n</release>"
	if result != want {
		t.Fatalf("runSkill = %q, want %q", result, want)
	}
}

func TestRunSkillExecutesVisibleScript(t *testing.T) {
	t.Parallel()

	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cwd, ".discboeing", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptDir := filepath.Join(cwd, ".discboeing", "scripts")
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
	if err := os.MkdirAll(filepath.Join(cwd, ".discboeing", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeHiddenSkillScript(t, filepath.Join(cwd, ".discboeing", "scripts"), "secret")

	_, err := runSkill(context.Background(), cwd, nil, "secret", "")
	if err == nil {
		t.Fatal("expected hidden script lookup to fail")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want not found", err)
	}
}
