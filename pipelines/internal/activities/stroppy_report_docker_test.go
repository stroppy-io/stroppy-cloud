package activities

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/graphene-ci/pipeline/pkg/machine"
	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"go.temporal.io/sdk/testsuite"
)

// Explicitly opt in with a pinned image digest; uses a local Docker daemon only.
func TestReportDockerActivity(t *testing.T) {
	image := os.Getenv("STROPPY_REPORT_TEST_IMAGE")
	if image == "" {
		t.Skip("set STROPPY_REPORT_TEST_IMAGE to a Stroppy v6.1+ image digest")
	}
	t.Setenv(machine.EnvWorkspace, t.TempDir())
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, name := range []string{"stroppy-00-report-docker", "stroppy-00-report-docker-report-help"} {
			require.NoError(t, cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}))
		}
		require.NoError(t, cli.Close())
	})
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.SetTestTimeout(2 * time.Minute)
	env.RegisterActivity(RunSegment)
	value, err := env.ExecuteActivity(RunSegment, RunSegmentRequest{
		RunID: "report-docker-acceptance", URL: "noop://localhost",
		Workload: spec.Workload{DriverType: "noop", StroppyImage: image},
		Segment:  spec.Segment{Name: "report-docker", LogLevel: "error", Workload: spec.WorkloadParams{Script: "execute_sql", Params: map[string]any{"sql_body": "SELECT 1"}}, Run: spec.RunParams{Executor: spec.ExecutorSharedIterations, VUs: 1, Iterations: 3}},
	})
	require.NoError(t, err)
	var result RunSegmentResult
	require.NoError(t, value.Get(&result))
	require.Equal(t, spec.SegmentCompleted, result.Result.Status, result.Result.Error)
	require.Equal(t, 3.0, result.Result.Metrics["iterations_total"].Value)
	require.NotEmpty(t, result.ReportPath)
	raw, err := os.ReadFile(result.ReportPath)
	require.NoError(t, err)
	require.JSONEq(t, string(raw), string(result.Result.Report))
	var report struct {
		StroppyVersion string `json:"stroppy_version"`
	}
	require.NoError(t, json.Unmarshal(raw, &report))
	require.Equal(t, "v6.1.0", report.StroppyVersion)
}
