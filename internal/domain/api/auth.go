package api

import (
	"net/http"
	"strings"
)

// AuthMiddleware checks for a valid API key or user session token in the
// Authorization header. If apiKey is empty and no user store is provided,
// auth is disabled (development mode).
func AuthMiddleware(apiKey string, users *UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow agent endpoints without auth (agents use machine-to-machine trust).
			if strings.HasPrefix(r.URL.Path, "/api/agent/") {
				next.ServeHTTP(w, r)
				return
			}

			// Allow health and binary download without auth.
			if r.URL.Path == "/health" || r.URL.Path == "/agent/binary" {
				next.ServeHTTP(w, r)
				return
			}

			// Allow login endpoint without auth.
			if r.URL.Path == "/api/v1/auth/login" {
				next.ServeHTTP(w, r)
				return
			}

			// If no API key configured, auth is optional — allow unauthenticated access
			// but still validate tokens if provided (SPA users can login for identity).
			auth := r.Header.Get("Authorization")
			if apiKey == "" && auth == "" {
				next.ServeHTTP(w, r)
				return
			}
			if auth == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			// Support "Bearer <key>" and raw "<key>".
			key := strings.TrimPrefix(auth, "Bearer ")

			// Check API key first (for programmatic access).
			if apiKey != "" && key == apiKey {
				next.ServeHTTP(w, r)
				return
			}

			// Then check user session token (for SPA).
			if users != nil {
				user := users.ValidateToken(key)
				if user != nil {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "invalid credentials", http.StatusForbidden)
		})
	}
}
