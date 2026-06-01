package helpers

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/obot-platform/discobot/discobot/internal/state"
	serverapi "github.com/obot-platform/discobot/server/api"
)

const sessionFileTreeLargeLimit = 240

type SessionDiffFile struct {
	ID       string
	Path     string
	Status   state.FileGitStatus
	Icon     string
	Approved bool
}

type SessionApprovalSummary struct {
	Total    int
	Approved int
}

type SessionHooksSummary struct {
	Total  int
	Passed int
	State  state.HookRunStatus
	Label  string
}

type SessionSideChatItem struct {
	Thread   state.Thread
	Selected bool
}

type SessionDetailSections struct {
	Workspace bool
	SideChats bool
	Hooks     bool
	Review    bool
}

func SessionsForWorkspace(sessions []state.Session, workspaceID string) []state.Session {
	var workspaceSessions []state.Session
	for _, session := range sessions {
		if session.WorkspaceID == workspaceID {
			workspaceSessions = append(workspaceSessions, session)
		}
	}
	return workspaceSessions
}

func SessionsSidebarWorkspaceName(workspace serverapi.Workspace) string {
	if workspace.DisplayName != nil && *workspace.DisplayName != "" {
		return *workspace.DisplayName
	}
	if workspace.Path != "" {
		return workspace.Path
	}
	return workspace.ID
}

func SessionViewMode(sessionState state.SessionPanelState, sessionID string) state.SessionViewMode {
	if sessionState.SessionViewModes == nil {
		return state.SessionViewModeFiles
	}
	return state.NormalizeSessionViewMode(sessionState.SessionViewModes[sessionID])
}

func SessionDetailSectionsFor(sessionID string, sessionState state.SessionPanelState) SessionDetailSections {
	return SessionDetailSections{
		Workspace: SessionDetailSectionVisible(sessionID, state.SessionDetailSectionWorkspace, sessionState),
		SideChats: SessionDetailSectionVisible(sessionID, state.SessionDetailSectionSideChats, sessionState),
		Hooks:     SessionDetailSectionVisible(sessionID, state.SessionDetailSectionHooks, sessionState),
		Review:    SessionDetailSectionVisible(sessionID, state.SessionDetailSectionReview, sessionState),
	}
}

func SessionDetailSectionVisible(sessionID string, section state.SessionDetailSection, sessionState state.SessionPanelState) bool {
	key := state.SessionDetailSectionKey(sessionID, section)
	return sessionState.VisibleSessionDetailSections[key]
}

func SessionDetailSectionsAnyVisible(sections SessionDetailSections) bool {
	return sections.Workspace || sections.SideChats || sections.Hooks || sections.Review
}

func SessionSideChats(session state.Session, sessionState state.SessionPanelState) []SessionSideChatItem {
	selectedID := SessionSelectedSideChatID(session, sessionState)
	items := make([]SessionSideChatItem, 0, len(session.SideChats))
	for _, thread := range session.SideChats {
		items = append(items, SessionSideChatItem{
			Thread:   thread,
			Selected: thread.ID == selectedID,
		})
	}
	return items
}

func SessionSelectedSideChatID(session state.Session, sessionState state.SessionPanelState) string {
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

func SessionFileTree(session state.Session, view state.View) FileTreeData {
	sessionState := SessionPanelState(view)
	nodes := SessionFileTreeNodes(session.Files, view, session.ID, "", 0)
	totalVisible := len(nodes)
	if totalVisible > sessionFileTreeLargeLimit {
		nodes = nodes[:sessionFileTreeLargeLimit]
	}
	return FileTreeData{
		Label:          "",
		Search:         sessionState.FileTreeSearch,
		SearchVisible:  sessionState.FileTreeSearchVisible,
		Density:        FileTreeDensityCompact,
		IconSet:        FileTreeIconSetLucide,
		TriggerMode:    FileTreeTriggerBoth,
		DragEnabled:    true,
		LargeTreeLimit: sessionFileTreeLargeLimit,
		TotalNodeCount: totalVisible,
		RenderedCount:  len(nodes),
		Controls: FileTreeControls{
			SearchCommand:      SessionFileCommand("/ui/commands/files/search"),
			ExpandAllCommand:   SessionFileCommand("/ui/commands/files/sessions/" + url.PathEscape(session.ID) + "/expand-all"),
			CollapseAllCommand: SessionFileCommand("/ui/commands/files/sessions/" + url.PathEscape(session.ID) + "/collapse-all"),
			RootDropCommand:    SessionFileCommand("/ui/commands/files/root/move"),
		},
		Nodes: nodes,
	}
}

func SessionDiffFiles(session state.Session) []SessionDiffFile {
	files := make([]SessionDiffFile, 0, len(session.Files))
	for _, file := range session.Files {
		if file.Kind != state.FileKindFile {
			continue
		}
		status := SessionFileTreeDerivedStatus(session.Files, file)
		if status == "" || status == state.FileGitStatusClean {
			continue
		}
		files = append(files, SessionDiffFile{
			ID:       file.ID,
			Path:     SessionFileTreePath(session.Files, file),
			Status:   status,
			Icon:     SessionFileTreeIcon(file.Name),
			Approved: file.Approved,
		})
	}
	return files
}

func SessionFileTreeNodes(files []state.FileNode, view state.View, sessionID string, parentID string, depth int) []FileTreeNode {
	return SessionFileTreeNodesWithOptions(files, view, sessionID, parentID, depth, true)
}

func SessionFileTreeNodesWithOptions(files []state.FileNode, view state.View, sessionID string, parentID string, depth int, flattenDirectories bool) []FileTreeNode {
	sessionState := SessionPanelState(view)
	var nodes []FileTreeNode
	for _, file := range files {
		if file.SessionID != sessionID || file.ParentID != parentID {
			continue
		}

		chain := SessionFileTreeFlattenChain(files, view, sessionID, file, flattenDirectories)
		displayFile := chain[len(chain)-1]
		displayName := SessionFileTreeDisplayName(chain)
		hasChildren := SessionFileTreeHasChildren(files, sessionID, displayFile.ID)
		expanded := sessionState.ExpandedFileIDs[displayFile.ID]
		searchMatched := SessionFileTreeMatchesSearch(files, sessionID, displayFile.ID, displayName, sessionState.FileTreeSearch)
		if !searchMatched {
			continue
		}
		if strings.TrimSpace(sessionState.FileTreeSearch) != "" {
			expanded = true
		}

		node := FileTreeNode{
			ID:                    displayFile.ID,
			Name:                  displayName,
			Kind:                  SessionFileTreeNodeKind(displayFile.Kind),
			Depth:                 depth,
			Expanded:              expanded,
			Selected:              sessionState.SelectedFileID == displayFile.ID,
			HasChildren:           hasChildren,
			GitStatus:             SessionFileTreeGitStatus(SessionFileTreeDerivedStatus(files, displayFile)),
			HasChangedDescendants: displayFile.HasChangedDescendants || SessionFileTreeHasChangedDescendants(files, sessionID, displayFile.ID),
			Icon:                  SessionFileTreeIcon(displayFile.Name),
			IconColorClass:        SessionFileTreeIconColorClass(displayFile.Name),
			CanDrag:               true,
			CanDrop:               displayFile.Kind == state.FileKindDirectory,
			ToggleCommand:         SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/toggle-expanded"),
			SelectCommand:         SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/select"),
			DeleteCommand:         SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/delete"),
			RenameCommand:         SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/rename"),
			NewFileCommand:        SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/children/file"),
			NewFolderCommand:      SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/children/directory"),
			MoveCommand:           SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/move"),
			DropCommand:           SessionFileCommand("/ui/commands/files/" + url.PathEscape(displayFile.ID) + "/drop"),
		}
		nodes = append(nodes, node)

		if displayFile.Kind == state.FileKindDirectory && expanded {
			nodes = append(nodes, SessionFileTreeNodesWithOptions(files, view, sessionID, displayFile.ID, depth+1, flattenDirectories)...)
		}
	}
	return nodes
}

func SessionFileTreeFlattenChain(files []state.FileNode, view state.View, sessionID string, file state.FileNode, flattenDirectories bool) []state.FileNode {
	sessionState := SessionPanelState(view)
	chain := []state.FileNode{file}
	if !flattenDirectories || file.Kind != state.FileKindDirectory {
		return chain
	}
	for {
		children := SessionFileTreeChildren(files, sessionID, chain[len(chain)-1].ID)
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

func SessionFileTreeChildren(files []state.FileNode, sessionID string, parentID string) []state.FileNode {
	var children []state.FileNode
	for _, file := range files {
		if file.SessionID == sessionID && file.ParentID == parentID {
			children = append(children, file)
		}
	}
	return children
}

func SessionFileTreeDisplayName(chain []state.FileNode) string {
	parts := make([]string, 0, len(chain))
	for _, file := range chain {
		parts = append(parts, file.Name)
	}
	return strings.Join(parts, "/")
}

func SessionFileTreeHasChildren(files []state.FileNode, sessionID string, parentID string) bool {
	return len(SessionFileTreeChildren(files, sessionID, parentID)) > 0
}

func SessionFileTreeMatchesSearch(files []state.FileNode, sessionID string, fileID string, name string, search string) bool {
	query := strings.TrimSpace(strings.ToLower(search))
	if query == "" {
		return true
	}
	if strings.Contains(strings.ToLower(name), query) {
		return true
	}
	for _, file := range files {
		if file.SessionID == sessionID && file.ParentID == fileID && SessionFileTreeMatchesSearch(files, sessionID, file.ID, file.Name, search) {
			return true
		}
	}
	return false
}

func SessionFileTreeHasChangedDescendants(files []state.FileNode, sessionID string, parentID string) bool {
	for _, file := range files {
		if file.SessionID != sessionID || file.ParentID != parentID {
			continue
		}
		if SessionFileTreeDerivedStatus(files, file) != state.FileGitStatusClean {
			return true
		}
		if file.HasChangedDescendants || SessionFileTreeHasChangedDescendants(files, sessionID, file.ID) {
			return true
		}
	}
	return false
}

func SessionFileTreeDerivedStatus(files []state.FileNode, file state.FileNode) state.FileGitStatus {
	path := SessionFileTreePath(files, file)
	return state.DeriveFileGitStatusFromPath(path, map[string]state.FileGitStatus{path: file.GitStatus})
}

func SessionFileTreePath(files []state.FileNode, file state.FileNode) string {
	parts := []string{file.Name}
	parentID := file.ParentID
	for parentID != "" {
		parent, ok := SessionFileTreeFind(files, parentID)
		if !ok {
			break
		}
		parts = append([]string{parent.Name}, parts...)
		parentID = parent.ParentID
	}
	return strings.Join(parts, "/")
}

func SessionFileTreeFind(files []state.FileNode, id string) (state.FileNode, bool) {
	for _, file := range files {
		if file.ID == id {
			return file, true
		}
	}
	return state.FileNode{}, false
}

func SessionFileTreeIcon(name string) string {
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

func SessionFileTreeIconColorClass(name string) string {
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

func SessionFileTreeNodeKind(kind state.FileKind) FileTreeNodeKind {
	if kind == state.FileKindDirectory {
		return FileTreeNodeDirectory
	}
	return FileTreeNodeFile
}

func SessionFileTreeGitStatus(status state.FileGitStatus) FileTreeGitStatus {
	switch status {
	case state.FileGitStatusModified:
		return FileTreeGitStatusModified
	case state.FileGitStatusAdded:
		return FileTreeGitStatusAdded
	case state.FileGitStatusDeleted:
		return FileTreeGitStatusDeleted
	case state.FileGitStatusRenamed:
		return FileTreeGitStatusRenamed
	case state.FileGitStatusUntracked:
		return FileTreeGitStatusUntracked
	case state.FileGitStatusIgnored:
		return FileTreeGitStatusIgnored
	default:
		return FileTreeGitStatusClean
	}
}

func SessionFileCommand(commandURL string) FileTreeCommand {
	return FileTreeCommand{
		Method: "POST",
		URL:    commandURL,
	}
}

func SessionsSidebarCommand(commandURL string) string {
	return "@discobotCommand(" + strconv.Quote(commandURL) + ", {method: \"POST\"})"
}

func SessionsSidebarSessionToggleCommand(id string) string {
	return "@post('/ui/commands/sessions/" + url.PathEscape(id) + "/toggle-expanded')"
}

func SessionsSidebarSessionSelectCommand(id string) string {
	return "@post('/ui/commands/sessions/" + url.PathEscape(id) + "/select')"
}

func SessionsSidebarHideCommand() string {
	return SessionsSidebarCommand("/ui/commands/sidebar/hide")
}

func SessionsSidebarSessionViewModeCommand(id string, mode state.SessionViewMode) string {
	return SessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/view-mode/" + url.PathEscape(string(mode)))
}

func SessionsSidebarSessionDetailSectionShowCommand(id string, section state.SessionDetailSection) string {
	return SessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/detail-sections/" + url.PathEscape(string(section)) + "/show")
}

func SessionsSidebarSessionDetailSectionHideCommand(id string, section state.SessionDetailSection) string {
	return SessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/detail-sections/" + url.PathEscape(string(section)) + "/hide")
}

func SessionsSidebarSessionDiffSummaryCommand(id string) string {
	return SessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/diff-summary")
}

func SessionsSidebarSideChatSelectCommand(sessionID string, threadID string) string {
	return SessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(sessionID) + "/side-chats/" + url.PathEscape(threadID) + "/select")
}

func SessionsSidebarFileSelectCommand(id string) string {
	return SessionsSidebarCommand("/ui/commands/files/" + url.PathEscape(id) + "/select")
}

func SessionsSidebarFileSearchToggleCommand() string {
	return SessionsSidebarCommand("/ui/commands/files/search/toggle")
}

func SessionsSidebarFileApprovalToggleCommand(id string) string {
	return SessionsSidebarCommand("/ui/commands/files/" + url.PathEscape(id) + "/approval/toggle")
}

func SessionsSidebarCommitCommand(id string) string {
	return SessionsSidebarCommand("/ui/commands/sessions/" + url.PathEscape(id) + "/commit")
}

func SessionsSidebarExpanderLabel(session state.Session, expanded bool) string {
	if expanded {
		return "Collapse " + session.Title
	}
	return "Expand " + session.Title
}

func SessionsSidebarViewButtonClass(viewMode state.SessionViewMode, buttonMode state.SessionViewMode) string {
	class := "sessions-sidebar--view-button"
	if viewMode == buttonMode {
		class += " sessions-sidebar--view-button--active"
	}
	return class
}

func SessionsSidebarIconButtonClass(active bool) string {
	class := "sessions-sidebar--icon-button"
	if active {
		class += " sessions-sidebar--icon-button--active"
	}
	return class
}

func SessionsSidebarDetailSectionClass(visible bool) string {
	class := "sessions-sidebar--detail-section"
	if visible {
		class += " sessions-sidebar--detail-section--visible"
	}
	return class
}

func SessionsSidebarDetailLauncherClass(active bool) string {
	class := "sessions-sidebar--detail-launcher"
	if active {
		class += " sessions-sidebar--detail-launcher--active"
	}
	return class
}

func SessionsSidebarDetailLauncherLabel(label string, active bool) string {
	if active {
		return label + " panel is open"
	}
	return "Open " + label + " panel"
}

func SessionsSidebarWorkspaceSummary(fileTree FileTreeData, diffFiles []SessionDiffFile, viewMode state.SessionViewMode) string {
	if viewMode == state.SessionViewModeDiff {
		return strconv.Itoa(len(diffFiles)) + " changes"
	}
	if fileTree.TotalNodeCount == 1 {
		return "1 file"
	}
	return strconv.Itoa(fileTree.TotalNodeCount) + " files"
}

func SessionHookSectionSummary(hooks []state.SessionHook) string {
	if len(hooks) == 0 {
		return "0 hooks"
	}
	summary := SessionHookSummary(hooks)
	return strconv.Itoa(summary.Passed) + "/" + strconv.Itoa(summary.Total) + " " + strings.ToLower(summary.Label)
}

func SessionSideChatSectionSummary(sideChats []SessionSideChatItem) string {
	if len(sideChats) == 1 {
		return "1 chat"
	}
	return strconv.Itoa(len(sideChats)) + " chats"
}

func SessionDiffFileStatusClass(status state.FileGitStatus) string {
	if status == "" {
		return ""
	}
	return "sessions-sidebar--diff-file-status--" + string(status)
}

func SessionDiffFileStatusText(status state.FileGitStatus) string {
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

func SessionDiffFileStatusTitle(status state.FileGitStatus) string {
	if status == "" || status == state.FileGitStatusClean {
		return ""
	}
	return "Git status: " + string(status)
}

func SessionDiffFileApprovalClass(approved bool) string {
	class := "sessions-sidebar--approval-toggle"
	if approved {
		class += " sessions-sidebar--approval-toggle--approved"
	}
	return class
}

func SessionDiffFileApprovalLabel(approved bool) string {
	if approved {
		return "Approved"
	}
	return "Not approved"
}

func SessionApprovalSummaryFor(diffFiles []SessionDiffFile) SessionApprovalSummary {
	summary := SessionApprovalSummary{
		Total: len(diffFiles),
	}
	for _, file := range diffFiles {
		if file.Approved {
			summary.Approved++
		}
	}
	return summary
}

func SessionApprovalStatusLabel(summary SessionApprovalSummary) string {
	if summary.Total == 0 {
		return "No changed files"
	}
	if summary.Approved == summary.Total {
		return "All files approved"
	}
	return strconv.Itoa(summary.Approved) + "/" + strconv.Itoa(summary.Total) + " approved"
}

func SessionApprovalStatusClass(summary SessionApprovalSummary) string {
	class := "sessions-sidebar--approval-status"
	if summary.Total > 0 && summary.Approved == summary.Total {
		class += " sessions-sidebar--approval-status--complete"
	}
	return class
}

func SessionApprovalComplete(summary SessionApprovalSummary) bool {
	return summary.Total > 0 && summary.Approved == summary.Total
}

func SessionHookSummary(hooks []state.SessionHook) SessionHooksSummary {
	summary := SessionHooksSummary{
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

func SessionHookStatusClass(status state.HookRunStatus) string {
	if status == "" {
		status = state.HookRunStatusPending
	}
	return "sessions-sidebar--hook-dot--" + string(status)
}

func SessionHookStatusLabel(status state.HookRunStatus) string {
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

func SessionSideChatClass(selected bool, unread bool) string {
	class := "sessions-sidebar--side-chat"
	if selected {
		class += " sessions-sidebar--side-chat--selected"
	}
	if unread {
		class += " sessions-sidebar--side-chat--unread"
	}
	return class
}

func SessionSideChatCountLabel(count int) string {
	if count == 1 {
		return "1 msg"
	}
	return strconv.Itoa(count) + " msgs"
}

func SessionsSidebarFilterMenu(view state.View, trigger templ.Component) MenuData {
	return MenuData{
		ID:            "sessions-filter-menu",
		Label:         "Open session filters",
		Trigger:       trigger,
		AnchorToClick: true,
		Items: []MenuItem{
			{
				Label: "Filter",
				Children: []MenuItem{
					SessionMenuCheckItem(view, "Copilot CLI", "copilot-cli", false),
					SessionMenuCheckItem(view, "Cloud", "cloud", false),
					SessionMenuCheckItem(view, "Claude", "claude", false),
					SessionMenuCheckItem(view, "Completed", "completed", true),
					SessionMenuCheckItem(view, "In Progress", "in-progress", false),
					SessionMenuCheckItem(view, "Input Needed", "input-needed", false),
					SessionMenuCheckItem(view, "Failed", "failed", false),
					SessionMenuCheckItem(view, "Done", "done", true),
					SessionMenuCheckItem(view, "Read", "read", false),
					{Label: "Reset", SeparatorBefore: true},
				},
			},
			SessionMenuCheckItem(view, "Sort by Created", "sort-created", true),
			SessionMenuCheckItem(view, "Sort by Updated", "sort-updated", false),
			SessionMenuCheckItem(view, "Group by Workspace", "group-workspace", true),
			SessionMenuCheckItem(view, "Group by Time", "group-time", false),
			SessionMenuCheckItem(view, "Show Recent Sessions", "show-recent-sessions", true),
			SessionMenuCheckItem(view, "Show All Sessions", "show-all-sessions", false),
			{Label: "Collapse All Groups", SeparatorBefore: true},
		},
	}
}

func SessionMenuCheckItem(view state.View, label string, key string, separatorBefore bool) MenuItem {
	sessionState := SessionPanelState(view)
	checkState := MenuCheckUnchecked
	if sessionState.SessionMenuChecks[key] {
		checkState = MenuCheckChecked
	}

	return MenuItem{
		Label:           label,
		CheckState:      checkState,
		SeparatorBefore: separatorBefore,
		Command: MenuCommand{
			Method: "POST",
			URL:    "/ui/commands/sessions/menu-checks/" + key + "/toggle",
		},
	}
}
