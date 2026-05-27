package app

import (
	"net/url"
	"strconv"
	"strings"

	appui "github.com/obot-platform/discobot/discobot/content/components/ui"
	"github.com/obot-platform/discobot/discobot/internal/state"
)

const sessionFileTreeLargeLimit = 100

func sessionsSidebarClass(view state.View) string {
	if view.SessionsSidebarVisible {
		return "sessions-sidebar"
	}
	return "sessions-sidebar sessions-sidebar--hidden"
}

func sessionsForWorkspace(sessions []state.Session, workspaceID string) []state.Session {
	var workspaceSessions []state.Session
	for _, session := range sessions {
		if session.WorkspaceID == workspaceID {
			workspaceSessions = append(workspaceSessions, session)
		}
	}
	return workspaceSessions
}

func sessionFileTree(session state.Session, view state.View) appui.FileTreeData {
	nodes := sessionFileTreeNodes(session.Files, view, session.ID, "", 0)
	totalVisible := len(nodes)
	if totalVisible > sessionFileTreeLargeLimit {
		nodes = nodes[:sessionFileTreeLargeLimit]
	}
	return appui.FileTreeData{
		Label:          "Files",
		Search:         view.FileTreeSearch,
		Density:        appui.FileTreeDensityCompact,
		IconSet:        appui.FileTreeIconSetCodicons,
		TriggerMode:    appui.FileTreeTriggerBoth,
		DragEnabled:    true,
		LargeTreeLimit: sessionFileTreeLargeLimit,
		TotalNodeCount: totalVisible,
		RenderedCount:  len(nodes),
		Controls: appui.FileTreeControls{
			SearchCommand:      sessionFileCommand("/ui/commands/files/search"),
			ExpandAllCommand:   sessionFileCommand("/ui/commands/files/sessions/" + url.PathEscape(session.ID) + "/expand-all"),
			CollapseAllCommand: sessionFileCommand("/ui/commands/files/sessions/" + url.PathEscape(session.ID) + "/collapse-all"),
			RootDropCommand:    sessionFileCommand("/ui/commands/files/root/move"),
		},
		Nodes: nodes,
	}
}

func sessionFileTreeNodes(files []state.FileNode, view state.View, sessionID string, parentID string, depth int) []appui.FileTreeNode {
	return sessionFileTreeNodesWithOptions(files, view, sessionID, parentID, depth, true)
}

func sessionFileTreeNodesWithOptions(files []state.FileNode, view state.View, sessionID string, parentID string, depth int, flattenDirectories bool) []appui.FileTreeNode {
	var nodes []appui.FileTreeNode
	for _, file := range files {
		if file.SessionID != sessionID || file.ParentID != parentID {
			continue
		}

		chain := sessionFileTreeFlattenChain(files, view, sessionID, file, flattenDirectories)
		displayFile := chain[len(chain)-1]
		displayName := sessionFileTreeDisplayName(chain)
		hasChildren := sessionFileTreeHasChildren(files, sessionID, displayFile.ID)
		expanded := view.ExpandedFileIDs[displayFile.ID]
		searchMatched := sessionFileTreeMatchesSearch(files, sessionID, displayFile.ID, displayName, view.FileTreeSearch)
		if !searchMatched {
			continue
		}
		if strings.TrimSpace(view.FileTreeSearch) != "" {
			expanded = true
		}

		node := appui.FileTreeNode{
			ID:                    displayFile.ID,
			Name:                  displayName,
			Kind:                  sessionFileTreeNodeKind(displayFile.Kind),
			Depth:                 depth,
			Expanded:              expanded,
			Selected:              view.SelectedFileID == displayFile.ID,
			HasChildren:           hasChildren,
			GitStatus:             sessionFileTreeGitStatus(sessionFileTreeDerivedStatus(files, displayFile)),
			HasChangedDescendants: displayFile.HasChangedDescendants || sessionFileTreeHasChangedDescendants(files, sessionID, displayFile.ID),
			Icon:                  sessionFileTreeIcon(displayFile.Name),
			IconColorClass:        sessionFileTreeIconColorClass(displayFile.Name),
			CanDrag:               true,
			CanDrop:               displayFile.Kind == state.FileKindDirectory,
			ToggleCommand:         sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/toggle-expanded"),
			SelectCommand:         sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/select"),
			DeleteCommand:         sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/delete"),
			RenameCommand:         sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/rename"),
			NewFileCommand:        sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/children/file"),
			NewFolderCommand:      sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/children/directory"),
			MoveCommand:           sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/move"),
			DropCommand:           sessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/drop"),
		}
		nodes = append(nodes, node)

		if displayFile.Kind == state.FileKindDirectory && expanded {
			nodes = append(nodes, sessionFileTreeNodesWithOptions(files, view, sessionID, displayFile.ID, depth+1, flattenDirectories)...)
		}
	}
	return nodes
}

func sessionFileTreeFlattenChain(files []state.FileNode, view state.View, sessionID string, file state.FileNode, flattenDirectories bool) []state.FileNode {
	chain := []state.FileNode{file}
	if !flattenDirectories || file.Kind != state.FileKindDirectory {
		return chain
	}
	for {
		children := sessionFileTreeChildren(files, sessionID, chain[len(chain)-1].ID)
		if len(children) != 1 || children[0].Kind != state.FileKindDirectory {
			return chain
		}
		// Keep an explicitly selected intermediate directory addressable instead of
		// hiding the row that currently owns user-visible state.
		if view.SelectedFileID == children[0].ID {
			return chain
		}
		chain = append(chain, children[0])
	}
}

func sessionFileTreeChildren(files []state.FileNode, sessionID string, parentID string) []state.FileNode {
	var children []state.FileNode
	for _, file := range files {
		if file.SessionID == sessionID && file.ParentID == parentID {
			children = append(children, file)
		}
	}
	return children
}

func sessionFileTreeDisplayName(chain []state.FileNode) string {
	parts := make([]string, 0, len(chain))
	for _, file := range chain {
		parts = append(parts, file.Name)
	}
	return strings.Join(parts, "/")
}

func sessionFileTreeHasChildren(files []state.FileNode, sessionID string, parentID string) bool {
	return len(sessionFileTreeChildren(files, sessionID, parentID)) > 0
}

func sessionFileTreeMatchesSearch(files []state.FileNode, sessionID string, fileID string, name string, search string) bool {
	query := strings.TrimSpace(strings.ToLower(search))
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(name), query) {
		return true
	}
	for _, file := range files {
		if file.SessionID == sessionID && file.ParentID == fileID && sessionFileTreeMatchesSearch(files, sessionID, file.ID, file.Name, search) {
			return true
		}
	}
	return false
}

func sessionFileTreeHasChangedDescendants(files []state.FileNode, sessionID string, parentID string) bool {
	for _, file := range files {
		if file.SessionID != sessionID || file.ParentID != parentID {
			continue
		}
		if sessionFileTreeDerivedStatus(files, file) != state.FileGitStatusClean {
			return true
		}
		if file.HasChangedDescendants || sessionFileTreeHasChangedDescendants(files, sessionID, file.ID) {
			return true
		}
	}
	return false
}

func sessionFileTreeDerivedStatus(files []state.FileNode, file state.FileNode) state.FileGitStatus {
	path := sessionFileTreePath(files, file)
	return state.DeriveFileGitStatusFromPath(path, map[string]state.FileGitStatus{path: file.GitStatus})
}

func sessionFileTreePath(files []state.FileNode, file state.FileNode) string {
	parts := []string{file.Name}
	parentID := file.ParentID
	for parentID != "" {
		parent, ok := sessionFileTreeFind(files, parentID)
		if !ok {
			break
		}
		parts = append([]string{parent.Name}, parts...)
		parentID = parent.ParentID
	}
	return strings.Join(parts, "/")
}

func sessionFileTreeFind(files []state.FileNode, id string) (state.FileNode, bool) {
	for _, file := range files {
		if file.ID == id {
			return file, true
		}
	}
	return state.FileNode{}, false
}

func sessionFileTreeIcon(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		return "markdown"
	case strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonc"):
		return "json"
	case strings.HasSuffix(lower, ".sh") || strings.HasSuffix(lower, ".bash") || strings.HasSuffix(lower, ".ps1"):
		return "terminal"
	case strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".svg"):
		return "file-media"
	case strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".css") || strings.HasSuffix(lower, ".templ"):
		return "file-code"
	default:
		return "file"
	}
}

func sessionFileTreeIconColorClass(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "file-tree__icon--go"
	case strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonc"):
		return "file-tree__icon--json"
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		return "file-tree__icon--markdown"
	case strings.HasSuffix(lower, ".sh") || strings.HasSuffix(lower, ".bash") || strings.HasSuffix(lower, ".ps1"):
		return "file-tree__icon--shell"
	case strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".svg"):
		return "file-tree__icon--media"
	case strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".css") || strings.HasSuffix(lower, ".templ"):
		return "file-tree__icon--code"
	default:
		return ""
	}
}

func sessionFileTreeNodeKind(kind state.FileKind) appui.FileTreeNodeKind {
	if kind == state.FileKindDirectory {
		return appui.FileTreeNodeDirectory
	}
	return appui.FileTreeNodeFile
}

func sessionFileTreeGitStatus(status state.FileGitStatus) appui.FileTreeGitStatus {
	switch status {
	case state.FileGitStatusModified:
		return appui.FileTreeGitStatusModified
	case state.FileGitStatusAdded:
		return appui.FileTreeGitStatusAdded
	case state.FileGitStatusDeleted:
		return appui.FileTreeGitStatusDeleted
	case state.FileGitStatusRenamed:
		return appui.FileTreeGitStatusRenamed
	case state.FileGitStatusUntracked:
		return appui.FileTreeGitStatusUntracked
	case state.FileGitStatusIgnored:
		return appui.FileTreeGitStatusIgnored
	default:
		return appui.FileTreeGitStatusClean
	}
}

func sessionFileCommand(commandURL string) appui.FileTreeCommand {
	return appui.FileTreeCommand{
		Method: "POST",
		URL:    commandURL,
	}
}

func sessionsSidebarStatusDotClass(status state.SessionStatus) string {
	if status == state.SessionStatusRunning {
		return "sessions-sidebar__status-dot sessions-sidebar__status-dot--running"
	}
	return "sessions-sidebar__status-dot"
}

func sessionsSidebarItemClass(selected bool, expanded bool) string {
	class := "sessions-sidebar__item"
	if expanded {
		class += " sessions-sidebar__item--expanded"
	}
	if selected {
		class += " sessions-sidebar__item--selected"
	}
	return class
}

func sessionsSidebarSessionToggleCommand(id string) string {
	return "@post('/ui/commands/sessions/" + url.PathEscape(id) + "/toggle-expanded')"
}

func sessionsSidebarSessionSelectCommand(id string) string {
	return "@post('/ui/commands/sessions/" + url.PathEscape(id) + "/select')"
}

func sessionsSidebarExpanderLabel(session state.Session, expanded bool) string {
	if expanded {
		return "Collapse " + session.Title
	}
	return "Expand " + session.Title
}

func sessionDiffStat(value int) string {
	return strconv.Itoa(value)
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sessionsSidebarFilterMenu(view state.View) appui.MenuData {
	return appui.MenuData{
		ID:      "sessions-filter-menu",
		Label:   "Open session filters",
		Trigger: IconSliders("icon-sm"),
		Items: []appui.MenuItem{
			{
				Label: "Filter",
				Children: []appui.MenuItem{
					sessionMenuCheckItem(view, "Copilot CLI", "copilot-cli", false),
					sessionMenuCheckItem(view, "Cloud", "cloud", false),
					sessionMenuCheckItem(view, "Claude", "claude", false),
					sessionMenuCheckItem(view, "Completed", "completed", true),
					sessionMenuCheckItem(view, "In Progress", "in-progress", false),
					sessionMenuCheckItem(view, "Input Needed", "input-needed", false),
					sessionMenuCheckItem(view, "Failed", "failed", false),
					sessionMenuCheckItem(view, "Done", "done", true),
					sessionMenuCheckItem(view, "Read", "read", false),
					{Label: "Reset", SeparatorBefore: true},
				},
			},
			sessionMenuCheckItem(view, "Sort by Created", "sort-created", true),
			sessionMenuCheckItem(view, "Sort by Updated", "sort-updated", false),
			sessionMenuCheckItem(view, "Group by Workspace", "group-workspace", true),
			sessionMenuCheckItem(view, "Group by Time", "group-time", false),
			sessionMenuCheckItem(view, "Show Recent Sessions", "show-recent-sessions", true),
			sessionMenuCheckItem(view, "Show All Sessions", "show-all-sessions", false),
			{Label: "Collapse All Groups", SeparatorBefore: true},
		},
	}
}

func sessionMenuCheckItem(view state.View, label string, key string, separatorBefore bool) appui.MenuItem {
	checkState := appui.MenuCheckUnchecked
	if view.SessionMenuChecks[key] {
		checkState = appui.MenuCheckChecked
	}

	return appui.MenuItem{
		Label:           label,
		CheckState:      checkState,
		SeparatorBefore: separatorBefore,
		Command: appui.MenuCommand{
			Method: "POST",
			URL:    "/ui/commands/sessions/menu-checks/" + key + "/toggle",
		},
	}
}
