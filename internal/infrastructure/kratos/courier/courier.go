// Package courier serves the Kratos mail templates (code to confirm the
// address, code to reset the password, and the "no such account" variants of
// both). The templates ship inside the server binary; Kratos fetches them by
// URL (courier.templates.*) and renders them itself with its v26.2.0 models.
package courier

import (
	"embed"
	"io/fs"
	"net/http"
	"slices"
)

//go:embed verification_code recovery_code
var templates embed.FS

// Prefix is where the templates are served:
// <Prefix>/{version}/{template}/{valid|invalid}/{file}. The version segment
// only busts the Kratos template cache (it keeps a fetched URL until a
// restart); any value serves the templates of this binary.
const Prefix = "/internal/kratos/mail"

// Files a template state has: the subject, the HTML body, the text body.
var Files = []string{"email.subject.gotmpl", "email.body.gotmpl", "email.body.plaintext.gotmpl"}

// Handler serves GET <Prefix>/{version}/{template}/{state}/{file}; anything
// else is a 404.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+Prefix+"/{version}/{template}/{state}/{file}", func(w http.ResponseWriter, r *http.Request) {
		state, file := r.PathValue("state"), r.PathValue("file")
		if (state != "valid" && state != "invalid") || !slices.Contains(Files, file) {
			http.NotFound(w, r)
			return
		}
		body, err := fs.ReadFile(templates, r.PathValue("template")+"/"+state+"/"+file)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(body) //nolint:errcheck,gosec // embedded template served as text/plain; a client that went away
	})
	return mux
}
