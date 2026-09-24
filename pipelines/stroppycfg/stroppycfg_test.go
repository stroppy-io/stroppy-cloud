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
		StroppyImage: "ghcr.io/stroppy-io/stroppy:v6.0.0.62",
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
	drv := cfg["drivers"].(map[string]any)["0"].(map[string]any)
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
	seg := segment()
	seg.Warmup = spec.Duration(10 * time.Minute)
	if got := Bound(seg, 30*time.Minute, 24*time.Hour); got != 90*time.Second+45*time.Second+10*time.Minute+30*time.Minute {
		t.Errorf("bound = %s", got)
	}
	seg.Run = spec.RunParams{Executor: spec.ExecutorSharedIterations, Iterations: 10}
	if got := Bound(seg, 30*time.Minute, 24*time.Hour); got != 24*time.Hour+10*time.Minute {
		t.Errorf("iterations bound = %s", got)
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
