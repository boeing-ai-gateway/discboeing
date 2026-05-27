package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	credentialstore "github.com/obot-platform/discobot/agent-go/internal/credentials"
)

func TestCredentialsRequiresAuthenticatedRequest(t *testing.T) {
	mgr := credentialstore.NewManager()
	notified := false
	handler := Credentials(mgr, func() { notified = true })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/threads", nil)
	req.Header.Set(credentialsHeader, `[{"envVar":"OPENAI_API_KEY","value":"secret","provider":"openai","authType":"api_key"}]`)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if notified {
		t.Fatal("credentials callback should not run without authenticated request")
	}
	if got := mgr.Get("OPENAI_API_KEY"); got != nil {
		t.Fatalf("credential was applied without authentication: %#v", got)
	}
}

func TestCredentialsSkipsPublicPathWithoutAuthentication(t *testing.T) {
	mgr := credentialstore.NewManager()
	notified := false
	handler := Auth(testSecretHash("server-secret"), "")(Credentials(mgr, func() { notified = true })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "/sessions/session-1/browser/cdp?token=browser-token", nil)
	req.Header.Set(credentialsHeader, `[{"envVar":"OPENAI_API_KEY","value":"secret","provider":"openai","authType":"api_key"}]`)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if notified {
		t.Fatal("credentials callback should not run for public unauthenticated path")
	}
	if got := mgr.Get("OPENAI_API_KEY"); got != nil {
		t.Fatalf("credential was applied for public unauthenticated path: %#v", got)
	}
}

func TestCredentialsAppliesForAuthenticatedRoutes(t *testing.T) {
	mgr := credentialstore.NewManager()
	notified := false
	handler := Auth(testSecretHash("server-secret"), "")(Credentials(mgr, func() { notified = true })(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodGet, "/threads", nil)
	req.Header.Set("Authorization", "Bearer server-secret")
	req.Header.Set(credentialsHeader, `[{"envVar":"OPENAI_API_KEY","value":"secret","provider":"openai","authType":"api_key"}]`)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !notified {
		t.Fatal("expected credentials callback to run")
	}
	if got := mgr.Get("OPENAI_API_KEY"); got == nil || got.Value != "secret" {
		t.Fatalf("credential was not applied, got %#v", got)
	}
}

func testSecretHash(secret string) string {
	salt := []byte("testsalt12345678")
	hash := sha256.New()
	hash.Write(salt)
	hash.Write([]byte(secret))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash.Sum(nil))
}
