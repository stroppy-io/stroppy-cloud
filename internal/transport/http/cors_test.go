package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// The product site reads the public API from a browser, nothing else; the
// SPA's own extra origins keep the full API.
func TestPublicCORS(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	router, err := New(Deps{
		API:           api,
		CORSOrigins:   []string{"http://localhost:5173"},
		PublicOrigins: []string{"https://stroppy.io", "http://localhost:3000/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path, origin string, header map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	preflight := map[string]string{"Access-Control-Request-Method": "GET"}
	for _, tc := range []struct {
		name, method, path, origin string
		header                     map[string]string
		allow                      string
		credentials                bool
	}{
		{"site reads the rating", http.MethodGet, "/api/v1/public/rating", "https://stroppy.io", nil, "https://stroppy.io", false},
		{"site preflight", http.MethodOptions, "/api/v1/public/rating", "https://stroppy.io", preflight, "https://stroppy.io", false},
		{"local site from config", http.MethodGet, "/api/v1/public/rating", "http://localhost:3000", nil, "http://localhost:3000", false},
		{"foreign origin", http.MethodGet, "/api/v1/public/rating", "https://evil.example", nil, "", false},
		{"foreign preflight", http.MethodOptions, "/api/v1/public/rating", "https://evil.example", preflight, "", false},
		{"site cannot reach the tenant API", http.MethodGet, "/api/v1/t/acme/runs", "https://stroppy.io", nil, "", false},
		{"site cannot write the public API", http.MethodOptions, "/api/v1/public/rating", "https://stroppy.io", map[string]string{"Access-Control-Request-Method": "POST"}, "", false},
		{"dev SPA keeps the whole API", http.MethodGet, "/api/v1/t/acme/runs", "http://localhost:5173", nil, "http://localhost:5173", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(tc.method, tc.path, tc.origin, tc.header)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.allow {
				t.Errorf("Allow-Origin = %q; want %q", got, tc.allow)
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials") == "true"; got != tc.credentials {
				t.Errorf("Allow-Credentials = %v; want %v", got, tc.credentials)
			}
			if tc.method == http.MethodOptions && tc.allow != "" {
				if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Methods") != "GET,HEAD,OPTIONS" {
					t.Errorf("preflight = %d %q", rec.Code, rec.Header().Get("Access-Control-Allow-Methods"))
				}
			}
		})
	}
}

// The Kratos mail templates live on the internal listener only.
func TestPublicListenerHidesInternal(t *testing.T) {
	router, err := New(Deps{API: http.NotFoundHandler(), SPA: fstest.MapFS{"index.html": {Data: []byte("<html>")}}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/internal/kratos/mail/v1/recovery_code/valid/email.body.gotmpl", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}
