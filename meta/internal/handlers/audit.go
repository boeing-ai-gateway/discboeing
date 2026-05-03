package handlers

import "net/http"

func (h *Handlers) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("listAuditEvents", w, r)
}
