package command

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

type fileNamePayload struct {
	Name string `json:"name"`
}

type fileSearchPayload struct {
	Query string `json:"query"`
}

type fileMovePayload struct {
	TargetID string `json:"targetID"`
}

// FileSelect marks a file tree node as selected in server-owned view state.
func (h *Handler) FileSelect(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	generation := h.view.SaveShell(func(data *state.Data, view *state.View) {
		view.SelectedFileID = id
		file, ok := dataFileByID(data, id)
		if !ok || file.Kind != state.FileKindFile {
			return
		}
		view.ActiveFileID = id
		if !openFileExists(view.OpenFileIDs, id) {
			view.OpenFileIDs = append(view.OpenFileIDs, id)
		}
	})
	writeGeneration(w, generation)
}

// FileRename renames a file tree node in server-owned prototype data.
func (h *Handler) FileRename(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var payload fileNamePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	name := strings.TrimSpace(payload.Name)
	generation := h.view.SaveData(func(data *state.Data) {
		if name == "" {
			return
		}
		sessionIndex, fileIndex, ok := fileLocation(data, id)
		if !ok {
			return
		}
		data.Sessions[sessionIndex].Files[fileIndex].Name = name
		data.Sessions[sessionIndex].Files[fileIndex].GitStatus = state.FileGitStatusRenamed
	})
	writeGeneration(w, generation)
}

// FileCreateFile adds a sample child file under a directory node.
func (h *Handler) FileCreateFile(w http.ResponseWriter, r *http.Request) {
	h.fileCreate(w, r, state.FileKindFile)
}

// FileCreateDirectory adds a sample child directory under a directory node.
func (h *Handler) FileCreateDirectory(w http.ResponseWriter, r *http.Request) {
	h.fileCreate(w, r, state.FileKindDirectory)
}

func (h *Handler) fileCreate(w http.ResponseWriter, r *http.Request, kind state.FileKind) {
	parentID := chi.URLParam(r, "id")
	var payload fileNamePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	name := strings.TrimSpace(payload.Name)
	generation := h.view.SaveShell(func(data *state.Data, view *state.View) {
		if name == "" {
			return
		}
		sessionIndex, parentIndex, ok := fileLocation(data, parentID)
		if !ok {
			return
		}
		files := &data.Sessions[sessionIndex].Files
		parent := (*files)[parentIndex]
		if !ok || parent.Kind != state.FileKindDirectory {
			return
		}
		id := nextFileID(*files, parentID, name)
		status := state.FileGitStatusUntracked
		changedDescendants := false
		if kind == state.FileKindDirectory {
			status = state.FileGitStatusClean
			changedDescendants = true
		}
		*files = append(*files, state.FileNode{
			ID:                    id,
			SessionID:             parent.SessionID,
			ParentID:              parent.ID,
			Name:                  name,
			Kind:                  kind,
			GitStatus:             status,
			HasChangedDescendants: changedDescendants,
		})
		if view.ExpandedFileIDs == nil {
			view.ExpandedFileIDs = map[string]bool{}
		}
		view.ExpandedFileIDs[parent.ID] = true
		view.SelectedFileID = id
	})
	writeGeneration(w, generation)
}

// FileExpandAll expands all directories for a session.
func (h *Handler) FileExpandAll(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	generation := h.view.SaveShell(func(data *state.Data, view *state.View) {
		if view.ExpandedFileIDs == nil {
			view.ExpandedFileIDs = map[string]bool{}
		}
		for _, session := range data.Sessions {
			if session.ID != sessionID {
				continue
			}
			for _, file := range session.Files {
				if file.Kind == state.FileKindDirectory {
					view.ExpandedFileIDs[file.ID] = true
				}
			}
		}
	})
	writeGeneration(w, generation)
}

// FileCollapseAll collapses all directories for a session.
func (h *Handler) FileCollapseAll(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	generation := h.view.SaveShell(func(data *state.Data, view *state.View) {
		for _, session := range data.Sessions {
			if session.ID != sessionID {
				continue
			}
			for _, file := range session.Files {
				if file.Kind == state.FileKindDirectory {
					delete(view.ExpandedFileIDs, file.ID)
				}
			}
		}
	})
	writeGeneration(w, generation)
}

// FileMove reparents a file tree node under another directory or root.
func (h *Handler) FileMove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var payload fileMovePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	generation := h.view.SaveData(func(data *state.Data) {
		moveFile(data, id, strings.TrimSpace(payload.TargetID))
	})
	writeGeneration(w, generation)
}

// FileDrop moves the dragged node onto this directory target.
func (h *Handler) FileDrop(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")
	var payload fileMovePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	generation := h.view.SaveData(func(data *state.Data) {
		moveFile(data, strings.TrimSpace(payload.TargetID), targetID)
	})
	writeGeneration(w, generation)
}

// FileMoveToRoot reparents a file tree node to the session root.
func (h *Handler) FileMoveToRoot(w http.ResponseWriter, r *http.Request) {
	var payload fileMovePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	generation := h.view.SaveData(func(data *state.Data) {
		moveFile(data, strings.TrimSpace(payload.TargetID), "")
	})
	writeGeneration(w, generation)
}

func moveFile(data *state.Data, id string, targetID string) bool {
	sessionIndex, sourceIndex, ok := fileLocation(data, id)
	if id == "" || id == targetID || !ok {
		return false
	}
	files := data.Sessions[sessionIndex].Files
	if fileIsDescendant(files, targetID, id) {
		return false
	}
	if targetID != "" {
		target, ok := fileByID(files, targetID)
		if !ok || target.Kind != state.FileKindDirectory {
			return false
		}
	}
	data.Sessions[sessionIndex].Files[sourceIndex].ParentID = targetID
	data.Sessions[sessionIndex].Files[sourceIndex].GitStatus = state.FileGitStatusRenamed
	return true
}

func fileIsDescendant(files []state.FileNode, id string, ancestorID string) bool {
	if id == "" || ancestorID == "" {
		return false
	}
	current, ok := fileByID(files, id)
	for ok && current.ParentID != "" {
		if current.ParentID == ancestorID {
			return true
		}
		current, ok = fileByID(files, current.ParentID)
	}
	return false
}

// FileSearch updates the server-owned file-tree search query.
func (h *Handler) FileSearch(w http.ResponseWriter, r *http.Request) {
	var payload fileSearchPayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	generation := h.view.SaveView(func(view *state.View) {
		view.FileTreeSearch = strings.TrimSpace(payload.Query)
	})
	writeGeneration(w, generation)
}

func fileByID(files []state.FileNode, id string) (state.FileNode, bool) {
	for _, file := range files {
		if file.ID == id {
			return file, true
		}
	}
	return state.FileNode{}, false
}

func dataFileByID(data *state.Data, id string) (state.FileNode, bool) {
	sessionIndex, fileIndex, ok := fileLocation(data, id)
	if !ok {
		return state.FileNode{}, false
	}
	return data.Sessions[sessionIndex].Files[fileIndex], true
}

func fileLocation(data *state.Data, id string) (int, int, bool) {
	for sessionIndex := range data.Sessions {
		for fileIndex := range data.Sessions[sessionIndex].Files {
			if data.Sessions[sessionIndex].Files[fileIndex].ID == id {
				return sessionIndex, fileIndex, true
			}
		}
	}
	return 0, 0, false
}

func nextFileID(files []state.FileNode, parentID string, name string) string {
	base := parentID + "-" + fileIDPart(name)
	id := base
	for suffix := 2; fileIDExists(files, id); suffix++ {
		id = base + "-" + strconv.Itoa(suffix)
	}
	return id
}

func fileIDPart(name string) string {
	name = strings.ToLower(name)
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	part := strings.Trim(builder.String(), "-")
	if part == "" {
		return "new"
	}
	return part
}

func fileIDExists(files []state.FileNode, id string) bool {
	for _, file := range files {
		if file.ID == id {
			return true
		}
	}
	return false
}

func openFileExists(openFileIDs []string, id string) bool {
	for _, openFileID := range openFileIDs {
		if openFileID == id {
			return true
		}
	}
	return false
}
