package run

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/google/uuid"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// tpchPass is a single-stream TPC-H segment: query i takes i*100 ms.
func tpchPass(workers int, queries int) pipelinespec.SegmentResult {
	m := map[string]pipelinespec.MetricValue{"queries_per_second": {Value: 1}}
	for i := 1; i <= queries; i++ {
		m[fmt.Sprintf("tpch_q%d_duration_avg", i)] = pipelinespec.MetricValue{Value: float64(i * 100)}
		m[fmt.Sprintf("tpch_q%d_duration_p99", i)] = pipelinespec.MetricValue{Value: 1}
	}
	return pipelinespec.SegmentResult{Name: "power", Status: pipelinespec.SegmentCompleted, Metrics: m, Workers: workers, Errors: &pipelinespec.ErrorCounts{}}
}

func tpchGeomean() float64 {
	var s float64
	for i := 1; i <= 22; i++ {
		s += math.Log(float64(i * 100))
	}
	return math.Exp(s / 22)
}

func TestDerivedHeadline(t *testing.T) {
	t.Parallel()
	report, err := os.ReadFile("../../../pipelines/stroppycfg/testdata/run-report-v4.json")
	if err != nil {
		t.Fatal(err)
	}
	load := pipelinespec.SegmentResult{Name: "load", Status: pipelinespec.SegmentCompleted, Steps: []pipelinespec.StepTiming{
		{Name: "drop_schema", Status: "completed", Seconds: 5},
		{Name: "create_schema", Status: "completed", Seconds: 1},
		{Name: "load_data", Status: "completed", Seconds: 100},
		{Name: "create_indexes", Status: "completed", Seconds: 20},
		{Name: "analyze", Status: "completed", Seconds: 4},
		{Name: "validate_answers", Status: "completed", Seconds: 9},
		{Name: "workload", Status: "skipped", Seconds: 0},
	}}
	failed := tpchPass(1, 22)
	failed.Errors = &pipelinespec.ErrorCounts{FailedQueries: 1, TerminalErrors: 1}
	fast := tpchPass(1, 22)
	fast.Metrics["tpch_q1_duration_avg"] = pipelinespec.MetricValue{Value: 0.4}
	cases := []struct {
		name   string
		segs   []pipelinespec.SegmentResult
		want   map[string]float64
		absent []string
	}{
		{
			name: "load and single stream",
			segs: []pipelinespec.SegmentResult{load, tpchPass(1, 22)},
			want: map[string]float64{HeadlineLoadSeconds: 125, HeadlineQueryGeomean: tpchGeomean(), HeadlineQueryTotal: 25.3},
		},
		{
			name:   "a failed query leaves the pass unranked",
			segs:   []pipelinespec.SegmentResult{load, failed},
			want:   map[string]float64{HeadlineLoadSeconds: 125},
			absent: []string{HeadlineQueryGeomean, HeadlineQueryTotal},
		},
		{
			name:   "a skipped query leaves the pass unranked",
			segs:   []pipelinespec.SegmentResult{tpchPass(1, 21)},
			absent: []string{HeadlineQueryGeomean, HeadlineLoadSeconds},
		},
		{
			name:   "several workers are not a single stream",
			segs:   []pipelinespec.SegmentResult{tpchPass(4, 22)},
			absent: []string{HeadlineQueryGeomean},
		},
		{
			name: "a query under 0.1 s counts as 0.1 s",
			segs: []pipelinespec.SegmentResult{fast},
			want: map[string]float64{HeadlineQueryGeomean: tpchGeomean(), HeadlineQueryTotal: 25.3 - 0.1 + 0.0004},
		},
		{
			name: "an older result reads steps and tpmC from the inline report",
			segs: []pipelinespec.SegmentResult{{Name: "main", Status: pipelinespec.SegmentCompleted, Report: report, Compliance: json.RawMessage(`{"tpm_c":3885}`)}},
			want: map[string]float64{HeadlineTPMC: 3885, HeadlineLoadSeconds: 0.133996147 + 2.031048383 + 0.190474061 + 0.221310855 + 1.513762483},
		},
		{
			name:   "no compliance, no tpmC",
			segs:   []pipelinespec.SegmentResult{{Name: "main", Status: pipelinespec.SegmentCompleted, Compliance: json.RawMessage(`{"tpm_c":0}`)}},
			absent: []string{HeadlineTPMC, HeadlineLoadSeconds},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := ResultHeadline(pipelinespec.Result{Segments: tc.segs})
			for k, v := range tc.want {
				if got, ok := h[k]; !ok || math.Abs(got-v) > 1e-6 {
					t.Fatalf("%s = %v (present %v), want %v", k, got, ok, v)
				}
			}
			for _, k := range tc.absent {
				if _, ok := h[k]; ok {
					t.Fatalf("%s must be absent: %v", k, h[k])
				}
			}
		})
	}
}

func BenchmarkResultHeadline(b *testing.B) {
	res := pipelinespec.Result{Segments: []pipelinespec.SegmentResult{tpchPass(1, 22)}}
	for b.Loop() {
		ResultHeadline(res)
	}
}

type headlineRepo struct {
	Repository
	runs []Run
	set  map[uuid.UUID]map[string]float64
}

func (h *headlineRepo) StaleHeadline(context.Context, int, int) ([]Run, error) { return h.runs, nil }
func (h *headlineRepo) SetHeadline(_ context.Context, id uuid.UUID, headline map[string]float64, rev int) error {
	if rev != HeadlineRev {
		return fmt.Errorf("revision %d", rev)
	}
	h.set[id] = headline
	return nil
}

func TestBackfillHeadline(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(pipelinespec.Result{Segments: []pipelinespec.SegmentResult{tpchPass(1, 22)}})
	if err != nil {
		t.Fatal(err)
	}
	stored := Run{ID: uuid.New(), Result: raw}
	broken := Run{ID: uuid.New(), Result: json.RawMessage(`[]`), Summary: Summary{Headline: map[string]float64{"tps": 7}}}
	repo := &headlineRepo{runs: []Run{stored, broken}, set: map[uuid.UUID]map[string]float64{}}
	_, _, p := recoveryFixture()
	p.repo = repo
	n, err := p.BackfillHeadline(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if math.Abs(repo.set[stored.ID][HeadlineQueryGeomean]-tpchGeomean()) > 1e-9 {
		t.Fatalf("recomputed headline: %v", repo.set[stored.ID])
	}
	if repo.set[broken.ID]["tps"] != 7 {
		t.Fatal("an undecodable result keeps its headline and is marked done")
	}
}
