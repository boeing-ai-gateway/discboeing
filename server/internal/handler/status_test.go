package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	mocksandbox "github.com/boeing-ai-gateway/discboeing/server/internal/sandbox/mock"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
	"github.com/boeing-ai-gateway/discboeing/server/internal/startup"
)

type statusReportingProvider struct {
	*mocksandbox.Provider
	statusFunc func() sandbox.ProviderStatus
}

func (p *statusReportingProvider) Status() sandbox.ProviderStatus {
	if p.statusFunc != nil {
		return p.statusFunc()
	}
	return sandbox.ProviderStatus{Available: true, State: "ready"}
}

func TestGetSystemStatusRefreshesProviderStatusesFirst(t *testing.T) {
	t.Parallel()

	systemManager := startup.NewSystemManager(nil, "local")
	systemManager.RegisterTask("wsl-start", "Starting managed WSL distro")
	systemManager.FailTask("wsl-start", errors.New("earlier bootstrap failed"))

	providerCalled := false
	sandboxProviderManager := sandbox.NewProviderManager()
	sandboxProviderManager.RegisterProvider("wsl", &statusReportingProvider{
		Provider: mocksandbox.NewProvider(),
		statusFunc: func() sandbox.ProviderStatus {
			providerCalled = true
			systemManager.RegisterTask("wsl-start", "Starting managed WSL distro")
			systemManager.StartTask("wsl-start")
			systemManager.UpdateTaskProgress("wsl-start", 100, "Managed WSL distro and Docker bridge are ready")
			systemManager.CompleteTask("wsl-start")
			return sandbox.ProviderStatus{Available: true, State: "ready"}
		},
	})

	sandboxSvc := service.NewSandboxService(nil, nil, nil, nil, nil, nil, nil)
	sandboxSvc.SetProviderManager(sandboxProviderManager)

	h := &Handler{sandboxService: sandboxSvc, systemManager: systemManager}
	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()

	h.GetSystemStatus(rec, req)

	if !providerCalled {
		t.Fatal("expected GetSystemStatus to refresh provider statuses")
	}

	var response startup.SystemStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for _, task := range response.StartupTasks {
		if task.ID != "wsl-start" {
			continue
		}
		if task.State != startup.TaskStateCompleted {
			t.Fatalf("wsl-start state = %q, want %q", task.State, startup.TaskStateCompleted)
		}
		if task.CurrentOperation != "Managed WSL distro and Docker bridge are ready" {
			t.Fatalf("wsl-start current operation = %q", task.CurrentOperation)
		}
		return
	}

	t.Fatal("expected wsl-start task in system status response")
}
