package state

import (
	"encoding/json"
	"sort"
	"strings"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"
)

// Data is server-owned application/domain state.
//
// Treat published Data values as immutable snapshots. Code that updates Data
// should clone the current value, mutate only the clone, then assign the clone
// back as the new snapshot. This keeps concurrent readers from observing in-place
// changes to maps, slices, or nested message parts without requiring read locks.
type Data struct {
	Title    string
	App      App
	Projects []serverapi.Project
	Project  map[string]ProjectData
}

// App describes the running app shown by clients.
type App struct {
	Name        string
	Description string
}

// ServiceData mirrors service-scoped state that is not part of the server API.
type ServiceData struct {
	Logs []string
}

// ProjectData mirrors one Discobot server project using generated client types.
type ProjectData struct {
	Project    serverapi.Project
	Workspaces []serverapi.Workspace
	Workspace  map[string]serverapi.Workspace
	Models     []serverapi.ModelInfo
	Sessions   []serverapi.Session
	Session    map[string]SessionData
}

// SessionData mirrors session-scoped state for one generated session.
type SessionData struct {
	Session   serverapi.Session
	Services  []serverapi.Service
	Service   map[string]ServiceData
	Threads   []serverapi.Thread
	Thread    map[string]ThreadData
	Files     []FileNode
	Hooks     []SessionHook
	Additions int
	Deletions int
}

// ThreadData mirrors thread-scoped state for one generated thread.
type ThreadData struct {
	Thread          serverapi.Thread
	Messages        []serverapi.Message
	PendingHistory  bool
	PendingMessages []serverapi.Message
}

// ServiceStatus describes whether a service is running.
type ServiceStatus string

const (
	// ServiceStatusStopped means the service is not running.
	ServiceStatusStopped ServiceStatus = "stopped"
	// ServiceStatusRunning means the service is running.
	ServiceStatusRunning ServiceStatus = "running"
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
	Messages     []serverapi.Message
}

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
	serverapi.WorkspaceFileEntry
	ID                    string
	SessionID             string
	ParentID              string
	GitStatus             FileGitStatus
	Approved              bool
	HasChangedDescendants bool
}

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
	defaultSessions := []Session{
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
				Messages: []serverapi.Message{
					sampleMessage("message-cobra-main-1", "user", "Add a session-header close button that hides this panel without resetting what I expanded."),
					sampleMessage("message-cobra-main-2", "assistant", "I’ll keep the session detail state intact and only change the panel visibility."),
					sampleMessage("message-cobra-main-3", "user", "While you are in there, make the center workspace behave like a real conversation view. I need enough fixture content to test the way it grows upward from the composer when the thread is no longer empty."),
					sampleMessage("message-cobra-main-4", "assistant", "I’ll expand the sample main thread so it produces a tall scrollable conversation and keeps the composer anchored near the bottom of the workspace."),
					sampleMessage("message-cobra-main-5", "user", "The important part is that I can see what happens when content wants to use the whole screen. A short two-message sample does not show the overflow behavior at all."),
					sampleMessage("message-cobra-main-6", "assistant", "That makes sense. I’ll add enough messages with realistic wrapping so the center area has to allocate space between the conversation, the session panel, and the composer."),
					sampleMessage("message-cobra-main-7", "user", "Please keep the messages varied. Some should be short status pings, and others should wrap over multiple lines so that I can judge bubble spacing and vertical rhythm."),
					sampleMessage("message-cobra-main-8", "assistant", "I’ll include a mix of compact updates and longer assistant responses. The fixture should be obviously fake but useful for tuning scroll regions and panel boundaries."),
					sampleMessage("message-cobra-main-9", "user", "Also make sure the panel buttons remain visible near the composer. I want the panel to grow upward, not push the controls below the viewport."),
					sampleMessage("message-cobra-main-10", "assistant", "The bottom-aligned center stack will keep its bottom offset, cap itself against the top buffer, and let the scrollable children absorb overflow instead of moving the composer off screen."),
					sampleMessage("message-cobra-main-11", "user", "Good. The side session panel should get as much room as possible too. It currently feels cramped when workspace, hooks, and review sections are open."),
					sampleMessage("message-cobra-main-12", "assistant", "I’ll remove the small fixed cap from the session panel and let it flex into the available vertical space. Its details area will keep scrolling internally when the panel content is taller than the allocated area."),
					sampleMessage("message-cobra-main-13", "user", "Can you use about a fifty pixel top buffer? I want it to go almost to the top but not collide with the chrome or feel glued to the edge."),
					sampleMessage("message-cobra-main-14", "assistant", "Yes. With the bottom offset still at thirty-four pixels, the bottom-aligned stack can use the viewport height minus eighty-four pixels, leaving roughly fifty pixels at the top."),
					sampleMessage("message-cobra-main-15", "user", "This should make it obvious whether the session panel can stretch naturally while the conversation is long."),
					sampleMessage("message-cobra-main-16", "assistant", "Exactly. The long fixture content will force the layout to exercise the same constraints a real working session would hit after several back-and-forth turns."),
					sampleMessage("message-cobra-main-17", "user", "Short ping."),
					sampleMessage("message-cobra-main-18", "assistant", "Acknowledged. The short rows help verify that compact messages do not create awkward gaps between longer wrapped bubbles."),
					sampleMessage("message-cobra-main-19", "user", "Add a couple more so the scrollbar is definitely present on a laptop-height viewport."),
					sampleMessage("message-cobra-main-20", "assistant", "I’ll add enough fixture turns to exceed the available center height on common screens. That should make the scroll behavior and the top buffer easy to inspect without needing live agent output."),
					sampleMessage("message-cobra-main-21", "user", "The goal is not perfect copy. The goal is visual pressure on the layout."),
					sampleMessage("message-cobra-main-22", "assistant", "Understood. I’ll optimize the sample data for visual density and layout coverage, not for production-quality conversation content."),
					sampleMessage("message-cobra-main-23", "user", "Make sure checks still pass."),
					sampleMessage("message-cobra-main-24", "assistant", "I’ll run the Discobot checks after the layout changes so the Go output stays in sync."),
				},
			},
			Files: []FileNode{
				{ID: "file-cobra-root", SessionID: "session-cobra", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-agent", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/agent-go", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-agent-provider", SessionID: "session-cobra", ParentID: "file-cobra-agent", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/agent-go/provider", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-agent-provider-openai", SessionID: "session-cobra", ParentID: "file-cobra-agent-provider", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/agent-go/provider/openai", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-agent-provider-openai-client", SessionID: "session-cobra", ParentID: "file-cobra-agent-provider-openai", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/agent-go/provider/openai/client.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-server", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/server", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-ui", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/ui", IsDir: true}},
				{ID: "file-cobra-vm", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/vm-assets", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-vm-network", SessionID: "session-cobra", ParentID: "file-cobra-vm", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/vm-assets/network", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-vm-network-bridge", SessionID: "session-cobra", ParentID: "file-cobra-vm-network", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/vm-assets/network/bridge.sh", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-vm-scripts", SessionID: "session-cobra", ParentID: "file-cobra-vm", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/vm-assets/scripts", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-vm-scripts-start", SessionID: "session-cobra", ParentID: "file-cobra-vm-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/vm-assets/scripts/start.sh", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-vm-fstab", SessionID: "session-cobra", ParentID: "file-cobra-vm", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/vm-assets/fstab", IsDir: false}, GitStatus: FileGitStatusDeleted},
				{ID: "file-cobra-readme", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/README.md", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-package", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/package.json", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-go-mod", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/go.mod", IsDir: false}, GitStatus: FileGitStatusRenamed, Approved: true},
				{ID: "file-cobra-cmd", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/cmd", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-cmd-discobot", SessionID: "session-cobra", ParentID: "file-cobra-cmd", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/cmd/discobot", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-internal", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-internal-command", SessionID: "session-cobra", ParentID: "file-cobra-internal", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-internal-state", SessionID: "session-cobra", ParentID: "file-cobra-internal", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-internal-server", SessionID: "session-cobra", ParentID: "file-cobra-internal", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-content", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-content-components", SessionID: "session-cobra", ParentID: "file-cobra-content", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-content-app", SessionID: "session-cobra", ParentID: "file-cobra-content-components", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-content-ui", SessionID: "session-cobra", ParentID: "file-cobra-content-components", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-static", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-static-lib", SessionID: "session-cobra", ParentID: "file-cobra-static", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-styles", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-docs", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-docs-design", SessionID: "session-cobra", ParentID: "file-cobra-docs", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-tests", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-tests-fixtures", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixtures", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-scripts", SessionID: "session-cobra", ParentID: "file-cobra-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-cobra-cmd-main", SessionID: "session-cobra", ParentID: "file-cobra-cmd-discobot", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/cmd/discobot/main.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-command-handler", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/handler.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-command-layout", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/layout_resize.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-command-panel", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/panel.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-command-response", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/response.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-state-model", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/state.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-state-test", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/state_test.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-server-router", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/server.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-app-shell", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/app_shell.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-session-workspace", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/session_workspace.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-conversation", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/conversation.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-sidebar-details", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/sessions_sidebar_session_details.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-panel-ui", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/panel.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-resize-ui", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/resize_handle.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-static-resize", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/resize.js", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-styles-app", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/app.css", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-docs-guidelines", SessionID: "session-cobra", ParentID: "file-cobra-docs", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/GUIDELINES.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-docs-file-tree", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/file-tree-feature-gap.md", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-e2e-layout", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/panel-layout.spec.ts", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-log", SessionID: "session-cobra", ParentID: "file-cobra-tests-fixtures", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixtures/long-session.json", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-script-check", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/check-layout.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-001", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_001.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-002", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_002.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-003", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_003.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-004", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_004.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-005", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_005.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-006", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_006.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-007", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_007.go", IsDir: false}, GitStatus: FileGitStatusUntracked, Approved: true},
				{ID: "file-cobra-fixture-008", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_008.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-009", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_009.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-010", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_010.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-011", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_011.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-012", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_012.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-013", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_013.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-014", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_014.md", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-015", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_015.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-016", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_016.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-017", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_017.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-018", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_018.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-019", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_019.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-020", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_020.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-021", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_021.go", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-022", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_022.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-023", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_023.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-024", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_024.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-025", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_025.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-026", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_026.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-027", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_027.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-028", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_028.js", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-fixture-029", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_029.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-030", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_030.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-031", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_031.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-032", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_032.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-033", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_033.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-034", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_034.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-035", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_035.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-fixture-036", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_036.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-037", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_037.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-038", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_038.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-039", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_039.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-040", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_040.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-041", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_041.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-042", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_042.go", IsDir: false}, GitStatus: FileGitStatusUntracked, Approved: true},
				{ID: "file-cobra-fixture-043", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_043.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-044", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_044.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-045", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_045.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-046", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_046.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-047", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_047.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-048", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_048.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-049", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_049.md", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-050", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_050.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-051", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_051.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-052", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_052.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-053", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_053.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-054", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_054.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-055", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_055.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-056", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_056.go", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-057", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_057.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-058", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_058.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-059", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_059.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-060", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_060.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-061", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_061.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-062", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_062.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-063", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_063.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-064", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_064.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-065", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_065.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-066", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_066.js", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-fixture-067", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_067.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-068", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_068.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-069", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_069.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-070", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_070.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-071", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_071.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-072", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_072.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-073", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_073.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-074", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_074.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-075", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_075.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-076", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_076.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-077", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_077.md", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-078", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_078.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-079", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_079.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-080", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_080.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-081", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_081.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-082", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_082.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-083", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_083.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-084", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_084.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-085", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_085.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-086", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_086.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-087", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_087.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-088", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_088.ts", IsDir: false}, GitStatus: FileGitStatusUntracked, Approved: true},
				{ID: "file-cobra-fixture-089", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_089.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-090", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_090.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-091", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_091.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-092", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_092.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-093", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_093.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-094", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_094.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-095", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_095.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-096", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_096.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-097", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_097.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-098", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_098.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-099", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_099.mjs", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-100", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_100.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-101", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_101.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-102", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_102.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-103", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_103.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-104", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_104.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-105", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_105.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-106", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_106.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-107", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_107.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-108", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_108.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-109", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_109.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-110", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_110.css", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-fixture-111", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_111.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-112", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_112.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-113", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_113.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-114", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_114.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-115", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_115.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-116", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_116.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-117", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_117.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-118", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_118.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-119", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_119.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-120", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_120.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-121", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_121.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-fixture-122", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_122.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-123", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_123.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-124", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_124.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-125", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_125.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-126", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_126.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-127", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_127.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-128", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_128.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-129", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_129.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-130", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_130.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-131", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_131.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-132", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_132.go", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-133", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_133.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-134", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_134.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-135", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_135.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-136", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_136.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-137", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_137.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-138", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_138.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-139", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_139.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-140", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_140.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-141", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_141.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-142", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_142.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-143", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_143.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-cobra-fixture-144", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_144.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-145", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_145.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-146", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_146.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-147", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_147.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-148", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_148.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-149", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_149.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-150", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_150.css", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-151", SessionID: "session-cobra", ParentID: "file-cobra-content-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/fixture_151.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-152", SessionID: "session-cobra", ParentID: "file-cobra-content-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/fixture_152.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-153", SessionID: "session-cobra", ParentID: "file-cobra-internal-command", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/command/fixture_153.go", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-154", SessionID: "session-cobra", ParentID: "file-cobra-internal-state", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/state/fixture_154.go", IsDir: false}, GitStatus: FileGitStatusAdded, Approved: true},
				{ID: "file-cobra-fixture-155", SessionID: "session-cobra", ParentID: "file-cobra-internal-server", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/internal/server/fixture_155.go", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-156", SessionID: "session-cobra", ParentID: "file-cobra-static-lib", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/static/lib/fixture_156.js", IsDir: false}, GitStatus: FileGitStatusModified},
				{ID: "file-cobra-fixture-157", SessionID: "session-cobra", ParentID: "file-cobra-docs-design", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/docs/design/fixture_157.md", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-158", SessionID: "session-cobra", ParentID: "file-cobra-tests", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/tests/fixture_158.ts", IsDir: false}, GitStatus: FileGitStatusUntracked},
				{ID: "file-cobra-fixture-159", SessionID: "session-cobra", ParentID: "file-cobra-scripts", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/scripts/fixture_159.mjs", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-cobra-fixture-160", SessionID: "session-cobra", ParentID: "file-cobra-styles", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/styles/fixture_160.css", IsDir: false}, GitStatus: FileGitStatusModified},
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
					Messages: []serverapi.Message{
						sampleMessage("message-cobra-review-1", "user", "Can you review the command wiring?"),
						sampleMessage("message-cobra-review-2", "assistant", "The command wiring looks ready after the route cleanup."),
						sampleMessage("message-cobra-review-3", "user", "Please double-check the commit path."),
						sampleMessage("message-cobra-review-4", "assistant", "The commit path preserves the expanded session state."),
					},
				},
				{
					ID:           "thread-cobra-terminal",
					Title:        "Terminal output",
					RelativeTime: "1h",
					Preview:      "go test ./... is clean.",
					Messages: []serverapi.Message{
						sampleMessage("message-cobra-terminal-1", "user", "Run the checks."),
						sampleMessage("message-cobra-terminal-2", "assistant", "go test ./... is clean."),
						sampleMessage("message-cobra-terminal-3", "user", "Any generated files changed?"),
						sampleMessage("message-cobra-terminal-4", "assistant", "Only the expected files changed."),
						sampleMessage("message-cobra-terminal-5", "user", "Keep the terminal summary short."),
						sampleMessage("message-cobra-terminal-6", "assistant", "Checks passed."),
						sampleMessage("message-cobra-terminal-7", "assistant", "No follow-up failures."),
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
				Messages: []serverapi.Message{
					sampleMessage("message-sidebar-main-1", "user", "Keep the sidebar compact while preserving the selected session state."),
				},
			},
			Files: []FileNode{
				{ID: "file-sidebar-root", SessionID: "session-sidebar", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-sidebar-content", SessionID: "session-sidebar", ParentID: "file-sidebar-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-sidebar-components", SessionID: "session-sidebar", ParentID: "file-sidebar-content", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-sidebar-app", SessionID: "session-sidebar", ParentID: "file-sidebar-components", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-sidebar-session", SessionID: "session-sidebar", ParentID: "file-sidebar-app", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/app/sessions_sidebar.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
				{ID: "file-sidebar-ui", SessionID: "session-sidebar", ParentID: "file-sidebar-components", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui", IsDir: true}, HasChangedDescendants: true},
				{ID: "file-sidebar-tree", SessionID: "session-sidebar", ParentID: "file-sidebar-ui", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/content/components/ui/file_tree.go", IsDir: false}, GitStatus: FileGitStatusAdded},
				{ID: "file-sidebar-state", SessionID: "session-sidebar", ParentID: "file-sidebar-root", WorkspaceFileEntry: serverapi.WorkspaceFileEntry{Path: "discobot/state.go", IsDir: false}, GitStatus: FileGitStatusModified, Approved: true},
			},
			Hooks: []SessionHook{
				{ID: "hook-sidebar-ui", Name: "UI guidelines", Type: "review", Status: HookRunStatusRunning, RunCount: 1},
				{ID: "hook-sidebar-security", Name: "Security reviewer", Type: "review", Status: HookRunStatusPending},
			},
			SideChats: []Thread{
				{
					ID:           "thread-sidebar-design",
					Title:        "Compact layout",
					RelativeTime: "Now",
					Preview:      "Keep the side-chat viewer list-like and aligned with hooks.",
					Unread:       true,
					Messages: []serverapi.Message{
						sampleMessage("message-sidebar-design-1", "user", "Keep the side-chat viewer list-like."),
						sampleMessage("message-sidebar-design-2", "assistant", "I’ll align it with the hooks section."),
						sampleMessage("message-sidebar-design-3", "user", "Make active rows obvious."),
						sampleMessage("message-sidebar-design-4", "assistant", "Active rows use a compact selected style."),
						sampleMessage("message-sidebar-design-5", "assistant", "Unread rows keep a subtle accent."),
					},
				},
				{
					ID:           "thread-sidebar-files",
					Title:        "File approval notes",
					RelativeTime: "9m",
					Preview:      "Diff approval should stay visible below the file tree.",
					Messages: []serverapi.Message{
						sampleMessage("message-sidebar-files-1", "user", "Keep diff approval visible below the file tree."),
						sampleMessage("message-sidebar-files-2", "assistant", "The approval footer stays in the detail dock."),
						sampleMessage("message-sidebar-files-3", "user", "Good, keep it stable."),
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
				Messages: []serverapi.Message{
					sampleMessage("message-openai-ws-main-1", "user", "Fix the reconnect flow when the OpenAI websocket drops."),
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
				Messages: []serverapi.Message{
					sampleMessage("message-provider-tests-main-1", "user", "Add provider websocket tests for the agent runtime."),
				},
			},
		},
	}

	workspaces := []serverapi.Workspace{
		{
			ID:          "workspace-disco",
			DisplayName: new("DISCO"),
			SourceType:  "local",
			Path:        "/Users/you/src/disco",
			Status:      "Local Discobot workspace",
		},
		{
			ID:          "workspace-discobot",
			DisplayName: new("discobot"),
			SourceType:  "git",
			Path:        "git@github.com:obot-platform/discobot.git",
			Status:      "Discobot Git repository",
		},
		{
			ID:          "workspace-agent",
			DisplayName: new("agent-go"),
			SourceType:  "local",
			Path:        "/Users/you/src/agent-go",
			Status:      "Local agent provider work",
		},
	}

	return Data{
		Title: "Discobot",
		App: App{
			Name:        "discobot",
			Description: "Server-owned Discobot UI state streamed as JSON.",
		},
		Project: defaultProjectData(workspaces, defaultSessions),
	}
}

func defaultProjectData(workspaces []serverapi.Workspace, sessions []Session) map[string]ProjectData {
	project := ProjectData{
		Project:    serverapi.Project{ID: "prototype"},
		Workspaces: append([]serverapi.Workspace(nil), workspaces...),
		Session:    map[string]SessionData{},
		Workspace:  map[string]serverapi.Workspace{},
	}
	for _, workspace := range project.Workspaces {
		project.Workspace[workspace.ID] = workspace
	}
	for _, session := range sessions {
		project.Sessions = append(project.Sessions, serverapi.Session{
			ID:            session.ID,
			WorkspaceID:   session.WorkspaceID,
			DisplayName:   new(session.Title),
			SandboxStatus: string(session.Status),
		})
		mainThread := serverapi.Thread{ID: session.ID, Name: session.MainThread.Title}
		threads := []serverapi.Thread{mainThread}
		threadData := map[string]ThreadData{
			mainThread.ID: {
				Thread:   mainThread,
				Messages: session.MainThread.Messages,
			},
		}
		for _, sideChat := range session.SideChats {
			thread := serverapi.Thread{ID: sideChat.ID, Name: sideChat.Title, LastMessage: new(sideChat.Preview)}
			threads = append(threads, thread)
			threadData[sideChat.ID] = ThreadData{Thread: thread, Messages: sideChat.Messages}
		}
		project.Session[session.ID] = SessionData{
			Session:   project.Sessions[len(project.Sessions)-1],
			Services:  defaultSessionServices(session.ID),
			Service:   defaultSessionServiceData(session.ID),
			Threads:   threads,
			Thread:    threadData,
			Files:     session.Files,
			Hooks:     session.Hooks,
			Additions: session.Additions,
			Deletions: session.Deletions,
		}
	}
	return map[string]ProjectData{project.Project.ID: project}
}

func defaultSessionServices(sessionID string) []serverapi.Service {
	if sessionID != "session-cobra" {
		return nil
	}
	return []serverapi.Service{
		{
			ID:     new("dev-server"),
			Name:   new("Dev Server"),
			Status: new(string(ServiceStatusRunning)),
			URL:    new("http://localhost:3100"),
		},
		{
			ID:     new("api-server"),
			Name:   new("API Server"),
			Status: new(string(ServiceStatusStopped)),
		},
	}
}

func defaultSessionServiceData(sessionID string) map[string]ServiceData {
	if sessionID != "session-cobra" {
		return nil
	}
	return map[string]ServiceData{
		"dev-server": {
			Logs: []string{
				"21:40:04 [dev-server] starting pnpm dev",
				"21:40:05 [dev-server] local: http://localhost:3100",
				"21:40:06 [dev-server] ready in 812ms",
			},
		},
		"api-server": {
			Logs: []string{
				"21:35:11 [api-server] stopped",
				"21:35:11 [api-server] exit status 0",
			},
		},
	}
}

// Sessions returns rendered session summaries derived from project data.
func Sessions(data Data) []Session {
	return renderSessions(data.Project)
}

// Workspaces returns rendered workspace summaries derived from project data.
func Workspaces(data Data) []serverapi.Workspace {
	var workspaces []serverapi.Workspace
	for _, project := range data.Project {
		workspaces = append(workspaces, project.Workspaces...)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaceName(workspaces[i]) < workspaceName(workspaces[j])
	})
	return workspaces
}

func workspaceName(workspace serverapi.Workspace) string {
	if workspace.DisplayName != nil && *workspace.DisplayName != "" {
		return *workspace.DisplayName
	}
	if workspace.Path != "" {
		return workspace.Path
	}
	return workspace.ID
}

func renderSessions(projects map[string]ProjectData) []Session {
	var sessions []Session
	for _, project := range projects {
		for _, source := range project.Sessions {
			sessionData := project.Session[source.ID]
			sessions = append(sessions, renderSession(source, sessionData))
		}
	}
	return sessions
}

func renderSession(source serverapi.Session, sessionData SessionData) Session {
	return Session{
		ID:          source.ID,
		WorkspaceID: sessionWorkspaceID(source),
		Title:       sessionTitle(source),
		Status:      renderSessionStatus(source),
		Additions:   sessionData.Additions,
		Deletions:   sessionData.Deletions,
		MainThread:  renderMainThread(sessionData),
		Files:       sessionData.Files,
		Hooks:       sessionData.Hooks,
		SideChats:   renderSideChats(sessionData),
	}
}

func renderMainThread(sessionData SessionData) Thread {
	if threadData, ok := sessionData.Thread[sessionData.Session.ID]; ok {
		return renderThread(threadData.Thread, threadData.Messages)
	}
	threads := sortedThreads(sessionData.Thread)
	if len(threads) == 0 {
		return Thread{ID: sessionData.Session.ID}
	}
	threadData := sessionData.Thread[threads[0].ID]
	return renderThread(threadData.Thread, threadData.Messages)
}

func renderSideChats(sessionData SessionData) []Thread {
	threads := sortedThreads(sessionData.Thread)
	rendered := make([]Thread, 0, len(threads))
	for _, thread := range threads {
		if thread.ID == sessionData.Session.ID {
			continue
		}
		threadData := sessionData.Thread[thread.ID]
		rendered = append(rendered, renderThread(threadData.Thread, threadData.Messages))
	}
	return rendered
}

func sortedThreads(threads map[string]ThreadData) []serverapi.Thread {
	result := make([]serverapi.Thread, 0, len(threads))
	for _, threadData := range threads {
		result = append(result, threadData.Thread)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func renderThread(thread serverapi.Thread, messages []serverapi.Message) Thread {
	rendered := Thread{ID: thread.ID, Title: thread.Name, Preview: stringValue(thread.LastMessage), Messages: messages}
	if rendered.Preview == "" && len(rendered.Messages) > 0 {
		rendered.Preview = messagePartText(rendered.Messages[len(rendered.Messages)-1])
	}
	return rendered
}

func messagePartText(message serverapi.Message) string {
	var parts []string
	for _, part := range message.Parts {
		switch part := part.(type) {
		case agentmessage.UITextPart:
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		case agentmessage.UIReasoningPart:
			if part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func renderSessionStatus(session serverapi.Session) SessionStatus {
	if session.SandboxStatus == "running" || (session.ThreadStatus != nil && session.ThreadStatus.Status == "running") {
		return SessionStatusRunning
	}
	return SessionStatusIdle
}

func sessionWorkspaceID(session serverapi.Session) string {
	if session.WorkspaceID != "" {
		return session.WorkspaceID
	}
	return session.ProjectID
}

func sessionTitle(session serverapi.Session) string {
	if session.DisplayName != nil && *session.DisplayName != "" {
		return *session.DisplayName
	}
	if session.Name != "" {
		return session.Name
	}
	return session.ID
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sampleMessage(id, role, text string) serverapi.Message {
	return serverapi.Message{
		ID:   id,
		Role: role,
		Parts: []agentmessage.UIPart{
			agentmessage.UITextPart{Type: "text", Text: text, State: "done"},
		},
	}
}

func cloneData(data Data) Data {
	if data.Projects != nil {
		data.Projects = append([]serverapi.Project(nil), data.Projects...)
	}
	if data.Project != nil {
		projects := make(map[string]ProjectData, len(data.Project))
		for projectID, projectData := range data.Project {
			projects[projectID] = CloneProjectData(projectData)
		}
		data.Project = projects
	}
	return data
}

// CloneProjectData returns an independent copy of project-scoped API state.
func CloneProjectData(project ProjectData) ProjectData {
	if project.Workspaces != nil {
		project.Workspaces = append([]serverapi.Workspace(nil), project.Workspaces...)
	}
	if project.Models != nil {
		project.Models = cloneModels(project.Models)
	}
	if project.Workspace != nil {
		workspaces := make(map[string]serverapi.Workspace, len(project.Workspace))
		for workspaceID, workspace := range project.Workspace {
			workspaces[workspaceID] = workspace
		}
		project.Workspace = workspaces
	}
	if project.Sessions != nil {
		project.Sessions = append([]serverapi.Session(nil), project.Sessions...)
	}
	if project.Session != nil {
		sessions := make(map[string]SessionData, len(project.Session))
		for sessionID, sessionData := range project.Session {
			sessions[sessionID] = cloneSessionData(sessionData)
		}
		project.Session = sessions
	}
	return project
}

func cloneModels(models []serverapi.ModelInfo) []serverapi.ModelInfo {
	clone := make([]serverapi.ModelInfo, len(models))
	for index, model := range models {
		clone[index] = model
		if model.ReasoningLevels != nil {
			levels := append([]string(nil), (*model.ReasoningLevels)...)
			clone[index].ReasoningLevels = &levels
		}
		if model.ServiceTiers != nil {
			tiers := append([]string(nil), (*model.ServiceTiers)...)
			clone[index].ServiceTiers = &tiers
		}
	}
	return clone
}

func cloneServiceData(service ServiceData) ServiceData {
	if service.Logs != nil {
		service.Logs = append([]string(nil), service.Logs...)
	}
	return service
}

func cloneSessionData(session SessionData) SessionData {
	if session.Services != nil {
		session.Services = append([]serverapi.Service(nil), session.Services...)
	}
	if session.Service != nil {
		services := make(map[string]ServiceData, len(session.Service))
		for serviceID, serviceData := range session.Service {
			services[serviceID] = cloneServiceData(serviceData)
		}
		session.Service = services
	}
	if session.Threads != nil {
		session.Threads = append([]serverapi.Thread(nil), session.Threads...)
	}
	if session.Thread != nil {
		threads := make(map[string]ThreadData, len(session.Thread))
		for threadID, threadData := range session.Thread {
			threads[threadID] = cloneThreadData(threadData)
		}
		session.Thread = threads
	}
	return session
}

func cloneThreadData(thread ThreadData) ThreadData {
	if thread.Messages != nil {
		thread.Messages = cloneMessages(thread.Messages)
	}
	if thread.PendingMessages != nil {
		thread.PendingMessages = cloneMessages(thread.PendingMessages)
	}
	return thread
}

func cloneMessages(messages []serverapi.Message) []serverapi.Message {
	clone := make([]serverapi.Message, len(messages))
	for index, message := range messages {
		clone[index] = cloneMessage(message)
	}
	return clone
}

func cloneMessage(message serverapi.Message) serverapi.Message {
	data, err := json.Marshal(message)
	if err != nil {
		return message
	}
	var clone serverapi.Message
	if err := json.Unmarshal(data, &clone); err != nil {
		return message
	}
	return clone
}

func cloneThread(thread Thread) Thread {
	if thread.Messages != nil {
		thread.Messages = cloneMessages(thread.Messages)
	}
	return thread
}
