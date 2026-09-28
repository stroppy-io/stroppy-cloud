package activities

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func TestReportOutcomeNeverFallsBackToText(t *testing.T) {
	base, err := os.ReadFile("../../stroppycfg/testdata/run-report.json")
	require.NoError(t, err)
	for _, tc := range []struct {
		name, replace, with string
		code                int
		missing             bool
		status              spec.SegmentStatus
		reason              string
	}{
		{name: "success", status: spec.SegmentCompleted},
		{name: "missing", missing: true, status: spec.SegmentFailed, reason: "read Stroppy report"},
		{name: "truncated", replace: string(base), with: `{"schema":`, status: spec.SegmentFailed, reason: "decode Stroppy report"},
		{name: "future-schema", replace: `"schema": 1`, with: `"schema": 2`, status: spec.SegmentFailed, reason: "unsupported Stroppy report"},
		{name: "failed-zero-exit", replace: `"status": "completed"`, with: `"status": "failed"`, status: spec.SegmentFailed, reason: "report status: failed"},
		{name: "completed-with-errors", replace: `"status": "completed"`, with: `"status": "completed_with_errors"`, status: spec.SegmentCompleted},
		{name: "canceled", replace: `"status": "completed"`, with: `"status": "canceled"`, code: 143, status: spec.SegmentCancelled, reason: "canceled"},
		{name: "hard-kill", missing: true, code: 137, status: spec.SegmentFailed, reason: "read Stroppy report"},
		{name: "bad-exit", code: 1, status: spec.SegmentFailed, reason: "exited 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "report.json")
			raw := base
			if tc.replace != "" {
				raw = []byte(strings.Replace(string(base), tc.replace, tc.with, 1))
			}
			if !tc.missing {
				require.NoError(t, os.WriteFile(name, raw, 0600))
			}
			out := stroppyOutput{ReportPath: name, ExitCode: tc.code, Log: []byte("=== bench summary ===\n  iterations_total  999\n")}
			var result spec.SegmentResult
			status, reason := segmentOutcome(context.Background(), &spec.Segment{}, &out, &result)
			require.Equal(t, tc.status, status, reason)
			if tc.reason != "" {
				require.Contains(t, reason, tc.reason)
			}
			require.NotEqual(t, 999.0, result.Metrics["iterations_total"].Value)
			if json.Valid(raw) && !tc.missing {
				require.JSONEq(t, string(raw), string(result.Report))
			}
		})
	}
}

func TestReportThresholdAndSize(t *testing.T) {
	raw, err := os.ReadFile("../../stroppycfg/testdata/run-report.json")
	require.NoError(t, err)
	file := filepath.Join(t.TempDir(), "report.json")
	// Unknown payloads and integers survive the native path without float conversion.
	raw = []byte(strings.Replace(string(raw), `"workload_reports": []`, `"workload_reports": [{"kind":"future","schema":2,"status":"ok","data":{"large":18446744073709551615}}]`, 1))
	require.NoError(t, os.WriteFile(file, raw, 0600))
	out := stroppyOutput{ReportPath: file}
	var result spec.SegmentResult
	status, reason := reportOutcome(&spec.Segment{Thresholds: spec.Thresholds{P99Ms: 0.001}}, &out, &result)
	require.Equal(t, spec.SegmentFailed, status)
	require.Contains(t, reason, "threshold:")
	require.Contains(t, string(result.Report), "18446744073709551615")
	large := strings.Replace(string(raw), `"kind": "run"`, `"padding":"`+strings.Repeat("x", inlineReportLimit)+`","kind":"run"`, 1)
	require.NoError(t, os.WriteFile(file, []byte(large), 0600))
	result = spec.SegmentResult{}
	status, reason = reportOutcome(&spec.Segment{}, &out, &result)
	require.Equal(t, spec.SegmentCompleted, status, reason)
	require.Empty(t, result.Report)
	require.NotEmpty(t, result.ReportOmitted)
	require.Equal(t, 3.0, result.Metrics["iterations_total"].Value)
	kept, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, large, string(kept), "artifact keeps all bytes")
}

func TestManagedReportArgsOwnDestination(t *testing.T) {
	args := []string{"run", "-f", "/workspace/private.json", "--report-file=old.json", "--no-report", "--report-format", "json", "--vus=2"}
	got := managedReportArgs(args, "/workspace/new.json")
	require.Equal(t, []string{"run", "-f", "/workspace/private.json", "--vus=2", "--report-file", "/workspace/new.json"}, got)
	require.Contains(t, args, "--no-report", "caller args remain unchanged")
}
