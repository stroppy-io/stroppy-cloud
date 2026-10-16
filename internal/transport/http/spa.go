package http

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// Cache lifetimes of the SPA files: hashed build output never changes under
// its name, index.html must be revalidated so a deploy is picked up, and the
// unhashed rest (fonts, icons, logos) may be a day old.
const (
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheRevalidate = "no-cache"
	cacheDay        = "public, max-age=86400"
)

// encodings are the precompressed siblings the build writes (`<file>.br`,
// `<file>.gz`), best first.
var encodings = []struct{ name, ext string }{{"br", ".br"}, {"gzip", ".gz"}}

// spa serves the built web app: real files as-is (a precompressed sibling
// when the browser takes it), every other path gets index.html
// (client-side routing). API-looking paths stay 404.
func spa(dist fs.FS) http.HandlerFunc {
	if dist == nil {
		return http.NotFound
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if strings.HasPrefix(p, "api/") || strings.HasPrefix(p, "v1/") {
			http.NotFound(w, r)
			return
		}
		cache := cacheDay
		switch {
		case p == "" || p == "index.html" || !exists(dist, p):
			p, cache = "index.html", cacheRevalidate
		case strings.HasPrefix(p, "assets/"):
			cache = cacheImmutable
		}
		w.Header().Set("Cache-Control", cache)
		serveFile(w, r, dist, p)
	}
}

func exists(dist fs.FS, name string) bool {
	info, err := fs.Stat(dist, name)
	return err == nil && !info.IsDir()
}

// serveFile writes name, or its precompressed sibling in an encoding the
// request accepts. The content type comes from name, never the sibling.
func serveFile(w http.ResponseWriter, r *http.Request, dist fs.FS, name string) {
	w.Header().Add("Vary", "Accept-Encoding")
	file := name
	for _, e := range encodings {
		if accepts(r.Header.Get("Accept-Encoding"), e.name) && exists(dist, name+e.ext) {
			file = name + e.ext
			w.Header().Set("Content-Encoding", e.name)
			break
		}
	}
	f, err := dist.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.Error(w, "file is not seekable", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, name, time.Time{}, rs)
}

// accepts reports whether an Accept-Encoding header allows the encoding
// (a "q=0" entry refuses it).
func accepts(header, encoding string) bool {
	for _, part := range strings.Split(header, ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(token), encoding) {
			continue
		}
		q := strings.ReplaceAll(strings.TrimSpace(params), " ", "")
		return q != "q=0" && q != "q=0.0" && q != "q=0.00" && q != "q=0.000"
	}
	return false
}
