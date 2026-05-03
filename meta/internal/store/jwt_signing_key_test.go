package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestJWTSigningKeyStoreCRUD(t *testing.T) {
	st := newTestStore(t)
	key := &model.JWTSigningKey{
		KeyID:         "kid_1",
		Algorithm:     "ES256",
		Backend:       model.JWTSigningKeyBackendDBLocal,
		PublicJWKJSON: []byte(`{"kty":"EC","kid":"kid_1"}`),
		Status:        model.JWTSigningKeyStatusNext,
	}
	if err := st.CreateJWTSigningKey(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if key.ID == "" {
		t.Fatal("expected BeforeCreate to set ID")
	}

	got, err := st.GetJWTSigningKeyByKID(context.Background(), "kid_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != key.ID {
		t.Fatalf("got ID %q, want %q", got.ID, key.ID)
	}

	keys, err := st.ListJWTSigningKeys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0].ID != key.ID {
		t.Fatalf("unexpected keys: %#v", keys)
	}

	notBefore := time.Now().UTC()
	notAfter := notBefore.Add(time.Hour)
	if err := st.UpdateJWTSigningKeyStatus(context.Background(), key.ID, model.JWTSigningKeyStatusActive, &notBefore, &notAfter); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetJWTSigningKeyByID(context.Background(), key.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.JWTSigningKeyStatusActive || got.NotBefore == nil || got.NotAfter == nil {
		t.Fatalf("unexpected updated key: %#v", got)
	}
}

func TestJWTSigningKeyStoreScopesByOrganization(t *testing.T) {
	st := newTestStore(t)
	orgID := "org_1"
	for _, key := range []*model.JWTSigningKey{
		{KeyID: "global", Algorithm: "ES256", Backend: model.JWTSigningKeyBackendDBLocal, PublicJWKJSON: json.RawMessage(`{"kid":"global"}`)},
		{OrganizationID: &orgID, KeyID: "org", Algorithm: "ES256", Backend: model.JWTSigningKeyBackendDBLocal, PublicJWKJSON: json.RawMessage(`{"kid":"org"}`)},
	} {
		if err := st.CreateJWTSigningKey(context.Background(), key); err != nil {
			t.Fatal(err)
		}
	}
	globalKeys, err := st.ListJWTSigningKeys(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(globalKeys) != 1 || globalKeys[0].KeyID != "global" {
		t.Fatalf("unexpected global keys: %#v", globalKeys)
	}
	orgKeys, err := st.ListJWTSigningKeys(context.Background(), &orgID)
	if err != nil {
		t.Fatal(err)
	}
	if len(orgKeys) != 1 || orgKeys[0].KeyID != "org" {
		t.Fatalf("unexpected org keys: %#v", orgKeys)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.JWTSigningKey{}); err != nil {
		t.Fatal(err)
	}
	return New(db, nil)
}
