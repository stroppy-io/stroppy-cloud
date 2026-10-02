// Package http is the single public listener: the ogen API under /api/v1,
// the WebSocket beside it, probes under /healthz and the SPA for everything
// else. Kratos is a separate origin (deployments/kratos/), not proxied.
package http

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gopherex/xlog"
	xloghttp "github.com/gopherex/xlog/pkg/http"
	"github.com/gopherex/xprobe"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Deps is what the router mounts.
type Deps struct {
	// API is the ogen server (already prefixed with /api/v1).
	API http.Handler
	// WS is the WebSocket at /api/v1/ws; nil = not mounted.
	WS http.Handler
	// PublicConfig is served at /config.json for the SPA (auth mode, Kratos origin).
	PublicConfig http.Handler
	// SPA is the built web app (index.html + assets); nil = 404.
	SPA fs.FS
	// Liveness / Readiness feed /healthz/*.
	Liveness  xprobe.Probe
	Readiness xprobe.Probe
	// CORSOrigins are extra origins allowed to call the API.
	CORSOrigins []string
	Log         *xlog.Logger
}

const (
	apiPrefix         = "/api"
	readHeaderTimeout = 10 * time.Second
)

// New builds the router.
func New(d Deps) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(propagateRequestID)
	r.Use(xloghttp.Middleware(d.Log))
	r.Use(middleware.Recoverer)
	r.Use(cors(d.CORSOrigins))

	// Probes: cheap, unauthenticated, outside tracing.
	r.Mount("/healthz", xprobe.Mux(
		xprobe.Liveness(d.Liveness),
		xprobe.Readiness(d.Readiness),
		xprobe.Startup(d.Readiness),
	))

	if d.PublicConfig != nil {
		r.Handle("/config.json", d.PublicConfig)
	}

	// The socket lives beside the API (the upgrade bypasses ogen; the log
	// middleware's recorder unwraps to the hijacker).
	if d.WS != nil {
		r.Handle(apiPrefix+"/v1/ws", d.WS)
	}
	// The API: otelhttp names the server span after the route; ogen adds
	// the operation-level span underneath.
	r.Handle(apiPrefix+"/*", otelhttp.NewHandler(d.API, "http.api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		})))

	r.NotFound(spa(d.SPA))
	return r, nil
}

// Server wraps the listener with sane timeouts.
func Server(addr string, h http.Handler) *http.Server {
	return &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: readHeaderTimeout}
}

// propagateRequestID copies chi's request id into the request header the
// log middleware reads, so both agree and the API can echo it.
func propagateRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" && r.Header.Get("X-Request-Id") == "" {
			r.Header.Set("X-Request-Id", id)
		}
		next.ServeHTTP(w, r)
	})
}

// spa serves the built web app: real files as-is, every other path gets
// index.html (client-side routing). API-looking paths stay 404.
func spa(dist fs.FS) http.HandlerFunc {
	if dist == nil {
		return http.NotFound
	}
	files := http.FS(dist)
	static := http.FileServer(files)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" || strings.HasPrefix(p, "api/") || strings.HasPrefix(p, "v1/") {
			if p != "" {
				http.NotFound(w, r)
				return
			}
			p = "index.html"
		}
		if f, err := dist.Open(p); err == nil {
			_ = f.Close()
			static.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		w.Header().Set("Cache-Control", "no-cache")
		static.ServeHTTP(w, r)
	}
}

// cors allows the SPA's own origin implicitly (same-origin needs nothing)
// plus the configured extras (the Vite dev server).
func cors(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowed[origin] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,X-Request-Id")
				h.Set("Access-Control-Max-Age", "600")
				h.Add("Vary", "Origin")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
