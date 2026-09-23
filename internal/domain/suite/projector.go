package suite

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// Projector follows the event stream of every live suite run: the suite's
// own status comes from its run-* events, the cells from their own runs
// (the run projector follows those).
type Projector struct {
	repo      Repository
	runRepo   run.Repository
	graphene  run.Graphene
	publisher run.Publisher
	scope     func(ctx context.Context, namespace string) context.Context
	log       *xlog.Logger

	mu     sync.Mutex
	active map[uuid.UUID]struct{}
	wg     sync.WaitGroup
}

// NewProjector wires the worker.
func NewProjector(repo Repository, runRepo run.Repository, g run.Graphene, publisher run.Publisher, scope func(context.Context, string) context.Context, log *xlog.Logger) *Projector {
	return &Projector{repo: repo, runRepo: runRepo, graphene: g, publisher: publisher, scope: scope, log: log, active: map[uuid.UUID]struct{}{}}
}

// Run ticks until ctx ends.
func (p *Projector) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		p.Tick(ctx)
		select {
		case <-ctx.Done():
			p.wg.Wait()
			return
		case <-t.C:
		}
	}
}

// Tick starts followers for unfollowed live suite runs.
func (p *Projector) Tick(ctx context.Context) int {
	live, err := p.repo.LiveRuns(ctx)
	if err != nil {
		p.log.Warn("suite projector: live runs", xlog.Error("error", err))
		return 0
	}
	n := 0
	for _, l := range live {
		p.mu.Lock()
		_, busy := p.active[l.ID]
		if !busy {
			p.active[l.ID] = struct{}{}
		}
		p.mu.Unlock()
		if busy {
			continue
		}
		n++
		p.wg.Add(1)
		go func(l run.Live) {
			defer p.wg.Done()
			defer func() { p.mu.Lock(); delete(p.active, l.ID); p.mu.Unlock() }()
			p.follow(ctx, l)
		}(l)
	}
	return n
}

// Following reports whether a suite run has a follower (tests).
func (p *Projector) Following(id uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.active[id]
	return ok
}

var errFinished = errors.New("suite finished")

func (p *Projector) follow(ctx context.Context, l run.Live) {
	r, err := p.repo.RunByID(ctx, l.ID)
	if err != nil || r.Status.Terminal() {
		return
	}
	sctx := p.scope(ctx, r.GrapheneNamespace)
	lastID, status := r.LastEventID, r.Status
	finished := false
	err = p.graphene.Events(sctx, r.ID.String(), lastID, true, func(e run.RawEvent) error {
		if e.ID <= lastID {
			return nil
		}
		lastID = e.ID
		_ = p.repo.SetRunEvent(ctx, r.ID, lastID) //nolint:errcheck // best-effort
		switch e.Kind {
		case "run-started":
			if status == run.StatusPending {
				status = run.StatusRunning
				return p.repo.SetRunStatus(ctx, r.ID, status, "", ptr(e.At), nil)
			}
		case "run-completed":
			finished = true
			p.finish(ctx, r, run.StatusCompleted, "", e.At)
			return errFinished
		case "run-failed", "run-timed-out", "run-terminated":
			finished = true
			p.finish(ctx, r, run.StatusFailed, orDefault(e.Error, e.Kind), e.At)
			return errFinished
		case "run-canceled", "run-cancelled":
			finished = true
			p.finish(ctx, r, run.StatusCancelled, "", e.At)
			return errFinished
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFinished) && ctx.Err() == nil {
		p.log.Debug("suite projector: events", xlog.String("suite_run", r.ID.String()), xlog.Error("error", err))
	}
	if finished || ctx.Err() != nil {
		return
	}
	st, err := p.graphene.RunStatus(sctx, r.ID.String())
	if err != nil {
		return
	}
	if final, ok := run.TerminalOf(st); ok {
		reason := ""
		if final == run.StatusFailed {
			reason = st
		}
		p.finish(ctx, r, final, reason, time.Now().UTC())
	}
}

// finish records the terminal status; a failed suite with every cell
// completed is a partial failure of the pipeline, not of the cells.
func (p *Projector) finish(ctx context.Context, r SuiteRun, status run.Status, reason string, at time.Time) {
	if err := p.repo.SetRunStatus(ctx, r.ID, status, reason, nil, ptr(at)); err != nil {
		p.log.Warn("suite projector: finish", xlog.String("suite_run", r.ID.String()), xlog.Error("error", err))
		return
	}
	// Cells the pipeline never started (Graphene does not know them) would
	// stay pending forever; cells Graphene knows are finished by their own
	// follower from their own events.
	sctx := p.scope(ctx, r.GrapheneNamespace)
	if children, err := p.runRepo.OfSuiteRun(ctx, r.ID); err == nil {
		for _, c := range children {
			if c.Status.Terminal() {
				continue
			}
			if _, err := p.graphene.RunStatus(sctx, c.GrapheneID()); err == nil {
				continue
			}
			final := run.StatusCancelled
			if status == run.StatusFailed && c.Status == run.StatusPending {
				final = run.StatusFailed
			}
			_ = p.runRepo.SetStatus(ctx, c.ID, final, run.PhaseDone, "suite "+string(status), nil, ptr(at)) //nolint:errcheck // best-effort
		}
	}
	if p.publisher == nil {
		return
	}
	cur, err := p.repo.RunByID(ctx, r.ID)
	if err != nil {
		cur = r
	}
	cur.Runs, _ = p.runRepo.OfSuiteRun(ctx, r.ID) //nolint:errcheck // best-effort
	event := EventSuiteFinished
	if status != run.StatusCompleted {
		event = EventSuiteFailed
	}
	pr := cur.Progress()
	_ = p.publisher.Publish(ctx, r.TenantID, event, map[string]any{"id": cur.ID, "name": cur.Name, "status": status, "status_reason": reason, "suite_id": cur.SuiteID, "progress": map[string]int{"total": pr.Total, "done": pr.Done, "failed": pr.Failed, "cancelled": pr.Cancelled}}) //nolint:errcheck // best-effort
}

func ptr(t time.Time) *time.Time { return &t }
