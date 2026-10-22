package activities

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/graphene-ci/pipeline/pkg/obs"
	temporalactivity "go.temporal.io/sdk/activity"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

const NameWaitDatabase = "stroppy.database.ready"

// NameWaitReachable is the wire name of WaitReachable.
const NameWaitReachable = "stroppy.database.reachable"

// WaitReachableRequest is the database endpoint the workload connects to.
type WaitReachableRequest struct {
	Endpoint string `json:"endpoint"`
}

// WaitReachable waits from the runner until the deployed database accepts
// TCP connections. A container healthcheck runs on the database's own
// machine; the path from the runner (security group rules of a fresh
// network) can open later, and Stroppy gives its first connection only a
// few seconds. An endpoint without host:port (an unknown DSN form) is
// reported reachable.
func WaitReachable(ctx context.Context, req WaitReachableRequest) (WaitDatabaseResult, error) {
	ctx = bindHeartbeatContext(ctx)
	address := endpointAddress(req.Endpoint)
	if address == "" {
		return WaitDatabaseResult{Ready: true}, nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	result, err := waitDatabaseEndpoint(waitCtx, address, func(attempt int) {
		temporalactivity.RecordHeartbeat(ctx, attempt)
	})
	if err != nil {
		return result, fmt.Errorf("database endpoint %s is unreachable from runner after %d attempts: %w", address, result.Attempts, err)
	}
	result.Ready = true
	obs.Info(ctx, "database endpoint reachable", obs.Str("endpoint", address), obs.Int("attempts", result.Attempts))
	return result, nil
}

// endpointAddress is the host:port of a driver URL: a URL with a host
// (postgres://, grpc://) or a Go MySQL DSN (user:pass@tcp(host:port)/db).
func endpointAddress(endpoint string) string {
	if i := strings.Index(endpoint, "@tcp("); i >= 0 {
		rest := endpoint[i+len("@tcp("):]
		if j := strings.IndexByte(rest, ')'); j > 0 {
			return rest[:j]
		}
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Port() == "" {
		return ""
	}
	return u.Host
}

type WaitDatabaseRequest struct {
	Endpoint       string        `json:"endpoint"`
	RunID          string        `json:"run_id"`
	Workload       spec.Workload `json:"workload"`
	RegistrySecret string        `json:"registry_secret,omitempty"`
}
type WaitDatabaseResult struct {
	Address       string `json:"address"`
	Attempts      int    `json:"attempts"`
	QueryAttempts int    `json:"query_attempts"`
	Ready         bool   `json:"ready"`
	Error         string `json:"error,omitempty"`
	ConfigPath    string `json:"config_path,omitempty"`
	LogPath       string `json:"log_path,omitempty"`
}

// WaitDatabase checks the managed endpoint from the workload's runner. Provider
// readiness does not prove that its load balancer is accepting client traffic.
// The same Stroppy image and credentials must also complete a read-only SELECT.
func WaitDatabase(ctx context.Context, req WaitDatabaseRequest) (WaitDatabaseResult, error) {
	ctx = bindHeartbeatContext(ctx)
	u, err := url.Parse(req.Endpoint)
	if err != nil || u.User != nil || (u.Scheme != "grpc" && u.Scheme != "grpcs") {
		return WaitDatabaseResult{}, fmt.Errorf("managed database needs a grpc(s) endpoint without userinfo")
	}
	if _, _, err := net.SplitHostPort(u.Host); err != nil {
		return WaitDatabaseResult{}, fmt.Errorf("managed database endpoint needs host:port")
	}
	if req.Workload.DriverType != "ydb" || req.Workload.StroppyImage == "" {
		return WaitDatabaseResult{}, fmt.Errorf("managed readiness needs the YDB workload image and driver")
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	result, err := waitDatabaseEndpoint(waitCtx, u.Host, func(attempt int) {
		temporalactivity.RecordHeartbeat(ctx, attempt)
	})
	if err != nil {
		return result, fmt.Errorf("managed database endpoint %s is unreachable from runner after %d attempts: %w", u.Host, result.Attempts, err)
	}
	obs.Info(ctx, "managed database endpoint reachable", obs.Str("endpoint", u.Host), obs.Str("remote_address", result.Address))
	query, err := probeManagedDatabase(waitCtx, req)
	query.Address, query.Attempts = result.Address, result.Attempts
	return query, err
}

func waitDatabaseEndpoint(ctx context.Context, address string, heartbeat func(int)) (WaitDatabaseResult, error) {
	result := WaitDatabaseResult{}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Attempts++
		heartbeat(result.Attempts)
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			result.Address = conn.RemoteAddr().String()
			_ = conn.Close()
			return result, nil
		}
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, fmt.Errorf("%w (last dial error: %w)", ctx.Err(), err)
		case <-timer.C:
		}
	}
}
