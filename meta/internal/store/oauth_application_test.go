package store

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestOAuthApplicationStoreLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Organization{}, &model.OAuthApplication{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	st := New(db, nil)
	organization := &model.Organization{Name: "Example", Domain: "example.com"}
	if err := st.CreateOrganization(ctx, organization); err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}

	app := &model.OAuthApplication{
		OrganizationID:          organization.ID,
		Provider:                model.OAuthApplicationProviderGitHub,
		ClientID:                "github-client",
		Name:                    "GitHub Login",
		RedirectURIsJSON:        []byte(`["https://example.com/callback"]`),
		GrantTypesJSON:          []byte(`["authorization_code"]`),
		ResponseTypesJSON:       []byte(`["code"]`),
		Scopes:                  "read:user user:email",
		ProviderConfigJSON:      []byte(`{"enterpriseBaseURL":"https://github.example.com"}`),
		TokenEndpointAuthMethod: "client_secret_basic",
		CreatedByPrincipal:      "bootstrap:example.com",
	}
	if err := st.CreateOAuthApplication(ctx, app); err != nil {
		t.Fatalf("CreateOAuthApplication() error = %v", err)
	}
	if app.ID == "" {
		t.Fatalf("expected CreateOAuthApplication to assign an ID")
	}
	if app.Status != model.OAuthApplicationStatusActive {
		t.Fatalf("expected default active status, got %q", app.Status)
	}

	apps, err := st.ListOAuthApplications(ctx, organization.ID)
	if err != nil {
		t.Fatalf("ListOAuthApplications() error = %v", err)
	}
	if len(apps) != 1 || apps[0].ID != app.ID {
		t.Fatalf("unexpected app list: %#v", apps)
	}

	got, err := st.GetOAuthApplication(ctx, organization.ID, app.ID)
	if err != nil {
		t.Fatalf("GetOAuthApplication() error = %v", err)
	}
	got.Name = "Updated GitHub Login"
	got.Status = model.OAuthApplicationStatusDisabled
	if err := st.UpdateOAuthApplication(ctx, got); err != nil {
		t.Fatalf("UpdateOAuthApplication() error = %v", err)
	}
	got, err = st.GetOAuthApplication(ctx, organization.ID, app.ID)
	if err != nil {
		t.Fatalf("GetOAuthApplication() after update error = %v", err)
	}
	if got.Name != "Updated GitHub Login" || got.Status != model.OAuthApplicationStatusDisabled {
		t.Fatalf("unexpected updated app: %#v", got)
	}

	if err := st.DeleteOAuthApplication(ctx, organization.ID, app.ID); err != nil {
		t.Fatalf("DeleteOAuthApplication() error = %v", err)
	}
	if _, err := st.GetOAuthApplication(ctx, organization.ID, app.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	var deleted model.OAuthApplication
	if err := db.Unscoped().First(&deleted, "id = ?", app.ID).Error; err != nil {
		t.Fatalf("expected soft-deleted OAuth application to remain in unscoped query: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatalf("expected OAuth application DeletedAt to be set")
	}
	if deleted.Status != model.OAuthApplicationStatusDeleted {
		t.Fatalf("expected deleted OAuth application status %q, got %q", model.OAuthApplicationStatusDeleted, deleted.Status)
	}
	apps, err = st.ListOAuthApplications(ctx, organization.ID)
	if err != nil {
		t.Fatalf("ListOAuthApplications() after delete error = %v", err)
	}
	if len(apps) != 0 {
		t.Fatalf("expected deleted app to be hidden, got %#v", apps)
	}
	if err := st.DeleteOAuthApplication(ctx, organization.ID, app.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting missing app, got %v", err)
	}
}
