package meta

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/obot-platform/discobot/meta/internal/config"
	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/jwtkeys"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func initJWTSigning(ctx context.Context, cfg *config.Config, st *store.Store, dbEncryptor dbcrypt.Encryptor) (*jwtkeys.PersistentSigningKeyStore, error) {
	if _, err := newJWTSigningManager(cfg, dbEncryptor); err != nil {
		return nil, err
	}
	signingKeyStoreOptions, err := newSigningKeyStoreOptions(ctx, cfg)
	if err != nil {
		return nil, err
	}
	signingKeyStore := jwtkeys.NewPersistentSigningKeyStore(st, dbEncryptor, signingKeyStoreOptions)
	if err := signingKeyStore.EnsureReady(ctx); err != nil {
		return nil, err
	}
	go runJWTSigningKeyRotation(ctx, signingKeyStore, jwtRotationCheckInterval(cfg))
	return signingKeyStore, nil
}

func newSigningKeyStoreOptions(ctx context.Context, cfg *config.Config) (jwtkeys.PersistentSigningKeyStoreOptions, error) {
	opts := jwtkeys.PersistentSigningKeyStoreOptions{
		Backend:   cfg.JWTSigning.Backend,
		Algorithm: cfg.JWTSigning.Alg,
		Policy: jwtkeys.RotationPolicy{
			Interval:            cfg.JWTSigning.RotationInterval,
			PrepublishWindow:    cfg.JWTSigning.PrepublishWindow,
			VerificationOverlap: cfg.JWTSigning.VerificationOverlap,
		},
	}
	if cfg.JWTSigning.Backend == config.JWTSigningBackendGCPKMS {
		factory, err := jwtkeys.NewGCPKMSSigningKeyFactory(ctx, cfg.JWTSigning.KeyID)
		if err != nil {
			return jwtkeys.PersistentSigningKeyStoreOptions{}, err
		}
		opts.KeyFactory = factory
	}
	return opts, nil
}

func runJWTSigningKeyRotation(ctx context.Context, signingKeyStore *jwtkeys.PersistentSigningKeyStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := signingKeyStore.EnsureReady(ctx); err != nil {
				log.Printf("failed to refresh JWT signing keys: %v", err)
			}
		}
	}
}

func jwtRotationCheckInterval(cfg *config.Config) time.Duration {
	interval := cfg.JWTSigning.RotationInterval / 24
	if prepublishInterval := cfg.JWTSigning.PrepublishWindow / 4; prepublishInterval < interval {
		interval = prepublishInterval
	}
	if interval > time.Hour {
		return time.Hour
	}
	if interval < time.Minute {
		return time.Minute
	}
	return interval
}

func newJWTSigningManager(cfg *config.Config, dbEncryptor dbcrypt.Encryptor) (*jwtkeys.Manager, error) {
	switch cfg.JWTSigning.Backend {
	case config.JWTSigningBackendDBLocal:
		return jwtkeys.NewManager(jwtkeys.JWTSignerRegistration{
			Backend: jwtkeys.BackendDBLocal,
			Signer:  jwtkeys.NewDBLocalSigner(dbEncryptor),
		}), nil
	case config.JWTSigningBackendGCPKMS:
		gcpSigner, err := jwtkeys.NewGCPKMSSigner(context.Background())
		if err != nil {
			return nil, err
		}
		return jwtkeys.NewManager(
			jwtkeys.JWTSignerRegistration{
				Backend: jwtkeys.BackendDBLocal,
				Signer:  jwtkeys.NewDBLocalSigner(dbEncryptor),
			},
			jwtkeys.JWTSignerRegistration{
				Backend: jwtkeys.BackendGCPKMS,
				Signer:  gcpSigner,
			},
		), nil
	case "aws-kms", "azure-key-vault":
		return nil, errors.New("cloud JWT signing backends are configured but not implemented yet")
	default:
		return nil, errors.New("unsupported JWT signing backend")
	}
}
