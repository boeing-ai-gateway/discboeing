package main

import (
	"log/slog"
	"os"

	"github.com/obot-platform/discobot/ui-go2/internal/config"
	"github.com/obot-platform/discobot/ui-go2/internal/server"
)

func main() {
	logger := slog.Default()
	app := server.New(config.FromEnv(), logger)
	if err := app.ListenAndServe(); err != nil {
		logger.Error("ui-go2 stopped", "error", err)
		os.Exit(1)
	}
}
