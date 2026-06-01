package command

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// SettingsOpen opens the application settings dialog.
func (h *Handler) SettingsOpen(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.Open = true
		view.Settings.Tab = state.NormalizeSettingsTab(view.Settings.Tab)
	})
	writeNoContent(w)
}

// SettingsClose closes the application settings dialog.
func (h *Handler) SettingsClose(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.Open = false
		view.Settings.SupportInfoOpen = false
		view.Settings.ClearCacheDialogOpen = false
		view.Settings.Tab = state.NormalizeSettingsTab(view.Settings.Tab)
	})
	writeNoContent(w)
}

// SettingsSupportOpen opens the support information dialog.
func (h *Handler) SettingsSupportOpen(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.Open = true
		view.Settings.SupportInfoOpen = true
		view.Settings.ClearCacheDialogOpen = false
	})
	writeNoContent(w)
}

// SettingsSupportClose closes the support information dialog.
func (h *Handler) SettingsSupportClose(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.SupportInfoOpen = false
	})
	writeNoContent(w)
}

// SettingsCacheConfirm opens the clear project cache confirmation dialog.
func (h *Handler) SettingsCacheConfirm(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.Open = true
		view.Settings.SupportInfoOpen = false
		view.Settings.ClearCacheDialogOpen = true
		view.Settings.CacheCleared = false
	})
	writeNoContent(w)
}

// SettingsCacheCancel closes the clear project cache confirmation dialog.
func (h *Handler) SettingsCacheCancel(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.ClearCacheDialogOpen = false
	})
	writeNoContent(w)
}

// SettingsCacheClear marks the prototype project cache clear flow as completed.
func (h *Handler) SettingsCacheClear(w http.ResponseWriter, r *http.Request) {
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.ClearCacheDialogOpen = false
		view.Settings.CacheCleared = true
	})
	writeNoContent(w)
}

// SettingsTabSelect switches the visible settings dialog tab.
func (h *Handler) SettingsTabSelect(w http.ResponseWriter, r *http.Request) {
	tab := state.NormalizeSettingsTab(state.SettingsTab(chi.URLParam(r, "tab")))
	h.view.SaveView(r.Context(), func(view *state.View) {
		view.Settings.Open = true
		view.Settings.Tab = tab
	})
	writeNoContent(w)
}
