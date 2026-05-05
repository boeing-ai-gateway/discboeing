package promptqueue

import (
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
)

func TestStoreQueueOperations(t *testing.T) {
	store := NewStore(t.TempDir())

	queue, queued, err := store.Append("thread1", Prompt{
		Message: message.UIMessage{
			ID:    "user-1",
			Role:  "user",
			Parts: []message.UIPart{message.UITextPart{Text: "first"}},
		},
		Model: "anthropic/claude-sonnet-4-6",
	})
	if err != nil {
		t.Fatal(err)
	}
	if queued.ID == "" {
		t.Fatal("expected queued prompt id")
	}
	if len(queue) != 1 {
		t.Fatalf("expected 1 queued prompt, got %d", len(queue))
	}

	queue, removed, err := store.Delete("thread1", queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected queued prompt to be removed")
	}
	if len(queue) != 0 {
		t.Fatalf("expected empty queue, got %d items", len(queue))
	}

	_, first, err := store.Append("thread1", Prompt{
		ID: "queue-1",
		Message: message.UIMessage{
			ID:    "user-1",
			Role:  "user",
			Parts: []message.UIPart{message.UITextPart{Text: "first"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Append("thread1", Prompt{
		ID: "queue-2",
		Message: message.UIMessage{
			ID:    "user-2",
			Role:  "user",
			Parts: []message.UIPart{message.UITextPart{Text: "second"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	queue, popped, err := store.Pop("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if popped == nil || popped.ID != first.ID {
		t.Fatalf("expected to pop %q, got %#v", first.ID, popped)
	}
	if len(queue) != 1 || queue[0].ID != "queue-2" {
		t.Fatalf("expected remaining queue [queue-2], got %#v", queue)
	}

	queue, err = store.Prepend("thread1", Prompt{
		ID: "queue-0",
		Message: message.UIMessage{
			ID:    "user-0",
			Role:  "user",
			Parts: []message.UIPart{message.UITextPart{Text: "zeroth"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(queue) != 2 || queue[0].ID != "queue-0" {
		t.Fatalf("expected queue-0 to be prepended, got %#v", queue)
	}

	future := time.Now().UTC().Add(2 * time.Hour)
	queue, updated, err := store.UpdatePrompt("thread1", "queue-0", Update{RunAfter: &future})
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected queued prompt runAfter to be updated")
	}
	if queue[0].RunAfter.IsZero() {
		t.Fatal("expected queue-0 runAfter to be set")
	}

	queue, popped, err = store.Pop("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if popped == nil || popped.ID != "queue-2" {
		t.Fatalf("expected queue-2 to pop while queue-0 is delayed, got %#v", popped)
	}
	if len(queue) != 1 || queue[0].ID != "queue-0" {
		t.Fatalf("expected delayed queue-0 to remain queued, got %#v", queue)
	}

	queue, updated, err = store.UpdatePrompt("thread1", "queue-0", Update{ClearRunAfter: true})
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected queued prompt runAfter to be cleared")
	}
	if !queue[0].RunAfter.IsZero() {
		t.Fatalf("expected queue-0 runAfter to be cleared, got %#v", queue[0].RunAfter)
	}

	_, popped, err = store.Pop("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if popped == nil || popped.ID != "queue-0" {
		t.Fatalf("expected queue-0 to pop after clearing runAfter, got %#v", popped)
	}
}
