package command

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestSidebarHidePreservesSessionState(t *testing.T) {
	view := state.DefaultView()
	sessionPanel := state.EnsureSessionPanelState(&view)
	sessionPanel.ExpandedSessionIDs = map[string]bool{"s": true}
	sessionPanel.VisibleSessionDetailSections = map[string]bool{
		state.SessionDetailSectionKey("s", state.SessionDetailSectionHooks): true,
	}

	store := &testCommandViewStore{view: view}
	handler := New(store).Routes()

	request := httptest.NewRequest(http.MethodPost, "/sidebar/hide", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}

	panel := state.EnsurePanel(&store.view, "session")
	if panel.Visible {
		t.Fatalf("session panel should be hidden")
	}

	sessionState := state.EnsureSessionPanelState(&store.view)
	if !sessionState.ExpandedSessionIDs["s"] {
		t.Fatalf("expanded session state should be preserved")
	}
	if !sessionState.VisibleSessionDetailSections[state.SessionDetailSectionKey("s", state.SessionDetailSectionHooks)] {
		t.Fatalf("visible detail section state should be preserved")
	}
}
