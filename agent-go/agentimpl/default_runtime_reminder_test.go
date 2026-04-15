package agentimpl

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func TestFormatRuntimeEnvironmentReminder_IncludesSnapshotDetails(t *testing.T) {
	cwd := t.TempDir()
	got := formatRuntimeEnvironmentReminder(cwd, "")

	if !strings.Contains(got, "<system-reminder>") {
		t.Error("missing <system-reminder> start tag")
	}
	if !strings.Contains(got, "</system-reminder>") {
		t.Error("missing </system-reminder> end tag")
	}
	if !strings.Contains(got, "Current working directory:") {
		t.Error("missing current working directory line")
	}
	if !strings.Contains(got, "OS/platform:") {
		t.Error("missing OS/platform line")
	}
	if !strings.Contains(got, "Current date/time:") {
		t.Error("missing current date/time line")
	}
	if !strings.Contains(got, "Git state (captured at the current time of this reminder; this may change throughout the conversation):") {
		t.Error("missing git state snapshot disclaimer")
	}
	if strings.Contains(got, "Current model:") {
		t.Error("expected no model line when modelName is empty")
	}
}

func TestFormatRuntimeEnvironmentReminder_IncludesModelName(t *testing.T) {
	cwd := t.TempDir()
	got := formatRuntimeEnvironmentReminder(cwd, "Claude Sonnet 4")

	if !strings.Contains(got, "- Current model: Claude Sonnet 4") {
		t.Errorf("expected model line, got %q", got)
	}
}

func TestFormatModeChangeReminder_IncludesTargetMode(t *testing.T) {
	plan := formatModeChangeReminder(true)
	if !strings.Contains(plan, "<system-reminder>") || !strings.Contains(plan, "</system-reminder>") {
		t.Error("plan reminder should be wrapped in <system-reminder> tags")
	}
	if !strings.Contains(plan, "mode is now plan") {
		t.Errorf("expected plan reminder to mention plan mode, got %q", plan)
	}

	build := formatModeChangeReminder(false)
	if !strings.Contains(build, "mode is now build") {
		t.Errorf("expected build reminder to mention build mode, got %q", build)
	}
	if !strings.Contains(build, "Plan mode has been exited") {
		t.Errorf("expected build reminder to mention exiting plan mode, got %q", build)
	}
}

func TestResolvePlanMode_OnlyChangesOnExplicitRequest(t *testing.T) {
	cfgPlan := thread.Config{Mode: thread.ModeState{Value: "plan"}}
	cfgBuild := thread.Config{Mode: thread.ModeState{Value: "build"}}

	planMode, changed := resolvePlanMode("", cfgPlan, true)
	if !planMode || changed {
		t.Fatalf("empty mode should keep current mode=true with changed=false, got planMode=%v changed=%v", planMode, changed)
	}

	planMode, changed = resolvePlanMode("plan", cfgBuild, true)
	if !planMode || !changed {
		t.Fatalf("plan request from build should change to plan, got planMode=%v changed=%v", planMode, changed)
	}

	planMode, changed = resolvePlanMode("plan", cfgPlan, true)
	if !planMode || changed {
		t.Fatalf("plan request from plan should not change, got planMode=%v changed=%v", planMode, changed)
	}

	planMode, changed = resolvePlanMode("", cfgBuild, false)
	if planMode || changed {
		t.Fatalf("missing config and empty mode should default to build with changed=false, got planMode=%v changed=%v", planMode, changed)
	}

	planMode, changed = resolvePlanMode("plan", cfgBuild, false)
	if !planMode || !changed {
		t.Fatalf("explicit plan without prior config should set plan and mark changed from default build, got planMode=%v changed=%v", planMode, changed)
	}

	planMode, changed = resolvePlanMode("build", cfgPlan, true)
	if planMode || !changed {
		t.Fatalf("explicit build from plan should change to build, got planMode=%v changed=%v", planMode, changed)
	}
}

func TestGeneratedThreadName_UsesFirstMeaningfulText(t *testing.T) {
	got := generatedThreadName([]message.UIPart{
		message.UIFilePart{Type: "file", URL: "file:///tmp/input.txt", MediaType: "text/plain"},
		message.UITextPart{Text: "  Fix the failing CI build for agent-go  ", State: "done"},
	})
	if got != "Fix the failing CI build for agent-go" {
		t.Fatalf("generatedThreadName() = %q", got)
	}
}

func TestGeneratedThreadName_StripsLeadingSlashCommand(t *testing.T) {
	got := generatedThreadName([]message.UIPart{
		message.UITextPart{Text: "/commit fix thread naming in agent-go", State: "done"},
	})
	if got != "fix thread naming in agent-go" {
		t.Fatalf("generatedThreadName() = %q", got)
	}
}

func TestGeneratedThreadName_TruncatesLongText(t *testing.T) {
	got := generatedThreadName([]message.UIPart{
		message.UITextPart{Text: strings.Repeat("a", generatedThreadNameMaxRunes+10), State: "done"},
	})
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) != generatedThreadNameMaxRunes {
		t.Fatalf("expected %d runes, got %d", generatedThreadNameMaxRunes, len([]rune(got)))
	}
}

func TestShouldGenerateThreadName_OnlyUsesUnsetNames(t *testing.T) {
	if !shouldGenerateThreadName(thread.Config{}) {
		t.Fatal("expected empty config to be eligible")
	}
	if shouldGenerateThreadName(thread.Config{Name: "Generated", NameSource: thread.ThreadNameSourceGenerated}) {
		t.Fatal("did not expect existing generated name to remain eligible")
	}
	if shouldGenerateThreadName(thread.Config{Name: "Custom", NameSource: thread.ThreadNameSourceUser}) {
		t.Fatal("did not expect non-empty name to be eligible")
	}
}

func TestGitStateSnapshot_NotGitRepo(t *testing.T) {
	got := gitStateSnapshot(t.TempDir())
	if got != "not a git repository" {
		t.Errorf("gitStateSnapshot() = %q, want %q", got, "not a git repository")
	}
}

func TestGitStateSnapshot_CleanAndDirty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	repo := t.TempDir()
	runGit(t, repo, "init", "-q")

	clean := gitStateSnapshot(repo)
	if !strings.Contains(clean, "branch=") {
		t.Errorf("expected branch in clean snapshot, got %q", clean)
	}
	if !strings.Contains(clean, "working_tree=clean") {
		t.Errorf("expected clean working tree, got %q", clean)
	}

	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	dirty := gitStateSnapshot(repo)
	if !strings.Contains(dirty, "working_tree=dirty") {
		t.Errorf("expected dirty working tree, got %q", dirty)
	}
}

func TestBootstrapNewThreadMessages_IncludesRecentThreadsReminder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := thread.NewStore(t.TempDir())
	agent := NewDefaultAgent(store, nil, nil, t.TempDir(), MCPConfig{})

	for i := range 6 {
		threadID := "thread-" + strconv.Itoa(i)
		if err := store.CreateThread(threadID); err != nil {
			t.Fatal(err)
		}

		if i == 1 {
			if err := store.SaveConfig(threadID, thread.Config{Name: "Named thread subject"}); err != nil {
				t.Fatal(err)
			}
		} else if err := store.SaveConfig(threadID, thread.Config{}); err != nil {
			t.Fatal(err)
		}

		mtime := time.Now().Add(time.Duration(i) * time.Minute)
		if err := filepath.Walk(store.ThreadDir(threadID), func(path string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Chtimes(path, mtime, mtime)
		}); err != nil {
			t.Fatal(err)
		}
	}

	leafID, err := agent.bootstrapNewThreadMessages(
		"current-thread",
		"system prompt",
		"Claude Sonnet 4",
		&sessionconfig.SessionConfig{MaxSubagentDepth: sessionconfig.DefaultMaxSubagentDepth},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	history, err := store.BuildHistory("current-thread", leafID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) < 3 {
		t.Fatalf("expected bootstrap history to include runtime and recent thread reminders, got %d messages", len(history))
	}

	recentReminder := messageText(history[2].Parts)
	if !strings.Contains(recentReminder, "Recent threads from this session are available if you need to read prior conversation context.") {
		t.Fatalf("expected recent threads reminder, got %q", recentReminder)
	}
	if !strings.Contains(recentReminder, "Current thread ID: current-thread") {
		t.Fatalf("expected current thread id in reminder, got %q", recentReminder)
	}
	if !strings.Contains(recentReminder, "Use "+readThreadScriptPath()+" <thread-id> to print a thread transcript.") {
		t.Fatalf("expected reader script path in reminder, got %q", recentReminder)
	}
	if !strings.Contains(recentReminder, "Use "+listThreadsScriptPath()+" to list available thread IDs and names. It skips the current thread automatically when DISCOBOT_SESSION_ID is set.") {
		t.Fatalf("expected list script path in reminder, got %q", recentReminder)
	}
	if strings.Contains(recentReminder, "- current-thread (thread ID: current-thread)") {
		t.Fatalf("did not expect current thread in thread list, got %q", recentReminder)
	}
	if !strings.Contains(recentReminder, "thread-5") || strings.Contains(recentReminder, "thread-0") {
		t.Fatalf("expected only the five most recent threads, got %q", recentReminder)
	}
	if !strings.Contains(recentReminder, "Named thread subject") {
		t.Fatalf("expected named thread subject in reminder, got %q", recentReminder)
	}
	if !strings.Contains(recentReminder, "- thread-2 (thread ID: thread-2)") {
		t.Fatalf("expected unnamed thread to fall back to thread id, got %q", recentReminder)
	}
}

func TestEnsureHelperScripts_WritesManagedScripts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	agent := NewDefaultAgent(thread.NewStore(t.TempDir()), nil, nil, t.TempDir(), MCPConfig{})
	agent.ensureHelperScripts()

	data, err := os.ReadFile(readThreadScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != readThreadScriptContent() {
		t.Fatal("read-thread script content did not match expected managed content")
	}

	listData, err := os.ReadFile(listThreadsScriptPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(listData) != listThreadsScriptContent() {
		t.Fatal("list-threads script content did not match expected managed content")
	}
}

func TestEnsureHelperScripts_SkipsUnchangedScript(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	path := readThreadScriptPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(readThreadScriptContent())
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	agent := NewDefaultAgent(thread.NewStore(t.TempDir()), nil, nil, t.TempDir(), MCPConfig{})
	agent.ensureHelperScripts()

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("expected unchanged script mtime, got before=%s after=%s", before.ModTime(), after.ModTime())
	}
}

func TestListThreadsScript_SkipsCurrentThread(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not available")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	store := thread.NewStore(filepath.Join(home, ".discobot", "threads"))
	agent := NewDefaultAgent(store, nil, nil, t.TempDir(), MCPConfig{})
	agent.ensureHelperScripts()

	for _, tc := range []struct {
		id   string
		name string
	}{
		{id: "thread-current", name: "Current thread"},
		{id: "thread-1", name: "First thread"},
		{id: "thread-2", name: ""},
	} {
		if err := store.CreateThread(tc.id); err != nil {
			t.Fatal(err)
		}
		if err := store.SaveConfig(tc.id, thread.Config{Name: tc.name}); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(listThreadsScriptPath())
	cmd.Env = append(os.Environ(),
		"DISCOBOT_THREADS_DIR="+filepath.Join(home, ".discobot", "threads"),
		"DISCOBOT_SESSION_ID=thread-current",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list-threads failed: %v\n%s", err, string(out))
	}

	got := string(out)
	if strings.Contains(got, "thread-current") {
		t.Fatalf("expected current thread to be skipped, got %q", got)
	}
	if !strings.Contains(got, "thread-1\tFirst thread") {
		t.Fatalf("expected named thread in output, got %q", got)
	}
	if !strings.Contains(got, "thread-2") {
		t.Fatalf("expected unnamed thread id in output, got %q", got)
	}
}

func messageText(parts []message.Part) string {
	textParts := make([]string, 0, len(parts))
	for _, part := range parts {
		textPart, ok := part.(message.TextPart)
		if !ok {
			continue
		}
		text := strings.TrimSpace(textPart.Text)
		if text == "" {
			continue
		}
		textParts = append(textParts, text)
	}
	return strings.TrimSpace(strings.Join(textParts, "\n"))
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
