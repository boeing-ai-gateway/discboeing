package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// FileDelete removes a file tree node and all descendants from server data.
func (h *Handler) FileDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		deleted := deleteFiles(data, id)
		sessionPanel := state.EnsureSessionPanelState(view)
		editorPanel := state.EnsureEditorPanelState(view)
		for deletedID := range deleted {
			delete(sessionPanel.ExpandedFileIDs, deletedID)
		}
		if deleted[sessionPanel.SelectedFileID] {
			sessionPanel.SelectedFileID = ""
		}
		if deleted[editorPanel.ActiveFileID] {
			editorPanel.ActiveFileID = ""
		}
		openFileIDs := editorPanel.OpenFileIDs[:0]
		for _, openFileID := range editorPanel.OpenFileIDs {
			if !deleted[openFileID] {
				openFileIDs = append(openFileIDs, openFileID)
			}
		}
		editorPanel.OpenFileIDs = openFileIDs
		if editorPanel.ActiveFileID == "" && len(editorPanel.OpenFileIDs) > 0 {
			editorPanel.ActiveFileID = editorPanel.OpenFileIDs[len(editorPanel.OpenFileIDs)-1]
		}
	})
	writeNoContent(w)
}

func deleteFiles(data *state.Data, id string) map[string]bool {
	location, ok := fileLocation(data, id)
	if !ok {
		return map[string]bool{}
	}
	projectData := data.Project[location.projectID]
	sessionData := projectData.Session[location.sessionID]
	sessionFiles := &sessionData.Files
	deleted := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, file := range *sessionFiles {
			if deleted[file.ID] || !deleted[file.ParentID] {
				continue
			}
			deleted[file.ID] = true
			changed = true
		}
	}

	files := (*sessionFiles)[:0]
	for _, file := range *sessionFiles {
		if !deleted[file.ID] {
			files = append(files, file)
		}
	}
	*sessionFiles = files
	projectData.Session[location.sessionID] = sessionData
	data.Project[location.projectID] = projectData
	return deleted
}
