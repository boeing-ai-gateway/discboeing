package local

import (
	"context"
	"net/http"
	"testing"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

func TestAcquireHTTPClientCachesPerSessionPort(t *testing.T) {
	provider := &Provider{
		processes: map[string]*processInfo{
			"session-1": {port: 3002},
		},
		httpClients: sandbox.NewHTTPClientCache(),
	}

	lease1, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1")
	if err != nil {
		t.Fatalf("AcquireHTTPClient() error = %v", err)
	}
	defer lease1.Release()

	lease2, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1")
	if err != nil {
		t.Fatalf("AcquireHTTPClient() second call error = %v", err)
	}
	defer lease2.Release()

	if lease1.Client != lease2.Client {
		t.Fatalf("AcquireHTTPClient() returned different clients for the same session port")
	}

	transport, ok := lease1.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport = %T, want *http.Transport", lease1.Client.Transport)
	}
	if transport.DisableKeepAlives {
		t.Fatalf("transport DisableKeepAlives = true, want false")
	}
}

func TestAcquireHTTPClientRefreshesWhenPortChanges(t *testing.T) {
	provider := &Provider{
		processes: map[string]*processInfo{
			"session-1": {port: 3002},
		},
		httpClients: sandbox.NewHTTPClientCache(),
	}

	lease1, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1")
	if err != nil {
		t.Fatalf("AcquireHTTPClient() error = %v", err)
	}

	provider.processes["session-1"].port = 3003

	lease2, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1")
	if err != nil {
		t.Fatalf("AcquireHTTPClient() after port change error = %v", err)
	}
	defer lease2.Release()

	if lease1.Client == lease2.Client {
		t.Fatalf("AcquireHTTPClient() reused a client after the session port changed")
	}

	lease1.Release()
}

func TestAcquireHTTPClientInvalidatesWhenSandboxStops(t *testing.T) {
	provider := &Provider{
		processes: map[string]*processInfo{
			"session-1": {port: 3002, status: sandbox.StatusRunning},
		},
		httpClients: sandbox.NewHTTPClientCache(),
	}

	lease1, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1")
	if err != nil {
		t.Fatalf("AcquireHTTPClient() error = %v", err)
	}

	provider.processes["session-1"].port = 0
	if _, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1"); err == nil {
		t.Fatalf("AcquireHTTPClient() after stop error = nil, want error")
	}

	provider.processes["session-1"].port = 3004
	lease2, err := provider.AcquireHTTPClient(context.Background(), nil, "session-1")
	if err != nil {
		t.Fatalf("AcquireHTTPClient() after restart error = %v", err)
	}
	defer lease2.Release()

	if lease1.Client == lease2.Client {
		t.Fatalf("AcquireHTTPClient() reused a stale client after the sandbox restarted")
	}

	lease1.Release()
}
