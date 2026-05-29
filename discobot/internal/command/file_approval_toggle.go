package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// FileApprovalToggle toggles whether a changed file is approved for commit.
func (h *Handler) FileApprovalToggle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveData(func(data *state.Data) {
		sessionIndex, fileIndex, ok := fileLocation(data, id)
		if !ok {
			return
		}
		file := data.Sessions[sessionIndex].Files[fileIndex]
		if file.Kind != state.FileKindFile || file.GitStatus == "" || file.GitStatus == state.FileGitStatusClean {
			return
		}
		data.Sessions[sessionIndex].Files[fileIndex].Approved = !file.Approved
	})
	writeNoContent(w)
}
