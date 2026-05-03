package store

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestOrganizationStoreLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Organization{}); err != nil {
		t.Fatalf("migrate organization: %v", err)
	}

	ctx := context.Background()
	st := New(db, nil)
	organization := &model.Organization{
		Name:   "Example",
		Domain: "example.com",
	}
	if err := st.CreateOrganization(ctx, organization); err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	if organization.ID == "" {
		t.Fatalf("expected CreateOrganization to run BeforeCreate and assign an ID")
	}
	if organization.Status != model.OrganizationStatusActive {
		t.Fatalf("expected status %q, got %q", model.OrganizationStatusActive, organization.Status)
	}

	byID, err := st.GetOrganizationByID(ctx, organization.ID)
	if err != nil {
		t.Fatalf("GetOrganizationByID() error = %v", err)
	}
	if byID.Domain != organization.Domain {
		t.Fatalf("expected domain %q, got %q", organization.Domain, byID.Domain)
	}

	byDomain, err := st.GetOrganizationByDomain(ctx, organization.Domain)
	if err != nil {
		t.Fatalf("GetOrganizationByDomain() error = %v", err)
	}
	if byDomain.ID != organization.ID {
		t.Fatalf("expected organization ID %q, got %q", organization.ID, byDomain.ID)
	}

	if err := st.DeleteOrganization(ctx, organization.ID); err != nil {
		t.Fatalf("DeleteOrganization() error = %v", err)
	}
	if _, err := st.GetOrganizationByID(ctx, organization.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	var deleted model.Organization
	if err := db.Unscoped().First(&deleted, "id = ?", organization.ID).Error; err != nil {
		t.Fatalf("expected soft-deleted organization to remain in unscoped query: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Fatalf("expected organization DeletedAt to be set")
	}
	if err := st.DeleteOrganization(ctx, organization.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting missing organization, got %v", err)
	}
}
