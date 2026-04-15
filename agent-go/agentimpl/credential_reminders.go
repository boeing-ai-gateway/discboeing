package agentimpl

import (
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/credentials"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

func (a *DefaultAgent) buildCredentialChangeReminder(
	communicated []thread.CommunicatedCredentialBinding,
) ([]thread.CommunicatedCredentialBinding, string) {
	currentCommunicated := thread.NormalizeCommunicatedCredentialBindings(communicated)
	reportable := reportableBindingsToCommunicated(a.registry.ReportableCredentialBindings())
	added, removed := thread.DiffCommunicatedCredentialBindings(currentCommunicated, reportable)
	reminder := sessionconfig.FormatCredentialChangeReminder(
		communicatedBindingsToReminderBindings(added),
		communicatedBindingsToReminderBindings(removed),
	)
	return reportable, reminder
}

func communicatedBindingsToReminderBindings(
	bindings []thread.CommunicatedCredentialBinding,
) []sessionconfig.CredentialReminderBinding {
	converted := make([]sessionconfig.CredentialReminderBinding, 0, len(bindings))
	for _, binding := range bindings {
		uses := make([]sessionconfig.CredentialReminderUse, 0, len(binding.Uses))
		for _, use := range binding.Uses {
			uses = append(uses, sessionconfig.CredentialReminderUse{
				ID:          use.ID,
				Description: use.Description,
			})
		}
		converted = append(converted, sessionconfig.CredentialReminderBinding{
			CredentialID: binding.CredentialID,
			EnvVar:       binding.EnvVar,
			Uses:         uses,
		})
	}
	return converted
}

func reportableBindingsToCommunicated(
	bindings []credentials.ReportableBinding,
) []thread.CommunicatedCredentialBinding {
	converted := make([]thread.CommunicatedCredentialBinding, 0, len(bindings))
	for _, binding := range bindings {
		uses := make([]thread.CommunicatedCredentialUse, 0, len(binding.Uses))
		for _, use := range binding.Uses {
			uses = append(uses, thread.CommunicatedCredentialUse{
				ID:          strings.TrimSpace(use.ID),
				Description: strings.TrimSpace(use.Description),
			})
		}
		converted = append(converted, thread.CommunicatedCredentialBinding{
			CredentialID: strings.TrimSpace(binding.CredentialID),
			EnvVar:       strings.TrimSpace(binding.EnvVar),
			Uses:         uses,
		})
	}
	return thread.NormalizeCommunicatedCredentialBindings(converted)
}
