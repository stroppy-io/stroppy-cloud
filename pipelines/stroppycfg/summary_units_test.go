package stroppycfg

import "testing"

func TestHistogramUnits(t *testing.T) {
	for name, unit := range map[string]string{"tx_queries_per_tx": "queries/transaction", "tx_total_duration": "ms", "custom_distribution": ""} {
		s := ParseOutput([]byte("=== bench summary ===\n  " + name + "  count=2 avg=5.000 p(50)~=5.000 p(90)~=5.000 p(95)~=5.000 p(99)~=5.000\n"))
		if got := s.Metrics[name+"_avg"]; got.Value != 5 || got.Unit != unit {
			t.Fatalf("%s: %+v", name, got)
		}
		if s.Metrics[name+"_count"].Unit != "" {
			t.Fatal("histogram sample count has a duration unit")
		}
	}
}
