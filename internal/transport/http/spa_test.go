package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAServesPrecompressedAndCaches(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":       {Data: []byte("<html>")},
		"assets/app.js":    {Data: []byte("raw js")},
		"assets/app.js.br": {Data: []byte("br js")},
		"assets/app.js.gz": {Data: []byte("gz js")},
		"gf/font.woff2":    {Data: []byte("font")},
	}
	h := spa(dist)
	for _, tc := range []struct {
		name, path, accept           string
		body, encoding, cache, ctype string
	}{
		{"brotli first", "/assets/app.js", "gzip, deflate, br", "br js", "br", cacheImmutable, "text/javascript; charset=utf-8"},
		{"gzip when no brotli", "/assets/app.js", "gzip", "gz js", "gzip", cacheImmutable, "text/javascript; charset=utf-8"},
		{"refused brotli", "/assets/app.js", "br;q=0, gzip", "gz js", "gzip", cacheImmutable, "text/javascript; charset=utf-8"},
		{"raw without encodings", "/assets/app.js", "", "raw js", "", cacheImmutable, "text/javascript; charset=utf-8"},
		{"unhashed file", "/gf/font.woff2", "br", "font", "", cacheDay, "font/woff2"},
		{"client route", "/t/acme/runs", "br", "<html>", "", cacheRevalidate, "text/html; charset=utf-8"},
		{"root", "/", "", "<html>", "", cacheRevalidate, "text/html; charset=utf-8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.accept != "" {
				req.Header.Set("Accept-Encoding", tc.accept)
			}
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
				t.Fatalf("got %d %q, want %q", rec.Code, rec.Body.String(), tc.body)
			}
			if got := rec.Header().Get("Content-Encoding"); got != tc.encoding {
				t.Fatalf("Content-Encoding %q, want %q", got, tc.encoding)
			}
			if got := rec.Header().Get("Cache-Control"); got != tc.cache {
				t.Fatalf("Cache-Control %q, want %q", got, tc.cache)
			}
			if got := rec.Header().Get("Content-Type"); got != tc.ctype {
				t.Fatalf("Content-Type %q, want %q", got, tc.ctype)
			}
		})
	}
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("api path: %d", rec.Code)
	}
}
