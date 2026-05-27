package main

import (
	"log/slog"
	"os"

	"github.com/obot-platform/discobot/discobot/internal/config"
	"github.com/obot-platform/discobot/discobot/internal/server"
)

func main() {
	logger := slog.Default()
	app := server.New(config.FromEnv(), logger)
	if err := app.ListenAndServe(); err != nil {
		logger.Error("discobot stopped", "error", err)
		os.Exit(1)
	}
}
