package run_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/library"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/run"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCompleteHeadline(t *testing.T) {
	cases := []struct {
		name       string
		in         map[string]float64
		qps        float64
		hasQPS     bool
		errors     float64
		hasErrors  bool
		unchanged  []string
		absentKeys []string
	}{
		{
			name:   "weighted by measurement window",
			in:     map[string]float64{"tpch.queries_per_second": 100, "tpch.measurement_seconds": 1, "tpcds.queries_per_second": 10, "tpcds.measurement_seconds": 9},
			qps:    (100*1 + 10*9) / 10.0,
			hasQPS: true,
		},
		{
			name:   "segments without a window are left out of the weighting",
			in:     map[string]float64{"a.queries_per_second": 50, "a.measurement_seconds": 10, "b.queries_per_second": 1000},
			qps:    50,
			hasQPS: true,
		},
		{
			name:   "no window at all: plain mean",
			in:     map[string]float64{"a.queries_per_second": 10, "b.queries_per_second": 30},
			qps:    20,
			hasQPS: true,
		},
		{
			name:   "a measured zero counts",
			in:     map[string]float64{"verify.queries_per_second": 0, "verify.measurement_seconds": 20, "verify.terminal_errors_total": 23748, "verify.failed_queries_total": 23748, "verify.failed_iterations_total": 0},
			qps:    0,
			hasQPS: true,
			// terminal = failed_iterations + failed_queries: not doubled.
			errors: 23748, hasErrors: true,
		},
		{
			name:   "errors summed over segments, larger than the last-segment summary",
			in:     map[string]float64{"errors": 5, "a.terminal_errors_total": 100, "b.terminal_errors_total": 5},
			errors: 105, hasErrors: true,
		},
		{
			name:   "segment without terminal_errors_total falls back to its failures",
			in:     map[string]float64{"a.failed_iterations_total": 2, "a.failed_queries_total": 3},
			errors: 5, hasErrors: true,
		},
		{
			name:       "no failures: errors stays absent",
			in:         map[string]float64{"a.terminal_errors_total": 0, "a.queries_per_second": 7},
			qps:        7,
			hasQPS:     true,
			absentKeys: []string{"errors"},
		},
		{
			name:      "reported errors larger than the derived are kept",
			in:        map[string]float64{"errors": 9, "a.terminal_errors_total": 4},
			errors:    9,
			hasErrors: true,
		},
		{
			name:       "no segment metrics: nothing derived",
			in:         map[string]float64{"tps": 12, "latency_p99_ms": 3},
			unchanged:  []string{"tps", "latency_p99_ms"},
			absentKeys: []string{"qps", "errors"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := map[string]float64{}
			for k, v := range tc.in {
				before[k] = v
			}
			h := run.CompleteHeadline(tc.in)
			if v, ok := h[run.HeadlineQPS]; ok != tc.hasQPS || (ok && !near(v, tc.qps)) {
				t.Fatalf("qps = %v (present %v), want %v (present %v)", v, ok, tc.qps, tc.hasQPS)
			}
			if tc.hasErrors {
				if v := h[run.HeadlineErrors]; !near(v, tc.errors) {
					t.Fatalf("errors = %v, want %v", v, tc.errors)
				}
			}
			for _, k := range tc.absentKeys {
				if _, ok := h[k]; ok {
					t.Fatalf("%s must be absent: %v", k, h)
				}
			}
			for _, k := range tc.unchanged {
				if h[k] != before[k] {
					t.Fatalf("%s changed: %v → %v", k, before[k], h[k])
				}
			}
		})
	}
	if run.CompleteHeadline(nil) != nil {
		t.Fatal("nil headline stays nil")
	}
}

func at(min int) *time.Time {
	v := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC).Add(time.Duration(min) * time.Minute)
	return &v
}

func segmentsSnapshot(t *testing.T, segs ...map[string]any) run.Snapshot {
	t.Helper()
	w := library.WorkloadSpec{}
	for _, s := range segs {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		w.Segments = append(w.Segments, raw)
	}
	return run.Snapshot{Workload: w}
}

func TestExpectedFinishAt(t *testing.T) {
	snap := segmentsSnapshot(t,
		map[string]any{"name": "load", "warmup": "1m", "run": map[string]any{"duration": "10m"}},
		map[string]any{"name": "main", "warmup": "0s", "run": map[string]any{"duration": "20m"}},
	)
	workload := run.PhaseState{ID: run.PhaseWorkload, Status: "running", StartedAt: at(0)}
	cases := []struct {
		name string
		r    run.Run
		want *time.Time
	}{
		{name: "before the workload", r: run.Run{Status: run.StatusRunning, Snapshot: snap, State: run.State{Phases: []run.PhaseState{{ID: run.PhaseDeploying, StartedAt: at(0)}}}}},
		{
			name: "workload started, no segment yet",
			r:    run.Run{Status: run.StatusRunning, Snapshot: snap, State: run.State{Phases: []run.PhaseState{workload}}},
			want: at(31),
		},
		{
			name: "second segment running re-anchors on its start",
			r: run.Run{Status: run.StatusRunning, Snapshot: snap, State: run.State{Phases: []run.PhaseState{workload}, Segments: []run.SegmentState{
				{Name: "load", Index: 0, StartedAt: at(2), FinishedAt: at(15)},
				{Name: "main", Index: 1, StartedAt: at(16)},
			}}},
			want: at(36),
		},
		{
			name: "between segments re-anchors on the last finish",
			r: run.Run{Status: run.StatusRunning, Snapshot: snap, State: run.State{Phases: []run.PhaseState{workload}, Segments: []run.SegmentState{
				{Name: "load", Index: 0, StartedAt: at(2), FinishedAt: at(15)},
				{Name: "main", Index: 1},
			}}},
			want: at(35),
		},
		{name: "finished run", r: run.Run{Status: run.StatusCompleted, Snapshot: snap, State: run.State{Phases: []run.PhaseState{workload}}}},
		{
			name: "a segment without a duration is unpredictable",
			r: run.Run{Status: run.StatusRunning, Snapshot: segmentsSnapshot(t, map[string]any{"name": "q", "run": map[string]any{"iterations": 10}}),
				State: run.State{Phases: []run.PhaseState{workload}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.r.ExpectedFinishAt()
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("want nil, got %v", *got)
			case tc.want != nil && (got == nil || !got.Equal(*tc.want)):
				t.Fatalf("got %v, want %v", got, *tc.want)
			}
		})
	}
}

func TestSeriesWindow(t *testing.T) {
	end := *at(60)
	// Segments bound the window; the step aims at ~30 points.
	r := run.Run{StartedAt: at(0), State: run.State{
		Phases:   []run.PhaseState{{ID: run.PhaseWorkload, StartedAt: at(5), FinishedAt: at(50)}},
		Segments: []run.SegmentState{{Name: "a", StartedAt: at(10), FinishedAt: at(20)}, {Name: "b", StartedAt: at(20), FinishedAt: at(40)}},
	}}
	from, to, step, ok := run.SeriesWindow(r, end)
	if !ok || !from.Equal(*at(10)) || !to.Equal(*at(40)) || step != time.Minute {
		t.Fatalf("segments: %v %v %v %v", from, to, step, ok)
	}
	// No segments: the workload phase; an open end is the finish time.
	r.State.Segments = nil
	r.State.Phases[0].FinishedAt = nil
	from, to, _, ok = run.SeriesWindow(r, end)
	if !ok || !from.Equal(*at(5)) || !to.Equal(end) {
		t.Fatalf("phase: %v %v %v", from, to, ok)
	}
	// A short window never asks for a step under 5s.
	short := run.Run{StartedAt: at(0), FinishedAt: ptrTime(at(0).Add(20 * time.Second))}
	_, _, step, ok = run.SeriesWindow(short, end)
	if !ok || step != 5*time.Second {
		t.Fatalf("short: %v %v", step, ok)
	}
	if _, _, _, ok := run.SeriesWindow(run.Run{}, end); ok {
		t.Fatal("a run that never started has no window")
	}
}

func ptrTime(v time.Time) *time.Time { return &v }
