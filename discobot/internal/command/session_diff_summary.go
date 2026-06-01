package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SessionDiffSummary opens a vertical diff summary for a session in the editor pane.
func (h *Handler) SessionDiffSummary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	h.view.SaveView(r.Context(), func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		sessionPanel.SelectedSessionID = id
		if sessionPanel.ExpandedSessionIDs == nil {
			sessionPanel.ExpandedSessionIDs = map[string]bool{}
		}
		sessionPanel.ExpandedSessionIDs[id] = true
		if sessionPanel.SessionViewModes == nil {
			sessionPanel.SessionViewModes = map[string]state.SessionViewMode{}
		}
		sessionPanel.SessionViewModes[id] = state.SessionViewModeDiff

		editorState := state.EnsureEditorPanelState(view)
		editorState.ActiveFileID = ""
		editorState.DiffSummarySessionID = id
		editorState.ServiceLogID = ""

		editorPanel := state.EnsurePanel(view, "editor")
		editorPanel.Visible = true
		state.SavePanel(view, "editor", editorPanel)
	})
	writeNoContent(w)
}
