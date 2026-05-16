package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"aidanwoods.dev/go-paseto"
)

// publicPaths are paths that do not require authentication.
var publicPaths = map[string]bool{
	"/":               true,
	"/health":         true,
	"/sudo/authorize": true,
}

// Auth returns middleware that validates Bearer tokens against a trusted public
// key or legacy shared secret hash. If both are empty, auth is disabled.
func Auth(secretHash, trustKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secretHash == "" && trustKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for public paths
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for service HTTP proxy (service handles its own auth)
			if strings.HasPrefix(r.URL.Path, "/services/") && strings.Contains(r.URL.Path, "/http/") {
				next.ServeHTTP(w, r)
				return
			}

			// Validate bearer token. For local in-sandbox clients, also accept the
			// configured hashed secret value directly as a bearer token so callers
			// that only know DISCOBOT_SECRET can still reach the API.
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeAuthError(w)
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if !verifyBearerToken(token, secretHash, trustKey) {
				writeAuthError(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func verifyBearerToken(token, secretHash, trustKey string) bool {
	if trustKey != "" && verifyPASETOToken(token, trustKey) {
		return true
	}
	return secretHash != "" && (token == secretHash || verifySecret(token, secretHash))
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
}

// verifySecret checks a plaintext token against a salted SHA-256 hash.
// The secretHash format is "saltHex:hashHex" where hash = SHA-256(salt + plaintext).
func verifySecret(token, secretHash string) bool {
	parts := strings.SplitN(secretHash, ":", 2)
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedHash := parts[1]

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(token))
	computedHash := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(computedHash), []byte(expectedHash))
}

func verifyPASETOToken(token, trustKey string) bool {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(trustKey)
	if err != nil {
		return false
	}
	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromBytes(publicKeyBytes)
	if err != nil {
		return false
	}
	_, err = paseto.NewParserForValidNow().ParseV4Public(publicKey, token, nil)
	return err == nil
}
