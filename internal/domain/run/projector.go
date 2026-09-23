package run

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gopherex/xlog"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Projector follows the Graphene event stream of every live run and folds
// it into the stored projection: timeline events, runtime state, status
// and phase, the result when the run finishes. One goroutine per live
// run; Tick starts followers for runs nobody follows yet (fresh launches,
// runs inherited after a restart).
type Projector struct {
	repo      Repository
	graphene  Graphene
	publisher Publisher
	scope     func(ctx context.Context, namespace string) context.Context
	log       *xlog.Logger

	mu     sync.Mutex
	active map[uuid.UUID]struct{}
	wg     sync.WaitGroup
}

// NewProjector wires the worker.
func NewProjector(repo Repository, g Graphene, publisher Publisher, scope func(context.Context, string) context.Context, log *xlog.Logger) *Projector {
	return &Projector{repo: repo, graphene: g, publisher: publisher, scope: scope, log: log, active: map[uuid.UUID]struct{}{}}
}

// Run ticks until ctx ends, then waits for the followers.
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

// Tick starts a follower for every live run without one. Returns how many
// were started.
func (p *Projector) Tick(ctx context.Context) int {
	live, err := p.repo.LiveRuns(ctx)
	if err != nil {
		p.log.Warn("projector: live runs", xlog.Error("error", err))
		return 0
	}
	started := 0
	for _, l := range live {
		if p.claim(l.ID) {
			started++
			p.wg.Add(1)
			go func(l Live) {
				defer p.wg.Done()
				defer p.release(l.ID)
				p.follow(ctx, l)
			}(l)
		}
	}
	return started
}

func (p *Projector) claim(id uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.active[id]; ok {
		return false
	}
	p.active[id] = struct{}{}
	return true
}

func (p *Projector) release(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, id)
}

// Following reports whether a run has a follower (tests).
func (p *Projector) Following(id uuid.UUID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.active[id]
	return ok
}

var errFinished = errors.New("run finished")

// follow streams events after the last projected id and applies them.
// When the stream closes without a terminal event the run status is
// asked once; a terminal answer finishes the run, anything else leaves it
// for the next tick.
func (p *Projector) follow(ctx context.Context, l Live) {
	r, err := p.repo.ByID(ctx, l.ID)
	if err != nil {
		p.log.Warn("projector: read run", xlog.String("run", l.ID.String()), xlog.Error("error", err))
		return
	}
	if r.Status.Terminal() {
		return
	}
	sctx := p.scope(ctx, r.GrapheneNamespace)
	state, status, lastID := r.State, r.Status, r.LastEventID
	finished := false
	err = p.graphene.Events(sctx, r.GrapheneID(), lastID, true, func(e RawEvent) error {
		if e.ID <= lastID {
			return nil
		}
		lastID = e.ID
		out := state.Apply(status, e)
		if out.Timeline != nil {
			ev := *out.Timeline
			ev.RunID = r.ID
			if _, err := p.repo.InsertEvent(ctx, ev); err != nil {
				return err
			}
		}
		if err := p.repo.SetProjection(ctx, r.ID, state, lastID); err != nil {
			return err
		}
		if out.Status != "" && (out.Status != status || out.Phase != "") {
			var startedAt, finishedAt *time.Time
			if out.Status == StatusRunning && status == StatusPending {
				startedAt = ptr(e.At)
			}
			if out.Finished {
				finishedAt = ptr(e.At)
			}
			phase := out.Phase
			if phase == "" {
				phase = r.Phase
			}
			// A cancel in flight keeps its status until the run ends.
			if status == StatusCancelling && !out.Finished {
				out.Status = StatusCancelling
			}
			if err := p.repo.SetStatus(ctx, r.ID, out.Status, phase, out.Reason, startedAt, finishedAt); err != nil {
				return err
			}
			status = out.Status
		}
		if out.Finished {
			finished = true
			p.finish(ctx, sctx, r, status, out.Reason)
			return errFinished
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFinished) && ctx.Err() == nil {
		// A suite cell the pipeline has not started yet answers not_found:
		// the next tick retries.
		p.log.Debug("projector: events", xlog.String("run", r.ID.String()), xlog.Error("error", err))
	}
	if finished || ctx.Err() != nil {
		return
	}
	// Stream closed without a terminal event: ask once.
	st, err := p.graphene.RunStatus(sctx, r.GrapheneID())
	if err != nil {
		return
	}
	if final, ok := TerminalOf(st); ok {
		state.finishPhases(string(final))
		_ = p.repo.SetProjection(ctx, r.ID, state, lastID) //nolint:errcheck // best-effort
		reason := ""
		if final == StatusFailed {
			reason = st
		}
		if err := p.repo.SetStatus(ctx, r.ID, final, PhaseDone, reason, nil, ptr(time.Now().UTC())); err != nil {
			p.log.Warn("projector: finish", xlog.String("run", r.ID.String()), xlog.Error("error", err))
			return
		}
		p.finish(ctx, sctx, r, final, reason)
	}
}

// TerminalOf maps a Graphene phase to the server's terminal status.
// Graphene speaks one lowercase vocabulary for runs and records alike:
// running, completed, failed, canceled, terminated, timed-out, deleted.
func TerminalOf(s string) (Status, bool) {
	switch strings.ToLower(s) {
	case "completed":
		return StatusCompleted, true
	case "failed", "timed-out", "terminated":
		return StatusFailed, true
	case "canceled":
		return StatusCancelled, true
	}
	return "", false
}

// finish stores the result of a finished run and tells the webhooks.
func (p *Projector) finish(ctx, sctx context.Context, r Run, status Status, reason string) {
	// A run that did not complete still collected something: its partial
	// result rides in the failure. Keep the raw envelope so explicit zeroes
	// and opaque report extensions survive storage.
	raw, failure, err := p.graphene.RunClose(sctx, r.GrapheneID())
	switch {
	case err != nil:
		p.log.Warn("projector: result", xlog.String("run", r.ID.String()), xlog.Error("error", err))
	case len(raw) == 0:
		// Nothing was collected (the run died before its first phase).
		p.log.Debug("projector: no result", xlog.String("run", r.ID.String()), xlog.String("failure", failure))
	default:
		var res spec.Result
		if err := json.Unmarshal(raw, &res); err != nil {
			p.log.Warn("projector: decode result", xlog.String("run", r.ID.String()), xlog.Error("error", err))
		} else {
			summary := r.Summary
			if status == StatusCompleted {
				summary.ProgressPct = 100
			}
			summary.Headline = headlineOf(res)
			var tps *float64
			if status == StatusCompleted && res.Summary.TPS > 0 {
				v := res.Summary.TPS
				tps = &v
			}
			if err := p.repo.SetResult(ctx, r.ID, raw, summary, tps); err != nil {
				p.log.Warn("projector: store result", xlog.String("run", r.ID.String()), xlog.Error("error", err))
			}
		}
	}
	// The stand is kept when the pipeline says so (stand.kept), not
	// because keep was asked: a failed run tears everything down.
	if cur, err := p.repo.ByID(ctx, r.ID); err == nil && cur.State.Stand != nil && r.Keep > 0 {
		until := time.Now().UTC().Add(r.Keep)
		_ = p.repo.SetKeep(ctx, r.ID, true, &until) //nolint:errcheck // best-effort
	}

	if p.publisher == nil {
		return
	}
	event := EventRunFinished
	switch status { //nolint:exhaustive // finish sees terminal statuses only
	case StatusFailed:
		event = EventRunFailed
	case StatusCancelled:
		event = EventRunCancelled
	}
	cur, err := p.repo.ByID(ctx, r.ID)
	if err != nil {
		cur = r
	}
	payload := map[string]any{"id": cur.ID, "name": cur.Name, "status": status, "status_reason": reason, "summary": cur.Summary, "test_id": cur.TestID, "suite_run_id": cur.SuiteRunID, "tps": cur.TPS}
	_ = p.publisher.Publish(ctx, r.TenantID, event, payload) //nolint:errcheck // delivery is best-effort
}

// headlineOf picks the headline numbers of a result.
func headlineOf(res spec.Result) map[string]float64 {
	out := map[string]float64{}
	if res.Summary.TPS > 0 {
		out["tps"] = res.Summary.TPS
	}
	if res.Summary.LatencyP50Ms > 0 {
		out["latency_p50_ms"] = res.Summary.LatencyP50Ms
	}
	if res.Summary.LatencyP95Ms > 0 {
		out["latency_p95_ms"] = res.Summary.LatencyP95Ms
	}
	if res.Summary.LatencyP99Ms > 0 {
		out["latency_p99_ms"] = res.Summary.LatencyP99Ms
	}
	if res.Summary.Errors > 0 {
		out["errors"] = float64(res.Summary.Errors)
	}
	for k, v := range res.Metrics {
		if _, ok := out[k]; !ok {
			out[k] = v.Value
		}
	}
	return out
}
