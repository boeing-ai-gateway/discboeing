package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// MessageBranch switches the active alternative for a branched conversation
// message in the current browser session.
func (h *Handler) MessageBranch(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	messageID := r.URL.Query().Get("message_id")
	direction := r.URL.Query().Get("direction")
	if messageID == "" || (direction != "previous" && direction != "next") {
		http.Error(w, "invalid branch command", http.StatusBadRequest)
		return
	}

	session.Save(func(view *viewmodel.ShellSnapshot) {
		for i := range view.Workspace.Conversation.Messages {
			message := &view.Workspace.Conversation.Messages[i]
			if message.ID != messageID || len(message.Branches) <= 1 {
				continue
			}
			if message.CurrentBranch < 0 || message.CurrentBranch >= len(message.Branches) {
				message.CurrentBranch = 0
			}
			if direction == "previous" {
				message.CurrentBranch = (message.CurrentBranch + len(message.Branches) - 1) % len(message.Branches)
			} else {
				message.CurrentBranch = (message.CurrentBranch + 1) % len(message.Branches)
			}
			view.Workspace.State = "Message branch changed"
			return
		}
	})

	w.WriteHeader(http.StatusNoContent)
}
