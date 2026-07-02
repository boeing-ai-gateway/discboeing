package sessionconfig

import (
	"strings"

	"github.com/boeing-ai-gateway/discboeing/agent-go/providers"
)

type skillFrontmatter struct {
	Name                     string `yaml:"name"`
	Description              string `yaml:"description"`
	discboeingMetadataEnvelope `yaml:",inline"`
}

type scriptFrontmatter struct {
	Name                     string `yaml:"name"`
	Description              string `yaml:"description"`
	Visible                  *bool  `yaml:"visible"`
	ArgumentHint             string `yaml:"argument-hint"`
	discboeingMetadataEnvelope `yaml:",inline"`
}

type subAgentFrontmatter struct {
	Name             string                     `yaml:"name"`
	Description      string                     `yaml:"description"`
	Model            string                     `yaml:"model"`
	SupportingModels providers.SupportingModels `yaml:"supporting-models"`
	AllowedTools     []string                   `yaml:"allowed-tools"`
	DisallowedTools  []string                   `yaml:"disallowed-tools"`
	MaxTurns         int                        `yaml:"max-turns"`
}

type systemPromptFrontmatter struct {
	AllowedTools []string `yaml:"allowed-tools"`
}

type discboeingCommandMetadataFrontmatter struct {
	UI                *bool                                  `yaml:"ui"`
	Label             string                                 `yaml:"label"`
	ActiveLabel       string                                 `yaml:"active-label"`
	Icon              string                                 `yaml:"icon"`
	Group             string                                 `yaml:"group"`
	Order             *int                                   `yaml:"order"`
	CredentialRequest []discboeingCredentialRequestFrontmatter `yaml:"credential-request"`
}

type discboeingCredentialRequestFrontmatter struct {
	EnvVar        string                                     `yaml:"env-var"`
	Name          string                                     `yaml:"name"`
	Justification string                                     `yaml:"justification"`
	ApprovedUses  []discboeingCredentialApprovedUseFrontmatter `yaml:"approved-uses"`
}

type discboeingCredentialApprovedUseFrontmatter struct {
	Description string `yaml:"description"`
}

type discboeingMetadataEnvelope struct {
	Discboeing                  discboeingCommandMetadataFrontmatter     `yaml:"discboeing"`
	LegacyDiscboeingUI          *bool                                  `yaml:"discboeing-ui"`
	LegacyDiscboeingLabel       string                                 `yaml:"discboeing-label"`
	LegacyDiscboeingActiveLabel string                                 `yaml:"discboeing-active-label"`
	LegacyDiscboeingIcon        string                                 `yaml:"discboeing-icon"`
	LegacyDiscboeingGroup       string                                 `yaml:"discboeing-group"`
	LegacyDiscboeingOrder       *int                                   `yaml:"discboeing-order"`
	LegacyCredentialRequest   []discboeingCredentialRequestFrontmatter `yaml:"discboeing-credential-request"`
}

func (fm discboeingMetadataEnvelope) discboeingMetadata() DiscboeingCommandMetadata {
	meta := DiscboeingCommandMetadata{}
	if fm.Discboeing.UI != nil {
		meta.UI = *fm.Discboeing.UI
	}
	if fm.Discboeing.Label != "" {
		meta.Label = strings.TrimSpace(fm.Discboeing.Label)
	}
	if fm.Discboeing.ActiveLabel != "" {
		meta.ActiveLabel = strings.TrimSpace(fm.Discboeing.ActiveLabel)
	}
	if fm.Discboeing.Icon != "" {
		meta.Icon = strings.TrimSpace(fm.Discboeing.Icon)
	}
	if fm.Discboeing.Group != "" {
		meta.Group = strings.TrimSpace(fm.Discboeing.Group)
	}
	if fm.Discboeing.Order != nil {
		meta.Order = *fm.Discboeing.Order
	}
	meta.CredentialRequest = convertCredentialRequests(fm.Discboeing.CredentialRequest)

	if fm.LegacyDiscboeingUI != nil {
		meta.UI = *fm.LegacyDiscboeingUI
	}
	if fm.LegacyDiscboeingLabel != "" {
		meta.Label = strings.TrimSpace(fm.LegacyDiscboeingLabel)
	}
	if fm.LegacyDiscboeingActiveLabel != "" {
		meta.ActiveLabel = strings.TrimSpace(fm.LegacyDiscboeingActiveLabel)
	}
	if fm.LegacyDiscboeingIcon != "" {
		meta.Icon = strings.TrimSpace(fm.LegacyDiscboeingIcon)
	}
	if fm.LegacyDiscboeingGroup != "" {
		meta.Group = strings.TrimSpace(fm.LegacyDiscboeingGroup)
	}
	if fm.LegacyDiscboeingOrder != nil {
		meta.Order = *fm.LegacyDiscboeingOrder
	}
	if len(fm.LegacyCredentialRequest) > 0 {
		meta.CredentialRequest = convertCredentialRequests(fm.LegacyCredentialRequest)
	}
	return meta
}

func convertCredentialRequests(items []discboeingCredentialRequestFrontmatter) []DiscboeingCredentialRequest {
	if len(items) == 0 {
		return nil
	}
	requests := make([]DiscboeingCredentialRequest, 0, len(items))
	for _, item := range items {
		request := DiscboeingCredentialRequest{
			EnvVar:        strings.TrimSpace(item.EnvVar),
			Name:          strings.TrimSpace(item.Name),
			Justification: strings.TrimSpace(item.Justification),
		}
		if len(item.ApprovedUses) > 0 {
			request.ApprovedUses = make([]DiscboeingCredentialApprovedUse, 0, len(item.ApprovedUses))
			for _, use := range item.ApprovedUses {
				description := strings.TrimSpace(use.Description)
				if description == "" {
					continue
				}
				request.ApprovedUses = append(request.ApprovedUses, DiscboeingCredentialApprovedUse{Description: description})
			}
		}
		requests = append(requests, request)
	}
	return requests
}
