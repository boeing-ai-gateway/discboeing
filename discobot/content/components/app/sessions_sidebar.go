package app

import (
	"net/url"
	"strconv"
	"strings"

	appui "github.com/obot-platform/discobot/discobot/content/components/ui"
	"github.com/obot-platform/discobot/discobot/internal/state"
	serverapi "github.com/obot-platform/discobot/server/api"
)

const sessionFileTreeLargeLimit = 240

type sessionDiffFile struct {
	ID       string
	Path     string
	Status   state.FileGitStatus
	Icon     string
	Approved bool
}

type sessionApprovalSummary struct {
	Total    int
	Approved int
}

type sessionHooksSummary struct {
	Total  int
	Passed int
	State  state.HookRunStatus
	Label  string
}

type sessionSideChatItem struct {
	Thread   state.Thread
	Selected bool
}

type sessionDetailSections struct {
	Workspace bool
	SideChats bool
	Hooks     bool
	Review    bool
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

func sessionsSidebarWorkspaceName(workspace serverapi.Workspace) string {
	if workspace.DisplayName != nil && *workspace.DisplayName != "" {
		return *workspace.DisplayName
	}
	if workspace.Path != "" {
		return workspace.Path
	}
	return workspace.ID
}

func sessionViewMode(sessionState state.SessionPanelState, sessionID string) state.SessionViewMode {
	if sessionState.SessionViewModes == nil {
		return state.SessionViewModeFiles
	}
	return state.NormalizeSessionViewMode(sessionState.SessionViewModes[sessionID])
}

func sessionDetailSectionsFor(sessionID string, sessionState state.SessionPanelState) sessionDetailSections {
	return sessionDetailSections{
		Workspace: sessionDetailSectionVisible(sessionID, state.SessionDetailSectionWorkspace, sessionState),
		SideChats: sessionDetailSectionVisible(sessionID, state.SessionDetailSectionSideChats, sessionState),
		Hooks:     sessionDetailSectionVisible(sessionID, state.SessionDetailSectionHooks, sessionState),
		Review:    sessionDetailSectionVisible(sessionID, state.SessionDetailSectionReview, sessionState),
	}
}

func sessionDetailSectionVisible(sessionID string, section state.SessionDetailSection, sessionState state.SessionPanelState) bool {
	key := state.SessionDetailSectionKey(sessionID, section)
	return sessionState.VisibleSessionDetailSections[key]
}

func sessionDetailSectionsAnyVisible(sections sessionDetailSections) bool {
	return sections.Workspace || sections.SideChats || sections.Hooks || sections.Review
}

func sessionSideChats(session state.Session, sessionState state.SessionPanelState) []sessionSideChatItem {
	selectedID := sessionSelectedSideChatID(session, sessionState)
	items := make([]sessionSideChatItem, 0, len(session.SideChats))
	for _, thread := range session.SideChats {
		items = append(items, sessionSideChatItem{
			Thread:   thread,
			Selected: thread.ID == selectedID,
		})
	}
	return items
}

func sessionSelectedSideChatID(session state.Session, sessionState state.SessionPanelState) string {
	for _, thread := range session.SideChats {
		if thread.ID == sessionState.SelectedSideChatID {
			return thread.ID
		}
	}
	if len(session.SideChats) == 0 {
		return ""
	}
	return session.SideChats[0].ID
}

func sessionFileTree(session state.Session, view state.View) appui.FileTreeData {
	sessionState := sessionPanelState(view)
	nodes := sessionFileTreeNodes(session.Files, view, session.ID, "", 0)
	totalVisible := len(nodes)
	if totalVisible > sessionFileTreeLargeLimit {
		nodes = nodes[:sessionFileTreeLargeLimit]
	}
	return appui.FileTreeData{
		Label:          "",
		Search:         sessionState.FileTreeSearch,
		SearchVisible:  sessionState.FileTreeSearchVisible,
		Density:        appui.FileTreeDensityCompact,
		IconSet:        appui.FileTreeIconSetLucide,
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

func sessionDiffFiles(session state.Session) []sessionDiffFile {
	files := make([]sessionDiffFile, 0, len(session.Files))
	for _, file := range session.Files {
		if file.Kind != state.FileKindFile {
			continue
		}
		status := sessionFileTreeDerivedStatus(session.Files, file)
		if status == "" || status == state.FileGitStatusClean {
			continue
		}
		files = append(files, sessionDiffFile{
			ID:       file.ID,
			Path:     sessionFileTreePath(session.Files, file),
			Status:   status,
			Icon:     sessionFileTreeIcon(file.Name),
			Approved: file.Approved,
		})
	}
	return files
}

func sessionFileTreeNodes(files []state.FileNode, view state.View, sessionID string, parentID string, depth int) []appui.FileTreeNode {
	return sessionFileTreeNodesWithOptions(files, view, sessionID, parentID, depth, true)
}

func sessionFileTreeNodesWithOptions(files []state.FileNode, view state.View, sessionID string, parentID string, depth int, flattenDirectories bool) []appui.FileTreeNode {
	sessionState := sessionPanelState(view)
	var nodes []appui.FileTreeNode
	for _, file := range files {
		if file.SessionID != sessionID || file.ParentID != parentID {
			continue
		}

		chain := sessionFileTreeFlattenChain(files, view, sessionID, file, flattenDirectories)
		displayFile := chain[len(chain)-1]
		displayName := sessionFileTreeDisplayName(chain)
		hasChildren := sessionFileTreeHasChildren(files, sessionID, displayFile.ID)
		expanded := sessionState.ExpandedFileIDs[displayFile.ID]
		searchMatched := sessionFileTreeMatchesSearch(files, sessionID, displayFile.ID, displayName, sessionState.FileTreeSearch)
		if !searchMatched {
			continue
		}
		if strings.TrimSpace(sessionState.FileTreeSearch) != "" {
			expanded = true
		}

		node := appui.FileTreeNode{
			ID:                    displayFile.ID,
			Name:                  displayName,
			Kind:                  sessionFileTreeNodeKind(displayFile.Kind),
			Depth:                 depth,
			Expanded:              expanded,
			Selected:              sessionState.SelectedFileID == displayFile.ID,
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
	sessionState := sessionPanelState(view)
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
		if sessionState.SelectedFileID == children[0].ID {
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
		return "file-tree--icon--go"
	case strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".jsonc"):
		return "file-tree--icon--json"
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		return "file-tree--icon--markdown"
	case strings.HasSuffix(lower, ".sh") || strings.HasSuffix(lower, ".bash") || strings.HasSuffix(lower, ".ps1"):
		return "file-tree--icon--shell"
	case strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".svg"):
		return "file-tree--icon--media"
	case strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".css") || strings.HasSuffix(lower, ".templ"):
		return "file-tree--icon--code"
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

func sessionsSidebarCommand(commandURL string) string {
	return "@discobotCommand(" + strconv.Quote(commandURL) + ", {method: \"POST\"})"
}

func sessionsSidebarSessionToggleCommand(id string) string {
	return "@post('/ui/commands/sessions/" + url.PathEscape(id) + "/toggle-expanded')"
}

func sessionsSidebarSessionSelectCommand(id string) string {
	return "@post('/ui/commands/sessions/" + url.PathEscape(id) + "/select')"
}

func sessionsSidebarHideCommand() string {
	return sessionsSidebarCommand("/ui/commands/sidebar/hide")
}

func sessionsSidebarSessionViewModeCommand(id string, mode state.SessionViewMode) string {
	return sessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/view-mode/" + url.PathEscape(string(mode)))
}

func sessionsSidebarSessionDetailSectionShowCommand(id string, section state.SessionDetailSection) string {
	return sessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/detail-sections/" + url.PathEscape(string(section)) + "/show")
}

func sessionsSidebarSessionDetailSectionHideCommand(id string, section state.SessionDetailSection) string {
	return sessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/detail-sections/" + url.PathEscape(string(section)) + "/hide")
}

func sessionsSidebarSessionDiffSummaryCommand(id string) string {
	return sessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/diff-summary")
}

func sessionsSidebarSideChatSelectCommand(sessionID string, threadID string) string {
	return sessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(sessionID) + "/side-chats/" + url.PathEscape(threadID) + "/select")
}

func sessionsSidebarFileSelectCommand(id string) string {
	return sessionsSidebarCommand("/ui/commands/files/" + url.PathEscape(id) + "/select")
}

func sessionsSidebarFileSearchToggleCommand() string {
	return sessionsSidebarCommand("/ui/commands/files/search/toggle")
}

func sessionsSidebarFileApprovalToggleCommand(id string) string {
	return sessionsSidebarCommand("/ui/commands/files/" + url.PathEscape(id) + "/approval/toggle")
}

func sessionsSidebarCommitCommand(id string) string {
	return sessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/commit")
}

func sessionsSidebarExpanderLabel(session state.Session, expanded bool) string {
	if expanded {
		return "Collapse " + session.Title
	}
	return "Expand " + session.Title
}

func sessionsSidebarViewButtonClass(viewMode state.SessionViewMode, buttonMode state.SessionViewMode) string {
	class := "sessions-sidebar--view-button"
	if viewMode == buttonMode {
		class += " sessions-sidebar--view-button--active"
	}
	return class
}

func sessionsSidebarIconButtonClass(active bool) string {
	class := "sessions-sidebar--icon-button"
	if active {
		class += " sessions-sidebar--icon-button--active"
	}
	return class
}

func sessionsSidebarDetailSectionClass(visible bool) string {
	class := "sessions-sidebar--detail-section"
	if visible {
		class += " sessions-sidebar--detail-section--visible"
	}
	return class
}

func sessionsSidebarDetailLauncherClass(active bool) string {
	class := "sessions-sidebar--detail-launcher"
	if active {
		class += " sessions-sidebar--detail-launcher--active"
	}
	return class
}

func sessionsSidebarDetailLauncherLabel(label string, active bool) string {
	if active {
		return label + " panel is open"
	}
	return "Open " + label + " panel"
}

func sessionsSidebarWorkspaceSummary(fileTree appui.FileTreeData, diffFiles []sessionDiffFile, viewMode state.SessionViewMode) string {
	if viewMode == state.SessionViewModeDiff {
		return strconv.Itoa(len(diffFiles)) + " changes"
	}
	if fileTree.TotalNodeCount == 1 {
		return "1 file"
	}
	return strconv.Itoa(fileTree.TotalNodeCount) + " files"
}

func sessionHookSectionSummary(hooks []state.SessionHook) string {
	if len(hooks) == 0 {
		return "0 hooks"
	}
	summary := sessionHookSummary(hooks)
	return strconv.Itoa(summary.Passed) + "/" + strconv.Itoa(summary.Total) + " " + strings.ToLower(summary.Label)
}

func sessionSideChatSectionSummary(sideChats []sessionSideChatItem) string {
	if len(sideChats) == 1 {
		return "1 chat"
	}
	return strconv.Itoa(len(sideChats)) + " chats"
}

func sessionDiffFileStatusClass(status state.FileGitStatus) string {
	if status == "" {
		return ""
	}
	return "sessions-sidebar--diff-file-status--" + string(status)
}

func sessionDiffFileStatusText(status state.FileGitStatus) string {
	switch status {
	case state.FileGitStatusModified:
		return "M"
	case state.FileGitStatusAdded:
		return "A"
	case state.FileGitStatusDeleted:
		return "D"
	case state.FileGitStatusRenamed:
		return "R"
	case state.FileGitStatusUntracked:
		return "U"
	case state.FileGitStatusIgnored:
		return "I"
	default:
		return ""
	}
}

func sessionDiffFileStatusTitle(status state.FileGitStatus) string {
	if status == "" || status == state.FileGitStatusClean {
		return ""
	}
	return "Git status: " + string(status)
}

func sessionDiffFileApprovalClass(approved bool) string {
	class := "sessions-sidebar--approval-toggle"
	if approved {
		class += " sessions-sidebar--approval-toggle--approved"
	}
	return class
}

func sessionDiffFileApprovalLabel(approved bool) string {
	if approved {
		return "Approved"
	}
	return "Not approved"
}

func sessionApprovalSummaryFor(diffFiles []sessionDiffFile) sessionApprovalSummary {
	summary := sessionApprovalSummary{
		Total: len(diffFiles),
	}
	for _, file := range diffFiles {
		if file.Approved {
			summary.Approved++
		}
	}
	return summary
}

func sessionApprovalStatusLabel(summary sessionApprovalSummary) string {
	if summary.Total == 0 {
		return "No changed files"
	}
	if summary.Approved == summary.Total {
		return "All files approved"
	}
	return strconv.Itoa(summary.Approved) + "/" + strconv.Itoa(summary.Total) + " approved"
}

func sessionApprovalStatusClass(summary sessionApprovalSummary) string {
	class := "sessions-sidebar--approval-status"
	if summary.Total > 0 && summary.Approved == summary.Total {
		class += " sessions-sidebar--approval-status--complete"
	}
	return class
}

func sessionApprovalComplete(summary sessionApprovalSummary) bool {
	return summary.Total > 0 && summary.Approved == summary.Total
}

func sessionHookSummary(hooks []state.SessionHook) sessionHooksSummary {
	summary := sessionHooksSummary{
		Total: len(hooks),
		State: state.HookRunStatusSuccess,
		Label: "Passed",
	}
	for _, hook := range hooks {
		switch hook.Status {
		case state.HookRunStatusSuccess:
			summary.Passed++
		case state.HookRunStatusRunning:
			summary.State = state.HookRunStatusRunning
			summary.Label = "Running"
		case state.HookRunStatusFailure:
			if summary.State != state.HookRunStatusRunning {
				summary.State = state.HookRunStatusFailure
				summary.Label = "Failed"
			}
		default:
			if summary.State != state.HookRunStatusRunning && summary.State != state.HookRunStatusFailure {
				summary.State = state.HookRunStatusPending
				summary.Label = "Pending"
			}
		}
	}
	return summary
}

func sessionHookStatusClass(status state.HookRunStatus) string {
	if status == "" {
		status = state.HookRunStatusPending
	}
	return "sessions-sidebar--hook-dot--" + string(status)
}

func sessionHookStatusLabel(status state.HookRunStatus) string {
	switch status {
	case state.HookRunStatusRunning:
		return "Running"
	case state.HookRunStatusSuccess:
		return "Passed"
	case state.HookRunStatusFailure:
		return "Failed"
	default:
		return "Pending"
	}
}

func sessionSideChatClass(selected bool, unread bool) string {
	class := "sessions-sidebar--side-chat"
	if selected {
		class += " sessions-sidebar--side-chat--selected"
	}
	if unread {
		class += " sessions-sidebar--side-chat--unread"
	}
	return class
}

func sessionSideChatCountLabel(count int) string {
	if count == 1 {
		return "1 msg"
	}
	return strconv.Itoa(count) + " msgs"
}

func sessionsSidebarFilterMenu(view state.View) appui.MenuData {
	return appui.MenuData{
		ID:            "sessions-filter-menu",
		Label:         "Open session filters",
		Trigger:       IconSliders("icon-sm"),
		AnchorToClick: true,
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
	sessionState := sessionPanelState(view)
	checkState := appui.MenuCheckUnchecked
	if sessionState.SessionMenuChecks[key] {
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
