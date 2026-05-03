package jwtkeys

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func TestPersistentSigningKeyStoreBootstrapsActiveKey(t *testing.T) {
	st, keyStore := newPersistentKeyStore(t)
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	active, err := keyStore.ActiveSigningKey(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != model.JWTSigningKeyStatusActive || active.PrivateKeyEncrypted == nil {
		t.Fatalf("unexpected active key: %#v", active)
	}
	persisted, err := st.GetJWTSigningKeyByID(context.Background(), active.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.KeyID != active.KeyID {
		t.Fatalf("persisted key ID = %q, want %q", persisted.KeyID, active.KeyID)
	}
}

func TestPersistentSigningKeyStoreCreatesNextAndPublishesJWKS(t *testing.T) {
	_, keyStore := newPersistentKeyStore(t)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	keyStore.now = func() time.Time { return now }
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	keyStore.now = func() time.Time { return now.Add(49 * time.Hour) }
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	jwksJSON, err := keyStore.PublicJWKS(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(jwksJSON, &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 {
		t.Fatalf("expected active and next keys in JWKS, got %s", jwksJSON)
	}
}

func TestPersistentSigningKeyStorePromotesAndRetiresWithOverlap(t *testing.T) {
	_, keyStore := newPersistentKeyStore(t)
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	keyStore.now = func() time.Time { return now }
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	active, err := keyStore.ActiveSigningKey(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	keyStore.now = func() time.Time { return now.Add(49 * time.Hour) }
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	keyStore.now = func() time.Time { return now.Add(74 * time.Hour) }
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	newActive, err := keyStore.ActiveSigningKey(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if newActive.ID == active.ID {
		t.Fatal("expected promoted key to become active")
	}
	jwksJSON, err := keyStore.PublicJWKS(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var jwks struct {
		Keys []map[string]string `json:"keys"`
	}
	if err := json.Unmarshal(jwksJSON, &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 2 {
		t.Fatalf("expected active and retired overlap keys in JWKS, got %s", jwksJSON)
	}
}

func TestPersistentSigningKeyStoreNoActiveKey(t *testing.T) {
	_, keyStore := newPersistentKeyStore(t)
	_, err := keyStore.ActiveSigningKey(context.Background(), "")
	if !errors.Is(err, ErrNoActiveSigningKey) {
		t.Fatalf("expected ErrNoActiveSigningKey, got %v", err)
	}
}

func TestPersistentSigningKeyStoreExternalBackendSkipsAutomaticCreation(t *testing.T) {
	st, keyStore := newPersistentKeyStoreWithOptions(t, PersistentSigningKeyStoreOptions{
		Backend: BackendGCPKMS,
	})
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	keys, err := st.ListJWTSigningKeys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected no automatically-created external keys, got %d", len(keys))
	}
}

func TestPersistentSigningKeyStoreExternalBackendUsesKeyFactory(t *testing.T) {
	st, keyStore := newPersistentKeyStoreWithOptions(t, PersistentSigningKeyStoreOptions{
		Backend: BackendGCPKMS,
		KeyFactory: func(_ context.Context, status string, notBefore, notAfter *time.Time, alg string) (*model.JWTSigningKey, error) {
			return &model.JWTSigningKey{
				KeyID:         "gcp-created",
				Algorithm:     alg,
				Backend:       BackendGCPKMS,
				BackendKeyID:  new("projects/p/locations/global/keyRings/r/cryptoKeys/k/cryptoKeyVersions/1"),
				PublicJWKJSON: []byte(`{"kty":"EC","kid":"gcp-created"}`),
				Status:        status,
				NotBefore:     notBefore,
				NotAfter:      notAfter,
			}, nil
		},
	})
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	keys, err := st.ListJWTSigningKeys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].Status != model.JWTSigningKeyStatusActive || keys[0].Backend != BackendGCPKMS {
		t.Fatalf("unexpected keys: %#v", keys)
	}
}

func TestPersistentSigningKeyStoreExternalBackendPromotesAndRetires(t *testing.T) {
	st, keyStore := newPersistentKeyStoreWithOptions(t, PersistentSigningKeyStoreOptions{
		Backend: BackendGCPKMS,
	})
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	activeStart := now.Add(-74 * time.Hour)
	nextStart := now.Add(-25 * time.Hour)
	active := &model.JWTSigningKey{
		KeyID:         "active",
		Algorithm:     AlgorithmES256,
		Backend:       BackendGCPKMS,
		BackendKeyID:  new("gcp-active"),
		PublicJWKJSON: []byte(`{"kty":"EC","kid":"active"}`),
		Status:        model.JWTSigningKeyStatusActive,
		NotBefore:     &activeStart,
	}
	next := &model.JWTSigningKey{
		KeyID:         "next",
		Algorithm:     AlgorithmES256,
		Backend:       BackendGCPKMS,
		BackendKeyID:  new("gcp-next"),
		PublicJWKJSON: []byte(`{"kty":"EC","kid":"next"}`),
		Status:        model.JWTSigningKeyStatusNext,
		NotBefore:     &nextStart,
	}
	if err := st.CreateJWTSigningKey(context.Background(), active); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateJWTSigningKey(context.Background(), next); err != nil {
		t.Fatal(err)
	}

	keyStore.now = func() time.Time { return now }
	if err := keyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	keys, err := st.ListJWTSigningKeys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]string{}
	for _, key := range keys {
		statuses[key.KeyID] = key.Status
	}
	if statuses["active"] != model.JWTSigningKeyStatusRetired {
		t.Fatalf("expected active key to retire, got %q", statuses["active"])
	}
	if statuses["next"] != model.JWTSigningKeyStatusActive {
		t.Fatalf("expected next key to promote, got %q", statuses["next"])
	}
}

func newPersistentKeyStore(t *testing.T) (*store.Store, *PersistentSigningKeyStore) {
	return newPersistentKeyStoreWithOptions(t, PersistentSigningKeyStoreOptions{})
}

func newPersistentKeyStoreWithOptions(t *testing.T, opts PersistentSigningKeyStoreOptions) (*store.Store, *PersistentSigningKeyStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.JWTSigningKey{}); err != nil {
		t.Fatal(err)
	}
	encryptor, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(db, nil)
	return st, NewPersistentSigningKeyStore(st, encryptor, opts)
}
