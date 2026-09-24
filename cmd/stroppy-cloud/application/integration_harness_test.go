//go:build integration

package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xprobe"
	"github.com/gopherex/xshutdown"

	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

/*
HARNESS: the real application core (services over testDB) behind the real
transport, with two substitutions — the IAM verifier (tokens are `stc_`
API tokens minted through the token service, so no IAM is needed) and the
Graphene door (fakeGraphene, integration_door_test.go: an in-process
Connect server running the real pipelines in virtual time).
*/

// e2e is one assembled application under test.
type e2e struct {
	t        *testing.T
	ts       *httptest.Server
	app      *Application
	graphene *fakeGraphene
	// ctx is the application's lifetime: workers ticked by tests must use
	// it so their followers stop at cleanup.
	ctx context.Context
	// pushes records the pipeline pushes the server asked for.
	pushes *fakePusher
}

// fakePusher stands in for publication of the packaged pipeline binaries.
type fakePusher struct {
	mu    sync.Mutex
	calls []string // "<namespace>/<pipeline>"
	fail  bool
}

func (f *fakePusher) Push(_ context.Context, pipeline, namespace string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, namespace+"/"+pipeline)
	if f.fail {
		return errors.New("registry unreachable")
	}
	return nil
}

func (f *fakePusher) count(namespace string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, namespace+"/") {
			n++
		}
	}
	return n
}

// e2eServer assembles the application over testDB and the fake door.
func e2eServer(t *testing.T) *e2e {
	t.Helper()
	return e2eServerWith(t, e2eOptions{})
}

// e2eOptions shape an assembled application.
type e2eOptions struct {
	// Speed is the door's virtual seconds per wall second (0 = instant).
	Speed float64
	// DevUsers turn dev mode on (`token=email[=name]`): the real handler
	// of a dev installation, no IAM.
	DevUsers []string
	// Addr is where the server listens ("" = a random local port).
	Addr string
	// PublicURL overrides the SPA origin.
	PublicURL string
}

// e2eServerWith assembles the application with options.
func e2eServerWith(t *testing.T, opts e2eOptions) *e2e {
	t.Helper()
	fake := newFakeGraphene()
	fake.speed = opts.Speed
	mux := http.NewServeMux()
	mux.Handle(managementv1connect.NewResourcesAPIHandler(fake))
	mux.Handle(managementv1connect.NewSecretsAPIHandler(fake))
	mux.Handle(managementv1connect.NewRunsAPIHandler(fake))
	mux.Handle(managementv1connect.NewRbacAPIHandler(fake))
	mux.Handle(managementv1connect.NewObserveAPIHandler(fake))
	door := httptest.NewServer(mux)
	t.Cleanup(door.Close)

	cfg := &Config{}
	cfg.HTTP.PublicURL = "http://stroppy.test"
	cfg.HTTP.WSPoll = 200 * time.Millisecond
	cfg.IAM.ClientID = "web"
	cfg.IAM.Environment = "test"
	cfg.IAM.BaseURL = "http://iam.test"
	cfg.IAM.WebhookPath = "/webhooks/iam"
	cfg.IAM.WebhookSigningSecret = "whsec-test"
	cfg.IAM.ProjectID = "proj-test"
	cfg.IAM.AccessTTL = 10 * time.Minute
	cfg.AdminEmails = []string{"root@example.com"}
	cfg.Dev.Users = opts.DevUsers
	if opts.PublicURL != "" {
		cfg.HTTP.PublicURL = opts.PublicURL
	}
	pushes := &fakePusher{}
	cfg.Infra.Pipelines.Runner = pushes
	cfg.Infra.Graphene = graphene.Config{Address: door.Listener.Addr().String(), Token: "test", Insecure: true}
	cfg.Infra.Victoria = victoria.Config{LogsURL: victoriaLogsURL, MetricsURL: victoriaMetricsURL, Timeout: 10 * time.Second}
	// Every simulated run leaves its telemetry in the real stores.
	fake.recorded = func(r *doorRun) {
		if err := pushTelemetry(context.Background(), victoriaLogsURL, victoriaMetricsURL, r); err != nil {
			t.Errorf("telemetry of %s: %v", r.id, err)
		}
	}
	cfg.HTTP.GrafanaURL = "http://grafana.test"
	cfg.HTTP.GrafanaDashboards = []string{"stroppy-run=Run overview", "stroppy-machine=Machine:per_machine"}

	ctx, cancel := context.WithCancel(context.Background())
	manager := xshutdown.New(ctx, xshutdown.WithTimeout(10*time.Second))
	infra := &Infra{Postgres: testDB, Graphene: graphene.New(&cfg.Infra.Graphene)}
	app := &Application{
		cfg: cfg, log: testLog, infra: infra, shutdown: manager, ready: xprobe.NewBool(), fatal: make(chan error, 1),
	}
	services, err := buildServices(ctx, cfg, infra, manager, testLog)
	if err != nil {
		t.Fatalf("build services: %v", err)
	}
	app.services = services
	app.ready.Set(true)

	var handler http.Handler
	if cfg.Dev.Enabled() {
		handler, err = app.handler(ctx)
	} else {
		var webhook http.Handler
		webhook, err = newIAMWebhook(&cfg.IAM, app.services.IAM, app.services.Profiles, testLog)
		if err != nil {
			t.Fatalf("iam webhook: %v", err)
		}
		handler, err = app.httpHandler(verifierChain{tokens: app.services.Tokens, iam: nil}, webhook)
	}
	if err != nil {
		t.Fatalf("build handler: %v", err)
	}
	ts := httptest.NewUnstartedServer(handler)
	if opts.Addr != "" {
		l, err := net.Listen("tcp", opts.Addr)
		if err != nil {
			t.Fatalf("listen %s: %v", opts.Addr, err)
		}
		_ = ts.Listener.Close()
		ts.Listener = l
	}
	ts.Start()
	t.Cleanup(func() {
		ts.Close()
		cancel()
		_ = manager.Stop()
	})
	return &e2e{t: t, ts: ts, app: app, graphene: fake, ctx: ctx, pushes: pushes}
}

// person seeds a profile and returns an actor for direct service calls.
func (e *e2e) person(email, name string) auth.Actor {
	e.t.Helper()
	id := uuid.New()
	ctx := context.Background()
	if _, _, err := e.app.services.Profiles.Ensure(ctx, id, email); err != nil {
		e.t.Fatalf("ensure profile: %v", err)
	}
	if _, err := e.app.services.Profiles.Seed(ctx, id, email, name, ""); err != nil {
		e.t.Fatalf("seed profile: %v", err)
	}
	return auth.Actor{UserID: id, Email: email}
}

// tenant creates an owned tenant for the actor through the service (the
// fake door records the namespace).
func (e *e2e) tenant(owner auth.Actor, name string) tenant.Tenant {
	e.t.Helper()
	t, err := e.app.services.Tenants.Create(auth.WithActor(context.Background(), owner), owner, tenant.Create{Name: name})
	if err != nil {
		e.t.Fatalf("create tenant: %v", err)
	}
	return t
}

// member adds an actor to a tenant with a role.
func (e *e2e) member(t tenant.Tenant, a auth.Actor, role tenant.Role) {
	e.t.Helper()
	repo := repositories.NewTenantRepo(testDB.TxDB)
	if err := repo.UpsertMember(context.Background(), t.ID, a.UserID, role); err != nil {
		e.t.Fatalf("add member: %v", err)
	}
}

// token mints a personal API token for the actor in the tenant and returns
// the bearer value.
func (e *e2e) token(a auth.Actor, t tenant.Tenant) string {
	e.t.Helper()
	c, err := e.app.services.Tokens.CreatePersonal(auth.WithActor(context.Background(), a), a, "test", t.ID, "", nil)
	if err != nil {
		e.t.Fatalf("mint token: %v", err)
	}
	return c.Secret
}

// serviceToken mints a tenant service token (admin actor) with a role.
func (e *e2e) serviceToken(admin auth.Actor, t tenant.Tenant, role string) string {
	e.t.Helper()
	c, err := e.app.services.Tokens.CreateService(auth.WithActor(context.Background(), admin), admin, t.ID, "ci", role, nil)
	if err != nil {
		e.t.Fatalf("mint service token: %v", err)
	}
	return c.Secret
}

// resp is one HTTP answer.
type resp struct {
	Status int
	Body   []byte
}

// req performs a request; body (if non-nil) is JSON; token (if non-empty)
// is a bearer.
func (e *e2e) req(method, path string, body any, token string) resp {
	e.t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(raw)
	}
	r, err := http.NewRequestWithContext(context.Background(), method, e.ts.URL+path, rdr)
	if err != nil {
		e.t.Fatalf("request: %v", err)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return resp{Status: res.StatusCode, Body: raw}
}

// reqAccept is req with an Accept header (exports).
func (e *e2e) reqAccept(method, path, accept, token string) resp {
	e.t.Helper()
	r, err := http.NewRequestWithContext(context.Background(), method, e.ts.URL+path, http.NoBody)
	if err != nil {
		e.t.Fatalf("request: %v", err)
	}
	r.Header.Set("Accept", accept)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return resp{Status: res.StatusCode, Body: raw}
}

// want asserts the status and decodes the body into dst (nil = ignore).
func (e *e2e) want(r resp, status int, dst any) {
	e.t.Helper()
	if r.Status != status {
		e.t.Fatalf("status = %d, want %d\nbody: %s", r.Status, status, r.Body)
	}
	if dst != nil && len(r.Body) > 0 {
		if err := json.Unmarshal(r.Body, dst); err != nil {
			e.t.Fatalf("decode (status %d): %v\nbody: %s", r.Status, err, r.Body)
		}
	}
}

// problem decodes a Problem body and asserts its code.
func (e *e2e) problem(r resp, status int, code string) {
	e.t.Helper()
	var p struct {
		Code string `json:"code"`
	}
	e.want(r, status, &p)
	if p.Code != code {
		e.t.Fatalf("problem code = %q, want %q\nbody: %s", p.Code, code, r.Body)
	}
}

// eventually polls until cond holds or the deadline passes.
func eventually(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func slug(prefix string) string { return prefix + "-" + uuid.NewString()[:8] }
