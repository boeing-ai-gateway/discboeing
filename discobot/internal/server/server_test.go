package server

import (
	"testing"

	agentmessage "github.com/obot-platform/discobot/agent-go/message"
	serverapi "github.com/obot-platform/discobot/server/api"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestSaveDataUsesCopyMutateAssign(t *testing.T) {
	server := &Server{
		data:        testImmutableData(),
		view:        state.DefaultView(),
		subscribers: map[chan struct{}]struct{}{},
	}
	previous := server.data

	server.SaveData(func(data *state.Data) {
		project := data.Project["project-1"]
		session := project.Session["session-1"]
		thread := session.Thread["thread-1"]
		thread.Messages[0].Parts[0] = agentmessage.UITextPart{Type: "text", Text: "updated", State: "done"}
		session.Thread["thread-1"] = thread
		project.Session["session-1"] = session
		data.Project["project-1"] = project
		data.Service["service-1"].Logs[0] = "updated log"
	})

	if got := uiMessageText(previous.Project["project-1"].Session["session-1"].Thread["thread-1"].Messages[0]); got != "initial" {
		t.Fatalf("previous project message text = %q, want initial", got)
	}
	if got := previous.Service["service-1"].Logs[0]; got != "initial log" {
		t.Fatalf("previous service log = %q, want initial log", got)
	}
	if got := uiMessageText(server.data.Project["project-1"].Session["session-1"].Thread["thread-1"].Messages[0]); got != "updated" {
		t.Fatalf("current project message text = %q, want updated", got)
	}
	if got := server.data.Service["service-1"].Logs[0]; got != "updated log" {
		t.Fatalf("current service log = %q, want updated log", got)
	}
}

func TestSaveShellUsesCopyMutateAssign(t *testing.T) {
	view := state.DefaultView()
	state.EnsureSessionPanelState(&view).ExpandedSessionIDs = map[string]bool{"session-1": true}
	server := &Server{
		data:        testImmutableData(),
		view:        view,
		subscribers: map[chan struct{}]struct{}{},
	}
	previousData := server.data
	previousView := server.view

	server.SaveShell(func(data *state.Data, view *state.View) {
		data.Service["service-1"].Logs[0] = "updated log"
		state.EnsureSessionPanelState(view).ExpandedSessionIDs["session-1"] = false
	})

	if got := previousData.Service["service-1"].Logs[0]; got != "initial log" {
		t.Fatalf("previous service log = %q, want initial log", got)
	}
	if !previousView.GlobalPanelLayout.SessionSidebar.State.ExpandedSessionIDs["session-1"] {
		t.Fatalf("previous view expanded state was mutated")
	}
	if got := server.data.Service["service-1"].Logs[0]; got != "updated log" {
		t.Fatalf("current service log = %q, want updated log", got)
	}
	if server.view.GlobalPanelLayout.SessionSidebar.State.ExpandedSessionIDs["session-1"] {
		t.Fatalf("current view expanded state = true, want false")
	}
}

func testImmutableData() state.Data {
	message := serverapi.Message{
		ID:   "message-1",
		Role: "assistant",
		Parts: []agentmessage.UIPart{
			agentmessage.UITextPart{Type: "text", Text: "initial", State: "done"},
		},
	}
	return state.Data{
		Services: []serverapi.Service{{ID: new("service-1")}},
		Service:  map[string]state.ServiceData{"service-1": {Logs: []string{"initial log"}}},
		Project: map[string]state.ProjectData{
			"project-1": {
				Project: serverapi.Project{ID: "project-1"},
				Session: map[string]state.SessionData{
					"session-1": {
						Session: serverapi.Session{ID: "session-1"},
						Thread: map[string]state.ThreadData{
							"thread-1": {
								Thread:   serverapi.Thread{ID: "thread-1"},
								Messages: []serverapi.Message{message},
							},
						},
					},
				},
			},
		},
	}
}

func uiMessageText(message serverapi.Message) string {
	if len(message.Parts) == 0 {
		return ""
	}
	part, ok := message.Parts[0].(agentmessage.UITextPart)
	if !ok {
		return ""
	}
	return part.Text
}
