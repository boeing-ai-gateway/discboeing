package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/model"
	"github.com/obot-platform/discobot/meta/internal/services"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func TestOAuthApplicationHandlersCRUD(t *testing.T) {
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

	encryptor := newOAuthApplicationTestEncryptor(t)
	h := New(Options{Store: st, DatabaseEncryptor: encryptor})
	router := chi.NewRouter()
	router.Post("/v1/org/{organizationDomain}/oauth-applications", h.CreateOrganizationOAuthApplication)
	router.Get("/v1/org/{organizationDomain}/oauth-applications", h.ListOrganizationOAuthApplications)
	router.Get("/v1/org/{organizationDomain}/oauth-applications/{oauthApplicationId}", h.GetOrganizationOAuthApplication)
	router.Patch("/v1/org/{organizationDomain}/oauth-applications/{oauthApplicationId}", h.UpdateOrganizationOAuthApplication)
	router.Delete("/v1/org/{organizationDomain}/oauth-applications/{oauthApplicationId}", h.DeleteOrganizationOAuthApplication)

	createBody := []byte(`{
		"name":"GitHub Login",
		"provider":"github",
		"clientId":"github-client",
		"clientSecret":"secret",
		"redirectUris":["https://example.com/callback"],
		"scopes":["read:user","user:email"],
		"github":{"enterpriseBaseURL":"https://github.example.com"}
	}`)
	createResp := doOAuthApplicationHandlerRequest(t, router, http.MethodPost, "/v1/org/example.com/oauth-applications", createBody)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created services.OAuthApplication
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.Provider != model.OAuthApplicationProviderGitHub || !created.HasClientSecret {
		t.Fatalf("unexpected created app: %#v", created)
	}
	if created.CreatedByPrincipal != "bootstrap:example.com" {
		t.Fatalf("createdByPrincipal = %q", created.CreatedByPrincipal)
	}
	if got := created.ProviderConfig["enterpriseBaseURL"]; got != "https://github.example.com" {
		t.Fatalf("provider config enterpriseBaseURL = %#v", got)
	}
	stored, err := st.GetOAuthApplication(t.Context(), organization.ID, created.ID)
	if err != nil {
		t.Fatalf("GetOAuthApplication() error = %v", err)
	}
	if len(stored.ClientSecretEncrypted) == 0 {
		t.Fatalf("expected client secret to be encrypted")
	}
	if bytes.Contains(stored.ClientSecretEncrypted, []byte(`"secret"`)) {
		t.Fatalf("encrypted client secret contains plaintext")
	}
	decrypted, err := h.OAuthApplications.DecryptClientSecret(t.Context(), stored)
	if err != nil {
		t.Fatalf("decryptOAuthClientSecret() error = %v", err)
	}
	if decrypted != "secret" {
		t.Fatalf("decrypted client secret = %q", decrypted)
	}

	patchBody := []byte(`{"name":"Google Login","provider":"google","clientId":"google-client","google":{"hostedDomain":"example.com"}}`)
	updateResp := doOAuthApplicationHandlerRequest(t, router, http.MethodPatch, "/v1/org/example.com/oauth-applications/"+created.ID, patchBody)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	var updated services.OAuthApplication
	if err := json.Unmarshal(updateResp.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Name != "Google Login" || updated.Provider != model.OAuthApplicationProviderGoogle || updated.ClientID != "google-client" {
		t.Fatalf("unexpected updated app: %#v", updated)
	}
	if got := updated.ProviderConfig["hostedDomain"]; got != "example.com" {
		t.Fatalf("provider config hostedDomain = %#v", got)
	}

	listResp := doOAuthApplicationHandlerRequest(t, router, http.MethodGet, "/v1/org/example.com/oauth-applications", nil)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var list struct {
		Items []services.OAuthApplication `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("unexpected list response: %#v", list)
	}

	deleteResp := doOAuthApplicationHandlerRequest(t, router, http.MethodDelete, "/v1/org/example.com/oauth-applications/"+created.ID, nil)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	getResp := doOAuthApplicationHandlerRequest(t, router, http.MethodGet, "/v1/org/example.com/oauth-applications/"+created.ID, nil)
	if getResp.Code != http.StatusNotFound {
		t.Fatalf("get deleted status = %d, body = %s", getResp.Code, getResp.Body.String())
	}
}

func doOAuthApplicationHandlerRequest(t *testing.T, handler http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithUserInfo(req.Context(), &auth.UserInfo{
		Name:   "bootstrap:example.com",
		UID:    "obt_test",
		Groups: []string{auth.GroupAuthenticated},
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
