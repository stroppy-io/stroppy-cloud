package run

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// lateRepo records whether a projection write got a live context.
type lateRepo struct {
	*recoveryRepo
	writes, expired int
}

func (m *lateRepo) SetProjection(ctx context.Context, id uuid.UUID, state State, cursor int64) error {
	m.writes++
	if ctx.Err() != nil {
		m.expired++
		return ctx.Err()
	}
	return m.recoveryRepo.SetProjection(ctx, id, state, cursor)
}

// lateGraphene delivers its event only once the stream's window closed, as
// one that arrives at the very end of the window.
type lateGraphene struct {
	*recoveryGraphene
	statusCalls int
}

func (g *lateGraphene) Events(ctx context.Context, _ string, after int64, _ bool, fn func(RawEvent) error) error {
	<-ctx.Done()
	for _, e := range g.events {
		if e.ID > after {
			if err := fn(e); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func (g *lateGraphene) RunStatus(ctx context.Context, id string) (string, error) {
	g.statusCalls++
	return g.recoveryGraphene.RunStatus(ctx, id)
}

// The window bounds reading the stream, not storing what it read: an event
// handed over as the window closes is still stored, and a window that ends
// normally asks Graphene nothing more.
func TestFollowWindowEndsReadingNotWriting(t *testing.T) {
	t.Parallel()
	base, g, p := recoveryFixture()
	repo := &lateRepo{recoveryRepo: base}
	graphene := &lateGraphene{recoveryGraphene: g}
	graphene.events = graphene.events[:1] // started, not finished
	p.repo, p.graphene = repo, graphene
	p.window, p.grace = 20*time.Millisecond, time.Second

	p.follow(context.Background(), Live{ID: base.r.ID})

	if repo.writes != 1 || repo.expired != 0 || base.r.LastEventID != 1 {
		t.Fatalf("writes %d, on an expired context %d, cursor %d", repo.writes, repo.expired, base.r.LastEventID)
	}
	if graphene.statusCalls != 0 {
		t.Fatalf("a window that ended normally asked the run status %d times", graphene.statusCalls)
	}
}
