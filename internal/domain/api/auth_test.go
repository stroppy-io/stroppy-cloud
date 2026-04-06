package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestAuthMiddleware_MissingHeaderRejects(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AgentPathBypassesAuth(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/agent/register", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent path, got %d", rec.Code)
	}
}

func TestAuthMiddleware_HealthBypassesAuth(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", rec.Code)
	}
}

func TestAuthMiddleware_LoginBypassesAuth(t *testing.T) {
	jwt := auth.NewJWTIssuer("test-secret")
	mw := auth.NewAuthMiddleware(jwt, nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for login path, got %d", rec.Code)
	}
}
