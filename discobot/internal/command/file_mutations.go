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
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		editorPanel := state.EnsureEditorPanelState(view)
		sessionPanel.SelectedFileID = id
		file, ok := dataFileByID(data, id)
		if !ok || file.Kind != state.FileKindFile {
			return
		}
		editorPanel.ActiveFileID = id
		editorPanel.DiffSummarySessionID = ""
		editorPanel.ServiceLogID = ""
		if !openFileExists(editorPanel.OpenFileIDs, id) {
			editorPanel.OpenFileIDs = append(editorPanel.OpenFileIDs, id)
		}
		panel := state.EnsurePanel(view, "editor")
		panel.Visible = true
		state.SavePanel(view, "editor", panel)
	})
	writeNoContent(w)
}

// FileRename renames a file tree node in server-owned prototype data.
func (h *Handler) FileRename(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var payload fileNamePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	name := strings.TrimSpace(payload.Name)
	h.view.SaveData(r.Context(), func(data *state.Data) {
		if name == "" {
			return
		}
		location, ok := fileLocation(data, id)
		if !ok {
			return
		}
		projectData := data.Project[location.projectID]
		sessionData := projectData.Session[location.sessionID]
		sessionData.Files[location.fileIndex].Name = name
		sessionData.Files[location.fileIndex].GitStatus = state.FileGitStatusRenamed
		projectData.Session[location.sessionID] = sessionData
		data.Project[location.projectID] = projectData
	})
	writeNoContent(w)
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
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		if name == "" {
			return
		}
		location, ok := fileLocation(data, parentID)
		if !ok {
			return
		}
		projectData := data.Project[location.projectID]
		sessionData := projectData.Session[location.sessionID]
		files := &sessionData.Files
		parent := (*files)[location.fileIndex]
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
		sessionPanel := state.EnsureSessionPanelState(view)
		if sessionPanel.ExpandedFileIDs == nil {
			sessionPanel.ExpandedFileIDs = map[string]bool{}
		}
		sessionPanel.ExpandedFileIDs[parent.ID] = true
		sessionPanel.SelectedFileID = id
		projectData.Session[location.sessionID] = sessionData
		data.Project[location.projectID] = projectData
	})
	writeNoContent(w)
}

// FileExpandAll expands all directories for a session.
func (h *Handler) FileExpandAll(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		if sessionPanel.ExpandedFileIDs == nil {
			sessionPanel.ExpandedFileIDs = map[string]bool{}
		}
		for _, session := range state.Sessions(*data) {
			if session.ID != sessionID {
				continue
			}
			for _, file := range session.Files {
				if file.Kind == state.FileKindDirectory {
					sessionPanel.ExpandedFileIDs[file.ID] = true
				}
			}
		}
	})
	writeNoContent(w)
}

// FileCollapseAll collapses all directories for a session.
func (h *Handler) FileCollapseAll(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")
	h.view.SaveShell(r.Context(), func(data *state.Data, view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		for _, session := range state.Sessions(*data) {
			if session.ID != sessionID {
				continue
			}
			for _, file := range session.Files {
				if file.Kind == state.FileKindDirectory {
					delete(sessionPanel.ExpandedFileIDs, file.ID)
				}
			}
		}
	})
	writeNoContent(w)
}

// FileMove reparents a file tree node under another directory or root.
func (h *Handler) FileMove(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var payload fileMovePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	h.view.SaveData(r.Context(), func(data *state.Data) {
		moveFile(data, id, strings.TrimSpace(payload.TargetID))
	})
	writeNoContent(w)
}

// FileDrop moves the dragged node onto this directory target.
func (h *Handler) FileDrop(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")
	var payload fileMovePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	h.view.SaveData(r.Context(), func(data *state.Data) {
		moveFile(data, strings.TrimSpace(payload.TargetID), targetID)
	})
	writeNoContent(w)
}

// FileMoveToRoot reparents a file tree node to the session root.
func (h *Handler) FileMoveToRoot(w http.ResponseWriter, r *http.Request) {
	var payload fileMovePayload
	_ = json.NewDecoder(r.Body).Decode(&payload)
	h.view.SaveData(r.Context(), func(data *state.Data) {
		moveFile(data, strings.TrimSpace(payload.TargetID), "")
	})
	writeNoContent(w)
}

func moveFile(data *state.Data, id string, targetID string) bool {
	location, ok := fileLocation(data, id)
	if id == "" || id == targetID || !ok {
		return false
	}
	projectData := data.Project[location.projectID]
	sessionData := projectData.Session[location.sessionID]
	files := sessionData.Files
	if fileIsDescendant(files, targetID, id) {
		return false
	}
	if targetID != "" {
		target, ok := fileByID(files, targetID)
		if !ok || target.Kind != state.FileKindDirectory {
			return false
		}
	}
	sessionData.Files[location.fileIndex].ParentID = targetID
	sessionData.Files[location.fileIndex].GitStatus = state.FileGitStatusRenamed
	projectData.Session[location.sessionID] = sessionData
	data.Project[location.projectID] = projectData
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
	h.view.SaveView(r.Context(), func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		sessionPanel.FileTreeSearch = strings.TrimSpace(payload.Query)
		if sessionPanel.FileTreeSearch != "" {
			sessionPanel.FileTreeSearchVisible = true
		}
	})
	writeNoContent(w)
}

// FileSearchToggle shows or hides the session file-tree search field.
func (h *Handler) FileSearchToggle(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		sessionPanel := state.EnsureSessionPanelState(view)
		sessionPanel.FileTreeSearchVisible = !sessionPanel.FileTreeSearchVisible
		if !sessionPanel.FileTreeSearchVisible {
			sessionPanel.FileTreeSearch = ""
		}
	})
	writeNoContent(w)
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
	location, ok := fileLocation(data, id)
	if !ok {
		return state.FileNode{}, false
	}
	return data.Project[location.projectID].Session[location.sessionID].Files[location.fileIndex], true
}

type fileLocationResult struct {
	projectID string
	sessionID string
	fileIndex int
}

func fileLocation(data *state.Data, id string) (fileLocationResult, bool) {
	for projectID, projectData := range data.Project {
		for sessionID, sessionData := range projectData.Session {
			for fileIndex := range sessionData.Files {
				if sessionData.Files[fileIndex].ID == id {
					return fileLocationResult{projectID: projectID, sessionID: sessionID, fileIndex: fileIndex}, true
				}
			}
		}
	}
	return fileLocationResult{}, false
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
