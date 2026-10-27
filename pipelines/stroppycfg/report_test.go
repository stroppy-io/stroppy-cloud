package stroppycfg

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeRunReport(t *testing.T) {
	raw, err := os.ReadFile("testdata/run-report.json")
	require.NoError(t, err)
	doc, summary, err := ParseReport(raw)
	require.NoError(t, err)
	require.Equal(t, "completed", doc.Status)
	require.Equal(t, 3.0, summary.Metrics["iterations_total"].Value)
	require.Equal(t, *doc.Metrics["queries_per_second"].Total, summary.Metrics["queries_per_second"].Value)
	require.Equal(t, "queries/s", summary.Metrics["queries_per_second"].Unit)
	require.Equal(t, "ms", summary.Metrics["iteration_duration_p99"].Unit)
	// Read from the buckets (three iterations, the slowest in 0.05–0.1 ms),
	// not from Stroppy's percentiles.
	require.InDelta(t, 0.0985, summary.Metrics["iteration_duration_p99"].Value, 1e-9)
	require.NotContains(t, summary.Metrics, "tps")
	require.Zero(t, summary.Errors.TerminalErrors)
	for _, patch := range []map[string]any{{"schema": 5}, {"schema": 0}, {"kind": "baseline"}, {"status": "other"}, {"metrics": nil}, {"errors": nil}, {"measurement_seconds": -1}} {
		var changed map[string]any
		require.NoError(t, json.Unmarshal(raw, &changed))
		for k, v := range patch {
			changed[k] = v
		}
		data, err := json.Marshal(changed)
		require.NoError(t, err)
		_, _, err = ParseReport(data)
		require.Error(t, err)
	}
	_, _, err = ParseReport(append(raw, []byte(`{}`)...))
	require.Error(t, err, "reject concatenated/partial reports")
}

func TestReportCapabilityUsesFlagNotMention(t *testing.T) {
	require.True(t, HasRunReport([]byte("  --report-file PATH  Write JSON report\n")))
	require.False(t, HasRunReport([]byte("unknown flag --report-file\n")))
	require.False(t, HasRunReport([]byte("Usage: stroppy run\n --help show help\n")))
}

func TestUnknownWorkloadReportRemainsSupported(t *testing.T) {
	raw, err := os.ReadFile("testdata/run-report.json")
	require.NoError(t, err)
	raw = []byte(strings.Replace(string(raw), `"workload_reports": []`, `"workload_reports": [{"kind":"future.test","schema":42,"status":"ok","data":{"n":18446744073709551615}}]`, 1))
	doc, _, err := ParseReport(raw)
	require.NoError(t, err)
	require.Equal(t, `{"n":18446744073709551615}`, string(doc.WorkloadReports[0].Data))
}

// Stroppy 6.2 writes envelope schema 4: the fields read here are unchanged.
func TestRunReportSchema4(t *testing.T) {
	raw, err := os.ReadFile("testdata/run-report-v4.json")
	require.NoError(t, err)
	doc, summary, err := ParseReport(raw)
	require.NoError(t, err)
	require.Equal(t, 4, doc.Schema)
	require.Equal(t, "completed_with_errors", doc.Status)
	require.Equal(t, "transactions/s", summary.Metrics["tps"].Unit)
	require.Positive(t, summary.Metrics["tps"].Value)
	require.Equal(t, "ms", summary.Metrics["iteration_duration_p99"].Unit)
	require.EqualValues(t, 46, summary.Errors.TerminalErrors)
	require.NotEmpty(t, summary.Compliance)
}

func TestPercentileInterpolatesInsideTheBucket(t *testing.T) {
	// The PostgreSQL 18 nightly: 99.6% of iterations in 100–250 ms, which
	// Stroppy reports as p50 = p99 = 250.
	m := ReportMetric{
		Bounds:       []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 20000, 30000, 60000},
		BucketCounts: []uint64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 14, 40, 91, 3253434, 12715, 1066, 36, 0, 0, 0, 0, 0, 0},
	}
	p50, _ := m.percentile(0.50)
	p99, ok := m.percentile(0.99)
	if !ok || p50 < 170 || p50 > 180 || p99 < 245 || p99 > 250 || p99 <= p50 {
		t.Fatalf("p50 %v p99 %v", p50, p99)
	}
	if _, ok := (ReportMetric{Percentiles: map[string]float64{"p50": 250}}).percentile(0.5); ok {
		t.Fatal("a histogram without buckets falls back to Stroppy's percentiles")
	}
	top := ReportMetric{Bounds: []float64{10, 20}, BucketCounts: []uint64{0, 0, 5}}
	if v, _ := top.percentile(0.5); v != 20 {
		t.Fatalf("above the last bound: %v", v)
	}
}
