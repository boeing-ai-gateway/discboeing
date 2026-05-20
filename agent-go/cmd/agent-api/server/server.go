// Package server implements the HTTP API server mode for agent-api.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/obot-platform/discobot/agent-go/internal/config"
	"github.com/obot-platform/discobot/agent-go/internal/routes"

	// Embed the API browser HTML.
	staticFiles "github.com/obot-platform/discobot/agent-go/cmd/agent-api/static"
)

// Run starts the HTTP API server and blocks until SIGINT/SIGTERM.
func Run(cfg *config.Config) {
	if cfg.DynamicConfigRequired() {
		runBootstrap(cfg)
		return
	}

	runtimeHandler, cleanup, runStartupHooks, err := buildRuntimeHandler(cfg, runtimeInitialCredentials{})
	if err != nil {
		log.Fatalf("agent-api: initialize runtime: %v", err)
	}

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Standard middleware (no global timeout — SSE and long AI calls need
	// unbounded connection lifetimes).
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Mount("/", runtimeHandler)

	// ── Meta routes ──────────────────────────────────────────────────────────

	// GET /api/ui — embedded API browser HTML (same UI as the main server).
	r.Get("/api/ui", func(w http.ResponseWriter, _ *http.Request) {
		content, err := staticFiles.Files.ReadFile("api-ui.html")
		if err != nil {
			http.Error(w, "API UI not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	})

	// GET /api/routes — returns route metadata for the API browser.
	r.Get("/api/routes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(routes.All())
	})

	// ── HTTP server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: r,
		// No read/write/idle timeouts — SSE and long-running AI calls need
		// connections that can stay open for minutes or longer.
	}

	go func() {
		log.Printf("agent-api: listening on :%d (cwd: %s)", cfg.Port, cfg.AgentCwd)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent-api: server error: %v", err)
		}
	}()

	runStartupHooks(nil)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("agent-api: shutting down...")
	cleanup()

	// Give in-flight requests up to 5 seconds to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("agent-api: shutdown error: %v", err)
	}

	log.Println("agent-api: stopped")
}
