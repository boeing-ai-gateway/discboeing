package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareStoresAnonymousUserInfo(t *testing.T) {
	h := Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		user, ok := UserInfoFromRequest(r)
		if !ok {
			t.Fatal("missing user info")
		}
		if user.Name != AnonymousName {
			t.Fatalf("name = %q", user.Name)
		}
		if user.IsAuthenticated() {
			t.Fatal("anonymous user should not be authenticated")
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestMiddlewareStoresAuthenticatedUserInfo(t *testing.T) {
	want := &UserInfo{Name: "bootstrap:public", UID: "obt_123", Groups: []string{GroupAuthenticated}}
	h := Middleware(AuthenticatorFunc(func(*http.Request) (*UserInfo, bool, error) {
		return want, true, nil
	}))(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, ok := UserInfoFromRequest(r)
		if !ok {
			t.Fatal("missing user info")
		}
		if got != want {
			t.Fatal("middleware did not store authenticator result")
		}
		if !got.IsAuthenticated() {
			t.Fatal("expected authenticated user")
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
