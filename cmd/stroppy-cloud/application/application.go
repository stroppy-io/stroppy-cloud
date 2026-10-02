package application

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gopherex/xlog"
	"github.com/gopherex/xprobe"
	"github.com/gopherex/xshutdown"

	"github.com/stroppy-io/stroppy-cloud/internal/api"
	"github.com/stroppy-io/stroppy-cloud/internal/build"
	"github.com/stroppy-io/stroppy-cloud/internal/oas"
	transport "github.com/stroppy-io/stroppy-cloud/internal/transport/http"
	"github.com/stroppy-io/stroppy-cloud/internal/transport/ws"
	"github.com/stroppy-io/stroppy-cloud/web"
)

/*
ROOT: config -> observability -> infra -> schema -> services -> transport.
The order is not decorative: the logger is needed by infra, infra by the
services, the services by the transport.

Fail early and loudly: a process that came up without its database would
answer clients with errors while pretending to be alive.
*/

const (
	shutdownTimeout = 30 * time.Second
	drainTimeout    = 15 * time.Second
)

// Application is what has been assembled.
type Application struct {
	cfg      *Config
	log      *xlog.Logger
	infra    *Infra
	services *Services
	shutdown *xshutdown.Manager
	ready    *xprobe.Bool
	fatal    chan error
}

// New raises everything except the listener; Run starts it.
func New(ctx context.Context) (*Application, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	log, traceShutdown, err := setupObservability(ctx, cfg)
	if err != nil {
		return nil, err
	}
	manager := xshutdown.New(ctx,
		xshutdown.WithTimeout(shutdownTimeout),
		xshutdown.WithErrorHandler(func(err error) {
			log.Error("shutdown task failed", xlog.ErrorCause(err))
		}),
	)
	manager.RegisterFnErr(traceShutdown)

	infra, err := connectInfra(ctx, &cfg.Infra, log)
	if err != nil {
		return nil, err
	}
	manager.RegisterFnErr(func(context.Context) error { infra.Close(); return nil })

	if cfg.Migrate {
		if err := infra.Postgres.Migrate(ctx); err != nil {
			infra.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
		log.Info("schema applied")
	}

	services, err := buildServices(ctx, cfg, infra, manager, log)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("services: %w", err)
	}
	return &Application{
		cfg:      cfg,
		log:      log,
		infra:    infra,
		services: services,
		shutdown: manager,
		ready:    xprobe.NewBool(),
		fatal:    make(chan error, 1),
	}, nil
}

// Run raises the listener, then waits for a signal or a fatal failure and
// shuts down within the timeout.
func (a *Application) Run(ctx context.Context) (runErr error) {
	a.log.Info("stroppy-cloud starting", xlog.String("addr", a.cfg.HTTP.Addr))
	defer func() {
		a.ready.Set(false)
		if err := a.shutdown.Stop(); err != nil {
			runErr = stderrors.Join(runErr, fmt.Errorf("shutdown: %w", err))
		}
	}()

	handler, err := a.handler()
	if err != nil {
		return err
	}
	if err := a.serve(ctx, handler); err != nil {
		return err
	}
	a.startWorkers()
	a.ready.Set(true)
	a.log.Info("stroppy-cloud is up")

	signalCtx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	select {
	case <-signalCtx.Done():
		return nil
	case err := <-a.fatal:
		return err
	}
}

// handler assembles the HTTP surface: the verifier, then the transport.
func (a *Application) handler() (http.Handler, error) {
	if a.cfg.Dev.Enabled() {
		dev, err := newDevVerifier(&a.cfg.Dev, a.services.Profiles, a.services.Tenants, a.log)
		if err != nil {
			return nil, err
		}
		a.log.Warn("DEV MODE: static tokens log in, Kratos is not asked — never expose this installation",
			xlog.Int("users", len(a.cfg.Dev.Users)))
		return a.httpHandler(verifierChain{tokens: a.services.Tokens, dev: dev})
	}
	kratos, err := newKratosVerifier(&a.cfg.Auth, a.services.Profiles, a.services.Tenants, a.log)
	if err != nil {
		return nil, err
	}
	a.log.Info("kratos is active",
		xlog.String("jwks", a.cfg.Auth.JWKSFile+""+a.cfg.Auth.JWKSURL),
		xlog.String("issuer", a.cfg.Auth.Issuer))
	return a.httpHandler(verifierChain{tokens: a.services.Tokens, kratos: kratos})
}

// httpHandler is the transport over an already-built verifier: the API
// server, the SPA and the probes. Tests build it with a verifier of their
// own.
func (a *Application) httpHandler(verifier api.Verifier) (http.Handler, error) {
	handler := api.New(api.Deps{
		Profiles:  a.services.Profiles,
		Tenants:   a.services.Tenants,
		Tokens:    a.services.Tokens,
		Audit:     a.services.Audit,
		Settings:  a.services.Settings,
		Providers: a.services.Providers,
		Webhooks:  a.services.Webhooks,
		Catalog:   a.services.Catalog,
		Library:   a.services.Library,
		Runs:      a.services.Runs,
		Suites:    a.services.Suites,
		Schedules: a.services.Schedules,
		Shares:    a.services.Shares,
		Compare:   a.services.Compare,
		Rating:    a.services.Rating,
		Admin:     a.services.Admin,
		Examples:  a.services.Examples,
		Observe:   a.services.Observe,
		Live:      a.services.Projector,
		Schemas:   a.services.Schemas,
		Public: api.PublicConfig{
			AuthMode: "kratos", KratosPublicURL: a.cfg.Auth.KratosPublicURL,
			TenantCreation: "anyone", PublicRating: true, Examples: true,
		},
		Probes: a.infra.Probes(),
		Log:    a.log,
	})
	apiServer, err := oas.NewServer(handler, api.NewSecurity(verifier),
		oas.WithPathPrefix(""),
		oas.WithErrorHandler(api.ErrorHandler(handler)),
		oas.WithMiddleware(api.UsersMiddleware(handler)),
	)
	if err != nil {
		return nil, fmt.Errorf("api server: %w", err)
	}

	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("web dist: %w", err)
	}

	alive := xprobe.NewBool()
	alive.Set(true)
	a.shutdown.RegisterFnErr(func(context.Context) error { alive.Set(false); return nil })

	socket := ws.New(verifier, api.NewStreams(handler), a.log)
	socket.Origins = a.cfg.HTTP.CORSOrigins
	socket.Base = a.shutdown.Context()
	if a.cfg.HTTP.WSPoll > 0 {
		socket.Poll = a.cfg.HTTP.WSPoll
	}
	return transport.New(transport.Deps{
		API:          api.AcceptMiddleware(apiServer),
		WS:           socket,
		PublicConfig: a.publicConfig(),
		SPA:          dist,
		Liveness:     alive,
		Readiness:    xprobe.All(a.infra.Probe(), a.ready),
		CORSOrigins:  a.cfg.HTTP.CORSOrigins,
		Log:          a.log,
	})
}

// publicConfig is what the SPA needs before login: the auth mode and where
// the browser reaches Kratos.
func (a *Application) publicConfig() http.Handler {
	mode := "kratos"
	if a.cfg.Dev.Enabled() {
		// The SPA asks for a token instead of talking to Kratos.
		mode = "dev"
	}
	body, _ := json.Marshal(map[string]any{ //nolint:errcheck // static map
		"auth_mode": mode,
		"kratos":    map[string]string{"public_url": a.cfg.Auth.KratosPublicURL},
		"version":   map[string]string{"version": buildVersion(), "commit": buildCommit()},
	})
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(body) //nolint:errcheck // client went away
	})
}

// serve starts the listener under the shutdown manager. Liveness drops
// first so the balancer learns about the stop before the socket closes.
func (a *Application) serve(ctx context.Context, handler http.Handler) error {
	srv := transport.Server(a.cfg.HTTP.Addr, handler)
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", a.cfg.HTTP.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", a.cfg.HTTP.Addr, err)
	}
	a.shutdown.Go(func(ctx context.Context) {
		go func() {
			<-ctx.Done()
			shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainTimeout)
			defer cancel()
			if err := srv.Shutdown(shutCtx); err != nil {
				a.log.Error("http: server shutdown", xlog.ErrorCause(err))
			}
		}()
		a.log.Info("http listening", xlog.String("addr", a.cfg.HTTP.Addr))
		if err := srv.Serve(listener); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
			a.fail("http server", err)
		}
	})
	return nil
}

func (a *Application) fail(component string, err error) {
	a.ready.Set(false)
	select {
	case a.fatal <- fmt.Errorf("%s: %w", component, err):
	default:
	}
}

func buildVersion() string { return build.Version }
func buildCommit() string  { return build.Commit }
