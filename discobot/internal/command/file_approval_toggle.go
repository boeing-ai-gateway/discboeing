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
		location, ok := fileLocation(data, id)
		if !ok {
			return
		}
		projectData := data.Project[location.projectID]
		sessionData := projectData.Session[location.sessionID]
		file := sessionData.Files[location.fileIndex]
		if file.Kind != state.FileKindFile || file.GitStatus == "" || file.GitStatus == state.FileGitStatusClean {
			return
		}
		sessionData.Files[location.fileIndex].Approved = !file.Approved
		projectData.Session[location.sessionID] = sessionData
		data.Project[location.projectID] = projectData
	})
	writeNoContent(w)
}
