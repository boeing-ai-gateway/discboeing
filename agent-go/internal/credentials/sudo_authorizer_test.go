package credentials

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/sudoauth"
)

func applySudoCredentialHeader(t *testing.T, mgr *Manager) {
	t.Helper()
	headerValue, err := json.Marshal([]map[string]any{{
		"sessionCredentialId": "session-sudo-1",
		"credentialId":        "cred-sudo-1",
		"uses":                []AuthorizedUse{{ID: "use-sudo-1", Description: "install apt packages needed for the build"}},
		"category":            sudoauth.TokenCategory,
		"envVar":              SudoTokenEnvVar,
		"value":               "sudo-token",
		"provider":            "discboeing",
		"authType":            "approval",
		"agentVisible":        true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	mgr.Apply(string(headerValue), "", "")
}

func TestSudoAuthorizer_AllowsInteractiveConsoleWithRegisteredToken(t *testing.T) {
	authorizer := NewSudoAuthorizer(nil, nil)
	authorizer.RegisterConsoleToken("0123456789abcdef0123456789abcdef")
	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{Runtime: "console", Token: "0123456789abcdef0123456789abcdef", TTY: true})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if !resp.Allow {
		t.Fatalf("expected console sudo to be allowed, got %#v", resp)
	}
}

func TestSudoAuthorizer_RejectsRevokedConsoleToken(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	authorizer := NewSudoAuthorizer(nil, nil)
	authorizer.RegisterConsoleToken(token)
	authorizer.RevokeConsoleToken(token)

	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{Runtime: "console", Token: token, TTY: true})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if resp.Allow || !strings.Contains(resp.Reason, "not valid") {
		t.Fatalf("expected revoked console token rejection, got %#v", resp)
	}
}

func TestSudoAuthorizer_RejectsConsoleWithoutRegisteredToken(t *testing.T) {
	authorizer := NewSudoAuthorizer(nil, nil)
	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{Runtime: "console", Token: "console", TTY: true})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if resp.Allow || !strings.Contains(resp.Reason, "not valid") {
		t.Fatalf("expected console token rejection, got %#v", resp)
	}
}

func TestSudoAuthorizer_AllowsRegisteredBootstrapToken(t *testing.T) {
	authorizer := NewSudoAuthorizer(nil, nil)
	authorizer.RegisterBootstrapToken("bootstrap-token")
	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{Runtime: "bootstrap", Token: "bootstrap-token"})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if !resp.Allow {
		t.Fatalf("expected bootstrap sudo to be allowed, got %#v", resp)
	}
}

func TestSudoAuthorizer_RejectsRevokedBootstrapToken(t *testing.T) {
	authorizer := NewSudoAuthorizer(nil, nil)
	authorizer.RegisterBootstrapToken("bootstrap-token")
	authorizer.RevokeBootstrapToken("bootstrap-token")
	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{Runtime: "bootstrap", Token: "bootstrap-token"})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if resp.Allow || !strings.Contains(resp.Reason, "not valid") {
		t.Fatalf("expected bootstrap token rejection, got %#v", resp)
	}
}

func TestSudoAuthorizer_RejectsAgentWithoutToken(t *testing.T) {
	authorizer := NewSudoAuthorizer(nil, nil)
	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{Runtime: "agent"})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if resp.Allow || !strings.Contains(resp.Reason, "not configured") {
		t.Fatalf("expected configured rejection, got %#v", resp)
	}
}

func TestSudoAuthorizer_AllowsApprovedAgentSudo(t *testing.T) {
	mgr := NewManager()
	applySudoCredentialHeader(t, mgr)
	resolver := &credentialValidationMockResolver{
		response: `{"allow":true,"reason":"matches approved sudo use"}`,
		models: map[string]AuthorizationModelRef{
			"\x00chat": {ProviderID: "mock", ModelID: "validator-model"},
		},
	}
	credentialAuthorizer := NewCredentialUseAuthorizer(resolver, mgr, "validator prompt")
	authorizer := NewSudoAuthorizer(credentialAuthorizer, mgr)

	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{
		Runtime:      "agent",
		Token:        "sudo-token",
		CredentialID: "session-sudo-1",
		UseID:        "use-sudo-1",
		ToolCallID:   "tool-1",
		Args:         []string{"apt-get", "update"},
		Cwd:          "/workspace",
	})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if !resp.Allow {
		t.Fatalf("expected approved sudo, got %#v", resp)
	}
	if len(resolver.requests) != 0 {
		t.Fatalf("expected sudo authorization to skip model validation, got %d requests", len(resolver.requests))
	}
}

func TestSudoAuthorizer_RejectsExpiredAgentSudoUse(t *testing.T) {
	mgr := NewManager()
	expiredAt := time.Now().Add(-time.Minute)
	headerValue, err := json.Marshal([]map[string]any{{
		"sessionCredentialId": "session-sudo-1",
		"credentialId":        "cred-sudo-1",
		"uses": []AuthorizedUse{{
			ID:          "use-sudo-1",
			Description: "install apt packages needed for the build",
			ExpiresAt:   &expiredAt,
		}},
		"category":     sudoauth.TokenCategory,
		"envVar":       SudoTokenEnvVar,
		"value":        "sudo-token",
		"provider":     "discboeing",
		"authType":     "approval",
		"agentVisible": true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	mgr.Apply(string(headerValue), "", "")
	authorizer := NewSudoAuthorizer(nil, mgr)

	resp, err := authorizer.AuthorizeSudo(context.Background(), sudoauth.AuthorizeRequest{
		Runtime:      "agent",
		Token:        "sudo-token",
		CredentialID: "session-sudo-1",
		UseID:        "use-sudo-1",
		ToolCallID:   "tool-1",
		Args:         []string{"apt-get", "update"},
		Cwd:          "/workspace",
	})
	if err != nil {
		t.Fatalf("AuthorizeSudo() error = %v", err)
	}
	if resp.Allow || !strings.Contains(resp.Reason, "expired") {
		t.Fatalf("expected expired sudo use rejection, got %#v", resp)
	}
}
