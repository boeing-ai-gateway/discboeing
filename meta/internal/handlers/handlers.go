// Package handlers contains Meta HTTP handlers keyed by OpenAPI operation ID.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/obot-platform/discobot/meta/internal/config"
	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/jwtkeys"
	"github.com/obot-platform/discobot/meta/internal/services"
	"github.com/obot-platform/discobot/meta/internal/store"
)

// Handlers holds dependencies shared by Meta HTTP handlers.
type Handlers struct {
	Config            *config.Config
	SigningKeyStore   *jwtkeys.PersistentSigningKeyStore
	Store             *store.Store
	DatabaseEncryptor dbcrypt.Encryptor
	OAuthApplications *services.OAuthApplicationService
}

// Options configures a Handlers value.
type Options struct {
	Config            *config.Config
	SigningKeyStore   *jwtkeys.PersistentSigningKeyStore
	Store             *store.Store
	DatabaseEncryptor dbcrypt.Encryptor
	OAuthApplications *services.OAuthApplicationService
}

// New creates a Meta handler set.
func New(opts Options) *Handlers {
	oauthApplications := opts.OAuthApplications
	if oauthApplications == nil {
		oauthApplications = &services.OAuthApplicationService{Store: opts.Store, DatabaseEncryptor: opts.DatabaseEncryptor}
	}
	return &Handlers{
		Config:            opts.Config,
		SigningKeyStore:   opts.SigningKeyStore,
		Store:             opts.Store,
		DatabaseEncryptor: opts.DatabaseEncryptor,
		OAuthApplications: oauthApplications,
	}
}

// NotImplemented writes the standard placeholder response for unimplemented operations.
func (h *Handlers) NotImplemented(operationID string, w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":      "not_implemented",
			"message":   "route handler is not implemented",
			"operation": operationID,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
