package state

import (
	"testing"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"
)

func TestDeriveFileGitStatusFromPath(t *testing.T) {
	status := DeriveFileGitStatusFromPath("content/file_tree.templ", map[string]FileGitStatus{
		"content/file_tree.templ": FileGitStatusModified,
	})
	if status != FileGitStatusModified {
		t.Fatalf("status = %q, want modified", status)
	}
	if clean := DeriveFileGitStatusFromPath("missing", nil); clean != FileGitStatusClean {
		t.Fatalf("missing status = %q, want clean", clean)
	}
}

func TestDefaultSessionsHaveThreadMessages(t *testing.T) {
	data := DefaultData()

	for _, session := range Sessions(data) {
		assertThreadHasMessages(t, session.ID+" main thread", session.MainThread)
		for _, thread := range session.SideChats {
			assertThreadHasMessages(t, session.ID+" side-chat thread", thread)
		}
	}
}

func TestNewShellClonesThreadMessages(t *testing.T) {
	data := DefaultData()
	shell := NewShell(data, DefaultView())

	project := shell.Data.Project["prototype"]
	sessionData := project.Session["session-cobra"]
	threadData := sessionData.Thread["session-cobra"]
	threadData.Messages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "changed main", State: "done"}
	sessionData.Thread["session-cobra"] = threadData
	project.Session["session-cobra"] = sessionData
	shell.Data.Project["prototype"] = project
	if messageText(data.Project["prototype"].Session["session-cobra"].Thread["session-cobra"].Messages[0]) == "changed main" {
		t.Fatalf("main thread messages were not cloned")
	}

	project = shell.Data.Project["prototype"]
	sessionData = project.Session["session-cobra"]
	threadData = sessionData.Thread["thread-cobra-review"]
	threadData.Messages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "changed side chat", State: "done"}
	sessionData.Thread["thread-cobra-review"] = threadData
	project.Session["session-cobra"] = sessionData
	shell.Data.Project["prototype"] = project
	if messageText(data.Project["prototype"].Session["session-cobra"].Thread["thread-cobra-review"].Messages[0]) == "changed side chat" {
		t.Fatalf("side-chat thread messages were not cloned")
	}
}

func TestNewShellNormalizesPanelLayout(t *testing.T) {
	shell := NewShell(DefaultData(), View{
		GlobalPanelLayout: DefaultGlobalPanelLayout(),
		SessionPanelLayouts: map[string]*SessionPanelLayout{
			"session-cobra": {
				Conversation: Panel[ConversationPanelState]{
					ID:      "composer",
					Visible: true,
					Width:   420,
				},
			},
		},
	})

	composer := shell.View.SessionPanelLayouts["session-cobra"].Conversation
	if composer.GridColumn == "" || composer.GridRow == "" {
		t.Fatalf("composer grid placement was not normalized: %#v", composer)
	}
	if composer.Width != 420 {
		t.Fatalf("composer width = %d, want preserved custom width 420", composer.Width)
	}
	if composer.MinWidth == 0 || composer.MaxWidth == 0 {
		t.Fatalf("composer bounds were not normalized: %#v", composer)
	}
	if !composer.Maximizable {
		t.Fatalf("composer maximizable = false, want true")
	}
	session := shell.View.GlobalPanelLayout.SessionSidebar
	if session.Maximizable {
		t.Fatalf("session maximizable = true, want false")
	}
	if session.State.SelectedSessionID == "" {
		t.Fatalf("missing session panel was not added from defaults")
	}
}

func TestNewShellNormalizesEmptyView(t *testing.T) {
	shell := NewShell(DefaultData(), View{})

	composer := shell.View.SessionPanelLayouts["session-cobra"].Conversation
	if composer.GridColumn != "3 / 4" || composer.GridRow != "1 / -1" {
		t.Fatalf("composer placement = %q %q, want 3 / 4 and 1 / -1", composer.GridColumn, composer.GridRow)
	}
	editor := shell.View.SessionPanelLayouts["session-cobra"].Editor
	if !editor.Visible || editor.GridColumn != "5 / 6" || editor.GridRow != "1" {
		t.Fatalf("editor placement = visible %t %q %q, want visible 5 / 6 and 1", editor.Visible, editor.GridColumn, editor.GridRow)
	}
	terminal := shell.View.SessionPanelLayouts["session-cobra"].Terminal
	if !terminal.Visible || terminal.GridColumn != "5 / 6" || terminal.GridRow != "3" {
		t.Fatalf("terminal placement = visible %t %q %q, want visible 5 / 6 and 3", terminal.Visible, terminal.GridColumn, terminal.GridRow)
	}
	if shell.View.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID == "" {
		t.Fatalf("missing session panel state")
	}
}

func TestNewShellPlacesComposerBeforeEditor(t *testing.T) {
	shell := NewShell(DefaultData(), DefaultView())
	composer := shell.View.SessionPanelLayouts["session-cobra"].Conversation
	editor := shell.View.SessionPanelLayouts["session-cobra"].Editor
	terminal := shell.View.SessionPanelLayouts["session-cobra"].Terminal

	if composer.GridColumn != "3 / 4" || composer.GridRow != "1 / -1" {
		t.Fatalf("composer placement = %q %q, want middle column", composer.GridColumn, composer.GridRow)
	}
	if editor.GridColumn != "5 / 6" || editor.GridRow != "1" {
		t.Fatalf("editor placement = %q %q, want top far-right column", editor.GridColumn, editor.GridRow)
	}
	if terminal.GridColumn != "5 / 6" || terminal.GridRow != "3" {
		t.Fatalf("terminal placement = %q %q, want bottom far-right column", terminal.GridColumn, terminal.GridRow)
	}
}

func TestNewShellKeepsTerminalInRightColumnWhenEditorHidden(t *testing.T) {
	view := DefaultView()
	editor := view.SessionPanelLayouts["session-cobra"].Editor
	editor.Visible = false
	view.SessionPanelLayouts["session-cobra"].Editor = editor

	shell := NewShell(DefaultData(), view)
	composer := shell.View.SessionPanelLayouts["session-cobra"].Conversation
	terminal := shell.View.SessionPanelLayouts["session-cobra"].Terminal

	if composer.GridColumn != "3 / 4" || composer.GridRow != "1 / -1" {
		t.Fatalf("composer placement = %q %q, want middle column", composer.GridColumn, composer.GridRow)
	}
	if terminal.GridColumn != "5 / 6" || terminal.GridRow != "1 / -1" {
		t.Fatalf("terminal placement = %q %q, want full-height far-right column", terminal.GridColumn, terminal.GridRow)
	}
}

func TestNewShellHidesSessionPanelWithoutSessions(t *testing.T) {
	data := DefaultData()
	view := DefaultView()

	withSessions := NewShell(data, view)
	if !withSessions.View.GlobalPanelLayout.SessionSidebar.Visible {
		t.Fatalf("session panel visible = false, want true when sessions exist")
	}

	data.Project = nil
	withoutSessions := NewShell(data, view)
	if withoutSessions.View.GlobalPanelLayout.SessionSidebar.Visible {
		t.Fatalf("session panel visible = true, want false without sessions")
	}
}

func assertThreadHasMessages(t *testing.T, label string, thread Thread) {
	t.Helper()

	if thread.ID == "" {
		t.Fatalf("%s missing ID", label)
	}
	if len(thread.Messages) == 0 {
		t.Fatalf("%s missing messages", label)
	}
	for _, message := range thread.Messages {
		if message.ID == "" {
			t.Fatalf("%s has a message without an ID", label)
		}
		if message.Role == "" {
			t.Fatalf("%s message %q missing role", label, message.ID)
		}
		if messageText(message) == "" {
			t.Fatalf("%s message %q missing text", label, message.ID)
		}
	}
}

func messageText(message serverapi.Message) string {
	for _, part := range message.Parts {
		if part, ok := part.(agentmessage.UITextPart); ok {
			return part.Text
		}
	}
	return ""
}
