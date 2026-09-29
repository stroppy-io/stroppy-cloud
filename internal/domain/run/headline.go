package run

import (
	"encoding/json"
	"strings"
	"time"
)

/*
HEADLINE: the run-level numbers the lists show, derived from what the
result already carries. The pipeline copies every segment metric into the
headline as `<segment>.<metric>` (stroppycfg.MergeMetrics); the run-level
numbers are folded from those here, in one place, so the finish path and
the SQL backfill (migration 20260929120000000_run_headline_qps) agree.
*/

// Headline keys derived from the segment metrics.
const (
	HeadlineQPS    = "qps"
	HeadlineErrors = "errors"
)

// Segment metric suffixes the derivation reads.
const (
	segQPS         = "queries_per_second"
	segMeasurement = "measurement_seconds"
	segTerminal    = "terminal_errors_total"
	segFailedIter  = "failed_iterations_total"
	segFailedQuery = "failed_queries_total"
)

// CompleteHeadline adds the run-level qps and errors to a headline that
// carries per-segment metrics. It mutates and returns h.
//
// qps is the throughput of the whole workload: Σ(qps·measurement_seconds)
// / Σ(measurement_seconds) over the segments reporting both (a positive
// measurement window); when no segment reports a window, the plain mean
// of the segment qps values. A segment's zero qps is a measurement and
// counts.
//
// errors: in Stroppy every terminal error is classified exactly once, as
// either iteration- or query-scoped, so terminal_errors_total ==
// failed_iterations_total + failed_queries_total per segment
// (stroppy pkg/bench/error_reporter.go). Adding failed_queries_total to
// terminal_errors_total would count query failures twice; the run's
// errors are Σ terminal_errors_total (failed_iterations_total +
// failed_queries_total for a segment without it). The result summary's
// errors cover only the last measuring segment (stroppycfg.Headline), so
// the larger of the two is kept.
func CompleteHeadline(h map[string]float64) map[string]float64 {
	if h == nil {
		return nil
	}
	type seg struct {
		qps, window                     float64
		hasQPS                          bool
		terminal, failedIter, failedQry float64
		hasTerminal, hasFailed          bool
	}
	segs := map[string]*seg{}
	at := func(name string) *seg {
		s := segs[name]
		if s == nil {
			s = &seg{}
			segs[name] = s
		}
		return s
	}
	for k, v := range h {
		i := strings.LastIndexByte(k, '.')
		if i <= 0 {
			continue
		}
		name, metric := k[:i], k[i+1:]
		switch metric {
		case segQPS:
			s := at(name)
			s.qps, s.hasQPS = v, true
		case segMeasurement:
			at(name).window = v
		case segTerminal:
			s := at(name)
			s.terminal, s.hasTerminal = v, true
		case segFailedIter:
			s := at(name)
			s.failedIter, s.hasFailed = v, true
		case segFailedQuery:
			s := at(name)
			s.failedQry, s.hasFailed = v, true
		}
	}
	var weighted, window, sum float64
	var n int
	var failed float64
	var hasErrors bool
	for _, s := range segs {
		if s.hasQPS {
			n++
			sum += s.qps
			if s.window > 0 {
				weighted += s.qps * s.window
				window += s.window
			}
		}
		switch {
		case s.hasTerminal:
			failed, hasErrors = failed+s.terminal, true
		case s.hasFailed:
			failed, hasErrors = failed+s.failedIter+s.failedQry, true
		}
	}
	switch {
	case window > 0:
		h[HeadlineQPS] = weighted / window
	case n > 0:
		h[HeadlineQPS] = sum / float64(n)
	}
	if hasErrors && failed > h[HeadlineErrors] {
		h[HeadlineErrors] = failed
	}
	return h
}

// Point is one sample of a summary series: unix milliseconds and value.
type Point struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// qpsSeriesPoints is how many points the list sparkline wants.
const qpsSeriesPoints = 30

// minSeriesStep is the finest step the metric store is asked for.
const minSeriesStep = 5 * time.Second

// SeriesWindow is the span a run's throughput is drawn over: its
// measured segments, else the workload phase, else the run itself. An
// open end falls back to `end` (the finish time). The step gives about
// qpsSeriesPoints points, never finer than minSeriesStep. ok is false
// when the run never started.
func SeriesWindow(r Run, end time.Time) (from, to time.Time, step time.Duration, ok bool) {
	for _, s := range r.State.Segments {
		if s.StartedAt != nil && (from.IsZero() || s.StartedAt.Before(from)) {
			from = *s.StartedAt
		}
		if s.FinishedAt != nil && s.FinishedAt.After(to) {
			to = *s.FinishedAt
		}
	}
	if from.IsZero() {
		for _, p := range r.State.Phases {
			if p.ID == PhaseWorkload && p.StartedAt != nil {
				from = *p.StartedAt
				if p.FinishedAt != nil {
					to = *p.FinishedAt
				}
			}
		}
	}
	if from.IsZero() && r.StartedAt != nil {
		from = *r.StartedAt
		if r.FinishedAt != nil {
			to = *r.FinishedAt
		}
	}
	if from.IsZero() {
		return time.Time{}, time.Time{}, 0, false
	}
	if !to.After(from) {
		to = end
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, 0, false
	}
	step = (to.Sub(from) / qpsSeriesPoints).Round(time.Second)
	if step < minSeriesStep {
		step = minSeriesStep
	}
	return from, to, step, true
}

// segmentBudget is how long a snapshot segment is planned to run:
// warmup + run.duration. ok is false for a segment without a duration
// (iteration-bound executors): its end cannot be predicted.
func segmentBudget(raw json.RawMessage) (name string, d time.Duration, ok bool) {
	var seg struct {
		Name   string `json:"name"`
		Warmup string `json:"warmup"`
		Run    struct {
			Duration string `json:"duration"`
		} `json:"run"`
	}
	if json.Unmarshal(raw, &seg) != nil || seg.Run.Duration == "" {
		return seg.Name, 0, false
	}
	run, err := time.ParseDuration(seg.Run.Duration)
	if err != nil || run <= 0 {
		return seg.Name, 0, false
	}
	var warmup time.Duration
	if seg.Warmup != "" {
		if warmup, err = time.ParseDuration(seg.Warmup); err != nil || warmup < 0 {
			return seg.Name, 0, false
		}
	}
	return seg.Name, warmup + run, true
}

// ExpectedFinishAt is when the run's workload is planned to end: the
// workload phase start plus every segment's warmup + duration, refined
// by the segments already observed (a finished segment anchors the rest
// at its finish, a running one at its start). Only workload time counts —
// collecting and teardown have no predictable duration. Nil for a run not
// yet in its workload, a finished run, or a workload with a segment of
// unknown length.
func (r Run) ExpectedFinishAt() *time.Time {
	if r.Status.Terminal() {
		return nil
	}
	var anchor time.Time
	for _, p := range r.State.Phases {
		if p.ID == PhaseWorkload && p.StartedAt != nil {
			anchor = *p.StartedAt
		}
	}
	if anchor.IsZero() || len(r.Snapshot.Workload.Segments) == 0 {
		return nil
	}
	budgets := make([]time.Duration, len(r.Snapshot.Workload.Segments))
	index := map[string]int{}
	for i, raw := range r.Snapshot.Workload.Segments {
		name, d, ok := segmentBudget(raw)
		if !ok {
			return nil
		}
		budgets[i] = d
		if name != "" {
			index[name] = i
		}
	}
	next := 0
	for _, s := range r.State.Segments {
		i, ok := index[s.Name]
		if !ok {
			i = s.Index
		}
		if i < 0 || i >= len(budgets) {
			continue
		}
		switch {
		case s.FinishedAt != nil && i+1 >= next:
			anchor, next = *s.FinishedAt, i+1
		case s.StartedAt != nil && s.FinishedAt == nil && i >= next:
			anchor, next = *s.StartedAt, i
		}
	}
	at := anchor
	for _, d := range budgets[next:] {
		at = at.Add(d)
	}
	return &at
}

// ProgressPct is the run's progress at now: the phase weight
// (State.Progress), and inside the workload the elapsed share of the time
// planned up to ExpectedFinishAt. A finished run keeps its stored value.
func (r Run) ProgressPct(now time.Time) float64 {
	if r.Status.Terminal() {
		return r.Summary.ProgressPct
	}
	if r.Phase == PhaseWorkload {
		if f, ok := r.workloadShare(now); ok {
			return State{}.Progress(PhaseWorkload) + 50*f
		}
	}
	return r.State.Progress(r.Phase)
}

// PhaseProgressPct is how far the current phase is at now: the workload
// by the elapsed share of its planned time (by finished segments when the
// plan is unknown), any other phase by its finished steps. Zero for a
// phase with nothing to count yet; 100 once the run completed.
func (r Run) PhaseProgressPct(now time.Time) float64 {
	if r.Status == StatusCompleted {
		return 100
	}
	if r.Status.Terminal() {
		return 0
	}
	if r.Phase == PhaseWorkload {
		if f, ok := r.workloadShare(now); ok {
			return 100 * f
		}
		if n := len(r.State.Segments); n > 0 {
			done := 0
			for _, s := range r.State.Segments {
				if finished(s.Status) {
					done++
				}
			}
			return 100 * float64(done) / float64(n)
		}
		return 0
	}
	for _, p := range r.State.Phases {
		if p.ID != r.Phase || len(p.Steps) == 0 {
			continue
		}
		done := 0
		for _, st := range p.Steps {
			if finished(st.Status) {
				done++
			}
		}
		return 100 * float64(done) / float64(len(p.Steps))
	}
	return 0
}

func finished(status string) bool {
	return status == "completed" || status == "failed" || status == "skipped"
}

// workloadShare is the elapsed share [0, 1] of the workload's planned
// time at now; ok is false when the plan is unknown.
func (r Run) workloadShare(now time.Time) (float64, bool) {
	eta := r.ExpectedFinishAt()
	var start time.Time
	for _, p := range r.State.Phases {
		if p.ID == PhaseWorkload && p.StartedAt != nil {
			start = *p.StartedAt
		}
	}
	if eta == nil || start.IsZero() || !eta.After(start) {
		return 0, false
	}
	f := float64(now.Sub(start)) / float64(eta.Sub(start))
	return min(max(f, 0), 1), true
}
