package api

import (
	"context"
	"net/http"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token != "" {
			ctx := context.WithValue(r.Context(), ctxKeyToken, token)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
