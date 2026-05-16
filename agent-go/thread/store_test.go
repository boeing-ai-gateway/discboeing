package thread

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
)

func TestSaveAndLoadMessage(t *testing.T) {
	store := NewStore(t.TempDir())

	msg := StoredMessage{
		ID:       "msg1",
		ParentID: "",
		Message: message.Message{
			Role: "user",
			Parts: []message.Part{
				message.TextPart{Text: "hello"},
			},
		},
	}

	if err := store.SaveMessage("thread1", msg); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadMessage("thread1", "msg1")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.ID != "msg1" {
		t.Errorf("expected ID=msg1, got %s", loaded.ID)
	}
	if loaded.ParentID != "" {
		t.Errorf("expected empty parentID, got %s", loaded.ParentID)
	}
	if loaded.Message.Role != "user" {
		t.Errorf("expected role=user, got %s", loaded.Message.Role)
	}
	if len(loaded.Message.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(loaded.Message.Parts))
	}
	tp, ok := loaded.Message.Parts[0].(message.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", loaded.Message.Parts[0])
	}
	if tp.Text != "hello" {
		t.Errorf("expected text=hello, got %s", tp.Text)
	}
	if loaded.Message.CreatedAt == nil {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestLoadMessage_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.LoadMessage("thread1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent message")
	}
}

func TestSaveMessage_DuplicateIDFails(t *testing.T) {
	store := NewStore(t.TempDir())
	msg := StoredMessage{
		ID:      "msg1",
		Message: message.Message{Role: "user"},
	}
	if err := store.SaveMessage("thread1", msg); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage("thread1", msg); !errors.Is(err, ErrMessageExists) {
		t.Fatalf("expected ErrMessageExists, got %v", err)
	}
}

func TestBuildHistory(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	// Create a chain: msg1 -> msg2 -> msg3
	messages := []StoredMessage{
		{
			ID:       "msg1",
			ParentID: "",
			Message:  message.Message{Role: "system", Parts: []message.Part{message.TextPart{Text: "system prompt"}}},
		},
		{
			ID:       "msg2",
			ParentID: "msg1",
			Message:  message.Message{Role: "user", Parts: []message.Part{message.TextPart{Text: "hi"}}},
		},
		{
			ID:       "msg3",
			ParentID: "msg2",
			Message:  message.Message{Role: "assistant", Parts: []message.Part{message.TextPart{Text: "hello"}}},
		},
	}

	for _, msg := range messages {
		if err := store.SaveMessage(threadID, msg); err != nil {
			t.Fatal(err)
		}
	}

	history, err := store.BuildHistory(threadID, "msg3")
	if err != nil {
		t.Fatal(err)
	}

	if len(history) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(history))
	}

	// Verify chronological order: system, user, assistant
	expectedRoles := []string{"system", "user", "assistant"}
	for i, role := range expectedRoles {
		if history[i].Role != role {
			t.Errorf("history[%d]: expected role=%s, got %s", i, role, history[i].Role)
		}
	}
}

func TestBuildHistory_SingleMessage(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	if err := store.SaveMessage(threadID, StoredMessage{
		ID:      "msg1",
		Message: message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}

	history, err := store.BuildHistory(threadID, "msg1")
	if err != nil {
		t.Fatal(err)
	}

	if len(history) != 1 {
		t.Fatalf("expected 1 message, got %d", len(history))
	}
}

func TestBuildHistory_BrokenChain(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	// msg2 points to msg1 which doesn't exist
	if err := store.SaveMessage(threadID, StoredMessage{
		ID:       "msg2",
		ParentID: "msg1",
		Message:  message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := store.BuildHistory(threadID, "msg2")
	if err == nil {
		t.Error("expected error for broken parent chain")
	}
}

func TestBuildHistory_IgnoresCorruptAncestor(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	for _, msg := range []StoredMessage{
		{
			ID:      "msg1",
			Message: message.Message{Role: "system", Parts: []message.Part{message.TextPart{Text: "system"}}},
		},
		{
			ID:       "msg2",
			ParentID: "msg1",
			Message:  message.Message{Role: "user", Parts: []message.Part{message.TextPart{Text: "hello"}}},
		},
		{
			ID:       "msg3",
			ParentID: "msg2",
			Message:  message.Message{Role: "assistant", Parts: []message.Part{message.TextPart{Text: "world"}}},
		},
	} {
		if err := store.SaveMessage(threadID, msg); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(store.messagesDir(threadID), "msg2.json")
	if err := os.WriteFile(path, []byte(`{"id":"msg2"`), 0o644); err != nil {
		t.Fatal(err)
	}

	history, err := store.BuildHistory(threadID, "msg3")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 message after skipping corrupt ancestor, got %d", len(history))
	}
	if history[0].Role != "assistant" {
		t.Fatalf("expected assistant message, got %s", history[0].Role)
	}
}

func TestBuildHistoryWithIDs_IgnoresCorruptAncestor(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	for _, msg := range []StoredMessage{
		{
			ID:      "msg1",
			Message: message.Message{Role: "system"},
		},
		{
			ID:       "msg2",
			ParentID: "msg1",
			Message:  message.Message{Role: "user"},
		},
		{
			ID:       "msg3",
			ParentID: "msg2",
			Message:  message.Message{Role: "assistant"},
		},
	} {
		if err := store.SaveMessage(threadID, msg); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(store.messagesDir(threadID), "msg2.json")
	if err := os.WriteFile(path, []byte(`{"id":"msg2"`), 0o644); err != nil {
		t.Fatal(err)
	}

	history, err := store.BuildHistoryWithIDs(threadID, "msg3")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 entry after skipping corrupt ancestor, got %d", len(history))
	}
	if history[0].ID != "msg3" {
		t.Fatalf("expected msg3, got %s", history[0].ID)
	}
}

func TestFindLeaf_IgnoresCorruptMessage(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	if err := store.SaveMessage(threadID, StoredMessage{
		ID:      "msg1",
		Message: message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.messagesDir(threadID), "msg2.json"), []byte(`{"id":"msg2"`), 0o644); err != nil {
		t.Fatal(err)
	}

	leaf, err := store.FindLeaf(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if leaf != "msg1" {
		t.Fatalf("expected msg1 leaf, got %s", leaf)
	}
}

func TestListThreads(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// No threads yet
	threads, err := store.ListThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Errorf("expected 0 threads, got %d", len(threads))
	}

	// Create two threads by saving messages
	if err := store.SaveMessage("thread-a", StoredMessage{
		ID:      "m1",
		Message: message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage("thread-b", StoredMessage{
		ID:      "m1",
		Message: message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}

	threads, err = store.ListThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d", len(threads))
	}

	// Check both exist (order not guaranteed)
	found := map[string]bool{}
	for _, id := range threads {
		found[id] = true
	}
	if !found["thread-a"] || !found["thread-b"] {
		t.Errorf("expected thread-a and thread-b, got %v", threads)
	}
}

func TestListThreads_EmptyBaseDir(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nonexistent"))
	threads, err := store.ListThreads()
	if err != nil {
		t.Fatal(err)
	}
	if len(threads) != 0 {
		t.Errorf("expected 0 threads, got %d", len(threads))
	}
}

func TestCreateStepFileAndAppendChunk(t *testing.T) {
	store := NewStore(t.TempDir())

	f, err := store.CreateStepFile("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	chunks := []message.ProviderMessageChunk{
		message.TextStartChunk{ID: "t1"},
		message.TextDeltaChunk{ID: "t1", Delta: "hello"},
		message.TextEndChunk{ID: "t1"},
	}

	for _, chunk := range chunks {
		if err := store.AppendChunk(f, chunk); err != nil {
			t.Fatal(err)
		}
	}

	f.Close()

	// Read back and verify
	readF, err := os.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer readF.Close()

	scanner := bufio.NewScanner(readF)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Verify each line is valid JSON with a type field
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
		if _, ok := m["type"]; !ok {
			t.Fatalf("line %d: missing type field", i)
		}
	}

	// Verify round-trip via UnmarshalProviderChunk
	chunk0, err := message.UnmarshalProviderChunk([]byte(lines[0]))
	if err != nil {
		t.Fatal(err)
	}
	ts, ok := chunk0.(message.TextStartChunk)
	if !ok {
		t.Fatalf("expected TextStartChunk, got %T", chunk0)
	}
	if ts.ID != "t1" {
		t.Errorf("expected ID=t1, got %s", ts.ID)
	}
}

func TestHistoryTurnIDs(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.SaveMessage("thread1", StoredMessage{
		ID:      "user-a",
		Message: message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage("thread1", StoredMessage{
		ID:       "assistant-a",
		ParentID: "user-a",
		Message:  message.Message{Role: "assistant"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMessage("thread1", StoredMessage{
		ID:       "tool-user-a",
		ParentID: "assistant-a",
		Message:  message.Message{Role: "user"},
	}); err != nil {
		t.Fatal(err)
	}
	f, err := store.CreateStepFile("thread1", "turn-a", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepResult("thread1", "turn-a", 0, StepResult{
		AssistantMessageID: "assistant-a",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepEventMessages("thread1", "turn-a", 0, StepEventMessages{
		MessageIDs: []string{"tool-user-a"},
	}); err != nil {
		t.Fatal(err)
	}

	turnIDs, err := store.HistoryTurnIDs("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if turnIDs["user-a"] != "turn-a" {
		t.Fatalf("expected user-a to map to turn-a, got %q", turnIDs["user-a"])
	}
	if turnIDs["assistant-a"] != "turn-a" {
		t.Fatalf("expected assistant-a to map to turn-a, got %q", turnIDs["assistant-a"])
	}
	if turnIDs["tool-user-a"] != "turn-a" {
		t.Fatalf("expected tool-user-a to map to turn-a, got %q", turnIDs["tool-user-a"])
	}
}

func TestLoadStepChunks(t *testing.T) {
	store := NewStore(t.TempDir())

	f, err := store.CreateStepFile("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}

	chunks := []message.ProviderMessageChunk{
		message.TextStartChunk{ID: "t1"},
		message.TextDeltaChunk{ID: "t1", Delta: "hello"},
		message.TextEndChunk{ID: "t1"},
	}
	for _, chunk := range chunks {
		if err := store.AppendChunk(f, chunk); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()

	loaded, err := store.LoadStepChunks("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(loaded))
	}
	ts, ok := loaded[0].(message.TextStartChunk)
	if !ok {
		t.Fatalf("expected TextStartChunk, got %T", loaded[0])
	}
	if ts.ID != "t1" {
		t.Errorf("expected ID=t1, got %s", ts.ID)
	}
}

func TestLoadStepChunks_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	chunks, err := store.LoadStepChunks("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if chunks != nil {
		t.Errorf("expected nil for nonexistent step file, got %v", chunks)
	}
}

func TestLoadStepChunks_PartialLastRecord(t *testing.T) {
	store := NewStore(t.TempDir())

	f, err := store.CreateStepFile("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Write two valid chunks.
	validChunks := []message.ProviderMessageChunk{
		message.TextStartChunk{ID: "t1"},
		message.TextDeltaChunk{ID: "t1", Delta: "hello"},
	}
	for _, chunk := range validChunks {
		if err := store.AppendChunk(f, chunk); err != nil {
			t.Fatal(err)
		}
	}

	// Simulate a crash mid-write: append a partial (invalid) JSON line.
	if _, err := f.WriteString(`{"type":"text-delta","id":"t1","delta":"wor`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	loaded, err := store.LoadStepChunks("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	// The partial last record must be skipped; only the two valid chunks returned.
	if len(loaded) != 2 {
		t.Fatalf("expected 2 chunks (partial skipped), got %d", len(loaded))
	}
}

// --- Turn State Tests ---

func TestSaveAndLoadTurnState(t *testing.T) {
	store := NewStore(t.TempDir())

	startedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	state := TurnState{
		ID:          "turn1",
		ThreadID:    "thread1",
		LeafID:      "msg0",
		CurrentStep: 0,
		Phase:       PhaseStreaming,
		StartedAt:   &startedAt,
		Config: TurnConfig{
			Model:     "test-model",
			Reasoning: "enabled",
		},
	}

	if err := store.SaveTurnState("thread1", state); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadTurnState("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil turn state")
	}
	if loaded.ID != "turn1" {
		t.Errorf("expected ID=turn1, got %s", loaded.ID)
	}
	if loaded.Phase != PhaseStreaming {
		t.Errorf("expected phase=streaming, got %s", loaded.Phase)
	}
	if loaded.Config.Model != "test-model" {
		t.Errorf("expected model=test-model, got %s", loaded.Config.Model)
	}
	if loaded.StartedAt == nil || !loaded.StartedAt.Equal(startedAt) {
		t.Fatalf("expected startedAt=%v, got %v", startedAt, loaded.StartedAt)
	}
	if loaded.UpdatedAt == nil {
		t.Fatal("expected updatedAt to be set")
	}

	record, err := store.LoadTurnRecord("thread1", "turn1")
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatal("expected non-nil turn record")
	}
	if record.StartedAt == nil || !record.StartedAt.Equal(startedAt) {
		t.Fatalf("expected turn record startedAt=%v, got %v", startedAt, record.StartedAt)
	}
}

func TestLoadTurnState_IgnoresCorruptActiveTurnState(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"

	if err := store.SaveTurnState(threadID, TurnState{ID: "turn1", ThreadID: threadID, Phase: PhaseStreaming}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(store.turnStatePath(threadID), []byte(`{"id":"turn1"`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadTurnState(threadID)
	if err != nil {
		t.Fatalf("expected corrupt active turn state to be ignored, got %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil turn state after corrupt file, got %#v", loaded)
	}

	record, err := store.LoadTurnRecord(threadID, "turn1")
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatal("expected durable turn record to remain readable")
	}
	if record.ID != "turn1" {
		t.Fatalf("expected durable turn record ID turn1, got %s", record.ID)
	}
}

func TestLoadTurnState_NoActiveTurn(t *testing.T) {
	store := NewStore(t.TempDir())
	state, err := store.LoadTurnState("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if state != nil {
		t.Error("expected nil for no active turn")
	}
}

func TestDeleteTurnState(t *testing.T) {
	store := NewStore(t.TempDir())
	state := TurnState{ID: "turn1", ThreadID: "thread1", Phase: "streaming"}
	if err := store.SaveTurnState("thread1", state); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteTurnState("thread1"); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadTurnState("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Error("expected nil after delete")
	}
}

func TestDeleteTurnState_NoFile(t *testing.T) {
	store := NewStore(t.TempDir())
	// Should not error if file doesn't exist.
	if err := store.DeleteTurnState("thread1"); err != nil {
		t.Fatal(err)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	store := NewStore(t.TempDir())

	cfg := Config{
		Name:          "Friendly thread name",
		NameSource:    ThreadNameSourceGenerated,
		LastMessage:   "latest prompt",
		ErrorMessage:  "invalid model",
		LastTurnState: StateCancelled,
		Model:         "anthropic/claude-sonnet-4-6",
		ServiceTier:   "priority",
		CWD:           "/tmp/project",
		ActiveLeafID:  "msg-active",
		ActiveCommand: "discobot-commit",
		CommunicatedSkillLikeEntries: []CommunicatedSkillLikeEntry{{
			Name:        "commit",
			Description: "Commit pending changes",
		}},
	}

	if err := store.SaveConfig("thread1", cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadConfig("thread1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("expected model=%q, got %q", cfg.Model, loaded.Model)
	}
	if loaded.ServiceTier != cfg.ServiceTier {
		t.Errorf("expected serviceTier=%q, got %q", cfg.ServiceTier, loaded.ServiceTier)
	}
	if loaded.Name != cfg.Name {
		t.Errorf("expected name=%q, got %q", cfg.Name, loaded.Name)
	}
	if loaded.NameSource != cfg.NameSource {
		t.Errorf("expected nameSource=%q, got %q", cfg.NameSource, loaded.NameSource)
	}
	if loaded.LastMessage != cfg.LastMessage {
		t.Errorf("expected lastMessage=%q, got %q", cfg.LastMessage, loaded.LastMessage)
	}
	if loaded.ErrorMessage != cfg.ErrorMessage {
		t.Errorf("expected errorMessage=%q, got %q", cfg.ErrorMessage, loaded.ErrorMessage)
	}
	if loaded.LastTurnState != cfg.LastTurnState {
		t.Errorf("expected lastTurnState=%q, got %q", cfg.LastTurnState, loaded.LastTurnState)
	}
	if loaded.CWD != cfg.CWD {
		t.Errorf("expected cwd=%q, got %q", cfg.CWD, loaded.CWD)
	}
	if loaded.ActiveLeafID != cfg.ActiveLeafID {
		t.Errorf("expected activeLeafId=%q, got %q", cfg.ActiveLeafID, loaded.ActiveLeafID)
	}
	if loaded.ActiveCommand != cfg.ActiveCommand {
		t.Errorf("expected activeCommand=%q, got %q", cfg.ActiveCommand, loaded.ActiveCommand)
	}
	if len(loaded.CommunicatedSkillLikeEntries) != 1 || loaded.CommunicatedSkillLikeEntries[0] != cfg.CommunicatedSkillLikeEntries[0] {
		t.Errorf("expected communicatedSkillLikeEntries=%#v, got %#v", cfg.CommunicatedSkillLikeEntries, loaded.CommunicatedSkillLikeEntries)
	}
}

func TestThreadInfoFromConfigFallsBackToTurnServiceTier(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"
	assistantID := "assistant1"

	if err := store.SaveMessage(threadID, StoredMessage{
		ID:       assistantID,
		ParentID: "user1",
		Message:  message.Message{Role: "assistant"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTurnState(threadID, TurnState{
		ID:     "turn1",
		Config: TurnConfig{ServiceTier: "priority"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepResult(threadID, "turn1", 0, StepResult{
		AssistantMessageID: assistantID,
	}); err != nil {
		t.Fatal(err)
	}

	info := store.ThreadInfoFromConfig(threadID, Config{
		Model:        "codex/gpt-5.5",
		ActiveLeafID: assistantID,
	})
	if info.ServiceTier != "priority" {
		t.Fatalf("expected service tier from turn history, got %q", info.ServiceTier)
	}
}

// --- Step Result Tests ---

func TestSaveAndLoadStepResult(t *testing.T) {
	store := NewStore(t.TempDir())

	result := StepResult{
		AssistantMessage: message.Message{
			Role: "assistant",
			Parts: []message.Part{
				message.TextPart{Text: "hello"},
				message.ToolCallPart{ToolCallID: "tc1", ToolName: "bash", Input: `{"cmd":"ls"}`},
			},
		},
		ToolCalls: []ToolCallInfo{
			{ToolCallID: "tc1", ToolName: "bash", Input: `{"cmd":"ls"}`},
		},
	}

	// Need to create the turn dir first.
	f, err := store.CreateStepFile("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := store.SaveStepResult("thread1", "turn1", 0, result); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadStepResult("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil step result")
	}
	if loaded.AssistantMessage.Role != "assistant" {
		t.Errorf("expected role=assistant, got %s", loaded.AssistantMessage.Role)
	}
	if len(loaded.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(loaded.ToolCalls))
	}
	if loaded.ToolCalls[0].ToolCallID != "tc1" {
		t.Errorf("expected tool call ID=tc1, got %s", loaded.ToolCalls[0].ToolCallID)
	}
}

func TestLoadStepResult_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	result, err := store.LoadStepResult("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent step result")
	}
}

// --- Tool Results Tests ---

func TestSaveAndLoadToolResults(t *testing.T) {
	store := NewStore(t.TempDir())

	// Create turn dir.
	f, err := store.CreateStepFile("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	results := StepToolResults{
		Results: []message.ToolResultPart{
			{
				ToolCallID: "tc1",
				ToolName:   "bash",
				Output:     message.TextOutput{Value: "file1.txt\nfile2.txt"},
			},
			{
				ToolCallID: "tc2",
				ToolName:   "read_file",
				Output:     message.ErrorTextOutput{Value: "not found"},
			},
		},
	}

	if err := store.SaveToolResults("thread1", "turn1", 0, results); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadToolResults("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(loaded.Results))
	}
	if loaded.Results[0].ToolCallID != "tc1" {
		t.Errorf("expected tool call ID=tc1, got %s", loaded.Results[0].ToolCallID)
	}
	if loaded.Results[0].ToolName != "bash" {
		t.Errorf("expected tool name=bash, got %s", loaded.Results[0].ToolName)
	}
	textOut, ok := loaded.Results[0].Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput, got %T", loaded.Results[0].Output)
	}
	if textOut.Value != "file1.txt\nfile2.txt" {
		t.Errorf("unexpected output: %s", textOut.Value)
	}
	errOut, ok := loaded.Results[1].Output.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput, got %T", loaded.Results[1].Output)
	}
	if errOut.Value != "not found" {
		t.Errorf("unexpected error output: %s", errOut.Value)
	}
}

func TestLoadToolResults_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	results, err := store.LoadToolResults("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) != 0 {
		t.Errorf("expected empty results, got %d", len(results.Results))
	}
}

func TestSaveAndLoadStepEventMessages(t *testing.T) {
	store := NewStore(t.TempDir())
	events := StepEventMessages{MessageIDs: []string{"m1", "m2"}}
	if err := store.SaveStepEventMessages("thread1", "turn1", 0, events); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadStepEventMessages("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.MessageIDs) != 2 || loaded.MessageIDs[0] != "m1" || loaded.MessageIDs[1] != "m2" {
		t.Fatalf("unexpected step event messages: %+v", loaded.MessageIDs)
	}
}

func TestLoadStepEventMessages_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	events, err := store.LoadStepEventMessages("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events.MessageIDs) != 0 {
		t.Fatalf("expected no step event messages, got %v", events.MessageIDs)
	}
}

func TestStepResultPersistsUsage(t *testing.T) {
	store := NewStore(t.TempDir())
	usage := message.Usage{
		InputTokens: message.InputTokens{
			Total:      100,
			NoCache:    70,
			CacheRead:  20,
			CacheWrite: 10,
		},
		OutputTokens: message.OutputTokens{
			Total:     30,
			Text:      25,
			Reasoning: 5,
		},
	}

	if err := store.SaveTurnState("thread1", TurnState{ID: "turn1", ThreadID: "thread1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepResult("thread1", "turn1", 0, StepResult{
		AssistantMessage: message.Message{Role: "assistant"},
		Usage:            usage,
	}); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadStepResult("thread1", "turn1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected step result")
	}
	if loaded.Usage != usage {
		t.Fatalf("expected usage %+v, got %+v", usage, loaded.Usage)
	}
}

func TestAggregateTurnUsage(t *testing.T) {
	store := NewStore(t.TempDir())
	prices := message.TokenPrices{Input: 3, Output: 15}
	cfg := TurnConfig{ContextWindow: 200000, MaxOutputTokens: 16000, TokenPrices: prices}
	if err := store.SaveTurnState("thread1", TurnState{ID: "turn1", ThreadID: "thread1", Config: cfg}); err != nil {
		t.Fatal(err)
	}
	first := message.Usage{
		InputTokens:  message.InputTokens{Total: 100, NoCache: 80, CacheRead: 20},
		OutputTokens: message.OutputTokens{Total: 40, Text: 30, Reasoning: 10},
	}
	second := message.Usage{
		InputTokens:  message.InputTokens{Total: 60, NoCache: 40, CacheWrite: 20},
		OutputTokens: message.OutputTokens{Total: 20, Text: 20},
	}
	if err := store.SaveStepResult("thread1", "turn1", 0, StepResult{AssistantMessage: message.Message{Role: "assistant"}, Usage: first}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepResult("thread1", "turn1", 1, StepResult{AssistantMessage: message.Message{Role: "assistant"}, Usage: second}); err != nil {
		t.Fatal(err)
	}

	summary, err := aggregateTurnUsage(store, "thread1", "turn1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total.InputTokens.Total != 160 || summary.Total.OutputTokens.Total != 60 {
		t.Fatalf("unexpected total usage: %+v", summary.Total)
	}
	if summary.Total.InputTokens.CacheRead != 20 || summary.Total.InputTokens.CacheWrite != 20 {
		t.Fatalf("unexpected input breakdown: %+v", summary.Total.InputTokens)
	}
	if summary.Total.OutputTokens.Reasoning != 10 || summary.Total.OutputTokens.Text != 50 {
		t.Fatalf("unexpected output breakdown: %+v", summary.Total.OutputTokens)
	}
	if summary.LastStep != second {
		t.Fatalf("expected last step %+v, got %+v", second, summary.LastStep)
	}
	if summary.ModelMaxTokens != 200000 || summary.MaxOutputTokens != 16000 {
		t.Fatalf("unexpected model limits: %+v", summary)
	}
	if summary.Prices != prices {
		t.Fatalf("expected prices %+v, got %+v", prices, summary.Prices)
	}
}

func TestAggregateTurnUsageCostEstimate(t *testing.T) {
	store := NewStore(t.TempDir())
	prices := message.TokenPrices{Input: 2.5, Output: 10}
	cfg := TurnConfig{ContextWindow: 200000, MaxOutputTokens: 16000, TokenPrices: prices}
	if err := store.SaveTurnState("thread1", TurnState{ID: "turn1", ThreadID: "thread1", Config: cfg}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepResult("thread1", "turn1", 0, StepResult{
		AssistantMessage: message.Message{Role: "assistant"},
		Usage: message.Usage{
			InputTokens:  message.InputTokens{Total: 1_000_000},
			OutputTokens: message.OutputTokens{Total: 200_000},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStepResult("thread1", "turn1", 1, StepResult{
		AssistantMessage: message.Message{Role: "assistant"},
		Usage: message.Usage{
			InputTokens:  message.InputTokens{Total: 500_000},
			OutputTokens: message.OutputTokens{Total: 300_000},
		},
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := aggregateTurnUsage(store, "thread1", "turn1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Prices != prices {
		t.Fatalf("expected prices %+v, got %+v", prices, summary.Prices)
	}
	if summary.Total.InputTokens.Total != 1_500_000 || summary.Total.OutputTokens.Total != 500_000 {
		t.Fatalf("unexpected aggregate usage: %+v", summary.Total)
	}
	if got, want := estimateTokenCostForTest(summary.Total, summary.Prices), 8.75; got != want {
		t.Fatalf("expected cost %.2f, got %.2f", want, got)
	}
}

func TestCompleteTurnUpdatesThreadUsage(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"
	prices := message.TokenPrices{Input: 3, Output: 15}
	cfg := TurnConfig{ContextWindow: 200000, MaxOutputTokens: 16000, TokenPrices: prices}
	if err := store.SaveConfig(threadID, Config{Model: "provider/model"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTurnState(threadID, TurnState{
		ID:       "turn0",
		ThreadID: threadID,
		TokenUsage: TokenUsageInfo{
			Total: message.Usage{
				InputTokens:  message.InputTokens{Total: 10},
				OutputTokens: message.OutputTokens{Total: 5},
			},
			ModelMaxTokens: 1000,
		},
	}); err != nil {
		t.Fatal(err)
	}

	turnState := TurnState{
		ID:          "turn1",
		ThreadID:    threadID,
		Config:      cfg,
		CurrentStep: 0,
	}
	if err := store.SaveTurnState(threadID, turnState); err != nil {
		t.Fatal(err)
	}
	lastTurnUsage := message.Usage{
		InputTokens:  message.InputTokens{Total: 100, NoCache: 100},
		OutputTokens: message.OutputTokens{Total: 25, Text: 25},
	}
	if err := store.SaveStepResult(threadID, "turn1", 0, StepResult{AssistantMessage: message.Message{Role: "assistant"}, Usage: lastTurnUsage}); err != nil {
		t.Fatal(err)
	}

	if err := completeTurn(store, threadID, &turnState); err != nil {
		t.Fatal(err)
	}
	info, err := store.GetThreadInfo(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if info.TokenUsage.LastTurn != lastTurnUsage {
		t.Fatalf("expected last turn %+v, got %+v", lastTurnUsage, info.TokenUsage.LastTurn)
	}
	if info.TokenUsage.LastStep != lastTurnUsage {
		t.Fatalf("expected last step %+v, got %+v", lastTurnUsage, info.TokenUsage.LastStep)
	}
	if info.TokenUsage.Total.InputTokens.Total != 110 || info.TokenUsage.Total.OutputTokens.Total != 30 {
		t.Fatalf("unexpected thread total: %+v", info.TokenUsage.Total)
	}
	if info.TokenUsage.ModelMaxTokens != 200000 || info.TokenUsage.MaxOutputTokens != 16000 {
		t.Fatalf("unexpected model limits: %+v", info.TokenUsage)
	}
	if info.TokenUsage.Prices != prices {
		t.Fatalf("expected prices %+v, got %+v", prices, info.TokenUsage.Prices)
	}
}

func TestAggregateThreadUsageCostEstimate(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread1"
	prices := message.TokenPrices{Input: 1.25, Output: 5}
	if err := store.SaveTurnState(threadID, TurnState{
		ID:       "turn0",
		ThreadID: threadID,
		TokenUsage: TokenUsageInfo{
			Total: message.Usage{
				InputTokens:  message.InputTokens{Total: 2_000_000},
				OutputTokens: message.OutputTokens{Total: 1_000_000},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	current := TurnState{
		ID:          "turn1",
		ThreadID:    threadID,
		CurrentStep: 0,
		Config: TurnConfig{
			ContextWindow:   200000,
			MaxOutputTokens: 16000,
			TokenPrices:     prices,
		},
	}
	if err := store.SaveTurnState(threadID, current); err != nil {
		t.Fatal(err)
	}
	currentUsage := message.Usage{
		InputTokens:  message.InputTokens{Total: 1_000_000},
		OutputTokens: message.OutputTokens{Total: 500_000},
	}
	if err := store.SaveStepResult(threadID, current.ID, 0, StepResult{
		AssistantMessage: message.Message{Role: "assistant"},
		Usage:            currentUsage,
	}); err != nil {
		t.Fatal(err)
	}
	tokenUsage, err := aggregateTurnUsage(store, threadID, current.ID, current.CurrentStep, current.Config)
	if err != nil {
		t.Fatal(err)
	}
	current.TokenUsage = tokenUsage

	summary, err := aggregateThreadUsage(store, threadID, &current)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Prices != prices {
		t.Fatalf("expected prices %+v, got %+v", prices, summary.Prices)
	}
	if summary.LastTurn != currentUsage {
		t.Fatalf("expected last turn %+v, got %+v", currentUsage, summary.LastTurn)
	}
	if summary.Total.InputTokens.Total != 3_000_000 || summary.Total.OutputTokens.Total != 1_500_000 {
		t.Fatalf("unexpected aggregate usage: %+v", summary.Total)
	}
	if got, want := estimateTokenCostForTest(summary.LastTurn, summary.Prices), 3.75; got != want {
		t.Fatalf("expected last turn cost %.2f, got %.2f", want, got)
	}
	if got, want := estimateTokenCostForTest(summary.Total, summary.Prices), 11.25; got != want {
		t.Fatalf("expected total cost %.2f, got %.2f", want, got)
	}
}

func estimateTokenCostForTest(usage message.Usage, prices message.TokenPrices) float64 {
	return (float64(usage.InputTokens.Total)*prices.Input + float64(usage.OutputTokens.Total)*prices.Output) / 1_000_000
}

func TestStepFileNaming(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	f, err := store.CreateStepFile("thread1", "turn1", 5)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	expected := filepath.Join(dir, "thread1", "turns", "turn1", "step-005.jsonl")
	if f.Name() != expected {
		t.Errorf("expected path %s, got %s", expected, f.Name())
	}
}
