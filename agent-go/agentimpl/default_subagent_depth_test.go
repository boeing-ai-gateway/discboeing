package agentimpl

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type toolCaptureProvider struct {
	lastRequest providers.CompleteRequest
}

func (p *toolCaptureProvider) ID() string { return "mock" }

func (p *toolCaptureProvider) Complete(_ context.Context, req providers.CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error] {
	// Skip the background thread-naming request so it doesn't overwrite the
	// main request's tools (both Complete calls race; the naming request has
	// no user parts and doesn't carry tool definitions).
	if !isThreadNameRequest(req) {
		p.lastRequest = req
	}
	return func(yield func(message.ProviderMessageChunk, error) bool) {
		yield(message.StreamStartChunk{}, nil)
		yield(message.TextStartChunk{ID: "t1"}, nil)
		yield(message.TextDeltaChunk{ID: "t1", Delta: "ok"}, nil)
		yield(message.TextEndChunk{ID: "t1"}, nil)
		yield(message.FinishChunk{FinishReason: message.FinishReason{Unified: "stop"}}, nil)
	}
}

func (p *toolCaptureProvider) ListModels(_ context.Context) ([]providers.ModelInfo, error) {
	return nil, nil
}

func (p *toolCaptureProvider) DefaultModels() map[string]providers.ModelRef {
	return map[string]providers.ModelRef{
		providers.ModelTaskChat: {ProviderID: "mock", ModelID: "test-model"},
	}
}

func hasToolNamed(tools []providers.ToolDefinition, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func TestPrompt_SubagentDepthControlsTaskToolExposure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("---\nname: helper\n---\nHelp with nested work."), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		depth    int
		wantTask bool
	}{
		{name: "below limit keeps task", depth: sessionconfig.DefaultMaxSubagentDepth - 1, wantTask: true},
		{name: "at limit strips task", depth: sessionconfig.DefaultMaxSubagentDepth, wantTask: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := thread.NewStore(t.TempDir())
			registry := providers.NewProviderRegistry(nil)
			provider := &toolCaptureProvider{}
			registry.Add(provider)

			agentImpl := NewDefaultAgent(store, registry, nil, root, MCPConfig{})
			for _, err := range agentImpl.Prompt(context.Background(), "thread-depth", agent.PromptRequest{
				SubagentType:  "helper",
				SubagentDepth: tc.depth,
				UserParts:     []message.UIPart{message.UITextPart{Text: "hello"}},
			}) {
				if err != nil {
					t.Fatal(err)
				}
			}

			gotTask := hasToolNamed(provider.lastRequest.Tools, "Task")
			if gotTask != tc.wantTask {
				t.Fatalf("Task tool present = %v, want %v", gotTask, tc.wantTask)
			}
		})
	}
}

func TestPrompt_ModelUsedForSubagentWithoutModel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("---\nname: helper\n---\nHelp with nested work."), 0o644); err != nil {
		t.Fatal(err)
	}

	store := thread.NewStore(t.TempDir())
	registry := providers.NewProviderRegistry(nil)
	provider := &toolCaptureProvider{}
	registry.Add(provider)

	agentImpl := NewDefaultAgent(store, registry, nil, root, MCPConfig{})
	for _, err := range agentImpl.Prompt(context.Background(), "thread-default-model", agent.PromptRequest{
		SubagentType:  "helper",
		UserParts:     []message.UIPart{message.UITextPart{Text: "hello"}},
		Model:         "mock/parent-model",
		SubagentDepth: 1,
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	if provider.lastRequest.Model.ProviderID != "mock" || provider.lastRequest.Model.ModelID != "parent-model" {
		t.Fatalf("model = %#v, want mock/parent-model", provider.lastRequest.Model)
	}
}

func TestPrompt_SubagentModelOverridesRequestModel(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(root, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	agentFile := "---\nname: helper\nmodel: mock/subagent-model\n---\nHelp with nested work."
	if err := os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte(agentFile), 0o644); err != nil {
		t.Fatal(err)
	}

	store := thread.NewStore(t.TempDir())
	registry := providers.NewProviderRegistry(nil)
	provider := &toolCaptureProvider{}
	registry.Add(provider)

	agentImpl := NewDefaultAgent(store, registry, nil, root, MCPConfig{})
	for _, err := range agentImpl.Prompt(context.Background(), "thread-subagent-model", agent.PromptRequest{
		SubagentType:  "helper",
		UserParts:     []message.UIPart{message.UITextPart{Text: "hello"}},
		Model:         "mock/parent-model",
		SubagentDepth: 1,
	}) {
		if err != nil {
			t.Fatal(err)
		}
	}

	if provider.lastRequest.Model.ProviderID != "mock" || provider.lastRequest.Model.ModelID != "subagent-model" {
		t.Fatalf("model = %#v, want mock/subagent-model", provider.lastRequest.Model)
	}
}
