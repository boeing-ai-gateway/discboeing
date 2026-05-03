package handlers

import "net/http"

func (h *Handlers) CreateAgentSessionTokenConvenience(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("createAgentSessionTokenConvenience", w, r)
}

func (h *Handlers) IntrospectToken(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("introspectToken", w, r)
}
