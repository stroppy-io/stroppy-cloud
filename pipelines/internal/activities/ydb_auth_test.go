package activities

import (
	"encoding/json"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestYDBRuntimeCredentialDoesNotEnterArtifact(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "stroppy-config.json")
	_, err := writeSegmentInputs(dir, RunSegmentRequest{
		RunID: "test-run", URL: "grpcs://test",
		Workload: spec.Workload{DriverType: "ydb", URL: "grpcs://test"},
		Segment:  spec.Segment{Name: "simple", Workload: spec.WorkloadParams{Script: "simple"}, Run: spec.RunParams{Executor: spec.ExecutorConstantVUs, VUs: 1, Duration: spec.Duration(time.Second)}},
	})
	require.NoError(t, err)
	original, err := os.ReadFile(config)
	require.NoError(t, err)
	input := []string{"run", "-f", "/workspace/stroppy-config.json"}
	args, cleanup, err := runtimeTokenConfig(config, "test-token", input)
	require.NoError(t, err)
	defer cleanup()
	require.Equal(t, "/workspace/stroppy-config.json", input[2])
	require.NotContains(t, args, "test-token")
	artifact, err := os.ReadFile(config)
	require.NoError(t, err)
	require.Equal(t, original, artifact)
	runtimeFile := filepath.Join(dir, filepath.Base(args[2]))
	stat, err := os.Stat(runtimeFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), stat.Mode().Perm())
	raw, err := os.ReadFile(runtimeFile)
	require.NoError(t, err)
	var decoded struct {
		Drivers map[string]struct{ AuthToken string }
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, "test-token", decoded.Drivers["0"].AuthToken)
	cleanup()
	_, err = os.Stat(runtimeFile)
	require.True(t, os.IsNotExist(err))
	_, _, err = runtimeTokenConfig(config, "test-token", []string{"run"})
	require.Error(t, err)
	files, err := filepath.Glob(filepath.Join(dir, ".stroppy-iam-*"))
	require.NoError(t, err)
	require.Empty(t, files)
}
