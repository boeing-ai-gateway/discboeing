// Package main is the entry point for the proxy server.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	proxyapi "github.com/obot-platform/discobot/proxy/internal/api"
	"github.com/obot-platform/discobot/proxy/internal/cert"
	"github.com/obot-platform/discobot/proxy/internal/config"
	"github.com/obot-platform/discobot/proxy/internal/logger"
	"github.com/obot-platform/discobot/proxy/internal/proxy"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "proxy: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "serve":
			return runServe(args[1:])
		case "init-certs":
			return runInitCerts(args[1:])
		case "help", "-h", "--help":
			printUsage()
			return nil
		default:
			if !strings.HasPrefix(args[0], "-") {
				return fmt.Errorf("unknown command %q\n%s", args[0], usage())
			}
		}
	}

	return runServe(args)
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	configFile := flags.String("config", "config.yaml", "Path to configuration file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Load configuration
	cfg, err := loadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create logger
	log, err := logger.New(cfg.Logging)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() { _ = log.Close() }()

	// Create proxy server
	proxyServer, err := proxy.New(cfg, log)
	if err != nil {
		log.Error("failed to create proxy server")
		return err
	}

	// Create API server
	apiServer := proxyapi.New(proxyServer, log)

	// Start config file watcher
	watcher := config.NewWatcher(*configFile, func(newCfg *config.Config) {
		log.Info("config reloaded")
		proxyServer.ApplyConfig(newCfg)
	})
	if err := watcher.Start(); err != nil {
		log.Warn("config watcher failed to start")
	} else {
		defer watcher.Stop()
	}

	// Handle shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Start API server in goroutine
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Proxy.APIPort)
		if err := apiServer.ListenAndServe(addr); err != nil {
			log.Error("api server error")
		}
	}()

	// Start proxy server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- proxyServer.ListenAndServe()
	}()

	// Wait for shutdown signal or error
	select {
	case <-shutdown:
		log.Info("shutting down...")
	case err := <-errCh:
		if err != nil {
			log.Error("proxy server error")
		}
	}

	// Graceful shutdown
	if err := proxyServer.Close(); err != nil {
		log.Error("error during shutdown")
	}

	log.Info("shutdown complete")
	return nil
}

func runInitCerts(args []string) error {
	flags := flag.NewFlagSet("init-certs", flag.ContinueOnError)
	configFile := flags.String("config", "config.yaml", "Path to configuration file")
	certDir := flags.String("cert-dir", "", "Certificate directory override")
	userName := flags.String("user", "", "Runtime user whose NSS DB should trust the CA")
	skipSystemTrust := flags.Bool("skip-system-trust", false, "Skip installing CA in the system trust store")
	skipUserTrust := flags.Bool("skip-user-trust", false, "Skip installing CA in the runtime user's NSS DB")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg, err := loadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if *certDir == "" {
		*certDir = cfg.TLS.CertDir
	}

	certPath, generated, err := cert.EnsureCA(*certDir)
	if err != nil {
		return fmt.Errorf("initialize CA: %w", err)
	}
	if generated {
		fmt.Printf("discobot-proxy: proxy CA certificate generated at %s\n", certPath)
	} else {
		fmt.Printf("discobot-proxy: proxy CA certificate already exists at %s\n", certPath)
	}

	var trustUser *cert.TrustUser
	if !*skipUserTrust && strings.TrimSpace(*userName) != "" {
		trustUser, err = cert.LookupTrustUser(*userName)
		if err != nil {
			return fmt.Errorf("lookup trust user %q: %w", *userName, err)
		}
	}

	if *skipSystemTrust {
		if trustUser == nil {
			return nil
		}
		return cert.InstallUserNSSDB(certPath, trustUser)
	}

	if *skipUserTrust {
		return cert.InstallSystemTrust(certPath)
	}

	return cert.InstallTrust(certPath, trustUser)
}

func loadConfig(path string) (*config.Config, error) {
	// Try to load config file
	cfg, err := config.Load(path)
	if err != nil {
		// If file doesn't exist, use defaults
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Config file not found, using defaults\n")
			return config.Default(), nil
		}
		return nil, err
	}
	return cfg, nil
}

func printUsage() {
	fmt.Fprint(os.Stderr, usage())
}

func usage() string {
	return `usage:
  proxy [serve] [-config config.yaml]
  proxy init-certs [-config config.yaml] [-cert-dir dir] [-user name] [-skip-system-trust] [-skip-user-trust]

commands:
  serve       Run the proxy server (default)
  init-certs  Generate the proxy CA and install it in trust stores
`
}
