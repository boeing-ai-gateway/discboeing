package meta

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/authz"
	"github.com/obot-platform/discobot/meta/internal/config"
	"github.com/obot-platform/discobot/meta/internal/dbcrypt"
	"github.com/obot-platform/discobot/meta/internal/handlers"
	"github.com/obot-platform/discobot/meta/internal/jwtkeys"
	"github.com/obot-platform/discobot/meta/internal/routes"
	"github.com/obot-platform/discobot/meta/internal/store"
)

func newRouter(cfg *config.Config, st *store.Store, signingKeyStore *jwtkeys.PersistentSigningKeyStore, dbEncryptor dbcrypt.Encryptor) http.Handler {
	r := chi.NewRouter()
	r.Use(auth.Middleware(auth.BootstrapAuthenticator{Store: st}))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs", http.StatusFound)
	})
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	authorizer := authz.NewMetaAuthorizer(st)
	routeHandlers := handlers.New(handlers.Options{Config: cfg, SigningKeyStore: signingKeyStore, Store: st, DatabaseEncryptor: dbEncryptor})
	routes.RegisterGeneratedWithWrapper(r, routeHandlers, func(_ routes.Route, handler http.HandlerFunc) http.HandlerFunc {
		return authz.Protect(authorizer, handler)
	})

	return r
}
