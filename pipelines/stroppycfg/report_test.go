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
	require.Equal(t, doc.Metrics["iteration_duration"].Percentiles["p99"], summary.Metrics["iteration_duration_p99"].Value)
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
