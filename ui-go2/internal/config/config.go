package config

import "os"

// Config contains runtime settings for the ui-go2 server.
type Config struct {
	Port      string
	StaticDir string
}

// FromEnv reads ui-go2 configuration from the process environment.
func FromEnv() Config {
	port := os.Getenv("UI_GO2_PORT")
	if port == "" {
		port = "3300"
	}

	staticDir := os.Getenv("UI_GO2_STATIC_DIR")
	if staticDir == "" {
		staticDir = "static"
	}

	return Config{
		Port:      port,
		StaticDir: staticDir,
	}
}
