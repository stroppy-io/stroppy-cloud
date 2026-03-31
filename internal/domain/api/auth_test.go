package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestAuthMiddleware_EmptyKeyAllowsAll(t *testing.T) {
	mw := AuthMiddleware("", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ValidKeyPasses(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_RawKeyPasses(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	req.Header.Set("Authorization", "secret-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidKeyRejects(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingHeaderRejects(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/run/123/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AgentPathBypassesAuth(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/api/agent/register", nil)
	// No Authorization header
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for agent path, got %d", rec.Code)
	}
}

func TestAuthMiddleware_HealthBypassesAuth(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", rec.Code)
	}
}

func TestAuthMiddleware_AgentBinaryBypassesAuth(t *testing.T) {
	mw := AuthMiddleware("secret-key", nil)
	handler := mw(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/agent/binary", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /agent/binary, got %d", rec.Code)
	}
}
