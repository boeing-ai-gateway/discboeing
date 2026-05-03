package meta

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/meta/internal/config"
	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/jwtkeys"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func TestMetadataEndpoints(t *testing.T) {
	cfg := metadataTestConfig()
	st := newBootstrapTestStore(t)
	signingKeyStore := newMetadataTestSigningKeyStore(t, st)
	router := newRouter(cfg, st, signingKeyStore, nil)

	tests := []struct {
		path string
		want map[string]any
	}{
		{
			path: "/.well-known/openid-configuration",
			want: map[string]any{
				"issuer":                 cfg.JWTSigning.Issuer,
				"authorization_endpoint": metadataIssuerURL(cfg, "/authorize"),
				"token_endpoint":         metadataIssuerURL(cfg, "/token"),
				"jwks_uri":               metadataIssuerURL(cfg, "/.well-known/jwks.json"),
				"userinfo_endpoint":      metadataIssuerURL(cfg, "/userinfo"),
			},
		},
		{
			path: "/.well-known/oauth-authorization-server",
			want: map[string]any{
				"issuer":                 cfg.JWTSigning.Issuer,
				"authorization_endpoint": metadataIssuerURL(cfg, "/authorize"),
				"token_endpoint":         metadataIssuerURL(cfg, "/token"),
				"jwks_uri":               metadataIssuerURL(cfg, "/.well-known/jwks.json"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			for key, want := range tt.want {
				if got[key] != want {
					t.Fatalf("%s = %#v, want %#v", key, got[key], want)
				}
			}
		})
	}
}

func TestJWKSEndpoint(t *testing.T) {
	cfg := metadataTestConfig()
	st := newBootstrapTestStore(t)
	signingKeyStore := newMetadataTestSigningKeyStore(t, st)
	router := newRouter(cfg, st, signingKeyStore, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
		t.Fatal(err)
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected one JWKS key, got %s", rec.Body.String())
	}
	if jwks.Keys[0]["alg"] != jwtkeys.AlgorithmES256 || jwks.Keys[0]["use"] != "sig" {
		t.Fatalf("unexpected JWK: %#v", jwks.Keys[0])
	}
}

func metadataTestConfig() *config.Config {
	return &config.Config{
		CORSOrigins: []string{"*"},
		JWTSigning: config.JWTSigningConfig{
			Issuer:              "https://meta.example.test",
			Alg:                 jwtkeys.AlgorithmES256,
			RotationInterval:    72 * time.Hour,
			PrepublishWindow:    24 * time.Hour,
			VerificationOverlap: 7 * 24 * time.Hour,
		},
	}
}

func metadataIssuerURL(cfg *config.Config, path string) string {
	issuer := strings.TrimRight(cfg.JWTSigning.Issuer, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return issuer + path
}

func newMetadataTestSigningKeyStore(t *testing.T, st *store.Store) *jwtkeys.PersistentSigningKeyStore {
	t.Helper()
	encryptor, err := dbcrypt.NewLocalEncryptor("test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	signingKeyStore := jwtkeys.NewPersistentSigningKeyStore(st, encryptor, jwtkeys.PersistentSigningKeyStoreOptions{
		Backend:   jwtkeys.BackendDBLocal,
		Algorithm: jwtkeys.AlgorithmES256,
	})
	if err := signingKeyStore.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	return signingKeyStore
}
