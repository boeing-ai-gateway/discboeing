package command

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// PromptQueueAction handles server-owned queued prompt row actions for the
// temporary ui-go read model.
func (h *Handler) PromptQueueAction(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	promptID := r.URL.Query().Get("prompt_id")
	action := r.URL.Query().Get("action")
	if promptID == "" || action == "" {
		http.Error(w, "invalid queued prompt command", http.StatusBadRequest)
		return
	}

	session.Save(func(view *viewmodel.ShellSnapshot) {
		queue := &view.Workspace.Composer.PromptQueue
		for index := range *queue {
			if (*queue)[index].ID != promptID {
				continue
			}
			applyPromptQueueAction(view, queue, index, action, r.URL.Query().Get("offset_minutes"))
			return
		}
	})

	w.WriteHeader(http.StatusNoContent)
}

func applyPromptQueueAction(view *viewmodel.ShellSnapshot, queue *[]viewmodel.QueuedPrompt, index int, action string, offsetMinutes string) {
	entry := &(*queue)[index]
	if entry.Saving {
		return
	}

	switch action {
	case "move-up":
		if index > 0 {
			(*queue)[index-1], (*queue)[index] = (*queue)[index], (*queue)[index-1]
			view.Workspace.State = "Queued prompt moved up"
		}
	case "move-down":
		if index < len(*queue)-1 {
			(*queue)[index+1], (*queue)[index] = (*queue)[index], (*queue)[index+1]
			view.Workspace.State = "Queued prompt moved down"
		}
	case "edit":
		entry.Editing = !entry.Editing
		entry.ScheduleOpen = false
		view.Workspace.State = "Queued prompt edit toggled"
	case "cancel-edit", "save-edit":
		entry.Editing = false
		view.Workspace.State = "Queued prompt edit closed"
	case "schedule":
		entry.ScheduleOpen = !entry.ScheduleOpen
		entry.Editing = false
		view.Workspace.State = "Queued prompt schedule toggled"
	case "later":
		minutes, err := strconv.Atoi(offsetMinutes)
		if err != nil || minutes <= 0 {
			return
		}
		entry.RunAfter = time.Now().Add(time.Duration(minutes) * time.Minute).Format("2006-01-02T15:04")
		entry.RunAfterLabel = queuedPromptOffsetLabel(minutes)
		entry.ScheduleOpen = false
		view.Workspace.State = "Queued prompt scheduled"
	case "pause":
		entry.RunAfter = ""
		entry.RunAfterLabel = "Paused"
		entry.ScheduleOpen = false
		view.Workspace.State = "Queued prompt paused"
	case "run-now":
		runQueuedPrompt(view, (*queue)[index])
		*queue = append((*queue)[:index], (*queue)[index+1:]...)
	case "delete":
		*queue = append((*queue)[:index], (*queue)[index+1:]...)
		view.Workspace.State = "Queued prompt deleted"
	case "save-custom":
		entry.ScheduleOpen = false
		view.Workspace.State = "Queued prompt schedule saved"
	}
}

func runQueuedPrompt(view *viewmodel.ShellSnapshot, entry viewmodel.QueuedPrompt) {
	commandID := nextLabel(&view.Sidebar.Commands)
	view.Workspace.State = "Queued prompt ran"
	view.Workspace.Message = ""
	view.Workspace.Conversation.Messages = append(view.Workspace.Conversation.Messages,
		viewmodel.ConversationMessage{
			ID:      fmt.Sprintf("queued-user-%d", commandID),
			Role:    "user",
			Content: entry.Text,
		},
		viewmodel.ConversationMessage{
			ID:      fmt.Sprintf("queued-assistant-%d", commandID),
			Role:    "assistant",
			Content: "ui-go ran this queued prompt in the session-scoped view model. Real queue execution will replace this placeholder in a later integration slice.",
		},
	)
}

func queuedPromptOffsetLabel(minutes int) string {
	switch minutes {
	case 15:
		return "Runs in 15 min"
	case 60:
		return "Runs in 1 hour"
	case 1440:
		return "Runs in 1 day"
	default:
		return fmt.Sprintf("Runs in %d min", minutes)
	}
}
