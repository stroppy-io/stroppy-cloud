package stroppycfg

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

// Headers of the blocks stroppy prints to stderr at the end of a run.
//
// doc: stroppy pkg/bench/runtime.go summary.printTo,
// pkg/bench/error_reporter.go writeSummary.
const (
	summaryHeader = "=== bench summary ==="
	errorsHeader  = "=== bench completed with errors ==="
	noMetricsLine = "bench: no metrics recorded"
)

// Metric statistics of a histogram line, in the order stroppy prints them.
var (
	// "  name  123.000" — a counter or gauge total.
	scalarLine = regexp.MustCompile(`^\s{2}(\S+)\s+(-?\d+(?:\.\d+)?)\s*$`)
	// "  name  count=N avg=X p(50)~=X p(90)~=X p(95)~=X p(99)~=X"
	histogramLine = regexp.MustCompile(`^\s{2}(\S+)\s+count=(\d+) avg=(\S+) p\(50\)~=(\S+) p\(90\)~=(\S+) p\(95\)~=(\S+) p\(99\)~=(\S+)\s*$`)
	// "  terminal_errors_total  123" inside the errors block.
	errorLine = regexp.MustCompile(`^\s{2}(terminal_errors_total|failed_iterations_total|failed_queries_total|retry_attempts_total)\s+(\d+)\s*$`)
	// {"compliance": …} — one JSON line on stdout (workloads/tpcc/report.go).
	compliancePrefix = []byte(`{"compliance":`)
)

// Summary is what one stroppy process reported.
type Summary struct {
	// Metrics are the bench summary values by key: counters and gauges under
	// their name (iterations_total), histograms as <name>_count, _avg, _p50,
	// _p90, _p95, _p99 (milliseconds for duration histograms).
	Metrics map[string]spec.MetricValue
	// Errors is the "completed with errors" block; nil when the run was clean.
	Errors *spec.ErrorCounts
	// Compliance is the TPC-C report, when the workload printed one.
	Compliance json.RawMessage
	// Found tells whether a summary block was seen at all.
	Found bool
}

// ParseOutput reads the merged stdout/stderr of a stroppy run.
func ParseOutput(out []byte) Summary {
	s := Summary{Metrics: map[string]spec.MetricValue{}}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	section := ""
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == summaryHeader:
			section, s.Found = "summary", true
			continue
		case trimmed == errorsHeader:
			section = "errors"
			if s.Errors == nil {
				s.Errors = &spec.ErrorCounts{}
			}
			continue
		case trimmed == noMetricsLine:
			s.Found = true
			continue
		case bytes.HasPrefix([]byte(trimmed), compliancePrefix):
			var doc struct {
				Compliance json.RawMessage `json:"compliance"`
			}
			if err := json.Unmarshal([]byte(trimmed), &doc); err == nil && len(doc.Compliance) > 0 {
				s.Compliance = doc.Compliance
			}
			continue
		case trimmed == "":
			continue
		}
		switch section {
		case "summary":
			if !parseSummaryLine(line, s.Metrics) {
				// The summary block ends at the first line that is not a
				// metric (a log line, the errors header handled above).
				if !strings.HasPrefix(line, "  ") {
					section = ""
				}
			}
		case "errors":
			if m := errorLine.FindStringSubmatch(line); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil {
					continue
				}
				switch m[1] {
				case "terminal_errors_total":
					s.Errors.TerminalErrors = n
				case "failed_iterations_total":
					s.Errors.FailedIterations = n
				case "failed_queries_total":
					s.Errors.FailedQueries = n
				case "retry_attempts_total":
					s.Errors.RetryAttempts = n
				}
			} else if !strings.HasPrefix(line, "  ") {
				section = ""
			}
		}
	}
	return s
}

func parseSummaryLine(line string, into map[string]spec.MetricValue) bool {
	if m := histogramLine.FindStringSubmatch(line); m != nil {
		name := m[1]
		unit := ""
		if strings.HasSuffix(name, "_duration") {
			unit = "ms"
		} else if name == "tx_queries_per_tx" {
			unit = "queries/transaction"
		}
		if count, err := strconv.ParseFloat(m[2], 64); err == nil {
			into[name+"_count"] = spec.MetricValue{Value: count}
		}
		for i, stat := range []string{"avg", "p50", "p90", "p95", "p99"} {
			v, err := strconv.ParseFloat(m[3+i], 64)
			if err != nil {
				continue
			}
			into[name+"_"+stat] = spec.MetricValue{Value: v, Unit: unit}
		}
		return true
	}
	if m := scalarLine.FindStringSubmatch(line); m != nil {
		v, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return false
		}
		unit := ""
		switch m[1] {
		case "tps":
			unit = "transactions/s"
		case "iterations_per_second":
			unit = "iterations/s"
		case "queries_per_second":
			unit = "queries/s"
		case "measurement_seconds":
			unit = "s"
		}
		into[m[1]] = spec.MetricValue{Value: v, Unit: unit}
		return true
	}
	return false
}

// ThresholdViolation names the bound a segment broke; empty when none.
// A zero threshold is "not set" — the schema requires positive bounds.
func ThresholdViolation(t spec.Thresholds, s Summary) string {
	if t.P99Ms > 0 {
		if p99, ok := s.Metrics["iteration_duration_p99"]; ok && p99.Value > t.P99Ms {
			return fmt.Sprintf("iteration_duration p99 %.3f ms > %.3f ms", p99.Value, t.P99Ms)
		}
	}
	if t.ErrorRate > 0 {
		if rate := ErrorRate(s); rate > t.ErrorRate {
			return fmt.Sprintf("error rate %.4f > %.4f", rate, t.ErrorRate)
		}
	}
	return ""
}

// ErrorRate is failed iterations (or queries, for query-set workloads) over
// iterations; 0 when nothing ran.
func ErrorRate(s Summary) float64 {
	iterations := s.Metrics["iterations_total"].Value
	if iterations <= 0 {
		return 0
	}
	failed := s.Metrics["failed_iterations_total"].Value
	if failed == 0 {
		failed = s.Metrics["failed_queries_total"].Value
	}
	return failed / iterations
}

// Headline copies the last measuring segment's reported metrics. TPS must
// be supplied by Stroppy itself: activity time includes container preparation
// and is not a measurement window. TPC-C tpmC stays in its compliance report;
// it is not interchangeable with overall transaction throughput.
func Headline(segments []spec.SegmentResult) spec.Summary {
	var out spec.Summary
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		m := seg.Metrics
		iterations, ok := m["iterations_total"]
		if !ok || iterations.Value == 0 {
			continue
		}
		out.TPS = m["tps"].Value
		if seg.FinishedAt.After(seg.StartedAt) {
			out.Duration = spec.Duration(seg.FinishedAt.Sub(seg.StartedAt))
		}
		out.LatencyP50Ms = m["iteration_duration_p50"].Value
		out.LatencyP95Ms = m["iteration_duration_p95"].Value
		out.LatencyP99Ms = m["iteration_duration_p99"].Value
		if seg.Errors != nil {
			out.Errors = seg.Errors.FailedIterations + seg.Errors.FailedQueries
		}
		break
	}
	return out
}

// MergeMetrics folds segment metrics into run-level metrics, prefixed by
// the segment name so two segments never collide.
func MergeMetrics(segments []spec.SegmentResult) map[string]spec.MetricValue {
	out := map[string]spec.MetricValue{}
	for _, seg := range segments {
		keys := make([]string, 0, len(seg.Metrics))
		for k := range seg.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out[seg.Name+"."+k] = seg.Metrics[k]
		}
	}
	return out
}

// ParseBaseline reads the JSON report of `stroppy baseline --json`.
//
// doc: cmd/stroppy/commands/baseline — schema 1: {schema, stroppy_version,
// time, host, tiers[], verdicts[{check,status,detail}]}.
func ParseBaseline(stdout []byte) (spec.BaselineResult, error) {
	start := bytes.IndexByte(stdout, '{')
	end := bytes.LastIndexByte(stdout, '}')
	if start < 0 || end < start {
		return spec.BaselineResult{}, fmt.Errorf("baseline: no JSON report in output")
	}
	raw := bytes.TrimSpace(stdout[start : end+1])
	var doc struct {
		Schema   int                    `json:"schema"`
		Verdicts []spec.BaselineVerdict `json:"verdicts"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return spec.BaselineResult{}, fmt.Errorf("baseline: %w", err)
	}
	res := spec.BaselineResult{OK: true, Verdicts: doc.Verdicts, Report: json.RawMessage(raw)}
	for _, v := range doc.Verdicts {
		if v.Status == "fail" {
			res.OK = false
		}
	}
	return res, nil
}

// ExitStatus explains a stroppy exit code.
//
// doc: `stroppy run --help` Signals.
func ExitStatus(code int) (canceled bool, text string) {
	switch code {
	case 0:
		return false, "ok"
	case 130:
		return true, "canceled by SIGINT"
	case 143:
		return true, "canceled by SIGTERM"
	case 2:
		return true, "forced exit after a second signal"
	default:
		return false, fmt.Sprintf("exit status %d", code)
	}
}
