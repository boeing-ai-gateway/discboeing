package handler

import (
	"net/http"
	"strings"

	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
)

// AuthorizeSudo handles POST /sudo/authorize. It is called by the sandbox sudo
// wrapper before the wrapper executes the real sudo binary.
func (h *Handler) AuthorizeSudo(w http.ResponseWriter, r *http.Request) {
	if h.sudoAuthorizer == nil {
		h.JSON(w, http.StatusForbidden, sudoauth.AuthorizeResponse{Allow: false, Reason: "sudo authorization is not configured", Guidance: sudoauth.Guidance})
		return
	}

	var req sudoauth.AuthorizeRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Runtime = strings.TrimSpace(req.Runtime)
	req.Token = strings.TrimSpace(req.Token)
	req.CredentialID = strings.TrimSpace(req.CredentialID)
	req.UseID = strings.TrimSpace(req.UseID)

	resp, err := h.sudoAuthorizer.AuthorizeSudo(r.Context(), req)
	if err != nil {
		h.JSON(w, http.StatusForbidden, sudoauth.AuthorizeResponse{Allow: false, Reason: err.Error(), Guidance: sudoauth.Guidance})
		return
	}
	if !resp.Allow {
		if strings.TrimSpace(resp.Guidance) == "" {
			resp.Guidance = sudoauth.Guidance
		}
		h.JSON(w, http.StatusForbidden, resp)
		return
	}
	h.JSON(w, http.StatusOK, resp)
}
