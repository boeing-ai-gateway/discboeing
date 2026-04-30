package sessionconfig

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/providers"
)

func TestDiscoverBuiltinSubAgents_IncludesGeneralPurpose(t *testing.T) {
	agents, err := discoverBuiltinSubAgents()
	if err != nil {
		t.Fatal(err)
	}

	var general *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "general-purpose" {
			general = &agents[i]
			break
		}
	}
	if general == nil {
		t.Fatal("expected built-in general-purpose agent")
	}
	if general.Description == "" {
		t.Error("expected built-in general-purpose description")
	}
	if general.Prompt == "" {
		t.Error("expected built-in general-purpose prompt")
	}
	if len(general.AllowedTools) == 0 {
		t.Error("expected built-in general-purpose allowedTools")
	}
}

func TestFormatSubAgentReminder(t *testing.T) {
	got := FormatSubAgentReminder([]SubAgentConfig{
		{Name: "reviewer", Description: "Reviews code"},
		{Name: "general-purpose"},
	})

	if !strings.Contains(got, "<system-reminder>") || !strings.Contains(got, "</system-reminder>") {
		t.Fatalf("expected system reminder tags, got %q", got)
	}
	if !strings.Contains(got, "- reviewer: Reviews code") {
		t.Fatalf("expected reviewer description, got %q", got)
	}
	if !strings.Contains(got, "- general-purpose") {
		t.Fatalf("expected general-purpose name, got %q", got)
	}
	if !strings.Contains(got, "Use only one of these exact values for Task.subagent_type.") {
		t.Fatalf("expected exact-value instruction, got %q", got)
	}
}

func TestFormatSubAgentReminder_Empty(t *testing.T) {
	if got := FormatSubAgentReminder(nil); got != "" {
		t.Fatalf("expected empty reminder, got %q", got)
	}
	if got := FormatSubAgentReminder([]SubAgentConfig{{Name: " "}}); got != "" {
		t.Fatalf("expected empty reminder for blank names, got %q", got)
	}
}

func TestDiscoverSubAgents_WithFrontmatter(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "reviewer.md"), `---
name: code-reviewer
description: Reviews code for quality
model: gpt-4
supportingModels:
  thread_summarization: openai/gpt-5.4-mini
allowedTools:
  - Read
  - Grep
  - Glob
maxTurns: 5
---
You are a code reviewer. Review the code for quality and correctness.`)

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}

	var a *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "code-reviewer" {
			a = &agents[i]
			break
		}
	}
	if a == nil {
		t.Fatal("expected code-reviewer agent")
	}

	if a.Name != "code-reviewer" {
		t.Errorf("name = %s, want code-reviewer", a.Name)
	}
	if a.Description != "Reviews code for quality" {
		t.Errorf("description = %s", a.Description)
	}
	if a.Model != "gpt-4" {
		t.Errorf("model = %s, want gpt-4", a.Model)
	}
	if got := a.SupportingModels[providers.SupportingModelThreadSummarization]; got != "openai/gpt-5.4-mini" {
		t.Errorf("supportingModels.thread_summarization = %q", got)
	}
	if len(a.AllowedTools) != 3 {
		t.Errorf("allowedTools = %v, want 3 items", a.AllowedTools)
	}
	if a.MaxTurns != 5 {
		t.Errorf("maxTurns = %d, want 5", a.MaxTurns)
	}
	if a.Prompt != "You are a code reviewer. Review the code for quality and correctness." {
		t.Errorf("prompt = %q", a.Prompt)
	}
}

func TestDiscoverSubAgents_WithSupportingModelsKeyValueString(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "reviewer.md"), `---
name: code-reviewer
supportingModels: thread_summarization=openai/gpt-5.4-nano
---
You are a code reviewer.`)

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	var reviewer *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "code-reviewer" {
			reviewer = &agents[i]
			break
		}
	}
	if reviewer == nil {
		t.Fatal("expected code-reviewer agent")
	}
	if got := reviewer.SupportingModels[providers.SupportingModelThreadSummarization]; got != "openai/gpt-5.4-nano" {
		t.Fatalf("supportingModels.thread_summarization = %q", got)
	}
}

func TestDiscoverSubAgents_WithSupportingModelsKeyValueStringList(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "reviewer.md"), `---
name: code-reviewer
supportingModels: thread_summarization=openai/gpt-5.4-nano, custom_helper=anthropic/claude-haiku-4-5-20251001
---
You are a code reviewer.`)

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	var reviewer *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "code-reviewer" {
			reviewer = &agents[i]
			break
		}
	}
	if reviewer == nil {
		t.Fatal("expected code-reviewer agent")
	}
	if got := reviewer.SupportingModels[providers.SupportingModelThreadSummarization]; got != "openai/gpt-5.4-nano" {
		t.Fatalf("thread_summarization = %q", got)
	}
	if got := reviewer.SupportingModels[providers.SupportingModelType("custom_helper")]; got != "anthropic/claude-haiku-4-5-20251001" {
		t.Fatalf("custom_helper = %q", got)
	}
}

func TestDiscoverSubAgents_InvalidSupportingModelsKeyValueString(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "reviewer.md"), `---
name: code-reviewer
supportingModels: thread_summarization
---
You are a code reviewer.`)

	_, err := discoverSubAgents(root)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestDiscoverSubAgents_NoFrontmatter(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "simple.md"), "Just a simple agent prompt.")

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}

	var simple *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "simple" {
			simple = &agents[i]
			break
		}
	}
	if simple == nil {
		t.Fatal("expected simple agent")
	}

	if simple.Name != "simple" {
		t.Errorf("name = %s, want simple (from filename)", simple.Name)
	}
	if simple.Prompt != "Just a simple agent prompt." {
		t.Errorf("prompt = %q", simple.Prompt)
	}
}

func TestDiscoverSubAgents_MultipleAgents(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "alpha.md"), "---\nname: alpha\n---\nAlpha agent.")
	writeFile(t, filepath.Join(agentsDir, "beta.md"), "---\nname: beta\n---\nBeta agent.")

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}

	// Project agents should be sorted by filename and appear before built-ins.
	if agents[0].Name != "alpha" {
		t.Errorf("first agent = %s, want alpha", agents[0].Name)
	}
	if agents[1].Name != "beta" {
		t.Errorf("second agent = %s, want beta", agents[1].Name)
	}
	if agents[2].Name != "general-purpose" {
		t.Errorf("third agent = %s, want general-purpose", agents[2].Name)
	}
}

func TestDiscoverSubAgents_MissingDir(t *testing.T) {
	root := t.TempDir()
	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) == 0 {
		t.Fatal("expected built-in agents even when project dir is missing")
	}
	var found bool
	for _, agent := range agents {
		if agent.Name == "general-purpose" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected built-in general-purpose agent")
	}
}

func TestDiscoverSubAgents_SkipsNonMarkdown(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "agent.md"), "---\nname: real\n---\nReal agent.")
	writeFile(t, filepath.Join(agentsDir, "notes.txt"), "Not an agent.")
	mkdirAll(t, filepath.Join(agentsDir, "subdir"))

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	var realAgent *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "real" {
			realAgent = &agents[i]
			break
		}
	}
	if realAgent == nil {
		t.Fatal("expected real agent")
	}
	if realAgent.Name != "real" {
		t.Errorf("name = %s, want real", realAgent.Name)
	}
}

func TestDiscoverSubAgents_ProjectOverridesBuiltin(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "general-purpose.md"), `---
name: general-purpose
description: Project override
---
Project prompt.`)

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	for _, agent := range agents {
		if agent.Name == "general-purpose" {
			count++
			if agent.Description != "Project override" {
				t.Fatalf("description = %q, want project override", agent.Description)
			}
			if agent.Prompt != "Project prompt." {
				t.Fatalf("prompt = %q, want project prompt", agent.Prompt)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 general-purpose agent, got %d", count)
	}
}

func TestDiscoverSubAgents_DisallowedTools(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, ".claude", "agents")
	mkdirAll(t, agentsDir)

	writeFile(t, filepath.Join(agentsDir, "safe.md"), `---
name: safe-agent
disallowedTools:
  - Bash
  - Write
---
I cannot run commands or write files.`)

	agents, err := discoverSubAgents(root)
	if err != nil {
		t.Fatal(err)
	}
	var safe *SubAgentConfig
	for i := range agents {
		if agents[i].Name == "safe-agent" {
			safe = &agents[i]
			break
		}
	}
	if safe == nil {
		t.Fatal("expected safe-agent")
	}
	if len(safe.DisallowedTools) != 2 {
		t.Errorf("disallowedTools = %v, want 2 items", safe.DisallowedTools)
	}
}
