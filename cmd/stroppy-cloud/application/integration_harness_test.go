//go:build integration

package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/gopherex/xprobe"
	"github.com/gopherex/xshutdown"

	managementv1 "github.com/graphene-ci/graphene/pkg/proto/management/v1"
	"github.com/graphene-ci/graphene/pkg/proto/management/v1/managementv1connect"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/provider"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/tenant"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/graphene"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/victoria"
)

/*
HARNESS: the real application core (services over testDB) behind the real
transport, with two substitutions — the IAM verifier (tokens are `stc_`
API tokens minted through the token service, so no IAM is needed) and the
Graphene door (fakeGraphene, an in-process Connect server that records
namespaces/secrets and answers the probe pipelines with canned results).
*/

// fakeGraphene is the Connect half of a Graphene door, in memory.
type fakeGraphene struct {
	managementv1connect.UnimplementedResourcesAPIHandler
	managementv1connect.UnimplementedSecretsAPIHandler
	managementv1connect.UnimplementedRunsAPIHandler
	managementv1connect.UnimplementedRbacAPIHandler
	managementv1connect.UnimplementedObserveAPIHandler

	mu         sync.Mutex
	cond       *sync.Cond
	namespaces map[string]bool
	secrets    map[string]string // "<ns>/<name>" → value
	runs       map[string]*fakeRun
	commands   []string // "<ref> <command>" of every Invoke
	deleted    []string // refs deleted
	// verifyOK / quotas shape the probe pipelines' answers.
	verifyOK               bool
	quotas                 []provider.Quota
	quotaUnavailableReason string
	quotaError             error
	nextID                 int64
}

// fakeRun is one started pipeline run; stroppy-run runs carry a scripted
// event stream the test feeds through emit/milestone/finish.
type fakeRun struct {
	namespace string
	pipeline  string
	params    json.RawMessage
	status    string
	events    []*managementv1.Event
	result    json.RawMessage
	done      bool
}

func newFakeGraphene() *fakeGraphene {
	f := &fakeGraphene{
		namespaces: map[string]bool{}, secrets: map[string]string{}, runs: map[string]*fakeRun{},
		verifyOK: true, quotas: []provider.Quota{{Name: "compute.instanceCores.count", Limit: 32, Used: 4, Unit: "cores"}},
	}
	f.cond = sync.NewCond(&f.mu)
	return f
}

// emit appends a raw event to a run's stream and wakes followers.
func (f *fakeGraphene) emit(runID, kind, subject string, input any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.runs[runID]
	if r == nil {
		return
	}
	f.nextID++
	ev := &managementv1.Event{EventId: f.nextID, TimeUnixNano: time.Now().UnixNano(), Kind: kind, Subject: subject}
	if input != nil {
		ev.Input, _ = json.Marshal(input)
	}
	r.events = append(r.events, ev)
	f.cond.Broadcast()
}

// milestone emits a pipeline milestone the way workerapi.EmitEvent lands.
func (f *fakeGraphene) milestone(runID, name string, payload map[string]any) {
	f.emit(runID, "signal-received", "entity-note", map[string]any{"name": name, "payload": payload})
}

// finish ends a run with a terminal kind and result.
func (f *fakeGraphene) finish(runID, kind, status string, result any) {
	f.emit(runID, kind, "", nil)
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.runs[runID]; r != nil {
		r.status, r.done = status, true
		r.result, _ = json.Marshal(result)
	}
	f.cond.Broadcast()
}

// startChild registers a run another pipeline started (suite cells).
func (f *fakeGraphene) startChild(runID, ns string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.runs[runID]; !ok {
		f.runs[runID] = &fakeRun{namespace: ns, pipeline: "stroppy-run"}
	}
}

// runOf reads a recorded run.
func (f *fakeGraphene) runOf(runID string) (fakeRun, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[runID]
	if !ok {
		return fakeRun{}, false
	}
	return *r, true
}

// Events streams a run's events after an id; follow blocks until done.
func (f *fakeGraphene) Events(ctx context.Context, req *connect.Request[managementv1.EventsRequest], stream *connect.ServerStream[managementv1.Event]) error {
	runID := req.Msg.GetRef()[len("run/"):]
	after := req.Msg.GetAfterEventId()
	stop := context.AfterFunc(ctx, func() { f.mu.Lock(); f.cond.Broadcast(); f.mu.Unlock() })
	defer stop()
	for {
		f.mu.Lock()
		r, ok := f.runs[runID]
		if !ok {
			f.mu.Unlock()
			return connect.NewError(connect.CodeNotFound, nil)
		}
		var batch []*managementv1.Event
		for _, ev := range r.events {
			if ev.EventId > after {
				batch = append(batch, ev)
				after = ev.EventId
			}
		}
		done := r.done
		if len(batch) == 0 && !done && req.Msg.GetFollow() && ctx.Err() == nil {
			f.cond.Wait()
			f.mu.Unlock()
			continue
		}
		f.mu.Unlock()
		for _, ev := range batch {
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
		if done || !req.Msg.GetFollow() || ctx.Err() != nil {
			return nil
		}
	}
}

func (f *fakeGraphene) GetRun(_ context.Context, req *connect.Request[managementv1.GetRunRequest]) (*connect.Response[managementv1.GetRunResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[req.Msg.GetRunId()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	status := r.status
	if status == "" {
		status = "running"
	}
	return connect.NewResponse(&managementv1.GetRunResponse{Status: status}), nil
}

func (f *fakeGraphene) CancelRun(_ context.Context, req *connect.Request[managementv1.CancelRunRequest]) (*connect.Response[managementv1.CancelRunResponse], error) {
	f.mu.Lock()
	r, ok := f.runs[req.Msg.GetRunId()]
	f.mu.Unlock()
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	if !r.done {
		f.finish(req.Msg.GetRunId(), "run-canceled", "canceled", map[string]any{})
	}
	return connect.NewResponse(&managementv1.CancelRunResponse{}), nil
}

func (f *fakeGraphene) Tree(_ context.Context, req *connect.Request[managementv1.TreeRequest]) (*connect.Response[managementv1.TreeResponse], error) {
	owner := req.Msg.GetOwner()
	root := &managementv1.TreeNode{Resource: &managementv1.Resource{Ref: owner, Kind: "run", Phase: "running"}, Children: []*managementv1.TreeNode{
		{Resource: &managementv1.Resource{Ref: "vm/db-1", Kind: "vm", Phase: "ready", Owner: owner, Labels: map[string]string{"stroppy.io/role": "db"}}},
	}}
	return connect.NewResponse(&managementv1.TreeResponse{Roots: []*managementv1.TreeNode{root}}), nil
}

func (f *fakeGraphene) List(_ context.Context, req *connect.Request[managementv1.ListRequest]) (*connect.Response[managementv1.ListResponse], error) {
	sel := req.Msg.GetSelector()
	if sel.GetKind() != "artifact" {
		return connect.NewResponse(&managementv1.ListResponse{}), nil
	}
	state, _ := json.Marshal(map[string]any{"name": "stroppy-raw.json", "kind": "stroppy_raw", "blob": map[string]any{"contentType": "application/json", "size": 2}})
	return connect.NewResponse(&managementv1.ListResponse{Resources: []*managementv1.Resource{{Ref: "artifact/" + sel.GetOwner()[len("run/"):] + "-raw", Kind: "artifact", Owner: sel.GetOwner(), State: state}}}), nil
}

func (f *fakeGraphene) Download(_ context.Context, req *connect.Request[managementv1.DownloadRequest], stream *connect.ServerStream[managementv1.DownloadChunk]) error {
	return stream.Send(&managementv1.DownloadChunk{Data: []byte("{}")})
}

func (f *fakeGraphene) Invoke(_ context.Context, req *connect.Request[managementv1.InvokeRequest]) (*connect.Response[managementv1.InvokeResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, req.Msg.GetRef()+" "+req.Msg.GetCommand())
	return connect.NewResponse(&managementv1.InvokeResponse{}), nil
}

func nsOf(req connect.AnyRequest) string { return req.Header().Get(graphene.NamespaceHeader) }

func (f *fakeGraphene) Apply(_ context.Context, req *connect.Request[managementv1.ApplyRequest]) (*connect.Response[managementv1.ApplyResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Msg.GetKind() == "namespace" {
		f.namespaces[req.Msg.GetId()] = true
	}
	return connect.NewResponse(&managementv1.ApplyResponse{Ref: req.Msg.GetKind() + "/" + req.Msg.GetId()}), nil
}

func (f *fakeGraphene) Delete(_ context.Context, req *connect.Request[managementv1.DeleteRequest]) (*connect.Response[managementv1.DeleteResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ref := req.Msg.GetRef()
	f.deleted = append(f.deleted, ref)
	switch {
	case len(ref) > len("namespace/") && ref[:len("namespace/")] == "namespace/":
		delete(f.namespaces, ref[len("namespace/"):])
	case len(ref) > len("secret/") && ref[:len("secret/")] == "secret/":
		delete(f.secrets, nsOf(req)+"/"+ref[len("secret/"):])
	}
	return connect.NewResponse(&managementv1.DeleteResponse{}), nil
}

func (f *fakeGraphene) SetSecret(_ context.Context, req *connect.Request[managementv1.SetSecretRequest]) (*connect.Response[managementv1.SetSecretResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.secrets[nsOf(req)+"/"+req.Msg.GetName()] = req.Msg.GetValue()
	return connect.NewResponse(&managementv1.SetSecretResponse{Version: 1}), nil
}

func (f *fakeGraphene) WhoAmI(context.Context, *connect.Request[managementv1.WhoAmIRequest]) (*connect.Response[managementv1.WhoAmIResponse], error) {
	return connect.NewResponse(&managementv1.WhoAmIResponse{Subject: "sa:stroppy", Namespace: "*"}), nil
}

func (f *fakeGraphene) StartRun(_ context.Context, req *connect.Request[managementv1.StartRunRequest]) (*connect.Response[managementv1.StartRunResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs[req.Msg.GetRunId()] = &fakeRun{namespace: nsOf(req), pipeline: req.Msg.GetPipeline(), params: req.Msg.GetParams()}
	return connect.NewResponse(&managementv1.StartRunResponse{WorkflowId: req.Msg.GetRunId()}), nil
}

func (f *fakeGraphene) RunResult(_ context.Context, req *connect.Request[managementv1.RunResultRequest]) (*connect.Response[managementv1.RunResultResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.runs[req.Msg.GetRunId()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	var result any
	switch run.pipeline {
	case graphene.PipelineProviderVerify:
		result = provider.VerifyResult{OK: f.verifyOK, AccountID: "acc-1", Permissions: []provider.Permission{{Name: "compute.instances.list", Granted: f.verifyOK}}, Error: map[bool]string{true: "", false: "missing rights"}[f.verifyOK]}
	case graphene.PipelineQuotas:
		if f.quotaError != nil {
			return nil, f.quotaError
		}
		result = provider.QuotasResult{ObservedAt: time.Now().UTC(), Quotas: f.quotas, UnavailableReason: f.quotaUnavailableReason, Scope: "cloud:b1gcloud000000000000"}
	default:
		if len(run.result) > 0 {
			return connect.NewResponse(&managementv1.RunResultResponse{Result: run.result}), nil
		}
		result = map[string]any{}
	}
	raw, _ := json.Marshal(result)
	return connect.NewResponse(&managementv1.RunResultResponse{Result: raw}), nil
}

// secret returns a stored secret value.
func (f *fakeGraphene) secret(ns, name string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.secrets[ns+"/"+name]
	return v, ok
}

func (f *fakeGraphene) hasNamespace(ns string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.namespaces[ns]
}

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
	// victoria is the fake telemetry store.
	victoria *fakeVictoria
}

// fakeVictoria answers VictoriaLogs and VictoriaMetrics queries with
// canned data and records the queries it saw.
type fakeVictoria struct {
	mu      sync.Mutex
	queries []string
	// lines are the log records returned for every query (NDJSON).
	lines []map[string]string
}

func (f *fakeVictoria) last() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.queries) == 0 {
		return ""
	}
	return f.queries[len(f.queries)-1]
}

func (f *fakeVictoria) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	f.queries = append(f.queries, r.URL.Path+" "+r.Form.Get("query"))
	lines := f.lines
	f.mu.Unlock()
	switch r.URL.Path {
	case "/select/logsql/query":
		w.Header().Set("Content-Type", "application/stream+json")
		enc := json.NewEncoder(w)
		for _, l := range lines {
			_ = enc.Encode(l)
		}
	case "/select/logsql/facets":
		_ = json.NewEncoder(w).Encode(map[string]any{"facets": []any{
			map[string]any{"field_name": "role", "values": []any{map[string]any{"field_value": "db", "hits": 3}, map[string]any{"field_value": "runner", "hits": 1}}},
			map[string]any{"field_name": "_stream", "values": []any{map[string]any{"field_value": "x", "hits": 1}}},
		}})
	case "/api/v1/query_range":
		if strings.Contains(r.Form.Get("query"), "boom") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"status":"error","error":"boom"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []any{
			map[string]any{"metric": map[string]string{"machine": "db-1", "role": "db"}, "values": []any{[]any{1.0, "10"}, []any{2.0, "30"}, []any{3.0, "20"}}},
		}}})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// fakePusher stands in for the `stroppy-* push` subprocesses.
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
	fake := newFakeGraphene()
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
	pushes := &fakePusher{}
	cfg.Infra.Pipelines.Runner = pushes
	cfg.Infra.Graphene = graphene.Config{Address: door.Listener.Addr().String(), Token: "test", Insecure: true}
	victoriaFake := &fakeVictoria{}
	vsrv := httptest.NewServer(victoriaFake)
	t.Cleanup(vsrv.Close)
	cfg.Infra.Victoria = victoria.Config{LogsURL: vsrv.URL, MetricsURL: vsrv.URL, RunLabel: "stroppy_run_id", Timeout: 5 * time.Second}
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

	webhook, err := newIAMWebhook(&cfg.IAM, app.services.IAM, app.services.Profiles, testLog)
	if err != nil {
		t.Fatalf("iam webhook: %v", err)
	}
	handler, err := app.httpHandler(verifierChain{tokens: app.services.Tokens, iam: nil}, webhook)
	if err != nil {
		t.Fatalf("build handler: %v", err)
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
		cancel()
		_ = manager.Stop()
	})
	return &e2e{t: t, ts: ts, app: app, graphene: fake, ctx: ctx, pushes: pushes, victoria: victoriaFake}
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
