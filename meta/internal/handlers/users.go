package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/obot-platform/discobot/meta/internal/auth"
)

func (h *Handlers) Whoami(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserInfoFromRequest(r)
	if !ok || user == nil {
		user = auth.AnonymousUserInfo("authentication middleware did not run")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user": user,
	})
}

func (h *Handlers) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("getCurrentUser", w, r)
}

func (h *Handlers) ListCurrentUserSessions(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("listCurrentUserSessions", w, r)
}

func (h *Handlers) RevokeCurrentUserSession(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("revokeCurrentUserSession", w, r)
}
