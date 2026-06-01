package app

import (
	"strconv"
	"strings"
	"testing"

	appui "github.com/obot-platform/discobot/discobot/content/components/ui"
	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestSessionFileTreeNodesExpansion(t *testing.T) {
	files := []state.FileNode{
		{ID: "root", SessionID: "s", Name: "root", Kind: state.FileKindDirectory},
		{ID: "child", SessionID: "s", ParentID: "root", Name: "child.go", Kind: state.FileKindFile},
	}

	collapsed := sessionFileTreeNodesWithOptions(files, testViewWithSessionPanel(state.SessionPanelState{ExpandedFileIDs: map[string]bool{}}), "s", "", 0, false)
	if len(collapsed) != 1 || collapsed[0].ID != "root" {
		t.Fatalf("collapsed tree = %#v, want only root", collapsed)
	}

	expanded := sessionFileTreeNodesWithOptions(files, testViewWithSessionPanel(state.SessionPanelState{ExpandedFileIDs: map[string]bool{"root": true}}), "s", "", 0, false)
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

	nodes := sessionFileTreeNodesWithOptions(files, testViewWithSessionPanel(state.SessionPanelState{ExpandedFileIDs: map[string]bool{}}), "s", "", 0, true)
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
	view := testViewWithSessionPanel(state.SessionPanelState{ExpandedFileIDs: map[string]bool{}, FileTreeSearch: "main"})

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
	view := testViewWithSessionPanel(state.SessionPanelState{ExpandedFileIDs: map[string]bool{"root": true}})

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

	tree := sessionFileTree(state.Session{ID: "s", Files: files}, testViewWithSessionPanel(state.SessionPanelState{ExpandedFileIDs: map[string]bool{}}))
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

func TestSessionDiffFilesReturnsChangedFiles(t *testing.T) {
	session := state.Session{
		ID: "s",
		Files: []state.FileNode{
			{ID: "root", SessionID: "s", Name: "root", Kind: state.FileKindDirectory},
			{ID: "src", SessionID: "s", ParentID: "root", Name: "src", Kind: state.FileKindDirectory},
			{ID: "main", SessionID: "s", ParentID: "src", Name: "main.go", Kind: state.FileKindFile, GitStatus: state.FileGitStatusModified, Approved: true},
			{ID: "readme", SessionID: "s", ParentID: "root", Name: "README.md", Kind: state.FileKindFile},
			{ID: "deleted", SessionID: "s", ParentID: "root", Name: "old.go", Kind: state.FileKindFile, GitStatus: state.FileGitStatusDeleted},
		},
	}

	files := sessionDiffFiles(session)
	if len(files) != 2 {
		t.Fatalf("diff files length = %d, want 2: %#v", len(files), files)
	}
	if files[0].Path != "root/src/main.go" || files[0].Status != state.FileGitStatusModified {
		t.Fatalf("files[0] = %#v, want modified root/src/main.go", files[0])
	}
	if !files[0].Approved {
		t.Fatalf("files[0].Approved = false, want true")
	}
	if files[1].Path != "root/old.go" || files[1].Status != state.FileGitStatusDeleted {
		t.Fatalf("files[1] = %#v, want deleted root/old.go", files[1])
	}
}

func TestSessionApprovalSummaryCountsApprovedFiles(t *testing.T) {
	summary := sessionApprovalSummaryFor([]sessionDiffFile{
		{Approved: true},
		{},
		{Approved: true},
	})
	if summary.Total != 3 || summary.Approved != 2 {
		t.Fatalf("summary = %#v, want 2 of 3 approved", summary)
	}
	if sessionApprovalComplete(summary) {
		t.Fatalf("approval summary should not be complete")
	}
}

func TestSessionViewModeDefaultsToFiles(t *testing.T) {
	sessionState := state.SessionPanelState{}
	if mode := sessionViewMode(sessionState, "s"); mode != state.SessionViewModeFiles {
		t.Fatalf("default mode = %q, want files", mode)
	}

	sessionState.SessionViewModes = map[string]state.SessionViewMode{"s": state.SessionViewModeDiff}
	if mode := sessionViewMode(sessionState, "s"); mode != state.SessionViewModeDiff {
		t.Fatalf("stored mode = %q, want diff", mode)
	}
}

func TestSessionDetailSectionsDefaultToHidden(t *testing.T) {
	sections := sessionDetailSectionsFor("s", state.SessionPanelState{})
	if sections.Workspace || sections.SideChats || sections.Hooks || sections.Review {
		t.Fatalf("sections = %#v, want all hidden by default", sections)
	}

	sessionState := state.SessionPanelState{
		VisibleSessionDetailSections: map[string]bool{
			state.SessionDetailSectionKey("s", state.SessionDetailSectionHooks):  true,
			state.SessionDetailSectionKey("s", state.SessionDetailSectionReview): true,
		},
	}
	sections = sessionDetailSectionsFor("s", sessionState)
	if sections.Workspace || sections.SideChats || !sections.Hooks || !sections.Review {
		t.Fatalf("sections = %#v, want hooks and review visible only", sections)
	}
	if !sessionDetailSectionsAnyVisible(sections) {
		t.Fatalf("sections should report visible panels")
	}
}

func TestSessionDetailSectionLabels(t *testing.T) {
	if summary := sessionsSidebarWorkspaceSummary(appui.FileTreeData{TotalNodeCount: 1}, nil, state.SessionViewModeFiles); summary != "1 file" {
		t.Fatalf("workspace summary = %q, want 1 file", summary)
	}
	if summary := sessionSideChatSectionSummary([]sessionSideChatItem{{}}); summary != "1 chat" {
		t.Fatalf("side chat summary = %q, want 1 chat", summary)
	}
	if class := sessionsSidebarDetailSectionClass(true); !strings.Contains(class, "sessions-sidebar--detail-section--visible") {
		t.Fatalf("section class = %q, want visible modifier", class)
	}
	if class := sessionsSidebarDetailLauncherClass(true); !strings.Contains(class, "sessions-sidebar--detail-launcher--active") {
		t.Fatalf("launcher class = %q, want active modifier", class)
	}
	if label := sessionsSidebarDetailLauncherLabel("Hooks", false); label != "Open Hooks panel" {
		t.Fatalf("launcher label = %q, want open label", label)
	}
}

func TestSessionSideChatsSelectStoredOrFirst(t *testing.T) {
	session := state.Session{
		ID: "s",
		SideChats: []state.Thread{
			{ID: "thread-a", Title: "A"},
			{ID: "thread-b", Title: "B"},
		},
	}

	items := sessionSideChats(session, state.SessionPanelState{SelectedSideChatID: "thread-b"})
	if len(items) != 2 || items[0].Selected || !items[1].Selected {
		t.Fatalf("items = %#v, want thread-b selected", items)
	}

	items = sessionSideChats(session, state.SessionPanelState{SelectedSideChatID: "missing"})
	if len(items) != 2 || !items[0].Selected || items[1].Selected {
		t.Fatalf("items = %#v, want first chat selected", items)
	}
}

func TestSessionSideChatLabels(t *testing.T) {
	if label := sessionSideChatCountLabel(1); label != "1 msg" {
		t.Fatalf("single message label = %q", label)
	}
	if label := sessionSideChatCountLabel(3); label != "3 msgs" {
		t.Fatalf("message label = %q", label)
	}
	if class := sessionSideChatClass(true, true); !strings.Contains(class, "sessions-sidebar--side-chat--selected") || !strings.Contains(class, "sessions-sidebar--side-chat--unread") {
		t.Fatalf("class = %q, want selected and unread modifiers", class)
	}
}

func TestSessionHookSummaryPrioritizesRunning(t *testing.T) {
	summary := sessionHookSummary([]state.SessionHook{
		{Status: state.HookRunStatusSuccess},
		{Status: state.HookRunStatusFailure},
		{Status: state.HookRunStatusRunning},
	})
	if summary.Passed != 1 || summary.Total != 3 {
		t.Fatalf("summary counts = %#v, want 1 passed of 3", summary)
	}
	if summary.State != state.HookRunStatusRunning || summary.Label != "Running" {
		t.Fatalf("summary state = %#v, want running", summary)
	}
}

func testViewWithSessionPanel(sessionState state.SessionPanelState) state.View {
	view := state.DefaultView()
	view.GlobalPanelLayout.SessionSidebar.State = sessionState
	return view
}
