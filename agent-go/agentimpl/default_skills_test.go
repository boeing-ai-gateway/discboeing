package agentimpl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
	"github.com/boeing-ai-gateway/discboeing/agent-go/sessionconfig"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestPrompt_ReportsSkillLikeReminderChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSkill(t, root, "test-commit-skill", "Commit pending changes")
	loadedCfg, err := sessionconfig.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedCfg.Skills) == 0 {
		t.Fatalf("expected test skill to be discovered from %s", root)
	}

	store := thread.NewStore(t.TempDir())
	registry := providers.NewProviderRegistry(nil)
	mockProvider := &compactCommandMockProvider{responses: []string{"Done.", "Done again.", "Still done.", "Finished."}}
	registry.Add(mockProvider)
	agentImpl := NewDefaultAgent(store, registry, nil, root, MCPConfig{})
	threadID := "thread-skill-refresh"

	tools := sessionconfig.BuiltinTools("")
	if !hasNamedTool(tools, "Skill") {
		t.Fatal("expected test tools to include Skill")
	}
	initialEntries := currentVisibleSkillLikeEntries(loadedCfg, tools)
	if !skillLikeEntriesContain(initialEntries, "test-commit-skill", "Commit pending changes") {
		t.Fatalf("expected initial project skill entry, got %#v", initialEntries)
	}

	if err := store.SaveMessage(threadID, thread.StoredMessage{
		ID: "root",
		Message: message.Message{
			Role:  "user",
			Parts: []message.Part{message.TextPart{Text: "initial thread"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfig(threadID, thread.Config{
		Name:                         "Existing thread",
		NameSource:                   thread.ThreadNameSourceUser,
		Model:                        "mock/test-model",
		CWD:                          root,
		ActiveLeafID:                 "root",
		CommunicatedSkillLikeEntries: initialEntries,
	}); err != nil {
		t.Fatal(err)
	}
	assertDynamicSkillsReminderCount(t, store, threadID, 0)

	writeTestSkill(t, root, "test-lobstercash", "Manage Lobster Cash registration")
	promptSkillTest(t, agentImpl, threadID, tools, "what skills are available?")

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CommunicatedSkillLikeEntries) != len(initialEntries)+1 {
		t.Fatalf("expected updated skill entries to be persisted, got %#v", cfg.CommunicatedSkillLikeEntries)
	}
	assertDynamicSkillsReminderCount(t, store, threadID, 1)
	if !latestProviderRequestContainsAll(mockProvider, "Added:", "test-lobstercash") {
		t.Fatalf("expected latest provider request to mention added skill, got %#v", mockProvider.requests)
	}

	writeTestSkill(t, root, "test-lobstercash", "Use Lobster Cash wallets")
	promptSkillTest(t, agentImpl, threadID, tools, "anything changed?")
	assertDynamicSkillsReminderCount(t, store, threadID, 2)
	if !latestProviderRequestContainsAll(mockProvider, "Changed descriptions:", "test-lobstercash", "Use Lobster Cash wallets") {
		t.Fatalf("expected latest provider request to mention changed skill description, got %#v", mockProvider.requests)
	}

	if err := os.RemoveAll(filepath.Join(root, ".claude", "skills", "test-commit-skill")); err != nil {
		t.Fatal(err)
	}
	promptSkillTest(t, agentImpl, threadID, tools, "and now?")
	assertDynamicSkillsReminderCount(t, store, threadID, 3)
	if !latestProviderRequestContainsAll(mockProvider, "Removed:", "test-commit-skill") {
		t.Fatalf("expected latest provider request to mention removed skill, got %#v", mockProvider.requests)
	}

	promptSkillTest(t, agentImpl, threadID, tools, "again")
	assertDynamicSkillsReminderCount(t, store, threadID, 3)
}

func skillLikeEntriesContain(entries []thread.CommunicatedSkillLikeEntry, name, description string) bool {
	for _, entry := range entries {
		if entry.Name == name && entry.Description == description {
			return true
		}
	}
	return false
}

func promptSkillTest(t *testing.T, agentImpl *DefaultAgent, threadID string, tools []providers.ToolDefinition, text string) {
	t.Helper()
	for _, err := range agentImpl.Prompt(context.Background(), threadID, agent.PromptRequest{
		UserParts: []message.UIPart{message.UITextPart{Text: text, State: "done"}},
		Tools:     tools,
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestSkill(t *testing.T, root, name, description string) {
	t.Helper()
	dir := filepath.Join(root, ".claude", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: " + description + "\n---\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertDynamicSkillsReminderCount(t *testing.T, store *thread.Store, threadID string, want int) {
	t.Helper()

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		t.Fatal(err)
	}
	history, err := store.BuildHistory(threadID, cfg.ActiveLeafID)
	if err != nil {
		t.Fatal(err)
	}
	got := 0
	for _, msg := range history {
		for _, part := range msg.Parts {
			textPart, ok := part.(message.TextPart)
			if !ok {
				continue
			}
			meta, ok := message.UnmarshalProviderMetadata(textPart.ProviderMetadata)
			if ok && meta.ReminderKind == "skills" && strings.Contains(textPart.Text, "Available skills for the Skill tool changed") {
				got++
			}
		}
	}
	if got != want {
		t.Fatalf("expected %d stored dynamic skills reminders, got %d", want, got)
	}
}

func latestProviderRequestContainsAll(provider *compactCommandMockProvider, texts ...string) bool {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.requests) == 0 {
		return false
	}
	latest := provider.requests[len(provider.requests)-1]
	for _, want := range texts {
		found := false
		for _, msg := range latest.Messages {
			for _, part := range msg.Parts {
				textPart, ok := part.(message.TextPart)
				if ok && strings.Contains(textPart.Text, want) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
