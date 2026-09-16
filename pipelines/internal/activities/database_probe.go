package activities

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func databaseProbeRequest(req WaitDatabaseRequest) RunSegmentRequest {
	return RunSegmentRequest{
		RunID: req.RunID, Index: -1, Workload: req.Workload, URL: req.Endpoint,
		RegistrySecret: req.RegistrySecret,
		Segment: spec.Segment{
			Name: "managed-ydb-ready", LogLevel: "info",
			Workload: spec.WorkloadParams{Script: "execute_sql", Params: map[string]any{"sql_body": "--= readiness\nSELECT 1;"}},
			Run:      spec.RunParams{Executor: "shared-iterations", VUs: 1, Iterations: 1, QueryTimeout: spec.Duration(10 * time.Second)},
		},
		// This is a readiness check, not a measured workload segment. Native
		// OTLP stays disabled; Graphene persists the check's execution log.
	}
}

func probeManagedDatabase(ctx context.Context, req WaitDatabaseRequest) (WaitDatabaseResult, error) {
	// Image acquisition has its own progress heartbeats and shares the overall
	// readiness deadline; the per-attempt bound applies to the actual probe.
	if _, err := PullImage(ctx, PullImageRequest{Image: req.Workload.StroppyImage, RegistrySecret: req.RegistrySecret}); err != nil {
		return WaitDatabaseResult{}, err
	}
	dir, err := workspaceSubdir("managed-ydb-readiness")
	if err != nil {
		return WaitDatabaseResult{}, err
	}
	logPath := filepath.Join(dir, "readiness.log")
	log, err := os.Create(logPath)
	if err != nil {
		return WaitDatabaseResult{}, err
	}
	defer log.Close()
	result := waitDatabaseQueries(ctx, 2*time.Second, func(ctx context.Context, attempt int) (RunSegmentResult, error) {
		activity.RecordHeartbeat(ctx, attempt)
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		res, err := RunSegment(attemptCtx, databaseProbeRequest(req))
		if _, writeErr := fmt.Fprintf(log, "=== readiness attempt %d ===\n", attempt); writeErr != nil {
			return res, writeErr
		}
		if res.LogPath != "" {
			source, openErr := os.Open(res.LogPath)
			if openErr != nil {
				return res, openErr
			}
			_, copyErr := io.Copy(log, source)
			_ = source.Close()
			if copyErr != nil {
				return res, copyErr
			}
		}
		return res, err
	})
	result.LogPath = logPath
	return result, nil
}

// Only failures expected while a newly created database becomes discoverable
// retry. Invalid credentials, SQL/configuration errors and unknown failures stop.
func transientDatabaseProbe(message string) bool {
	if strings.Contains(message, "transport/NotFound") && strings.Contains(message, "Database not found") {
		return true
	}
	for _, marker := range []string{"transport/Unavailable", "transport/ResourceExhausted", "OVERLOADED", "failed to dial"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func waitDatabaseQueries(ctx context.Context, delay time.Duration, probe func(context.Context, int) (RunSegmentResult, error)) WaitDatabaseResult {
	result := WaitDatabaseResult{}
	for {
		if err := ctx.Err(); err != nil {
			result.Error = err.Error() + ": " + result.Error
			return result
		}
		result.QueryAttempts++
		res, err := probe(ctx, result.QueryAttempts)
		if res.ConfigPath != "" {
			result.ConfigPath = res.ConfigPath
		}
		if err != nil {
			result.Error = err.Error()
			return result
		}
		metrics := res.Result.Metrics
		iterations, hasIterations := metrics["iterations_total"]
		terminal, hasTerminal := metrics["terminal_errors_total"]
		if res.Result.Status == spec.SegmentCompleted && hasIterations && iterations.Value == 1 && hasTerminal && terminal.Value == 0 && metrics["failed_queries_total"].Value == 0 {
			result.Ready, result.Error = true, ""
			return result
		}
		result.Error = res.Result.Error
		if result.Error == "" {
			result.Error = "readiness SELECT did not report one successful error-free iteration"
			if res.LogPath != "" {
				result.Error += ": " + tail(lastLines(res.LogPath), errTailBytes)
			}
		}
		if !transientDatabaseProbe(result.Error) {
			return result
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
