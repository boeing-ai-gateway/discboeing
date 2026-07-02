package sessionconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLookupSkillLike_PrioritizesSkillOverCommandAndScript(t *testing.T) {
	root := t.TempDir()

	skillDir := filepath.Join(root, ".claude", "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: skill\n---\nskill body"), 0o644); err != nil {
		t.Fatal(err)
	}

	commandDir := filepath.Join(root, ".claude", "commands", "commit")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "SKILL.md"), []byte("---\ndescription: command\n---\ncommand body"), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptDir := filepath.Join(root, ".discboeing", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "commit"), []byte("#!/bin/sh\n#---\n# description: script\n#---\nprintf 'script\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, found, err := LookupSkillLike(root, "commit", true)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected entry to be found")
	}
	if cfg.Kind != SkillLikeKindSkill {
		t.Fatalf("kind = %q, want %q", cfg.Kind, SkillLikeKindSkill)
	}
	if cfg.Skill == nil || cfg.Skill.Body != "skill body" {
		t.Fatalf("skill = %#v", cfg.Skill)
	}
}

func TestLookupSkillLike_HidesHiddenScriptWhenRequested(t *testing.T) {
	root := t.TempDir()
	scriptDir := filepath.Join(root, ".discboeing", "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "secret"), []byte("#!/bin/sh\n#---\n# visible: false\n#---\nprintf 'secret\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, found, err := LookupSkillLike(root, "secret", true)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected hidden script to be excluded")
	}

	cfg, found, err := LookupSkillLike(root, "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected hidden script to be found when visibleOnly=false")
	}
	if cfg.Kind != SkillLikeKindScript {
		t.Fatalf("kind = %q, want %q", cfg.Kind, SkillLikeKindScript)
	}
	if cfg.Script == nil || cfg.Script.Name != "secret" {
		t.Fatalf("script = %#v", cfg.Script)
	}
}
