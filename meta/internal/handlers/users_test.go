package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obot-platform/discobot/meta/internal/auth"
)

func TestWhoamiUsesUserInfoFromContext(t *testing.T) {
	h := New(Options{})
	handler := auth.Middleware(auth.AuthenticatorFunc(func(*http.Request) (*auth.UserInfo, bool, error) {
		return &auth.UserInfo{Name: "bootstrap:public", UID: "obt_123", Groups: []string{auth.GroupAuthenticated}}, true, nil
	}))(http.HandlerFunc(h.Whoami))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		User auth.UserInfo `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.User.Name != "bootstrap:public" || body.User.UID != "obt_123" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestWhoamiReturnsAnonymousWhenUnauthenticated(t *testing.T) {
	h := New(Options{})
	handler := auth.Middleware(nil)(http.HandlerFunc(h.Whoami))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		User auth.UserInfo `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.User.Name != auth.AnonymousName || body.User.IsAuthenticated() {
		t.Fatalf("unexpected user: %#v", body.User)
	}
}
