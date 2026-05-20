package credentials

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
	"github.com/obot-platform/discobot/agent-go/message"
)

const credentialUseValidationTimeout = 20 * time.Second

type CredentialUseAuthorizer struct {
	resolver AuthorizationModelResolver
	credMgr  *Manager
	prompt   string
}

type AuthorizationModelRef struct {
	ProviderID string
	ModelID    string
}

type AuthorizationModelResolver interface {
	ResolveAuthorizationModel(currentProviderID string) (AuthorizationModelRef, error)
	CompleteText(ctx context.Context, model AuthorizationModelRef, messages []message.Message, maxTokens *int) (string, error)
}

type credentialUseValidationResult struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

type credentialUseValidationEntry struct {
	Binding     CredentialUseBinding
	ApprovedUse AuthorizedUse
}

type CredentialUseBinding struct {
	CredentialID string
	UseID        string
	EnvVar       string
}

type credentialUseValidationRequest struct {
	ToolCallID          string                              `json:"toolCallId"`
	Command             string                              `json:"command"`
	CommandDescription  string                              `json:"commandDescription,omitempty"`
	BindingEnvVar       string                              `json:"bindingEnvVar"`
	CredentialSessionID string                              `json:"credentialSessionId"`
	CredentialProvider  string                              `json:"credentialProvider"`
	CredentialAuthType  string                              `json:"credentialAuthType"`
	ApprovedUses        []credentialUseValidationRequestUse `json:"approvedUses"`
}

type credentialUseValidationRequestUse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

func NewCredentialUseAuthorizer(resolver AuthorizationModelResolver, credMgr *Manager, prompt string) *CredentialUseAuthorizer {
	return &CredentialUseAuthorizer{resolver: resolver, credMgr: credMgr, prompt: prompt}
}

func (a *CredentialUseAuthorizer) Authorize(ctx context.Context, currentProviderID, toolCallID, command, description string, uses []CredentialUseBinding) error {
	groups := make(map[string]struct {
		cred    *EnvVar
		entries []credentialUseValidationEntry
	})
	order := make([]string, 0, len(uses))

	for _, use := range uses {
		cred := a.credMgr.SessionCredential(use.CredentialID)
		if cred == nil {
			return fmt.Errorf("credential id %s is not available in this session", use.CredentialID)
		}
		if !cred.AgentVisible {
			return fmt.Errorf("credential id %s is not visible to the agent in this session", use.CredentialID)
		}
		if cred.EnvVar != use.EnvVar {
			return fmt.Errorf("credential id %s is not authorized for environment variable %s", use.CredentialID, use.EnvVar)
		}

		var approvedUse *AuthorizedUse
		for i := range cred.Uses {
			if cred.Uses[i].ID == use.UseID {
				approvedUse = &cred.Uses[i]
				break
			}
		}
		if approvedUse == nil {
			return fmt.Errorf("credential use %s is not authorized for credential id %s", use.UseID, use.CredentialID)
		}
		if cred.Category == sudoauth.TokenCategory && cred.EnvVar == sudoauth.TokenEnvVar {
			continue
		}

		groupKey := use.CredentialID + "\x00" + use.EnvVar
		group, ok := groups[groupKey]
		if !ok {
			group.cred = cred
			order = append(order, groupKey)
		}
		group.entries = append(group.entries, credentialUseValidationEntry{
			Binding:     use,
			ApprovedUse: *approvedUse,
		})
		groups[groupKey] = group
	}

	for _, groupKey := range order {
		group := groups[groupKey]
		result, err := a.validateCommandAgainstApprovedUses(ctx, currentProviderID, toolCallID, command, description, *group.cred, group.entries)
		if err != nil {
			return err
		}
		if !result.Allow {
			reason := strings.TrimSpace(result.Reason)
			if reason == "" {
				reason = "the command does not match any approved use"
			}
			useIDs := make([]string, 0, len(group.entries))
			for _, entry := range group.entries {
				useIDs = append(useIDs, entry.Binding.UseID)
			}
			return fmt.Errorf("credential uses %s are not valid for this command: %s", strings.Join(useIDs, ", "), reason)
		}
	}
	return nil
}

func (a *CredentialUseAuthorizer) validateCommandAgainstApprovedUses(ctx context.Context, currentProviderID, toolCallID, command, description string, cred EnvVar, entries []credentialUseValidationEntry) (credentialUseValidationResult, error) {
	modelRef, err := a.resolver.ResolveAuthorizationModel(currentProviderID)
	if err != nil {
		return credentialUseValidationResult{}, fmt.Errorf("credential uses could not be validated: %w", err)
	}

	validationCtx, cancel := context.WithTimeout(ctx, credentialUseValidationTimeout)
	defer cancel()

	approvedUses := make([]credentialUseValidationRequestUse, 0, len(entries))
	for _, entry := range entries {
		approvedUses = append(approvedUses, credentialUseValidationRequestUse{
			ID:          entry.ApprovedUse.ID,
			Description: entry.ApprovedUse.Description,
		})
	}
	requestPayload := credentialUseValidationRequest{
		ToolCallID:          toolCallID,
		Command:             command,
		CommandDescription:  description,
		BindingEnvVar:       entries[0].Binding.EnvVar,
		CredentialSessionID: entries[0].Binding.CredentialID,
		CredentialProvider:  cred.Provider,
		CredentialAuthType:  cred.AuthType,
		ApprovedUses:        approvedUses,
	}
	requestJSON, err := json.Marshal(requestPayload)
	if err != nil {
		return credentialUseValidationResult{}, fmt.Errorf("credential uses could not be validated: %w", err)
	}

	maxTokens := 200
	respText, err := a.resolver.CompleteText(validationCtx, modelRef, []message.Message{
		{
			Role:  "system",
			Parts: []message.Part{message.TextPart{Text: a.prompt}},
		},
		{
			Role:  "user",
			Parts: []message.Part{message.TextPart{Text: string(requestJSON)}},
		},
	}, &maxTokens)
	if err != nil {
		return credentialUseValidationResult{}, fmt.Errorf("credential uses could not be validated: %w", err)
	}

	result, err := parseCredentialUseValidationResult(respText)
	if err != nil {
		return credentialUseValidationResult{}, fmt.Errorf("credential uses could not be validated: %w", err)
	}
	return result, nil
}

func parseCredentialUseValidationResult(raw string) (credentialUseValidationResult, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return credentialUseValidationResult{}, fmt.Errorf("validator returned empty output")
	}
	if body, ok := strings.CutPrefix(trimmed, "```"); ok {
		trimmed = strings.TrimSpace(strings.TrimPrefix(body, "json"))
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, "```"))
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		trimmed = trimmed[start : end+1]
	}
	var result credentialUseValidationResult
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return credentialUseValidationResult{}, fmt.Errorf("validator returned invalid JSON: %w", err)
	}
	return result, nil
}
