package meta

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/obot-platform/discobot/meta/internal/config"
)

// Run initializes and serves the Meta application until it receives an
// interrupt signal.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	db, st, err := initDatabase(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("failed to close database: %v", err)
		}
	}()

	bootstrap, err := ensurePublicOrganizationBootstrap(ctx, st)
	if err != nil {
		return err
	}
	logBootstrapResult(bootstrap)

	dbEncryptor, err := initDatabaseEncryptor(cfg)
	if err != nil {
		return err
	}
	signingKeyStore, err := initJWTSigning(ctx, cfg, st, dbEncryptor)
	if err != nil {
		return err
	}

	return serve(ctx, cfg, newRouter(cfg, st, signingKeyStore, dbEncryptor))
}
