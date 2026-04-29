package cli

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/config"
)

func TestNewRemoteSession_RequiresSecret(t *testing.T) {
	session := newRemoteSession(&config.Config{Port: 3002, AgentCwd: t.TempDir()})
	if session != nil {
		t.Fatal("expected nil session when DISCOBOT_SECRET is unset")
	}
}

func TestNewRemoteSession_UsesConfiguredPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	called := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/commands", func(w http.ResponseWriter, _ *http.Request) {
		select {
		case called <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"commands": []any{}})
	})
	server := &http.Server{Handler: mux}
	defer server.Close()
	go func() {
		_ = server.Serve(ln)
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	session := newRemoteSession(&config.Config{
		Port:       port,
		SecretHash: "test-secret",
		AgentCwd:   t.TempDir(),
	})
	if session == nil {
		t.Fatal("expected remote session when DISCOBOT_SECRET is set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := session.ListCommands(ctx); err != nil {
		t.Fatalf("expected remote list commands to succeed: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request to configured port")
	}
}
