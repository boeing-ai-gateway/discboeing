package meta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func TestEnsurePublicOrganizationBootstrapRotatesUntilInitialized(t *testing.T) {
	st := newBootstrapTestStore(t)

	first, err := ensurePublicOrganizationBootstrap(context.Background(), st)
	if err != nil {
		t.Fatalf("ensurePublicOrganizationBootstrap() error = %v", err)
	}
	if first.Organization.Domain != model.PublicOrganizationDomain {
		t.Fatalf("domain = %q", first.Organization.Domain)
	}
	if !first.SetupMode || !first.CreatedToken || !strings.HasPrefix(first.Token, auth.BootstrapTokenPrefix) {
		t.Fatalf("expected setup mode with created token, got %#v", first)
	}
	if strings.ContainsAny(strings.TrimPrefix(first.Token, auth.BootstrapTokenPrefix), "-_") {
		t.Fatalf("bootstrap token suffix should use Crockford alphabet without separators: %q", first.Token)
	}

	principal, err := authenticateBootstrapTest(st, first.Token)
	if err != nil {
		t.Fatalf("authenticateBootstrap() error = %v", err)
	}
	if principal.Name != "bootstrap:"+model.PublicOrganizationDomain || principal.Extra["organization.domain"][0] != model.PublicOrganizationDomain {
		t.Fatalf("unexpected principal: %#v", principal)
	}

	second, err := ensurePublicOrganizationBootstrap(context.Background(), st)
	if err != nil {
		t.Fatalf("second ensurePublicOrganizationBootstrap() error = %v", err)
	}
	if !second.SetupMode || !second.CreatedToken || !strings.HasPrefix(second.Token, auth.BootstrapTokenPrefix) {
		t.Fatalf("expected regenerated setup token, got %#v", second)
	}
	if second.Token == first.Token {
		t.Fatal("expected regenerated token to differ from first token")
	}
	if _, err := authenticateBootstrapTest(st, first.Token); err == nil {
		t.Fatal("expected first token to be revoked after regeneration")
	}
	if _, err := authenticateBootstrapTest(st, second.Token); err != nil {
		t.Fatalf("expected second token to authenticate: %v", err)
	}
}

func TestEnsurePublicOrganizationBootstrapSkipsWhenInitialized(t *testing.T) {
	st := newBootstrapTestStore(t)
	result, err := ensurePublicOrganizationBootstrap(context.Background(), st)
	if err != nil {
		t.Fatalf("ensurePublicOrganizationBootstrap() error = %v", err)
	}
	user := &model.User{PrimaryEmail: "owner@example.com", EmailVerified: true}
	if err := st.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	member := &model.OrganizationMember{OrganizationID: result.Organization.ID, UserID: user.ID, Role: model.OrganizationRoleOwner}
	if err := st.CreateOrganizationMember(context.Background(), member); err != nil {
		t.Fatalf("CreateOrganizationMember() error = %v", err)
	}

	initialized, err := ensurePublicOrganizationBootstrap(context.Background(), st)
	if err != nil {
		t.Fatalf("initialized ensurePublicOrganizationBootstrap() error = %v", err)
	}
	if initialized.SetupMode || initialized.CreatedToken || initialized.Token != "" {
		t.Fatalf("expected initialized org to skip setup token, got %#v", initialized)
	}

	tokens, err := st.ListActiveOrganizationBootstrapTokens(context.Background(), result.Organization.ID, time.Now())
	if err != nil {
		t.Fatalf("ListActiveOrganizationBootstrapTokens() error = %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected active bootstrap tokens to be revoked after initialization, got %d", len(tokens))
	}
}

func newBootstrapTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.New(db, nil)
}

func authenticateBootstrapTest(st *store.Store, token string) (*auth.UserInfo, error) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	user, _, err := auth.BootstrapAuthenticator{Store: st}.Authenticate(req)
	return user, err
}
