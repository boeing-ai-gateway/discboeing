package command

import (
	"net/http"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

// ComposerStop clears the temporary generating state for the current browser
// session. Real agent cancellation will be wired through the Discobot API later.
func (h *Handler) ComposerStop(w http.ResponseWriter, r *http.Request) {
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	session.Save(func(view *viewmodel.ShellSnapshot) {
		view.Workspace.Composer.SubmitStatus = ""
		view.Workspace.Composer.IsStreaming = false
		view.Workspace.State = "Composer stop requested"
	})

	w.WriteHeader(http.StatusNoContent)
}
