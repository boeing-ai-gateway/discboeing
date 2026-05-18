package session

import "context"

const (
	// CookieName is the browser cookie that binds a user to ui-go session state.
	CookieName = "ui_go_session_id"

	// QueryParam is a cookie fallback for embedded service panels where the
	// browser may block iframe cookies.
	QueryParam = "ui_go_session_id"

	// HeaderName carries the session ID for same-origin scripted requests.
	HeaderName = "X-UI-Go-Session"
)

type contextKey struct{}

// WithID stores the ui-go session ID on ctx.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// ID returns the ui-go session ID from ctx.
func ID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)
	return id, ok && id != ""
}
