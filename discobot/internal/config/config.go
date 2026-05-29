package config

import (
	"net/http"
	"os"
)

// Config contains runtime settings for the Discobot server.
type Config struct {
	Port      string
	StaticDir string
	StaticFS  http.FileSystem
	DevReload bool
}

// FromEnv reads Discobot configuration from the process environment.
func FromEnv() Config {
	port := os.Getenv("DISCOBOT_PORT")
	if port == "" {
		port = "3300"
	}

	staticDir := os.Getenv("DISCOBOT_STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}

	devReload := os.Getenv("DISCOBOT_DEV_RELOAD") == "1"
	return Config{
		Port:      port,
		StaticDir: staticDir,
		DevReload: devReload,
	}
}
