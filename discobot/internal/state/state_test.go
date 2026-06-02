package state

import (
	"testing"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"
)

func TestDeriveFileGitStatusFromPath(t *testing.T) {
	status := DeriveFileGitStatusFromPath("content/file_tree.ui", map[string]FileGitStatus{
		"content/file_tree.ui": FileGitStatusModified,
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

func TestNewDataClonesThreadMessages(t *testing.T) {
	data := DefaultData()
	snapshot := NewData(data)

	project := snapshot.Project["prototype"]
	sessionData := project.Session["session-cobra"]
	threadData := sessionData.Thread["session-cobra"]
	threadData.Messages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "changed main", State: "done"}
	sessionData.Thread["session-cobra"] = threadData
	project.Session["session-cobra"] = sessionData
	snapshot.Project["prototype"] = project
	if messageText(data.Project["prototype"].Session["session-cobra"].Thread["session-cobra"].Messages[0]) == "changed main" {
		t.Fatalf("main thread messages were not cloned")
	}

	project = snapshot.Project["prototype"]
	sessionData = project.Session["session-cobra"]
	threadData = sessionData.Thread["thread-cobra-review"]
	threadData.Messages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "changed side chat", State: "done"}
	sessionData.Thread["thread-cobra-review"] = threadData
	project.Session["session-cobra"] = sessionData
	snapshot.Project["prototype"] = project
	if messageText(data.Project["prototype"].Session["session-cobra"].Thread["thread-cobra-review"].Messages[0]) == "changed side chat" {
		t.Fatalf("side-chat thread messages were not cloned")
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
