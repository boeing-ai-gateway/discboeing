package config

import (
	"net/http"
	"os"
	"path/filepath"
)

// Config contains runtime settings for the Discobot server.
type Config struct {
	Port          string
	StaticDir     string
	StaticFS      http.FileSystem
	DevReload     bool
	ServerBaseURL string
	SessionDir    string
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

	serverBaseURL := os.Getenv("DISCOBOT_SERVER_URL")
	if serverBaseURL == "" {
		serverBaseURL = "http://localhost:3001"
	}

	sessionDir := os.Getenv("DISCOBOT_SESSION_DIR")
	if sessionDir == "" {
		configDir, err := os.UserConfigDir()
		if err == nil {
			sessionDir = filepath.Join(configDir, "discobot", "sessions")
		}
	}

	devReload := os.Getenv("DISCOBOT_DEV_RELOAD") == "1"
	return Config{
		Port:          port,
		StaticDir:     staticDir,
		DevReload:     devReload,
		ServerBaseURL: serverBaseURL,
		SessionDir:    sessionDir,
	}
}
