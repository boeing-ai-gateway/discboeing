package command

import (
	"testing"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestDeleteFilesRemovesDescendants(t *testing.T) {
	data := state.Data{
		Sessions: []state.Session{
			{
				ID: "s",
				Files: []state.FileNode{
					{ID: "root", SessionID: "s", Kind: state.FileKindDirectory},
					{ID: "child", SessionID: "s", ParentID: "root", Kind: state.FileKindDirectory},
					{ID: "grandchild", SessionID: "s", ParentID: "child", Kind: state.FileKindFile},
					{ID: "sibling", SessionID: "s", Kind: state.FileKindFile},
				},
			},
		},
	}

	deleted := deleteFiles(&data, "child")
	if !deleted["child"] || !deleted["grandchild"] || deleted["sibling"] {
		t.Fatalf("deleted = %#v, want child/grandchild only", deleted)
	}
	files := data.Sessions[0].Files
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
	data := state.Data{
		Sessions: []state.Session{
			{
				ID: "s",
				Files: []state.FileNode{
					{ID: "root", SessionID: "s", Kind: state.FileKindDirectory},
					{ID: "child", SessionID: "s", ParentID: "root", Kind: state.FileKindDirectory},
					{ID: "leaf", SessionID: "s", ParentID: "child", Kind: state.FileKindFile},
				},
			},
		},
	}

	if !moveFile(&data, "leaf", "root") {
		t.Fatalf("move leaf to root failed")
	}
	leaf := data.Sessions[0].Files[2]
	if leaf.ParentID != "root" || leaf.GitStatus != state.FileGitStatusRenamed {
		t.Fatalf("leaf after move = %#v", leaf)
	}
	if moveFile(&data, "root", "child") {
		t.Fatalf("move root into descendant succeeded, want blocked")
	}
}
