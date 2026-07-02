package middleware

import (
	"net/http"

	"github.com/boeing-ai-gateway/discboeing/agent-go/internal/credentials"
)

const (
	credentialsHeader  = "X-Discboeing-Credentials"
	gitUserNameHeader  = "X-Discboeing-Git-User-Name"
	gitUserEmailHeader = "X-Discboeing-Git-User-Email"
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
