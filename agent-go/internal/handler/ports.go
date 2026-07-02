package handler

import (
	"net/http"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/api"
	"github.com/boeing-ai-gateway/discboeing/agent-go/portwatcher"
)

// ListPorts handles GET /ports — lists TCP listening ports owned by visible processes.
func (h *Handler) ListPorts(w http.ResponseWriter, r *http.Request) {
	ports, err := portwatcher.Scan(r.Context())
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	apiPorts := make([]api.PortEntry, 0, len(ports))
	for _, port := range ports {
		apiPorts = append(apiPorts, api.PortEntry{
			LocalAddress: port.LocalAddress,
			Port:         port.Port,
			Process:      port.Process,
			Protocol:     port.Protocol,
			PID:          port.PID,
			FD:           port.FD,
		})
	}

	h.JSON(w, http.StatusOK, api.ListPortsResponse{Ports: apiPorts})
}
