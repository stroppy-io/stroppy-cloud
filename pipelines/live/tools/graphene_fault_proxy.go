//go:build ignore

// A temporary local server-to-Graphene fault proxy. Never logs headers or bodies.
// Build this file explicitly. Bind forwarding to a Docker bridge address and
// control to loopback; upstream TLS validation stays enabled. Faults expire.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type fault struct {
	Method  string `json:"method"`
	After   bool   `json:"after"`
	Seconds int    `json:"seconds"`
}

type controller struct {
	sync.Mutex
	fault    fault
	until    time.Time
	hits     int
	accepted int
	next     int
	active   map[int]context.CancelFunc
}

func main() {
	listen := flag.String("listen", "", "Docker bridge IP:port (required)")
	control := flag.String("control", "127.0.0.1:18349", "loopback control address")
	upstream := flag.String("upstream", "https://graphene.stroppy.io", "validated HTTPS upstream")
	flag.Parse()
	host, _, err := net.SplitHostPort(*control)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("control must bind loopback")
	}
	host, _, err = net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsPrivate() {
		log.Fatal("forward listener must bind a private bridge IP")
	}
	u, err := url.Parse(*upstream)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		log.Fatal("HTTPS upstream required")
	}
	c := &controller{active: make(map[int]context.CancelFunc)}
	proxy := httputil.NewSingleHostReverseProxy(u)
	director := proxy.Director
	proxy.Director = func(r *http.Request) { director(r); r.Host = u.Host }
	proxy.FlushInterval = -1
	proxy.ErrorLog = log.New(io.Discard, "", 0)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) { unavailable(w) }
	forward := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		c.Lock()
		id := c.next
		c.next++
		c.active[id] = cancel
		fail := time.Now().Before(c.until) && (c.fault.Method == "*" || strings.HasSuffix(r.URL.Path, "/"+c.fault.Method))
		after := c.fault.After
		if fail {
			c.hits++
		}
		c.Unlock()
		defer func() { c.Lock(); delete(c.active, id); c.Unlock() }()
		if fail && !after {
			unavailable(w)
			return
		}
		if r.Method == http.MethodConnect {
			// HTTPS_PROXY mode preserves the real Graphene endpoint and image
			// names. TLS remains end-to-end; only whole-connection outages can
			// be injected in this mode, not individual encrypted RPCs.
			address := u.Host
			if u.Port() == "" {
				address = net.JoinHostPort(u.Hostname(), "443")
			}
			if r.Host != address {
				http.Error(w, "unexpected upstream", http.StatusForbidden)
				return
			}
			up, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", address)
			if err != nil {
				unavailable(w)
				return
			}
			defer up.Close()
			down, buffered, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			defer down.Close()
			_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
			if err := buffered.Flush(); err != nil {
				return
			}
			go func() { <-ctx.Done(); _ = up.Close(); _ = down.Close() }()
			go func() { _, _ = io.Copy(up, buffered); cancel() }()
			_, _ = io.Copy(down, up)
			return
		}
		if fail {
			// Only unary StartRun is allowed in after-acceptance mode. Discard
			// its real response, so the caller cannot distinguish acceptance.
			d := &discardResponse{header: make(http.Header)}
			proxy.ServeHTTP(d, r.WithContext(ctx))
			if d.status == http.StatusOK {
				c.Lock()
				c.accepted++
				c.Unlock()
			}
			unavailable(w)
			return
		}
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, _ *http.Request) {
		c.Lock()
		defer c.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"fault": c.fault, "until": c.until, "active": len(c.active), "hits": c.hits, "accepted_responses_discarded": c.accepted})
	})
	mux.HandleFunc("POST /fault", func(w http.ResponseWriter, r *http.Request) {
		var f fault
		if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&f); err != nil || f.Seconds < 0 || f.Seconds > 300 || (f.After && f.Method != "StartRun") {
			http.Error(w, "invalid fault", 400)
			return
		}
		c.Lock()
		c.fault, c.until, c.hits, c.accepted = f, time.Now().Add(time.Duration(f.Seconds)*time.Second), 0, 0
		if f.Method == "*" && f.Seconds > 0 {
			for _, cancel := range c.active {
				cancel()
			}
		}
		c.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	go func() {
		s := &http.Server{Addr: *control, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		log.Fatal(s.ListenAndServe())
	}()
	fmt.Println("fault proxy ready; no request headers or bodies are logged")
	s := &http.Server{Addr: *listen, Handler: forward, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(s.ListenAndServe())
}

func unavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, `{"code":"unavailable","message":"local resilience test"}`)
}

type discardResponse struct {
	header http.Header
	status int
}

func (d *discardResponse) Header() http.Header    { return d.header }
func (d *discardResponse) WriteHeader(status int) { d.status = status }
func (d *discardResponse) Write(b []byte) (int, error) {
	if d.status == 0 {
		d.status = 200
	}
	return len(b), nil
}
func (d *discardResponse) Flush() {}
