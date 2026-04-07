package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/agentimpl"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func setTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestExecuteRequestUserCredential_PausesWithCredentialPayload(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), "thread-1")
	input := `{"credentials":[{"envVar":" GITHUB_TOKEN ","name":" GitHub access token ","justification":" clone a private repo ","approvedUses":[{"description":" create pull requests "}]}]}`

	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "tc-credential",
		ToolName:   "RequestUserCredential",
		Input:      input,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Approval == nil {
		t.Fatal("expected RequestUserCredential to require approval")
	}
	if len(result.Approval.Questions) != 0 {
		t.Fatalf("expected no questions payload, got %s", string(result.Approval.Questions))
	}

	var credentials []api.RequestedCredential
	if err := json.Unmarshal(result.Approval.Credentials, &credentials); err != nil {
		t.Fatalf("failed to parse credential payload: %v", err)
	}
	if len(credentials) != 1 || credentials[0].EnvVar != "GITHUB_TOKEN" {
		t.Fatalf("unexpected credential payload: %#v", credentials)
	}
	if credentials[0].Name != "GitHub access token" || credentials[0].Justification != "clone a private repo" {
		t.Fatalf("expected trimmed credential fields, got %#v", credentials[0])
	}
	if len(credentials[0].ApprovedUses) != 1 || credentials[0].ApprovedUses[0].Description != "create pull requests" {
		t.Fatalf("unexpected credential approved uses: %#v", credentials[0].ApprovedUses)
	}
}

func TestResolveRequestUserCredential_HidesSecretValue(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), "thread-1")

	result, err := e.ResolveAnswer(nil, message.ToolCallPart{
		ToolCallID: "tc-credential",
		ToolName:   "RequestUserCredential",
		Input:      `{"credentials":[{"envVar":"GITHUB_TOKEN","name":"GitHub access token","justification":"clone a private repo","approvedUses":[{"description":"create pull requests"}]}]}`,
	}, api.AnswerQuestionRequest{
		Answers: map[string]string{
			requestUserCredentialGrantedKey: `{"grantedCredentials":[{"credentialId":"cred_s_123","envVar":"GITHUB_TOKEN","name":"GitHub access token","approvedUses":[{"id":"use_s_456","description":"create pull requests"}]}]}`,
		},
	})
	if err != nil {
		t.Fatalf("ResolveAnswer returned error: %v", err)
	}

	jsonOut, ok := result.Result.Output.(message.JSONOutput)
	if !ok {
		t.Fatalf("expected JSONOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(string(jsonOut.Value), `"credentialId":"cred_s_123"`) {
		t.Fatalf("expected credential id in result, got %q", string(jsonOut.Value))
	}
	if !strings.Contains(string(jsonOut.Value), `"id":"use_s_456"`) {
		t.Fatalf("expected use id in result, got %q", string(jsonOut.Value))
	}
	if strings.Contains(string(jsonOut.Value), "super-secret-token") {
		t.Fatalf("tool result leaked secret: %q", string(jsonOut.Value))
	}
}

func TestResolveRequestUserCredential_RejectionIncludesReason(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), "thread-1")

	result, err := e.ResolveAnswer(nil, message.ToolCallPart{
		ToolCallID: "tc-credential",
		ToolName:   "RequestUserCredential",
		Input:      `{"credentials":[{"envVar":"GITHUB_TOKEN","name":"GitHub access token","justification":"clone a private repo","approvedUses":[{"description":"create pull requests"}]}]}`,
	}, api.AnswerQuestionRequest{
		Answers: map[string]string{
			"__request_user_credential_rejected__":         "true",
			"__request_user_credential_rejection_reason__": "I don't want to expose that token.",
		},
	})
	if err != nil {
		t.Fatalf("ResolveAnswer returned error: %v", err)
	}

	textOut, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(textOut.Value, "will not supply") {
		t.Fatalf("expected rejection result, got %q", textOut.Value)
	}
	if !strings.Contains(textOut.Value, "I don't want to expose that token.") {
		t.Fatalf("expected rejection reason, got %q", textOut.Value)
	}
}

func TestExecuteRequestUserCredential_RequiresDurationValueForDurationKind(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), "thread-1")

	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "tc-credential",
		ToolName:   "RequestUserCredential",
		Input:      `{"credentials":[{"envVar":"GITHUB_TOKEN","name":"GitHub access token","justification":"","approvedUses":[{"description":"create pull requests"}]}]}`,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	errOut, ok := result.Result.Output.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(errOut.Value, "justification is required") {
		t.Fatalf("unexpected error output: %q", errOut.Value)
	}
}
func TestExecuteEnterPlanMode_SetsPlanMode(t *testing.T) {
	dataDir := t.TempDir()
	setTempHome(t)
	store := thread.NewStore(t.TempDir())
	agent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	e := New(t.TempDir(), dataDir, "thread-1")
	toolCtx := &thread.ToolContext{ThreadID: "thread-1", Agent: agent}

	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: "tc-enter",
		ToolName:   "EnterPlanMode",
		Input:      "{}",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Approval != nil {
		t.Fatal("EnterPlanMode should not require approval")
	}
	if !toolCtx.PlanMode {
		t.Fatal("expected PlanMode=true after EnterPlanMode")
	}
	if toolCtx.ModeChange == nil || *toolCtx.ModeChange != "plan" {
		t.Fatalf("expected ModeChange=plan, got %v", toolCtx.ModeChange)
	}

	cfg, err := store.LoadConfig("thread-1")
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if !strings.EqualFold(cfg.Mode.Value, "plan") {
		t.Fatal("expected persisted mode=plan after EnterPlanMode")
	}

	textOut, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput result, got %T", result.Result.Output)
	}
	prefix := "Plan mode activated. Plan file: "
	if !strings.HasPrefix(textOut.Value, prefix) {
		t.Fatalf("expected plan file prefix in output, got %q", textOut.Value)
	}
	firstLine := strings.SplitN(textOut.Value, "\n", 2)[0]
	planFile := strings.TrimPrefix(firstLine, prefix)
	if filepath.Ext(planFile) != ".md" {
		t.Fatalf("expected markdown plan file, got %q", planFile)
	}
	expectedPrefix := filepath.Join(dataDir, "threads", "thread-1", "plans") + string(filepath.Separator)
	if !strings.HasPrefix(planFile, expectedPrefix) {
		t.Fatalf("expected plan file under %q, got %q", expectedPrefix, planFile)
	}
	if strings.Contains(filepath.Base(planFile), " ") {
		t.Fatalf("expected LLM-friendly filename with no spaces, got %q", filepath.Base(planFile))
	}
}

func TestExecuteExitPlanMode_AutoApprovesWhenLLMEnteredPlanMode(t *testing.T) {
	dataDir := t.TempDir()
	home := setTempHome(t)
	threadID := "thread-1"
	planDir := filepath.Join(home, ".discobot", "plans", threadID)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "auto-plan.md"), []byte("## Auto plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := thread.NewStore(t.TempDir())
	agent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	if err := store.CreateThread(threadID); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfig(threadID, thread.Config{Mode: thread.ModeState{Value: "plan", SetBy: "llm"}}); err != nil {
		t.Fatal(err)
	}
	e := New(t.TempDir(), dataDir, threadID)
	toolCtx := &thread.ToolContext{
		ThreadID: threadID,
		PlanMode: true,
		Agent:    agent,
	}

	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: "tc-exit-auto",
		ToolName:   "ExitPlanMode",
		Input:      "{}",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Approval != nil {
		t.Fatal("expected ExitPlanMode to skip approval when prompt request mode was not plan")
	}
	if toolCtx.PlanMode {
		t.Fatal("expected PlanMode=false after auto-approved ExitPlanMode")
	}
	if toolCtx.ModeChange == nil || *toolCtx.ModeChange != "build" {
		t.Fatalf("expected ModeChange=build, got %v", toolCtx.ModeChange)
	}

	textOut, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(textOut.Value, "Plan mode exited") {
		t.Fatalf("expected auto-exit confirmation in output, got %q", textOut.Value)
	}
}

func TestExecuteExitPlanMode_RequiresApprovalWhenUserEnteredPlanMode(t *testing.T) {
	dataDir := t.TempDir()
	home := setTempHome(t)
	threadID := "thread-1"
	planDir := filepath.Join(home, ".discobot", "plans", threadID)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "manual-plan.md"), []byte("## Manual plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := thread.NewStore(t.TempDir())
	agent := agentimpl.NewDefaultAgent(store, nil, nil, t.TempDir(), agentimpl.MCPConfig{})
	if err := store.CreateThread(threadID); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfig(threadID, thread.Config{Mode: thread.ModeState{Value: "plan", SetBy: "user"}}); err != nil {
		t.Fatal(err)
	}
	e := New(t.TempDir(), dataDir, threadID)
	toolCtx := &thread.ToolContext{
		ThreadID: threadID,
		PlanMode: true,
		Agent:    agent,
	}

	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: "tc-exit-manual",
		ToolName:   "ExitPlanMode",
		Input:      "{}",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Approval == nil {
		t.Fatal("expected ExitPlanMode to request approval when prompt request mode was plan")
	}
	if !toolCtx.PlanMode {
		t.Fatal("expected PlanMode to remain true while waiting for approval")
	}
	if toolCtx.ModeChange != nil {
		t.Fatalf("expected ModeChange to be nil before approval, got %v", toolCtx.ModeChange)
	}

	var questions []map[string]any
	if err := json.Unmarshal(result.Approval.Questions, &questions); err != nil {
		t.Fatalf("failed to parse approval questions: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected 1 approval question, got %d", len(questions))
	}
	if questions[0]["header"] != "Plan approval" {
		t.Fatalf("expected Plan approval header, got %v", questions[0]["header"])
	}
}

func TestExecutePlanModeApplyPatch_AllowsPlanFileOnly(t *testing.T) {
	dataDir := t.TempDir()
	setTempHome(t)
	e := New(t.TempDir(), dataDir, "thread-1")
	planFile := filepath.Join(dataDir, "threads", "thread-1", "plans", "plan.md")
	toolCtx := &thread.ToolContext{
		ThreadID:     "thread-1",
		PlanMode:     true,
		PlanFilePath: planFile,
	}

	patch := "*** Begin Patch\n*** Add File: " + planFile + "\n+## Plan\n*** End Patch"
	raw, err := json.Marshal(map[string]any{"input": patch})
	if err != nil {
		t.Fatal(err)
	}

	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: "tc-plan-patch",
		ToolName:   "apply_patch",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	textOut, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(textOut.Value, "A "+planFile) {
		t.Fatalf("expected apply_patch success output, got %q", textOut.Value)
	}

	data, err := os.ReadFile(planFile)
	if err != nil {
		t.Fatalf("failed to read plan file: %v", err)
	}
	if string(data) != "## Plan\n" {
		t.Fatalf("unexpected plan file content: %q", string(data))
	}
}

func TestExecutePlanModeApplyPatch_RejectsNonPlanFile(t *testing.T) {
	dataDir := t.TempDir()
	setTempHome(t)
	cwd := t.TempDir()
	e := New(cwd, dataDir, "thread-1")
	planFile := filepath.Join(dataDir, "threads", "thread-1", "plans", "plan.md")
	toolCtx := &thread.ToolContext{
		ThreadID:     "thread-1",
		PlanMode:     true,
		PlanFilePath: planFile,
	}

	patch := "*** Begin Patch\n*** Add File: other.md\n+## Not the plan\n*** End Patch"
	raw, err := json.Marshal(map[string]any{"input": patch})
	if err != nil {
		t.Fatal(err)
	}

	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: "tc-other-patch",
		ToolName:   "apply_patch",
		Input:      string(raw),
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	errOut, ok := result.Result.Output.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(errOut.Value, "Write, Edit, or apply_patch") {
		t.Fatalf("expected updated plan-mode guidance, got %q", errOut.Value)
	}
	if !strings.Contains(errOut.Value, planFile) {
		t.Fatalf("expected plan file path in guidance, got %q", errOut.Value)
	}
}

func TestExecutePlanModeBlockedToolMessageMentionsApplyPatch(t *testing.T) {
	dataDir := t.TempDir()
	setTempHome(t)
	e := New(t.TempDir(), dataDir, "thread-1")
	planFile := filepath.Join(dataDir, "threads", "thread-1", "plans", "plan.md")
	toolCtx := &thread.ToolContext{
		ThreadID:     "thread-1",
		PlanMode:     true,
		PlanFilePath: planFile,
	}

	result, err := e.Execute(context.Background(), toolCtx, message.ToolCallPart{
		ToolCallID: "tc-bash",
		ToolName:   "Bash",
		Input:      "{}",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	errOut, ok := result.Result.Output.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput result, got %T", result.Result.Output)
	}
	if !strings.Contains(errOut.Value, "Write, Edit, and apply_patch are allowed for the plan file") {
		t.Fatalf("expected apply_patch in plan-mode guidance, got %q", errOut.Value)
	}
}
