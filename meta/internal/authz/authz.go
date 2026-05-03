// Package authz provides router-level authorization for Meta routes.
package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/obot-platform/discobot/meta/internal/auth"
	"github.com/obot-platform/discobot/meta/internal/routes"
)

// Action names the concrete operation being attempted by a route.
type Action string

type contextKey string

const actionKey contextKey = "action"

// RequestInfo is the router-derived authorization input for a request.
type RequestInfo struct {
	Method      string
	Path        string
	Pattern     string
	OperationID string
	Action      Action
	Params      map[string]string
	Query       url.Values
	User        *auth.UserInfo
}

// Authorizer grants access to a request. Returning false does not deny the
// request directly; it lets the next authorizer decide. If no authorizer grants
// access, Protect denies the request by default.
type Authorizer interface {
	Authorize(context.Context, RequestInfo) bool
}

// AuthorizerFunc adapts a function into an Authorizer.
type AuthorizerFunc func(context.Context, RequestInfo) bool

// Authorize calls f(ctx, info).
func (f AuthorizerFunc) Authorize(ctx context.Context, info RequestInfo) bool {
	return f(ctx, info)
}

// Authorizers evaluates grant-only authorizers in order.
type Authorizers []Authorizer

// Authorize returns true as soon as any authorizer grants access.
func (a Authorizers) Authorize(ctx context.Context, info RequestInfo) bool {
	for _, authorizer := range a {
		if authorizer != nil && authorizer.Authorize(ctx, info) {
			return true
		}
	}
	return false
}

// Protect wraps a generated route handler with operation-aware authorization.
func Protect(authorizer Authorizer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routeInfo, ok := routes.RequestRouteInfoFromRequest(r)
		if !ok {
			writeDenied(w, RequestInfo{Method: r.Method, Path: r.URL.Path}, http.StatusInternalServerError, "route info missing from request context")
			return
		}
		info := RequestInfo{
			Method:      r.Method,
			Path:        r.URL.Path,
			Pattern:     routeInfo.Pattern,
			OperationID: routeInfo.OperationID,
			Action:      ActionForOperation(routeInfo.OperationID),
			Params:      routeInfo.PathParams,
			Query:       routeInfo.QueryParams,
		}
		info.User, _ = auth.UserInfoFromRequest(r)

		if authorizer == nil || !authorizer.Authorize(r.Context(), info) {
			writeDenied(w, info, defaultDenyStatus(info), "authorization denied")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithAction(r.Context(), info.Action)))
	}
}

// WithAction stores the authorized action on a context for audit and handlers.
func WithAction(ctx context.Context, action Action) context.Context {
	return context.WithValue(ctx, actionKey, action)
}

// ActionFromContext returns the authorized action from a context.
func ActionFromContext(ctx context.Context) (Action, bool) {
	action, ok := ctx.Value(actionKey).(Action)
	return action, ok
}

// ActionForOperation maps generated OpenAPI operation IDs to concrete actions.
func ActionForOperation(operationID string) Action {
	if action, ok := operationActions[operationID]; ok {
		return action
	}
	return Action(operationID)
}

func defaultDenyStatus(info RequestInfo) int {
	if info.User == nil || !info.User.IsAuthenticated() {
		return http.StatusUnauthorized
	}
	return http.StatusForbidden
}

func writeDenied(w http.ResponseWriter, info RequestInfo, status int, reason string) {
	if status == 0 {
		status = http.StatusForbidden
	}
	code := "forbidden"
	if status == http.StatusUnauthorized {
		code = "unauthorized"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"code":      code,
			"message":   reason,
			"action":    info.Action,
			"operation": info.OperationID,
		},
	})
}
