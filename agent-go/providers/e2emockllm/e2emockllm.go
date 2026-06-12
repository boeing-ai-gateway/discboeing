//go:build e2e_mock_llm

// Package e2emockllm provides a build-tagged deterministic provider for e2e tests.
package e2emockllm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/obot-platform/discobot/agent-go/llm-responses"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
)

const (
	ProviderID        = "e2e-mock-llm"
	defaultModelID    = "mock"
	fixtureDirEnv     = "DISCOBOT_E2E_MOCK_LLM_RESPONSES_DIR"
	defaultTextID     = "mock-text-0"
	defaultToolCallID = "mock-tool-0"
)

func init() {
	providers.Register(ProviderID, New)
}

type Provider struct {
	fixtures *FixtureSet
}

func New(_ providers.Config) (providers.Provider, error) {
	fixtures, err := LoadFixturesFromEnv()
	if err != nil {
		return nil, err
	}
	return &Provider{fixtures: fixtures}, nil
}

func (p *Provider) ID() string { return ProviderID }

func (p *Provider) DefaultModels() map[string]providers.ModelRef {
	ref := providers.ModelRef{ProviderID: ProviderID, ModelID: defaultModelID}
	return map[string]providers.ModelRef{
		providers.ModelTaskChat:                ref,
		providers.ModelTaskAuthorization:       ref,
		providers.ModelTaskThreadSummarization: ref,
	}
}

func (p *Provider) Complete(ctx context.Context, req providers.CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error] {
	return func(yield func(message.ProviderMessageChunk, error) bool) {
		input := RequestInput(req)
		resp, name, ok, err := p.fixtures.Match(input)
		if err != nil {
			yield(nil, err)
			return
		}
		if !ok {
			yield(nil, fmt.Errorf("%s: no fixture matched input %q", ProviderID, input))
			return
		}
		if resp.Error != "" {
			yield(nil, fmt.Errorf("%s fixture %q: %s", ProviderID, name, resp.Error))
			return
		}
		emitResponse(ctx, req.Model.String(), resp, yield)
	}
}

func emitResponse(ctx context.Context, modelID string, resp Response, yield func(message.ProviderMessageChunk, error) bool) {
	if ctx.Err() != nil {
		yield(nil, ctx.Err())
		return
	}
	if !yield(message.StreamStartChunk{}, nil) {
		return
	}
	now := time.Now().UTC()
	if !yield(message.ResponseMetadataChunk{ID: "e2e-mock-llm-response", Timestamp: &now, ModelID: modelID}, nil) {
		return
	}
	if resp.Text != "" {
		id := resp.TextID
		if id == "" {
			id = defaultTextID
		}
		if !yield(message.TextStartChunk{ID: id}, nil) {
			return
		}
		if !yield(message.TextDeltaChunk{ID: id, Delta: resp.Text}, nil) {
			return
		}
		if !yield(message.TextEndChunk{ID: id}, nil) {
			return
		}
	}
	for i, tc := range resp.ToolCalls {
		id := tc.ID
		if id == "" {
			id = fmt.Sprintf("%s-%d", defaultToolCallID, i)
		}
		if !yield(message.ToolCallChunk{ToolCallID: id, ToolName: tc.Name, Input: tc.Input}, nil) {
			return
		}
	}
	finish := "stop"
	if len(resp.ToolCalls) > 0 {
		finish = "tool-calls"
	}
	if resp.FinishReason != "" {
		finish = resp.FinishReason
	}
	yield(message.FinishChunk{FinishReason: message.FinishReason{Unified: finish, Raw: finish}}, nil)
}

type FixtureSet struct {
	Responses []Entry   `json:"responses"`
	Fallback  *Response `json:"fallback,omitempty"`
}

type Entry struct {
	Name     string   `json:"name,omitempty"`
	Match    Match    `json:"match"`
	Response Response `json:"response"`
}

type Match struct {
	Exact    string `json:"exact,omitempty"`
	Contains string `json:"contains,omitempty"`
	Regex    string `json:"regex,omitempty"`
}

type Response struct {
	Text         string     `json:"text,omitempty"`
	TextID       string     `json:"textId,omitempty"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
	FinishReason string     `json:"finishReason,omitempty"`
	Error        string     `json:"error,omitempty"`
}

type ToolCall struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Input string `json:"input"`
}

func LoadFixturesFromEnv() (*FixtureSet, error) {
	if dir := os.Getenv(fixtureDirEnv); dir != "" {
		return LoadFixturesFromDir(dir)
	}
	return LoadFixturesFS(llmresponses.FS, ".")
}

func LoadFixturesFromDir(dir string) (*FixtureSet, error) {
	return loadFixtureFiles(os.DirFS(dir), ".")
}

func LoadFixturesFS(fsys fs.FS, dir string) (*FixtureSet, error) {
	return loadFixtureFiles(fsys, dir)
}

func loadFixtureFiles(fsys fs.FS, dir string) (*FixtureSet, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("%s: read fixtures: %w", ProviderID, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	set := &FixtureSet{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.ToSlash(filepath.Join(dir, entry.Name()))
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("%s: read fixture %s: %w", ProviderID, path, err)
		}
		var file FixtureSet
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("%s: parse fixture %s: %w", ProviderID, path, err)
		}
		set.Responses = append(set.Responses, file.Responses...)
		if file.Fallback != nil {
			set.Fallback = file.Fallback
		}
	}
	if len(set.Responses) == 0 && set.Fallback == nil {
		return nil, fmt.Errorf("%s: no fixtures found", ProviderID)
	}
	return set, nil
}

func (s *FixtureSet) Match(input string) (Response, string, bool, error) {
	if s == nil {
		return Response{}, "", false, fmt.Errorf("%s: fixture set is nil", ProviderID)
	}
	for _, entry := range s.Responses {
		ok, err := entry.Match.matches(input)
		if err != nil {
			return Response{}, entry.Name, false, err
		}
		if ok {
			return entry.Response, entry.Name, true, nil
		}
	}
	if s.Fallback != nil {
		return *s.Fallback, "fallback", true, nil
	}
	return Response{}, "", false, nil
}

func (m Match) matches(input string) (bool, error) {
	if m.Exact != "" && input == m.Exact {
		return true, nil
	}
	if m.Contains != "" && strings.Contains(input, m.Contains) {
		return true, nil
	}
	if m.Regex != "" {
		ok, err := regexp.MatchString(m.Regex, input)
		if err != nil {
			return false, fmt.Errorf("%s: invalid fixture regex %q: %w", ProviderID, m.Regex, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// RequestInput returns the deterministic input string used for fixture matching.
// It prefers the latest non-synthetic user text, falling back to all visible text.
func RequestInput(req providers.CompleteRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if msg.Role == "user" && !msg.Synthetic {
			if text := messageText(msg); text != "" {
				return text
			}
		}
	}
	var parts []string
	for _, msg := range req.Messages {
		if msg.Synthetic {
			continue
		}
		if text := messageText(msg); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func messageText(msg message.Message) string {
	var parts []string
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextPart:
			parts = append(parts, p.Text)
		case message.ToolResultPart:
			parts = append(parts, fmt.Sprintf("%s result", p.ToolName))
		}
	}
	return strings.Join(parts, "\n")
}
