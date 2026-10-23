// Package http is the single public listener: the ogen API under /api/v1,
// the WebSocket beside it, the Kratos reverse proxy under /v1 (same-origin
// browser flows of the SPA), probes under /healthz and the SPA for
// everything else.
package http

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	// KratosUpstream is the Kratos public URL proxied under /v1: the SPA's
	// browser flows (login, registration, whoami) stay same-origin, so
	// their cookies need no cross-site games. Empty = not mounted.
	KratosUpstream string
	// PublicConfig is served at /config.json for the SPA (auth mode, Kratos origin).
	PublicConfig http.Handler
	// SPA is the built web app (index.html + assets); nil = 404.
	SPA fs.FS
	// Liveness / Readiness feed /healthz/*.
	Liveness  xprobe.Probe
	Readiness xprobe.Probe
	// CORSOrigins are extra origins allowed to call the API.
	CORSOrigins []string
	// PublicOrigins may only read the public API (publicPrefix) from a
	// browser: GET/HEAD, no credentials.
	PublicOrigins []string
	Log           *xlog.Logger
}

const (
	publicPrefix      = "/api/v1/public/"
	kratosPrefix      = "/v1"
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
	r.Use(publicCORS(d.PublicOrigins))

	// Probes: cheap, unauthenticated, outside tracing.
	r.Mount("/healthz", xprobe.Mux(
		xprobe.Liveness(d.Liveness),
		xprobe.Readiness(d.Readiness),
		xprobe.Startup(d.Readiness),
	))

	if d.PublicConfig != nil {
		r.Handle("/config.json", d.PublicConfig)
	}

	// Kratos browser flows of the SPA (cookies, CSRF) go through the same
	// origin; reverseProxy strips /v1 (Kratos's base_url only shapes the
	// links it generates).
	if d.KratosUpstream != "" {
		proxy, err := reverseProxy(d.KratosUpstream)
		if err != nil {
			return nil, fmt.Errorf("kratos proxy: %w", err)
		}
		r.Handle(kratosPrefix+"/*", proxy)
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

	// /internal/* belongs to the internal listener; the public one never
	// answers it, not even with the SPA.
	r.Handle("/internal/*", http.NotFoundHandler())
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

// reverseProxy forwards to Kratos under /v1: the prefix is stripped (base_url
// only affects link generation, Kratos serves its paths at the root), so
// /v1/self-service/... upstream sees /self-service/....
func reverseProxy(base string) (http.Handler, error) {
	target, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("upstream url %q: %w", base, err)
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, kratosPrefix)
			if pr.Out.URL.Path == "" {
				pr.Out.URL.Path = "/"
			}
			pr.SetXForwarded()
		},
	}
	return proxy, nil
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

// publicCORS lets the product site read the public API from a browser: the
// rating and shared reports, GET and HEAD only, without credentials (a
// cookie or token never travels cross-site). Nothing else of the API answers
// these origins; an origin cors() already allows keeps its own headers.
func publicCORS(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			h := w.Header()
			if origin == "" || !allowed[origin] || !strings.HasPrefix(r.URL.Path, publicPrefix) || h.Get("Access-Control-Allow-Origin") != "" {
				next.ServeHTTP(w, r)
				return
			}
			h.Add("Vary", "Origin")
			switch r.Method {
			case http.MethodGet, http.MethodHead:
				h.Set("Access-Control-Allow-Origin", origin)
				next.ServeHTTP(w, r)
			case http.MethodOptions:
				if m := r.Header.Get("Access-Control-Request-Method"); m != http.MethodGet && m != http.MethodHead {
					w.WriteHeader(http.StatusNoContent) // no CORS headers: the browser refuses
					return
				}
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Methods", "GET,HEAD,OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
