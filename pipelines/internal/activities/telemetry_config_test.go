package activities

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
	"os"
	"testing"
	"time"
)

func TestStroppyConfigUsesExecutorLocalOTLP(t *testing.T) {
	t.Setenv("GRAPHENE_OTLP_ENDPOINT", "127.0.0.1:43123")
	t.Setenv("GRAPHENE_OTLP_ENDPOINT_BRIDGE", "172.17.0.1:43123")
	req := RunSegmentRequest{RunID: "test", Workload: spec.Workload{DriverType: "postgres", URL: "postgres://localhost/test"}, Segment: spec.Segment{Name: "main", Workload: spec.WorkloadParams{Script: "simple"}, Run: spec.RunParams{VUs: 1, Duration: spec.Duration(time.Second)}}}
	path, err := writeSegmentInputsContext(t.Context(), t.TempDir(), req)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	global := doc["global"].(map[string]any)
	exporter := global["exporter"].(map[string]any)["otlpExport"].(map[string]any)
	require.Equal(t, "127.0.0.1:43123", exporter["otlpHttpEndpoint"])
	require.Equal(t, true, exporter["otlpEndpointInsecure"])
	require.NotContains(t, exporter, "otlpHeaders")
	require.NotContains(t, string(raw), "172.17.0.1")
}
