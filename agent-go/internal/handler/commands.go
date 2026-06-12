package handler

import "net/http"

// ListCommands handles GET /commands.
func (h *Handler) ListCommands(w http.ResponseWriter, _ *http.Request) {
	commands, err := h.commandsSnapshot()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "failed to list commands: "+err.Error())
		return
	}
	h.JSON(w, http.StatusOK, commands)
}
