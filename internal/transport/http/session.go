package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// sessionPath tells the product site whether its visitor is signed in to
// the console, for its "Sign in / Open console" buttons. It is the one
// public path answered with credentials (see publicCORS): the site sends
// the visitor's Kratos cookie, the server asks Kratos itself and answers a
// bare yes or no — no identity, no token. Served beside ogen, outside the
// contract, like the WebSocket.
const sessionPath = publicPrefix + "session"

// sessionCheckTimeout bounds the Kratos whoami behind one answer.
const sessionCheckTimeout = 3 * time.Second

// signedIn answers GET sessionPath with {"signed_in": bool}: true when
// Kratos (its public listener at upstream) accepts the request's cookies as
// a session. Kratos unreachable reads as signed out.
func signedIn(upstream string, client *http.Client) http.Handler {
	whoami := strings.TrimRight(upstream, "/") + "/sessions/whoami"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		ok := cookie(r) != "" && kratosSession(r.Context(), client, whoami, cookie(r))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]bool{"signed_in": ok}) //nolint:errchkjson,errcheck // static shape; client went away
	})
}

// cookie is the request's Kratos cookies only: nothing else of the
// browser's cookie jar goes to Kratos.
func cookie(r *http.Request) string {
	var parts []string
	for _, c := range r.Cookies() {
		if strings.HasPrefix(c.Name, "ory_kratos_session") {
			parts = append(parts, c.Name+"="+c.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func kratosSession(ctx context.Context, client *http.Client, whoami, cookies string) bool {
	ctx, cancel := context.WithTimeout(ctx, sessionCheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, whoami, http.NoBody)
	if err != nil {
		return false
	}
	req.Header.Set("Cookie", cookies)
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	_ = res.Body.Close()
	return res.StatusCode == http.StatusOK
}
