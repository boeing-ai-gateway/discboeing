// Package state owns Discobot's server-side application and view state.
package state

// Shell is the render model for the full-page shell.
type Shell struct {
	Generation uint64
	Data       Data
	View       View
}

// View is server-owned UI view state.
type View struct {
	SessionsSidebarVisible bool
	TerminalPanelVisible   bool
	SelectedSessionID      string
	SelectedFileID         string
	OpenFileIDs            []string
	ActiveFileID           string
	ExpandedSessionIDs     map[string]bool
	ExpandedFileIDs        map[string]bool
	SessionMenuChecks      map[string]bool
	FileTreeSearch         string
}

// Data is server-owned application/domain state.
type Data struct {
	Title      string
	App        App
	Workspaces []Workspace
	Sessions   []Session
}

// App describes the running Discobot application.
type App struct {
	Name        string
	Description string
}

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
	Files        []FileNode
}

// FileNode is a server-owned file tree node scoped to a session.
type FileNode struct {
	ID                    string
	SessionID             string
	ParentID              string
	Name                  string
	Kind                  FileKind
	GitStatus             FileGitStatus
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
			Description: "A small Datastar + templ scaffold mirroring ui-go structure.",
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
		Sessions: []Session{
			{
				ID:           "session-cobra",
				WorkspaceID:  "workspace-disco",
				Title:        "Create Cobra app skeleton",
				RelativeTime: "3 days ago",
				Status:       SessionStatusIdle,
				Additions:    248,
				Deletions:    37,
				Files: []FileNode{
					{ID: "file-cobra-root", SessionID: "session-cobra", Name: "discobot", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "agent-go", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent-provider", SessionID: "session-cobra", ParentID: "file-cobra-agent", Name: "provider", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent-provider-openai", SessionID: "session-cobra", ParentID: "file-cobra-agent-provider", Name: "openai", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-agent-provider-openai-client", SessionID: "session-cobra", ParentID: "file-cobra-agent-provider-openai", Name: "client.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-server", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "server", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-ui", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "ui", Kind: FileKindDirectory},
					{ID: "file-cobra-vm", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "vm-assets", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-vm-network", SessionID: "session-cobra", ParentID: "file-cobra-vm", Name: "network", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-vm-network-bridge", SessionID: "session-cobra", ParentID: "file-cobra-vm-network", Name: "bridge.sh", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-cobra-vm-scripts", SessionID: "session-cobra", ParentID: "file-cobra-vm", Name: "scripts", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-cobra-vm-scripts-start", SessionID: "session-cobra", ParentID: "file-cobra-vm-scripts", Name: "start.sh", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-vm-fstab", SessionID: "session-cobra", ParentID: "file-cobra-vm", Name: "fstab", Kind: FileKindFile, GitStatus: FileGitStatusDeleted},
					{ID: "file-cobra-readme", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "README.md", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-cobra-package", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "package.json", Kind: FileKindFile, GitStatus: FileGitStatusUntracked},
					{ID: "file-cobra-go-mod", SessionID: "session-cobra", ParentID: "file-cobra-root", Name: "go.mod", Kind: FileKindFile, GitStatus: FileGitStatusRenamed},
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
				Files: []FileNode{
					{ID: "file-sidebar-root", SessionID: "session-sidebar", Name: "discobot", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-content", SessionID: "session-sidebar", ParentID: "file-sidebar-root", Name: "content", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-components", SessionID: "session-sidebar", ParentID: "file-sidebar-content", Name: "components", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-app", SessionID: "session-sidebar", ParentID: "file-sidebar-components", Name: "app", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-session", SessionID: "session-sidebar", ParentID: "file-sidebar-app", Name: "sessions_sidebar.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
					{ID: "file-sidebar-ui", SessionID: "session-sidebar", ParentID: "file-sidebar-components", Name: "ui", Kind: FileKindDirectory, HasChangedDescendants: true},
					{ID: "file-sidebar-tree", SessionID: "session-sidebar", ParentID: "file-sidebar-ui", Name: "file_tree.templ", Kind: FileKindFile, GitStatus: FileGitStatusAdded},
					{ID: "file-sidebar-state", SessionID: "session-sidebar", ParentID: "file-sidebar-root", Name: "state.go", Kind: FileKindFile, GitStatus: FileGitStatusModified},
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
			},
			{
				ID:           "session-provider-tests",
				WorkspaceID:  "workspace-agent",
				Title:        "Add provider websocket tests",
				RelativeTime: "2 days ago",
				Status:       SessionStatusIdle,
				Additions:    126,
				Deletions:    21,
			},
		},
	}
}

// DefaultView returns the initial server-owned view state.
func DefaultView() View {
	return View{
		SessionsSidebarVisible: true,
		SelectedSessionID:      "session-cobra",
		OpenFileIDs:            []string{},
		ExpandedSessionIDs: map[string]bool{
			"session-cobra": true,
		},
		ExpandedFileIDs: map[string]bool{
			"file-cobra-root": true,
			"file-cobra-vm":   true,
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
	}
}

// NewShell packages server app and view state for templ rendering.
func NewShell(generation uint64, data Data, view View) Shell {
	return Shell{
		Generation: generation,
		Data:       cloneData(data),
		View:       cloneView(view),
	}
}

func cloneData(data Data) Data {
	if data.Workspaces != nil {
		data.Workspaces = append([]Workspace(nil), data.Workspaces...)
	}
	if data.Sessions != nil {
		data.Sessions = append([]Session(nil), data.Sessions...)
		for index := range data.Sessions {
			if data.Sessions[index].Files != nil {
				data.Sessions[index].Files = append([]FileNode(nil), data.Sessions[index].Files...)
			}
		}
	}
	return data
}

func cloneView(view View) View {
	if view.OpenFileIDs != nil {
		view.OpenFileIDs = append([]string(nil), view.OpenFileIDs...)
	}

	if view.ExpandedSessionIDs != nil {
		expanded := make(map[string]bool, len(view.ExpandedSessionIDs))
		for key, value := range view.ExpandedSessionIDs {
			expanded[key] = value
		}
		view.ExpandedSessionIDs = expanded
	}

	if view.ExpandedFileIDs != nil {
		expanded := make(map[string]bool, len(view.ExpandedFileIDs))
		for key, value := range view.ExpandedFileIDs {
			expanded[key] = value
		}
		view.ExpandedFileIDs = expanded
	}

	if view.SessionMenuChecks != nil {
		checks := make(map[string]bool, len(view.SessionMenuChecks))
		for key, value := range view.SessionMenuChecks {
			checks[key] = value
		}
		view.SessionMenuChecks = checks
	}
	return view
}
