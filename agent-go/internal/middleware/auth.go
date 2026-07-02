package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"aidanwoods.dev/go-paseto"
)

// publicPaths are paths that do not require authentication.
var publicPaths = map[string]bool{
	"/":               true,
	"/health":         true,
	"/sudo/authorize": true,
}

const authClockSkew = 12 * time.Hour

type authenticatedContextKey struct{}

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
			if isPublicPath(r.URL.Path) {
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
			// that only know DISCBOEING_SECRET can still reach the API.
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				logAuthRejected(r, authResult{
					Reason: "missing_bearer_authorization",
					Detail: configuredAuthDetail(secretHash, trustKey),
				}, "")
				writeAuthError(w)
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			result := inspectBearerToken(token, secretHash, trustKey)
			if !result.OK {
				logAuthRejected(r, result, token)
				writeAuthError(w)
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), authenticatedContextKey{}, true))
			next.ServeHTTP(w, r)
		})
	}
}

func requestAuthenticated(r *http.Request) bool {
	authenticated, _ := r.Context().Value(authenticatedContextKey{}).(bool)
	return authenticated
}

func isPublicPath(path string) bool {
	if publicPaths[path] {
		return true
	}
	return isBrowserCDPPath(path)
}

func isBrowserCDPPath(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 4 &&
		parts[0] == "sessions" &&
		parts[1] != "" &&
		parts[2] == "browser" &&
		parts[3] == "cdp"
}

type authResult struct {
	OK     bool
	Method string
	Reason string
	Detail string
}

func inspectBearerToken(token, secretHash, trustKey string) authResult {
	if token == "" {
		return authResult{
			Reason: "empty_bearer_token",
			Detail: configuredAuthDetail(secretHash, trustKey),
		}
	}

	var details []string
	if trustKey != "" {
		ok, detail := verifyPASETOTokenDetail(token, trustKey)
		if ok {
			return authResult{OK: true, Method: "trust_key"}
		}
		details = append(details, "trust_key="+detail)
	}

	if secretHash != "" {
		if token == secretHash {
			return authResult{OK: true, Method: "legacy_secret_hash"}
		}
		ok, detail := verifySecretDetail(token, secretHash)
		if ok {
			return authResult{OK: true, Method: "legacy_secret"}
		}
		details = append(details, "legacy_secret="+detail)
	}

	if len(details) == 0 {
		details = append(details, configuredAuthDetail(secretHash, trustKey))
	}
	return authResult{
		Reason: "token_rejected",
		Detail: strings.Join(details, "; "),
	}
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
}

func logAuthRejected(r *http.Request, result authResult, token string) {
	log.Printf(
		"agent-api auth rejected: method=%s path=%s remote=%s reason=%s detail=%s token=%s now=%s",
		r.Method,
		r.URL.Path,
		r.RemoteAddr,
		result.Reason,
		result.Detail,
		tokenFingerprint(token),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
}

func configuredAuthDetail(secretHash, trustKey string) string {
	var modes []string
	if trustKey != "" {
		modes = append(modes, "trust_key")
	}
	if secretHash != "" {
		modes = append(modes, "legacy_secret")
	}
	if len(modes) == 0 {
		return "configured_modes=none"
	}
	return "configured_modes=" + strings.Join(modes, ",")
}

func tokenFingerprint(token string) string {
	if token == "" {
		return "empty"
	}
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("sha256:%s len=%d", hex.EncodeToString(sum[:6]), len(token))
}

// verifySecret checks a plaintext token against a salted SHA-256 hash.
// The secretHash format is "saltHex:hashHex" where hash = SHA-256(salt + plaintext).
func verifySecret(token, secretHash string) bool {
	ok, _ := verifySecretDetail(token, secretHash)
	return ok
}

func verifySecretDetail(token, secretHash string) (bool, string) {
	parts := strings.SplitN(secretHash, ":", 2)
	if len(parts) != 2 {
		return false, "invalid_secret_hash_format"
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false, "invalid_secret_hash_salt"
	}
	expectedHash := parts[1]

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(token))
	computedHash := hex.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(computedHash), []byte(expectedHash)) {
		return false, "hash_mismatch"
	}
	return true, "matched"
}

func verifyPASETOTokenDetail(token, trustKey string) (bool, string) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(trustKey)
	if err != nil {
		return false, "invalid_trust_key_base64"
	}
	publicKey, err := paseto.NewV4AsymmetricPublicKeyFromBytes(publicKeyBytes)
	if err != nil {
		return false, "invalid_trust_key"
	}
	_, err = paseto.MakeParser([]paseto.Rule{validAtWithClockSkew(time.Now().UTC(), authClockSkew)}).ParseV4Public(publicKey, token, nil)
	if err != nil {
		return false, diagnosePASETOTokenFailure(publicKey, token, err)
	}
	return true, "matched"
}

func validAtWithClockSkew(now time.Time, skew time.Duration) paseto.Rule {
	return func(token paseto.Token) error {
		issuedAt, err := token.GetIssuedAt()
		if err != nil {
			return err
		}
		if now.Add(skew).Before(issuedAt) {
			return fmt.Errorf("the ValidAt time plus clock skew is before this token was issued")
		}

		notBefore, err := token.GetNotBefore()
		if err != nil {
			return err
		}
		if now.Add(skew).Before(notBefore) {
			return fmt.Errorf("the ValidAt time plus clock skew is before this token's not before time")
		}

		expiresAt, err := token.GetExpiration()
		if err != nil {
			return err
		}
		if now.Add(-skew).After(expiresAt) {
			return fmt.Errorf("the ValidAt time minus clock skew is after this token expires")
		}

		return nil
	}
}

func diagnosePASETOTokenFailure(publicKey paseto.V4AsymmetricPublicKey, token string, validationErr error) string {
	parsedToken, err := paseto.NewParserWithoutExpiryCheck().ParseV4Public(publicKey, token, nil)
	if err != nil {
		return "token_parse_or_signature_failed: " + err.Error()
	}

	now := time.Now().UTC()
	issuedAt, err := parsedToken.GetIssuedAt()
	if err != nil {
		return "token_validation_failed: invalid_iat: " + err.Error()
	}
	issuedAt = issuedAt.UTC()
	if now.Add(authClockSkew).Before(issuedAt) {
		return fmt.Sprintf(
			"token_issued_in_future iat=%s now=%s skew=%s tolerance=%s",
			formatAuthTime(issuedAt),
			formatAuthTime(now),
			issuedAt.Sub(now).Round(time.Second),
			authClockSkew,
		)
	}

	notBefore, err := parsedToken.GetNotBefore()
	if err != nil {
		return "token_validation_failed: invalid_nbf: " + err.Error()
	}
	notBefore = notBefore.UTC()
	if now.Add(authClockSkew).Before(notBefore) {
		return fmt.Sprintf(
			"token_not_yet_valid nbf=%s now=%s skew=%s tolerance=%s",
			formatAuthTime(notBefore),
			formatAuthTime(now),
			notBefore.Sub(now).Round(time.Second),
			authClockSkew,
		)
	}

	expiresAt, err := parsedToken.GetExpiration()
	if err != nil {
		return "token_validation_failed: invalid_exp: " + err.Error()
	}
	expiresAt = expiresAt.UTC()
	if now.Add(-authClockSkew).After(expiresAt) {
		return fmt.Sprintf(
			"token_expired exp=%s now=%s age=%s tolerance=%s",
			formatAuthTime(expiresAt),
			formatAuthTime(now),
			now.Sub(expiresAt).Round(time.Second),
			authClockSkew,
		)
	}

	return "token_validation_failed: " + validationErr.Error()
}

func formatAuthTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
