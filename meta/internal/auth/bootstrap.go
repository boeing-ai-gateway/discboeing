package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/obot-platform/discobot/meta/internal/id"
	"github.com/obot-platform/discobot/meta/internal/store"
)

const BootstrapTokenPrefix = "mboot_"

// NewBootstrapToken creates a raw setup token and its deterministic lookup hash.
func NewBootstrapToken() (raw string, hash string, err error) {
	// 52 base32 characters provide 260 bits of entropy. The alphabet intentionally
	// excludes dashes/underscores so mboot_ is the only separator in the token.
	random, err := id.RandomCrockford(52)
	if err != nil {
		return "", "", fmt.Errorf("generate bootstrap token: %w", err)
	}
	raw = BootstrapTokenPrefix + random
	return raw, HashBootstrapToken(raw), nil
}

// HashBootstrapToken returns a deterministic lookup hash for a bootstrap token.
//
// Bootstrap tokens are generated with 260 bits of cryptographic randomness, so a
// fast SHA-256 hash is appropriate here. This is different from password hashing:
// there is no low-entropy user-chosen secret to slow down with bcrypt/argon2, and
// Meta never needs to recover the raw token after printing it once.
func HashBootstrapToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// BootstrapAuthenticator authenticates setup-mode organization bootstrap tokens.
type BootstrapAuthenticator struct {
	Store *store.Store
}

// Authenticate resolves mboot_ bearer tokens into bootstrap principals.
func (a BootstrapAuthenticator) Authenticate(r *http.Request) (*UserInfo, bool, error) {
	rawToken := BearerToken(r)
	if rawToken == "" || !strings.HasPrefix(rawToken, BootstrapTokenPrefix) {
		return nil, false, nil
	}
	if a.Store == nil {
		return nil, true, errors.New("bootstrap authenticator store is nil")
	}
	token, err := a.Store.GetActiveOrganizationBootstrapTokenByHash(r.Context(), HashBootstrapToken(rawToken), time.Now())
	if err != nil {
		return nil, true, err
	}
	org, err := a.Store.GetOrganizationByID(r.Context(), token.OrganizationID)
	if err != nil {
		return nil, true, err
	}
	if err := a.Store.MarkOrganizationBootstrapTokenUsed(r.Context(), token.ID, time.Now()); err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, true, err
	}
	return &UserInfo{
		Name:   "bootstrap:" + org.Domain,
		UID:    token.ID,
		Groups: []string{GroupAuthenticated, "bootstrap", "organization:" + org.Domain + ":bootstrap"},
		Extra: map[string][]string{
			"principal.type":      {"bootstrap"},
			"organization.id":     {org.ID},
			"organization.domain": {org.Domain},
			"role":                {"bootstrap"},
			"token.id":            {token.ID},
		},
	}, true, nil
}
