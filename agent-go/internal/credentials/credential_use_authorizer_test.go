package credentials

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
	"github.com/obot-platform/discobot/agent-go/message"
)

type credentialValidationMockResolver struct {
	response string
	requests []credentialValidationMockRequest
	models   map[string]AuthorizationModelRef
}

type credentialValidationMockRequest struct {
	Model     AuthorizationModelRef
	Messages  []message.Message
	MaxTokens *int
}

func (r *credentialValidationMockResolver) ResolveAuthorizationModel(currentProviderID string) (AuthorizationModelRef, error) {
	for _, taskType := range []string{"authorization", "chat"} {
		if model, ok := r.models[currentProviderID+"\x00"+taskType]; ok {
			return model, nil
		}
	}
	if model, ok := r.models["\x00chat"]; ok {
		return model, nil
	}
	return AuthorizationModelRef{}, context.DeadlineExceeded
}

func (r *credentialValidationMockResolver) CompleteText(_ context.Context, model AuthorizationModelRef, messages []message.Message, maxTokens *int) (string, error) {
	r.requests = append(r.requests, credentialValidationMockRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
	})
	response := r.response
	if response == "" {
		response = `{"allow":true,"reason":"ok"}`
	}
	return response, nil
}

func applyCredentialHeader(t *testing.T, mgr *Manager, approvedUses []AuthorizedUse) {
	t.Helper()
	headerValue, err := json.Marshal([]map[string]any{
		{
			"sessionCredentialId": "session-cred-1",
			"credentialId":        "cred-1",
			"uses":                approvedUses,
			"envVar":              "GITHUB_TOKEN",
			"value":               "secret",
			"provider":            "github-git",
			"authType":            "oauth",
			"agentVisible":        true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr.Apply(string(headerValue), "", "")
}

func messageText(t *testing.T, msg message.Message) string {
	t.Helper()
	if len(msg.Parts) != 1 {
		t.Fatalf("expected one message part, got %d", len(msg.Parts))
	}
	part, ok := msg.Parts[0].(message.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", msg.Parts[0])
	}
	return part.Text
}

func TestCredentialUseAuthorizer_AllowsValidatedCommand(t *testing.T) {
	mgr := NewManager()
	applyCredentialHeader(t, mgr, []AuthorizedUse{{ID: "use-1", Description: "create pull requests with gh"}})

	resolver := &credentialValidationMockResolver{
		response: `{"allow":true,"reason":"matches approved gh PR creation"}`,
		models: map[string]AuthorizationModelRef{
			"mock\x00chat": {ProviderID: "mock", ModelID: "validator-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err := authorizer.Authorize(context.Background(), "mock", "tool-call-1", "gh pr create --fill", "open a pull request", []CredentialUseBinding{{
		CredentialID: "session-cred-1",
		UseID:        "use-1",
		EnvVar:       "GITHUB_TOKEN",
	}})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(resolver.requests) != 1 {
		t.Fatalf("expected 1 validation request, got %d", len(resolver.requests))
	}
	payload := messageText(t, resolver.requests[0].Messages[1])
	if !strings.Contains(payload, `"approvedUses":[{"id":"use-1","description":"create pull requests with gh"}]`) {
		t.Fatalf("validation payload missing approved uses: %s", payload)
	}
	if !strings.Contains(payload, `"command":"gh pr create --fill"`) {
		t.Fatalf("validation payload missing command: %s", payload)
	}
}

func TestCredentialUseAuthorizer_RejectsExpiredApprovedUse(t *testing.T) {
	mgr := NewManager()
	expiredAt := time.Now().Add(-time.Minute)
	applyCredentialHeader(t, mgr, []AuthorizedUse{{
		ID:          "use-1",
		Description: "create pull requests with gh",
		ExpiresAt:   &expiredAt,
	}})

	resolver := &credentialValidationMockResolver{
		models: map[string]AuthorizationModelRef{
			"mock\x00chat": {ProviderID: "mock", ModelID: "validator-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err := authorizer.Authorize(context.Background(), "mock", "tool-call-1", "gh pr create --fill", "open a pull request", []CredentialUseBinding{{
		CredentialID: "session-cred-1",
		UseID:        "use-1",
		EnvVar:       "GITHUB_TOKEN",
	}})
	if err == nil {
		t.Fatal("expected expired use rejection")
	}
	if !strings.Contains(err.Error(), "has expired") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolver.requests) != 0 {
		t.Fatalf("expected expired use to skip model validation, got %d requests", len(resolver.requests))
	}
}

func TestCredentialUseAuthorizer_SkipsModelForSudoToken(t *testing.T) {
	mgr := NewManager()
	headerValue, err := json.Marshal([]map[string]any{
		{
			"sessionCredentialId": "session-sudo-1",
			"credentialId":        "cred-sudo-1",
			"uses":                []AuthorizedUse{{ID: "use-sudo-1", Description: "install apt packages"}},
			"category":            sudoauth.TokenCategory,
			"envVar":              sudoauth.TokenEnvVar,
			"value":               "sudo-token",
			"provider":            "discobot",
			"authType":            "approval",
			"agentVisible":        true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr.Apply(string(headerValue), "", "")

	resolver := &credentialValidationMockResolver{
		models: map[string]AuthorizationModelRef{
			"mock\x00chat": {ProviderID: "mock", ModelID: "validator-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err = authorizer.Authorize(context.Background(), "mock", "tool-call-1", "sudo apt-get update", "run sudo", []CredentialUseBinding{{
		CredentialID: "session-sudo-1",
		UseID:        "use-sudo-1",
		EnvVar:       sudoauth.TokenEnvVar,
	}})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(resolver.requests) != 0 {
		t.Fatalf("expected sudo token to skip model validation, got %d requests", len(resolver.requests))
	}
}

func TestCredentialUseAuthorizer_AllowsCommandWhenAnyApprovedUseMatches(t *testing.T) {
	mgr := NewManager()
	applyCredentialHeader(t, mgr, []AuthorizedUse{
		{ID: "use-1", Description: "clone private repositories"},
		{ID: "use-2", Description: "create pull requests with gh"},
	})

	resolver := &credentialValidationMockResolver{
		response: `{"allow":true,"reason":"the command matches the PR creation use even though it does not match the clone use"}`,
		models: map[string]AuthorizationModelRef{
			"mock\x00chat": {ProviderID: "mock", ModelID: "validator-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err := authorizer.Authorize(context.Background(), "mock", "tool-call-1", "gh pr create --fill", "open a pull request", []CredentialUseBinding{
		{CredentialID: "session-cred-1", UseID: "use-1", EnvVar: "GITHUB_TOKEN"},
		{CredentialID: "session-cred-1", UseID: "use-2", EnvVar: "GITHUB_TOKEN"},
	})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(resolver.requests) != 1 {
		t.Fatalf("expected one grouped validation request, got %d", len(resolver.requests))
	}
	payload := messageText(t, resolver.requests[0].Messages[1])
	if !strings.Contains(payload, `"approvedUses":[{"id":"use-1","description":"clone private repositories"},{"id":"use-2","description":"create pull requests with gh"}]`) {
		t.Fatalf("grouped validation payload missing approved uses: %s", payload)
	}
}

func TestCredentialUseAuthorizer_RejectsInvalidCommand(t *testing.T) {
	mgr := NewManager()
	applyCredentialHeader(t, mgr, []AuthorizedUse{{ID: "use-1", Description: "create pull requests with gh"}})

	resolver := &credentialValidationMockResolver{
		response: `{"allow":false,"reason":"the command pushes tags, which is broader than creating a pull request"}`,
		models: map[string]AuthorizationModelRef{
			"mock\x00chat": {ProviderID: "mock", ModelID: "validator-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err := authorizer.Authorize(context.Background(), "mock", "tool-call-1", "git push --tags", "push git refs", []CredentialUseBinding{{
		CredentialID: "session-cred-1",
		UseID:        "use-1",
		EnvVar:       "GITHUB_TOKEN",
	}})
	if err == nil {
		t.Fatal("expected rejection error")
	}
	if !strings.Contains(err.Error(), "credential uses use-1 are not valid for this command") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "broader than creating a pull request") {
		t.Fatalf("expected validator reason in error, got %v", err)
	}
}

func TestCredentialUseAuthorizer_PrefersAuthorizationModel(t *testing.T) {
	mgr := NewManager()
	applyCredentialHeader(t, mgr, []AuthorizedUse{{ID: "use-1", Description: "create pull requests with gh"}})

	resolver := &credentialValidationMockResolver{
		response: `{"allow":true,"reason":"ok"}`,
		models: map[string]AuthorizationModelRef{
			"mock\x00authorization": {ProviderID: "mock", ModelID: "validator-authorization-model"},
			"mock\x00chat":          {ProviderID: "mock", ModelID: "validator-chat-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err := authorizer.Authorize(context.Background(), "mock", "tool-call-1", "gh pr create --fill", "open a pull request", []CredentialUseBinding{{
		CredentialID: "session-cred-1",
		UseID:        "use-1",
		EnvVar:       "GITHUB_TOKEN",
	}})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(resolver.requests) != 1 {
		t.Fatalf("expected 1 validation request, got %d", len(resolver.requests))
	}
	if resolver.requests[0].Model.ModelID != "validator-authorization-model" {
		t.Fatalf("expected authorization model, got %#v", resolver.requests[0].Model)
	}
}

func TestCredentialUseAuthorizer_FallsBackToChatModel(t *testing.T) {
	mgr := NewManager()
	applyCredentialHeader(t, mgr, []AuthorizedUse{{ID: "use-1", Description: "create pull requests with gh"}})

	resolver := &credentialValidationMockResolver{
		response: `{"allow":true,"reason":"ok"}`,
		models: map[string]AuthorizationModelRef{
			"mock\x00chat": {ProviderID: "mock", ModelID: "validator-chat-model"},
		},
	}

	authorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	err := authorizer.Authorize(context.Background(), "mock", "tool-call-1", "gh pr create --fill", "open a pull request", []CredentialUseBinding{{
		CredentialID: "session-cred-1",
		UseID:        "use-1",
		EnvVar:       "GITHUB_TOKEN",
	}})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if len(resolver.requests) != 1 {
		t.Fatalf("expected 1 validation request, got %d", len(resolver.requests))
	}
	if resolver.requests[0].Model.ModelID != "validator-chat-model" {
		t.Fatalf("expected chat model fallback, got %#v", resolver.requests[0].Model)
	}
}

func TestParseCredentialUseValidationResult_StripsCodeFences(t *testing.T) {
	result, err := parseCredentialUseValidationResult("```json\n{\"allow\":false,\"reason\":\"no\"}\n```")
	if err != nil {
		t.Fatalf("parseCredentialUseValidationResult() error = %v", err)
	}
	if result.Allow {
		t.Fatal("expected allow=false")
	}
	if result.Reason != "no" {
		t.Fatalf("unexpected reason %q", result.Reason)
	}
}
