package store

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/model"
)

func TestOrganizationBootstrapTokenStore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Organization{}, &model.OrganizationBootstrapToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	st := New(db, nil)
	org := &model.Organization{Name: "Public", Domain: model.PublicOrganizationDomain}
	if err := st.CreateOrganization(ctx, org); err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	token := &model.OrganizationBootstrapToken{OrganizationID: org.ID, TokenHash: "hash"}
	if err := st.CreateOrganizationBootstrapToken(ctx, token); err != nil {
		t.Fatalf("CreateOrganizationBootstrapToken() error = %v", err)
	}
	if token.ID == "" {
		t.Fatal("expected token ID")
	}

	got, err := st.GetActiveOrganizationBootstrapTokenByHash(ctx, "hash", time.Now())
	if err != nil {
		t.Fatalf("GetActiveOrganizationBootstrapTokenByHash() error = %v", err)
	}
	if got.OrganizationID != org.ID {
		t.Fatalf("organization ID = %q, want %q", got.OrganizationID, org.ID)
	}

	tokens, err := st.ListActiveOrganizationBootstrapTokens(ctx, org.ID, time.Now())
	if err != nil {
		t.Fatalf("ListActiveOrganizationBootstrapTokens() error = %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 active token, got %d", len(tokens))
	}

	now := time.Now()
	if err := st.MarkOrganizationBootstrapTokenUsed(ctx, token.ID, now); err != nil {
		t.Fatalf("MarkOrganizationBootstrapTokenUsed() error = %v", err)
	}
}
