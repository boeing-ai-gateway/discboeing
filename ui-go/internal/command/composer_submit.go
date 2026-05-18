package command

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// ComposerSubmit handles the composer runtime slice for the current browser
// session. It parses the form, leaves a seam for the Discobot client operation,
// and updates the session-scoped frontend view model.
func (h *Handler) ComposerSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Warn("failed to parse composer command form", "error", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	prompt := strings.TrimSpace(r.FormValue("prompt"))
	var updateErr error
	session.Save(func(view *viewmodel.ShellSnapshot) {
		commandID := nextLabel(&view.Sidebar.Commands)
		attachments := append([]viewmodel.ComposerAttachment(nil), view.Workspace.Composer.Attachments...)
		if prompt == "" && len(attachments) == 0 {
			return
		}
		if view.Workspace.IsPending {
			projectID, workspaceID, err := h.defaultProjectWorkspace(r)
			if err != nil {
				updateErr = err
				return
			}
			created, err := h.client.Sessions.Create(r.Context(), projectID, api.CreateSessionRequest{WorkspaceID: workspaceID})
			if err != nil {
				updateErr = err
				return
			}
			if err := h.rebuildSidebarView(r.Context(), view, created.ID, ""); err != nil {
				updateErr = err
				return
			}
		}
		view.Workspace.State = "Composer command handled"
		view.Workspace.Message = ""
		content := prompt
		if len(attachments) > 0 {
			content = contentWithAttachments(content, attachments)
		}
		view.Workspace.Conversation.Messages = append(view.Workspace.Conversation.Messages,
			viewmodel.ConversationMessage{
				ID:      fmt.Sprintf("user-%d", commandID),
				Role:    "user",
				Content: content,
			},
			viewmodel.ConversationMessage{
				ID:   fmt.Sprintf("assistant-%d", commandID),
				Role: "assistant",
				Branches: []string{
					"ui-go recorded this prompt in the session-scoped view model. The Discobot API call will replace this placeholder response in a later integration slice.",
					"Branch 2 placeholder: this proves the server-owned branch selector can switch assistant alternatives without client-side state.",
					"Branch 3 placeholder: the real Discobot thread branch data will replace these generated alternatives in a later integration slice.",
				},
			},
		)
		view.Workspace.Composer.Draft = ""
		view.Workspace.Composer.Attachments = nil
	})
	if updateErr != nil {
		h.logger.Warn("failed to submit composer command", "error", updateErr)
		http.Error(w, "failed to submit prompt", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func contentWithAttachments(prompt string, attachments []viewmodel.ComposerAttachment) string {
	var builder strings.Builder
	builder.WriteString(prompt)
	if prompt != "" {
		builder.WriteString("\n\n")
	}
	builder.WriteString("Attached files:")
	for _, attachment := range attachments {
		builder.WriteString("\n- ")
		builder.WriteString(attachment.Filename)
		if attachment.MediaType != "" {
			builder.WriteString(" (")
			builder.WriteString(attachment.MediaType)
			builder.WriteString(")")
		}
	}
	return builder.String()
}

func nextLabel(label *string) uint64 {
	current, _ := strconv.ParseUint(*label, 10, 64)
	current++
	*label = strconv.FormatUint(current, 10)
	return current
}
