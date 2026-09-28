package activities

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"

	"github.com/stretchr/testify/require"
)

func TestYDBServiceAccountFileIsPrivateAndRemoved(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "stroppy-config.json")
	original := []byte(`{"drivers":{"0":{"driverType":"ydb"}}}`)
	require.NoError(t, os.WriteFile(config, original, 0600))
	key := `{"private_key":"test-secret","service_account_id":"sa","id":"key"}`
	input := []string{"run", "-f", "/workspace/stroppy-config.json"}
	args, cleanup, err := runtimeServiceAccountConfig(config, key, input)
	require.NoError(t, err)
	defer cleanup()
	require.Equal(t, "/workspace/stroppy-config.json", input[2])
	require.NotContains(t, args, key)
	runtimePath := filepath.Join(dir, filepath.Base(args[2]))
	raw, err := os.ReadFile(runtimePath)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "test-secret")
	require.NotContains(t, string(raw), "authToken")
	var cfg struct {
		Drivers map[string]struct{ ServiceAccountKeyFile string }
	}
	require.NoError(t, json.Unmarshal(raw, &cfg))
	keyPath := filepath.Join(dir, filepath.Base(cfg.Drivers["0"].ServiceAccountKeyFile))
	for _, filename := range []string{runtimePath, keyPath} {
		info, err := os.Stat(filename)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	stored, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	require.Equal(t, key, string(stored))
	artifact, err := os.ReadFile(config)
	require.NoError(t, err)
	require.Equal(t, original, artifact)
	cleanup()
	_, err = os.Stat(keyPath)
	require.True(t, os.IsNotExist(err))
	_, _, err = runtimeServiceAccountConfig(config, key, []string{"run"})
	require.Error(t, err)
	files, err := filepath.Glob(filepath.Join(dir, ".stroppy-*"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestLegacyIAMRequiresDeadlineWithinCredentialLifetime(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	require.Error(t, validateLegacyIAMDeadline(t.Context(), expires))
	short, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	require.NoError(t, validateLegacyIAMDeadline(short, expires))
	long, cancelLong := context.WithTimeout(t.Context(), 2*time.Hour)
	defer cancelLong()
	require.Error(t, validateLegacyIAMDeadline(long, expires))
	require.Error(t, validateLegacyIAMDeadline(short, time.Now().Add(-time.Minute)))
}

func TestYDBRuntimeCredentialDoesNotEnterArtifact(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "stroppy-config.json")
	_, err := writeSegmentInputsContext(t.Context(), dir, RunSegmentRequest{
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
	require.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
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
