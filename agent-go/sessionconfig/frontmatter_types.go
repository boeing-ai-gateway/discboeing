package sessionconfig

import (
	"strings"

	"github.com/obot-platform/discobot/agent-go/providers"
)

type skillFrontmatter struct {
	Name                     string `yaml:"name"`
	Description              string `yaml:"description"`
	discobotMetadataEnvelope `yaml:",inline"`
}

type scriptFrontmatter struct {
	Name                     string `yaml:"name"`
	Description              string `yaml:"description"`
	Visible                  *bool  `yaml:"visible"`
	ArgumentHint             string `yaml:"argument-hint"`
	discobotMetadataEnvelope `yaml:",inline"`
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

type discobotCommandMetadataFrontmatter struct {
	UI                *bool                                  `yaml:"ui"`
	Label             string                                 `yaml:"label"`
	ActiveLabel       string                                 `yaml:"active-label"`
	Icon              string                                 `yaml:"icon"`
	Group             string                                 `yaml:"group"`
	Order             *int                                   `yaml:"order"`
	CredentialRequest []discobotCredentialRequestFrontmatter `yaml:"credential-request"`
}

type discobotCredentialRequestFrontmatter struct {
	EnvVar        string                                     `yaml:"env-var"`
	Name          string                                     `yaml:"name"`
	Justification string                                     `yaml:"justification"`
	ApprovedUses  []discobotCredentialApprovedUseFrontmatter `yaml:"approved-uses"`
}

type discobotCredentialApprovedUseFrontmatter struct {
	Description string `yaml:"description"`
}

type discobotMetadataEnvelope struct {
	Discobot                  discobotCommandMetadataFrontmatter     `yaml:"discobot"`
	LegacyDiscobotUI          *bool                                  `yaml:"discobot-ui"`
	LegacyDiscobotLabel       string                                 `yaml:"discobot-label"`
	LegacyDiscobotActiveLabel string                                 `yaml:"discobot-active-label"`
	LegacyDiscobotIcon        string                                 `yaml:"discobot-icon"`
	LegacyDiscobotGroup       string                                 `yaml:"discobot-group"`
	LegacyDiscobotOrder       *int                                   `yaml:"discobot-order"`
	LegacyCredentialRequest   []discobotCredentialRequestFrontmatter `yaml:"discobot-credential-request"`
}

func (fm discobotMetadataEnvelope) discobotMetadata() DiscobotCommandMetadata {
	meta := DiscobotCommandMetadata{}
	if fm.Discobot.UI != nil {
		meta.UI = *fm.Discobot.UI
	}
	if fm.Discobot.Label != "" {
		meta.Label = strings.TrimSpace(fm.Discobot.Label)
	}
	if fm.Discobot.ActiveLabel != "" {
		meta.ActiveLabel = strings.TrimSpace(fm.Discobot.ActiveLabel)
	}
	if fm.Discobot.Icon != "" {
		meta.Icon = strings.TrimSpace(fm.Discobot.Icon)
	}
	if fm.Discobot.Group != "" {
		meta.Group = strings.TrimSpace(fm.Discobot.Group)
	}
	if fm.Discobot.Order != nil {
		meta.Order = *fm.Discobot.Order
	}
	meta.CredentialRequest = convertCredentialRequests(fm.Discobot.CredentialRequest)

	if fm.LegacyDiscobotUI != nil {
		meta.UI = *fm.LegacyDiscobotUI
	}
	if fm.LegacyDiscobotLabel != "" {
		meta.Label = strings.TrimSpace(fm.LegacyDiscobotLabel)
	}
	if fm.LegacyDiscobotActiveLabel != "" {
		meta.ActiveLabel = strings.TrimSpace(fm.LegacyDiscobotActiveLabel)
	}
	if fm.LegacyDiscobotIcon != "" {
		meta.Icon = strings.TrimSpace(fm.LegacyDiscobotIcon)
	}
	if fm.LegacyDiscobotGroup != "" {
		meta.Group = strings.TrimSpace(fm.LegacyDiscobotGroup)
	}
	if fm.LegacyDiscobotOrder != nil {
		meta.Order = *fm.LegacyDiscobotOrder
	}
	if len(fm.LegacyCredentialRequest) > 0 {
		meta.CredentialRequest = convertCredentialRequests(fm.LegacyCredentialRequest)
	}
	return meta
}

func convertCredentialRequests(items []discobotCredentialRequestFrontmatter) []DiscobotCredentialRequest {
	if len(items) == 0 {
		return nil
	}
	requests := make([]DiscobotCredentialRequest, 0, len(items))
	for _, item := range items {
		request := DiscobotCredentialRequest{
			EnvVar:        strings.TrimSpace(item.EnvVar),
			Name:          strings.TrimSpace(item.Name),
			Justification: strings.TrimSpace(item.Justification),
		}
		if len(item.ApprovedUses) > 0 {
			request.ApprovedUses = make([]DiscobotCredentialApprovedUse, 0, len(item.ApprovedUses))
			for _, use := range item.ApprovedUses {
				description := strings.TrimSpace(use.Description)
				if description == "" {
					continue
				}
				request.ApprovedUses = append(request.ApprovedUses, DiscobotCredentialApprovedUse{Description: description})
			}
		}
		requests = append(requests, request)
	}
	return requests
}
