package command

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

// ServiceStart marks a prototype service as running.
func (h *Handler) ServiceStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveData(func(data *state.Data) {
		service := serviceByID(data.Services, id)
		if service == nil {
			return
		}
		service.Status = state.ServiceStatusRunning
		service.Logs = append(service.Logs, serviceLogLine(service, "started"))
	})
	writeNoContent(w)
}

// ServiceStop marks a prototype service as stopped.
func (h *Handler) ServiceStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveData(func(data *state.Data) {
		service := serviceByID(data.Services, id)
		if service == nil {
			return
		}
		service.Status = state.ServiceStatusStopped
		service.Logs = append(service.Logs, serviceLogLine(service, "stopped"))
	})
	writeNoContent(w)
}

// ServiceLogs opens the selected service logs in the editor panel.
func (h *Handler) ServiceLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveShell(func(data *state.Data, view *state.View) {
		if serviceByID(data.Services, id) == nil {
			return
		}
		editorState := state.EnsureEditorPanelState(view)
		editorState.ActiveFileID = ""
		editorState.DiffSummarySessionID = ""
		editorState.ServiceLogID = id

		editorPanel := state.EnsurePanel(view, "editor")
		editorPanel.Visible = true
		view.PanelLayout.Panels["editor"] = editorPanel
	})
	writeNoContent(w)
}

func serviceByID(services []state.Service, id string) *state.Service {
	for index := range services {
		if services[index].ID == id {
			return &services[index]
		}
	}
	return nil
}

func serviceLogLine(service *state.Service, event string) string {
	return time.Now().Format("15:04:05") + " [" + service.ID + "] " + event
}
