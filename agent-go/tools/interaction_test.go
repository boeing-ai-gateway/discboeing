package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

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

func TestExecuteReadyForReviewSetsThreadPhase(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), "thread-1")
	var phase string

	result, err := e.Execute(context.Background(), &thread.ToolContext{
		SetThreadPhase: func(next string) error {
			phase = next
			return nil
		},
	}, message.ToolCallPart{
		ToolCallID: "tc-ready",
		ToolName:   "ReadyForReview",
		Input:      `{}`,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if phase != "review" {
		t.Fatalf("phase = %q, want review", phase)
	}
	output, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("output type = %T, want TextOutput", result.Result.Output)
	}
	if !strings.Contains(output.Value, "review") {
		t.Fatalf("output = %q, want review message", output.Value)
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
