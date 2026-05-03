package meta

import (
	"context"
	"errors"

	"github.com/obot-platform/discobot/meta/internal/config"
	"github.com/obot-platform/discobot/meta/internal/database"
	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func initDatabase(cfg *config.Config) (*database.DB, *store.Store, error) {
	db, err := database.New(cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Migrate(); err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return db, store.New(db.DB, db.ReadDB), nil
}

func initDatabaseEncryptor(cfg *config.Config) (dbcrypt.Encryptor, error) {
	switch cfg.DatabaseEncryption.Provider {
	case config.DatabaseEncryptionProviderLocal:
		return dbcrypt.NewLocalEncryptor(cfg.DatabaseEncryption.KeyID, cfg.DatabaseEncryption.Key)
	case config.DatabaseEncryptionProviderGCPKMS:
		return dbcrypt.NewGCPKMSEncryptor(context.Background(), cfg.DatabaseEncryption.KeyID)
	case "aws-kms", "azure-key-vault":
		return nil, errors.New("cloud database encryption providers are configured but not implemented yet")
	default:
		return nil, errors.New("unsupported database encryption provider")
	}
}
