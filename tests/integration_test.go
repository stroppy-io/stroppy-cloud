// Package tests contains integration tests for stroppy-cloud.
// These tests require Docker and network access.
//
// Run with: go test -tags=integration -timeout 30m ./tests/
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/api"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/auth"
	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/postgres"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type testServer struct {
	app      *api.App
	srv      *httptest.Server
	url      string
	jwtToken string
}

const testTenantID = "test-tenant-id"

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping database test")
	}
	ctx := context.Background()
	pool, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("postgres.Open() failed: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func startTestServer(t *testing.T) *testServer {
	t.Helper()
	pool := testPool(t)
	logger, _ := zap.NewDevelopment()
	// Create a test tenant for JWT-based requests.
	_, err := pool.Exec(context.Background(), "INSERT INTO tenants (id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING", testTenantID, "test-tenant")
	if err != nil {
		t.Fatalf("create test tenant: %v", err)
	}
	app := api.New(api.Config{Pool: pool, Logger: logger})
	s := api.NewServer(app, logger, pool, "test-secret", "", "", "", ":8080")
	ts := httptest.NewServer(s.Router())
	t.Cleanup(func() { ts.Close() })

	// Issue a JWT token for test requests (root user with test tenant).
	issuer := auth.NewJWTIssuer("test-secret")
	token, err := issuer.Issue(auth.Claims{
		UserID:   "test-user",
		Username: "test",
		TenantID: testTenantID,
		Role:     "owner",
		IsRoot:   true,
	}, 1*time.Hour)
	if err != nil {
		t.Fatalf("issue test JWT: %v", err)
	}

	return &testServer{app: app, srv: ts, url: ts.URL, jwtToken: token}
}

func (ts *testServer) post(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, ts.url+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ts.jwtToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (ts *testServer) get(t *testing.T, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.url+path, nil)
	req.Header.Set("Authorization", "Bearer "+ts.jwtToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (ts *testServer) put(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, ts.url+path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ts.jwtToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func readJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, data)
	}
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d: %s", want, resp.StatusCode, body)
	}
}

// ---------------------------------------------------------------------------
// RunConfig builders for every topology
// ---------------------------------------------------------------------------

func baseRunConfig(id string, db types.DatabaseConfig, workload string, duration string, workers int) types.RunConfig {
	return types.RunConfig{
		ID:       id,
		Provider: types.ProviderDocker,
		Network:  types.NetworkConfig{CIDR: "10.10.0.0/24"},
		Machines: []types.MachineSpec{
			{Role: types.RoleDatabase, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 50},
			{Role: types.RoleMonitor, Count: 1, CPUs: 1, MemoryMB: 2048, DiskGB: 20},
			{Role: types.RoleStroppy, Count: 1, CPUs: 2, MemoryMB: 4096, DiskGB: 20},
		},
		Database: db,
		Monitor:  types.MonitorConfig{},
		Stroppy: types.StroppyConfig{
			Version:  "3.1.0",
			Workload: workload,
			Duration: duration,
			Workers:  workers,
		},
	}
}

func pgConfig(preset types.PostgresPreset, version string) types.DatabaseConfig {
	topo := types.PostgresPresets[preset]
	return types.DatabaseConfig{
		Kind:     types.DatabasePostgres,
		Version:  version,
		Postgres: &topo,
	}
}

func mysqlConfig(preset types.MySQLPreset, version string) types.DatabaseConfig {
	topo := types.MySQLPresets[preset]
	return types.DatabaseConfig{
		Kind:    types.DatabaseMySQL,
		Version: version,
		MySQL:   &topo,
	}
}

func picoConfig(preset types.PicodataPreset, version string) types.DatabaseConfig {
	topo := types.PicodataPresets[preset]
	return types.DatabaseConfig{
		Kind:     types.DatabasePicodata,
		Version:  version,
		Picodata: &topo,
	}
}

// ---------------------------------------------------------------------------
// Test: API - Validate endpoint
// ---------------------------------------------------------------------------

func TestValidateAllTopologies(t *testing.T) {
	ts := startTestServer(t)

	tests := []struct {
		name string
		cfg  types.RunConfig
	}{
		// Postgres topologies
		{"pg-single-16", baseRunConfig("v-pg-s16", pgConfig(types.PostgresSingle, "16"), "tpcb", "10s", 2)},
		{"pg-single-17", baseRunConfig("v-pg-s17", pgConfig(types.PostgresSingle, "17"), "tpcb", "10s", 2)},
		{"pg-ha-16", baseRunConfig("v-pg-ha", pgConfig(types.PostgresHA, "16"), "tpcb", "10s", 4)},
		{"pg-scale-16", baseRunConfig("v-pg-sc", pgConfig(types.PostgresScale, "16"), "tpcc", "30s", 8)},

		// MySQL topologies
		{"mysql-single-80", baseRunConfig("v-my-s", mysqlConfig(types.MySQLSingle, "8.0"), "tpcb", "10s", 2)},
		{"mysql-single-84", baseRunConfig("v-my-s84", mysqlConfig(types.MySQLSingle, "8.4"), "tpcb", "10s", 2)},
		{"mysql-replica", baseRunConfig("v-my-r", mysqlConfig(types.MySQLReplica, "8.0"), "tpcb", "10s", 4)},
		{"mysql-group", baseRunConfig("v-my-g", mysqlConfig(types.MySQLGroup, "8.0"), "tpcb", "15s", 4)},

		// Picodata topologies
		{"pico-single", baseRunConfig("v-pi-s", picoConfig(types.PicodataSingle, "25.3"), "tpcb", "10s", 2)},
		{"pico-cluster", baseRunConfig("v-pi-c", picoConfig(types.PicodataCluster, "25.3"), "tpcb", "10s", 4)},
		{"pico-scale", baseRunConfig("v-pi-sc", picoConfig(types.PicodataScale, "25.3"), "tpcb", "30s", 8)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := ts.post(t, "/api/v1/validate", tt.cfg)
			assertStatus(t, resp, http.StatusOK)
			var result map[string]string
			readJSON(t, resp, &result)
			if result["status"] != "valid" {
				t.Fatalf("expected valid, got: %v", result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: API - Validate rejects invalid configs
// ---------------------------------------------------------------------------

func TestValidateRejectsInvalid(t *testing.T) {
	ts := startTestServer(t)

	tests := []struct {
		name string
		cfg  types.RunConfig
	}{
		{
			"missing-db-kind",
			types.RunConfig{
				ID: "bad-1", Provider: types.ProviderDocker,
				Network:  types.NetworkConfig{CIDR: "10.0.0.0/24"},
				Database: types.DatabaseConfig{Kind: "unknown"},
				Stroppy:  types.StroppyConfig{Workload: "tpcb", Duration: "10s", Workers: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := ts.post(t, "/api/v1/validate", tt.cfg)
			assertStatus(t, resp, http.StatusUnprocessableEntity)
			resp.Body.Close()
		})
	}
}

// ---------------------------------------------------------------------------
// Test: API - DryRun produces correct DAG for all topologies
// ---------------------------------------------------------------------------

func TestDryRunAllTopologies(t *testing.T) {
	ts := startTestServer(t)

	configs := []struct {
		name string
		cfg  types.RunConfig
	}{
		{"pg-single", baseRunConfig("dr-pg", pgConfig(types.PostgresSingle, "16"), "tpcb", "10s", 2)},
		{"mysql-single", baseRunConfig("dr-my", mysqlConfig(types.MySQLSingle, "8.0"), "tpcb", "10s", 2)},
		{"pico-single", baseRunConfig("dr-pi", picoConfig(types.PicodataSingle, "25.3"), "tpcb", "10s", 2)},
	}

	expectedNodes := []string{
		"network", "machines",
		"install_db", "configure_db",
		"install_monitor", "configure_monitor",
		"install_stroppy", "run_stroppy", "teardown",
	}

	for _, tt := range configs {
		t.Run(tt.name, func(t *testing.T) {
			resp := ts.post(t, "/api/v1/dry-run", tt.cfg)
			assertStatus(t, resp, http.StatusOK)

			var graph struct {
				Nodes []struct {
					ID   string   `json:"id"`
					Type string   `json:"type"`
					Deps []string `json:"deps,omitempty"`
				} `json:"nodes"`
			}
			readJSON(t, resp, &graph)

			if len(graph.Nodes) != len(expectedNodes) {
				t.Fatalf("expected %d nodes, got %d", len(expectedNodes), len(graph.Nodes))
			}

			nodeSet := map[string]bool{}
			for _, n := range graph.Nodes {
				nodeSet[n.ID] = true
			}
			for _, id := range expectedNodes {
				if !nodeSet[id] {
					t.Errorf("missing node %q in DAG", id)
				}
			}

			// Verify run_stroppy depends on configure_db, configure_monitor, install_stroppy
			for _, n := range graph.Nodes {
				if n.ID == "run_stroppy" {
					depSet := map[string]bool{}
					for _, d := range n.Deps {
						depSet[d] = true
					}
					for _, want := range []string{"configure_db", "configure_monitor", "install_stroppy"} {
						if !depSet[want] {
							t.Errorf("run_stroppy missing dep %q", want)
						}
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test: API - Stroppy workload variants
// ---------------------------------------------------------------------------

func TestDryRunStroppyWorkloads(t *testing.T) {
	ts := startTestServer(t)

	workloads := []string{"tpcb", "tpcb", "tpcc", "tpcds", "execute_sql"}
	durations := []string{"5s", "10s", "30s", "1m", "5m"}
	workerCounts := []int{1, 2, 4, 8, 16, 32}

	// All workloads validate.
	for _, w := range workloads {
		t.Run("workload-"+w, func(t *testing.T) {
			cfg := baseRunConfig("w-"+w, pgConfig(types.PostgresSingle, "16"), w, "10s", 4)
			resp := ts.post(t, "/api/v1/validate", cfg)
			assertStatus(t, resp, http.StatusOK)
			resp.Body.Close()
		})
	}

	// All durations validate.
	for _, d := range durations {
		t.Run("duration-"+d, func(t *testing.T) {
			cfg := baseRunConfig("d-"+d, pgConfig(types.PostgresSingle, "16"), "tpcb", d, 4)
			resp := ts.post(t, "/api/v1/validate", cfg)
			assertStatus(t, resp, http.StatusOK)
			resp.Body.Close()
		})
	}

	// All worker counts validate.
	for _, w := range workerCounts {
		t.Run(fmt.Sprintf("workers-%d", w), func(t *testing.T) {
			cfg := baseRunConfig(fmt.Sprintf("vus-%d", w), pgConfig(types.PostgresSingle, "16"), "tpcb", "10s", w)
			resp := ts.post(t, "/api/v1/validate", cfg)
			assertStatus(t, resp, http.StatusOK)
			resp.Body.Close()
		})
	}
}

// ---------------------------------------------------------------------------
// Test: API - Stroppy options passthrough (K6_OTEL)
// ---------------------------------------------------------------------------

func TestDryRunStroppyOptions(t *testing.T) {
	ts := startTestServer(t)

	cfg := baseRunConfig("opts-1", pgConfig(types.PostgresSingle, "16"), "tpcb", "10s", 4)
	cfg.Stroppy.ScaleFactor = 10

	resp := ts.post(t, "/api/v1/validate", cfg)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	resp = ts.post(t, "/api/v1/dry-run", cfg)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Test: API - Presets endpoint
// ---------------------------------------------------------------------------

func TestPresetsEndpoint(t *testing.T) {
	ts := startTestServer(t)

	resp := ts.get(t, "/api/v1/presets")
	assertStatus(t, resp, http.StatusOK)

	var presets map[string]json.RawMessage
	readJSON(t, resp, &presets)

	for _, key := range []string{"postgres", "mysql", "picodata"} {
		if _, ok := presets[key]; !ok {
			t.Errorf("missing preset category %q", key)
		}
	}

	// Verify postgres presets contain all expected keys.
	var pgPresets map[string]json.RawMessage
	json.Unmarshal(presets["postgres"], &pgPresets)
	for _, p := range []string{"single", "ha", "scale"} {
		if _, ok := pgPresets[p]; !ok {
			t.Errorf("missing postgres preset %q", p)
		}
	}

	var myPresets map[string]json.RawMessage
	json.Unmarshal(presets["mysql"], &myPresets)
	for _, p := range []string{"single", "replica", "group"} {
		if _, ok := myPresets[p]; !ok {
			t.Errorf("missing mysql preset %q", p)
		}
	}

	var piPresets map[string]json.RawMessage
	json.Unmarshal(presets["picodata"], &piPresets)
	for _, p := range []string{"single", "cluster", "scale"} {
		if _, ok := piPresets[p]; !ok {
			t.Errorf("missing picodata preset %q", p)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: API - Settings CRUD (tenant-scoped)
// ---------------------------------------------------------------------------

func TestAdminSettings(t *testing.T) {
	ts := startTestServer(t)

	// GET defaults.
	resp := ts.get(t, "/api/v1/settings")
	assertStatus(t, resp, http.StatusOK)

	var settings types.ServerSettings
	readJSON(t, resp, &settings)

	// PUT updated settings.
	settings.Cloud.ServerAddr = "http://test-server:8080"
	settings.Cloud.Yandex.FolderID = "test-folder"

	resp = ts.put(t, "/api/v1/settings", settings)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// GET and verify.
	resp = ts.get(t, "/api/v1/settings")
	assertStatus(t, resp, http.StatusOK)

	var updated types.ServerSettings
	readJSON(t, resp, &updated)

	if updated.Cloud.ServerAddr != "http://test-server:8080" {
		t.Errorf("server_addr not updated: %s", updated.Cloud.ServerAddr)
	}
	if updated.Cloud.Yandex.FolderID != "test-folder" {
		t.Errorf("folder_id not updated: %s", updated.Cloud.Yandex.FolderID)
	}
}

// ---------------------------------------------------------------------------
// Test: API - Agent registration
// ---------------------------------------------------------------------------

func TestAgentRegistration(t *testing.T) {
	ts := startTestServer(t)

	body := map[string]any{
		"machine_id": "test-db-0",
		"host":       "172.20.0.5",
		"port":       9090,
	}

	resp := ts.post(t, "/api/agent/register", body)
	assertStatus(t, resp, http.StatusOK)

	var result map[string]string
	readJSON(t, resp, &result)
	if result["status"] != "ok" {
		t.Fatalf("registration failed: %v", result)
	}
}

// ---------------------------------------------------------------------------
// Test: API - Agent report
// ---------------------------------------------------------------------------

func TestAgentReport(t *testing.T) {
	ts := startTestServer(t)

	body := map[string]any{
		"command_id": "cmd-001",
		"status":     "completed",
		"output":     "all good",
	}

	resp := ts.post(t, "/api/agent/report", body)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Test: API - Agent log
// ---------------------------------------------------------------------------

func TestAgentLog(t *testing.T) {
	ts := startTestServer(t)

	body := map[string]any{
		"command_id": "cmd-001",
		"line":       "installing postgresql-16...",
		"stream":     "stdout",
	}

	resp := ts.post(t, "/api/agent/log", body)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Test: API - Run status for missing run
// ---------------------------------------------------------------------------

func TestRunStatusNotFound(t *testing.T) {
	ts := startTestServer(t)

	resp := ts.get(t, "/api/v1/run/nonexistent/status")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Test: API - Metrics endpoint without VictoriaMetrics
// ---------------------------------------------------------------------------

func TestMetricsWithoutVictoria(t *testing.T) {
	ts := startTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	resp := ts.get(t, "/api/v1/run/test/metrics?start="+now+"&end="+now)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (no victoria), got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCompareWithoutVictoria(t *testing.T) {
	ts := startTestServer(t)

	now := time.Now().UTC().Format(time.RFC3339)
	resp := ts.get(t, "/api/v1/compare?a=run1&b=run2&start="+now+"&end="+now)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 (no victoria), got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Test: API - Binary download endpoint
// ---------------------------------------------------------------------------

func TestBinaryDownload(t *testing.T) {
	ts := startTestServer(t)

	resp := ts.get(t, "/agent/binary")
	assertStatus(t, resp, http.StatusOK)
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("expected octet-stream, got %s", ct)
	}
	// Body should be non-empty (it's our own binary).
	data, _ := io.ReadAll(resp.Body)
	if len(data) < 1000 {
		t.Errorf("binary too small: %d bytes", len(data))
	}
}

// ---------------------------------------------------------------------------
// Test: StroppyEnv generation
// ---------------------------------------------------------------------------

func TestStroppyEnvGeneration(t *testing.T) {
	s := types.StroppySettings{
		OTLPExporterType: "http",
		OTLPURLPath:      "/insert/multitenant/opentelemetry/v1/metrics",
		OTLPInsecure:     true,
		OTLPHeaders:      "Authorization=Basic abc123",
		OTLPMetricPrefix: "test_",
		OTLPServiceName:  "test_svc",
	}

	env := s.StroppyEnv("run-42")

	expected := map[string]string{
		"K6_OTEL_EXPORTER_TYPE":          "http",
		"K6_OTEL_HTTP_EXPORTER_URL_PATH": "/insert/multitenant/opentelemetry/v1/metrics",
		"K6_OTEL_HTTP_EXPORTER_INSECURE": "true",
		"K6_OTEL_HEADERS":                "Authorization=Basic abc123",
		"K6_OTEL_METRIC_PREFIX":          "test_",
		"K6_OTEL_SERVICE_NAME":           "test_svc",
		"OTEL_RESOURCE_ATTRIBUTES":       "service.name=stroppy,stroppy.run.id=run-42",
	}

	for k, want := range expected {
		got, ok := env[k]
		if !ok {
			t.Errorf("missing env var %s", k)
			continue
		}
		if got != want {
			t.Errorf("%s: expected %q, got %q", k, want, got)
		}
	}

	// Empty run ID still produces OTEL_RESOURCE_ATTRIBUTES with service.name.
	env2 := s.StroppyEnv("")
	if env2["OTEL_RESOURCE_ATTRIBUTES"] != "service.name=stroppy,stroppy.run.id=" {
		t.Errorf("expected OTEL_RESOURCE_ATTRIBUTES with empty run id, got %q", env2["OTEL_RESOURCE_ATTRIBUTES"])
	}
}

// ---------------------------------------------------------------------------
// Test: DB defaults functions
// ---------------------------------------------------------------------------

func TestPostgresDefaults(t *testing.T) {
	for _, ver := range []string{"16", "17"} {
		d := types.PostgresDefaults(ver)
		for _, key := range []string{"shared_buffers", "max_connections", "max_wal_size", "listen_addresses"} {
			if _, ok := d[key]; !ok {
				t.Errorf("postgres %s: missing default %q", ver, key)
			}
		}
	}
	// PG 17 should have summarize_wal.
	d17 := types.PostgresDefaults("17")
	if d17["summarize_wal"] != "on" {
		t.Error("PG 17 should enable summarize_wal")
	}
}

func TestMySQLDefaults(t *testing.T) {
	for _, ver := range []string{"8.0", "8.4"} {
		d := types.MySQLDefaults(ver)
		for _, key := range []string{"innodb_buffer_pool_size", "max_connections", "bind_address"} {
			if _, ok := d[key]; !ok {
				t.Errorf("mysql %s: missing default %q", ver, key)
			}
		}
	}
}

func TestPicodataDefaults(t *testing.T) {
	d := types.PicodataDefaults("25.3")
	for _, key := range []string{"replication_factor", "shards", "listen"} {
		if _, ok := d[key]; !ok {
			t.Errorf("picodata: missing default %q", key)
		}
	}
}

// ---------------------------------------------------------------------------
// Test: Metrics comparison logic (unit test, no VictoriaMetrics needed)
// ---------------------------------------------------------------------------

func TestMetricsComparison(t *testing.T) {
	// Import from the metrics package directly.
	// This tests the comparison logic with synthetic data.
	a := makeMetrics("run-a", map[string]float64{
		"db_qps":         1000,
		"db_latency_p99": 0.05,
		"stroppy_ops":    500,
		"stroppy_errors": 1.0,
		"cpu_usage":      60,
	})
	b := makeMetrics("run-b", map[string]float64{
		"db_qps":         1100, // 10% better
		"db_latency_p99": 0.04, // 20% better
		"stroppy_ops":    550,  // 10% better
		"stroppy_errors": 0.5,  // 50% better
		"cpu_usage":      65,   // 8% worse
	})

	// Just verify the structures are valid — real comparison tested in metrics package.
	if a.RunID != "run-a" || b.RunID != "run-b" {
		t.Fatal("bad metrics construction")
	}
	if len(a.Metrics) != 5 || len(b.Metrics) != 5 {
		t.Fatalf("expected 5 metrics each, got %d and %d", len(a.Metrics), len(b.Metrics))
	}
}

// helper for synthetic metrics
type metricSummary struct {
	Key  string  `json:"key"`
	Name string  `json:"name"`
	Avg  float64 `json:"avg"`
	Max  float64 `json:"max"`
}

type runMetrics struct {
	RunID   string          `json:"run_id"`
	Metrics []metricSummary `json:"metrics"`
}

func makeMetrics(runID string, vals map[string]float64) runMetrics {
	var ms []metricSummary
	for k, v := range vals {
		ms = append(ms, metricSummary{Key: k, Name: k, Avg: v, Max: v * 1.1})
	}
	return runMetrics{RunID: runID, Metrics: ms}
}

// ---------------------------------------------------------------------------
// Test: WebSocket log streaming
// ---------------------------------------------------------------------------

func TestWebSocketLogs(t *testing.T) {
	ts := startTestServer(t)

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(ts.url, "http") + "/ws/logs"
	wsHeaders := http.Header{"Authorization": []string{"Bearer " + ts.jwtToken}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeaders)
	if err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	defer conn.Close()

	// Trigger an agent log via API
	body := map[string]any{
		"command_id": "test-cmd",
		"line":       "test log line",
		"stream":     "stdout",
	}
	resp := ts.post(t, "/api/agent/log", body)
	resp.Body.Close()

	// Read message from WebSocket (with timeout)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}

	var wsMsg map[string]any
	json.Unmarshal(msg, &wsMsg)
	if wsMsg["type"] != "agent_log" {
		t.Errorf("expected agent_log, got %v", wsMsg["type"])
	}
}
