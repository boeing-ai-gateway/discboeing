package handler

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
	"github.com/boeing-ai-gateway/discboeing/server/internal/service"
)

// DebugDockerServer runs a standalone HTTP server that proxies Docker API requests
// to the Docker daemon inside a VZ VM. This allows using standard Docker CLI:
//
//	DOCKER_HOST=tcp://localhost:2375 docker ps
type DebugDockerServer struct {
	server    *http.Server
	projectID string
}

// NewDebugDockerServer creates a new debug Docker proxy server for the given project.
func NewDebugDockerServer(sandboxService *service.SandboxService, projectID string, port int) (*DebugDockerServer, error) {
	if sandboxService == nil {
		return nil, fmt.Errorf("sandbox service is not configured")
	}
	proxyProvider, err := sandboxService.DockerProxyProvider()
	if err != nil {
		return nil, err
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "localhost"
			req.Host = "localhost"
		},
		Transport: &debugDockerTransport{
			provider:  proxyProvider,
			projectID: projectID,
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			log.Printf("Debug Docker proxy error: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
		},
	}

	return &DebugDockerServer{
		projectID: projectID,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: proxy,
		},
	}, nil
}

// Start starts the debug Docker proxy server in the background.
func (s *DebugDockerServer) Start() {
	go func() {
		log.Printf("Debug Docker proxy listening on %s (project: %s)", s.server.Addr, s.projectID)
		log.Printf("  Usage: DOCKER_HOST=tcp://localhost%s docker ps", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Debug Docker proxy error: %v", err)
		}
	}()
}

// Stop stops the debug Docker proxy server.
func (s *DebugDockerServer) Stop() {
	_ = s.server.Close()
}

// debugDockerTransport lazily resolves the Docker transport for the project VM.
// This allows the proxy to start before the VM is ready (e.g., during image download).
type debugDockerTransport struct {
	provider  sandbox.DockerProxyProvider
	projectID string
}

func (t *debugDockerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport, err := t.provider.DockerTransport(t.projectID)
	if err != nil {
		return nil, fmt.Errorf("VM not available: %w", err)
	}
	return transport.RoundTrip(req)
}
