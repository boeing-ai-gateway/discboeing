package command

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

type testCommandViewStore struct {
	data state.Data
	view state.View
}

func (store *testCommandViewStore) SaveView(_ context.Context, update func(*state.View)) {
	update(&store.view)
}

func (store *testCommandViewStore) SaveData(_ context.Context, update func(*state.Data)) {
	update(&store.data)
}

func (store *testCommandViewStore) SaveShell(_ context.Context, update func(*state.Data, *state.View)) {
	update(&store.data, &store.view)
}

func TestSessionDetailSectionShowAndHide(t *testing.T) {
	store := &testCommandViewStore{view: state.DefaultView()}
	handler := New(store).Routes()

	request := httptest.NewRequest(http.MethodPost, "/sessions/s/detail-sections/hooks/show", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("show status = %d, want %d", response.Code, http.StatusNoContent)
	}

	sessionState := state.EnsureSessionPanelState(&store.view)
	key := state.SessionDetailSectionKey("s", state.SessionDetailSectionHooks)
	if !sessionState.VisibleSessionDetailSections[key] {
		t.Fatalf("hooks section should be visible after show")
	}
	if !sessionState.ExpandedSessionIDs["s"] {
		t.Fatalf("session should be expanded after show")
	}

	request = httptest.NewRequest(http.MethodPost, "/sessions/s/detail-sections/hooks/hide", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("hide status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if sessionState.VisibleSessionDetailSections[key] {
		t.Fatalf("hooks section should be hidden after hide")
	}
}

func TestSessionDetailSectionInvalidSection(t *testing.T) {
	store := &testCommandViewStore{view: state.DefaultView()}
	handler := New(store).Routes()

	request := httptest.NewRequest(http.MethodPost, "/sessions/s/detail-sections/nope/show", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "invalid session detail section") {
		t.Fatalf("body = %q, want invalid section error", response.Body.String())
	}
}
