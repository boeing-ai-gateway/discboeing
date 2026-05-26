package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aidanwoods.dev/go-paseto"
)

func TestVerifySecret(t *testing.T) {
	// Create a known salt:hash pair.
	salt := []byte("testsalt12345678")
	saltHex := hex.EncodeToString(salt)
	plaintext := "my-secret-token"

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(plaintext))
	hashHex := hex.EncodeToString(h.Sum(nil))

	secretHash := saltHex + ":" + hashHex

	tests := []struct {
		name   string
		token  string
		hash   string
		expect bool
	}{
		{"valid token", plaintext, secretHash, true},
		{"wrong token", "wrong-token", secretHash, false},
		{"empty token", "", secretHash, false},
		{"empty hash", plaintext, "", false},
		{"no colon", plaintext, "nocolon", false},
		{"bad salt hex", plaintext, "zzzz:" + hashHex, false},
		{"wrong hash", plaintext, saltHex + ":0000000000000000000000000000000000000000000000000000000000000000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifySecret(tt.token, tt.hash)
			if got != tt.expect {
				t.Errorf("verifySecret(%q, %q) = %v, want %v", tt.token, tt.hash, got, tt.expect)
			}
		})
	}
}

func TestAuthAcceptsAuthorizationHeader(t *testing.T) {
	salt := []byte("testsalt12345678")
	plaintext := "my-secret-token"

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(plaintext))
	secretHash := hex.EncodeToString(salt) + ":" + hex.EncodeToString(h.Sum(nil))

	called := false
	handler := Auth(secretHash, "")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/threads", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}

func TestAuthSkipsSudoAuthorize(t *testing.T) {
	called := false
	handler := Auth("salt:hash", "invalid-trust-key")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/sudo/authorize", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}

func TestAuthStillRequiresTokenForOtherRoutes(t *testing.T) {
	called := false
	handler := Auth("salt:hash", "invalid-trust-key")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/exec", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("wrapped handler should not be called")
	}
}

func TestAuthAcceptsSignedTrustKeyToken(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pasetoKey, err := paseto.NewV4AsymmetricSecretKeyFromEd25519(privateKey)
	if err != nil {
		t.Fatalf("failed to convert key: %v", err)
	}
	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now)
	token.SetNotBefore(now.Add(-time.Second))
	token.SetExpiration(now.Add(time.Minute))
	tokenText := token.V4Sign(pasetoKey, nil)

	called := false
	handler := Auth("", base64.StdEncoding.EncodeToString(publicKey))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/threads", nil)
	req.Header.Set("Authorization", "Bearer "+tokenText)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("expected wrapped handler to be called")
	}
}

func TestInspectBearerTokenReportsTrustKeyValidationFailure(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pasetoKey, err := paseto.NewV4AsymmetricSecretKeyFromEd25519(privateKey)
	if err != nil {
		t.Fatalf("failed to convert key: %v", err)
	}

	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now.Add(-2 * time.Minute))
	token.SetNotBefore(now.Add(-2 * time.Minute))
	token.SetExpiration(now.Add(-time.Minute))
	tokenText := token.V4Sign(pasetoKey, nil)

	result := inspectBearerToken(tokenText, "", base64.StdEncoding.EncodeToString(publicKey))
	if result.OK {
		t.Fatal("expected expired token to be rejected")
	}
	if result.Reason != "token_rejected" {
		t.Fatalf("reason = %q, want token_rejected", result.Reason)
	}
	if !strings.Contains(result.Detail, "trust_key=token_expired") {
		t.Fatalf("detail = %q, want expired token detail", result.Detail)
	}
}

func TestInspectBearerTokenReportsNotYetValidToken(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	pasetoKey, err := paseto.NewV4AsymmetricSecretKeyFromEd25519(privateKey)
	if err != nil {
		t.Fatalf("failed to convert key: %v", err)
	}

	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now)
	token.SetNotBefore(now.Add(time.Minute))
	token.SetExpiration(now.Add(2 * time.Minute))
	tokenText := token.V4Sign(pasetoKey, nil)

	result := inspectBearerToken(tokenText, "", base64.StdEncoding.EncodeToString(publicKey))
	if result.OK {
		t.Fatal("expected not-yet-valid token to be rejected")
	}
	if !strings.Contains(result.Detail, "trust_key=token_not_yet_valid") {
		t.Fatalf("detail = %q, want not-yet-valid token detail", result.Detail)
	}
}

func TestInspectBearerTokenReportsTrustKeyMismatch(t *testing.T) {
	configuredPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate configured key: %v", err)
	}
	_, tokenPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate token key: %v", err)
	}
	pasetoKey, err := paseto.NewV4AsymmetricSecretKeyFromEd25519(tokenPrivateKey)
	if err != nil {
		t.Fatalf("failed to convert key: %v", err)
	}

	token := paseto.NewToken()
	now := time.Now()
	token.SetIssuedAt(now)
	token.SetNotBefore(now.Add(-time.Second))
	token.SetExpiration(now.Add(time.Minute))
	tokenText := token.V4Sign(pasetoKey, nil)

	result := inspectBearerToken(tokenText, "", base64.StdEncoding.EncodeToString(configuredPublicKey))
	if result.OK {
		t.Fatal("expected token signed by another key to be rejected")
	}
	if !strings.Contains(result.Detail, "trust_key=token_parse_or_signature_failed") {
		t.Fatalf("detail = %q, want trust key mismatch detail", result.Detail)
	}
}

func TestInspectBearerTokenReportsLegacySecretMismatch(t *testing.T) {
	salt := []byte("testsalt12345678")
	plaintext := "my-secret-token"

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(plaintext))
	secretHash := hex.EncodeToString(salt) + ":" + hex.EncodeToString(h.Sum(nil))

	result := inspectBearerToken("wrong-token", secretHash, "")
	if result.OK {
		t.Fatal("expected wrong token to be rejected")
	}
	if result.Detail != "legacy_secret=hash_mismatch" {
		t.Fatalf("detail = %q, want legacy_secret=hash_mismatch", result.Detail)
	}
}
