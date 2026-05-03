package config

import (
	"strings"
	"testing"
)

func TestLoadDatabaseConfigFromMetaEnv(t *testing.T) {
	t.Setenv("META_DATABASE_DSN", "sqlite3:///tmp/meta-test.db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseDriver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", cfg.DatabaseDriver)
	}
	if got := cfg.CleanDSN(); got != "/tmp/meta-test.db" {
		t.Fatalf("expected clean DSN /tmp/meta-test.db, got %q", got)
	}
}

func TestLoadDatabaseConfigFallsBackToDatabaseDSN(t *testing.T) {
	t.Setenv("DATABASE_DSN", "postgres://user:pass@localhost:5432/meta")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseDriver != "postgres" {
		t.Fatalf("expected postgres driver, got %q", cfg.DatabaseDriver)
	}
	if got := cfg.CleanDSN(); got != "postgres://user:pass@localhost:5432/meta" {
		t.Fatalf("expected postgres clean DSN, got %q", got)
	}
}

func TestLoadDatabaseConfigDefaultsToSQLite(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseDriver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", cfg.DatabaseDriver)
	}
	if !strings.HasSuffix(cfg.CleanDSN(), "discobot/meta.db") {
		t.Fatalf("expected default meta sqlite path, got %q", cfg.CleanDSN())
	}
}

func TestLoadGCPJWTSigningKeyID(t *testing.T) {
	t.Setenv("META_JWT_SIGNING_BACKEND", JWTSigningBackendGCPKMS)
	t.Setenv("META_JWT_SIGNING_KEY_ID", "projects/p/locations/global/keyRings/r/cryptoKeys/k")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.JWTSigning.KeyID != "projects/p/locations/global/keyRings/r/cryptoKeys/k" {
		t.Fatalf("unexpected JWT signing key ID %q", cfg.JWTSigning.KeyID)
	}
}

func TestLoadGCPJWTSigningRequiresKeyID(t *testing.T) {
	t.Setenv("META_JWT_SIGNING_BACKEND", JWTSigningBackendGCPKMS)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "META_JWT_SIGNING_KEY_ID") {
		t.Fatalf("expected META_JWT_SIGNING_KEY_ID error, got %v", err)
	}
}
