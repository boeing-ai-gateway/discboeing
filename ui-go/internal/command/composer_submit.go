package command

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/live"
)

// ComposerSubmit resolves any pending workspace/session state, forwards the
// prompt and staged attachments to the backend chat API, and updates the
// session-scoped frontend view model.
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
	current := session.View()
	attachments := append([]viewmodel.ComposerAttachment(nil), current.Workspace.Composer.Attachments...)
	if prompt == "" && len(attachments) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	projectID, sessionID, threadID, workspaceID, err := h.composerSubmitTarget(r, current)
	if err != nil {
		h.logger.Warn("failed to resolve composer submit target", "error", err)
		http.Error(w, "failed to prepare session", http.StatusBadRequest)
		return
	}

	runAfter := strings.TrimSpace(r.FormValue("run_after"))
	chatReq := api.ChatRequest{
		Messages:    []json.RawMessage{composerUserMessage(prompt, attachments)},
		WorkspaceID: workspaceID,
		Model:       current.Workspace.Composer.ModelID,
		Reasoning:   current.Workspace.Composer.ReasoningValue,
		ServiceTier: current.Workspace.Composer.ServiceTierValue,
		RunAfter:    runAfter,
	}
	if _, err := h.client.Sessions.StartChat(r.Context(), projectID, sessionID, threadID, chatReq); err != nil {
		h.logger.Warn("failed to submit composer prompt", "error", err)
		session.Save(func(view *viewmodel.ShellSnapshot) {
			view.Workspace.Composer.Error = err.Error()
		})
		http.Error(w, "failed to submit prompt", http.StatusBadGateway)
		return
	}

	var updateErr error
	session.Save(func(view *viewmodel.ShellSnapshot) {
		commandID := nextLabel(&view.Sidebar.Commands)
		if err := h.rebuildSidebarView(r.Context(), view, sessionID, threadID); err != nil {
			updateErr = err
			return
		}
		view.Workspace.State = "Prompt submitted"
		view.Workspace.Message = ""
		if runAfter != "" {
			content := prompt
			if len(attachments) > 0 {
				content = contentWithAttachments(content, attachments)
			}
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
		view.Workspace.Composer.Draft = ""
		view.Workspace.Composer.Attachments = nil
		view.Workspace.Composer.Error = ""
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

func (h *Handler) composerSubmitTarget(r *http.Request, view viewmodel.ShellSnapshot) (string, string, string, string, error) {
	projectID := live.DefaultProjectID
	sessionID, threadID := currentSidebarSelection(view)
	workspaceID := ""
	if view.Workspace.IsPending {
		var err error
		projectID, workspaceID, err = h.pendingComposerWorkspace(r, view.Workspace.Composer.WorkspaceSelector)
		if err != nil {
			return "", "", "", "", err
		}
		sessionID = randomComposerID()
		threadID = sessionID
	} else if sessionID == "" {
		return "", "", "", "", fmt.Errorf("missing selected session")
	}
	if threadID == "" {
		threadID = sessionID
	}
	return projectID, sessionID, threadID, workspaceID, nil
}

func composerUserMessage(prompt string, attachments []viewmodel.ComposerAttachment) json.RawMessage {
	type messagePart struct {
		Type      string `json:"type"`
		Text      string `json:"text,omitempty"`
		Filename  string `json:"filename,omitempty"`
		MediaType string `json:"mediaType,omitempty"`
		URL       string `json:"url,omitempty"`
	}
	message := struct {
		ID    string        `json:"id"`
		Role  string        `json:"role"`
		Parts []messagePart `json:"parts"`
	}{
		ID:   randomComposerID(),
		Role: "user",
	}
	if strings.TrimSpace(prompt) != "" {
		message.Parts = append(message.Parts, messagePart{Type: "text", Text: prompt})
	}
	for _, attachment := range attachments {
		message.Parts = append(message.Parts, messagePart{
			Type:      "file",
			Filename:  attachment.Filename,
			MediaType: attachment.MediaType,
			URL:       attachment.URL,
		})
	}
	data, err := json.Marshal(message)
	if err != nil {
		return nil
	}
	return data
}

func randomComposerID() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
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
