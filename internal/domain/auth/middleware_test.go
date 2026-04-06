package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func issuerForTest() *JWTIssuer {
	return NewJWTIssuer("test-secret-key-32bytes-long!!")
}

func TestAuthMiddleware_ValidJWT(t *testing.T) {
	issuer := issuerForTest()
	mw := NewAuthMiddleware(issuer, nil)

	claims := Claims{UserID: "u1", Username: "alice", TenantID: "t1", Role: "operator"}
	token, _ := issuer.Issue(claims, 15*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_NoAuth(t *testing.T) {
	issuer := issuerForTest()
	mw := NewAuthMiddleware(issuer, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil)
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_PublicPaths(t *testing.T) {
	issuer := issuerForTest()
	mw := NewAuthMiddleware(issuer, nil)

	paths := []string{"/health", "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/agent/register"}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		mw(okHandler()).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", p, rec.Code)
		}
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	claims := &Claims{UserID: "u1", Role: "viewer"}
	ctx := WithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	RequireRole("operator")(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_Allowed(t *testing.T) {
	claims := &Claims{UserID: "u1", Role: "owner"}
	ctx := WithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	RequireRole("operator")(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_RootBypassesAll(t *testing.T) {
	claims := &Claims{UserID: "u1", Role: "viewer", IsRoot: true}
	ctx := WithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	RequireRole("owner")(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (root bypasses), got %d", rec.Code)
	}
}

func TestRequireRoot(t *testing.T) {
	claims := &Claims{UserID: "u1", Role: "owner", IsRoot: false}
	ctx := WithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	RequireRoot()(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestTenantRequired_NoTenant(t *testing.T) {
	claims := &Claims{UserID: "u1", Role: "operator"}
	ctx := WithClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claims)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runs", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	TenantRequired()(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
