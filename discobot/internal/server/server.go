package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/config"
	"github.com/obot-platform/discobot/discobot/internal/state"
	datasync "github.com/obot-platform/discobot/discobot/internal/sync"
)

// Server owns Discobot HTTP routing and state streaming.
type Server struct {
	config      config.Config
	logger      *slog.Logger
	mu          sync.Mutex
	data        state.Data
	devReloadID string
	subscribers map[chan struct{}]struct{}
	syncManager dataSyncManager
}

type dataSyncManager interface {
	Run(context.Context)
}

// New wires the Discobot server dependencies and route table.
func New(cfg config.Config, logger *slog.Logger) *Server {
	server := &Server{
		config:      cfg,
		logger:      logger,
		data:        state.DefaultData(),
		devReloadID: time.Now().UTC().Format(time.RFC3339Nano),
		subscribers: map[chan struct{}]struct{}{},
	}
	if cfg.ServerBaseURL != "" {
		if len(cfg.ServerBaseURL) >= len("file://") && cfg.ServerBaseURL[:len("file://")] == "file://" {
			if syncManager, err := datasync.NewFileManager(cfg.ServerBaseURL, server, logger); err != nil {
				logger.Warn("failed to create discobot file data sync manager", "error", err)
			} else {
				server.syncManager = syncManager
			}
		} else {
			if syncManager, err := datasync.NewManager(cfg.ServerBaseURL, server, logger); err != nil {
				logger.Warn("failed to create discobot data sync manager", "error", err)
			} else {
				server.syncManager = syncManager
			}
		}
	}
	return server
}

// Handler returns the HTTP route tree for Discobot.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/ui/stream", s.handleUIStream)
	return r
}

// ListenAndServe starts the Discobot HTTP server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := ":" + s.config.Port
	if s.syncManager != nil {
		go s.syncManager.Run(ctx)
	}
	s.logger.Info("starting discobot", "addr", addr, "staticDir", s.config.StaticDir, "serverBaseURL", s.config.ServerBaseURL)
	server := &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to shut down discobot server", "error", err)
		}
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) handleUIStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	events, cancel := s.subscribe()
	defer cancel()

	if err := writeDataEvent(w, s.snapshot()); err != nil {
		s.logger.Warn("failed to send initial state", "error", err)
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
			if err := writeDataEvent(w, s.snapshot()); err != nil {
				s.logger.Warn("failed to send state update", "error", err)
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) snapshot() state.Data {
	s.mu.Lock()
	data := s.data
	s.mu.Unlock()
	return state.NewData(data)
}

// SaveData publishes an updated copy of the server-owned application data.
// Always preserve the clone/mutate/assign pattern here so callers cannot mutate
// snapshots that may still be read concurrently by renderers or stream handlers.
func (s *Server) SaveData(_ context.Context, update func(*state.Data)) {
	s.mu.Lock()
	data := state.NewData(s.data)
	update(&data)
	s.data = data
	s.mu.Unlock()
	s.publish()
}

func writeDataEvent(w http.ResponseWriter, data state.Data) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}

func (s *Server) publish() {
	s.mu.Lock()
	subscribers := make([]chan struct{}, 0, len(s.subscribers))
	for subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func (s *Server) subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, cancel
}
