package services

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestConnectionRefusedHTMLReloadsOnceProxyStopsReturningStartupErrors(t *testing.T) {
	html := connectionRefusedHTML(3015)

	if !strings.Contains(html, "res.status !== 502 && res.status !== 503") {
		t.Fatalf("expected startup page to reload once the proxy stops returning 502/503, got:\n%s", html)
	}

	if !strings.Contains(html, "cache: 'no-store'") {
		t.Fatalf("expected startup page retry fetch to bypass caches, got:\n%s", html)
	}
}

func TestProxyHTTPRestoresDiscobotForwardedHeaders(t *testing.T) {
	var gotPath string
	var gotForwardedHost string
	var gotForwardedProto string
	var gotForwardedFor string
	var gotDiscobotForwardedHost string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotForwardedHost = r.Header.Get("X-Forwarded-Host")
		gotForwardedProto = r.Header.Get("X-Forwarded-Proto")
		gotForwardedFor = r.Header.Get("X-Forwarded-For")
		gotDiscobotForwardedHost = r.Header.Get(discobotForwardedHostHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	_, portString, err := net.SplitHostPort(backend.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://vm.exe.xyz/services/ui/http/wrong", nil)
	req.Host = "vm.exe.xyz"
	req.Header.Set("X-Forwarded-Host", "vm.exe.xyz")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Header.Set("X-Forwarded-Path", "/wrong")
	req.Header.Set(discobotForwardedHostHeader, "session1234-svc-ui.localhost:3001")
	req.Header.Set(discobotForwardedProtoHeader, "http")
	req.Header.Set(discobotForwardedForHeader, "127.0.0.1")
	req.Header.Set(discobotForwardedPathHeader, "/actual/path")

	rr := httptest.NewRecorder()
	ProxyHTTP(port).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if gotPath != "/actual/path" {
		t.Fatalf("path = %q, want /actual/path", gotPath)
	}
	if gotForwardedHost != "session1234-svc-ui.localhost:3001" {
		t.Fatalf("X-Forwarded-Host = %q", gotForwardedHost)
	}
	if gotForwardedProto != "http" {
		t.Fatalf("X-Forwarded-Proto = %q", gotForwardedProto)
	}
	if gotForwardedFor != "127.0.0.1, 192.0.2.1" {
		t.Fatalf("X-Forwarded-For = %q", gotForwardedFor)
	}
	if gotDiscobotForwardedHost != "" {
		t.Fatalf("%s leaked to service: %q", discobotForwardedHostHeader, gotDiscobotForwardedHost)
	}
}
