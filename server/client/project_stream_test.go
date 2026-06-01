package client

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	serverapi "github.com/obot-platform/discobot/server/api"
)

func TestProjectSubscriptionTracksSubscribedThreads(t *testing.T) {
	sub := &ProjectSubscription{
		ctx:     context.Background(),
		events:  make(chan ProjectStreamEvent, 1),
		threads: map[ProjectThreadSubscription]serverapi.ChatStreamSubscriptionOptions{},
	}

	if err := sub.SubscribeThread(context.Background(), "session-b", "thread-b"); err != nil {
		t.Fatalf("subscribe thread: %v", err)
	}
	if err := sub.SubscribeThread(context.Background(), "session-a", "thread-a"); err != nil {
		t.Fatalf("subscribe thread: %v", err)
	}

	got := sub.SubscribedThreads()
	want := []ProjectThreadSubscription{
		{SessionID: "session-a", ThreadID: "thread-a"},
		{SessionID: "session-b", ThreadID: "thread-b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscribed threads = %#v, want %#v", got, want)
	}

	if err := sub.UnsubscribeThread(context.Background(), "session-a", "thread-a"); err != nil {
		t.Fatalf("unsubscribe thread: %v", err)
	}
	got = sub.SubscribedThreads()
	want = []ProjectThreadSubscription{{SessionID: "session-b", ThreadID: "thread-b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("subscribed threads after unsubscribe = %#v, want %#v", got, want)
	}
}

func TestProjectStreamSocketMessageMarshalAddsType(t *testing.T) {
	data, err := json.Marshal(ProjectStreamSocketMessageJSON{
		Message: serverapi.ProjectStreamSubscribedEvent{
			Stream: serverapi.ProjectStreamTypeChat,
		},
	})
	if err != nil {
		t.Fatalf("marshal project stream message: %v", err)
	}

	var got struct {
		Type   string                      `json:"type"`
		Stream serverapi.ProjectStreamType `json:"stream"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal project stream message: %v", err)
	}
	if got.Type != "subscribed" {
		t.Fatalf("type = %q, want %q", got.Type, "subscribed")
	}
	if got.Stream != serverapi.ProjectStreamTypeChat {
		t.Fatalf("stream = %q, want %q", got.Stream, serverapi.ProjectStreamTypeChat)
	}
}
