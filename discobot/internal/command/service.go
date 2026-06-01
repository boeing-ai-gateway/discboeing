package command

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
	serverapi "github.com/obot-platform/discobot/server/api"
)

// ServiceStart marks a prototype service as running.
func (h *Handler) ServiceStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h.view.SaveData(func(data *state.Data) {
		service := serviceByID(data.Services, id)
		if service == nil {
			return
		}
		service.Status = new(string(state.ServiceStatusRunning))
		appendServiceLog(data, id, serviceLogLine(service, "started"))
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
		service.Status = new(string(state.ServiceStatusStopped))
		appendServiceLog(data, id, serviceLogLine(service, "stopped"))
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
		state.SavePanel(view, "editor", editorPanel)
	})
	writeNoContent(w)
}

func serviceByID(services []serverapi.Service, id string) *serverapi.Service {
	for index := range services {
		if serviceID(services[index]) == id {
			return &services[index]
		}
	}
	return nil
}

func appendServiceLog(data *state.Data, serviceID string, line string) {
	if data.Service == nil {
		data.Service = map[string]state.ServiceData{}
	}
	serviceData := data.Service[serviceID]
	serviceData.Logs = append(serviceData.Logs, line)
	data.Service[serviceID] = serviceData
}

func serviceLogLine(service *serverapi.Service, event string) string {
	return time.Now().Format("15:04:05") + " [" + serviceID(*service) + "] " + event
}

func serviceID(service serverapi.Service) string {
	if service.ID == nil {
		return ""
	}
	return *service.ID
}
