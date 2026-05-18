package toolrenderers

import (
	"encoding/json"
	"strings"
)

type RequestUserCredentialView struct {
	Input            string
	Output           string
	ErrorText        string
	State            string
	Open             bool
	Raw              bool
	Queued           bool
	ApprovalStatus   string
	ApprovalError    string
	RejectionSummary string
}

type RequestedCredentialView struct {
	Name          string                       `json:"name"`
	EnvVar        string                       `json:"envVar"`
	Justification string                       `json:"justification"`
	ApprovedUses  []RequestedCredentialUseView `json:"approvedUses"`
}

type RequestedCredentialUseView struct {
	Description string `json:"description"`
}

type requestUserCredentialInput struct {
	Credentials []RequestedCredentialView `json:"credentials"`
}

type GrantedCredentialView struct {
	Name         string                     `json:"name"`
	EnvVar       string                     `json:"envVar"`
	CredentialID string                     `json:"credentialId"`
	Uses         []GrantedCredentialUseView `json:"uses"`
	ApprovedUses []GrantedCredentialUseView `json:"approvedUses"`
}

type GrantedCredentialUseView struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	ExpiresAt   string `json:"expiresAt"`
}

type requestUserCredentialOutput struct {
	GrantedCredentials []GrantedCredentialView `json:"grantedCredentials"`
}

type requestUserCredentialGrantedDetail struct {
	Granted        GrantedCredentialView
	Request        RequestedCredentialView
	CredentialName string
	Justification  string
	IsSudo         bool
}

const (
	requestUserCredentialRejectedPrefix       = "The user rejected the credential request."
	requestUserCredentialRejectedReasonPrefix = requestUserCredentialRejectedPrefix + " Reason: "
	sudoTokenEnvVar                           = "DISCOBOT_SUDO_TOKEN"
)

func parseRequestUserCredentialInput(input string) (requestUserCredentialInput, bool) {
	if strings.TrimSpace(input) == "" {
		return requestUserCredentialInput{}, false
	}
	var parsed requestUserCredentialInput
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		return requestUserCredentialInput{}, false
	}
	return parsed, len(parsed.Credentials) > 0
}

func parseRequestUserCredentialOutput(output string) (requestUserCredentialOutput, bool) {
	if strings.TrimSpace(output) == "" {
		return requestUserCredentialOutput{}, false
	}
	var parsed requestUserCredentialOutput
	if err := json.Unmarshal([]byte(output), &parsed); err == nil {
		return parsed, len(parsed.GrantedCredentials) > 0
	}
	return requestUserCredentialOutput{}, false
}

func requestUserCredentialApprovedUses(request RequestedCredentialView) []string {
	uses := make([]string, 0, len(request.ApprovedUses))
	for _, use := range request.ApprovedUses {
		if trimmed := strings.TrimSpace(use.Description); trimmed != "" {
			uses = append(uses, trimmed)
		}
	}
	return uses
}

func requestUserCredentialIsSudo(request RequestedCredentialView) bool {
	return strings.TrimSpace(request.EnvVar) == sudoTokenEnvVar
}

func requestUserCredentialIsSudoOnly(credentials []RequestedCredentialView) bool {
	if len(credentials) == 0 {
		return false
	}
	for _, credential := range credentials {
		if !requestUserCredentialIsSudo(credential) {
			return false
		}
	}
	return true
}

func requestUserCredentialName(request RequestedCredentialView) string {
	if strings.TrimSpace(request.Name) != "" {
		return strings.TrimSpace(request.Name)
	}
	if requestUserCredentialIsSudo(request) {
		return "Sudo approval"
	}
	if strings.TrimSpace(request.EnvVar) != "" {
		return strings.TrimSpace(request.EnvVar)
	}
	return "Credential"
}

func requestUserCredentialWasRejected(view RequestUserCredentialView) (string, bool) {
	if strings.TrimSpace(view.RejectionSummary) != "" {
		return strings.TrimSpace(view.RejectionSummary), true
	}
	output := strings.TrimSpace(view.Output)
	if !strings.HasPrefix(output, requestUserCredentialRejectedPrefix) {
		return "", false
	}
	if reason, ok := strings.CutPrefix(output, requestUserCredentialRejectedReasonPrefix); ok {
		return strings.TrimSpace(reason), true
	}
	return "", true
}

func requestUserCredentialGrantedUses(granted GrantedCredentialView) []GrantedCredentialUseView {
	if len(granted.Uses) > 0 {
		return granted.Uses
	}
	return granted.ApprovedUses
}

func requestUserCredentialGrantedName(granted GrantedCredentialView) string {
	if strings.TrimSpace(granted.Name) != "" {
		return strings.TrimSpace(granted.Name)
	}
	if strings.TrimSpace(granted.EnvVar) != "" {
		return strings.TrimSpace(granted.EnvVar)
	}
	return "Credential"
}

func requestUserCredentialGrantedDetails(output requestUserCredentialOutput, input requestUserCredentialInput) []requestUserCredentialGrantedDetail {
	requestsByEnvVar := make(map[string]RequestedCredentialView, len(input.Credentials))
	for _, request := range input.Credentials {
		if envVar := strings.TrimSpace(request.EnvVar); envVar != "" {
			requestsByEnvVar[envVar] = request
		}
	}
	details := make([]requestUserCredentialGrantedDetail, 0, len(output.GrantedCredentials))
	for _, granted := range output.GrantedCredentials {
		request := requestsByEnvVar[strings.TrimSpace(granted.EnvVar)]
		name := requestUserCredentialGrantedName(granted)
		if strings.TrimSpace(granted.Name) == "" && strings.TrimSpace(request.Name) != "" {
			name = strings.TrimSpace(request.Name)
		}
		details = append(details, requestUserCredentialGrantedDetail{
			Granted:        granted,
			Request:        request,
			CredentialName: name,
			Justification:  strings.TrimSpace(request.Justification),
			IsSudo:         requestUserCredentialIsSudo(request) || strings.TrimSpace(granted.EnvVar) == sudoTokenEnvVar,
		})
	}
	return details
}

func requestUserCredentialGrantedIsSudoOnly(details []requestUserCredentialGrantedDetail) bool {
	if len(details) == 0 {
		return false
	}
	for _, detail := range details {
		if !detail.IsSudo {
			return false
		}
	}
	return true
}
