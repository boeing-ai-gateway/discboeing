package command

import (
	"testing"

	serverapi "github.com/obot-platform/discobot/server/api"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestDeleteFilesRemovesDescendants(t *testing.T) {
	data := testFileData([]state.FileNode{
		{ID: "root", SessionID: "s", Kind: state.FileKindDirectory},
		{ID: "child", SessionID: "s", ParentID: "root", Kind: state.FileKindDirectory},
		{ID: "grandchild", SessionID: "s", ParentID: "child", Kind: state.FileKindFile},
		{ID: "sibling", SessionID: "s", Kind: state.FileKindFile},
	})

	deleted := deleteFiles(&data, "child")
	if !deleted["child"] || !deleted["grandchild"] || deleted["sibling"] {
		t.Fatalf("deleted = %#v, want child/grandchild only", deleted)
	}
	files := data.Project["p"].Session["s"].Files
	if len(files) != 2 {
		t.Fatalf("remaining files = %#v, want root and sibling", files)
	}
	for _, file := range files {
		if file.ID == "child" || file.ID == "grandchild" {
			t.Fatalf("deleted file %q remained in %#v", file.ID, files)
		}
	}
}

func TestMoveFileReparentsAndBlocksCycles(t *testing.T) {
	data := testFileData([]state.FileNode{
		{ID: "root", SessionID: "s", Kind: state.FileKindDirectory},
		{ID: "child", SessionID: "s", ParentID: "root", Kind: state.FileKindDirectory},
		{ID: "leaf", SessionID: "s", ParentID: "child", Kind: state.FileKindFile},
	})

	if !moveFile(&data, "leaf", "root") {
		t.Fatalf("move leaf to root failed")
	}
	leaf := data.Project["p"].Session["s"].Files[2]
	if leaf.ParentID != "root" || leaf.GitStatus != state.FileGitStatusRenamed {
		t.Fatalf("leaf after move = %#v", leaf)
	}
	if moveFile(&data, "root", "child") {
		t.Fatalf("move root into descendant succeeded, want blocked")
	}
}

func testFileData(files []state.FileNode) state.Data {
	return state.Data{
		Project: map[string]state.ProjectData{
			"p": {
				Project:  serverapi.Project{ID: "p"},
				Sessions: []serverapi.Session{{ID: "s"}},
				Session: map[string]state.SessionData{
					"s": {Session: serverapi.Session{ID: "s"}, Files: files},
				},
			},
		},
	}
}
