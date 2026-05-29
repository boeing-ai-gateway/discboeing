package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// SessionCommit accepts the commit button command for a session row.
func (h *Handler) SessionCommit(w http.ResponseWriter, r *http.Request) {
	if chi.URLParam(r, "id") == "" {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}
	writeNoContent(w)
}
