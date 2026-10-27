package stroppycfg

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// RunReport decodes only the fields used by pipeline policy. The original JSON
// is retained separately, including unknown fields and exact integer literals.
// Contract: Stroppy pkg/report and docs/run-reports.md, envelope schemas 1
// (v6.1) to 4 (v6.2): schemas 2–4 only add fields (named drivers,
// executions, measurements, capture); the ones read here are unchanged.
type RunReport struct {
	Schema             int                     `json:"schema"`
	Kind               string                  `json:"kind"`
	ID                 string                  `json:"id"`
	RunID              string                  `json:"run_id"`
	Workload           string                  `json:"workload"`
	Status             string                  `json:"status"`
	MeasurementSeconds *float64                `json:"measurement_seconds"`
	Metrics            map[string]ReportMetric `json:"metrics"`
	Errors             *spec.ErrorCounts       `json:"errors"`
	Failure            *struct {
		Phase  string `json:"phase"`
		Reason string `json:"reason"`
	} `json:"failure"`
	WorkloadReports []struct {
		Kind   string          `json:"kind"`
		Schema int             `json:"schema"`
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	} `json:"workload_reports"`
}

// ReportMetric preserves native aggregate values; no throughput is calculated.
type ReportMetric struct {
	Type        string             `json:"type"`
	Unit        string             `json:"unit"`
	Total       *float64           `json:"total"`
	Count       *uint64            `json:"count"`
	Average     *float64           `json:"average"`
	Percentiles map[string]float64 `json:"percentiles"`
	// Bounds are a histogram's bucket upper bounds; BucketCounts has one
	// more entry, the bucket above the last bound.
	Bounds       []float64 `json:"bounds"`
	BucketCounts []uint64  `json:"bucket_counts"`
}

// percentile is a histogram's quantile q (0..1) interpolated linearly inside
// the bucket that holds it, as Prometheus histogram_quantile does. Stroppy's
// own percentiles are the bucket's upper bound, so a run whose iterations
// all take 100–250 ms reports p50 = p99 = 250. False without buckets.
//
// doc: https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantile
func (m ReportMetric) percentile(q float64) (float64, bool) {
	if len(m.Bounds) == 0 || len(m.BucketCounts) != len(m.Bounds)+1 {
		return 0, false
	}
	var total uint64
	for _, n := range m.BucketCounts {
		total += n
	}
	if total == 0 {
		return 0, false
	}
	rank := q * float64(total)
	var seen uint64
	for i, n := range m.BucketCounts {
		if n == 0 || float64(seen+n) < rank {
			seen += n
			continue
		}
		if i == len(m.Bounds) {
			// Above the last bound there is no upper edge to interpolate to.
			return m.Bounds[len(m.Bounds)-1], true
		}
		lower := 0.0
		if i > 0 {
			lower = m.Bounds[i-1]
		}
		return lower + (m.Bounds[i]-lower)*(rank-float64(seen))/float64(n), true
	}
	return m.Bounds[len(m.Bounds)-1], true
}

// ParseReport accepts additive fields but fails closed on incompatible envelopes.
func ParseReport(raw []byte) (RunReport, Summary, error) {
	var doc RunReport
	summary := Summary{Metrics: map[string]spec.MetricValue{}}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return doc, summary, fmt.Errorf("decode Stroppy report: %w", err)
	}
	if doc.Schema < 1 || doc.Schema > 4 || doc.Kind != "run" {
		return doc, summary, fmt.Errorf("unsupported Stroppy report kind %q schema %d", doc.Kind, doc.Schema)
	}
	if doc.ID == "" || doc.Workload == "" || doc.Metrics == nil || doc.Errors == nil || doc.MeasurementSeconds == nil {
		return doc, summary, fmt.Errorf("incomplete Stroppy run report")
	}
	switch doc.Status {
	case "completed", "completed_with_errors", "failed", "canceled":
	default:
		return doc, summary, fmt.Errorf("unknown Stroppy report status %q", doc.Status)
	}
	if *doc.MeasurementSeconds < 0 || doc.Errors.TerminalErrors < 0 || doc.Errors.FailedIterations < 0 || doc.Errors.FailedQueries < 0 || doc.Errors.RetryAttempts < 0 {
		return doc, summary, fmt.Errorf("negative Stroppy report measurement or error count")
	}
	for name, metric := range doc.Metrics {
		unit := metric.Unit
		if metric.Type == "histogram" {
			if metric.Count != nil {
				summary.Metrics[name+"_count"] = spec.MetricValue{Value: float64(*metric.Count)}
			}
			if metric.Average != nil {
				summary.Metrics[name+"_avg"] = spec.MetricValue{Value: *metric.Average, Unit: unit}
			}
			for _, p := range []struct {
				key string
				q   float64
			}{{"p50", 0.50}, {"p90", 0.90}, {"p95", 0.95}, {"p99", 0.99}} {
				value, ok := metric.percentile(p.q)
				if !ok {
					value, ok = metric.Percentiles[p.key]
				}
				if ok {
					summary.Metrics[name+"_"+p.key] = spec.MetricValue{Value: value, Unit: unit}
				}
			}
		} else if metric.Total != nil {
			if canonical := nativeMetricUnit(name); canonical != "" {
				unit = canonical
			}
			summary.Metrics[name] = spec.MetricValue{Value: *metric.Total, Unit: unit}
		}
	}
	// This is Stroppy's measured window, not activity/container wall time.
	summary.Metrics["measurement_seconds"] = spec.MetricValue{Value: *doc.MeasurementSeconds, Unit: "s"}
	if len(summary.Metrics) > 256 {
		return doc, summary, fmt.Errorf("stroppy report exceeds 256 projected metrics; full report retained as artifact")
	}
	summary.Found, summary.Errors = true, doc.Errors
	for _, payload := range doc.WorkloadReports {
		if payload.Kind == "tpcc.compliance" && payload.Schema == 1 && payload.Status == "ok" {
			summary.Compliance = payload.Data
		}
	}
	return doc, summary, nil
}

func nativeMetricUnit(name string) string {
	switch name {
	case "tps":
		return "transactions/s"
	case "iterations_per_second":
		return "iterations/s"
	case "queries_per_second":
		return "queries/s"
	case "measurement_seconds":
		return "s"
	default:
		return ""
	}
}

// HasRunReport checks the actual CLI, including custom/dev images without semver.
func HasRunReport(help []byte) bool {
	for _, line := range strings.Split(string(help), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "--report-file" {
			return true
		}
	}
	return false
}
