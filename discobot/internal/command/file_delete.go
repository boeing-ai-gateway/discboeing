package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// FileDelete removes a file tree node and all descendants from server data.
func (h *Handler) FileDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	generation := h.view.SaveShell(func(data *state.Data, view *state.View) {
		deleted := deleteFiles(data, id)
		for deletedID := range deleted {
			delete(view.ExpandedFileIDs, deletedID)
		}
		if deleted[view.SelectedFileID] {
			view.SelectedFileID = ""
		}
		if deleted[view.ActiveFileID] {
			view.ActiveFileID = ""
		}
		openFileIDs := view.OpenFileIDs[:0]
		for _, openFileID := range view.OpenFileIDs {
			if !deleted[openFileID] {
				openFileIDs = append(openFileIDs, openFileID)
			}
		}
		view.OpenFileIDs = openFileIDs
		if view.ActiveFileID == "" && len(view.OpenFileIDs) > 0 {
			view.ActiveFileID = view.OpenFileIDs[len(view.OpenFileIDs)-1]
		}
	})
	writeGeneration(w, generation)
}

func deleteFiles(data *state.Data, id string) map[string]bool {
	sessionIndex, _, ok := fileLocation(data, id)
	if !ok {
		return map[string]bool{}
	}
	sessionFiles := &data.Sessions[sessionIndex].Files
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
	return deleted
}
