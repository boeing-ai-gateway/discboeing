package handler

import "net/http"

// ControlSocket accepts the server-initiated sandbox control WebSocket.
func (h *Handler) ControlSocket(w http.ResponseWriter, r *http.Request) {
	if h.controlSocket == nil {
		http.Error(w, "control socket is not configured", http.StatusNotFound)
		return
	}
	h.controlSocket.ServeWebSocket(w, r)
}
