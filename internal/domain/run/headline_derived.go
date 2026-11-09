package run

import (
	"context"
	"encoding/json"
	"math"
	"regexp"

	pipelinespec "github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

/*
DERIVED HEADLINE: the numbers each scenario is ranked by, computed from the
segments of a result, never from logs.

  - tpmc: TPC-C New-Order transactions per minute from Stroppy's compliance
    report (the canonical TPC-C metric); the last segment that has one.
  - load_duration_seconds: wall time of the steps that build the database
    (schema, load, indexes, keys, procedures, statistics) over all segments.
  - query_geomean_ms, query_total_seconds: the single-stream pass of a query
    set (TPC-H, TPC-DS). Taken from the last segment that ran it with one
    worker, every query of the set present and none failed; a run with a
    failed, timed-out or skipped query has no value and does not rank.
    A query under 0.1 s counts as 0.1 s, as in the TPC-H power test, so
    sub-millisecond answers do not swing the geometric mean.
*/

// Headline keys derived from the segments.
const (
	HeadlineTPMC         = "tpmc"
	HeadlineLoadSeconds  = "load_duration_seconds"
	HeadlineQueryGeomean = "query_geomean_ms"
	HeadlineQueryTotal   = "query_total_seconds"
)

// HeadlineRev versions the headline derivation: a stored headline of an
// older revision is recomputed from the stored result (BackfillHeadline).
const HeadlineRev = 1

// isLoadStep tells the bench steps that build the database.
// doc: stroppy workloads/*: drop_schema and validate_* are not part of it.
func isLoadStep(name string) bool {
	switch name {
	case "create_schema", "create_schema_column", "set_unlogged", "load_data",
		"create_indexes", "create_foreign_keys", "create_procedures", "set_logged", "analyze":
		return true
	default:
		return false
	}
}

// querySet is a query-set workload: the name of its per-query average and
// the statement count of a complete pass.
type querySet struct {
	pattern    *regexp.Regexp
	statements int
}

// querySets are the query-set workloads Stroppy times per query.
// doc: stroppy workloads/tpch/tpch.go initMetrics (q1..q22); tpcds has 99
// queries in 103 statements (14, 23, 24 and 39 have two parts).
func querySets() []querySet {
	return []querySet{
		{regexp.MustCompile(`^tpch_q\d+_duration_avg$`), 22},
		{regexp.MustCompile(`^tpcds_query_\w+_duration_avg$`), 103},
	}
}

// queryFloorMs is the shortest time a query counts for.
const queryFloorMs = 100

// ResultHeadline picks the headline numbers of a result: the summary's,
// every merged metric, the run-level qps and errors, and the derived ones.
func ResultHeadline(res pipelinespec.Result) map[string]float64 {
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
	deriveHeadline(out, res.Segments)
	return CompleteHeadline(out)
}

// BackfillHeadline recomputes the headline of finished runs whose stored
// one an older derivation produced, from their stored result. One pass; a
// result that does not decode is left as it is and not asked again.
func (p *Projector) BackfillHeadline(ctx context.Context) (int, error) {
	runs, err := p.repo.StaleHeadline(ctx, HeadlineRev, backfillLimit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range runs {
		var res pipelinespec.Result
		headline := r.Summary.Headline
		if json.Unmarshal(r.Result, &res) == nil {
			headline = ResultHeadline(res)
		}
		if err := p.repo.SetHeadline(ctx, r.ID, headline, HeadlineRev); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func deriveHeadline(h map[string]float64, segs []pipelinespec.SegmentResult) {
	var load float64
	var loaded bool
	for _, seg := range segs {
		steps, workers := segmentFacts(seg)
		for _, s := range steps {
			if isLoadStep(s.Name) && s.Status == "completed" {
				load, loaded = load+s.Seconds, true
			}
		}
		if v, ok := tpmcOf(seg.Compliance); ok {
			h[HeadlineTPMC] = v
		}
		if geomean, total, ok := singleStream(seg, workers); ok {
			h[HeadlineQueryGeomean], h[HeadlineQueryTotal] = geomean, total
		}
	}
	if loaded {
		h[HeadlineLoadSeconds] = load
	}
}

// segmentFacts are the step timings and measured workers of a segment;
// results older than the typed fields carry them only in the inline report.
func segmentFacts(seg pipelinespec.SegmentResult) (steps []pipelinespec.StepTiming, workers int) {
	if len(seg.Steps) > 0 || seg.Workers > 0 || len(seg.Report) == 0 {
		return seg.Steps, seg.Workers
	}
	doc, _, err := stroppycfg.ParseReport(seg.Report)
	if err != nil {
		return nil, 0
	}
	return doc.Timings(), doc.Workers()
}

func tpmcOf(raw json.RawMessage) (float64, bool) {
	var doc struct {
		TPMC float64 `json:"tpm_c"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &doc) != nil || doc.TPMC <= 0 {
		return 0, false
	}
	return doc.TPMC, true
}

// singleStream is the geometric mean (ms) and the sum (s) of the per-query
// times of a complete, clean single-worker pass of a query set.
func singleStream(seg pipelinespec.SegmentResult, workers int) (geomean, total float64, ok bool) {
	if seg.Status != pipelinespec.SegmentCompleted || workers != 1 {
		return 0, 0, false
	}
	if e := seg.Errors; e != nil && (e.FailedQueries > 0 || e.FailedIterations > 0 || e.TerminalErrors > 0) {
		return 0, 0, false
	}
	for _, set := range querySets() {
		var logSum, sum float64
		n := 0
		for k, v := range seg.Metrics {
			if !set.pattern.MatchString(k) {
				continue
			}
			ms := math.Max(v.Value, queryFloorMs)
			logSum += math.Log(ms)
			sum += v.Value
			n++
		}
		if n == 0 {
			continue
		}
		if n != set.statements {
			return 0, 0, false
		}
		return math.Exp(logSum / float64(n)), sum / 1000, true
	}
	return 0, 0, false
}
