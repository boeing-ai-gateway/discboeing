package handler

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/server/internal/middleware"
	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/sandboxapi"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
)

// ============================================================================
// Service Endpoints
// ============================================================================

// ListServices lists all services in the session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/services
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	result, err := h.chatService.ListServices(ctx, projectID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}
	h.applyLocalhostServiceBinds(sessionID, result)

	h.JSON(w, http.StatusOK, result)
}

// StartService starts a service in the session's sandbox.
// POST /api/projects/{projectId}/sessions/{sessionId}/services/{serviceId}/start
func (h *Handler) StartService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	serviceID := chi.URLParam(r, "serviceId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if serviceID == "" {
		h.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}

	result, err := h.chatService.StartService(ctx, projectID, sessionID, serviceID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "service_not_found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "already_running") {
			status = http.StatusConflict
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusAccepted, result)
}

// StopService stops a service in the session's sandbox.
// POST /api/projects/{projectId}/sessions/{sessionId}/services/{serviceId}/stop
func (h *Handler) StopService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	serviceID := chi.URLParam(r, "serviceId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if serviceID == "" {
		h.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}

	result, err := h.chatService.StopService(ctx, projectID, sessionID, serviceID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "service_not_found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "not_running") {
			status = http.StatusBadRequest
		}
		h.Error(w, status, err.Error())
		return
	}
	if h.serviceBindManager != nil {
		h.serviceBindManager.Unbind(sessionID, serviceID)
	}

	h.JSON(w, http.StatusOK, result)
}

type bindServiceLocalhostRequest struct {
	Port *int `json:"port,omitempty"`
}

type bindServiceLocalhostResponse struct {
	Localhost sandboxapi.ServiceLocalhostBind `json:"localhost"`
}

type unbindServiceLocalhostResponse struct {
	Status    string `json:"status"`
	ServiceID string `json:"serviceId"`
}

// BindServiceLocalhost binds a sandbox service target port to a host localhost port.
// POST /api/projects/{projectId}/sessions/{sessionId}/services/{serviceId}/localhost
func (h *Handler) BindServiceLocalhost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	serviceID := chi.URLParam(r, "serviceId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if serviceID == "" {
		h.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}
	if h.serviceBindManager == nil {
		h.Error(w, http.StatusServiceUnavailable, "localhost service binding is unavailable")
		return
	}

	var req bindServiceLocalhostRequest
	if err := h.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	services, err := h.chatService.ListServices(ctx, projectID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	selected, ok := findService(services.Services, serviceID)
	if !ok {
		h.Error(w, http.StatusNotFound, "service not found")
		return
	}
	targetPort, scheme, ok := serviceTargetPort(selected)
	if !ok {
		h.Error(w, http.StatusBadRequest, "service does not expose an HTTP or HTTPS port")
		return
	}

	port := targetPort
	if req.Port != nil {
		port = *req.Port
	}
	bind, err := h.serviceBindManager.Bind(sessionID, serviceID, port, targetPort, scheme)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidLocalhostPort):
			h.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrLocalhostPortInUse):
			h.Error(w, http.StatusConflict, err.Error())
		default:
			h.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.JSON(w, http.StatusOK, bindServiceLocalhostResponse{Localhost: *bind})
}

// UnbindServiceLocalhost closes the host localhost listener for a service.
// DELETE /api/projects/{projectId}/sessions/{sessionId}/services/{serviceId}/localhost
func (h *Handler) UnbindServiceLocalhost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	serviceID := chi.URLParam(r, "serviceId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if serviceID == "" {
		h.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}
	if _, err := h.chatService.GetSession(ctx, projectID, sessionID); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	if h.serviceBindManager != nil {
		h.serviceBindManager.Unbind(sessionID, serviceID)
	}
	h.JSON(w, http.StatusOK, unbindServiceLocalhostResponse{
		Status:    "unbound",
		ServiceID: serviceID,
	})
}

// GetServiceOutput streams the output of a service via SSE.
// GET /api/projects/{projectId}/sessions/{sessionId}/services/{serviceId}/output
func (h *Handler) GetServiceOutput(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := h.withShutdownContext(r.Context())
	defer cancel()

	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	serviceID := chi.URLParam(r, "serviceId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if serviceID == "" {
		h.Error(w, http.StatusBadRequest, "serviceId is required")
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get the stream from sandbox
	sseCh, err := h.chatService.GetServiceOutput(ctx, projectID, sessionID, serviceID)
	if err != nil {
		writeServiceSSEError(w, err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	// Pass through raw SSE lines from sandbox
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			log.Printf("[ServiceOutput] Client disconnected, stopping SSE stream")
			return
		case line, ok := <-sseCh:
			if !ok {
				// Channel closed without explicit DONE
				_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}
			if line.Done {
				log.Printf("[ServiceOutput] Received [DONE] signal from sandbox")
				_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}
			// Pass through raw data line without parsing
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line.Data)
			flusher.Flush()
		}
	}
}

// writeServiceSSEError sends an error SSE event followed by the [DONE] signal.
func writeServiceSSEError(w http.ResponseWriter, errorText string) {
	_, _ = fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":\"%s\"}\n\n", errorText)
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *Handler) applyLocalhostServiceBinds(sessionID string, result *sandboxapi.ListServicesResponse) {
	if h.serviceBindManager == nil || result == nil {
		return
	}
	for i := range result.Services {
		result.Services[i].Localhost = h.serviceBindManager.Get(sessionID, result.Services[i].ID)
	}
}

func findService(services []sandboxapi.Service, serviceID string) (sandboxapi.Service, bool) {
	for _, svc := range services {
		if svc.ID == serviceID {
			return svc, true
		}
	}
	return sandboxapi.Service{}, false
}

func serviceTargetPort(svc sandboxapi.Service) (int, string, bool) {
	if svc.HTTPS > 0 {
		return svc.HTTPS, "https", true
	}
	if svc.HTTP > 0 {
		return svc.HTTP, "http", true
	}
	return 0, "", false
}
