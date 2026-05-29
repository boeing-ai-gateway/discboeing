// Package command owns server-side UI command handlers.
package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// ViewStore persists server-owned view state and publishes updates to streams.
type ViewStore interface {
	SaveView(func(*state.View))
	SaveData(func(*state.Data))
	SaveShell(func(*state.Data, *state.View))
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
	r.Post("/sidebar/hide", h.SidebarHide)
	r.Post("/terminal/toggle", h.TerminalToggle)
	r.Post("/settings/open", h.SettingsOpen)
	r.Post("/settings/close", h.SettingsClose)
	r.Post("/settings/support/open", h.SettingsSupportOpen)
	r.Post("/settings/support/close", h.SettingsSupportClose)
	r.Post("/settings/cache/confirm", h.SettingsCacheConfirm)
	r.Post("/settings/cache/cancel", h.SettingsCacheCancel)
	r.Post("/settings/cache/clear", h.SettingsCacheClear)
	r.Post("/settings/tabs/{tab}", h.SettingsTabSelect)
	r.Post("/panels/{id}/toggle", h.PanelToggle)
	r.Post("/panels/{id}/maximize", h.PanelMaximize)
	r.Post("/panels/{id}/restore", h.PanelRestore)
	r.Post("/layout/resize", h.LayoutResize)
	r.Post("/sessions/{id}/select", h.SessionSelect)
	r.Post("/sessions/{id}/toggle-expanded", h.SessionToggleExpanded)
	r.Post("/sessions/{id}/detail-sections/{section}/show", h.SessionDetailSectionShow)
	r.Post("/sessions/{id}/detail-sections/{section}/hide", h.SessionDetailSectionHide)
	r.Post("/sessions/{id}/view-mode/{mode}", h.SessionSetViewMode)
	r.Post("/sessions/{id}/side-chats/{threadID}/select", h.SessionSideChatSelect)
	r.Post("/sessions/{id}/diff-summary", h.SessionDiffSummary)
	r.Post("/services/{id}/start", h.ServiceStart)
	r.Post("/services/{id}/stop", h.ServiceStop)
	r.Post("/services/{id}/logs", h.ServiceLogs)
	r.Post("/files/root/move", h.FileMoveToRoot)
	r.Post("/files/search", h.FileSearch)
	r.Post("/files/search/toggle", h.FileSearchToggle)
	r.Post("/files/sessions/{sessionID}/expand-all", h.FileExpandAll)
	r.Post("/files/sessions/{sessionID}/collapse-all", h.FileCollapseAll)
	r.Post("/files/{id}/toggle-expanded", h.FileToggleExpanded)
	r.Post("/files/{id}/select", h.FileSelect)
	r.Post("/files/{id}/approval/toggle", h.FileApprovalToggle)
	r.Post("/files/{id}/delete", h.FileDelete)
	r.Post("/files/{id}/rename", h.FileRename)
	r.Post("/files/{id}/children/file", h.FileCreateFile)
	r.Post("/files/{id}/children/directory", h.FileCreateDirectory)
	r.Post("/files/{id}/move", h.FileMove)
	r.Post("/files/{id}/drop", h.FileDrop)
	r.Post("/sessions/menu-checks/{key}/toggle", h.SessionMenuCheckToggle)
	return r
}
