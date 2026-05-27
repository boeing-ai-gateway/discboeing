package middleware

import (
	"net/http"

	"github.com/obot-platform/discobot/agent-go/internal/credentials"
)

const (
	credentialsHeader  = "X-Discobot-Credentials"
	gitUserNameHeader  = "X-Discobot-Git-User-Name"
	gitUserEmailHeader = "X-Discobot-Git-User-Email"
)

// Credentials returns middleware that applies credential environment variables
// and git user configuration from request headers.
func Credentials(mgr *credentials.Manager, onCredentialsApplied func()) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !requestAuthenticated(r) {
				next.ServeHTTP(w, r)
				return
			}

			credHeader := r.Header.Get(credentialsHeader)
			gitName := r.Header.Get(gitUserNameHeader)
			gitEmail := r.Header.Get(gitUserEmailHeader)

			if credHeader != "" || gitName != "" || gitEmail != "" {
				mgr.Apply(credHeader, gitName, gitEmail)
				if credHeader != "" && onCredentialsApplied != nil {
					onCredentialsApplied()
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
