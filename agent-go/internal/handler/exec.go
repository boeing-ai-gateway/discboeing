package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/internal/processes"
)

type execResizeRequest struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

type execListResponse struct {
	Sessions []processes.Session `json:"sessions"`
}

type execOutputResponse struct {
	Events []processes.OutputEvent `json:"events"`
}

// ExecCapabilities handles GET /exec/capabilities.
func (h *Handler) ExecCapabilities(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, h.processManager.Capabilities())
}

// CreateExec handles POST /exec.
func (h *Handler) CreateExec(w http.ResponseWriter, r *http.Request) {
	var req processes.CreateRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Kind == "" {
		req.Kind = processes.KindUserExec
	}
	session, err := h.processManager.Start(r.Context(), req)
	if err != nil {
		h.processError(w, err)
		return
	}
	h.JSON(w, http.StatusCreated, session)
}

// ListExec handles GET /exec.
func (h *Handler) ListExec(w http.ResponseWriter, r *http.Request) {
	h.JSON(w, http.StatusOK, execListResponse{
		Sessions: h.processManager.List(processes.Kind(r.URL.Query().Get("kind"))),
	})
}

// GetExec handles GET /exec/{id}.
func (h *Handler) GetExec(w http.ResponseWriter, r *http.Request) {
	session, err := h.processManager.Get(chi.URLParam(r, "id"))
	if err != nil {
		h.processError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, session)
}

// ExecOutput handles GET /exec/{id}/output.
func (h *Handler) ExecOutput(w http.ResponseWriter, r *http.Request) {
	h.ExecEvents(w, r)
}

// ExecEvents handles GET /exec/{id}/events.
func (h *Handler) ExecEvents(w http.ResponseWriter, r *http.Request) {
	query, follow, err := parseExecEventQuery(r)
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if follow {
		h.followExecEvents(w, r, query)
		return
	}
	events, err := h.processManager.Events(chi.URLParam(r, "id"), query)
	if err != nil {
		h.processError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, execOutputResponse{Events: events})
}

func (h *Handler) followExecEvents(w http.ResponseWriter, r *http.Request, query processes.EventQuery) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	id := chi.URLParam(r, "id")
	live, unsubscribe, done, err := h.processManager.Subscribe(id)
	if err != nil {
		h.processError(w, err)
		return
	}
	defer unsubscribe()

	events, err := h.processManager.Events(id, query)
	if err != nil {
		h.processError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	var lastSeq int64
	for _, event := range events {
		if event.Seq > lastSeq {
			lastSeq = event.Seq
		}
		if !writeExecSSEEvent(w, flusher, event) {
			return
		}
	}

	for {
		select {
		case event, ok := <-live:
			if !ok {
				return
			}
			if event.Seq <= lastSeq || !execEventMatches(event, query) {
				continue
			}
			lastSeq = event.Seq
			if !writeExecSSEEvent(w, flusher, event) {
				return
			}
		case <-done:
			return
		case <-r.Context().Done():
			return
		}
	}
}

// ResizeExec handles POST /exec/{id}/resize.
func (h *Handler) ResizeExec(w http.ResponseWriter, r *http.Request) {
	var req execResizeRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Rows <= 0 || req.Cols <= 0 {
		h.Error(w, http.StatusBadRequest, "rows and cols must be greater than zero")
		return
	}
	if err := h.processManager.Resize(r.Context(), chi.URLParam(r, "id"), req.Rows, req.Cols); err != nil {
		h.processError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// KillExec handles POST /exec/{id}/kill.
func (h *Handler) KillExec(w http.ResponseWriter, r *http.Request) {
	if err := h.processManager.Kill(chi.URLParam(r, "id")); err != nil {
		h.processError(w, err)
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"status": "killed"})
}

// DeleteExec handles DELETE /exec/{id}.
func (h *Handler) DeleteExec(w http.ResponseWriter, r *http.Request) {
	h.KillExec(w, r)
}

// AttachExec handles GET /exec/{id}/attach.
func (h *Handler) AttachExec(w http.ResponseWriter, r *http.Request) {
	stream, events, unsubscribe, err := h.processManager.Attach(chi.URLParam(r, "id"))
	if err != nil {
		h.processError(w, err)
		return
	}
	defer unsubscribe()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1", "localhost"},
	})
	if err != nil {
		log.Printf("exec attach accept failed: %v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "exec attach closing")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	errCh := make(chan error, 2)
	go h.execWebSocketInput(ctx, conn, stream, errCh)
	go h.execWebSocketOutput(ctx, conn, events, errCh)

	if err := <-errCh; err != nil && !errors.Is(err, io.EOF) {
		log.Printf("exec attach closed with error: %v", err)
	}
	cancel()
	_ = conn.Close(websocket.StatusNormalClosure, "done")
}

func (h *Handler) execWebSocketInput(ctx context.Context, conn *websocket.Conn, stream processes.Stream, errCh chan<- error) {
	for {
		msgType, payload, err := conn.Read(ctx)
		if err != nil {
			errCh <- err
			return
		}
		if msgType != websocket.MessageText && msgType != websocket.MessageBinary {
			continue
		}
		if len(payload) == 0 {
			continue
		}
		if _, err := stream.Write(payload); err != nil {
			errCh <- err
			return
		}
	}
}

func (h *Handler) execWebSocketOutput(ctx context.Context, conn *websocket.Conn, events <-chan processes.OutputEvent, errCh chan<- error) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				errCh <- nil
				return
			}
			if event.Data == "" {
				continue
			}
			if err := conn.Write(ctx, websocket.MessageBinary, []byte(event.Data)); err != nil {
				errCh <- err
				return
			}
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		}
	}
}

func parseExecEventQuery(r *http.Request) (processes.EventQuery, bool, error) {
	values := r.URL.Query()
	var query processes.EventQuery
	if raw := values.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 0 {
			return query, false, fmt.Errorf("limit must be a non-negative integer")
		}
		query.Limit = limit
	}
	if raw := values.Get("after"); raw != "" {
		after, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 {
			return query, false, fmt.Errorf("after must be a non-negative integer")
		}
		query.After = &after
	} else if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		after, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && after >= 0 {
			query.After = &after
		}
	}
	if raw := values.Get("since"); raw != "" {
		since, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return query, false, fmt.Errorf("since must be an RFC3339 timestamp")
		}
		query.Since = &since
	}
	follow := false
	if raw := values.Get("follow"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return query, false, fmt.Errorf("follow must be a boolean")
		}
		follow = parsed
	}
	return query, follow, nil
}

func execEventMatches(event processes.OutputEvent, query processes.EventQuery) bool {
	if query.After != nil && event.Seq <= *query.After {
		return false
	}
	if query.Since != nil && !event.Timestamp.After(*query.Since) {
		return false
	}
	return true
}

func writeExecSSEEvent(w http.ResponseWriter, flusher http.Flusher, event processes.OutputEvent) bool {
	data, err := json.Marshal(event)
	if err != nil {
		return true
	}
	writeSSEEvent(w, strconv.FormatInt(event.Seq, 10), event.Type, data)
	flusher.Flush()
	return true
}

func (h *Handler) processError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, processes.ErrNotFound):
		h.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, processes.ErrTTYUnsupported), errors.Is(err, processes.ErrUserSwitchUnsupported):
		h.Error(w, http.StatusBadRequest, err.Error())
	default:
		h.Error(w, http.StatusInternalServerError, err.Error())
	}
}
