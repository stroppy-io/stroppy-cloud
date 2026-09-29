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

	"github.com/stroppy-io/stroppy-cloud/internal/domain/errs"
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
	telemetry Telemetry

	mu     sync.Mutex
	active map[uuid.UUID]struct{}
	wg     sync.WaitGroup
}

// NewProjector wires the worker.
func NewProjector(repo Repository, g Graphene, publisher Publisher, scope func(context.Context, string) context.Context, log *xlog.Logger) *Projector {
	return &Projector{repo: repo, graphene: g, publisher: publisher, scope: scope, log: log, active: map[uuid.UUID]struct{}{}}
}

// Telemetry reads a run's workload throughput from the metric store
// (observe.Service); nil disables the stored series.
type Telemetry interface {
	// QPSSeries samples queries per second over [from, to] at step. An
	// empty answer means the store has no such series.
	QPSSeries(ctx context.Context, r Run, from, to time.Time, step time.Duration) ([]Point, error)
}

// WithTelemetry connects the metric store the finish path samples the
// throughput series from.
func (p *Projector) WithTelemetry(t Telemetry) *Projector {
	p.telemetry = t
	return p
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
	// Periodically reload durable commands (notably cancellation), even if
	// the event stream remains open and silent.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	r, err := p.repo.ByID(ctx, l.ID)
	if err != nil {
		p.log.Warn("projector: read run", xlog.String("run", l.ID.String()), xlog.Error("error", err))
		return
	}
	if r.Status.Terminal() {
		p.reconcileKeep(ctx, r)
		return
	}
	sctx := p.scope(ctx, r.GrapheneNamespace)
	if terminal := r.State.Close; terminal != nil {
		p.finish(ctx, sctx, r, terminal.Status, terminal.Reason, terminal.At)
		return
	}
	if r.SuiteRunID == nil && (r.Status == StatusPending || (r.Status == StatusCancelling && r.StartedAt == nil)) {
		if err := Submit(sctx, p.graphene, r.CreatedAt, r.GrapheneID(), "stroppy-run", r.RunSpec, runLabels(r)); err != nil {
			if errors.Is(err, ErrStartRejected) {
				_ = p.repo.SetStatus(ctx, r.ID, StatusFailed, PhaseDone, "start: "+err.Error(), nil, ptr(time.Now().UTC())) //nolint:errcheck // retried on the next tick
			}
			p.log.Warn("projector: submission", xlog.String("run", r.ID.String()), xlog.Error("error", err))
			return
		}
	}
	if r.Status == StatusCancelling {
		if err := p.graphene.CancelRun(sctx, r.GrapheneID()); err != nil {
			// The workflow may already have closed while cancellation was
			// in flight. Still observe its outcome; retry the command next tick.
			p.log.Debug("projector: cancel delivery", xlog.String("run", r.ID.String()), xlog.Error("error", err))
		}
	}
	state, status, lastID := r.State, r.Status, r.LastEventID
	finished := false
	err = p.graphene.Events(sctx, r.GrapheneID(), lastID, true, func(e RawEvent) error {
		if e.ID <= lastID {
			return nil
		}
		lastID = e.ID
		out := state.Apply(status, e)
		if out.Finished {
			state.Close = &CloseState{Status: out.Status, Reason: out.Reason, At: e.At}
		}
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
		if !out.Finished && out.Status != "" && (out.Status != status || out.Phase != "") {
			var startedAt *time.Time
			if out.Status == StatusRunning && status == StatusPending {
				startedAt = ptr(e.At)
				if r.StartedAt == nil {
					r.StartedAt = startedAt
				}
			}
			phase := out.Phase
			if phase == "" {
				phase = r.Phase
			}
			// A cancel in flight keeps its status until the run ends.
			if status == StatusCancelling {
				out.Status = StatusCancelling
			}
			if err := p.repo.SetStatus(ctx, r.ID, out.Status, phase, out.Reason, startedAt, nil); err != nil {
				return err
			}
			status = out.Status
		}
		if out.Finished {
			finished = true
			r.State = state
			p.finish(ctx, sctx, r, out.Status, out.Reason, e.At)
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
		r.State = state
		p.finish(ctx, sctx, r, final, reason, time.Now().UTC())
	}
}

// A release may finish after its HTTP caller disconnects, and expiry happens
// without a caller at all. Reconcile the projection from actual holdings;
// retained artifacts do not mean the infrastructure is still kept.
func (p *Projector) reconcileKeep(ctx context.Context, r Run) {
	if !r.StandKept {
		return
	}
	sctx := p.scope(ctx, r.GrapheneNamespace)
	held, err := p.graphene.Holdings(sctx, r.GrapheneRef())
	if err != nil {
		p.log.Warn("projector: kept holdings", xlog.String("run", r.ID.String()), xlog.Error("error", err))
		return
	}
	if len(standHoldings(held)) != 0 {
		return
	}
	if err := p.repo.SetKeep(ctx, r.ID, false, nil); err != nil {
		p.log.Warn("projector: clear released keep", xlog.String("run", r.ID.String()), xlog.Error("error", err))
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
func (p *Projector) finish(ctx, sctx context.Context, r Run, status Status, reason string, at time.Time) {
	// A run that did not complete still collected something: its partial
	// result rides in the failure. Keep the raw envelope so explicit zeroes
	// and opaque report extensions survive storage.
	raw, failure, err := p.graphene.RunClose(sctx, r.GrapheneID())
	switch {
	case err != nil:
		p.log.Warn("projector: result", xlog.String("run", r.ID.String()), xlog.Error("error", err))
		return
	case len(raw) == 0:
		// Nothing was collected (the run died before its first phase).
		p.log.Debug("projector: no result", xlog.String("run", r.ID.String()), xlog.String("failure", failure))
	default:
		var res spec.Result
		if err := json.Unmarshal(raw, &res); err != nil {
			p.log.Warn("projector: decode result", xlog.String("run", r.ID.String()), xlog.Error("error", err))
			return
		} else {
			summary := r.Summary
			if status == StatusCompleted {
				summary.ProgressPct = 100
			}
			summary.Headline = headlineOf(res)
			if series, ok := p.qpsSeries(ctx, r, at); ok {
				summary.QPSSeries = series
			}
			var tps *float64
			if status == StatusCompleted && res.Summary.TPS > 0 {
				v := res.Summary.TPS
				tps = &v
			}
			if err := p.repo.SetResult(ctx, r.ID, raw, summary, tps); err != nil {
				p.log.Warn("projector: store result", xlog.String("run", r.ID.String()), xlog.Error("error", err))
				return
			}
		}
	}
	// The stand is kept when the pipeline says so (stand.kept), not
	// because keep was asked: a failed run tears everything down.
	cur, err := p.repo.ByID(ctx, r.ID)
	if err != nil {
		return
	}
	if cur.State.Stand != nil && r.Keep > 0 {
		held, err := p.graphene.Holdings(sctx, r.GrapheneRef())
		if err != nil {
			return
		}
		infra := standHoldings(held)
		var until *time.Time
		if len(infra) > 0 {
			until = infra[0].KeepUntil
		}
		if err := p.repo.SetKeep(ctx, r.ID, len(infra) > 0, until); err != nil {
			return
		}
	}
	// Do this LAST: active rows are the durable finalization queue. If the
	// result RPC or any write failed, the next follower resumes this work.
	if err := p.repo.SetStatus(ctx, r.ID, status, PhaseDone, reason, nil, ptr(at)); err != nil {
		return
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
	cur, err = p.repo.ByID(ctx, r.ID)
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
	return CompleteHeadline(out)
}

// qpsSeries samples the run's throughput for the list sparkline. A store
// failure never fails finalization: ok is false and the startup backfill
// retries later.
func (p *Projector) qpsSeries(ctx context.Context, r Run, end time.Time) ([]Point, bool) {
	if p.telemetry == nil {
		return nil, false
	}
	from, to, step, ok := SeriesWindow(r, end)
	if !ok {
		return nil, false
	}
	// Bounded apart from the follower's budget: a slow store must not
	// starve the writes that finalize the run.
	ctx, cancel := context.WithTimeout(ctx, qpsSeriesTimeout)
	defer cancel()
	series, err := p.telemetry.QPSSeries(ctx, r, from, to, step)
	if err != nil {
		p.log.Warn("projector: qps series", xlog.String("run", r.ID.String()), xlog.Error("error", err))
		return nil, false
	}
	if series == nil {
		series = []Point{}
	}
	return series, true
}

const qpsSeriesTimeout = 10 * time.Second

// BackfillQPSSeries stores the throughput series of finished runs that
// have none — runs older than the series, or whose finish could not reach
// the metric store. One pass, at most `concurrency` store queries at a
// time. A run the store does not know (telemetry expired) or that never
// measured anything gets an empty series and is not asked again; a store
// outage leaves the run for the next pass.
func (p *Projector) BackfillQPSSeries(ctx context.Context, concurrency int) (stored, empty, skipped int) {
	if p.telemetry == nil {
		return 0, 0, 0
	}
	runs, err := p.repo.WithoutQPSSeries(ctx, backfillLimit)
	if err != nil {
		p.log.Warn("qps backfill: list", xlog.Error("error", err))
		return 0, 0, 0
	}
	if concurrency < 1 {
		concurrency = 1
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, r := range runs {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(r Run) {
			defer wg.Done()
			defer func() { <-sem }()
			series, ok := p.backfillOne(ctx, r)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case !ok:
				skipped++
			case len(series) == 0:
				empty++
			default:
				stored++
			}
		}(r)
	}
	wg.Wait()
	return stored, empty, skipped
}

const backfillLimit = 10000

func (p *Projector) backfillOne(ctx context.Context, r Run) ([]Point, bool) {
	end := time.Now().UTC()
	if r.FinishedAt != nil {
		end = *r.FinishedAt
	}
	series := []Point{}
	if from, to, step, ok := SeriesWindow(r, end); ok {
		qctx, cancel := context.WithTimeout(ctx, qpsSeriesTimeout)
		got, err := p.telemetry.QPSSeries(qctx, r, from, to, step)
		cancel()
		switch {
		case errs.CodeOf(err) == errs.CodeNotFound:
		case err != nil:
			p.log.Debug("qps backfill: query", xlog.String("run", r.ID.String()), xlog.Error("error", err))
			return nil, false
		case got != nil:
			series = got
		}
	}
	if err := p.repo.SetQPSSeries(ctx, r.ID, series); err != nil {
		p.log.Warn("qps backfill: store", xlog.String("run", r.ID.String()), xlog.Error("error", err))
		return nil, false
	}
	return series, true
}
