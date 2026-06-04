package thread

import (
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
)

func TestThreadInfoPhaseCreateAndUpdate(t *testing.T) {
	store := NewStore(t.TempDir())

	info, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID:    "thread-1",
		Phase: " Review ",
	})
	if err != nil {
		t.Fatalf("CreateThreadInfo() failed: %v", err)
	}
	if info.Phase != "review" {
		t.Fatalf("created phase = %q, want review", info.Phase)
	}
	if phase, err := store.loadLegacyThreadConfigPhase("thread-1"); err != nil {
		t.Fatalf("loadLegacyThreadConfigPhase() failed: %v", err)
	} else if phase != "" {
		t.Fatalf("thread config phase = %q, want empty", phase)
	}
	if _, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID: "thread-2",
	}); err != nil {
		t.Fatalf("CreateThreadInfo(thread-2) failed: %v", err)
	}
	info, err = store.GetThreadInfo("thread-2")
	if err != nil {
		t.Fatalf("GetThreadInfo(thread-2) failed: %v", err)
	}
	if info.Phase != "review" {
		t.Fatalf("second thread phase = %q, want review", info.Phase)
	}
	info, err = store.GetThreadInfo("thread-1")
	if err != nil {
		t.Fatalf("GetThreadInfo() failed: %v", err)
	}
	if info.Phase != "review" {
		t.Fatalf("reloaded phase = %q, want review", info.Phase)
	}
	infos, err := store.ListThreadInfos()
	if err != nil {
		t.Fatalf("ListThreadInfos() failed: %v", err)
	}
	if len(infos) != 2 || infos[0].Phase != "review" || infos[1].Phase != "review" {
		t.Fatalf("listed infos = %#v, want phase review", infos)
	}

	emptyPhase := ""
	info, err = store.UpdateThreadInfo("thread-2", UpdateThreadRequest{
		Phase: &emptyPhase,
	})
	if err != nil {
		t.Fatalf("UpdateThreadInfo() failed: %v", err)
	}
	if info.Phase != "" {
		t.Fatalf("updated phase = %q, want empty", info.Phase)
	}
	info, err = store.GetThreadInfo("thread-1")
	if err != nil {
		t.Fatalf("GetThreadInfo(thread-1) failed: %v", err)
	}
	if info.Phase != "" {
		t.Fatalf("first thread phase after session update = %q, want empty", info.Phase)
	}
}

func TestThreadInfoRejectsInvalidPhase(t *testing.T) {
	store := NewStore(t.TempDir())

	if _, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID:    "thread-1",
		Phase: "ship",
	}); err == nil || !strings.Contains(err.Error(), "invalid session phase") {
		t.Fatalf("CreateThreadInfo() error = %v, want invalid session phase", err)
	}

	if _, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID: "thread-2",
	}); err != nil {
		t.Fatalf("CreateThreadInfo(thread-2) failed: %v", err)
	}
	invalid := "ship"
	if _, err := store.UpdateThreadInfo("thread-2", UpdateThreadRequest{
		Phase: &invalid,
	}); err == nil || !strings.Contains(err.Error(), "invalid session phase") {
		t.Fatalf("UpdateThreadInfo() error = %v, want invalid session phase", err)
	}
}

func TestThreadInfoExposesActiveTurnUsage(t *testing.T) {
	store := NewStore(t.TempDir())
	threadID := "thread-1"
	if _, err := store.CreateThreadInfo("/workspace", CreateThreadRequest{
		ID: threadID,
	}); err != nil {
		t.Fatal(err)
	}

	completedUsage := message.Usage{
		InputTokens:  message.InputTokens{Total: 100},
		OutputTokens: message.OutputTokens{Total: 20},
	}
	if err := store.SaveTurnState(threadID, TurnState{
		ID:       "turn-1",
		ThreadID: threadID,
		TokenUsage: TokenUsageInfo{
			Total: completedUsage,
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTurnState(threadID); err != nil {
		t.Fatal(err)
	}

	lastStepUsage := message.Usage{
		InputTokens:  message.InputTokens{Total: 50},
		OutputTokens: message.OutputTokens{Total: 10},
	}
	activeTotalUsage := message.Usage{
		InputTokens:  message.InputTokens{Total: 75},
		OutputTokens: message.OutputTokens{Total: 15},
	}
	prices := message.TokenPrices{Input: 1.25, Output: 5}
	if err := store.SaveTurnState(threadID, TurnState{
		ID:          "turn-2",
		ThreadID:    threadID,
		CurrentStep: 1,
		TokenUsage: TokenUsageInfo{
			Total:           activeTotalUsage,
			LastStep:        lastStepUsage,
			ModelMaxTokens:  200000,
			MaxOutputTokens: 16000,
			Prices:          prices,
		},
	}); err != nil {
		t.Fatal(err)
	}

	info, err := store.GetThreadInfo(threadID)
	if err != nil {
		t.Fatal(err)
	}
	if info.TokenUsage.LastTurn != activeTotalUsage {
		t.Fatalf("expected active last turn %+v, got %+v", activeTotalUsage, info.TokenUsage.LastTurn)
	}
	if info.TokenUsage.LastStep != lastStepUsage {
		t.Fatalf("expected active last step %+v, got %+v", lastStepUsage, info.TokenUsage.LastStep)
	}
	if info.TokenUsage.Total.InputTokens.Total != 175 || info.TokenUsage.Total.OutputTokens.Total != 35 {
		t.Fatalf("unexpected total usage: %+v", info.TokenUsage.Total)
	}
	if info.TokenUsage.ModelMaxTokens != 200000 || info.TokenUsage.MaxOutputTokens != 16000 {
		t.Fatalf("unexpected model limits: %+v", info.TokenUsage)
	}
	if info.TokenUsage.Prices != prices {
		t.Fatalf("expected prices %+v, got %+v", prices, info.TokenUsage.Prices)
	}
}
