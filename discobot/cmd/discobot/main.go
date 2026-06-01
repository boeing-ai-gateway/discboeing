package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/obot-platform/discobot/discobot/internal/config"
	"github.com/obot-platform/discobot/discobot/internal/server"
)

func main() {
	logger := slog.Default()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app := server.New(config.FromEnv(), logger)
	if err := app.ListenAndServe(ctx); err != nil {
		logger.Error("discobot stopped", "error", err)
		os.Exit(1)
	}
}
