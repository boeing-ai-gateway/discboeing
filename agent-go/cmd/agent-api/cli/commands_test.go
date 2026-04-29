package cli

import (
	"context"
	"encoding/base64"
	"flag"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/clisession"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
)

type testSession struct {
	commands []agent.Command
	threads  map[string]api.Thread
}

func (s *testSession) WorkspaceRoot() string { return "" }
func (s *testSession) Close()                {}
func (s *testSession) ListCommands(context.Context) ([]agent.Command, error) {
	return s.commands, nil
}
func (s *testSession) ListThreads(context.Context) ([]api.Thread, error) {
	var out []api.Thread
	for _, thread := range s.threads {
		out = append(out, thread)
	}
	return out, nil
}
func (s *testSession) GetThread(_ context.Context, id string) (api.Thread, error) {
	thread, ok := s.threads[id]
	if !ok {
		return api.Thread{}, clisession.ErrNotFound
	}
	return thread, nil
}
func (s *testSession) UpdateThread(_ context.Context, id string, req api.UpdateThreadRequest) (api.Thread, error) {
	thread := s.threads[id]
	if req.Name != "" {
		thread.Name = req.Name
	}
	if req.CWD != "" {
		thread.CWD = req.CWD
	}
	s.threads[id] = thread
	return thread, nil
}
func (s *testSession) Messages(context.Context, string) ([]message.UIMessage, error) { return nil, nil }
func (s *testSession) HasInterruptedTurn(context.Context, string) (bool, error)      { return false, nil }
func (s *testSession) PendingQuestion(context.Context, string) (*agent.PendingQuestion, error) {
	return nil, nil
}
func (s *testSession) SubmitAnswer(context.Context, string, string, api.AnswerQuestionRequest) error {
	return nil
}
func (s *testSession) Prompt(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	return nil, nil
}
func (s *testSession) Resume(context.Context, string, agent.PromptRequest) (iter.Seq2[message.MessageChunk, error], error) {
	return nil, nil
}

type testModelListProvider struct {
	id string
}

func (p *testModelListProvider) ID() string { return p.id }

func (p *testModelListProvider) Complete(_ context.Context, _ providers.CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error] {
	return func(func(message.ProviderMessageChunk, error) bool) {}
}

func (p *testModelListProvider) DefaultModels() map[string]providers.ModelRef { return nil }

func TestAddFlags_ShortAliases(t *testing.T) {
	oldCommandLine := flag.CommandLine
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flag.CommandLine = fs
	defer func() {
		flag.CommandLine = oldCommandLine
	}()

	flags := AddFlags()
	if err := flag.CommandLine.Parse([]string{"-m", "anthropic/claude-sonnet-4", "-p", "-r", "thread-123"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got := *flags.model; got != "anthropic/claude-sonnet-4" {
		t.Fatalf("model = %q, want %q", got, "anthropic/claude-sonnet-4")
	}
	if !*flags.plan {
		t.Fatal("expected -p alias to enable plan mode")
	}
	if got := *flags.resume; got != "thread-123" {
		t.Fatalf("resume = %q, want %q", got, "thread-123")
	}
}

func TestAgentSlashCommands_LoadsSkillsAndCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	skillDir := filepath.Join(root, ".claude", "skills", "deploy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: deploy\ndescription: Deploy app\n---\nDeploy now."), 0o644); err != nil {
		t.Fatal(err)
	}

	commandsDir := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "release.md"), []byte("Release command body."), 0o644); err != nil {
		t.Fatal(err)
	}

	commands, err := agentSlashCommands(root)
	if err != nil {
		t.Fatalf("agentSlashCommands() error = %v", err)
	}
	if _, ok := commands["/deploy"]; !ok {
		t.Fatalf("expected /deploy in discovered commands")
	}
	if _, ok := commands["/release"]; !ok {
		t.Fatalf("expected /release in discovered commands")
	}
}

func TestHandleSlashCommand_ForwardsKnownAgentCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	skillDir := filepath.Join(root, ".claude", "skills", "deploy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Deploy skill body."), 0o644); err != nil {
		t.Fatal(err)
	}

	session := &testSession{commands: []agent.Command{{Name: "deploy"}}}
	threadID, handled := handleSlashCommand(context.Background(), "/deploy", session, "thread-1", nil, nil, nil, nil)
	if handled {
		t.Fatalf("expected /deploy to be forwarded to agent, handled=%v", handled)
	}
	if threadID != "thread-1" {
		t.Fatalf("threadID changed unexpectedly: %q", threadID)
	}
}

func TestHandleSlashCommand_UnknownStillHandledLocally(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	session := &testSession{}
	threadID, handled := handleSlashCommand(context.Background(), "/does-not-exist", session, "thread-1", nil, nil, nil, nil)
	if !handled {
		t.Fatalf("expected unknown slash command to be handled locally")
	}
	if threadID != "thread-1" {
		t.Fatalf("threadID changed unexpectedly: %q", threadID)
	}
}

func TestHandleSlashCommand_PlanDoesNotCreateThreadBeforePrompt(t *testing.T) {
	threadsDir := t.TempDir()
	session := &testSession{}
	planMode := false
	threadID := "thread-lazy"

	newThreadID, handled := handleSlashCommand(context.Background(), "/plan", session, threadID, nil, nil, &planMode, nil)
	if !handled {
		t.Fatalf("expected /plan to be handled locally")
	}
	if newThreadID != threadID {
		t.Fatalf("expected thread id to remain %q, got %q", threadID, newThreadID)
	}
	if !planMode {
		t.Fatalf("expected /plan to toggle plan mode on")
	}
	if _, err := os.Stat(filepath.Join(threadsDir, threadID)); !os.IsNotExist(err) {
		t.Fatalf("expected no thread directory before first prompt, stat err=%v", err)
	}
}

func TestHandleSlashCommand_LocalCommandsTakePriority(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	skillDir := filepath.Join(root, ".claude", "skills", "clear")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Clear skill body."), 0o644); err != nil {
		t.Fatal(err)
	}

	pendingFresh := map[string]bool{}
	session := &testSession{}
	newThreadID, handled := handleSlashCommand(context.Background(), "/clear", session, "thread-1", nil, nil, nil, pendingFresh)
	if !handled {
		t.Fatalf("expected /clear to be handled locally")
	}
	if newThreadID != "thread-1" {
		t.Fatalf("expected /clear to keep current thread, got %q", newThreadID)
	}
	if !pendingFresh["thread-1"] {
		t.Fatalf("expected /clear to mark the current thread for fresh context")
	}
}

func TestHandleModelsCommand_SortsModelList(t *testing.T) {
	reg := providers.NewProviderRegistry(nil)
	reg.Add(&testModelListProvider{
		id: "openai",
	})
	reg.Add(&testModelListProvider{
		id: "anthropic",
	})

	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinReader.Close()
	defer stderrReader.Close()

	if _, err := stdinWriter.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	if err := stdinWriter.Close(); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	oldStderr := os.Stderr
	oldNoColor := noColor
	os.Stdin = stdinReader
	os.Stderr = stderrWriter
	noColor = true
	defer func() {
		os.Stdin = oldStdin
		os.Stderr = oldStderr
		noColor = oldNoColor
	}()

	currentModel := ""
	handleModelsCommand(context.Background(), reg, &currentModel)

	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}

	lines := []string{
		"anthropic/claude-opus-4-20250514 — Claude Opus 4",
		"anthropic/claude-sonnet-4-20250514 — Claude Sonnet 4",
		"openai/gpt-4.1 — GPT-4.1",
		"openai/gpt-4o — GPT-4o",
	}
	lastIndex := -1
	for _, line := range lines {
		idx := strings.Index(string(output), line)
		if idx == -1 {
			t.Fatalf("expected output to contain %q, got:\n%s", line, string(output))
		}
		if idx <= lastIndex {
			t.Fatalf("expected output line %q to appear after previous model, got:\n%s", line, string(output))
		}
		lastIndex = idx
	}
}

func TestImagePartFromPathInput_DetectsImageFile(t *testing.T) {
	root := t.TempDir()
	pngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	if err := os.WriteFile(filepath.Join(root, "img.png"), pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	part, ok, err := imagePartFromPathInput([]byte("img.png"), root)
	if err != nil {
		t.Fatalf("imagePartFromPathInput() error = %v", err)
	}
	if !ok {
		t.Fatal("expected image path to be detected")
	}
	if !strings.HasPrefix(part.MediaType, "image/") {
		t.Fatalf("expected image media type, got %q", part.MediaType)
	}
	decoded, err := base64.StdEncoding.DecodeString(part.URL)
	if err != nil {
		t.Fatalf("decode base64 image: %v", err)
	}
	if string(decoded) != string(pngBytes) {
		t.Fatalf("decoded image bytes mismatch")
	}
}

func TestImagePartFromRawBytes_DetectsImageData(t *testing.T) {
	input := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0, '\n'}
	part, ok := imagePartFromRawBytes(input)
	if !ok {
		t.Fatal("expected raw image bytes to be detected")
	}
	if !strings.HasPrefix(part.MediaType, "image/") {
		t.Fatalf("expected image media type, got %q", part.MediaType)
	}
	decoded, err := base64.StdEncoding.DecodeString(part.URL)
	if err != nil {
		t.Fatalf("decode base64 image: %v", err)
	}
	trimmed := input[:len(input)-1]
	if string(decoded) != string(trimmed) {
		t.Fatalf("decoded image bytes mismatch")
	}
}
