package api

import (
	"net/http"
	"strings"
)

// AuthMiddleware checks for a valid API key in the Authorization header.
// If apiKey is empty, auth is disabled (development mode).
func AuthMiddleware(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Allow agent endpoints without auth (agents use machine-to-machine trust)
			if strings.HasPrefix(r.URL.Path, "/api/agent/") {
				next.ServeHTTP(w, r)
				return
			}

			// Allow health and binary download without auth
			if r.URL.Path == "/health" || r.URL.Path == "/agent/binary" {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Support "Bearer <key>" and raw "<key>"
			key := strings.TrimPrefix(auth, "Bearer ")
			if key != apiKey {
				http.Error(w, "invalid API key", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
