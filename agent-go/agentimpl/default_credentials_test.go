package agentimpl

import (
	"context"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agent"
	internalcredentials "github.com/obot-platform/discobot/agent-go/internal/credentials"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestPrompt_ReportsCredentialBindingChangesAndPersistsCommunicatedIDs(t *testing.T) {
	store := thread.NewStore(t.TempDir())
	threadID := "thread-credential-report"
	if err := store.SaveConfig(threadID, thread.Config{Model: "mock/test-model"}); err != nil {
		t.Fatal(err)
	}

	credMgr := internalcredentials.NewManager()
	credMgr.Apply(`[
		{
			"sessionCredentialId":"cred_s_123",
			"envVar":"GH_TOKEN",
			"value":"secret",
			"provider":"github-git",
			"authType":"oauth",
			"agentVisible":true,
			"uses":[{"id":"use_s_456","description":"authenticate gh"}]
		}
	]`, "", "")

	registry := providers.NewProviderRegistry(credMgr)
	mockProvider := &compactCommandMockProvider{responses: []string{"Done."}}
	registry.Add(mockProvider)

	agentImpl := NewDefaultAgent(store, registry, nil, t.TempDir(), MCPConfig{})

	for _, err := range agentImpl.Prompt(context.Background(), threadID, agent.PromptRequest{
		UserParts: []message.UIPart{message.UITextPart{Text: "hello", State: "done"}},
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(mockProvider.requests) == 0 {
		t.Fatal("expected provider request")
	}
	var sawCredentialReminder bool
	for _, req := range mockProvider.requests {
		for _, msg := range req.Messages {
			if msg.Role != "user" {
				continue
			}
			for _, part := range msg.Parts {
				textPart, ok := part.(message.TextPart)
				if !ok {
					continue
				}
				if strings.Contains(textPart.Text, "Credential ID update:") && strings.Contains(textPart.Text, "credentialId=cred_s_123") {
					sawCredentialReminder = true
				}
			}
		}
	}
	if !sawCredentialReminder {
		t.Fatalf("expected credential reminder in provider requests, got %#v", mockProvider.requests)
	}

	cfg, err := store.LoadConfig(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CommunicatedCredentials) != 1 {
		t.Fatalf("expected 1 communicated credential binding, got %#v", cfg.CommunicatedCredentials)
	}
	if cfg.CommunicatedCredentials[0].CredentialID != "cred_s_123" {
		t.Fatalf("unexpected communicated credentials %#v", cfg.CommunicatedCredentials)
	}
	assertCredentialReminderCount(t, store, threadID, 1)

	for _, err := range agentImpl.Prompt(context.Background(), threadID, agent.PromptRequest{
		UserParts: []message.UIPart{message.UITextPart{Text: "hello again", State: "done"}},
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	cfg, err = store.LoadConfig(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CommunicatedCredentials) != 1 || cfg.CommunicatedCredentials[0].CredentialID != "cred_s_123" {
		t.Fatalf("unexpected communicated credentials after second prompt %#v", cfg.CommunicatedCredentials)
	}
	assertCredentialReminderCount(t, store, threadID, 1)
}

func assertCredentialReminderCount(t *testing.T, store *thread.Store, threadID string, want int) {
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
			if ok && meta.ReminderKind == "credentials" && strings.Contains(textPart.Text, "Credential ID update:") {
				got++
			}
		}
	}
	if got != want {
		t.Fatalf("expected %d stored credential reminder messages, got %d", want, got)
	}
}
