package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/agent-go/thread"
)

func TestStoreAppendAndloadBrowserEvents(t *testing.T) {
	store := NewStore(t.TempDir())

	if err := store.appendBrowserEvent("thread1", "turn1", 2, thread.BrowserEvent{
		RequestID: "7",
		Method:    "Page.navigate",
		Direction: "request",
		Payload:   json.RawMessage(`{"id":7}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.appendBrowserEvent("thread1", "turn1", 2, thread.BrowserEvent{
		RequestID: "7",
		Direction: "response",
		Payload:   json.RawMessage(`{"id":7,"result":{}}`),
	}); err != nil {
		t.Fatal(err)
	}

	events, err := store.loadBrowserEvents("thread1", "turn1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Direction != "request" || events[1].Direction != "response" {
		t.Fatalf("unexpected events %#v", events)
	}
}

func TestStoresaveBrowserScreenshot(t *testing.T) {
	store := NewStore(t.TempDir())

	fileRef, err := store.saveBrowserScreenshot("thread1", "turn1", 2, "event-1", []byte("\x89PNG\r\n\x1a\ntest"))
	if err != nil {
		t.Fatal(err)
	}
	if fileRef.Path != "artifacts/browser/sha256/7428e98493880d020400580e2ce7e49cb21b3c55d4dd09d6c401c54e1d7d0817.png" {
		t.Fatalf("unexpected screenshot path %q", fileRef.Path)
	}
	if fileRef.URI != "artifacts://artifacts/browser/sha256/7428e98493880d020400580e2ce7e49cb21b3c55d4dd09d6c401c54e1d7d0817.png" {
		t.Fatalf("unexpected screenshot uri %q", fileRef.URI)
	}
	if fileRef.MediaType != "image/png" {
		t.Fatalf("unexpected screenshot media type %q", fileRef.MediaType)
	}
	data, err := os.ReadFile(filepath.Join(store.threadDir("thread1"), filepath.FromSlash(fileRef.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\x89PNG\r\n\x1a\ntest" {
		t.Fatalf("unexpected screenshot bytes %q", string(data))
	}
}

func TestStoresaveBrowserScreenshotDeduplicatesByContent(t *testing.T) {
	store := NewStore(t.TempDir())
	png := []byte("\x89PNG\r\n\x1a\nsame")

	firstRef, err := store.saveBrowserScreenshot("thread1", "turn1", 2, "event-1", png)
	if err != nil {
		t.Fatal(err)
	}
	secondRef, err := store.saveBrowserScreenshot("thread1", "turn1", 3, "event-2", png)
	if err != nil {
		t.Fatal(err)
	}
	if firstRef.Path != secondRef.Path {
		t.Fatalf("expected duplicate screenshots to share path, got %q and %q", firstRef.Path, secondRef.Path)
	}
	if firstRef.URI != secondRef.URI {
		t.Fatalf("expected duplicate screenshots to share URI, got %q and %q", firstRef.URI, secondRef.URI)
	}

	matches, err := filepath.Glob(filepath.Join(store.threadDir("thread1"), "artifacts", "browser", "sha256", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 stored screenshot, got %d", len(matches))
	}
}

func TestStoreloadAllBrowserEventEntries(t *testing.T) {
	baseDir := t.TempDir()
	store := NewStore(baseDir)
	threadStore := thread.NewStore(baseDir)
	f, err := threadStore.CreateStepFile("thread1", "turn-a", 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := threadStore.SaveStepResult("thread1", "turn-a", 0, thread.StepResult{
		AssistantMessageID: "assistant-a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.appendBrowserEvent("thread1", "turn-a", 0, thread.BrowserEvent{
		EventID:   "browser-1",
		Method:    "Page.navigate",
		Direction: "request",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.appendBrowserEvent("thread1", "turn-a", 0, thread.BrowserEvent{
		EventID:   "browser-2",
		Method:    "Page.navigate",
		Direction: "response",
	}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.loadAllBrowserEventEntries("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 browser event entries, got %d", len(entries))
	}
	if entries[0].TurnID != "turn-a" {
		t.Fatalf("expected turn-a, got %q", entries[0].TurnID)
	}
	if entries[0].AssistantMessageID != "assistant-a" {
		t.Fatalf("expected assistant-a, got %q", entries[0].AssistantMessageID)
	}
	if entries[0].Event.EventID != "browser-1" || entries[1].Event.EventID != "browser-2" {
		t.Fatalf("unexpected browser event ids: %#v", entries)
	}
}
