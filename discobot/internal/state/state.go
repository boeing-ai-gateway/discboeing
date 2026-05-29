// Package state owns Discobot's server-side application and view state.
package state

// Shell is the render model for the full-page shell.
type Shell struct {
	Data Data
	View View
}

// View is server-owned UI view state.
type View struct {
	PanelLayout PanelLayout
	Settings    SettingsDialogState
}

// SettingsDialogState owns server-rendered settings dialog state.
type SettingsDialogState struct {
	Open                 bool
	SupportInfoOpen      bool
	ClearCacheDialogOpen bool
	CacheCleared         bool
	Tab                  SettingsTab
}

// SettingsTab describes the active settings section.
type SettingsTab string

const (
	// SettingsTabAppearance shows mode and palette preferences.
	SettingsTabAppearance SettingsTab = "appearance"
	// SettingsTabChat shows conversation defaults.
	SettingsTabChat SettingsTab = "chat"
	// SettingsTabProviders shows sandbox provider settings.
	SettingsTabProviders SettingsTab = "providers"
	// SettingsTabUpdate shows update and cache tools.
	SettingsTabUpdate SettingsTab = "update"
	// SettingsTabCredentials shows API credential settings.
	SettingsTabCredentials SettingsTab = "credentials"
)

// NormalizeSettingsTab returns a supported settings tab.
func NormalizeSettingsTab(tab SettingsTab) SettingsTab {
	switch tab {
	case SettingsTabAppearance, SettingsTabChat, SettingsTabProviders, SettingsTabUpdate, SettingsTabCredentials:
		return tab
	default:
		return SettingsTabAppearance
	}
}

// PanelLayout is the server-owned workspace panel layout.
type PanelLayout struct {
	Panels map[string]Panel
}

// Panel describes server-owned panel sizing bounds and current size.
type Panel struct {
	ID          string
	Visible     bool
	Maximizable bool
	Maximized   bool
	GridColumn  string
	GridRow     string
	Width       int
	Height      int
	MinWidth    int
	MaxWidth    int
	MinHeight   int
	MaxHeight   int
	Session     *SessionPanelState
	Editor      *EditorPanelState
	Composer    *ComposerPanelState
	Terminal    *TerminalPanelState
}

// SessionPanelState owns state for a session navigation/file-tree panel.
type SessionPanelState struct {
	SelectedSessionID            string
	SelectedFileID               string
	SelectedSideChatID           string
	ExpandedSessionIDs           map[string]bool
	VisibleSessionDetailSections map[string]bool
	ExpandedFileIDs              map[string]bool
	SessionViewModes             map[string]SessionViewMode
	SessionMenuChecks            map[string]bool
	FileTreeSearch               string
	FileTreeSearchVisible        bool
}

// SessionViewMode describes the expanded session item content.
type SessionViewMode string

const (
	// SessionViewModeFiles shows the session file tree.
	SessionViewModeFiles SessionViewMode = "files"
	// SessionViewModeDiff shows a flat changed-file list.
	SessionViewModeDiff SessionViewMode = "diff"
)

// NormalizeSessionViewMode returns a supported session view mode.
func NormalizeSessionViewMode(mode SessionViewMode) SessionViewMode {
	if mode == SessionViewModeDiff {
		return SessionViewModeDiff
	}
	return SessionViewModeFiles
}

// SessionDetailSection describes a visible panel in an expanded session row.
type SessionDetailSection string

const (
	// SessionDetailSectionWorkspace shows the session file/diff workspace.
	SessionDetailSectionWorkspace SessionDetailSection = "workspace"
	// SessionDetailSectionSideChats shows compact side-chat thread rows.
	SessionDetailSectionSideChats SessionDetailSection = "side-chats"
	// SessionDetailSectionHooks shows hook run state.
	SessionDetailSectionHooks SessionDetailSection = "hooks"
	// SessionDetailSectionReview shows diff review and approval controls.
	SessionDetailSectionReview SessionDetailSection = "review"
)

// IsSessionDetailSection reports whether section is supported.
func IsSessionDetailSection(section SessionDetailSection) bool {
	switch section {
	case SessionDetailSectionWorkspace, SessionDetailSectionSideChats, SessionDetailSectionHooks, SessionDetailSectionReview:
		return true
	default:
		return false
	}
}

// SessionDetailSectionKey scopes a section visibility state to a session.
func SessionDetailSectionKey(sessionID string, section SessionDetailSection) string {
	return sessionID + ":" + string(section)
}

// EditorPanelState owns state for an editor panel instance.
type EditorPanelState struct {
	OpenFileIDs          []string
	ActiveFileID         string
	DiffSummarySessionID string
	ServiceLogID         string
}

// ComposerPanelState owns state for a composer panel instance.
type ComposerPanelState struct {
	PromptHeight int
}

// TerminalPanelState owns state for a terminal panel instance.
type TerminalPanelState struct{}

// WorkspacePanelIDs returns the mutually visible workspace panels.
func WorkspacePanelIDs() []string {
	return []string{"editor", "composer", "terminal"}
}

// IsWorkspacePanel reports whether a panel participates in the main workspace.
func IsWorkspacePanel(panelID string) bool {
	for _, id := range WorkspacePanelIDs() {
		if id == panelID {
			return true
		}
	}
	return false
}

// PanelVisible reports a panel's effective visibility, including defaults.
func PanelVisible(view View, panelID string) bool {
	panel, ok := view.PanelLayout.Panels[panelID]
	if ok && panel.ID != "" {
		return panel.Visible
	}
	defaultPanel, ok := DefaultPanelLayout().Panels[panelID]
	return ok && defaultPanel.Visible
}

// VisibleWorkspacePanelCount returns the number of visible workspace panels.
func VisibleWorkspacePanelCount(view View) int {
	count := 0
	for _, panelID := range WorkspacePanelIDs() {
		if PanelVisible(view, panelID) {
			count++
		}
	}
	return count
}

// CanHideWorkspacePanel prevents the workspace from having no visible panels.
func CanHideWorkspacePanel(view View, panelID string) bool {
	if !IsWorkspacePanel(panelID) || !PanelVisible(view, panelID) {
		return true
	}
	return VisibleWorkspacePanelCount(view) > 1
}

// Data is server-owned application/domain state.
type Data struct {
	Title      string
	App        App
	Workspaces []Workspace
	Sessions   []Session
	Services   []Service
}

// App describes the running Discobot application.
type App struct {
	Name        string
	Description string
}

// Service is a workspace background process shown in the service menu.
type Service struct {
	ID          string
	Name        string
	Description string
	Status      ServiceStatus
	URL         string
	Logs        []string
}

// ServiceStatus describes whether a service is running.
type ServiceStatus string

const (
	// ServiceStatusStopped means the service is not running.
	ServiceStatusStopped ServiceStatus = "stopped"
	// ServiceStatusRunning means the service is running.
	ServiceStatusRunning ServiceStatus = "running"
)

// Workspace is a local directory or Git remote that owns sessions.
type Workspace struct {
	ID          string
	Name        string
	SourceType  WorkspaceSourceType
	Path        string
	RemoteURL   string
	Description string
}

// WorkspaceSourceType describes how a workspace is sourced.
type WorkspaceSourceType string

const (
	// WorkspaceSourceLocal means the workspace points at a local directory.
	WorkspaceSourceLocal WorkspaceSourceType = "local"
	// WorkspaceSourceGit means the workspace points at a Git remote.
	WorkspaceSourceGit WorkspaceSourceType = "git"
)

// Session is a conversation session scoped to a workspace.
type Session struct {
	ID           string
	WorkspaceID  string
	Title        string
	RelativeTime string
	Status       SessionStatus
	Additions    int
	Deletions    int
	MainThread   Thread
	Files        []FileNode
	Hooks        []SessionHook
	SideChats    []Thread
}

// Thread is a conversation thread attached to a session.
type Thread struct {
	ID           string
	Title        string
	RelativeTime string
	Preview      string
	Unread       bool
	Messages     []ThreadMessage
}

// ThreadMessage is one visible message in a thread.
type ThreadMessage struct {
	ID   string
	Role ThreadMessageRole
	Text string
}

// ThreadMessageRole describes who authored a thread message.
type ThreadMessageRole string

const (
	// ThreadMessageRoleUser marks a user-authored message.
	ThreadMessageRoleUser ThreadMessageRole = "user"
	// ThreadMessageRoleAssistant marks an assistant-authored message.
	ThreadMessageRoleAssistant ThreadMessageRole = "assistant"
)

// SessionHook is a hook configured for a session.
type SessionHook struct {
	ID       string
	Name     string
	Type     string
	Status   HookRunStatus
	RunCount int
}

// HookRunStatus describes the display state for a session hook.
type HookRunStatus string

const (
	// HookRunStatusPending means the hook has not run yet.
	HookRunStatusPending HookRunStatus = "pending"
	// HookRunStatusRunning means the hook is currently running.
	HookRunStatusRunning HookRunStatus = "running"
	// HookRunStatusSuccess means the hook last completed successfully.
	HookRunStatusSuccess HookRunStatus = "success"
	// HookRunStatusFailure means the hook last failed.
	HookRunStatusFailure HookRunStatus = "failure"
)

// FileNode is a server-owned file tree node scoped to a session.
type FileNode struct {
	ID                    string
	SessionID             string
	ParentID              string
	Name                  string
	Kind                  FileKind
	GitStatus             FileGitStatus
	Approved              bool
	HasChangedDescendants bool
}

// FileKind describes whether a file tree node is a file or directory.
type FileKind string

const (
	// FileKindDirectory is an expandable directory node.
	FileKindDirectory FileKind = "directory"
	// FileKindFile is a leaf file node.
	FileKindFile FileKind = "file"
)

// FileGitStatus describes a file's workspace diff state.
type FileGitStatus string

const (
	// FileGitStatusClean has no visible diff status.
	FileGitStatusClean FileGitStatus = "clean"
	// FileGitStatusModified marks a changed tracked file.
	FileGitStatusModified FileGitStatus = "modified"
	// FileGitStatusAdded marks a newly added tracked file.
	FileGitStatusAdded FileGitStatus = "added"
	// FileGitStatusDeleted marks a deleted tracked file.
	FileGitStatusDeleted FileGitStatus = "deleted"
	// FileGitStatusRenamed marks a renamed tracked file.
	FileGitStatusRenamed FileGitStatus = "renamed"
	// FileGitStatusUntracked marks an untracked file.
	FileGitStatusUntracked FileGitStatus = "untracked"
	// FileGitStatusIgnored marks an ignored file.
	FileGitStatusIgnored FileGitStatus = "ignored"
)

// DeriveFileGitStatusFromPath is the prototype hook for mapping backend diff
// entries to file tree status. Real workspace integration can replace the input
// source while keeping this server-side derivation boundary.
func DeriveFileGitStatusFromPath(path string, explicit map[string]FileGitStatus) FileGitStatus {
	if status, ok := explicit[path]; ok {
		return status
	}
	return FileGitStatusClean
}

// SessionStatus describes the current session state.
type SessionStatus string

const (
	// SessionStatusIdle means no command is currently running.
	SessionStatusIdle SessionStatus = "idle"
	// SessionStatusRunning means the session is actively working.
	SessionStatusRunning SessionStatus = "running"
)

// DefaultData returns the initial server-owned app state.
func DefaultData() Data {
	return Data{
		Title: "Discobot",
		App: App{
			Name:        "discobot",
			Description: "A small Datastar + templ scaffold for the Discobot UI.",
		},
		Workspaces: []Workspace{
			{
				ID:          "workspace-disco",
				Name:        "DISCO",
				SourceType:  WorkspaceSourceLocal,
				Path:        "/Users/you/src/disco",
				Description: "Local Discobot workspace",
			},
			{
				ID:          "workspace-discobot",
				Name:        "discobot",
				SourceType:  WorkspaceSourceGit,
				RemoteURL:   "git@github.com:obot-platform/discobot.git",
				Description: "Discobot Git repository",
			},
			{
				ID:          "workspace-agent",
				Name:        "agent-go",
				SourceType:  WorkspaceSourceLocal,
				Path:        "/Users/you/src/agent-go",
				Description: "Local agent provider work",
			},
		},
		Services: []Service{
			{
				ID:          "dev-server",
				Name:        "Dev Server",
				Description: "Vite development server",
				Status:      ServiceStatusRunning,
				URL:         "http://localhost:3100",
				Logs: []string{
					"21:40:04 [dev-server] starting pnpm dev",
					"21:40:05 [dev-server] local: http://localhost:3100",
					"21:40:06 [dev-server] ready in 812ms",
				},
			},
			{
				ID:          "api-server",
				Name:        "API Server",
				Description: "Go backend API",
				Status:      ServiceStatusStopped,
				Logs: []string{
					"21:35:11 [api-server] stopped",
					"21:35:11 [api-server] exit status 0",
				},
			},
		},
		Sessions: []Session{
			{
				ID:           "session-cobra",
				WorkspaceID:  "workspace-disco",
				Title:        "Create Cobra app skeleton",
				RelativeTime: "3 days ago",
				Status:       SessionStatusIdle,
				Additions:    248,
				Deletions:    37,
				MainThread: Thread{
					ID: "thread-cobra-main",
					Messages: []ThreadMessage{
						{
							ID:   "message-cobra-main-1",
							Role: ThreadMessageRoleUser,
							Text: "Add a session-header close button that hides this panel without resetting what I expanded.",
						},
						{
							ID:   "message-cobra-main-2",
							Role: ThreadMessageRoleAssistant,
							Text: "I’ll keep the session detail state intact and only change the panel visibility.",
						},
						{
							ID:   "message-cobra-main-3",
							Role: ThreadMessageRoleUser,
							Text: "While you are in there, make the center workspace behave like a real conversation view. I need enough fixture content to test the way it grows upward from the composer when the thread is no longer empty.",
						},
						{
							ID:   "message-cobra-main-4",
							Role: ThreadMessageRoleAssistant,
							Text: "I’ll expand the sample main thread so it produces a tall scrollable conversation and keeps the composer anchored near the bottom of the workspace.",
						},
						{
							ID:   "message-cobra-main-5",
							Role: ThreadMessageRoleUser,
							Text: "The important part is that I can see what happens when content wants to use the whole screen. A short two-message sample does not show the overflow behavior at all.",
						},
						{
							ID:   "message-cobra-main-6",
							Role: ThreadMessageRoleAssistant,
							Text: "That makes sense. I’ll add enough messages with realistic wrapping so the center area has to allocate space between the conversation, the session panel, and the composer.",
						},
						{
							ID:   "message-cobra-main-7",
							Role: ThreadMessageRoleUser,
							Text: "Please keep the messages varied. Some should be short status pings, and others should wrap over multiple lines so that I can judge bubble spacing and vertical rhythm.",
						},
						{
							ID:   "message-cobra-main-8",
							Role: ThreadMessageRoleAssistant,
							Text: "I’ll include a mix of compact updates and longer assistant responses. The fixture should be obviously fake but useful for tuning scroll regions and panel boundaries.",
						},
						{
							ID:   "message-cobra-main-9",
							Role: ThreadMessageRoleUser,
							Text: "Also make sure the panel buttons remain visible near the composer. I want the panel to grow upward, not push the controls below the viewport.",
						},
						{
							ID:   "message-cobra-main-10",
							Role: ThreadMessageRoleAssistant,
							Text: "The bottom-aligned center stack will keep its bottom offset, cap itself against the top buffer, and let the scrollable children absorb overflow instead of moving the composer off screen.",
						},
						{
							ID:   "message-cobra-main-11",
							Role: ThreadMessageRoleUser,
							Text: "Good. The side session panel should get as much room as possible too. It currently feels cramped when workspace, hooks, and review sections are open.",
						},
						{
							ID:   "message-cobra-main-12",
							Role: ThreadMessageRoleAssistant,
							Text: "I’ll remove the small fixed cap from the session panel and let it flex into the available vertical space. Its details area will keep scrolling internally when the panel content is taller than the allocated area.",
						},
						{
							ID:   "message-cobra-main-13",
							Role: ThreadMessageRoleUser,
							Text: "Can you use about a fifty pixel top buffer? I want it to go almost to the top but not collide with the chrome or feel glued to the edge.",
						},
						{
							ID:   "message-cobra-main-14",
							Role: ThreadMessageRoleAssistant,
							Text: "Yes. With the bottom offset still at thirty-four pixels, the bottom-aligned stack can use the viewport height minus eighty-four pixels, leaving roughly fifty pixels at the top.",
						},
						{
							ID:   "message-cobra-main-15",
							Role: ThreadMessageRoleUser,
							Text: "This should make it obvious whether the session panel can stretch naturally while the conversation is long.",
						},
						{
							ID:   "message-cobra-main-16",
							Role: ThreadMessageRoleAssistant,
							Text: "Exactly. The long fixture content will force the layout to exercise the same constraints a real working session would hit after several back-and-forth turns.",
						},
						{
							ID:   "message-cobra-main-17",
							Role: ThreadMessageRoleUser,
							Text: "Short ping.",
						},
						{
							ID:   "message-cobra-main-18",
							Role: ThreadMessageRoleAssistant,
							Text: "Acknowledged. The short rows help verify that compact messages do not create awkward gaps between longer wrapped bubbles.",
						},
						{
							ID:   "message-cobra-main-19",
							Role: ThreadMessageRoleUser,
							Text: "Add a couple more so the scrollbar is definitely present on a laptop-height viewport.",
						},
						{
							ID:   "message-cobra-main-20",
							Role: ThreadMessageRoleAssistant,
							Text: "I’ll add enough fixture turns to exceed the available center height on common screens. That should make the scroll behavior and the top buffer easy to inspect without needing live agent output.",
						},
						{
							ID:   "message-cobra-main-21",
							Role: ThreadMessageRoleUser,
							Text: "The goal is not perfect copy. The goal is visual pressure on the layout.",
						},
						{
							ID:   "message-cobra-main-22",
							Role: ThreadMessageRoleAssistant,
							Text: "Understood. I’ll optimize the sample data for visual density and layout coverage, not for production-quality conversation content.",
						},
						{
							ID:   "message-cobra-main-23",
							Role: ThreadMessageRoleUser,
							Text: "Make sure checks still pass after templ generation.",
						},
						{
							ID:   "message-cobra-main-24",
							Role: ThreadMessageRoleAssistant,
							Text: "I’ll regenerate templ output after the layout changes and run the Discobot check command so the generated Go and Tailwind output stay in sync.",
						},
					},
				},
				Files: []FileNode{
					{ID: "file-cobra-root", SessionID: "session-cobra", Name: "discobot", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "agent-go", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent-provider", SessionID: "session-cobra", ParentID: "file-cobra-agent", Name: "provider", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent-provider-openai", SessionID: "session-cobra", ParentID: "file-cobra-agent-provider", Name: "openai", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent-provider-openai-client", SessionID: "session-cobra", ParentID: "file-cobra-agent-provider-openai", Name: "client.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-server", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "server", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-ui", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "ui", Kind: FileKindDirectory},
					{ID: "file-cobra-vm", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "vm-assets", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-vm-network", SessionID: "session-cobra", ParentID: "file-cobra-vm", Name: "network", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-vm-network-bridge", SessionID: "session-cobra", ParentID: "file-cobra-vm-network", Name: "bridge.sh", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-vm-scripts", SessionID: "session-cobra", ParentID: "file-cobra-vm", Name: "scripts", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-vm-scripts-start", SessionID: "session-cobra", ParentID: "file-cobra-vm-scripts", Name: "start.sh", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-vm-fstab", SessionID: "session-cobra", ParentID: "file-cobra-vm", Name: "fstab", Kind: FileKindFile, GitStatus: FileGitStatusDeleted},
					{ID: "file-cobra-readme", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "README.md", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-package", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "package.json", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-go-mod", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "go.mod", Kind: FileKindFile, GitStatus: FileGitStatusRenamed, Approved: true},
					{ID: "file-cobra-cmd", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "cmd", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-cmd-discobot", SessionID: "session-cobra", ParentID: "file-cobra-cmd", Name: "discobot", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-internal", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "internal", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-internal-command", SessionID: "session-cobra", ParentID: "file-cobra-internal", Name: "command", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-internal-state", SessionID: "session-cobra", ParentID: "file-cobra-internal", Name: "state", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-internal-server", SessionID: "session-cobra", ParentID: "file-cobra-internal", Name: "server", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-content", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "content", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-content-components", SessionID: "session-cobra", ParentID: "file-cobra-content", Name: "components", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-content-app", SessionID: "session-cobra", ParentID: "file-cobra-content-components", Name: "app", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-content-ui", SessionID: "session-cobra", ParentID: "file-cobra-content-components", Name: "ui", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-static", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "static", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-static-lib", SessionID: "session-cobra", ParentID: "file-cobra-static", Name: "lib", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-styles", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "styles", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-docs", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "docs", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-docs-design", SessionID: "session-cobra", ParentID: "file-cobra-docs", Name: "design", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-tests", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "tests", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-tests-fixtures", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixtures", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-scripts", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "scripts", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-cmd-main", SessionID: "session-cobra", ParentID: "file-cobra-cmd-discobot", Name: "main.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-command-handler", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "handler.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-command-layout", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "layout_resize.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-command-panel", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "panel.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-command-response", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "response.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-state-model", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "state.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-state-test", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "state_test.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-server-router", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "server.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-app-shell", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "app_shell.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-session-workspace", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "session_workspace.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-conversation", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "conversation.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-sidebar-details", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "sessions_sidebar_session_details.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-panel-templ", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "panel.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-resize-templ", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "resize_handle.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-static-resize", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "resize.js", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-styles-app", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "app.css", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-docs-guidelines", SessionID: "session-cobra", ParentID: "file-cobra-docs", Name: "GUIDELINES.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-docs-file-tree", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "file-tree-feature-gap.md", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-e2e-layout", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "panel-layout.spec.ts", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-log", SessionID: "session-cobra", ParentID: "file-cobra-tests-fixtures", Name: "long-session.json", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-script-check", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "check-layout.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-001", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_001.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-002", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_002.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-003", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_003.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-004", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_004.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-005", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_005.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-006", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_006.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-007", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_007.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked, Approved: true},
					{ID: "file-cobra-fixture-008", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_008.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-009", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_009.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-010", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_010.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-011", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_011.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-012", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_012.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-013", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_013.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-014", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_014.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-015", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_015.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-016", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_016.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-017", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_017.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-018", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_018.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-019", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_019.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-020", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_020.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-021", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_021.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-022", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_022.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-023", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_023.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-024", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_024.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-025", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_025.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-026", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_026.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-027", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_027.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-028", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_028.js", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-fixture-029", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_029.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-030", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_030.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-031", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_031.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-032", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_032.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-033", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_033.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-034", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_034.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-035", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_035.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-fixture-036", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_036.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-037", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_037.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-038", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_038.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-039", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_039.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-040", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_040.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-041", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_041.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-042", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_042.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked, Approved: true},
					{ID: "file-cobra-fixture-043", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_043.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-044", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_044.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-045", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_045.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-046", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_046.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-047", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_047.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-048", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_048.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-049", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_049.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-050", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_050.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-051", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_051.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-052", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_052.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-053", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_053.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-054", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_054.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-055", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_055.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-056", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_056.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-057", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_057.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-058", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_058.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-059", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_059.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-060", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_060.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-061", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_061.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-062", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_062.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-063", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_063.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-064", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_064.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-065", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_065.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-066", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_066.js", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-fixture-067", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_067.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-068", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_068.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-069", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_069.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-070", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_070.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-071", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_071.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-072", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_072.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-073", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_073.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-074", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_074.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-075", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_075.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-076", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_076.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-077", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_077.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-078", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_078.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-079", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_079.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-080", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_080.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-081", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_081.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-082", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_082.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-083", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_083.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-084", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_084.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-085", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_085.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-086", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_086.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-087", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_087.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-088", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_088.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked, Approved: true},
					{ID: "file-cobra-fixture-089", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_089.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-090", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_090.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-091", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_091.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-092", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_092.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-093", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_093.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-094", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_094.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-095", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_095.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-096", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_096.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-097", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_097.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-098", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_098.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-099", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_099.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-100", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_100.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-101", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_101.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-102", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_102.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-103", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_103.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-104", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_104.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-105", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_105.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-106", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_106.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-107", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_107.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-108", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_108.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-109", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_109.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-110", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_110.css", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-fixture-111", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_111.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-112", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_112.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-113", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_113.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-114", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_114.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-115", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_115.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-116", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_116.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-117", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_117.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-118", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_118.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-119", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_119.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-120", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_120.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-121", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_121.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-fixture-122", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_122.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-123", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_123.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-124", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_124.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-125", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_125.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-126", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_126.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-127", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_127.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-128", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_128.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-129", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_129.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-130", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_130.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-131", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_131.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-132", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_132.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-133", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_133.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-134", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_134.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-135", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_135.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-136", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_136.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-137", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_137.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-138", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_138.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-139", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_139.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-140", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_140.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-141", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_141.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-142", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_142.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-143", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_143.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-cobra-fixture-144", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_144.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-145", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_145.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-146", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_146.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-147", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_147.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-148", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_148.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-149", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_149.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-150", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_150.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-151", SessionID: "session-cobra", ParentID: "file-cobra-content-app", Name: "fixture_151.templ", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-152", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", Name: "fixture_152.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-153", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", Name: "fixture_153.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-154", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", Name: "fixture_154.go", Kind: FileKindFile, GitStatus: FileGitStatusAdded, Approved: true},
					{ID: "file-cobra-fixture-155", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", Name: "fixture_155.go", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-156", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", Name: "fixture_156.js", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-fixture-157", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", Name: "fixture_157.md", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-158", SessionID: "session-cobra", ParentID: "file-cobra-tests", Name: "fixture_158.ts", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-fixture-159", SessionID: "session-cobra", ParentID: "file-cobra-scripts", Name: "fixture_159.mjs", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-fixture-160", SessionID: "session-cobra", ParentID: "file-cobra-styles", Name: "fixture_160.css", Kind: FileKindFile, GitStatus: FileGitStatusModified},
				},
				Hooks: []SessionHook{
					{ID: "hook-cobra-tests", Name: "Go tests", Type: "test", Status: HookRunStatusSuccess, RunCount: 2},
					{ID: "hook-cobra-lint", Name: "Backend lint", Type: "lint", Status: HookRunStatusFailure, RunCount: 1},
					{ID: "hook-cobra-security", Name: "Security reviewer", Type: "review", Status: HookRunStatusPending},
				},
				SideChats: []Thread{
					{
						ID:           "thread-cobra-review",
						Title:        "Review follow-up",
						RelativeTime: "12m",
						Preview:      "Need a quick pass on command wiring before commit.",
						Unread:       true,
						Messages: []ThreadMessage{
							{ID: "message-cobra-review-1", Role: ThreadMessageRoleUser, Text: "Can you review the command wiring?"},
							{ID: "message-cobra-review-2", Role: ThreadMessageRoleAssistant, Text: "The command wiring looks ready after the route cleanup."},
							{ID: "message-cobra-review-3", Role: ThreadMessageRoleUser, Text: "Please double-check the commit path."},
							{ID: "message-cobra-review-4", Role: ThreadMessageRoleAssistant, Text: "The commit path preserves the expanded session state."},
						},
					},
					{
						ID:           "thread-cobra-terminal",
						Title:        "Terminal output",
						RelativeTime: "1h",
						Preview:      "go test ./... is clean after regenerating templ output.",
						Messages: []ThreadMessage{
							{ID: "message-cobra-terminal-1", Role: ThreadMessageRoleUser, Text: "Run the checks after regenerating templ output."},
							{ID: "message-cobra-terminal-2", Role: ThreadMessageRoleAssistant, Text: "go test ./... is clean."},
							{ID: "message-cobra-terminal-3", Role: ThreadMessageRoleUser, Text: "Any generated files changed?"},
							{ID: "message-cobra-terminal-4", Role: ThreadMessageRoleAssistant, Text: "Only the expected templ outputs changed."},
							{ID: "message-cobra-terminal-5", Role: ThreadMessageRoleUser, Text: "Keep the terminal summary short."},
							{ID: "message-cobra-terminal-6", Role: ThreadMessageRoleAssistant, Text: "Checks passed."},
							{ID: "message-cobra-terminal-7", Role: ThreadMessageRoleAssistant, Text: "No follow-up failures."},
						},
					},
				},
			},
			{
				ID:           "session-sidebar",
				WorkspaceID:  "workspace-disco",
				Title:        "Build sessions sidebar",
				RelativeTime: "Today",
				Status:       SessionStatusRunning,
				Additions:    164,
				Deletions:    28,
				MainThread: Thread{
					ID: "thread-sidebar-main",
					Messages: []ThreadMessage{
						{
							ID:   "message-sidebar-main-1",
							Role: ThreadMessageRoleUser,
							Text: "Keep the sidebar compact while preserving the selected session state.",
						},
					},
				},
				Files: []FileNode{
					{ID: "file-sidebar-root", SessionID: "session-sidebar", Name: "discobot", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-content", SessionID: "session-sidebar", ParentID: "file-sidebar-root", Name: "content", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-components", SessionID: "session-sidebar", ParentID: "file-sidebar-content", Name: "components", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-app", SessionID: "session-sidebar", ParentID: "file-sidebar-components", Name: "app", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-session", SessionID: "session-sidebar", ParentID: "file-sidebar-app", Name: "sessions_sidebar.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
					{ID: "file-sidebar-ui", SessionID: "session-sidebar", ParentID: "file-sidebar-components", Name: "ui", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-tree", SessionID: "session-sidebar", ParentID: "file-sidebar-ui", Name: "file_tree.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-sidebar-state", SessionID: "session-sidebar", ParentID: "file-sidebar-root", Name: "state.go", Kind: FileKindFile, GitStatus: FileGitStatusModified, Approved: true},
				},
				Hooks: []SessionHook{
					{ID: "hook-sidebar-templ", Name: "templ guidelines", Type: "review", Status: HookRunStatusRunning, RunCount: 1},
					{ID: "hook-sidebar-security", Name: "Security reviewer", Type: "review", Status: HookRunStatusPending},
				},
				SideChats: []Thread{
					{
						ID:           "thread-sidebar-design",
						Title:        "Compact layout",
						RelativeTime: "Now",
						Preview:      "Keep the side-chat viewer list-like and aligned with hooks.",
						Unread:       true,
						Messages: []ThreadMessage{
							{ID: "message-sidebar-design-1", Role: ThreadMessageRoleUser, Text: "Keep the side-chat viewer list-like."},
							{ID: "message-sidebar-design-2", Role: ThreadMessageRoleAssistant, Text: "I’ll align it with the hooks section."},
							{ID: "message-sidebar-design-3", Role: ThreadMessageRoleUser, Text: "Make active rows obvious."},
							{ID: "message-sidebar-design-4", Role: ThreadMessageRoleAssistant, Text: "Active rows use a compact selected style."},
							{ID: "message-sidebar-design-5", Role: ThreadMessageRoleAssistant, Text: "Unread rows keep a subtle accent."},
						},
					},
					{
						ID:           "thread-sidebar-files",
						Title:        "File approval notes",
						RelativeTime: "9m",
						Preview:      "Diff approval should stay visible below the file tree.",
						Messages: []ThreadMessage{
							{ID: "message-sidebar-files-1", Role: ThreadMessageRoleUser, Text: "Keep diff approval visible below the file tree."},
							{ID: "message-sidebar-files-2", Role: ThreadMessageRoleAssistant, Text: "The approval footer stays in the detail dock."},
							{ID: "message-sidebar-files-3", Role: ThreadMessageRoleUser, Text: "Good, keep it stable."},
						},
					},
				},
			},
			{
				ID:           "session-openai-ws",
				WorkspaceID:  "workspace-discobot",
				Title:        "Fix OpenAI websocket reconnect",
				RelativeTime: "Yesterday",
				Status:       SessionStatusIdle,
				Additions:    92,
				Deletions:    14,
				MainThread: Thread{
					ID: "thread-openai-ws-main",
					Messages: []ThreadMessage{
						{
							ID:   "message-openai-ws-main-1",
							Role: ThreadMessageRoleUser,
							Text: "Fix the reconnect flow when the OpenAI websocket drops.",
						},
					},
				},
			},
			{
				ID:           "session-provider-tests",
				WorkspaceID:  "workspace-agent",
				Title:        "Add provider websocket tests",
				RelativeTime: "2 days ago",
				Status:       SessionStatusIdle,
				Additions:    126,
				Deletions:    21,
				MainThread: Thread{
					ID: "thread-provider-tests-main",
					Messages: []ThreadMessage{
						{
							ID:   "message-provider-tests-main-1",
							Role: ThreadMessageRoleUser,
							Text: "Add provider websocket tests for the agent runtime.",
						},
					},
				},
			},
		},
	}
}

// DefaultView returns the initial server-owned view state.
func DefaultView() View {
	return View{
		PanelLayout: DefaultPanelLayout(),
		Settings: SettingsDialogState{
			Tab: SettingsTabAppearance,
		},
	}
}

// DefaultPanelLayout returns the initial server-owned workspace panel layout.
func DefaultPanelLayout() PanelLayout {
	return PanelLayout{
		Panels: map[string]Panel{
			"session": {
				ID:          "session",
				Visible:     true,
				Maximizable: false,
				GridColumn:  "1",
				GridRow:     "1 / -1",
				Width:       280,
				MinWidth:    220,
				MaxWidth:    460,
				Session: &SessionPanelState{
					SelectedSessionID:  "session-cobra",
					SelectedSideChatID: "thread-cobra-review",
					ExpandedSessionIDs: map[string]bool{
						"session-cobra": true,
					},
					ExpandedFileIDs: map[string]bool{
						"file-cobra-root":                  true,
						"file-cobra-agent":                 true,
						"file-cobra-agent-provider":        true,
						"file-cobra-agent-provider-openai": true,
						"file-cobra-vm":                    true,
						"file-cobra-vm-network":            true,
						"file-cobra-vm-scripts":            true,
						"file-cobra-cmd":                   true,
						"file-cobra-cmd-discobot":          true,
						"file-cobra-internal":              true,
						"file-cobra-internal-command":      true,
						"file-cobra-internal-state":        true,
						"file-cobra-internal-server":       true,
						"file-cobra-content":               true,
						"file-cobra-content-components":    true,
						"file-cobra-content-app":           true,
						"file-cobra-content-ui":            true,
						"file-cobra-static":                true,
						"file-cobra-static-lib":            true,
						"file-cobra-styles":                true,
						"file-cobra-docs":                  true,
						"file-cobra-docs-design":           true,
						"file-cobra-tests":                 true,
						"file-cobra-tests-fixtures":        true,
						"file-cobra-scripts":               true,
					},
					SessionMenuChecks: map[string]bool{
						"copilot-cli":          true,
						"cloud":                true,
						"claude":               true,
						"completed":            true,
						"in-progress":          true,
						"input-needed":         true,
						"failed":               true,
						"done":                 false,
						"read":                 true,
						"sort-created":         true,
						"sort-updated":         false,
						"group-workspace":      true,
						"group-time":           false,
						"show-recent-sessions": true,
						"show-all-sessions":    false,
					},
				},
			},
			"editor": {
				ID:          "editor",
				Visible:     true,
				Maximizable: true,
				GridColumn:  "5 / 6",
				GridRow:     "1",
				Editor: &EditorPanelState{
					OpenFileIDs: []string{},
				},
			},
			"composer": {
				ID:          "composer",
				Visible:     true,
				Maximizable: true,
				GridColumn:  "3",
				GridRow:     "1",
				Width:       360,
				MinWidth:    280,
				MaxWidth:    560,
				Composer: &ComposerPanelState{
					PromptHeight: 58,
				},
			},
			"terminal": {
				ID:          "terminal",
				Visible:     true,
				Maximizable: true,
				GridColumn:  "5 / 6",
				GridRow:     "3",
				Height:      220,
				MinHeight:   160,
				MaxHeight:   420,
				Terminal:    &TerminalPanelState{},
			},
		},
	}
}

// NewShell packages server app and view state for templ rendering.
func NewShell(data Data, view View) Shell {
	data = cloneData(data)
	view = normalizeShellView(data, NormalizeView(view))
	return Shell{
		Data: data,
		View: view,
	}
}

// NormalizeView returns a cloned view with all known panel defaults applied.
func NormalizeView(view View) View {
	view = cloneView(view)
	view.Settings.Tab = NormalizeSettingsTab(view.Settings.Tab)
	defaultLayout := DefaultPanelLayout()
	if view.PanelLayout.Panels == nil {
		view.PanelLayout = defaultLayout
		normalizeWorkspacePanelPlacement(&view)
		return view
	}

	panels := make(map[string]Panel, len(defaultLayout.Panels)+len(view.PanelLayout.Panels))
	for id, panel := range view.PanelLayout.Panels {
		panels[id] = panel
	}
	for id, defaultPanel := range defaultLayout.Panels {
		panel, ok := panels[id]
		if !ok {
			panels[id] = defaultPanel
			continue
		}
		panels[id] = normalizePanel(id, panel, defaultPanel)
	}

	view.PanelLayout.Panels = panels
	normalizeWorkspacePanelPlacement(&view)
	return view
}

func normalizeShellView(data Data, view View) View {
	if len(data.Sessions) == 0 {
		panel := view.PanelLayout.Panels["session"]
		panel.Visible = false
		view.PanelLayout.Panels["session"] = panel
		normalizeWorkspacePanelPlacement(&view)
	}
	return view
}

func normalizeWorkspacePanelPlacement(view *View) {
	panels := view.PanelLayout.Panels
	sessionVisible := panels["session"].Visible
	workspaceColumns := "3 / 6"
	workspaceStartColumn := "3"
	if !sessionVisible {
		workspaceColumns = "1 / 6"
		workspaceStartColumn = "1"
	}

	editor := panels["editor"]
	composer := panels["composer"]
	terminal := panels["terminal"]
	editorVisible := editor.Visible
	composerVisible := composer.Visible
	terminalVisible := terminal.Visible

	switch {
	case editorVisible && composerVisible && terminalVisible:
		composer.GridColumn = workspaceStartColumn + " / 4"
		composer.GridRow = "1 / -1"
		editor.GridColumn = "5 / 6"
		editor.GridRow = "1"
		terminal.GridColumn = "5 / 6"
		terminal.GridRow = "3"
	case editorVisible && composerVisible:
		composer.GridColumn = workspaceStartColumn + " / 4"
		composer.GridRow = "1 / -1"
		editor.GridColumn = "5 / 6"
		editor.GridRow = "1 / -1"
	case editorVisible && terminalVisible:
		editor.GridColumn = workspaceColumns
		editor.GridRow = "1"
		terminal.GridColumn = workspaceColumns
		terminal.GridRow = "3"
	case composerVisible && terminalVisible:
		composer.GridColumn = workspaceStartColumn + " / 4"
		composer.GridRow = "1 / -1"
		terminal.GridColumn = "5 / 6"
		terminal.GridRow = "1 / -1"
	case editorVisible:
		editor.GridColumn = workspaceColumns
		editor.GridRow = "1 / -1"
	case composerVisible:
		composer.GridColumn = workspaceColumns
		composer.GridRow = "1 / -1"
	case terminalVisible:
		terminal.GridColumn = workspaceColumns
		terminal.GridRow = "1 / -1"
	}

	panels["editor"] = editor
	panels["composer"] = composer
	panels["terminal"] = terminal
}

func normalizePanel(panelID string, panel Panel, defaultPanel Panel) Panel {
	if panel.ID == "" {
		panel.ID = defaultPanel.ID
	}
	if defaultPanel.Maximizable {
		panel.Maximizable = true
	} else {
		panel.Maximizable = false
		panel.Maximized = false
	}
	if panel.GridColumn == "" {
		panel.GridColumn = defaultPanel.GridColumn
	}
	if panel.GridRow == "" {
		panel.GridRow = defaultPanel.GridRow
	}
	if panel.Width == 0 {
		panel.Width = defaultPanel.Width
	}
	if panel.Height == 0 {
		panel.Height = defaultPanel.Height
	}
	if panel.MinWidth == 0 {
		panel.MinWidth = defaultPanel.MinWidth
	}
	if panel.MaxWidth == 0 {
		panel.MaxWidth = defaultPanel.MaxWidth
	}
	if panel.MinHeight == 0 {
		panel.MinHeight = defaultPanel.MinHeight
	}
	if panel.MaxHeight == 0 {
		panel.MaxHeight = defaultPanel.MaxHeight
	}

	switch panelID {
	case "session":
		if panel.Session == nil {
			panel.Session = cloneSessionPanelState(defaultPanel.Session)
		}
		panel.Editor = nil
		panel.Composer = nil
		panel.Terminal = nil
	case "editor":
		if panel.Editor == nil {
			panel.Editor = cloneEditorPanelState(defaultPanel.Editor)
		}
		panel.Session = nil
		panel.Composer = nil
		panel.Terminal = nil
	case "composer":
		if panel.Composer == nil {
			panel.Composer = cloneComposerPanelState(defaultPanel.Composer)
		}
		panel.Session = nil
		panel.Editor = nil
		panel.Terminal = nil
	case "terminal":
		if panel.Terminal == nil {
			panel.Terminal = &TerminalPanelState{}
		}
		panel.Session = nil
		panel.Editor = nil
		panel.Composer = nil
	}

	return panel
}

func cloneData(data Data) Data {
	if data.Workspaces != nil {
		data.Workspaces = append([]Workspace(nil), data.Workspaces...)
	}
	if data.Services != nil {
		data.Services = append([]Service(nil), data.Services...)
		for index := range data.Services {
			if data.Services[index].Logs != nil {
				data.Services[index].Logs = append([]string(nil), data.Services[index].Logs...)
			}
		}
	}
	if data.Sessions != nil {
		data.Sessions = append([]Session(nil), data.Sessions...)
		for index := range data.Sessions {
			data.Sessions[index].MainThread = cloneThread(data.Sessions[index].MainThread)
			if data.Sessions[index].Files != nil {
				data.Sessions[index].Files = append([]FileNode(nil), data.Sessions[index].Files...)
			}
			if data.Sessions[index].Hooks != nil {
				data.Sessions[index].Hooks = append([]SessionHook(nil), data.Sessions[index].Hooks...)
			}
			if data.Sessions[index].SideChats != nil {
				data.Sessions[index].SideChats = append([]Thread(nil), data.Sessions[index].SideChats...)
				for chatIndex := range data.Sessions[index].SideChats {
					data.Sessions[index].SideChats[chatIndex] = cloneThread(data.Sessions[index].SideChats[chatIndex])
				}
			}
		}
	}
	return data
}

func cloneThread(thread Thread) Thread {
	if thread.Messages != nil {
		thread.Messages = append([]ThreadMessage(nil), thread.Messages...)
	}
	return thread
}

// EnsurePanel returns a panel, initializing it from defaults when needed.
func EnsurePanel(view *View, panelID string) Panel {
	if view.PanelLayout.Panels == nil {
		view.PanelLayout.Panels = map[string]Panel{}
	}

	defaultPanel, hasDefault := DefaultPanelLayout().Panels[panelID]
	panel, ok := view.PanelLayout.Panels[panelID]
	if !hasDefault {
		view.PanelLayout.Panels[panelID] = panel
		return panel
	}
	if !ok {
		panel = defaultPanel
	} else {
		panel = normalizePanel(panelID, panel, defaultPanel)
	}
	view.PanelLayout.Panels[panelID] = panel
	return panel
}

// EnsureSessionPanelState returns mutable state for the session panel.
func EnsureSessionPanelState(view *View) *SessionPanelState {
	panel := EnsurePanel(view, "session")
	if panel.Session == nil {
		panel.Session = cloneSessionPanelState(DefaultPanelLayout().Panels["session"].Session)
	}
	panel.Editor = nil
	panel.Composer = nil
	panel.Terminal = nil
	view.PanelLayout.Panels["session"] = panel
	return panel.Session
}

// EnsureEditorPanelState returns mutable state for the editor panel.
func EnsureEditorPanelState(view *View) *EditorPanelState {
	panel := EnsurePanel(view, "editor")
	if panel.Editor == nil {
		panel.Editor = cloneEditorPanelState(DefaultPanelLayout().Panels["editor"].Editor)
	}
	panel.Session = nil
	panel.Composer = nil
	panel.Terminal = nil
	view.PanelLayout.Panels["editor"] = panel
	return panel.Editor
}

// EnsureComposerPanelState returns mutable state for the composer panel.
func EnsureComposerPanelState(view *View) *ComposerPanelState {
	panel := EnsurePanel(view, "composer")
	if panel.Composer == nil {
		panel.Composer = cloneComposerPanelState(DefaultPanelLayout().Panels["composer"].Composer)
	}
	panel.Session = nil
	panel.Editor = nil
	panel.Terminal = nil
	view.PanelLayout.Panels["composer"] = panel
	return panel.Composer
}

// EnsureTerminalPanelState returns mutable state for the terminal panel.
func EnsureTerminalPanelState(view *View) *TerminalPanelState {
	panel := EnsurePanel(view, "terminal")
	if panel.Terminal == nil {
		panel.Terminal = &TerminalPanelState{}
	}
	panel.Session = nil
	panel.Editor = nil
	panel.Composer = nil
	view.PanelLayout.Panels["terminal"] = panel
	return panel.Terminal
}

func cloneView(view View) View {
	if view.PanelLayout.Panels != nil {
		panels := make(map[string]Panel, len(view.PanelLayout.Panels))
		for key, value := range view.PanelLayout.Panels {
			value.Session = cloneSessionPanelState(value.Session)
			value.Editor = cloneEditorPanelState(value.Editor)
			value.Composer = cloneComposerPanelState(value.Composer)
			if value.Terminal != nil {
				value.Terminal = &TerminalPanelState{}
			}
			panels[key] = value
		}
		view.PanelLayout.Panels = panels
	}
	return view
}

func cloneSessionPanelState(state *SessionPanelState) *SessionPanelState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.ExpandedSessionIDs = cloneBoolMap(state.ExpandedSessionIDs)
	clone.VisibleSessionDetailSections = cloneBoolMap(state.VisibleSessionDetailSections)
	clone.ExpandedFileIDs = cloneBoolMap(state.ExpandedFileIDs)
	clone.SessionViewModes = cloneSessionViewModeMap(state.SessionViewModes)
	clone.SessionMenuChecks = cloneBoolMap(state.SessionMenuChecks)
	return &clone
}

func cloneEditorPanelState(state *EditorPanelState) *EditorPanelState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.OpenFileIDs != nil {
		clone.OpenFileIDs = append([]string(nil), state.OpenFileIDs...)
	}
	return &clone
}

func cloneComposerPanelState(state *ComposerPanelState) *ComposerPanelState {
	if state == nil {
		return nil
	}
	clone := *state
	return &clone
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	clone := make(map[string]bool, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneSessionViewModeMap(source map[string]SessionViewMode) map[string]SessionViewMode {
	if source == nil {
		return nil
	}
	clone := make(map[string]SessionViewMode, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
