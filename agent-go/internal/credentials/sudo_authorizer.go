package credentials

import (
	"context"
	"strings"
	"sync"

	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
)

const SudoTokenEnvVar = sudoauth.TokenEnvVar

// SudoAuthorizer validates sudo wrapper requests. Console requests are allowed
// only when they came from an interactive TTY. Agent requests must present a
// sudo-category token and approved use metadata injected by the Bash credential
// flow. Unlike normal credential use, sudo approval is already explicit, so this
// path validates the token locally and does not ask a model to re-judge it.
type SudoAuthorizer struct {
	credentialUseAuthorizer *CredentialUseAuthorizer
	credMgr                 *Manager
	consoleTokensMu         sync.RWMutex
	consoleTokens           map[string]struct{}
	bootstrapTokensMu       sync.RWMutex
	bootstrapTokens         map[string]struct{}
}

func NewSudoAuthorizer(credentialUseAuthorizer *CredentialUseAuthorizer, credMgr *Manager) *SudoAuthorizer {
	return &SudoAuthorizer{
		credentialUseAuthorizer: credentialUseAuthorizer,
		credMgr:                 credMgr,
		consoleTokens:           make(map[string]struct{}),
		bootstrapTokens:         make(map[string]struct{}),
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

func (a *SudoAuthorizer) RegisterBootstrapToken(token string) {
	token = strings.TrimSpace(token)
	if a == nil || token == "" {
		return
	}
	a.bootstrapTokensMu.Lock()
	defer a.bootstrapTokensMu.Unlock()
	a.bootstrapTokens[token] = struct{}{}
}

func (a *SudoAuthorizer) RevokeBootstrapToken(token string) {
	token = strings.TrimSpace(token)
	if a == nil || token == "" {
		return
	}
	a.bootstrapTokensMu.Lock()
	defer a.bootstrapTokensMu.Unlock()
	delete(a.bootstrapTokens, token)
}

func (a *SudoAuthorizer) validBootstrapToken(token string) bool {
	if a == nil || strings.TrimSpace(token) == "" {
		return false
	}
	a.bootstrapTokensMu.RLock()
	defer a.bootstrapTokensMu.RUnlock()
	_, ok := a.bootstrapTokens[token]
	return ok
}

func (a *SudoAuthorizer) AuthorizeSudo(ctx context.Context, req sudoauth.AuthorizeRequest) (sudoauth.AuthorizeResponse, error) {
	switch req.Runtime {
	case "bootstrap":
		if !a.validBootstrapToken(req.Token) {
			return sudoauth.AuthorizeResponse{Allow: false, Reason: "bootstrap sudo token is not valid"}, nil
		}
		return sudoauth.AuthorizeResponse{Allow: true, Reason: "bootstrap sudo is allowed"}, nil
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

func (a *SudoAuthorizer) authorizeAgentSudo(_ context.Context, req sudoauth.AuthorizeRequest) (sudoauth.AuthorizeResponse, error) {
	if a == nil || a.credMgr == nil {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo authorization is not configured"}, nil
	}
	if req.Token == "" || req.CredentialID == "" || req.UseID == "" {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo requires a RequestUserCredential approval token"}, nil
	}
	cred := a.credMgr.SessionCredentialForValue(req.CredentialID, req.Token)
	if cred == nil {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo token is not valid for this session"}, nil
	}
	if !cred.AgentVisible || cred.EnvVar != SudoTokenEnvVar || cred.Category != sudoauth.TokenCategory {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo token is not authorized for sudo"}, nil
	}
	if !credentialHasApprovedUse(cred, req.UseID) {
		return sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo use is not authorized for this token"}, nil
	}
	return sudoauth.AuthorizeResponse{Allow: true, Reason: "sudo token and approved use are valid"}, nil
}

func credentialHasApprovedUse(cred *EnvVar, useID string) bool {
	for _, use := range cred.Uses {
		if use.ID == useID {
			return true
		}
	}
	return false
}
