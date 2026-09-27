//go:build integration

package application

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres/repositories"
)

// Fail at the HTTP boundary, before dispatch or after the server committed
// the RPC. No real Graphene installation is disrupted by these tests.
type rpcFaults struct {
	mu        sync.Mutex
	method    string
	remaining int
	after     bool
	hits      int
}

func (f *rpcFaults) set(method string, count int, after bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.method, f.remaining, f.after, f.hits = method, count, after, 0
}
func (f *rpcFaults) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.hits }
func (f *rpcFaults) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		fail := f.remaining != 0 && strings.HasSuffix(r.URL.Path, "/"+f.method)
		after := f.after
		if fail {
			f.hits++
			if f.remaining > 0 {
				f.remaining--
			}
		}
		f.mu.Unlock()
		if !fail {
			next.ServeHTTP(w, r)
			return
		}
		if after {
			next.ServeHTTP(httptest.NewRecorder(), r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"code":"unavailable","message":"injected connection loss"}`)
	})
}

func (e *e2e) launchWithKey(path, key, tok string) resp {
	e.t.Helper()
	req, err := http.NewRequestWithContext(e.ctx, http.MethodPost, e.ts.URL+path, strings.NewReader(`{}`))
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	e.recordAcceptanceHTTP(http.MethodPost, path, r.StatusCode)
	return resp{Status: r.StatusCode, Body: body}
}

func TestE2ESubmissionFaultRecovery(t *testing.T) {
	for _, kind := range []string{"run", "suite"} {
		for _, after := range []bool{false, true} {
			name := kind + " unavailable before acceptance"
			if after {
				name = kind + " accepted but answer lost"
			}
			t.Run(name, func(t *testing.T) {
				e := e2eServer(t)
				base, tok, testID, _ := runFixtureWith(t, e, nil, "0s")
				path := base + "/tests/" + testID + ":launch"
				if kind == "suite" {
					var s struct{ ID string }
					e.want(e.req(http.MethodPost, base+"/suites", map[string]any{"name": "recovery", "tests": []any{map[string]any{"ref": map[string]any{"id": testID}}}, "concurrency": 1}, tok), http.StatusCreated, &s)
					path = base + "/suites/" + s.ID + ":launch"
				}
				// Keep the fault active through the client's immediate RPC retries.
				e.faults.set("StartRun", -1, after)
				var first, again struct{ ID, Status string }
				e.want(e.launchWithKey(path, "recovery-"+testID, tok), http.StatusCreated, &first)
				e.want(e.launchWithKey(path, "recovery-"+testID, tok), http.StatusCreated, &again)
				if first.ID != again.ID || first.Status != "pending" || e.faults.count() < 3 {
					t.Fatal("lost submission or duplicate server row")
				}
				e.faults.set("", 0, false)
				if kind == "run" {
					r := finishRun(t, e, base, tok, first.ID)
					if r.Status != "completed" || len(r.Result.Segments) == 0 {
						t.Fatal("run result missing")
					}
				} else {
					eventually(t, 10*time.Second, func() bool {
						e.app.services.SuiteProj.Tick(e.ctx)
						e.app.services.Projector.Tick(e.ctx)
						var view suiteRunView
						e.want(e.req(http.MethodGet, base+"/suite-runs/"+first.ID, nil, tok), http.StatusOK, &view)
						return view.Status == "completed" && view.Progress.Done == 1
					})
				}
			})
		}
	}
}

func TestE2EResultOutageRecovery(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixtureWith(t, e, nil, "0s")
	var r runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok), http.StatusCreated, &r)
	e.faults.set("RunResult", -1, false)
	e.app.services.Projector.Tick(e.ctx)
	eventually(t, 10*time.Second, func() bool { return e.faults.count() > 0 && !e.app.services.Projector.Following(uuid.MustParse(r.ID)) })
	e.want(e.req(http.MethodGet, base+"/runs/"+r.ID, nil, tok), http.StatusOK, &r)
	if r.Status == "completed" || r.Status == "failed" {
		t.Fatal("terminal row lost its result retry")
	}
	e.faults.set("", 0, false)
	r = finishRun(t, e, base, tok, r.ID)
	if r.Status != "completed" || len(r.Result.Segments) == 0 {
		t.Fatal("result not recovered")
	}
}

func TestE2ECancelCommandSurvivesStaleProjection(t *testing.T) {
	e := e2eServer(t)
	base, tok, testID, _ := runFixtureWith(t, e, nil, "0s")
	e.graphene.hold("segment.started")
	var r runView
	e.want(e.req(http.MethodPost, base+"/tests/"+testID+":launch", nil, tok), http.StatusCreated, &r)
	e.faults.set("CancelRun", -1, false)
	e.problem(e.req(http.MethodPost, base+"/runs/"+r.ID+":cancel", nil, tok), http.StatusServiceUnavailable, "unavailable")
	if e.faults.count() < 3 {
		t.Fatal("cancellation outage did not cover immediate RPC retries")
	}
	e.faults.set("", 0, false)
	repo := repositories.NewRunRepo(testDB.TxDB, postgres.NewTransactor(testDB.Tx))
	id := uuid.MustParse(r.ID)
	// A stale stream can report running after HTTP persisted cancellation.
	if err := repo.SetStatus(e.ctx, id, run.StatusRunning, run.PhaseWorkload, "", nil, nil); err != nil {
		t.Fatal(err)
	}
	row, err := repo.ByID(e.ctx, id)
	if err != nil || row.Status != run.StatusCancelling {
		t.Fatal("stale observer erased cancellation")
	}
	r = finishRun(t, e, base, tok, r.ID)
	if r.Status != "cancelled" {
		t.Fatal("lost cancel RPC was not retried")
	}
}
