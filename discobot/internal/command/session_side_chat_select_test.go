package command

import (
	"testing"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestSessionSideChatExists(t *testing.T) {
	sessions := []state.Session{
		{
			ID: "s",
			SideChats: []state.Thread{
				{ID: "thread-a"},
			},
		},
	}

	if !sessionSideChatExists(sessions, "s", "thread-a") {
		t.Fatalf("thread-a should exist")
	}
	if sessionSideChatExists(sessions, "s", "missing") {
		t.Fatalf("missing thread should not exist")
	}
	if sessionSideChatExists(sessions, "other", "thread-a") {
		t.Fatalf("thread in another session should not exist")
	}
}
