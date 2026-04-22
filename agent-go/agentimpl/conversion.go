package agentimpl

import (
	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/scriptexec"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
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

func discobotCommandMetadata(metadata sessionconfig.DiscobotCommandMetadata) agent.DiscobotCommandMetadata {
	converted := agent.DiscobotCommandMetadata{
		UI:          metadata.UI,
		Label:       metadata.Label,
		ActiveLabel: metadata.ActiveLabel,
		Icon:        metadata.Icon,
		Group:       metadata.Group,
		Order:       metadata.Order,
	}
	if len(metadata.CredentialRequest) > 0 {
		converted.CredentialRequest = make([]agent.DiscobotCredentialRequest, 0, len(metadata.CredentialRequest))
		for _, request := range metadata.CredentialRequest {
			credential := agent.DiscobotCredentialRequest{
				EnvVar:        request.EnvVar,
				Name:          request.Name,
				Justification: request.Justification,
			}
			if len(request.ApprovedUses) > 0 {
				credential.ApprovedUses = make([]agent.DiscobotCredentialApprovedUse, 0, len(request.ApprovedUses))
				for _, use := range request.ApprovedUses {
					credential.ApprovedUses = append(credential.ApprovedUses, agent.DiscobotCredentialApprovedUse{
						Description: use.Description,
					})
				}
			}
			converted.CredentialRequest = append(converted.CredentialRequest, credential)
		}
	}
	return converted
}
