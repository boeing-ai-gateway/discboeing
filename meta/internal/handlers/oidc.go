package handlers

import (
	"log"
	"net/http"
	"strings"
)

func (h *Handlers) GetOpenIDConfiguration(w http.ResponseWriter, _ *http.Request) {
	metadata := h.oauthAuthorizationServerMetadata()
	metadata["userinfo_endpoint"] = h.issuerURL("/userinfo")
	metadata["end_session_endpoint"] = h.issuerURL("/logout")
	metadata["subject_types_supported"] = []string{"public"}
	metadata["id_token_signing_alg_values_supported"] = []string{h.Config.JWTSigning.Alg}
	metadata["claims_supported"] = []string{
		"aud",
		"email",
		"email_verified",
		"exp",
		"iat",
		"iss",
		"name",
		"picture",
		"preferred_username",
		"sub",
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (h *Handlers) GetOAuthAuthorizationMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.oauthAuthorizationServerMetadata())
}

func (h *Handlers) GetJWKS(w http.ResponseWriter, r *http.Request) {
	jwks, err := h.SigningKeyStore.PublicJWKS(r.Context(), "")
	if err != nil {
		log.Printf("failed to load JWKS: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]any{
				"code":    "jwks_unavailable",
				"message": "JWKS is unavailable",
			},
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(jwks)
}

func (h *Handlers) oauthAuthorizationServerMetadata() map[string]any {
	return map[string]any{
		"issuer":                                h.Config.JWTSigning.Issuer,
		"authorization_endpoint":                h.issuerURL("/authorize"),
		"token_endpoint":                        h.issuerURL("/token"),
		"jwks_uri":                              h.issuerURL("/.well-known/jwks.json"),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:token-exchange"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post", "none"},
		"scopes_supported":                      []string{"openid", "profile", "email", "offline_access"},
		"code_challenge_methods_supported":      []string{"S256"},
	}
}

func (h *Handlers) issuerURL(path string) string {
	issuer := strings.TrimRight(h.Config.JWTSigning.Issuer, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return issuer + path
}

func (h *Handlers) Authorize(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("authorize", w, r)
}

func (h *Handlers) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("getUserInfo", w, r)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("logout", w, r)
}

func (h *Handlers) Token(w http.ResponseWriter, r *http.Request) {
	h.NotImplemented("token", w, r)
}
