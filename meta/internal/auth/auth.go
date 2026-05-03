// Package auth provides request authentication context for Meta.
package auth

import (
	"context"
	"net/http"
	"strings"
)

const (
	GroupAuthenticated   = "system:authenticated"
	GroupUnauthenticated = "system:unauthenticated"
	AnonymousName        = "system:anonymous"
)

type contextKey string

const userInfoKey contextKey = "userInfo"

// Authenticator resolves request credentials into a request identity.
//
// The handled return value reports whether this authenticator recognized the
// request credentials. Returning handled=false lets later authenticators try the
// same request. Returning an error marks recognized credentials as invalid, but
// the middleware still stores an anonymous user and lets authorization decide
// whether to reject the request.
type Authenticator interface {
	Authenticate(*http.Request) (user *UserInfo, handled bool, err error)
}

// AuthenticatorFunc adapts a function into an Authenticator.
type AuthenticatorFunc func(*http.Request) (*UserInfo, bool, error)

// Authenticate calls f(r).
func (f AuthenticatorFunc) Authenticate(r *http.Request) (*UserInfo, bool, error) {
	return f(r)
}

// Chain tries authenticators in order until one handles the request.
type Chain []Authenticator

// Authenticate resolves a request with the first authenticator that handles it.
func (c Chain) Authenticate(r *http.Request) (*UserInfo, bool, error) {
	for _, authenticator := range c {
		if authenticator == nil {
			continue
		}
		user, handled, err := authenticator.Authenticate(r)
		if !handled {
			continue
		}
		return user, true, err
	}
	return nil, false, nil
}

// UserInfo is the request identity shape used by Meta.
//
// It intentionally mirrors the Kubernetes user.Info shape: a stable name, UID,
// groups, and arbitrary extra attributes. Authorization code should consume this
// from request context after authentication middleware runs.
type UserInfo struct {
	Name   string              `json:"name"`
	UID    string              `json:"uid,omitempty"`
	Groups []string            `json:"groups,omitempty"`
	Extra  map[string][]string `json:"extra,omitempty"`
}

// IsAuthenticated reports whether the user has the authenticated group.
func (u *UserInfo) IsAuthenticated() bool {
	if u == nil {
		return false
	}
	for _, group := range u.Groups {
		if group == GroupAuthenticated {
			return true
		}
	}
	return false
}

// WithUserInfo stores user info on a context.
func WithUserInfo(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, userInfoKey, user)
}

// UserInfoFromContext returns user info from a context.
func UserInfoFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(userInfoKey).(*UserInfo)
	return user, ok
}

// UserInfoFromRequest returns user info from an HTTP request.
func UserInfoFromRequest(r *http.Request) (*UserInfo, bool) {
	return UserInfoFromContext(r.Context())
}

// AnonymousUserInfo returns an unauthenticated request identity.
func AnonymousUserInfo(reason string) *UserInfo {
	extra := map[string][]string{}
	if reason != "" {
		extra["auth.reason"] = []string{reason}
	}
	return &UserInfo{
		Name:   AnonymousName,
		Groups: []string{GroupUnauthenticated},
		Extra:  extra,
	}
}

// Middleware authenticates requests and stores UserInfo in context.
//
// The middleware never rejects requests. Missing, invalid, or unsupported
// credentials resolve to an unauthenticated UserInfo. Authorization middleware is
// responsible for turning that into 401/403 decisions for protected actions.
func Middleware(authenticators ...Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := AnonymousUserInfo("missing credentials")
			if authenticated, handled, err := Chain(authenticators).Authenticate(r); err != nil {
				user = AnonymousUserInfo("invalid credentials")
			} else if authenticated != nil {
				user = authenticated
			} else if handled {
				user = AnonymousUserInfo("invalid credentials")
			}
			next.ServeHTTP(w, r.WithContext(WithUserInfo(r.Context(), user)))
		})
	}
}

// BearerToken returns the RFC 6750 bearer token from the Authorization header.
func BearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[len("Bearer "):])
}
