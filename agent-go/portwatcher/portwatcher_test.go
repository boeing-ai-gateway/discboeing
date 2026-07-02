package portwatcher

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseSSOutputKeepsOnlyLinesWithPID(t *testing.T) {
	output := `LISTEN 0      4096   127.0.0.1:5900  0.0.0.0:*
LISTEN 0      1      127.0.0.1:44949 0.0.0.0:* users:(("python3",pid=5141,fd=3))
LISTEN 0      4096           *:3002        *:* users:(("discboeing-agent-",pid=1903,fd=6))
LISTEN 0      4096        [::1]:8080     [::]:* users:(("node",pid=42,fd=9))`

	entries := ParseSSOutput(output)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %#v", len(entries), entries)
	}

	want := []Entry{
		{LocalAddress: "*:3002", Port: 3002, Process: "discboeing-agent-", Protocol: ProtocolUnknown, PID: 1903, FD: 6},
		{LocalAddress: "[::1]:8080", Port: 8080, Process: "node", Protocol: ProtocolUnknown, PID: 42, FD: 9},
		{LocalAddress: "127.0.0.1:44949", Port: 44949, Process: "python3", Protocol: ProtocolUnknown, PID: 5141, FD: 3},
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Fatalf("entry %d mismatch\nwant: %#v\n got: %#v", i, want[i], entries[i])
		}
	}
}

func TestParsePort(t *testing.T) {
	for _, localAddress := range []string{"127.0.0.1:3000", "*:3002", "[::]:5432", "[::1]:8080"} {
		port, ok := parsePort(localAddress)
		if !ok || port == 0 {
			t.Fatalf("expected port from %q, got %d %v", localAddress, port, ok)
		}
	}
}

func TestDetectProtocolHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("expected HEAD, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	entry := entryForListener(t, server.Listener)
	if got := DetectProtocol(context.Background(), entry); got != ProtocolHTTP {
		t.Fatalf("expected %q, got %q", ProtocolHTTP, got)
	}
}

func TestDetectProtocolHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("expected HEAD, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	entry := entryForListener(t, server.Listener)
	if got := DetectProtocol(context.Background(), entry); got != ProtocolHTTPS {
		t.Fatalf("expected %q, got %q", ProtocolHTTPS, got)
	}
}

func TestDetectProtocolUnknown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	entry := entryForListener(t, listener)
	if got := DetectProtocol(context.Background(), entry); got != ProtocolUnknown {
		t.Fatalf("expected %q, got %q", ProtocolUnknown, got)
	}
	_ = listener.Close()
	<-done
}

func entryForListener(t *testing.T, listener net.Listener) Entry {
	t.Helper()

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, ok := parsePort(listener.Addr().String())
	if !ok {
		t.Fatalf("failed to parse port from %q", listener.Addr())
	}
	return Entry{LocalAddress: net.JoinHostPort(host, portText), Port: port, Protocol: ProtocolUnknown}
}
