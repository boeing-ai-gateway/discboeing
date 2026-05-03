package services

import (
	"bytes"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func TestOAuthApplicationServiceCRUD(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Organization{}, &model.OAuthApplication{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	st := store.New(db, nil)
	organization := &model.Organization{Name: "Example", Domain: "example.com"}
	if err := st.CreateOrganization(t.Context(), organization); err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	encryptor, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewLocalEncryptor() error = %v", err)
	}
	svc := &OAuthApplicationService{Store: st, DatabaseEncryptor: encryptor}

	name := "GitHub Login"
	provider := model.OAuthApplicationProviderGitHub
	clientID := "github-client"
	clientSecret := "secret"
	created, err := svc.CreateOAuthApplication(t.Context(), organization.Domain, "bootstrap:example.com", OAuthApplicationInput{
		Name:         &name,
		Provider:     &provider,
		ClientID:     &clientID,
		ClientSecret: &clientSecret,
		RedirectURIs: []string{"https://example.com/callback"},
		Scopes:       []string{"read:user", "user:email"},
		GitHub:       map[string]any{"enterpriseBaseURL": "https://github.example.com"},
	})
	if err != nil {
		t.Fatalf("CreateOAuthApplication() error = %v", err)
	}
	if created.ID == "" || created.Provider != model.OAuthApplicationProviderGitHub || !created.HasClientSecret {
		t.Fatalf("unexpected created app: %#v", created)
	}

	stored, err := st.GetOAuthApplication(t.Context(), organization.ID, created.ID)
	if err != nil {
		t.Fatalf("GetOAuthApplication() error = %v", err)
	}
	if len(stored.ClientSecretEncrypted) == 0 || bytes.Contains(stored.ClientSecretEncrypted, []byte(`"secret"`)) {
		t.Fatalf("client secret was not encrypted correctly: %s", stored.ClientSecretEncrypted)
	}
	decrypted, err := svc.DecryptClientSecret(t.Context(), stored)
	if err != nil {
		t.Fatalf("DecryptClientSecret() error = %v", err)
	}
	if decrypted != clientSecret {
		t.Fatalf("decrypted client secret = %q", decrypted)
	}

	updatedName := "Google Login"
	updatedProvider := model.OAuthApplicationProviderGoogle
	updatedClientID := "google-client"
	updated, err := svc.UpdateOAuthApplication(t.Context(), organization.Domain, created.ID, OAuthApplicationInput{
		Name:     &updatedName,
		Provider: &updatedProvider,
		ClientID: &updatedClientID,
		Google:   map[string]any{"hostedDomain": "example.com"},
	})
	if err != nil {
		t.Fatalf("UpdateOAuthApplication() error = %v", err)
	}
	if updated.Name != updatedName || updated.Provider != updatedProvider || updated.ClientID != updatedClientID {
		t.Fatalf("unexpected updated app: %#v", updated)
	}

	items, err := svc.ListOAuthApplications(t.Context(), organization.Domain)
	if err != nil {
		t.Fatalf("ListOAuthApplications() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("unexpected list: %#v", items)
	}
	if err := svc.DeleteOAuthApplication(t.Context(), organization.Domain, created.ID); err != nil {
		t.Fatalf("DeleteOAuthApplication() error = %v", err)
	}
	if _, err := svc.GetOAuthApplication(t.Context(), organization.Domain, created.ID); !IsNotFound(err) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}
