package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/boeing-ai-gateway/discboeing/agent-go/agent"
	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/services"
)

func TestServiceProxyDoesNotAutoStartStoppedExecutableService(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	servicesDir := filepath.Join(workspaceRoot, services.ServicesDir)
	if err := os.MkdirAll(servicesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() failed: %v", err)
	}

	servicePath := filepath.Join(servicesDir, "preview.sh")
	content := `#!/bin/bash
#---
# name: Preview
# http: 65534
#---
exec sleep 30
`
	if err := os.WriteFile(servicePath, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}

	serviceManager := services.NewManager(workspaceRoot)
	h := New(workspaceRoot, agent.NewConversationManager(&streamTestAgent{}), nil, serviceManager, nil)

	req := httptest.NewRequest(http.MethodGet, "/services/preview/http/health", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("serviceId", "preview")
	req = req.WithContext(contextWithRouteContext(req, rctx))

	rr := httptest.NewRecorder()
	h.ServiceProxy(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
	if serviceManager.IsManaged("preview") {
		t.Fatal("service proxy unexpectedly started the service")
	}
}

func contextWithRouteContext(req *http.Request, rctx *chi.Context) context.Context {
	return context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
}
