package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

// ListMessages handles GET /threads/{id}/messages — returns thread history as
// UI-projected messages.
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")
	if strings.TrimSpace(threadID) == "" {
		h.Error(w, http.StatusBadRequest, "id is required")
		return
	}

	messages, err := h.conversations.Messages(threadID, strings.TrimSpace(r.URL.Query().Get("leafId")))
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []message.UIMessage{}
	}

	h.JSON(w, http.StatusOK, api.ListMessagesResponse{Messages: messages})
}
