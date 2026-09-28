package suite

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestSuiteInlineReportBudgetPreservesChildAndReferences(t *testing.T) {
	raw := json.RawMessage(`{"schema":1,"future":18446744073709551615}`)
	child := spec.Result{Segments: []spec.SegmentResult{{Name: "measure", Report: raw, ReportArtifact: "artifact/native-report"}}}
	budget := len(raw)
	first, second := child, child
	limitSuiteReports(&first, &budget)
	require.Equal(t, raw, first.Segments[0].Report)
	require.Zero(t, budget)
	limitSuiteReports(&second, &budget)
	require.Empty(t, second.Segments[0].Report)
	require.Contains(t, second.Segments[0].ReportOmitted, "suite inline report budget exceeded")
	require.Equal(t, "artifact/native-report", second.Segments[0].ReportArtifact)
	require.Equal(t, raw, child.Segments[0].Report, "child result must not be modified")
	require.Equal(t, raw, first.Segments[0].Report)
}
