package handler

import (
	"log"
	"net/http"

	"github.com/coder/websocket"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/realtime"
)

// ChatWebSocket multiplexes project-scoped realtime streams over a single WebSocket.
//
// Client messages:
//   - {"type":"subscribe","stream":"chat","sessionId":"...","threadId":"...","replay":true,"lastEventId":"..."}
//   - {"type":"unsubscribe","stream":"chat","sessionId":"...","threadId":"..."}
//   - {"type":"subscribe","stream":"service","sessionId":"...","serviceId":"..."}
//   - {"type":"unsubscribe","stream":"service","sessionId":"...","serviceId":"..."}
//   - {"type":"subscribe","stream":"project-events","afterId":"..."}
//   - {"type":"unsubscribe","stream":"project-events"}
//
// Server messages:
//   - {"type":"subscribed",...}
//   - {"type":"event",...}
//   - {"type":"complete",...}
//   - {"type":"error",...}
//   - {"type":"unsubscribed",...}
func (h *Handler) ChatWebSocket(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	if projectID == "" {
		h.Error(w, http.StatusBadRequest, "projectId is required")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("failed to upgrade chat websocket: %v", err)
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	ctx, cancel := h.withShutdownContext(r.Context())
	defer cancel()

	socket := realtime.NewProjectStreamSocket(
		ctx,
		cancel,
		conn,
		projectID,
		h.chatService,
		h.eventBroker,
	)
	socket.Run()
}
