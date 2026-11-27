package stroppycfg

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func segment() spec.Segment {
	return spec.Segment{
		Name: "load",
		Workload: spec.WorkloadParams{Script: "tpcc/tx", Params: map[string]any{
			"scale_factor": float64(10), "load_workers": float64(8), "pg_unlogged": true,
			"tx_isolation": nil, "sql_file": "tpcc/ydb_no_indexes",
		}},
		Run:         spec.RunParams{Executor: spec.ExecutorConstantVUs, VUs: 16, Duration: spec.Duration(90 * time.Second), QueryTimeout: spec.Duration(5 * time.Second)},
		Steps:       []string{"create_schema", "load_data"},
		ExtraParams: map[string]string{"warehouse-start": "3"},
		Thresholds:  spec.Thresholds{P99Ms: 50, ErrorRate: func() *float64 { v := 0.01; return &v }()},
		LogLevel:    "debug",
	}
}

func workload() spec.Workload {
	return spec.Workload{
		StroppyImage: "ghcr.io/stroppy-io/stroppy:v6.3.0.68",
		DriverType:   "postgres",
		URL:          "postgres://stroppy:x@${ip:role:db}:5432/bench?sslmode=disable",
		Driver:       map[string]any{"bulkSize": float64(5000), "pool": map[string]any{"maxConns": float64(64)}},
		CACert:       "-----BEGIN CERTIFICATE-----\n",
	}
}

func TestConfig(t *testing.T) {
	cfg, err := Config(Input{
		RunID: "r1", Segment: segment(), Workload: workload(), URL: "postgres://stroppy:x@10.0.0.5:5432/bench?sslmode=disable",
		OTLPEndpoint: "https://otel.stroppy.io", OTLPHeaders: "Authorization=Bearer x",
		Labels: map[string]string{"stroppy_run_id": "r1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg["version"] != "1" || cfg["script"] != "tpcc/tx" {
		t.Errorf("envelope = %v", cfg)
	}
	drv := cfg["drivers"].(map[string]any)["default"].(map[string]any)
	if drv["url"] != "postgres://stroppy:x@10.0.0.5:5432/bench?sslmode=disable" || drv["driverType"] != "postgres" {
		t.Errorf("driver = %v", drv)
	}
	if drv["bulkSize"] != float64(5000) || drv["caCertFile"] != "/workspace/ca.pem" {
		t.Errorf("driver options = %v", drv)
	}
	run := cfg["run"].(map[string]any)
	if run["executor"] != "constant-vus" || run["vus"] != int64(16) || run["duration"] != "1m30s" || run["queryTimeout"] != "5s" {
		t.Errorf("run = %v", run)
	}
	params := cfg["params"].(map[string]any)
	if params["scaleFactor"] != float64(10) || params["loadWorkers"] != float64(8) || params["pgUnlogged"] != true {
		t.Errorf("params = %v", params)
	}
	if _, ok := params["txIsolation"]; ok {
		t.Error("null parameter leaked into params")
	}
	if params["sqlFile"] != "tpcc/ydb_no_indexes" {
		t.Errorf("preset sql file rewritten: %v", params["sqlFile"])
	}
	if _, ok := cfg["env"]; ok {
		t.Error("legacy env map written")
	}
	if !reflect.DeepEqual(cfg["steps"], []string{"create_schema", "load_data"}) {
		t.Errorf("steps = %v", cfg["steps"])
	}
	global := cfg["global"].(map[string]any)
	exp := global["exporter"].(map[string]any)["otlpExport"].(map[string]any)
	if exp["otlpHttpEndpoint"] != "otel.stroppy.io" || exp["otlpHeaders"] != "Authorization=Bearer x" || exp["otlpEndpointInsecure"] == true {
		t.Errorf("otlp = %v", exp)
	}
	if global["logger"].(map[string]any)["logLevel"] != "debug" {
		t.Errorf("logger = %v", global["logger"])
	}
	if _, err := json.Marshal(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestFileParamsAndExecutors(t *testing.T) {
	seg := segment()
	seg.Workload.Params["sql_file"] = "tpcc/pico"
	params, err := Params(seg)
	if err != nil {
		t.Fatal(err)
	}
	// A file parameter is a stroppy preset id, passed through as written.
	if params["sqlFile"] != "tpcc/pico" {
		t.Errorf("preset id not passed through: %v", params["sqlFile"])
	}

	iter := spec.RunParams{Executor: spec.ExecutorSharedIterations, Iterations: 1000}
	run, err := RunObject(iter)
	if err != nil || run["iterations"] != int64(1000) || run["vus"] != int64(1) {
		t.Errorf("iterations run = %v %v", run, err)
	}
	if _, err := RunObject(spec.RunParams{Executor: spec.ExecutorConstantVUs}); err == nil {
		t.Error("constant-vus without duration accepted")
	}
	if _, err := RunObject(spec.RunParams{Executor: "ramping-vus", Duration: spec.Duration(time.Second)}); err == nil {
		t.Error("unknown executor accepted")
	}
	if _, err := Driver(Input{Workload: spec.Workload{DriverType: "postgres"}}); err == nil {
		t.Error("missing url accepted")
	}
}

func TestArgs(t *testing.T) {
	got := strings.Join(Args(segment()), " ")
	want := "run -f /workspace/stroppy-config.json --log-mode production --log-level debug --warehouse-start=3"
	if got != want {
		t.Errorf("Args = %q, want %q", got, want)
	}
	b := spec.Baseline{Enabled: true, Tiers: []string{"noop", "wire"}, Quick: true, VUs: 8, Rows: 100000, Duration: spec.Duration(2 * time.Second)}
	got = strings.Join(BaselineArgs(b), " ")
	want = "baseline --json --no-save --download always --quick --tiers noop,wire --vus 8 --rows 100000 --duration 2s"
	if got != want {
		t.Errorf("BaselineArgs = %q, want %q", got, want)
	}
}

func TestBound(t *testing.T) {
	t.Parallel()
	timed := segment() // 10 warehouses (1 GB) loaded, 90 s run, 10 min warm-up
	timed.Warmup = spec.Duration(10 * time.Minute)
	big := segment() // 1000 warehouses (100 GB) loaded, an hour of load
	big.Workload.Params["scale_factor"] = float64(1000)
	big.Run.Duration = spec.Duration(time.Hour)
	noLoad := big
	noLoad.Steps = []string{"workload"}
	skipped := big
	skipped.Steps, skipped.NoSteps = nil, []string{"load_data"}
	iterations := segment()
	iterations.Warmup = spec.Duration(10 * time.Minute)
	iterations.Run = spec.RunParams{Executor: spec.ExecutorSharedIterations, Iterations: 10}
	explicit := big
	explicit.Timeout = spec.Duration(2 * time.Hour)
	for _, tc := range []struct {
		name string
		seg  spec.Segment
		want time.Duration
	}{
		// 1.5 × duration + warm-up + margin + 3 min per loaded GB.
		{"timed segment loading 1 GB", timed, 135*time.Second + 10*time.Minute + 30*time.Minute + 3*time.Minute},
		// CockroachDB spent 55 min loading TPC-C 1000 WH and the old bound
		// (2 h) cut the hour of load that followed.
		{"timed segment loading 100 GB", big, 90*time.Minute + 30*time.Minute + 300*time.Minute},
		{"steps without the load", noLoad, 90*time.Minute + 30*time.Minute},
		{"load skipped by no_steps", skipped, 90*time.Minute + 30*time.Minute},
		{"iterations", iterations, 24*time.Hour + 10*time.Minute},
		{"explicit timeout wins", explicit, 2 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Bound(tc.seg, 30*time.Minute, 24*time.Hour); got != tc.want {
				t.Errorf("bound = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/run-output.log")
	if err != nil {
		t.Fatal(err)
	}
	s := ParseOutput(raw)
	if !s.Found {
		t.Fatal("summary not found")
	}
	if s.Metrics["iterations_total"].Value != 1860963 || s.Metrics["insert_rows_total"].Value != 100 {
		t.Errorf("counters = %v", s.Metrics)
	}
	h := s.Metrics["iteration_duration_p99"]
	if h.Value != 0.010 || h.Unit != "ms" || s.Metrics["iteration_duration_count"].Value != 1860963 || s.Metrics["iteration_duration_avg"].Value != 0.001 {
		t.Errorf("histogram = %v", s.Metrics)
	}
	if s.Errors == nil || s.Errors.TerminalErrors != 1860963 || s.Errors.FailedIterations != 1860963 || s.Errors.FailedQueries != 0 || s.Errors.RetryAttempts != 0 {
		t.Errorf("errors = %+v", s.Errors)
	}
	var rep struct {
		TpmC float64 `json:"tpm_c"`
	}
	if err := json.Unmarshal(s.Compliance, &rep); err != nil || rep.TpmC != 1234.5 {
		t.Errorf("compliance = %s %v", s.Compliance, err)
	}
	if rate := ErrorRate(s); rate != 1 {
		t.Errorf("error rate = %v", rate)
	}
	if v := ThresholdViolation(segment().Thresholds, s); !strings.HasPrefix(v, "error rate") {
		t.Errorf("threshold = %q", v)
	}
	clean := ParseOutput([]byte("noise\n=== bench summary ===\n  iterations_total                         10.000\n  iteration_duration                       count=10 avg=1.500 p(50)~=1.000 p(90)~=2.000 p(95)~=2.500 p(99)~=3.000\n"))
	if clean.Errors != nil || clean.Metrics["iteration_duration_p95"].Value != 2.5 {
		t.Errorf("clean = %+v", clean)
	}
	if v := ThresholdViolation(spec.Thresholds{P99Ms: 2}, clean); !strings.HasPrefix(v, "iteration_duration p99") {
		t.Errorf("p99 threshold = %q", v)
	}
	if ParseOutput([]byte("nothing here")).Found {
		t.Error("summary found in noise")
	}
}

func TestHeadlineAndMerge(t *testing.T) {
	start := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	segs := []spec.SegmentResult{
		{Name: "boot"},
		{
			Name: "steady", StartedAt: start, FinishedAt: start.Add(100 * time.Second),
			Metrics: map[string]spec.MetricValue{
				"iterations_total": {Value: 5000}, "iteration_duration_p50": {Value: 10}, "iteration_duration_p95": {Value: 25.5}, "iteration_duration_p99": {Value: 40},
			},
			Errors: &spec.ErrorCounts{FailedIterations: 7},
		},
	}
	h := Headline(segs)
	if h.TPS != 0 || h.LatencyP99Ms != 40 || h.Errors != 7 || h.Duration.Std() != 100*time.Second {
		t.Errorf("headline = %+v", h)
	}
	segs[1].Compliance = json.RawMessage(`{"tpm_c": 600}`)
	if h := Headline(segs); h.TPS != 0 {
		t.Errorf("tpmC must not be converted to TPS: %+v", h)
	}
	segs[1].Metrics["tps"] = spec.MetricValue{Value: 123.456, Unit: "transactions/s"}
	segs[1].FinishedAt = start.Add(time.Hour)
	if h := Headline(segs); h.TPS != 123.456 {
		t.Errorf("Stroppy TPS must be copied independently of activity time: %+v", h)
	}
	merged := MergeMetrics(segs)
	if _, ok := merged["steady.iterations_total"]; !ok {
		t.Errorf("merged = %v", merged)
	}
}

func TestParseBaseline(t *testing.T) {
	raw, err := os.ReadFile("testdata/baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ParseBaseline(append([]byte("downloading pg-noop…\n"), raw...))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || len(res.Verdicts) != 2 || res.Verdicts[1].Status != "warn" {
		t.Errorf("baseline = %+v", res)
	}
	var doc struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(res.Report, &doc); err != nil || doc.Schema != 1 {
		t.Errorf("report = %v %s", err, res.Report)
	}
	failed, _ := ParseBaseline([]byte(`{"schema":1,"verdicts":[{"check":"x","status":"fail"}]}`))
	if failed.OK {
		t.Error("failed verdict reported ok")
	}
	if _, err := ParseBaseline([]byte("no json")); err == nil {
		t.Error("garbage accepted")
	}
}

func TestExitStatus(t *testing.T) {
	for code, canceled := range map[int]bool{0: false, 1: false, 2: true, 130: true, 143: true} {
		if c, _ := ExitStatus(code); c != canceled {
			t.Errorf("exit %d canceled=%v", code, c)
		}
	}
}

func TestConfigSeparatesSegmentsWithoutMutatingLabels(t *testing.T) {
	labels := map[string]string{"graphene.namespace": "test", "stroppy.segment": "spoofed"}
	in := Input{RunID: "run", Labels: labels, Workload: workload(), Segment: segment()}
	in.Segment.Name = "first"
	first, err := Config(in)
	if err != nil {
		t.Fatal(err)
	}
	in.Segment.Name = "second"
	second, err := Config(in)
	if err != nil {
		t.Fatal(err)
	}
	for name, cfg := range map[string]map[string]any{"first": first, "second": second} {
		metadata := cfg["global"].(map[string]any)["metadata"].(map[string]string)
		if metadata["stroppy.segment"] != name || metadata["graphene.namespace"] != "test" {
			t.Fatalf("metadata = %v", metadata)
		}
	}
	if labels["stroppy.segment"] != "spoofed" {
		t.Fatal("mutated caller labels")
	}
}

// Strict TPC-C writes its deferred Delivery record into the workspace under
// the configured name; stress and other scripts write none.
func TestDeliveryResults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		params map[string]any
		script string
		want   string
	}{
		{"strict default name", map[string]any{"terminal_model": "strict"}, "tpcc/tx", "tpcc-delivery.jsonl"},
		{"strict own name", map[string]any{"terminal_model": "strict", "delivery_results": "d-1.jsonl"}, "tpcc/procs", "d-1.jsonl"},
		{"stress", map[string]any{"terminal_model": "stress", "delivery_results": "d-1.jsonl"}, "tpcc/tx", ""},
		{"no model", map[string]any{}, "tpcc/tx", ""},
		{"other script", map[string]any{"terminal_model": "strict"}, "tpcb/tx", ""},
		{"path outside the workspace", map[string]any{"terminal_model": "strict", "delivery_results": "../../etc/passwd"}, "tpcc/tx", ""},
		{"absolute path", map[string]any{"terminal_model": "strict", "delivery_results": "/etc/passwd"}, "tpcc/tx", ""},
		{"dot name", map[string]any{"terminal_model": "strict", "delivery_results": ".."}, "tpcc/tx", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seg := spec.Segment{Workload: spec.WorkloadParams{Script: tc.script, Params: tc.params}}
			if got := DeliveryResults(seg); got != tc.want {
				t.Errorf("DeliveryResults = %q, want %q", got, tc.want)
			}
		})
	}
}

// A named TPC-H mode runs one worker per stream: the config leaves workers
// and iterations to it, keeping the executor and timeouts.
func TestConfigLeavesModeSchedule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		mode string
		want map[string]any
	}{
		{"full", "full", map[string]any{"executor": "shared-iterations", "queryTimeout": "1h0m0s"}},
		{"legacy", "legacy", map[string]any{"executor": "shared-iterations", "vus": 1, "iterations": int64(1), "queryTimeout": "1h0m0s"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seg := spec.Segment{Name: "q", Workload: spec.WorkloadParams{Script: "tpch/tx", Params: map[string]any{"mode": tc.mode, "refresh_functions": false}},
				Run: spec.RunParams{Executor: spec.ExecutorSharedIterations, VUs: 1, Iterations: 1, QueryTimeout: spec.Duration(time.Hour)}}
			cfg, err := Config(Input{Segment: seg, Workload: spec.Workload{DriverType: "postgres", URL: "postgres://x"}})
			if err != nil {
				t.Fatal(err)
			}
			params, _ := json.Marshal(cfg["params"])
			if strings.Contains(string(params), "refreshFunctions") != (tc.mode == "legacy") {
				t.Errorf("params %s: refreshFunctions belongs to legacy only", params)
			}
			got, _ := json.Marshal(cfg["run"])
			want, _ := json.Marshal(tc.want)
			if string(got) != string(want) {
				t.Errorf("run %s, want %s", got, want)
			}
		})
	}
}
