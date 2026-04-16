package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewAuthMiddleware returns a chi-compatible middleware that authenticates requests.
// It supports JWT tokens and tenant API tokens (hash lookup in DB).
func NewAuthMiddleware(jwt *JWTIssuer, pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")

			// 1. Try JWT.
			if claims, err := jwt.Parse(token); err == nil {
				ctx := WithClaims(r.Context(), claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 2. Try tenant API token (hash lookup in DB).
			if pool != nil {
				hash := sha256.Sum256([]byte(token))
				hashStr := hex.EncodeToString(hash[:])
				var tenantID, role string
				err := pool.QueryRow(r.Context(),
					`SELECT tenant_id, role FROM tenant_api_tokens
                     WHERE token_hash = $1 AND (expires_at IS NULL OR expires_at > NOW())`,
					hashStr,
				).Scan(&tenantID, &role)
				if err == nil {
					claims := &Claims{
						UserID:   "api-token",
						Username: "api-token",
						TenantID: tenantID,
						Role:     role,
					}
					ctx := WithClaims(r.Context(), claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, "invalid credentials", http.StatusUnauthorized)
		})
	}
}

func RequireRole(minRole string) func(http.Handler) http.Handler {
	minLevel := RoleLevel(minRole)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if claims.IsRoot || RoleLevel(claims.Role) >= minLevel {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}

func RequireRoot() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !claims.IsRoot {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func TenantRequired() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil || claims.TenantID == "" {
				http.Error(w, "tenant selection required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isPublicPath(path string) bool {
	// Auth endpoints.
	switch path {
	case "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout":
		return true
	}
	// Public share links.
	if strings.HasPrefix(path, "/api/share/") {
		return true
	}
	// Agent API is now authenticated via JWT (agent tokens).
	// Everything outside /api/ is public: health, SPA (index.html, JS, CSS), agent binary, packages.
	if !strings.HasPrefix(path, "/api/") {
		return true
	}
	return false
}
