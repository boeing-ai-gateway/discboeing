package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func TestSessionStoreCreatesCookieSession(t *testing.T) {
	store := newSessionStore(t.TempDir(), slog.Default())
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	id := store.sessionID(response, request)
	if !validSessionID(id) {
		t.Fatalf("session id %q is invalid", id)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	if cookies[0].Name != sessionCookieName || cookies[0].Value != id {
		t.Fatalf("cookie = %s:%s, want session cookie %s", cookies[0].Name, cookies[0].Value, id)
	}
}

func TestSessionStoreRestoresPersistedView(t *testing.T) {
	dir := t.TempDir()
	const id = "0123456789abcdef0123456789abcdef"
	store := newSessionStore(dir, slog.Default())

	store.save(id, func(view *state.View) {
		panel := state.EnsurePanel(view, "session")
		panel.Width = 333
		state.SavePanel(view, "session", panel)
	})
	store.persist(id)

	restarted := newSessionStore(dir, slog.Default())
	view := restarted.view(id)
	panel := state.EnsurePanel(&view, "session")
	if panel.Width != 333 {
		t.Fatalf("restored session width = %d, want 333", panel.Width)
	}
}
