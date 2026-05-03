package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/routes"
)

func TestProtectRejectsUnauthenticatedProtectedAction(t *testing.T) {
	h := Protect(NewMetaAuthorizer(nil), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	ctx := routes.WithRequestRouteInfo(req.Context(), routes.RequestRouteInfo{
		Method:      http.MethodGet,
		Pattern:     "/v1/projects",
		OperationID: "listProjects",
	})
	h.ServeHTTP(rec, req.WithContext(auth.WithUserInfo(ctx, auth.AnonymousUserInfo("missing credentials"))))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code   string `json:"code"`
			Action Action `json:"action"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "unauthorized" || body.Error.Action != ActionProjectList {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestProtectRejectsAuthenticatedProtectedActionByDefault(t *testing.T) {
	user := &auth.UserInfo{Name: "user@example.test", Groups: []string{auth.GroupAuthenticated}}
	h := Protect(NewMetaAuthorizer(nil), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	ctx := routes.WithRequestRouteInfo(req.Context(), routes.RequestRouteInfo{
		Method:      http.MethodGet,
		Pattern:     "/v1/projects",
		OperationID: "listProjects",
	})
	h.ServeHTTP(rec, req.WithContext(auth.WithUserInfo(ctx, user)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAuthorizersShortCircuitOnGrant(t *testing.T) {
	calls := 0
	authorizers := Authorizers{
		AuthorizerFunc(func(context.Context, RequestInfo) bool {
			calls++
			return false
		}),
		AuthorizerFunc(func(context.Context, RequestInfo) bool {
			calls++
			return true
		}),
		AuthorizerFunc(func(context.Context, RequestInfo) bool {
			calls++
			return true
		}),
	}

	if !authorizers.Authorize(context.Background(), RequestInfo{}) {
		t.Fatal("expected authorization grant")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestProtectStoresActionForAuthorizedRoute(t *testing.T) {
	route := routes.Route{Method: http.MethodGet, Pattern: "/v1/projects/{projectId}", OperationID: "getProject", Meta: routes.Meta{
		Params: []routes.Param{{Name: "scope", In: "query"}},
	}}
	r := chi.NewRouter()
	authorizer := AuthorizerFunc(func(_ context.Context, info RequestInfo) bool {
		if info.Params["projectId"] != "prj_123" {
			t.Fatalf("projectId param = %q", info.Params["projectId"])
		}
		if info.Query.Get("scope") != "agent.chat" {
			t.Fatalf("scope query = %q", info.Query.Get("scope"))
		}
		return true
	})
	routes.NewRegistry().Register(r, routes.Route{
		Method:      route.Method,
		Pattern:     route.Pattern,
		OperationID: route.OperationID,
		Meta:        route.Meta,
		Handler: Protect(authorizer, func(w http.ResponseWriter, req *http.Request) {
			action, ok := ActionFromContext(req.Context())
			if !ok || action != ActionProjectRead {
				t.Fatalf("action = %q, %v", action, ok)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj_123?scope=agent.chat", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}
