package metrics

import "testing"

func TestCompare(t *testing.T) {
	a := &RunMetrics{
		RunID: "run-a",
		Metrics: []MetricSummary{
			{Key: "db_qps", Name: "DB QPS", Unit: "ops/s", Avg: 1000, Min: 800, Max: 1200, Last: 1100},
			{Key: "db_latency_p99", Name: "Latency", Unit: "s", Avg: 0.05, Min: 0.01, Max: 0.1, Last: 0.04},
			{Key: "stroppy_ops", Name: "Ops", Unit: "ops/s", Avg: 500, Min: 400, Max: 600, Last: 550},
		},
	}
	b := &RunMetrics{
		RunID: "run-b",
		Metrics: []MetricSummary{
			{Key: "db_qps", Avg: 1100, Max: 1300},         // 10% better
			{Key: "db_latency_p99", Avg: 0.04, Max: 0.08}, // 20% better (lower is better)
			{Key: "stroppy_ops", Avg: 500, Max: 600},      // same (within 5%)
		},
	}

	comp := Compare(a, b, 5.0)

	// Check counts
	if comp.Summary.Better == 0 {
		t.Error("expected some better")
	}
	if comp.Summary.Same == 0 {
		t.Error("expected some same")
	}

	// Check specific verdicts
	for _, m := range comp.Metrics {
		switch m.Key {
		case "db_qps":
			if m.Verdict != "better" {
				t.Errorf("db_qps should be better, got %s", m.Verdict)
			}
		case "db_latency_p99":
			if m.Verdict != "better" {
				t.Errorf("latency should be better, got %s", m.Verdict)
			}
		case "stroppy_ops":
			if m.Verdict != "same" {
				t.Errorf("ops should be same, got %s", m.Verdict)
			}
		}
	}
}
