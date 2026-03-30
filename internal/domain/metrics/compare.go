package metrics

import "math"

// Comparison holds two runs side by side with per-metric diffs.
type Comparison struct {
	RunA    string            `json:"run_a"`
	RunB    string            `json:"run_b"`
	Metrics []MetricDiff      `json:"metrics"`
	Summary ComparisonVerdict `json:"summary"`
}

// MetricDiff compares a single metric between two runs.
type MetricDiff struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Unit string `json:"unit"`

	AvgA float64 `json:"avg_a"`
	AvgB float64 `json:"avg_b"`

	MaxA float64 `json:"max_a"`
	MaxB float64 `json:"max_b"`

	// DiffPct is (B - A) / A * 100. Positive means B is higher.
	DiffAvgPct float64 `json:"diff_avg_pct"`
	DiffMaxPct float64 `json:"diff_max_pct"`

	// Verdict per metric: "better", "worse", "same".
	Verdict string `json:"verdict"`
}

// ComparisonVerdict is the overall comparison result.
type ComparisonVerdict struct {
	Better int `json:"better"` // count of metrics that improved
	Worse  int `json:"worse"`  // count of metrics that degraded
	Same   int `json:"same"`   // count of metrics within threshold
}

// Compare two RunMetrics and produce a diff.
// threshold is the percentage below which a difference is considered "same" (e.g. 5.0 = 5%).
func Compare(a, b *RunMetrics, threshold float64) *Comparison {
	indexA := indexByKey(a)
	indexB := indexByKey(b)

	// Union of all keys, preserving order from A then B.
	seen := map[string]bool{}
	var keys []string
	for _, m := range a.Metrics {
		if !seen[m.Key] {
			keys = append(keys, m.Key)
			seen[m.Key] = true
		}
	}
	for _, m := range b.Metrics {
		if !seen[m.Key] {
			keys = append(keys, m.Key)
			seen[m.Key] = true
		}
	}

	comp := &Comparison{
		RunA: a.RunID,
		RunB: b.RunID,
	}

	for _, key := range keys {
		ma := indexA[key]
		mb := indexB[key]

		diff := MetricDiff{
			Key:  key,
			Name: ma.Name,
			Unit: ma.Unit,
			AvgA: ma.Avg,
			AvgB: mb.Avg,
			MaxA: ma.Max,
			MaxB: mb.Max,
		}
		if diff.Name == "" {
			diff.Name = mb.Name
			diff.Unit = mb.Unit
		}

		diff.DiffAvgPct = pctDiff(ma.Avg, mb.Avg)
		diff.DiffMaxPct = pctDiff(ma.Max, mb.Max)
		diff.Verdict = verdict(key, diff.DiffAvgPct, threshold)

		comp.Metrics = append(comp.Metrics, diff)

		switch diff.Verdict {
		case "better":
			comp.Summary.Better++
		case "worse":
			comp.Summary.Worse++
		default:
			comp.Summary.Same++
		}
	}

	return comp
}

// higherIsBetter lists metric keys where an increase is positive.
var higherIsBetter = map[string]bool{
	"db_qps":      true,
	"stroppy_ops": true,
}

// lowerIsBetter lists metric keys where a decrease is positive.
var lowerIsBetter = map[string]bool{
	"db_latency_p99":      true,
	"db_repl_lag":         true,
	"cpu_usage":           true,
	"memory_usage":        true,
	"stroppy_latency_p99": true,
	"stroppy_errors":      true,
}

func verdict(key string, diffPct, threshold float64) string {
	if math.Abs(diffPct) < threshold {
		return "same"
	}
	increased := diffPct > 0
	if higherIsBetter[key] {
		if increased {
			return "better"
		}
		return "worse"
	}
	if lowerIsBetter[key] {
		if increased {
			return "worse"
		}
		return "better"
	}
	// Unknown metrics: just report direction.
	if increased {
		return "worse"
	}
	return "better"
}

func pctDiff(a, b float64) float64 {
	if a == 0 {
		if b == 0 {
			return 0
		}
		return 100
	}
	return (b - a) / a * 100
}

func indexByKey(rm *RunMetrics) map[string]MetricSummary {
	idx := make(map[string]MetricSummary, len(rm.Metrics))
	for _, m := range rm.Metrics {
		idx[m.Key] = m
	}
	return idx
}
