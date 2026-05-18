package command

import (
	"log/slog"
	"net/http"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/internal/live"
	uisession "github.com/obot-platform/discobot/ui-go/internal/session"
	"github.com/obot-platform/discobot/ui-go/internal/state"
)

// Handler owns server-side command routes triggered by ui-go data hooks.
type Handler struct {
	store  *state.Store
	live   *live.Store
	client *api.Client
	logger *slog.Logger
}

// New returns a command handler bound to store.
func New(store *state.Store, liveStore *live.Store, client *api.Client, logger *slog.Logger) *Handler {
	return &Handler{
		store:  store,
		live:   liveStore,
		client: client,
		logger: logger,
	}
}

func (h *Handler) session(r *http.Request) (*state.Session, bool) {
	id, ok := uisession.ID(r.Context())
	if !ok {
		return nil, false
	}
	return h.store.Session(id), true
}
