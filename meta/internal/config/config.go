package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/joho/godotenv"
)

const (
	DatabaseEncryptionProviderLocal  = "local"
	DatabaseEncryptionProviderGCPKMS = "gcp-kms"
	JWTSigningBackendDBLocal         = "db-local"
	JWTSigningBackendGCPKMS          = "gcp-kms"
)

// Config holds all configuration for the Meta service.
type Config struct {
	Addr        string
	CORSOrigins []string

	DatabaseDSN    string
	DatabaseDriver string

	DatabaseEncryption DatabaseEncryptionConfig
	JWTSigning         JWTSigningConfig
}

// DatabaseEncryptionConfig configures operational database field encryption.
type DatabaseEncryptionConfig struct {
	Provider string
	Required bool
	KeyID    string
	Key      []byte
	KeyFile  string
}

// JWTSigningConfig configures Meta-owned JWT signing key management.
type JWTSigningConfig struct {
	Backend             string
	Issuer              string
	Alg                 string
	KeyID               string
	RotationInterval    time.Duration
	PrepublishWindow    time.Duration
	VerificationOverlap time.Duration
}

// Load reads Meta configuration from .env files and environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Addr:        getEnv("META_ADDR", ":3011"),
		CORSOrigins: getEnvList("META_CORS_ORIGINS", []string{"*"}),
		DatabaseDSN: getEnv("META_DATABASE_DSN",
			getEnv("DATABASE_DSN", "sqlite3://"+filepath.Join(xdg.DataHome, "discobot", "meta.db"))),
		DatabaseEncryption: DatabaseEncryptionConfig{
			Provider: strings.ToLower(getEnv("META_DB_ENCRYPTION_PROVIDER", DatabaseEncryptionProviderLocal)),
			Required: getEnvBool("META_DB_ENCRYPTION_REQUIRED", false),
			KeyID:    getEnv("META_DB_ENCRYPTION_KEY_ID", "local"),
			KeyFile:  getEnv("META_DB_ENCRYPTION_KEY_FILE", ""),
		},
		JWTSigning: JWTSigningConfig{
			Backend:             strings.ToLower(getEnv("META_JWT_SIGNING_BACKEND", JWTSigningBackendDBLocal)),
			Issuer:              strings.TrimRight(getEnv("META_JWT_ISSUER", "http://localhost:3011"), "/"),
			Alg:                 getEnv("META_JWT_SIGNING_ALG", "ES256"),
			KeyID:               getEnv("META_JWT_SIGNING_KEY_ID", ""),
			RotationInterval:    getEnvDuration("META_JWT_SIGNING_ROTATION_INTERVAL", 72*time.Hour),
			PrepublishWindow:    getEnvDuration("META_JWT_SIGNING_PREPUBLISH_WINDOW", 24*time.Hour),
			VerificationOverlap: getEnvDuration("META_JWT_SIGNING_VERIFICATION_OVERLAP", 7*24*time.Hour),
		},
	}
	cfg.DatabaseDriver = detectDriver(cfg.DatabaseDSN)

	if err := loadDatabaseEncryptionKey(&cfg.DatabaseEncryption); err != nil {
		return nil, err
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func detectDriver(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://"):
		return "postgres"
	case strings.HasPrefix(dsn, "sqlite3://") || strings.HasPrefix(dsn, "sqlite://"):
		return "sqlite"
	case strings.HasSuffix(dsn, ".db") || strings.HasSuffix(dsn, ".sqlite"):
		return "sqlite"
	default:
		return "postgres"
	}
}

func (c *Config) CleanDSN() string {
	dsn := c.DatabaseDSN
	dsn = strings.TrimPrefix(dsn, "postgres://")
	dsn = strings.TrimPrefix(dsn, "postgresql://")
	dsn = strings.TrimPrefix(dsn, "sqlite3://")
	dsn = strings.TrimPrefix(dsn, "sqlite://")
	if c.DatabaseDriver == "postgres" {
		return "postgres://" + dsn
	}
	return dsn
}

func loadDatabaseEncryptionKey(cfg *DatabaseEncryptionConfig) error {
	if cfg.Provider != DatabaseEncryptionProviderLocal {
		return nil
	}

	keyHex := getEnv("META_DB_ENCRYPTION_KEY", "")
	if keyHex == "" && cfg.KeyFile != "" {
		data, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("read META_DB_ENCRYPTION_KEY_FILE: %w", err)
		}
		keyHex = strings.TrimSpace(string(data))
	}
	if keyHex == "" {
		if cfg.Required {
			return fmt.Errorf("META_DB_ENCRYPTION_KEY or META_DB_ENCRYPTION_KEY_FILE is required when META_DB_ENCRYPTION_REQUIRED=true")
		}
		keyHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}

	key, err := hex.DecodeString(strings.TrimSpace(keyHex))
	if err != nil {
		return fmt.Errorf("META_DB_ENCRYPTION_KEY must be hex encoded: %w", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("META_DB_ENCRYPTION_KEY must be exactly 32 bytes (64 hex chars), got %d bytes", len(key))
	}
	cfg.Key = key
	return nil
}

func validate(cfg *Config) error {
	switch cfg.DatabaseEncryption.Provider {
	case DatabaseEncryptionProviderLocal:
	case "aws-kms", DatabaseEncryptionProviderGCPKMS, "azure-key-vault":
		if cfg.DatabaseEncryption.KeyID == "" {
			return fmt.Errorf("META_DB_ENCRYPTION_KEY_ID is required for provider %q", cfg.DatabaseEncryption.Provider)
		}
	default:
		return fmt.Errorf("META_DB_ENCRYPTION_PROVIDER must be one of: local, aws-kms, gcp-kms, azure-key-vault")
	}

	switch cfg.JWTSigning.Backend {
	case JWTSigningBackendDBLocal, "aws-kms", JWTSigningBackendGCPKMS, "azure-key-vault":
	default:
		return fmt.Errorf("META_JWT_SIGNING_BACKEND must be one of: db-local, aws-kms, gcp-kms, azure-key-vault")
	}
	if cfg.JWTSigning.Backend == JWTSigningBackendGCPKMS && cfg.JWTSigning.KeyID == "" {
		return fmt.Errorf("META_JWT_SIGNING_KEY_ID is required when META_JWT_SIGNING_BACKEND=gcp-kms")
	}
	if cfg.JWTSigning.Alg != "ES256" {
		return fmt.Errorf("META_JWT_SIGNING_ALG must be ES256")
	}
	if cfg.JWTSigning.RotationInterval <= 0 {
		return fmt.Errorf("META_JWT_SIGNING_ROTATION_INTERVAL must be greater than zero")
	}
	if cfg.JWTSigning.PrepublishWindow <= 0 {
		return fmt.Errorf("META_JWT_SIGNING_PREPUBLISH_WINDOW must be greater than zero")
	}
	if cfg.JWTSigning.VerificationOverlap <= 0 {
		return fmt.Errorf("META_JWT_SIGNING_VERIFICATION_OVERLAP must be greater than zero")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvList(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
		return result
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
