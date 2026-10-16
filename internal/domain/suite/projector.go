package suite

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

// Projector follows every live suite run: its status is derived from its
// cells, which are ordinary runs (the run projector follows those).
type Projector struct {
	repo      Repository
	runRepo   run.Repository
	publisher run.Publisher
	log       *xlog.Logger

	mu     sync.Mutex
	active map[uuid.UUID]struct{}
	wg     sync.WaitGroup
}

// NewProjector wires the worker.
func NewProjector(repo Repository, runRepo run.Repository, publisher run.Publisher, log *xlog.Logger) *Projector {
	return &Projector{repo: repo, runRepo: runRepo, publisher: publisher, log: log, active: map[uuid.UUID]struct{}{}}
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

// follow derives the suite run's status from its cells: running once a
// cell started, finished when every cell is — cancelled when the suite was
// being cancelled, failed when a cell failed, completed otherwise.
func (p *Projector) follow(ctx context.Context, l run.Live) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r, err := p.repo.RunByID(ctx, l.ID)
	if err != nil || r.Status.Terminal() {
		return
	}
	cells, err := p.runRepo.OfSuiteRun(ctx, r.ID)
	if err != nil {
		p.log.Warn("suite projector: cells", xlog.String("suite_run", r.ID.String()), xlog.Error("error", err))
		return
	}
	var started *time.Time
	done, failed, cancelled := 0, 0, 0
	var finishedAt time.Time
	for _, c := range cells {
		if c.StartedAt != nil && (started == nil || c.StartedAt.Before(*started)) {
			started = c.StartedAt
		}
		if !c.Status.Terminal() {
			continue
		}
		done++
		switch c.Status { //nolint:exhaustive // completed cells only count as done
		case run.StatusFailed:
			failed++
		case run.StatusCancelled:
			cancelled++
		}
		if c.FinishedAt != nil && c.FinishedAt.After(finishedAt) {
			finishedAt = *c.FinishedAt
		}
	}
	if done < len(cells) || len(cells) == 0 {
		if r.Status == run.StatusPending && started != nil {
			if err := p.repo.SetRunStatus(ctx, r.ID, run.StatusRunning, "", started, nil); err != nil {
				p.log.Warn("suite projector: start", xlog.String("suite_run", r.ID.String()), xlog.Error("error", err))
			}
		}
		return
	}
	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}
	switch {
	case r.Status == run.StatusCancelling || cancelled == len(cells):
		p.finish(ctx, r, run.StatusCancelled, "", finishedAt)
	case failed > 0:
		p.finish(ctx, r, run.StatusFailed, fmt.Sprintf("%d of %d cells failed", failed, len(cells)), finishedAt)
	default:
		p.finish(ctx, r, run.StatusCompleted, "", finishedAt)
	}
}

// finish records the terminal status and tells the webhooks.
func (p *Projector) finish(ctx context.Context, r SuiteRun, status run.Status, reason string, at time.Time) {
	if err := p.repo.SetRunStatus(ctx, r.ID, status, reason, nil, ptr(at)); err != nil {
		p.log.Warn("suite projector: finish", xlog.String("suite_run", r.ID.String()), xlog.Error("error", err))
		return
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
