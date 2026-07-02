package agentimpl

import (
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/scriptexec"
	"github.com/boeing-ai-gateway/discboeing/agent-go/sessionconfig"
	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func scriptExecutionMetadata(execution scriptexec.Execution) *thread.ScriptExecutionMetadata {
	return &thread.ScriptExecutionMetadata{
		ScriptName: execution.Script.Name,
		ScriptPath: execution.Script.Path,
		ExitCode:   execution.ExitCode,
		Success:    execution.Success,
		Stdout:     execution.Stdout,
		Stderr:     execution.Stderr,
	}
}

func discboeingCommandMetadata(metadata sessionconfig.DiscboeingCommandMetadata) api.CommandDiscboeingMetadata {
	converted := api.CommandDiscboeingMetadata{
		UI:          metadata.UI,
		Label:       metadata.Label,
		ActiveLabel: metadata.ActiveLabel,
		Icon:        metadata.Icon,
		Group:       metadata.Group,
		Order:       metadata.Order,
	}
	if len(metadata.CredentialRequest) > 0 {
		converted.CredentialRequest = make([]api.CommandCredentialRequest, 0, len(metadata.CredentialRequest))
		for _, request := range metadata.CredentialRequest {
			credential := api.CommandCredentialRequest{
				EnvVar:        request.EnvVar,
				Name:          request.Name,
				Justification: request.Justification,
			}
			if len(request.ApprovedUses) > 0 {
				credential.ApprovedUses = make([]api.CommandApprovedUse, 0, len(request.ApprovedUses))
				for _, use := range request.ApprovedUses {
					credential.ApprovedUses = append(credential.ApprovedUses, api.CommandApprovedUse{
						Description: use.Description,
					})
				}
			}
			converted.CredentialRequest = append(converted.CredentialRequest, credential)
		}
	}
	return converted
}
