package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
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
	handler := Auth(secretHash)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
