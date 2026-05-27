package app

import (
	"strconv"
	"testing"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestSessionFileTreeNodesExpansion(t *testing.T) {
	files := []state.FileNode{
		{ID: "root", SessionID: "s", Name: "root", Kind: state.FileKindDirectory},
		{ID: "child", SessionID: "s", ParentID: "root", Name: "child.go", Kind: state.FileKindFile},
	}

	collapsed := sessionFileTreeNodesWithOptions(files, state.View{ExpandedFileIDs: map[string]bool{}}, "s", "", 0, false)
	if len(collapsed) != 1 || collapsed[0].ID != "root" {
		t.Fatalf("collapsed tree = %#v, want only root", collapsed)
	}

	expanded := sessionFileTreeNodesWithOptions(files, state.View{ExpandedFileIDs: map[string]bool{"root": true}}, "s", "", 0, false)
	if len(expanded) != 2 || expanded[1].ID != "child" || expanded[1].Depth != 1 {
		t.Fatalf("expanded tree = %#v, want root and child at depth 1", expanded)
	}
}

func TestSessionFileTreeNodesFlattenDirectories(t *testing.T) {
	files := []state.FileNode{
		{ID: "content", SessionID: "s", Name: "content", Kind: state.FileKindDirectory},
		{ID: "components", SessionID: "s", ParentID: "content", Name: "components", Kind: state.FileKindDirectory},
		{ID: "ui", SessionID: "s", ParentID: "components", Name: "ui", Kind: state.FileKindDirectory},
		{ID: "tree", SessionID: "s", ParentID: "ui", Name: "file_tree.templ", Kind: state.FileKindFile},
	}

	nodes := sessionFileTreeNodesWithOptions(files, state.View{ExpandedFileIDs: map[string]bool{}}, "s", "", 0, true)
	if len(nodes) != 1 {
		t.Fatalf("nodes length = %d, want 1", len(nodes))
	}
	if nodes[0].ID != "ui" || nodes[0].Name != "content/components/ui" {
		t.Fatalf("flattened node = %#v, want terminal ui with joined name", nodes[0])
	}
}

func TestSessionFileTreeSearchKeepsAncestors(t *testing.T) {
	files := []state.FileNode{
		{ID: "root", SessionID: "s", Name: "root", Kind: state.FileKindDirectory},
		{ID: "src", SessionID: "s", ParentID: "root", Name: "src", Kind: state.FileKindDirectory},
		{ID: "main", SessionID: "s", ParentID: "src", Name: "main.go", Kind: state.FileKindFile},
		{ID: "readme", SessionID: "s", ParentID: "root", Name: "README.md", Kind: state.FileKindFile},
	}
	view := state.View{ExpandedFileIDs: map[string]bool{}, FileTreeSearch: "main"}

	nodes := sessionFileTreeNodesWithOptions(files, view, "s", "", 0, false)
	if len(nodes) != 3 {
		t.Fatalf("nodes length = %d, want root/src/main", len(nodes))
	}
	for index, want := range []string{"root", "src", "main"} {
		if nodes[index].ID != want {
			t.Fatalf("nodes[%d].ID = %q, want %q in %#v", index, nodes[index].ID, want, nodes)
		}
	}
}

func TestSessionFileTreeGitStatusAndDescendantIndicators(t *testing.T) {
	files := []state.FileNode{
		{ID: "root", SessionID: "s", Name: "root", Kind: state.FileKindDirectory},
		{ID: "child", SessionID: "s", ParentID: "root", Name: "child.go", Kind: state.FileKindFile, GitStatus: state.FileGitStatusModified},
	}
	view := state.View{ExpandedFileIDs: map[string]bool{"root": true}}

	nodes := sessionFileTreeNodesWithOptions(files, view, "s", "", 0, false)
	if !nodes[0].HasChangedDescendants {
		t.Fatalf("root HasChangedDescendants = false, want true")
	}
	if nodes[1].GitStatus == "" {
		t.Fatalf("child GitStatus empty, want mapped status")
	}
}

func TestSessionFileTreeAppliesLargeTreeLimit(t *testing.T) {
	const fixtureSize = 3000
	files := make([]state.FileNode, 0, fixtureSize)
	for index := 0; index < fixtureSize; index++ {
		files = append(files, state.FileNode{
			ID:        "file-" + strconv.Itoa(index),
			SessionID: "s",
			Name:      "file.go",
			Kind:      state.FileKindFile,
		})
	}

	tree := sessionFileTree(state.Session{ID: "s", Files: files}, state.View{ExpandedFileIDs: map[string]bool{}})
	if len(tree.Nodes) != sessionFileTreeLargeLimit {
		t.Fatalf("rendered nodes = %d, want %d", len(tree.Nodes), sessionFileTreeLargeLimit)
	}
	if tree.TotalNodeCount != fixtureSize || tree.RenderedCount != sessionFileTreeLargeLimit {
		t.Fatalf("counts = rendered %d total %d", tree.RenderedCount, tree.TotalNodeCount)
	}
}

func TestSessionFileTreeIconMapping(t *testing.T) {
	if icon := sessionFileTreeIcon("README.md"); icon != "markdown" {
		t.Fatalf("markdown icon = %q", icon)
	}
	if class := sessionFileTreeIconColorClass("main.go"); class == "" {
		t.Fatalf("go icon color class empty")
	}
}
