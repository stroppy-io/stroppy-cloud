// Package http is the single public listener: the ogen API under /api/v1,
// the IAM reverse proxy under /v1, the Grafana relay under /grafana, the
// IAM webhook, probes under /healthz and the SPA for everything else.
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
	// IAMURL is the IAM base URL proxied under /v1 (same-origin for the
	// SPA; Set-Cookie domains are rewritten to this host).
	IAMURL string
	// GrafanaURL is relayed under /grafana; empty = not mounted.
	GrafanaURL string
	// Webhook is the IAM webhook handler at WebhookPath; nil = not mounted.
	Webhook     http.Handler
	WebhookPath string
	// PublicConfig is served at /config.json for the SPA (IAM base, client id).
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
	iamPrefix         = "/v1"
	grafanaPrefix     = "/grafana"
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

	if d.Webhook != nil {
		r.Handle(d.WebhookPath, d.Webhook)
	}
	if d.PublicConfig != nil {
		r.Handle("/config.json", d.PublicConfig)
	}

	// No IAM (dev mode): nothing to proxy.
	if d.IAMURL != "" {
		iam, err := reverseProxy(d.IAMURL, true)
		if err != nil {
			return nil, fmt.Errorf("iam proxy: %w", err)
		}
		r.Handle(iamPrefix+"/*", iam)
	}

	if d.GrafanaURL != "" {
		grafana, err := reverseProxy(d.GrafanaURL, false)
		if err != nil {
			return nil, fmt.Errorf("grafana proxy: %w", err)
		}
		r.Handle(grafanaPrefix+"/*", grafana)
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

// reverseProxy forwards to base without rewriting paths. stripCookieDomain
// pins upstream Set-Cookie to this host: a cookie scoped to the IAM domain
// would never reach the browser behind the proxy.
func reverseProxy(base string, stripCookieDomain bool) (http.Handler, error) {
	target, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("upstream url %q: %w", base, err)
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.SetXForwarded()
		},
	}
	if stripCookieDomain {
		proxy.ModifyResponse = func(resp *http.Response) error {
			cookies := resp.Cookies()
			if len(cookies) == 0 {
				return nil
			}
			resp.Header.Del("Set-Cookie")
			for _, c := range cookies {
				c.Domain = ""
				resp.Header.Add("Set-Cookie", c.String())
			}
			return nil
		}
	}
	return proxy, nil
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
