package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The site learns only whether its visitor is signed in: the server asks
// Kratos with the visitor's Kratos cookie and answers yes or no.
func TestSessionForTheSite(t *testing.T) {
	var forwarded string
	kratos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/whoami" {
			http.NotFound(w, r)
			return
		}
		forwarded = r.Header.Get("Cookie")
		if c, err := r.Cookie("ory_kratos_session"); err == nil && c.Value == "valid" {
			_, _ = w.Write([]byte(`{"identity":{"traits":{"email":"a@b.c"}}}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer kratos.Close()
	router, err := New(Deps{
		API:            http.NotFoundHandler(),
		KratosUpstream: kratos.URL,
		PublicOrigins:  []string{"https://stroppy.io"},
	})
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, origin, cookie string, header map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, sessionPath, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	for _, tc := range []struct {
		name, cookie, want string
	}{
		{"no cookie", "", `{"signed_in":false}`},
		{"stale session", "ory_kratos_session=expired", `{"signed_in":false}`},
		{"session", "ory_kratos_session=valid; _ga=tracking", `{"signed_in":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(http.MethodGet, "https://stroppy.io", tc.cookie, nil)
			if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != tc.want {
				t.Fatalf("got %d %s, want 200 %s", rec.Code, rec.Body.String(), tc.want)
			}
			h := rec.Header()
			if h.Get("Access-Control-Allow-Origin") != "https://stroppy.io" || h.Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatalf("cors headers: %v", h)
			}
			if h.Get("Cache-Control") != "no-store" || !strings.Contains(strings.Join(h.Values("Vary"), ","), "Origin") {
				t.Fatalf("cache headers: %v", h)
			}
			if strings.Contains(rec.Body.String(), "a@b.c") {
				t.Fatal("identity leaked")
			}
		})
	}
	if forwarded != "ory_kratos_session=valid" {
		t.Fatalf("Kratos got cookies %q, want only the session", forwarded)
	}

	// A foreign origin gets an answer its browser cannot read.
	if rec := do(http.MethodGet, "https://evil.example", "ory_kratos_session=valid", nil); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign origin allowed: %v", rec.Header())
	}
	// Preflight: GET only, with credentials.
	rec := do(http.MethodOptions, "https://stroppy.io", "", map[string]string{"Access-Control-Request-Method": "GET"})
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Credentials") != "true" || rec.Header().Get("Access-Control-Allow-Methods") != "GET,OPTIONS" {
		t.Fatalf("preflight: %d %v", rec.Code, rec.Header())
	}
	if rec := do(http.MethodOptions, "https://stroppy.io", "", map[string]string{"Access-Control-Request-Method": "POST"}); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("POST preflight allowed: %v", rec.Header())
	}
	if rec := do(http.MethodPost, "https://stroppy.io", "ory_kratos_session=valid", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", rec.Code)
	}
	// The rest of the public API stays without credentials.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/rating", nil)
	req.Header.Set("Origin", "https://stroppy.io")
	other := httptest.NewRecorder()
	router.ServeHTTP(other, req)
	if other.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("rating answered with credentials: %v", other.Header())
	}
}
