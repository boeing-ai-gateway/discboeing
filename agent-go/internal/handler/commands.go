package handler

import (
	"net/http"
	"sort"

	"github.com/obot-platform/discobot/agent-go/internal/api"
)

// ListCommands handles GET /commands.
func (h *Handler) ListCommands(w http.ResponseWriter, _ *http.Request) {
	commands, err := h.conversations.ListCommands()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "failed to list commands: "+err.Error())
		return
	}

	result := make([]api.Command, 0, len(commands))
	for _, command := range commands {
		item := api.Command{
			Name:        command.Name,
			Description: command.Description,
			Kind:        string(command.Kind),
			Discobot: api.CommandDiscobotMetadata{
				UI:          command.Discobot.UI,
				Label:       command.Discobot.Label,
				ActiveLabel: command.Discobot.ActiveLabel,
				Icon:        command.Discobot.Icon,
				Group:       command.Discobot.Group,
				Order:       command.Discobot.Order,
			},
		}
		if len(command.Discobot.CredentialRequest) > 0 {
			item.Discobot.CredentialRequest = make([]api.CommandCredentialRequest, 0, len(command.Discobot.CredentialRequest))
			for _, request := range command.Discobot.CredentialRequest {
				converted := api.CommandCredentialRequest{
					EnvVar:        request.EnvVar,
					Name:          request.Name,
					Justification: request.Justification,
				}
				if len(request.ApprovedUses) > 0 {
					converted.ApprovedUses = make([]api.CommandApprovedUse, 0, len(request.ApprovedUses))
					for _, use := range request.ApprovedUses {
						converted.ApprovedUses = append(converted.ApprovedUses, api.CommandApprovedUse{Description: use.Description})
					}
				}
				item.Discobot.CredentialRequest = append(item.Discobot.CredentialRequest, converted)
			}
		}
		result = append(result, item)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Discobot.Order != result[j].Discobot.Order {
			return result[i].Discobot.Order < result[j].Discobot.Order
		}
		return result[i].Name < result[j].Name
	})

	h.JSON(w, http.StatusOK, api.ListCommandsResponse{Commands: result})
}
