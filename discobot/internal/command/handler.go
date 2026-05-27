// Package command owns server-side UI command handlers.
package command

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// ViewStore persists server-owned view state and publishes updates to streams.
type ViewStore interface {
	SaveView(func(*state.View)) uint64
	SaveData(func(*state.Data)) uint64
	SaveShell(func(*state.Data, *state.View)) uint64
}

// Handler owns server-side command routes triggered by Datastar data hooks.
type Handler struct {
	view ViewStore
}

// New returns a command handler bound to view state.
func New(view ViewStore) *Handler {
	return &Handler{
		view: view,
	}
}

// Routes returns the command route tree.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/sidebar/toggle", h.SidebarToggle)
	r.Post("/terminal/toggle", h.TerminalToggle)
	r.Post("/sessions/{id}/select", h.SessionSelect)
	r.Post("/sessions/{id}/toggle-expanded", h.SessionToggleExpanded)
	r.Post("/files/root/move", h.FileMoveToRoot)
	r.Post("/files/search", h.FileSearch)
	r.Post("/files/sessions/{sessionID}/expand-all", h.FileExpandAll)
	r.Post("/files/sessions/{sessionID}/collapse-all", h.FileCollapseAll)
	r.Post("/files/{id}/toggle-expanded", h.FileToggleExpanded)
	r.Post("/files/{id}/select", h.FileSelect)
	r.Post("/files/{id}/delete", h.FileDelete)
	r.Post("/files/{id}/rename", h.FileRename)
	r.Post("/files/{id}/children/file", h.FileCreateFile)
	r.Post("/files/{id}/children/directory", h.FileCreateDirectory)
	r.Post("/files/{id}/move", h.FileMove)
	r.Post("/files/{id}/drop", h.FileDrop)
	r.Post("/sessions/menu-checks/{key}/toggle", h.SessionMenuCheckToggle)
	return r
}

func writeGeneration(w http.ResponseWriter, generation uint64) {
	w.Header().Set("X-Discobot-UI-Generation", strconv.FormatUint(generation, 10))
	w.WriteHeader(http.StatusNoContent)
}
