package state

import "testing"

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

	for _, session := range data.Sessions {
		assertThreadHasMessages(t, session.ID+" main thread", session.MainThread)
		for _, thread := range session.SideChats {
			assertThreadHasMessages(t, session.ID+" side-chat thread", thread)
		}
	}
}

func TestNewShellClonesThreadMessages(t *testing.T) {
	data := DefaultData()
	shell := NewShell(data, DefaultView())

	shell.Data.Sessions[0].MainThread.Messages[0].Text = "changed main"
	if data.Sessions[0].MainThread.Messages[0].Text == "changed main" {
		t.Fatalf("main thread messages were not cloned")
	}

	shell.Data.Sessions[0].SideChats[0].Messages[0].Text = "changed side chat"
	if data.Sessions[0].SideChats[0].Messages[0].Text == "changed side chat" {
		t.Fatalf("side-chat thread messages were not cloned")
	}
}

func TestNewShellNormalizesPanelLayout(t *testing.T) {
	shell := NewShell(DefaultData(), View{
		PanelLayout: PanelLayout{
			Panels: map[string]Panel{
				"composer": {
					ID:      "composer",
					Visible: true,
					Width:   420,
				},
			},
		},
	})

	composer := shell.View.PanelLayout.Panels["composer"]
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
	if composer.Composer == nil {
		t.Fatalf("composer panel state is nil")
	}

	session := shell.View.PanelLayout.Panels["session"]
	if session.Maximizable {
		t.Fatalf("session maximizable = true, want false")
	}
	if session.Session == nil {
		t.Fatalf("missing session panel was not added from defaults")
	}
}

func TestNewShellNormalizesEmptyView(t *testing.T) {
	shell := NewShell(DefaultData(), View{})

	composer := shell.View.PanelLayout.Panels["composer"]
	if composer.GridColumn != "3 / 4" || composer.GridRow != "1 / -1" {
		t.Fatalf("composer placement = %q %q, want 3 / 4 and 1 / -1", composer.GridColumn, composer.GridRow)
	}
	editor := shell.View.PanelLayout.Panels["editor"]
	if !editor.Visible || editor.GridColumn != "5 / 6" || editor.GridRow != "1" {
		t.Fatalf("editor placement = visible %t %q %q, want visible 5 / 6 and 1", editor.Visible, editor.GridColumn, editor.GridRow)
	}
	terminal := shell.View.PanelLayout.Panels["terminal"]
	if !terminal.Visible || terminal.GridColumn != "5 / 6" || terminal.GridRow != "3" {
		t.Fatalf("terminal placement = visible %t %q %q, want visible 5 / 6 and 3", terminal.Visible, terminal.GridColumn, terminal.GridRow)
	}
	if shell.View.PanelLayout.Panels["session"].Session == nil {
		t.Fatalf("missing session panel state")
	}
}

func TestNewShellPlacesComposerBeforeEditor(t *testing.T) {
	shell := NewShell(DefaultData(), DefaultView())
	composer := shell.View.PanelLayout.Panels["composer"]
	editor := shell.View.PanelLayout.Panels["editor"]
	terminal := shell.View.PanelLayout.Panels["terminal"]

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
	editor := view.PanelLayout.Panels["editor"]
	editor.Visible = false
	view.PanelLayout.Panels["editor"] = editor

	shell := NewShell(DefaultData(), view)
	composer := shell.View.PanelLayout.Panels["composer"]
	terminal := shell.View.PanelLayout.Panels["terminal"]

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
	if !withSessions.View.PanelLayout.Panels["session"].Visible {
		t.Fatalf("session panel visible = false, want true when sessions exist")
	}

	data.Sessions = nil
	withoutSessions := NewShell(data, view)
	if withoutSessions.View.PanelLayout.Panels["session"].Visible {
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
		if message.Text == "" {
			t.Fatalf("%s message %q missing text", label, message.ID)
		}
	}
}
