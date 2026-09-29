package run

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
)

type fakeTelemetry struct {
	mu     sync.Mutex
	points []Point
	err    map[uuid.UUID]error
	calls  int
}

func (f *fakeTelemetry) QPSSeries(_ context.Context, r Run, from, to time.Time, step time.Duration) ([]Point, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if !to.After(from) || step < minSeriesStep {
		panic("bad window")
	}
	if err := f.err[r.ID]; err != nil {
		return nil, err
	}
	return f.points, nil
}

func TestFinishStoresQPSSeries(t *testing.T) {
	repo, _, p := recoveryFixture()
	tel := &fakeTelemetry{points: []Point{{T: 1, V: 10}, {T: 2, V: 12}}}
	p.WithTelemetry(tel)
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.Status != StatusCompleted || len(repo.r.Summary.QPSSeries) != 2 || tel.calls != 1 {
		t.Fatalf("series not stored: status=%s series=%v calls=%d", repo.r.Status, repo.r.Summary.QPSSeries, tel.calls)
	}
}

func TestFinishSurvivesTelemetryFailure(t *testing.T) {
	repo, _, p := recoveryFixture()
	p.WithTelemetry(&fakeTelemetry{err: map[uuid.UUID]error{repo.r.ID: errOutage}})
	p.follow(context.Background(), Live{ID: repo.r.ID})
	if repo.r.Status != StatusCompleted || len(repo.r.Result) == 0 {
		t.Fatal("a metric store failure blocked finalization")
	}
	if repo.r.Summary.QPSSeries != nil {
		t.Fatal("a failed sample must stay unset for the backfill")
	}
}

type backfillRepo struct {
	Repository
	mu     sync.Mutex
	runs   []Run
	stored map[uuid.UUID][]Point
}

func (b *backfillRepo) WithoutQPSSeries(context.Context, int) ([]Run, error) { return b.runs, nil }
func (b *backfillRepo) SetQPSSeries(_ context.Context, id uuid.UUID, s []Point) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stored[id] = s
	return nil
}

func TestBackfillQPSSeries(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	end := start.Add(10 * time.Minute)
	finished := func() Run { return Run{ID: uuid.New(), Status: StatusCompleted, StartedAt: &start, FinishedAt: &end} }
	ok, gone, down, never := finished(), finished(), finished(), Run{ID: uuid.New(), Status: StatusFailed}
	repo := &backfillRepo{runs: []Run{ok, gone, down, never}, stored: map[uuid.UUID][]Point{}}
	_, _, p := recoveryFixture()
	p.repo = repo
	p.WithTelemetry(&fakeTelemetry{points: []Point{{T: 1, V: 3}}, err: map[uuid.UUID]error{gone.ID: errs.NotFound("Graphene telemetry run"), down.ID: errOutage}})
	stored, empty, skipped := p.BackfillQPSSeries(context.Background(), 2)
	if stored != 1 || empty != 2 || skipped != 1 {
		t.Fatalf("stored=%d empty=%d skipped=%d", stored, empty, skipped)
	}
	if len(repo.stored[ok.ID]) != 1 {
		t.Fatal("series not stored")
	}
	for _, id := range []uuid.UUID{gone.ID, never.ID} {
		if s, marked := repo.stored[id]; !marked || s == nil || len(s) != 0 {
			t.Fatal("a run without telemetry must be marked with an empty series")
		}
	}
	if _, marked := repo.stored[down.ID]; marked {
		t.Fatal("a store outage must leave the run for the next pass")
	}
}
