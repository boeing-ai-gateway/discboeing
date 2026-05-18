package command

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/live"
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
			projectID, workspaceID, err := h.pendingComposerWorkspace(r, view.Workspace.Composer.WorkspaceSelector)
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
		runAfter := strings.TrimSpace(r.FormValue("run_after"))
		if runAfter != "" {
			view.Workspace.Composer.PromptQueue = append(view.Workspace.Composer.PromptQueue, viewmodel.QueuedPrompt{
				ID:              fmt.Sprintf("queued-%d", commandID),
				Text:            content,
				Model:           view.Workspace.Composer.ModelLabel,
				RunAfter:        runAfter,
				RunAfterLabel:   scheduledPromptRunAfterLabel(runAfter),
				AttachmentCount: len(attachments),
			})
			view.Workspace.Composer.Draft = ""
			view.Workspace.Composer.Attachments = nil
			view.Workspace.Composer.ScheduledRunAfter = ""
			view.Workspace.Composer.ScheduleOpen = false
			return
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
		view.Workspace.Composer.ScheduledRunAfter = ""
		view.Workspace.Composer.ScheduleOpen = false
	})
	if updateErr != nil {
		h.logger.Warn("failed to submit composer command", "error", updateErr)
		http.Error(w, "failed to submit prompt", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func scheduledPromptRunAfterLabel(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	if parsed.After(time.Now().AddDate(25, 0, 0)) {
		return "Paused"
	}
	return "Runs " + parsed.Format("Jan 2, 3:04 PM")
}

func (h *Handler) pendingComposerWorkspace(r *http.Request, selector viewmodel.ConversationWorkspaceSelectorSnapshot) (string, string, error) {
	projectID := live.DefaultProjectID
	if workspaceID, ok := strings.CutPrefix(selector.SelectedOption, "existing:"); ok {
		return projectID, workspaceID, nil
	}
	if selector.RequiresInput {
		if selector.ValidationSourceType != selector.SourceType || !selector.ValidationValid {
			return "", "", fmt.Errorf("validate workspace before submitting")
		}
		path := strings.TrimSpace(selector.ValidationPath)
		if path == "" {
			path = strings.TrimSpace(selector.SourceInput)
		}
		workspace, err := h.client.Workspaces.Create(r.Context(), projectID, api.CreateWorkspaceRequest{
			Path:       path,
			SourceType: selector.SourceType,
		})
		if err != nil {
			return "", "", err
		}
		return projectID, workspace.ID, nil
	}
	if selector.SelectedOption == "new-workspace" {
		return projectID, "", nil
	}
	return h.defaultProjectWorkspace(r)
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
