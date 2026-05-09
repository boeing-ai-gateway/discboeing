package credentials

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
)

const SudoTokenEnvVar = sudoauth.TokenEnvVar

// SudoAuthorizer validates sudo wrapper requests. Console requests are allowed
// only when they came from an interactive TTY. Agent requests must present the
// sudo token and approved use metadata injected by the Bash credential flow.
type SudoAuthorizer struct {
	credentialUseAuthorizer *CredentialUseAuthorizer
	credMgr                 *Manager
	consoleTokensMu         sync.RWMutex
	consoleTokens           map[string]struct{}
}

func NewSudoAuthorizer(credentialUseAuthorizer *CredentialUseAuthorizer, credMgr *Manager) *SudoAuthorizer {
	return &SudoAuthorizer{
		credentialUseAuthorizer: credentialUseAuthorizer,
		credMgr:                 credMgr,
		consoleTokens:           make(map[string]struct{}),
	}
}

func (a *SudoAuthorizer) RegisterConsoleToken(token string) {
	token = strings.TrimSpace(token)
	if a == nil || token == "" {
		return
	}
	a.consoleTokensMu.Lock()
	defer a.consoleTokensMu.Unlock()
	a.consoleTokens[token] = struct{}{}
}

func (a *SudoAuthorizer) RevokeConsoleToken(token string) {
	token = strings.TrimSpace(token)
	if a == nil || token == "" {
		return
	}
	a.consoleTokensMu.Lock()
	defer a.consoleTokensMu.Unlock()
	delete(a.consoleTokens, token)
}

func (a *SudoAuthorizer) validConsoleToken(token string) bool {
	if a == nil || strings.TrimSpace(token) == "" {
		return false
	}
	a.consoleTokensMu.RLock()
	defer a.consoleTokensMu.RUnlock()
	_, ok := a.consoleTokens[token]
	return ok
}

func (a *SudoAuthorizer) AuthorizeSudo(ctx context.Context, req sudoauth.AuthorizeRequest) (sudoauth.AuthorizeResponse, error) {
	switch req.Runtime {
	case "console":
		if !req.TTY {
			return sudoauth.AuthorizeResponse{Allow: false, Reason: "console sudo requires an interactive terminal"}, nil
		}
		if !a.validConsoleToken(req.Token) {
			return sudoauth.AuthorizeResponse{Allow: false, Reason: "console sudo token is not valid"}, nil
		}
		return sudoauth.AuthorizeResponse{Allow: true, Reason: "interactive console sudo is allowed"}, nil
	case "agent":
		return a.authorizeAgentSudo(ctx, req)
	default:
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo requires an approved runtime token"}, nil
	}
}

func (a *SudoAuthorizer) authorizeAgentSudo(ctx context.Context, req sudoauth.AuthorizeRequest) (sudoauth.AuthorizeResponse, error) {
	if a == nil || a.credentialUseAuthorizer == nil || a.credMgr == nil {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo authorization is not configured"}, nil
	}
	if req.Token == "" || req.CredentialID == "" || req.UseID == "" {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo requires a RequestUserCredential approval token"}, nil
	}
	cred := a.credMgr.SessionCredentialForValue(req.CredentialID, req.Token)
	if cred == nil {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo token is not valid for this session"}, nil
	}
	if !cred.AgentVisible || cred.EnvVar != SudoTokenEnvVar {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo token is not agent-visible for sudo"}, nil
	}

	command := sudoAuthorizationCommand(req)
	if err := a.credentialUseAuthorizer.Authorize(ctx, "", req.ToolCallID, command, "Authorize sudo privilege escalation", []CredentialUseBinding{{
		CredentialID: req.CredentialID,
		UseID:        req.UseID,
		EnvVar:       SudoTokenEnvVar,
	}}); err != nil {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: err.Error()}, nil
	}
	return sudoauth.AuthorizeResponse{Allow: true, Reason: "sudo use matches the approved credential use"}, nil
}

func sudoAuthorizationCommand(req sudoauth.AuthorizeRequest) string {
	parts := make([]string, 0, len(req.Args)+2)
	parts = append(parts, "sudo")
	parts = append(parts, req.Args...)
	cmd := strings.Join(parts, " ")
	if strings.TrimSpace(req.Command) == "" {
		return fmt.Sprintf("%s\ncwd: %s", cmd, req.Cwd)
	}
	return fmt.Sprintf("%s\ncwd: %s\noriginal Bash command: %s", cmd, req.Cwd, req.Command)
}
