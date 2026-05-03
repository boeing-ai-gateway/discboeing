package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/discobot/meta/internal/routes"
)

func TestProtectAllowsPublicAction(t *testing.T) {
	called := false
	h := Protect(NewMetaAuthorizer(nil), func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req = req.WithContext(routes.WithRequestRouteInfo(req.Context(), routes.RequestRouteInfo{
		Method:      http.MethodGet,
		Pattern:     "/whoami",
		OperationID: "whoami",
	}))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || !called {
		t.Fatalf("expected public route to reach handler, status=%d called=%v", rec.Code, called)
	}
}

func TestPublicAuthorizerUsesActionPolicy(t *testing.T) {
	authorizer := PublicAuthorizer{}
	if !authorizer.Authorize(context.Background(), RequestInfo{OperationID: "anything", Action: ActionUserWhoami}) {
		t.Fatal("expected public action to grant")
	}
	if authorizer.Authorize(context.Background(), RequestInfo{OperationID: "whoami", Action: ActionProjectList}) {
		t.Fatal("expected operation ID alone not to grant")
	}
}
