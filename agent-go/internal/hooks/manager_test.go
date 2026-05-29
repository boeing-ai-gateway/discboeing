package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
)

func TestFormatHookFailureMessage_UsesMarkdownWithInlineOutput(t *testing.T) {
	message := formatHookFailureMessage(buildHookFailureMessageMetadata(HookResult{
		Hook: Hook{
			Name:    "Go Check",
			Pattern: "*.go",
		},
		ExitCode: 2,
		Output:   "lint failed\nsecond line",
	}, []string{"main.go", "internal/app.go"}, "/tmp/go-check.log", ""))

	for _, want := range []string{
		"### Hook failed: Go Check",
		"- Exit code: `2`",
		"- Pattern: `*.go`",
		"- Files: main.go, internal/app.go",
		"#### Output",
		"```text",
		"lint failed\nsecond line",
		"Please fix the issues above and ensure the hook passes.",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected markdown hook failure message to contain %q, got:\n%s", want, message)
		}
	}
}

func TestFormatHookFailureMessage_UsesOutputPathWhenInlineOutputTooLarge(t *testing.T) {
	var outputLines []string
	for i := 1; i <= 20; i++ {
		outputLines = append(outputLines, strings.Repeat("x", 300)+strings.Join([]string{" line ", strconv.Itoa(i)}, ""))
	}

	message := formatHookFailureMessage(buildHookFailureMessageMetadata(HookResult{
		Hook: Hook{
			Name: "Go Check",
		},
		ExitCode: 1,
		Output:   strings.Join(outputLines, "\n"),
	}, nil, "/tmp/go-check.log", ""))

	if !strings.Contains(message, "Full output was written to `/tmp/go-check.log`.") {
		t.Fatalf("expected message to reference full output path, got:\n%s", message)
	}
	if !strings.Contains(message, "Last 15 lines:") {
		t.Fatalf("expected message to include the truncated output tail, got:\n%s", message)
	}
	if !strings.Contains(message, outputLines[5]) {
		t.Fatalf("expected message to include the first tailed line %q, got:\n%s", outputLines[5], message)
	}
	if !strings.Contains(message, outputLines[len(outputLines)-1]) {
		t.Fatalf("expected message to include the last tailed line %q, got:\n%s", outputLines[len(outputLines)-1], message)
	}
	if strings.Contains(message, outputLines[4]) {
		t.Fatalf("did not expect message to include lines before the tail, got:\n%s", message)
	}
}

func TestBuildHookFailureMessageMetadata_UsesTailForTruncatedOutput(t *testing.T) {
	var outputLines []string
	for i := 1; i <= 20; i++ {
		outputLines = append(outputLines, strings.Repeat("y", 300)+" line "+strconv.Itoa(i))
	}

	meta := buildHookFailureMessageMetadata(HookResult{
		Hook: Hook{
			Name: "Go Check",
		},
		ExitCode: 1,
		Output:   strings.Join(outputLines, "\n"),
	}, nil, "/tmp/go-check.log", "")

	if !meta.OutputTruncated {
		t.Fatal("expected output to be marked truncated")
	}
	if meta.OutputPath != "/tmp/go-check.log" {
		t.Fatalf("output path = %q, want %q", meta.OutputPath, "/tmp/go-check.log")
	}
	if meta.OutputTail != strings.Join(outputLines[5:], "\n") {
		t.Fatalf("output tail = %q, want %q", meta.OutputTail, strings.Join(outputLines[5:], "\n"))
	}
	if meta.Output != "" {
		t.Fatalf("expected inline output to be empty for truncated output, got %q", meta.Output)
	}
}

func TestBuildHookFailureMessageMetadata_IncludesHookPath(t *testing.T) {
	meta := buildHookFailureMessageMetadata(HookResult{
		Hook: Hook{
			Name:    "Go Check",
			Path:    ".claude/hooks/go-check.sh",
			Pattern: "*.go",
		},
		ExitCode: 1,
	}, []string{"main.go"}, "/tmp/go-check.log", "")

	if meta.HookPath != ".claude/hooks/go-check.sh" {
		t.Fatalf("hook path = %q, want %q", meta.HookPath, ".claude/hooks/go-check.sh")
	}
}

func TestBuildHookFailureMessageMetadata_RelativizesAbsoluteHookPath(t *testing.T) {
	// Use t.TempDir() so workspaceRoot is a real absolute path on all platforms
	// (e.g. C:\... on Windows, /tmp/... on Unix). A hardcoded Unix path like
	// "/workspace" is not considered absolute on Windows (no drive letter), which
	// causes filepath.IsAbs to return false and the relativization to be skipped.
	workspaceRoot := t.TempDir()
	meta := buildHookFailureMessageMetadata(HookResult{
		Hook: Hook{
			Name:    "Go Check",
			Path:    filepath.Join(workspaceRoot, ".discobot", "hooks", "go-check.sh"),
			Pattern: "*.go",
		},
		ExitCode: 1,
	}, []string{"main.go"}, "/tmp/go-check.log", workspaceRoot)

	if meta.HookPath != ".discobot/hooks/go-check.sh" {
		t.Fatalf("hook path = %q, want %q", meta.HookPath, ".discobot/hooks/go-check.sh")
	}
}

func TestDiscoverHooks_ParsesIgnoreAndExcludeFields(t *testing.T) {
	hooksDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooksDir, "ignore-field.md"), []byte(`---
name: Ignore Field
type: file
engine: ai
pattern: "*.go"
ignore: "*_templ.go"
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(ignore-field) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "exclude-field.md"), []byte(`---
name: Exclude Field
type: file
engine: ai
pattern: "*.go"
exclude: "mock_*.go"
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(exclude-field) failed: %v", err)
	}

	hooks, err := DiscoverHooks(hooksDir)
	if err != nil {
		t.Fatalf("DiscoverHooks() failed: %v", err)
	}

	ignoresByName := map[string]string{}
	for _, hook := range hooks {
		ignoresByName[hook.Name] = hook.Ignore
	}
	if ignoresByName["Ignore Field"] != "*_templ.go" {
		t.Fatalf("ignore field = %q, want *_templ.go", ignoresByName["Ignore Field"])
	}
	if ignoresByName["Exclude Field"] != "mock_*.go" {
		t.Fatalf("exclude field = %q, want mock_*.go", ignoresByName["Exclude Field"])
	}
}

func TestDiscoverHooks_ParsesReviewPhase(t *testing.T) {
	hooksDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooksDir, "review.md"), []byte(`---
name: Review Hook
type: file
engine: ai
pattern: "*.go"
phase: review
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(review hook) failed: %v", err)
	}

	hooks, err := DiscoverHooks(hooksDir)
	if err != nil {
		t.Fatalf("DiscoverHooks() failed: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("hook count = %d, want 1", len(hooks))
	}
	if hooks[0].Phase != "review" {
		t.Fatalf("phase = %q, want review", hooks[0].Phase)
	}
}

func TestDiscoverHooks_RejectsInvalidPhase(t *testing.T) {
	hooksDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hooksDir, "invalid.md"), []byte(`---
name: Invalid Phase
type: file
engine: ai
pattern: "*.go"
phase: ship
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invalid hook) failed: %v", err)
	}

	_, err := DiscoverHooks(hooksDir)
	if err == nil {
		t.Fatal("expected invalid phase error")
	}
	if !strings.Contains(err.Error(), "invalid hook phase") {
		t.Fatalf("error = %v, want invalid hook phase", err)
	}
}

func TestRerunHook_FailureWithNotifyLLMReturnsReprompt(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)        // Unix
	t.Setenv("USERPROFILE", testHomeDir) // Windows
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "go-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
echo "lint failed"
exit 1
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	runResult, err := mgr.RerunHook("go-check")
	if err != nil {
		t.Fatalf("RerunHook() failed: %v", err)
	}
	if runResult == nil {
		t.Fatal("expected rerun result")
	}
	if runResult.Result.Success {
		t.Fatal("expected rerun to fail")
	}
	if !runResult.Eval.ShouldReprompt {
		t.Fatal("expected rerun failure to request reprompt")
	}
	if !strings.Contains(runResult.Result.Output, "lint failed") {
		t.Fatalf("expected hook output to contain %q, got %q", "lint failed", runResult.Result.Output)
	}
	if runResult.Eval.HookFailure == nil {
		t.Fatal("expected rerun failure metadata")
	}
	if runResult.Eval.HookFailure.HookName != "Go Check" {
		t.Fatalf("hook name = %q, want %q", runResult.Eval.HookFailure.HookName, "Go Check")
	}
	if !strings.Contains(runResult.Eval.LLMMessage, "### Hook failed: Go Check") {
		t.Fatalf("expected rerun failure message, got: %s", runResult.Eval.LLMMessage)
	}
}

func TestRerunHook_RunsSessionHook(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "install-deps.sh")
	hookSource := `#!/bin/bash
#---
# name: Install Deps
# type: session
#---
echo "$DISCOBOT_SESSION_ID:$SESSION_HOOK_RERUN_ENV" > session-hook-ran
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	mgr.SetStartupHookEnv(func(Hook) map[string]string {
		return map[string]string{"SESSION_HOOK_RERUN_ENV": "ok"}
	})

	runResult, err := mgr.RerunHook("install-deps")
	if err != nil {
		t.Fatalf("RerunHook() failed: %v", err)
	}
	if runResult == nil {
		t.Fatal("expected rerun result")
	}
	if !runResult.Result.Success {
		t.Fatalf("expected session hook rerun to pass, output: %s", runResult.Result.Output)
	}
	marker, err := os.ReadFile(filepath.Join(workspaceRoot, "session-hook-ran"))
	if err != nil {
		t.Fatalf("ReadFile(marker) failed: %v", err)
	}
	if strings.TrimSpace(string(marker)) != "session-123:ok" {
		t.Fatalf("marker = %q, want session-123:ok", string(marker))
	}

	status := LoadStatus(mgr.hooksDataDir)
	got := status.Hooks["install-deps"]
	if got.LastResult != "success" {
		t.Fatalf("last result = %q, want success", got.LastResult)
	}
	if got.Type != string(HookTypeSession) {
		t.Fatalf("hook type = %q, want %q", got.Type, HookTypeSession)
	}
}

func TestRerunHook_ReturnsPausedWithoutRunning(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	runMarker := filepath.Join(workspaceRoot, "hook-ran")
	hookPath := filepath.Join(hooksDir, "go-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
touch hook-ran
exit 0
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := mgr.SetExecutionPaused(true); err != nil {
		t.Fatalf("SetExecutionPaused() failed: %v", err)
	}

	runResult, err := mgr.RerunHook("go-check")
	if !errors.Is(err, ErrHookPaused) {
		t.Fatalf("RerunHook() error = %v, want %v", err, ErrHookPaused)
	}
	if runResult != nil {
		t.Fatalf("RerunHook() result = %#v, want nil", runResult)
	}
	if _, err := os.Stat(runMarker); !os.IsNotExist(err) {
		t.Fatalf("paused hook wrote marker, stat err = %v", err)
	}
	status := mgr.GetStatus()
	if status.Hooks["go-check"].RunCount != 0 {
		t.Fatalf("run count = %d, want 0", status.Hooks["go-check"].RunCount)
	}
}

func TestSetExecutionPausedFalseKeepsIndividualHooksPaused(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	for name, source := range map[string]string{
		"go-check.sh": `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
exit 0
`,
		"setup.sh": `#!/bin/bash
#---
# name: Setup
# type: session
#---
exit 0
`,
	} {
		if err := os.WriteFile(filepath.Join(hooksDir, name), []byte(source), 0o755); err != nil {
			t.Fatalf("WriteFile(%s) failed: %v", name, err)
		}
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := mgr.SetHookExecutionPaused("go-check", true); err != nil {
		t.Fatalf("SetHookExecutionPaused(go-check) failed: %v", err)
	}
	if err := mgr.SetHookExecutionPaused("setup", true); err != nil {
		t.Fatalf("SetHookExecutionPaused(setup) failed: %v", err)
	}
	if err := mgr.SetExecutionPaused(true); err != nil {
		t.Fatalf("SetExecutionPaused(true) failed: %v", err)
	}

	if err := mgr.SetExecutionPaused(false); err != nil {
		t.Fatalf("SetExecutionPaused(false) failed: %v", err)
	}

	status := mgr.GetStatus()
	if status.ExecutionPaused {
		t.Fatal("global pause still set after resume")
	}
	for _, hookID := range []string{"go-check", "setup"} {
		if !status.Hooks[hookID].ExecutionPaused {
			t.Fatalf("hook %q resumed by global resume", hookID)
		}
	}
}

func TestSetHookExecutionPausedFalseClearsGlobalPause(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "go-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
exit 0
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := mgr.SetExecutionPaused(true); err != nil {
		t.Fatalf("SetExecutionPaused(true) failed: %v", err)
	}

	if err := mgr.SetHookExecutionPaused("go-check", false); err != nil {
		t.Fatalf("SetHookExecutionPaused(false) failed: %v", err)
	}

	status := mgr.GetStatus()
	if status.ExecutionPaused {
		t.Fatal("global pause still set after resuming one hook")
	}
	if status.Hooks["go-check"].ExecutionPaused {
		t.Fatal("hook pause still set after resuming one hook")
	}
}

func TestRerunHook_RunsAIHookInStableThread(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "review.md")
	hookSource := `---
name: Review
type: file
engine: ai
pattern: "*.go"
subagent: reviewer
---
Only approve idiomatic Go changes.
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	runner := &testAIHookAgent{response: "SUCCESS looks good"}
	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetAIHookAgent(runner)
	mgr.SetAIHookEvaluator(runner)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	first, err := mgr.RerunHook("review-md")
	if err != nil {
		t.Fatalf("RerunHook() failed: %v", err)
	}
	if first == nil {
		t.Fatal("expected rerun result")
	}
	if !first.Result.Success {
		t.Fatalf("expected AI hook to pass, output: %s", first.Result.Output)
	}
	if first.Result.Output != "SUCCESS looks good" {
		t.Fatalf("output = %q, want SUCCESS output", first.Result.Output)
	}

	runner.response = "FEEDBACK: add a test"
	second, err := mgr.RerunHook("review-md")
	if err != nil {
		t.Fatalf("second RerunHook() failed: %v", err)
	}
	if second.Result.Success {
		t.Fatal("expected AI feedback to fail the hook")
	}
	if second.Result.Output != "FEEDBACK: add a test" {
		t.Fatalf("output = %q, want feedback", second.Result.Output)
	}
	if len(runner.promptThreadIDs) != 2 {
		t.Fatalf("prompt calls = %d, want 2", len(runner.promptThreadIDs))
	}
	if len(runner.metadataTypes) != 2 {
		t.Fatalf("metadata type updates = %d, want 2", len(runner.metadataTypes))
	}
	for _, got := range runner.metadataTypes {
		if got != "hook" {
			t.Fatalf("metadata type = %q, want hook; all types: %v", got, runner.metadataTypes)
		}
	}
	if runner.promptThreadIDs[0] != runner.promptThreadIDs[1] {
		t.Fatalf("AI hook used different threads: %q then %q", runner.promptThreadIDs[0], runner.promptThreadIDs[1])
	}
	if runner.createdThreadID != runner.promptThreadIDs[0] {
		t.Fatalf("created thread = %q, prompt thread = %q", runner.createdThreadID, runner.promptThreadIDs[0])
	}
	if !strings.Contains(runner.prompts[0], "Only approve idiomatic Go changes.") {
		t.Fatalf("expected prompt to include hook body, got:\n%s", runner.prompts[0])
	}
	if !strings.Contains(runner.prompts[1], "New changes are available for review: Review.") {
		t.Fatalf("expected follow-up prompt to describe new changes, got:\n%s", runner.prompts[1])
	}
	if strings.Contains(runner.prompts[1], "You are running the Discobot hook") {
		t.Fatalf("follow-up prompt should not reintroduce hook execution, got:\n%s", runner.prompts[1])
	}
	contextDir := filepath.Dir(aiHookContextFilePath(runner.createdThreadID, "review-md", time.Now().UTC()))
	contextFiles, err := filepath.Glob(filepath.Join(contextDir, "context-*.md"))
	if err != nil {
		t.Fatalf("Glob(context) failed: %v", err)
	}
	if len(contextFiles) != 2 {
		t.Fatalf("context files = %d, want 2", len(contextFiles))
	}
	contextPath := contextFiles[0]
	if !strings.Contains(runner.prompts[0], contextPath) {
		t.Fatalf("expected prompt to reference context file %q, got:\n%s", contextPath, runner.prompts[0])
	}
	contextData, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("ReadFile(context) failed: %v", err)
	}
	if !strings.Contains(string(contextData), "Only approve idiomatic Go changes.") {
		t.Fatalf("expected context file to include hook body, got:\n%s", string(contextData))
	}
	if len(runner.subagents) != 2 {
		t.Fatalf("subagent calls = %d, want 2", len(runner.subagents))
	}
	for _, got := range runner.subagents {
		if got != "reviewer" {
			t.Fatalf("subagent = %q, want reviewer", got)
		}
	}
}

func TestRunAIHook_UsesProvidedModel(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()

	runner := &testAIHookAgent{response: "SUCCESS looks good"}
	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetAIHookAgent(runner)
	mgr.SetAIHookEvaluator(runner)

	result := mgr.runAIHook(Hook{
		ID:          "review-md",
		Name:        "Review",
		Description: "Review changes",
		Engine:      HookEngineAI,
		Prompt:      "Only approve safe changes.",
		Subagent:    "reviewer",
	}, runHookOptions{
		outputPath: filepath.Join(testHomeDir, "hook.log"),
		model:      "openai/gpt-5.5",
	})

	if !result.Success {
		t.Fatalf("expected AI hook to pass, output: %s", result.Output)
	}
	if len(runner.models) != 2 {
		t.Fatalf("prompt models = %d, want 2", len(runner.models))
	}
	for _, got := range runner.models {
		if got != "openai/gpt-5.5" {
			t.Fatalf("prompt model = %q, want openai/gpt-5.5", got)
		}
	}
}

func TestEvaluateFileHooks_AIHookReportsFilesSinceLastRun(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	hookPath := filepath.Join(hooksDir, "review.md")
	hookSource := `---
name: Review
type: file
engine: ai
pattern: "*.go"
---
Review Go changes.
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) failed: %v", err)
	}

	runner := &testAIHookAgent{response: "FEEDBACK: add a test"}
	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetAIHookAgent(runner)
	mgr.SetAIHookEvaluator(runner)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	first := mgr.EvaluateFileHooks()
	if !first.ShouldReprompt {
		t.Fatal("expected first AI hook failure to request reprompt")
	}
	if !strings.Contains(runner.prompts[0], "- main.go") {
		t.Fatalf("first prompt should include main.go, got:\n%s", runner.prompts[0])
	}

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "helper.go"), []byte("package main\n\nvar helper = true\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(helper.go) failed: %v", err)
	}

	second := mgr.EvaluateFileHooks()
	if !second.ShouldReprompt {
		t.Fatal("expected second AI hook failure to request reprompt")
	}
	if len(runner.prompts) < 2 {
		t.Fatalf("prompts = %d, want at least 2", len(runner.prompts))
	}
	secondHookPrompt := runner.prompts[1]
	if !strings.Contains(secondHookPrompt, "- helper.go") {
		t.Fatalf("second prompt should include helper.go, got:\n%s", secondHookPrompt)
	}
	if strings.Contains(secondHookPrompt, "- main.go") {
		t.Fatalf("second prompt should only report files changed since the last hook run, got:\n%s", secondHookPrompt)
	}
}

func TestEvaluateFileHooks_DoesNotMissFilesChangedDuringHookRun(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "review.md"), []byte(`---
name: Review
type: file
engine: ai
pattern: "*.go"
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(review) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) failed: %v", err)
	}

	wroteDuringRun := false
	runner := &testAIHookAgent{response: "SUCCESS: looks good"}
	runner.onPrompt = func() {
		if wroteDuringRun {
			return
		}
		wroteDuringRun = true
		time.Sleep(20 * time.Millisecond)
		if err := os.WriteFile(filepath.Join(workspaceRoot, "helper.go"), []byte("package main\n\nvar helper = true\n"), 0o644); err != nil {
			t.Errorf("WriteFile(helper.go) failed: %v", err)
		}
	}

	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetAIHookAgent(runner)
	mgr.SetAIHookEvaluator(runner)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	first := mgr.EvaluateFileHooks()
	if first.ShouldReprompt {
		t.Fatal("expected first AI hook success not to request reprompt")
	}
	if len(runner.prompts) != 1 {
		t.Fatalf("hook prompts = %d, want 1", len(runner.prompts))
	}
	if !strings.Contains(runner.prompts[0], "- main.go") {
		t.Fatalf("first prompt should include main.go, got:\n%s", runner.prompts[0])
	}

	second := mgr.EvaluateFileHooks()
	if second.ShouldReprompt {
		t.Fatal("expected second AI hook success not to request reprompt")
	}
	if len(runner.prompts) != 2 {
		t.Fatalf("hook prompts = %d, want 2; prompts:\n%v", len(runner.prompts), runner.prompts)
	}
	if !strings.Contains(runner.prompts[1], "- helper.go") {
		t.Fatalf("second prompt should include file changed during first run, got:\n%s", runner.prompts[1])
	}
	if strings.Contains(runner.prompts[1], "- main.go") {
		t.Fatalf("second prompt should only include changes after the first run cutoff, got:\n%s", runner.prompts[1])
	}
}

func TestEvaluateFileHooks_SkipsPendingHookWithoutNewMatchingFiles(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "review-go.md"), []byte(`---
name: Review Go
type: file
engine: ai
pattern: "*.go"
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(review-go) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "review-md.md"), []byte(`---
name: Review Markdown
type: file
engine: ai
pattern: "*.md"
---
Review Markdown changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(review-md) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) failed: %v", err)
	}

	runner := &testAIHookAgent{response: "FEEDBACK: add a test"}
	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetAIHookAgent(runner)
	mgr.SetAIHookEvaluator(runner)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	first := mgr.EvaluateFileHooks()
	if !first.ShouldReprompt {
		t.Fatal("expected first AI hook failure to request reprompt")
	}
	if len(runner.prompts) != 1 {
		t.Fatalf("hook prompts = %d, want 1", len(runner.prompts))
	}
	if !strings.Contains(runner.prompts[0], "- main.go") {
		t.Fatalf("first prompt should include main.go, got:\n%s", runner.prompts[0])
	}

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "README.md"), []byte("# readme\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) failed: %v", err)
	}

	second := mgr.EvaluateFileHooks()
	if !second.ShouldReprompt {
		t.Fatal("expected second AI hook failure to request reprompt")
	}
	if len(runner.prompts) != 2 {
		t.Fatalf("hook prompts = %d, want 2; prompts:\n%v", len(runner.prompts), runner.prompts)
	}
	if !strings.Contains(runner.prompts[1], "- README.md") {
		t.Fatalf("second prompt should include README.md, got:\n%s", runner.prompts[1])
	}
	if strings.Contains(runner.prompts[1], "- main.go") {
		t.Fatalf("pending Go hook should not rerun with old files, got:\n%s", runner.prompts[1])
	}
}

func TestEvaluateFileHooks_IgnoresPerHookAndGlobalPatterns(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "review-go.md"), []byte(`---
name: Review Go
type: file
engine: ai
pattern: "*.go"
ignore: "*_templ.go"
---
Review Go changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(review-go) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "review-md.md"), []byte(`---
name: Review Markdown
type: file
engine: ai
pattern: "*.md"
---
Review Markdown changes.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(review-md) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "ignore"), []byte("# generated docs\nignored.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ignore) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "view_templ.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(view_templ.go) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "ignored.md"), []byte("# generated\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(ignored.md) failed: %v", err)
	}

	runner := &testAIHookAgent{response: "SUCCESS: looks good"}
	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetAIHookAgent(runner)
	mgr.SetAIHookEvaluator(runner)
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	result := mgr.EvaluateFileHooks()
	if result.ShouldReprompt {
		t.Fatal("expected ignored files not to request reprompt")
	}
	if len(runner.prompts) != 1 {
		t.Fatalf("hook prompts = %d, want 1; prompts:\n%v", len(runner.prompts), runner.prompts)
	}
	if !strings.Contains(runner.prompts[0], "- main.go") {
		t.Fatalf("prompt should include non-ignored Go file, got:\n%s", runner.prompts[0])
	}
	for _, ignored := range []string{"view_templ.go", "ignored.md"} {
		if strings.Contains(runner.prompts[0], ignored) {
			t.Fatalf("prompt should not include ignored file %q, got:\n%s", ignored, runner.prompts[0])
		}
	}
}

func TestRunSessionHooks_CapturesBackgroundHookEnv(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "background.sh")
	hookSource := `#!/bin/bash
#---
# name: Background
# type: session
#---
echo "$CAPTURED_BACKGROUND_ENV:$DISCOBOT_SESSION_ID" > background-env
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	mgr.SetStartupHookEnv(func(Hook) map[string]string {
		return map[string]string{"CAPTURED_BACKGROUND_ENV": "available"}
	})

	wait := mgr.RunSessionHooks(nil)
	mgr.SetStartupHookEnv(nil)
	wait()

	data, err := os.ReadFile(filepath.Join(workspaceRoot, "background-env"))
	if err != nil {
		t.Fatalf("ReadFile(background-env) failed: %v", err)
	}
	if strings.TrimSpace(string(data)) != "available:session-123" {
		t.Fatalf("background env = %q, want available:session-123", string(data))
	}
}

func TestRunSessionHooks_SkipsPausedHook(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "setup.sh")
	hookSource := `#!/bin/bash
#---
# name: Setup
# type: session
#---
touch session-hook-ran
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := mgr.SetHookExecutionPaused("setup", true); err != nil {
		t.Fatalf("SetHookExecutionPaused() failed: %v", err)
	}

	wait := mgr.RunSessionHooks(nil)
	wait()

	runMarker := filepath.Join(workspaceRoot, "session-hook-ran")
	if _, err := os.Stat(runMarker); !os.IsNotExist(err) {
		t.Fatalf("paused session hook wrote marker, stat err = %v", err)
	}
	status := mgr.GetStatus()
	if status.Hooks["setup"].RunCount != 0 {
		t.Fatalf("run count = %d, want 0", status.Hooks["setup"].RunCount)
	}
}

func TestEmitCurrentStatusChunk_EmitsHooksStatusDataChunk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mgr := NewManager(t.TempDir(), "session-123")

	status := StatusFile{
		Hooks: map[string]HookRunStatus{
			"go-check": {
				HookID:       "go-check",
				HookName:     "Go Check",
				Type:         string(HookTypeFile),
				LastRunAt:    "2026-03-31T00:00:00Z",
				LastResult:   "pending",
				LastExitCode: 0,
				OutputPath:   "/tmp/go-check.log",
			},
		},
		PendingHooks:    []string{"go-check"},
		LastEvaluatedAt: "2026-03-31T00:00:00Z",
	}
	if err := SaveStatus(mgr.hooksDataDir, status); err != nil {
		t.Fatalf("SaveStatus() failed: %v", err)
	}

	var gotChunk message.MessageChunk
	mgr.SetChunkEmitter(func(chunk message.MessageChunk) {
		gotChunk = chunk
	})
	if gotChunk != nil {
		t.Fatalf("expected no initial chunk replay, got %#v", gotChunk)
	}

	mgr.emitCurrentStatusChunk()

	dataChunk, ok := gotChunk.(message.DataChunk)
	if !ok {
		t.Fatalf("expected DataChunk, got %T", gotChunk)
	}
	if dataChunk.DataType != "hooks-status" {
		t.Fatalf("data type = %q, want hooks-status", dataChunk.DataType)
	}

	var gotStatus StatusFile
	if err := json.Unmarshal(dataChunk.Data, &gotStatus); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}
	if gotStatus.Hooks["go-check"].LastResult != "pending" {
		t.Fatalf("hook result = %q, want pending", gotStatus.Hooks["go-check"].LastResult)
	}
}

func TestEvaluateFileHooks_RunsLowestPendingHookIDFirst(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)        // Unix
	t.Setenv("USERPROFILE", homeDir) // Windows

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	orderLog := filepath.Join(workspaceRoot, "hook-order.log")
	firstHookPath := filepath.Join(hooksDir, "01-first.sh")
	firstHook := `#!/bin/bash
#---
# name: Zulu First
# type: file
# pattern: "*.txt"
#---
echo "01-first" >> "hook-order.log"
exit 1
`
	if err := os.WriteFile(firstHookPath, []byte(firstHook), 0o755); err != nil {
		t.Fatalf("WriteFile(firstHook) failed: %v", err)
	}

	secondHookPath := filepath.Join(hooksDir, "02-second.sh")
	secondHook := `#!/bin/bash
#---
# name: Alpha Second
# type: file
# pattern: "*.txt"
#---
echo "02-second" >> "hook-order.log"
exit 0
`
	if err := os.WriteFile(secondHookPath, []byte(secondHook), 0o755); err != nil {
		t.Fatalf("WriteFile(secondHook) failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceRoot, "pending.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(pending.txt) failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	eval := mgr.EvaluateFileHooks()
	if !eval.ShouldReprompt {
		t.Fatal("expected first pending hook failure to request reprompt")
	}
	if eval.HookFailure == nil {
		t.Fatal("expected hook failure metadata")
	}
	if eval.HookFailure.HookName != "Zulu First" {
		t.Fatalf("hook failure name = %q, want %q", eval.HookFailure.HookName, "Zulu First")
	}

	orderData, err := os.ReadFile(orderLog)
	if err != nil {
		t.Fatalf("ReadFile(orderLog) failed: %v", err)
	}
	if string(orderData) != "01-first\n" {
		t.Fatalf("hook execution order = %q, want only first hook", string(orderData))
	}

	status := LoadStatus(mgr.hooksDataDir)
	if strings.Join(status.PendingHooks, ",") != "01-first,02-second" {
		t.Fatalf("pendingHooks = %v, want [01-first 02-second]", status.PendingHooks)
	}
}

func TestEvaluateFileHooks_SkipsPausedHook(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	runMarker := filepath.Join(workspaceRoot, "file-hook-ran")
	hookPath := filepath.Join(hooksDir, "go-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Go Check
# type: file
# pattern: "*.go"
#---
touch file-hook-ran
exit 1
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go) failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := mgr.SetHookExecutionPaused("go-check", true); err != nil {
		t.Fatalf("SetHookExecutionPaused() failed: %v", err)
	}

	eval := mgr.EvaluateFileHooks()
	if eval.ShouldReprompt {
		t.Fatal("paused hook should not request reprompt")
	}
	if _, err := os.Stat(runMarker); !os.IsNotExist(err) {
		t.Fatalf("paused file hook wrote marker, stat err = %v", err)
	}
	status := mgr.GetStatus()
	if status.Hooks["go-check"].RunCount != 0 {
		t.Fatalf("run count = %d, want 0", status.Hooks["go-check"].RunCount)
	}
	if strings.Join(status.PendingHooks, ",") != "go-check" {
		t.Fatalf("pendingHooks = %v, want [go-check]", status.PendingHooks)
	}
}

func TestEvaluateFileHooks_PhaseGatedHookWaitsForReview(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "review-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Review Check
# type: file
# pattern: "*.txt"
# phase: review
#---
echo "review hook ran" >> "hook-runs.log"
exit 1
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile(review hook) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "pending.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(pending.txt) failed: %v", err)
	}

	phase := ""
	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetThreadPhaseLookup(func(string) string {
		return phase
	})
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	first := mgr.EvaluateFileHooks("thread-1")
	if first.ShouldReprompt {
		t.Fatal("expected review hook not to run before review phase")
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, "hook-runs.log")); !os.IsNotExist(err) {
		t.Fatalf("hook-runs.log exists before review phase: %v", err)
	}
	status := LoadStatus(mgr.hooksDataDir)
	if strings.Join(status.PendingHooks, ",") != "review-check" {
		t.Fatalf("pendingHooks = %v, want [review-check]", status.PendingHooks)
	}
	if status.Hooks["review-check"].LastResult != "pending" {
		t.Fatalf("hook result = %q, want pending", status.Hooks["review-check"].LastResult)
	}

	phase = "review"
	second := mgr.EvaluateFileHooks("thread-1")
	if !second.ShouldReprompt {
		t.Fatal("expected review hook failure to request reprompt in review phase")
	}
	data, err := os.ReadFile(filepath.Join(workspaceRoot, "hook-runs.log"))
	if err != nil {
		t.Fatalf("ReadFile(hook-runs.log) failed: %v", err)
	}
	if string(data) != "review hook ran\n" {
		t.Fatalf("hook run log = %q", string(data))
	}
}

func TestSetEnvSnapshot_PassesVarsToRerunHook(t *testing.T) {
	testHomeDir := t.TempDir()
	t.Setenv("HOME", testHomeDir)
	t.Setenv("USERPROFILE", testHomeDir)
	workspaceRoot := t.TempDir()
	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "env-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Env Check
# type: file
# pattern: "*.go"
#---
if [ -z "$TEST_CREDENTIAL" ]; then
  echo "TEST_CREDENTIAL not set"
  exit 1
fi
echo "TEST_CREDENTIAL=$TEST_CREDENTIAL"
exit 0
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetEnvSnapshot(func() map[string]string {
		return map[string]string{"TEST_CREDENTIAL": "secret-value"}
	})
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	runResult, err := mgr.RerunHook("env-check")
	if err != nil {
		t.Fatalf("RerunHook() failed: %v", err)
	}
	if runResult == nil {
		t.Fatal("expected rerun result")
	}
	if !runResult.Result.Success {
		t.Fatalf("expected hook to succeed with env var present, got output: %s", runResult.Result.Output)
	}
	if !strings.Contains(runResult.Result.Output, "TEST_CREDENTIAL=secret-value") {
		t.Fatalf("expected hook output to contain credential value, got: %s", runResult.Result.Output)
	}
}

func TestSetEnvSnapshot_PassesVarsToEvaluateFileHooks(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = workspaceRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	hooksDir := filepath.Join(workspaceRoot, HooksDir)
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	hookPath := filepath.Join(hooksDir, "env-check.sh")
	hookSource := `#!/bin/bash
#---
# name: Env Check
# type: file
# pattern: "*.go"
#---
if [ -z "$TEST_CREDENTIAL" ]; then
  echo "TEST_CREDENTIAL not set"
  exit 1
fi
echo "TEST_CREDENTIAL=$TEST_CREDENTIAL"
exit 0
`
	if err := os.WriteFile(hookPath, []byte(hookSource), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	mgr := NewManager(workspaceRoot, "session-123")
	mgr.SetEnvSnapshot(func() map[string]string {
		return map[string]string{"TEST_CREDENTIAL": "secret-value"}
	})
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	eval := mgr.EvaluateFileHooks()
	if !eval.Evaluated {
		t.Fatal("expected hooks to be evaluated")
	}
	if eval.FailedResult != nil {
		t.Fatalf("expected hook to succeed with env var present, got output: %s", eval.FailedResult.Output)
	}
}

func TestGetHookOutput_ReturnsInlineOutputWhenUnderLimit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mgr := NewManager(t.TempDir(), "session-123")

	outputPath := GetHookOutputPath(mgr.hooksDataDir, "go-check")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	if err := os.WriteFile(outputPath, []byte("hello hook"), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	output, err := mgr.GetHookOutput("go-check")
	if err != nil {
		t.Fatalf("GetHookOutput() failed: %v", err)
	}
	if output.Output != "hello hook" {
		t.Fatalf("output = %q, want %q", output.Output, "hello hook")
	}
	if output.SizeBytes != int64(len("hello hook")) {
		t.Fatalf("size = %d, want %d", output.SizeBytes, len("hello hook"))
	}
	if output.DisplayedBytes != int64(len("hello hook")) {
		t.Fatalf("displayed size = %d, want %d", output.DisplayedBytes, len("hello hook"))
	}
	if output.TooLarge {
		t.Fatal("expected output to remain inline")
	}
}

func TestGetHookOutput_ReturnsTailWhenOverLimit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mgr := NewManager(t.TempDir(), "session-123")

	outputPath := GetHookOutputPath(mgr.hooksDataDir, "go-check")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}
	largeOutput := strings.Repeat("a", 32) + strings.Repeat("x", HookOutputInlineMaxBytes) + "tail"
	if err := os.WriteFile(outputPath, []byte(largeOutput), 0o644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	output, err := mgr.GetHookOutput("go-check")
	if err != nil {
		t.Fatalf("GetHookOutput() failed: %v", err)
	}
	wantTail := largeOutput[len(largeOutput)-HookOutputInlineMaxBytes:]
	if output.Output != wantTail {
		t.Fatalf("output = %q, want %q", output.Output, wantTail)
	}
	if output.SizeBytes != int64(len(largeOutput)) {
		t.Fatalf("size = %d, want %d", output.SizeBytes, len(largeOutput))
	}
	if output.DisplayedBytes != HookOutputInlineMaxBytes {
		t.Fatalf("displayed size = %d, want %d", output.DisplayedBytes, HookOutputInlineMaxBytes)
	}
	if !output.TooLarge {
		t.Fatal("expected output to be marked too large")
	}
}

type testAIHookAgent struct {
	response        string
	createdThreadID string
	threads         map[string]bool
	finalResponses  map[string]string
	promptThreadIDs []string
	prompts         []string
	evalPrompts     []string
	subagents       []string
	models          []string
	metadataTypes   []string
	onPrompt        func()
}

func (a *testAIHookAgent) CreateThread(_ context.Context, req agent.CreateThreadRequest) (agent.ThreadInfo, error) {
	if a.threads == nil {
		a.threads = map[string]bool{}
	}
	a.threads[req.ID] = true
	if a.createdThreadID == "" && strings.HasPrefix(req.ID, "hook-") && !strings.HasSuffix(req.ID, "-evaluation") {
		a.createdThreadID = req.ID
	}
	a.recordMetadataType(req.Metadata)
	return agent.ThreadInfo{ID: req.ID, Name: req.Name, CWD: req.CWD, Metadata: req.Metadata}, nil
}

func (a *testAIHookAgent) UpdateThread(_ context.Context, threadID string, req agent.UpdateThreadRequest) (agent.ThreadInfo, error) {
	if a.threads == nil || !a.threads[threadID] {
		return agent.ThreadInfo{}, os.ErrNotExist
	}
	a.recordMetadataType(req.Metadata)
	return agent.ThreadInfo{ID: threadID, Metadata: req.Metadata}, nil
}

func (a *testAIHookAgent) recordMetadataType(metadata json.RawMessage) {
	var values map[string]any
	if err := json.Unmarshal(metadata, &values); err == nil {
		if value, ok := values["type"].(string); ok {
			a.metadataTypes = append(a.metadataTypes, value)
		}
	}
}

func (a *testAIHookAgent) GetThreadInfo(threadID string) (agent.ThreadInfo, error) {
	if a.threads != nil && a.threads[threadID] {
		return agent.ThreadInfo{ID: threadID}, nil
	}
	return agent.ThreadInfo{}, os.ErrNotExist
}

func (a *testAIHookAgent) Prompt(_ context.Context, threadID string, req agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
	a.promptThreadIDs = append(a.promptThreadIDs, threadID)
	a.subagents = append(a.subagents, req.SubagentType)
	a.models = append(a.models, req.Model)
	response := a.response
	if len(req.UserParts) == 1 {
		if part, ok := req.UserParts[0].(message.UITextPart); ok {
			a.prompts = append(a.prompts, part.Text)
		}
	}
	if a.onPrompt != nil {
		a.onPrompt()
	}
	if a.finalResponses == nil {
		a.finalResponses = map[string]string{}
	}
	a.finalResponses[threadID] = response
	return func(yield func(message.MessageChunk, error) bool) {
		yield(message.TextDeltaChunk{Delta: response}, nil)
	}
}

func (a *testAIHookAgent) CompleteText(_ context.Context, model string, messages []message.Message, _ *int) (string, error) {
	a.models = append(a.models, model)
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if text, ok := part.(message.TextPart); ok {
				a.evalPrompts = append(a.evalPrompts, text.Text)
			}
		}
	}
	if strings.Contains(a.response, "SUCCESS") {
		return `{"success":true,"notifyLLM":false,"reason":"passed"}`, nil
	}
	return `{"success":false,"notifyLLM":true,"reason":"feedback"}`, nil
}

func (a *testAIHookAgent) Resume(_ context.Context, _ string, _ agent.PromptRequest) (agent.ResumeResult, error) {
	return agent.ResumeResult{
		Stream: func(yield func(message.MessageChunk, error) bool) {
			yield(message.TextDeltaChunk{Delta: a.response}, nil)
		},
	}, nil
}

func (a *testAIHookAgent) HasInterruptedTurn(string) (bool, error) {
	return false, nil
}

func (a *testAIHookAgent) PendingQuestion(string) (*agent.PendingQuestion, error) {
	return nil, nil
}

func (a *testAIHookAgent) FinalResponse(threadID string) (string, error) {
	if a.finalResponses == nil {
		return "", nil
	}
	return a.finalResponses[threadID], nil
}
