package run

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"
)

var errOutage = errors.New("injected outage")

type recoveryRepo struct {
	Repository
	r        Run
	writeErr error
	events   map[int64]Event
}

func (m *recoveryRepo) ByID(context.Context, uuid.UUID) (Run, error) { return m.r, nil }
func (m *recoveryRepo) SetStatus(_ context.Context, _ uuid.UUID, status Status, phase Phase, reason string, start, end *time.Time) error {
	if !m.r.Status.Terminal() {
		m.r.Status, m.r.Phase, m.r.StatusReason = status, phase, reason
		if m.r.StartedAt == nil {
			m.r.StartedAt = start
		}
		m.r.FinishedAt = end
	}
	return nil
}
func (m *recoveryRepo) SetProjection(_ context.Context, _ uuid.UUID, state State, cursor int64) error {
	m.r.State, m.r.LastEventID = state, cursor
	return nil
}
func (m *recoveryRepo) InsertEvent(_ context.Context, e Event) (bool, error) {
	_, exists := m.events[e.GrapheneID]
	m.events[e.GrapheneID] = e
	return !exists, nil
}
func (m *recoveryRepo) SetResult(_ context.Context, _ uuid.UUID, raw json.RawMessage, summary Summary, tps *float64) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.r.Result, m.r.Summary, m.r.TPS = raw, summary, tps
	return nil
}
func (m *recoveryRepo) SetKeep(_ context.Context, _ uuid.UUID, kept bool, until *time.Time) error {
	m.r.StandKept, m.r.KeepUntil = kept, until
	return nil
}

type recoveryGraphene struct {
	Graphene
	startErr, closeErr, cancelErr, statusErr error
	starts, cancels                          int
	ids                                      []string
	events                                   []RawEvent
	status                                   string
	raw                                      json.RawMessage
	held                                     []Holding
}

func (g *recoveryGraphene) StartRun(_ context.Context, id, _ string, _ any, _ map[string]string) error {
	g.starts++
	g.ids = append(g.ids, id)
	return g.startErr
}
func (g *recoveryGraphene) Events(_ context.Context, _ string, after int64, _ bool, fn func(RawEvent) error) error {
	for _, e := range g.events {
		if e.ID > after {
			if err := fn(e); err != nil {
				return err
			}
		}
	}
	return nil
}
func (g *recoveryGraphene) RunStatus(context.Context, string) (string, error) {
	return g.status, g.statusErr
}
func (g *recoveryGraphene) RunClose(context.Context, string) (json.RawMessage, string, error) {
	return g.raw, "", g.closeErr
}
func (g *recoveryGraphene) CancelRun(context.Context, string) error             { g.cancels++; return g.cancelErr }
func (g *recoveryGraphene) Holdings(context.Context, string) ([]Holding, error) { return g.held, nil }

func recoveryFixture() (*recoveryRepo, *recoveryGraphene, *Projector) {
	now := time.Now().UTC()
	repo := &recoveryRepo{r: Run{ID: uuid.New(), Status: StatusPending, CreatedAt: now, RunSpec: json.RawMessage(`{}`)}, events: map[int64]Event{}}
	g := &recoveryGraphene{status: "completed", raw: json.RawMessage(`{"summary":{"tps":123},"metrics":{"count":{"value":9007199254740993}}}`), events: []RawEvent{{ID: 1, Kind: "run-started", At: now}, {ID: 2, Kind: "run-completed", At: now.Add(time.Second)}}}
	log := xlog.NewConsole(xlog.WithWriter(io.Discard))
	p := NewProjector(repo, g, nil, func(ctx context.Context, _ string) context.Context { return ctx }, log)
	return repo, g, p
}

func TestRecoveryResultBeforeTerminal(t *testing.T) {
	for _, fault := range []string{"rpc", "database"} {
		t.Run(fault, func(t *testing.T) {
			repo, g, p := recoveryFixture()
			if fault == "rpc" {
				g.closeErr = errOutage
			} else {
				repo.writeErr = errOutage
			}
			p.follow(context.Background(), Live{ID: repo.r.ID})
			if repo.r.Status.Terminal() || repo.r.LastEventID != 2 || len(repo.r.Result) != 0 {
				t.Fatal("failed finalization was lost from the active queue")
			}
			g.closeErr, repo.writeErr = nil, nil
			// A fresh follower resumes after the persisted terminal event cursor.
			p.follow(context.Background(), Live{ID: repo.r.ID})
			if repo.r.Status != StatusCompleted || string(repo.r.Result) != string(g.raw) || len(repo.events) != 2 || g.starts != 1 {
				t.Fatalf("recovery lost/replayed data: status=%s events=%d starts=%d result=%s", repo.r.Status, len(repo.events), g.starts, repo.r.Result)
			}
			if repo.r.FinishedAt == nil || !repo.r.FinishedAt.Equal(g.events[1].At) {
				t.Fatal("recovery replaced the actual finish time with observation time")
			}
		})
	}
}

func TestRecoverySubmissionSameIdentity(t *testing.T) {
	repo, g, p := recoveryFixture()
	g.startErr = errOutage
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.Status != StatusPending {
		t.Fatal("ambiguous submission became terminal")
	}
	g.startErr = nil
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.Status != StatusCompleted || g.starts != 2 || g.ids[0] != g.ids[1] {
		t.Fatal("submission did not recover under the same identity")
	}
}

func TestRecoveryNeverSubmitsSuiteCell(t *testing.T) {
	repo, g, p := recoveryFixture()
	id := uuid.New()
	repo.r.SuiteRunID = &id
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if g.starts != 0 || repo.r.Status != StatusCompleted {
		t.Fatal("projector independently launched a suite cell")
	}
}

func TestRecoveryCancellationDelivery(t *testing.T) {
	repo, g, p := recoveryFixture()
	repo.r.Status, repo.r.StartedAt = StatusCancelling, ptr(time.Now().UTC())
	g.cancelErr = errOutage
	g.events[1].Kind, g.status = "run-canceled", "canceled"
	finishedEvents := g.events
	g.events, g.status = nil, "running"
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.Status != StatusCancelling {
		t.Fatal("cancel delivery failure lost the command")
	}
	g.cancelErr = nil
	g.events, g.status = finishedEvents, "canceled"
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.Status != StatusCancelled || g.cancels != 2 || g.starts != 0 {
		t.Fatal("cancel recovery relaunched or failed to cancel")
	}
}

func TestRecoveryKeepUsesRemoteDeadline(t *testing.T) {
	repo, g, p := recoveryFixture()
	repo.r.Keep, repo.r.State.Stand = time.Hour, &StandState{Root: "network/n"}
	deadline := time.Now().UTC().Add(10 * time.Minute)
	g.held = []Holding{{Ref: "network/n", KeepUntil: &deadline}, {Ref: "artifact/a"}}
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if !repo.r.StandKept || repo.r.KeepUntil == nil || !repo.r.KeepUntil.Equal(deadline) {
		t.Fatal("recovery moved the keep deadline")
	}
	g.held = []Holding{{Ref: "artifact/a"}}
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.StandKept || len(repo.r.Result) == 0 {
		t.Fatal("expiry lost result or treated artifacts as infrastructure")
	}
}

func TestRecoveryOldMissingIdentityIsNotRelaunched(t *testing.T) {
	repo, g, p := recoveryFixture()
	repo.r.CreatedAt = time.Now().Add(-48 * time.Hour)
	g.statusErr = ErrRemoteNotFound
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if g.starts != 0 || repo.r.Status != StatusPending {
		t.Fatal("possibly forgotten execution was recreated")
	}
}
