package config

import "os"

// Config contains runtime settings for the ui-go server.
type Config struct {
	Port       string
	StaticDir  string
	APIBaseURL string
}

// FromEnv reads ui-go configuration from the process environment.
func FromEnv() Config {
	port := os.Getenv("UI_GO_PORT")
	if port == "" {
		port = "3200"
	}

	staticDir := os.Getenv("UI_GO_STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}
	apiBaseURL := os.Getenv("DISCOBOT_API_BASE_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://127.0.0.1:3001"
	}

	return Config{
		Port:       port,
		StaticDir:  staticDir,
		APIBaseURL: apiBaseURL,
	}
}
