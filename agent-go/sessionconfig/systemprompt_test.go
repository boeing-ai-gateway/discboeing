package sessionconfig

import (
	"strings"
	"testing"
)

func TestFormatUserInstructions_Empty(t *testing.T) {
	result := FormatUserInstructions(nil)
	if result != "" {
		t.Errorf("expected empty string for nil entries, got: %s", result)
	}

	result = FormatUserInstructions([]InstructionEntry{})
	if result != "" {
		t.Errorf("expected empty string for empty entries, got: %s", result)
	}
}

func TestFormatUserInstructions_SingleEntry(t *testing.T) {
	entries := []InstructionEntry{
		{
			Path:        "CLAUDE.md",
			Description: "project instructions, checked into the codebase",
			Content:     "Always use gofmt.",
		},
	}

	result := FormatUserInstructions(entries)

	if !strings.HasPrefix(result, "<system-reminder>") {
		t.Error("should start with <system-reminder>")
	}
	if !strings.HasSuffix(result, "</system-reminder>") {
		t.Error("should end with </system-reminder>")
	}
	if !strings.Contains(result, "OVERRIDE any default behavior") {
		t.Error("missing override instruction")
	}
	if !strings.Contains(result, "Contents of CLAUDE.md (project instructions, checked into the codebase):") {
		t.Error("missing file header")
	}
	if !strings.Contains(result, "Always use gofmt.") {
		t.Error("missing content")
	}
}

func TestFormatUserInstructions_MultipleEntries(t *testing.T) {
	entries := []InstructionEntry{
		{
			Path:        "CLAUDE.md",
			Description: "project instructions, checked into the codebase",
			Content:     "Project rules.",
		},
		{
			Path:        "~/.claude/CLAUDE.md",
			Description: "user-level instructions",
			Content:     "User preferences.",
		},
		{
			Path:        ".discboeing/rules/style.md",
			Description: "project rule",
			Content:     "Use tabs.",
		},
	}

	result := FormatUserInstructions(entries)

	if !strings.Contains(result, "Contents of CLAUDE.md (project instructions, checked into the codebase):") {
		t.Error("missing CLAUDE.md header")
	}
	if !strings.Contains(result, "Contents of ~/.claude/CLAUDE.md (user-level instructions):") {
		t.Error("missing user-level header")
	}
	if !strings.Contains(result, "Contents of .discboeing/rules/style.md (project rule):") {
		t.Error("missing rule header")
	}
	if !strings.Contains(result, "Project rules.") {
		t.Error("missing project content")
	}
	if !strings.Contains(result, "User preferences.") {
		t.Error("missing user content")
	}
	if !strings.Contains(result, "Use tabs.") {
		t.Error("missing rule content")
	}

	idx1 := strings.Index(result, "Project rules.")
	idx2 := strings.Index(result, "User preferences.")
	idx3 := strings.Index(result, "Use tabs.")
	if idx1 >= idx2 || idx2 >= idx3 {
		t.Error("entries should be in order")
	}
}

func TestDefaultSystemPrompt_Content(t *testing.T) {
	prompt := defaultSystemPrompt()

	for _, want := range []string{
		"You are Discboeing’s coding agent.",
		"## Runtime rules",
		"## Working rules",
		"## Tool use",
		"## Communication style",
		"/discboeing/docs.txt",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestDefaultSystemConfig_AllowedTools(t *testing.T) {
	cfg, err := defaultSystemConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllowedTools) == 0 {
		t.Fatal("expected allowedTools in embedded SYSTEM.md")
	}
	if cfg.AllowedTools[0] != "Bash" {
		t.Errorf("first allowed tool = %q, want Bash", cfg.AllowedTools[0])
	}
	if !strings.Contains(cfg.PromptBody, "You are Discboeing’s coding agent.") {
		t.Error("missing prompt body")
	}
}
