package activities

import (
	"context"
	"strings"
	"testing"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func readyProbe() RunSegmentResult {
	return RunSegmentResult{Result: spec.SegmentResult{Status: spec.SegmentCompleted, Metrics: map[string]spec.MetricValue{
		"iterations_total": {Value: 1}, "terminal_errors_total": {Value: 0},
	}}}
}

func TestDatabaseDiscoveryPropagation(t *testing.T) {
	calls := 0
	got := waitDatabaseQueries(t.Context(), 0, func(context.Context, int) (RunSegmentResult, error) {
		calls++
		if calls == 1 {
			return RunSegmentResult{Result: spec.SegmentResult{Status: spec.SegmentFailed, Error: "transport/NotFound: Database not found"}}, nil
		}
		return readyProbe(), nil
	})
	if !got.Ready || got.QueryAttempts != 2 || got.Error != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestDatabaseProbeRejectsFailedIterationAndPermissions(t *testing.T) {
	for _, result := range []RunSegmentResult{
		{Result: spec.SegmentResult{Status: spec.SegmentCompleted, Metrics: map[string]spec.MetricValue{"iterations_total": {Value: 1}, "terminal_errors_total": {Value: 1}}}},
		{Result: spec.SegmentResult{Status: spec.SegmentCompleted}},
		{Result: spec.SegmentResult{Status: spec.SegmentFailed, Error: "transport/PermissionDenied"}},
	} {
		got := waitDatabaseQueries(t.Context(), 0, func(context.Context, int) (RunSegmentResult, error) { return result, nil })
		if got.Ready || got.QueryAttempts != 1 || got.Error == "" {
			t.Fatalf("got %+v", got)
		}
	}
}

func TestDatabaseProbeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	got := waitDatabaseQueries(ctx, 0, func(context.Context, int) (RunSegmentResult, error) {
		cancel()
		return RunSegmentResult{Result: spec.SegmentResult{Status: spec.SegmentFailed, Error: "transport/NotFound: Database not found"}}, nil
	})
	if got.Ready || got.QueryAttempts != 1 || !strings.Contains(got.Error, "context canceled") {
		t.Fatalf("got %+v", got)
	}
}

func TestDatabaseProbeUsesSameImageAndReadOnlyQuery(t *testing.T) {
	request := WaitDatabaseRequest{RunID: "test", Endpoint: "grpcs://db.example:2135/?database=/test", Workload: spec.Workload{StroppyImage: "image@sha256:pinned", DriverType: "ydb", YDBIAMCredentialsSecret: "yc-key"}, RegistrySecret: "registry-key"}
	got := databaseProbeRequest(request)
	if got.URL != request.Endpoint || got.Workload.StroppyImage != request.Workload.StroppyImage || got.Workload.YDBIAMCredentialsSecret != "yc-key" || got.RegistrySecret != "registry-key" {
		t.Fatalf("identity/image mismatch: %+v", got)
	}
	if got.Segment.Workload.Script != "execute_sql" || got.Segment.Workload.Params["sql_body"] != "--= readiness\nSELECT 1;" || got.Segment.Run.Iterations != 1 || got.Segment.Run.VUs != 1 || got.OTLPEndpoint != "" || got.Index >= 0 {
		t.Fatalf("unsafe or measured probe: %+v", got)
	}
}
