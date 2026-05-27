package app

import (
	"strings"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func editorOpenFiles(session state.Session, view state.View) []state.FileNode {
	openFiles := make([]state.FileNode, 0, len(view.OpenFileIDs))
	for _, id := range view.OpenFileIDs {
		if file, ok := editorFileByID(session.Files, id); ok && file.Kind == state.FileKindFile {
			openFiles = append(openFiles, file)
		}
	}
	return openFiles
}

func editorActiveFile(session state.Session, view state.View) (state.FileNode, bool) {
	if view.ActiveFileID != "" {
		if file, ok := editorFileByID(session.Files, view.ActiveFileID); ok && file.Kind == state.FileKindFile {
			return file, true
		}
	}
	openFiles := editorOpenFiles(session, view)
	if len(openFiles) == 0 {
		return state.FileNode{}, false
	}
	return openFiles[len(openFiles)-1], true
}

func editorFilePath(files []state.FileNode, file state.FileNode) string {
	parts := []string{file.Name}
	parentID := file.ParentID
	for parentID != "" {
		parent, ok := editorFileByID(files, parentID)
		if !ok {
			break
		}
		parts = append([]string{parent.Name}, parts...)
		parentID = parent.ParentID
	}
	return strings.Join(parts, "/")
}

func editorFileByID(files []state.FileNode, id string) (state.FileNode, bool) {
	for _, file := range files {
		if file.ID == id {
			return file, true
		}
	}
	return state.FileNode{}, false
}

func editorTabClass(fileID string, activeFileID string) string {
	class := "editor-pane__tab"
	if fileID == activeFileID {
		class += " editor-pane__tab--active"
	}
	return class
}
