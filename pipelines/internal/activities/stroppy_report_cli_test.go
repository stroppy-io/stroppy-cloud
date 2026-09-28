package activities

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"github.com/stroppy-io/stroppy-cloud/pipelines/stroppycfg"
)

// Additional local acceptance against a real build; fixtures cover the contract
// in ordinary CI. No database, registry or cloud access is needed.
func TestReportCLICompatibility(t *testing.T) {
	binary := os.Getenv("STROPPY_REPORT_TEST_BINARY")
	if binary == "" {
		t.Skip("set STROPPY_REPORT_TEST_BINARY to a Stroppy v6.1+ executable")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	help, err := exec.CommandContext(ctx, binary, "run", "--help").CombinedOutput()
	require.NoError(t, err)
	require.True(t, stroppycfg.HasRunReport(help))
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "canceled"}[canceled], func(t *testing.T) {
			dir := t.TempDir()
			seg := spec.Segment{Name: "measure", Workload: spec.WorkloadParams{Script: "execute_sql", Params: map[string]any{"sql_body": "SELECT 1"}}, Run: spec.RunParams{Executor: spec.ExecutorSharedIterations, Iterations: 3, VUs: 1}, LogLevel: "error"}
			if canceled {
				seg.Run = spec.RunParams{Executor: spec.ExecutorConstantVUs, Duration: spec.Duration(time.Minute), VUs: 1}
			}
			_, err := writeSegmentInputsContext(ctx, dir, RunSegmentRequest{RunID: "report-acceptance", Segment: seg, Workload: spec.Workload{DriverType: "noop"}, URL: "noop://localhost"})
			require.NoError(t, err)
			file := filepath.Join(dir, "report.json")
			args := managedReportArgs(stroppycfg.Args(seg), file)
			for i, arg := range args {
				args[i] = strings.ReplaceAll(arg, stroppycfg.ContainerWorkspace, dir)
			}
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Dir = dir
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			require.NoError(t, cmd.Start())
			if canceled {
				time.Sleep(250 * time.Millisecond)
				require.NoError(t, cmd.Process.Signal(os.Interrupt))
			}
			err = cmd.Wait()
			if !canceled {
				require.NoError(t, err, output.String())
			}
			out := stroppyOutput{ReportPath: file, ExitCode: cmd.ProcessState.ExitCode()}
			var result spec.SegmentResult
			status, reason := reportOutcome(&seg, &out, &result)
			want := spec.SegmentCompleted
			if canceled {
				want = spec.SegmentCancelled
			}
			require.Equal(t, want, status, reason+"\n"+output.String())
			require.NotEmpty(t, result.Report)
			require.NotContains(t, result.Metrics, "tps")
			if !canceled {
				require.Equal(t, 3.0, result.Metrics["iterations_total"].Value)
			}
		})
	}
}
