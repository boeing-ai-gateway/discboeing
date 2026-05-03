package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/routes"
)

func TestBootstrapAuthorizerGrantsOAuthApplicationInBootstrapOrganization(t *testing.T) {
	authorizer := BootstrapAuthorizer{}
	info := RequestInfo{
		Action: ActionOAuthApplicationCreate,
		Params: map[string]string{"organizationDomain": "public"},
		User: &auth.UserInfo{
			Name:   "bootstrap:public",
			Groups: []string{auth.GroupAuthenticated, "bootstrap"},
			Extra: map[string][]string{
				"principal.type":      {"bootstrap"},
				"organization.domain": {"public"},
			},
		},
	}

	if !authorizer.Authorize(context.Background(), info) {
		t.Fatal("expected bootstrap principal to manage OAuth applications in its organization")
	}
}

func TestBootstrapAuthorizerRejectsOutOfScopeRequests(t *testing.T) {
	authorizer := BootstrapAuthorizer{}
	tests := []struct {
		name string
		info RequestInfo
	}{
		{name: "missing user"},
		{name: "anonymous", info: RequestInfo{User: auth.AnonymousUserInfo("missing credentials")}},
		{name: "authenticated non-bootstrap", info: RequestInfo{User: &auth.UserInfo{
			Name: "user@example.test", Groups: []string{auth.GroupAuthenticated},
		}}},
		{name: "bootstrap extra without authenticated group", info: RequestInfo{User: &auth.UserInfo{
			Name: "bootstrap:public", Extra: map[string][]string{"principal.type": {"bootstrap"}},
		}}},
		{name: "non-oauth action", info: RequestInfo{
			Action: ActionProjectCreate,
			Params: map[string]string{"organizationDomain": "public"},
			User: &auth.UserInfo{
				Name:   "bootstrap:public",
				Groups: []string{auth.GroupAuthenticated},
				Extra: map[string][]string{
					"principal.type":      {"bootstrap"},
					"organization.domain": {"public"},
				},
			},
		}},
		{name: "different organization", info: RequestInfo{
			Action: ActionOAuthApplicationCreate,
			Params: map[string]string{"organizationDomain": "other.example"},
			User: &auth.UserInfo{
				Name:   "bootstrap:public",
				Groups: []string{auth.GroupAuthenticated},
				Extra: map[string][]string{
					"principal.type":      {"bootstrap"},
					"organization.domain": {"public"},
				},
			},
		}},
		{name: "missing organization route param", info: RequestInfo{
			Action: ActionOAuthApplicationCreate,
			User: &auth.UserInfo{
				Name:   "bootstrap:public",
				Groups: []string{auth.GroupAuthenticated},
				Extra: map[string][]string{
					"principal.type":      {"bootstrap"},
					"organization.domain": {"public"},
				},
			},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if authorizer.Authorize(context.Background(), tt.info) {
				t.Fatal("expected no grant")
			}
		})
	}
}

func TestProtectAllowsBootstrapOAuthApplicationAction(t *testing.T) {
	called := false
	user := &auth.UserInfo{
		Name:   "bootstrap:public",
		Groups: []string{auth.GroupAuthenticated},
		Extra: map[string][]string{
			"principal.type":      {"bootstrap"},
			"organization.domain": {"public"},
		},
	}
	h := Protect(NewMetaAuthorizer(nil), func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/org/public/oauth-applications", nil)
	ctx := routes.WithRequestRouteInfo(req.Context(), routes.RequestRouteInfo{
		Method:      http.MethodPost,
		Pattern:     "/v1/org/{organizationDomain}/oauth-applications",
		OperationID: "createOrganizationOAuthApplication",
		PathParams:  map[string]string{"organizationDomain": "public"},
	})
	h.ServeHTTP(rec, req.WithContext(auth.WithUserInfo(ctx, user)))

	if rec.Code != http.StatusNoContent || !called {
		t.Fatalf("expected bootstrap OAuth application route to reach handler, status=%d called=%v", rec.Code, called)
	}
}
