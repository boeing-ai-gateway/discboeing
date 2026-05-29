// Package web exposes the new Discobot Datastar UI as a mountable HTTP handler.
package web

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/obot-platform/discobot/discobot/internal/config"
	"github.com/obot-platform/discobot/discobot/internal/server"
	staticassets "github.com/obot-platform/discobot/discobot/static"
)

// Config controls how the new Discobot UI handler is mounted.
type Config struct {
	Logger *slog.Logger
}

// NewHandler returns an HTTP handler for the new Datastar UI.
func NewHandler(cfg Config) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	staticFS, err := fs.Sub(staticassets.Files, ".")
	if err != nil {
		panic("failed to initialize embedded Discobot UI assets: " + err.Error())
	}

	app := server.New(config.Config{
		StaticFS: http.FS(staticFS),
	}, logger)
	return app.Handler()
}
