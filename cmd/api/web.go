package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/dist
var webFS embed.FS

// spaHandler serves embedded SPA files with fallback to index.html
// for client-side routing.
func spaHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file. If it exists, serve it.
		if f, err := sub.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
