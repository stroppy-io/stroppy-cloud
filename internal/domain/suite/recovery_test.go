package suite

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

type recoverySuites struct {
	Repository
	r SuiteRun
}

func (m *recoverySuites) RunByID(context.Context, uuid.UUID) (SuiteRun, error) { return m.r, nil }
func (m *recoverySuites) SetRunEvent(_ context.Context, _ uuid.UUID, id int64) error {
	m.r.LastEventID = id
	return nil
}
func (m *recoverySuites) SetRunStatus(_ context.Context, _ uuid.UUID, status run.Status, reason string, start, end *time.Time) error {
	m.r.Status, m.r.StatusReason, m.r.FinishedAt = status, reason, end
	if m.r.StartedAt == nil {
		m.r.StartedAt = start
	}
	return nil
}

type recoveryChildren struct {
	run.Repository
	children []run.Run
	changes  int
}

func (m *recoveryChildren) OfSuiteRun(context.Context, uuid.UUID) ([]run.Run, error) {
	return m.children, nil
}
func (m *recoveryChildren) SetStatus(_ context.Context, id uuid.UUID, status run.Status, _ run.Phase, _ string, _, _ *time.Time) error {
	for i := range m.children {
		if m.children[i].ID == id {
			m.children[i].Status = status
			m.changes++
		}
	}
	return nil
}

type recoveryDoor struct {
	run.Graphene
	starts             int
	ids                []string
	requests           []spec.Suite
	startErr, childErr error
}

func (g *recoveryDoor) StartRun(_ context.Context, id, pipeline string, params any, _ map[string]string) error {
	if pipeline != Pipeline {
		panic("suite recovery tried to launch a child")
	}
	g.starts++
	g.ids = append(g.ids, id)
	g.requests = append(g.requests, params.(spec.Suite))
	return g.startErr
}
func (g *recoveryDoor) Events(_ context.Context, _ string, after int64, _ bool, fn func(run.RawEvent) error) error {
	if after < 1 {
		return fn(run.RawEvent{ID: 1, Kind: "run-completed", At: time.Now().UTC()})
	}
	return nil
}
func (g *recoveryDoor) RunStatus(_ context.Context, id string) (string, error) {
	if len(g.ids) > 0 && id == g.ids[0] {
		return "completed", nil
	}
	return "completed", g.childErr
}

func suiteRecoveryFixture(t *testing.T) (*recoverySuites, *recoveryChildren, *recoveryDoor, *Projector) {
	t.Helper()
	id, a, b := uuid.New(), uuid.New(), uuid.New()
	r := SuiteRun{ID: id, Status: run.StatusPending, CreatedAt: time.Now().UTC(), Concurrency: 2, Labels: map[string]string{"experiment": "recovery"}, Cells: []CellRun{{CellID: "b", RunID: b}, {CellID: "a", RunID: a}}}
	encode := func(name string) json.RawMessage {
		raw, err := json.Marshal(spec.Run{RunID: name, Tenant: "original-tenant"})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	repo := &recoverySuites{r: r}
	children := &recoveryChildren{children: []run.Run{{ID: a, Status: run.StatusPending, CellID: "a", RunSpec: encode("a")}, {ID: b, Status: run.StatusPending, CellID: "b", RunSpec: encode("b")}}}
	g := &recoveryDoor{}
	p := NewProjector(repo, children, g, nil, func(ctx context.Context, _ string) context.Context { return ctx }, xlog.NewConsole(xlog.WithWriter(io.Discard)))
	return repo, children, g, p
}

func TestRecoverySuiteReplaysPersistedCellsInOriginalOrder(t *testing.T) {
	repo, children, g, p := suiteRecoveryFixture(t)
	g.startErr = errors.New("answer lost")
	p.follow(context.Background(), run.Live{ID: repo.r.ID})
	if repo.r.Status != run.StatusPending || children.changes != 0 {
		t.Fatal("lost answer terminated suite cells")
	}
	g.startErr = nil
	p.follow(context.Background(), run.Live{ID: repo.r.ID})
	if repo.r.Status != run.StatusCompleted || g.starts != 2 || g.ids[0] != g.ids[1] {
		t.Fatal("suite submission did not recover")
	}
	a, b := g.requests[0], g.requests[1]
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) || a.Cells[0].ID != "b" || a.Cells[1].ID != "a" || a.Tenant != "original-tenant" || !a.Defaults.ContinueOnFailure {
		t.Fatal("recovery changed committed request")
	}
	if children.changes != 0 {
		t.Fatal("existing children were overwritten")
	}
}

func TestRecoverySuiteStatusOutageDoesNotCancelChildren(t *testing.T) {
	repo, children, g, p := suiteRecoveryFixture(t)
	g.childErr = errors.New("Temporal unavailable")
	p.follow(context.Background(), run.Live{ID: repo.r.ID})
	if repo.r.Status.Terminal() || children.changes != 0 {
		t.Fatal("outage was mistaken for absent children")
	}
	g.childErr = run.ErrRemoteNotFound
	p.follow(context.Background(), run.Live{ID: repo.r.ID})
	if repo.r.Status != run.StatusCompleted || children.changes != 2 {
		t.Fatal("genuinely unstarted children were not reconciled")
	}
}
