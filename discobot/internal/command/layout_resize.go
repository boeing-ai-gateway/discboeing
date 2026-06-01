package command

import (
	"encoding/json"
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

type layoutResizePayload struct {
	Key     string `json:"key"`
	PanelID string `json:"panelId"`
	Axis    string `json:"axis"`
	Size    int    `json:"size"`
}

// LayoutResize updates a server-owned panel or legacy layout size.
func (h *Handler) LayoutResize(w http.ResponseWriter, r *http.Request) {
	var payload layoutResizePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid resize payload", http.StatusBadRequest)
		return
	}

	size, ok := layoutResizeSize(payload, payload.Size)
	if !ok {
		http.Error(w, "invalid resize target", http.StatusBadRequest)
		return
	}

	h.view.SaveView(func(view *state.View) {
		if payload.PanelID != "" {
			savePanelSize(view, payload.PanelID, payload.Axis, size)
			return
		}
		saveLegacyLayoutSize(view, payload.Key, size)
	})
	writeNoContent(w)
}

func layoutResizeSize(payload layoutResizePayload, size int) (int, bool) {
	if payload.PanelID != "" {
		panel, ok := state.DefaultPanelFrames()[payload.PanelID]
		if !ok {
			return 0, false
		}
		switch payload.Axis {
		case "x":
			return clampInt(size, panel.MinWidth, panel.MaxWidth), true
		case "y":
			return clampInt(size, panel.MinHeight, panel.MaxHeight), true
		default:
			return 0, false
		}
	}

	switch payload.Key {
	case "sessions-sidebar-width":
		return clampInt(size, 240, 520), true
	case "composer-side-pane-width":
		return clampInt(size, 320, 620), true
	case "composer-prompt-height":
		return clampInt(size, 58, 240), true
	case "terminal-height":
		return clampInt(size, 180, 480), true
	default:
		return 0, false
	}
}

func savePanelSize(view *state.View, panelID string, axis string, size int) {
	panel := state.EnsurePanel(view, panelID)
	switch axis {
	case "x":
		panel.Width = size
	case "y":
		panel.Height = size
	}
	state.SavePanel(view, panelID, panel)
}

func saveLegacyLayoutSize(view *state.View, key string, size int) {
	switch key {
	case "sessions-sidebar-width":
		panel := state.EnsurePanel(view, "session")
		panel.Width = size
		state.SavePanel(view, "session", panel)
	case "composer-side-pane-width":
		panel := state.EnsurePanel(view, "composer")
		panel.Width = size
		state.SavePanel(view, "composer", panel)
	case "composer-prompt-height":
		composer := state.EnsureComposerPanelState(view)
		composer.PromptHeight = size
	case "terminal-height":
		panel := state.EnsurePanel(view, "terminal")
		panel.Height = size
		state.SavePanel(view, "terminal", panel)
	}
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
