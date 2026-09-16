package spec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	schemapb "github.com/gopherex/schemapb/go/schemapb"

	"github.com/stroppy-io/stroppy-cloud/pipelines/schemas"
)

// bake marshals v to JSON, decodes it into the generic form and bakes it
// through the named product schema — the round trip a server → pipeline
// hand-off makes. Any drift between the Go types and the schema shows up
// here as a validation error.
func bake(t *testing.T, id string, v any) {
	t.Helper()
	s, ok := schemas.ByID()[id]
	if !ok {
		t.Fatalf("schema %s not registered", id)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	eng, err := schemapb.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	_, res, err := eng.Bake(generic)
	if err != nil {
		t.Fatalf("%s: bake: %v", id, err)
	}
	if res.Blocking() {
		for _, e := range res.GetErrors() {
			t.Errorf("%s: %s: %s %s", id, e.GetPath(), e.GetCode(), e.GetMessage())
		}
		t.Fatalf("%s: value does not fit the schema", id)
	}
}

func sampleRun() Run {
	return Run{
		RunID:  "8f1c3f2a-0000-4000-8000-000000000001",
		Tenant: "acme",
		Provider: Provider{
			Kind:               ProviderYandex,
			Settings:           json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","network":{"kind":"create"}}`),
			CredentialsSecret:  "yc-sa-key",
			ProviderConfigName: "t-acme",
		},
		Network: Network{CIDR: "10.130.0.0/24", AllowPublicIPs: true, Ingress: []Ingress{{Port: 22, Proto: "tcp", CIDR: "0.0.0.0/0"}}},
		Machines: []Machine{
			{
				Name: "db-1", Role: "db", CPU: 8, MemoryGB: 32, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3",
				Disks: []Disk{{Name: "data", GB: 200, Type: "network-ssd", Mount: "/data"}}, Labels: map[string]string{"tier": "db"},
			},
			{Name: "runner-1", Role: "runner", CPU: 4, MemoryGB: 8, Image: "ubuntu-2404-lts", Location: "ru-central1-d", InstanceType: "standard-v3"},
		},
		Containers: []Container{{
			Name: "postgres", Role: "db", Machine: "db-1", Image: "docker.io/library/postgres:17",
			Env:         map[string]string{"POSTGRES_PASSWORD": "x"},
			Ports:       []Port{{Container: 5432, Host: 5432}},
			Files:       []File{{Path: "/etc/postgresql/postgresql.conf", Content: "shared_buffers = 8GB\n", Mode: "0644"}},
			Healthcheck: &Healthcheck{Cmd: []string{"pg_isready"}, Interval: Duration(10 * time.Second), Retries: 30},
			Restart:     "unless-stopped", Ulimits: map[string]int64{"nofile": 65536},
		}},
		HostPrep: []HostPrep{{Role: "db", Kind: HostPrepSysctl, Content: "vm.swappiness=1\n"}},
		Scrapes:  []Scrape{{Role: "db", URL: "http://127.0.0.1:9187/metrics", Job: "postgres"}},
		Flows:    []Flow{{FromRole: "runner", ToRole: "db", Protocol: "tcp", Port: 5432, Label: "postgres"}},
		Workload: Workload{
			RunnerRole: "runner", StroppyImage: "ghcr.io/stroppy-io/stroppy:v6.0.0.62",
			DriverType: "postgres", URL: "postgres://postgres:x@${ip:role:db}:5432/postgres?sslmode=disable",
			Driver:   map[string]any{"bulkSize": 5000, "pool": map[string]any{"maxConns": 64}},
			Segments: []json.RawMessage{json.RawMessage(`{"name":"load","workload":{"script":"tpcc/tx","scale_factor":10},"run":{"executor":"constant-vus","vus":8,"duration":"30s"}}`)},
			Baseline: &Baseline{Enabled: true, Tiers: []string{"noop", "wire"}, Quick: true},
		},
		Observability:      Observability{OTLPEndpoint: "http://otel.stroppy.io", OTLPHeaders: "Authorization=Bearer x", Labels: map[string]string{"stroppy_run_id": "x"}},
		Keep:               Duration(time.Hour),
		ResultExpectations: []string{"tps"},
	}
}

func TestRunFitsSchema(t *testing.T) { bake(t, "spec.run@1", sampleRun()) }

func TestResultFitsSchema(t *testing.T) {
	bake(t, "spec.result.run@1", Result{
		Metrics: map[string]MetricValue{"tps": {Value: 1234.5, Unit: "tx/s", Min: 1000, Max: 1300, Avg: 1200}},
		Segments: []SegmentResult{{
			Name: "load", Status: SegmentCompleted, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
			Metrics:    map[string]MetricValue{"iterations_total": {Value: 1234}, "iteration_duration_p99": {Value: 12.3, Unit: "ms"}},
			Errors:     &ErrorCounts{TerminalErrors: 1, FailedIterations: 1},
			ExitCode:   0,
			Compliance: json.RawMessage(`{"workload":"tpcc/tx","tpm_c":600}`),
		}},
		Artifacts: []string{"artifact/stroppy-raw"},
		Baseline: &BaselineResult{
			OK: true, Verdicts: []BaselineVerdict{{Check: "noop errors", Status: "ok", Detail: "no failed iterations"}},
			Report: json.RawMessage(`{"schema":1,"tiers":[]}`),
		},
		Summary: Summary{TPS: 1234.5, LatencyP99Ms: 12.3, Errors: 0, Duration: Duration(30 * time.Second)},
	})
}

func TestSuiteFitsSchema(t *testing.T) {
	bake(t, "spec.suite@1", Suite{
		SuiteRunID: "8f1c3f2a-0000-4000-8000-000000000002", Tenant: "acme",
		Cells:       []SuiteCell{{ID: "pg17-m", RunSpec: sampleRun()}},
		Concurrency: 2, Defaults: SuiteDefaults{ContinueOnFailure: true, Labels: map[string]string{"suite": "nightly"}},
	})
}

func TestServicePipelinesFitSchema(t *testing.T) {
	bake(t, "spec.result.quotas@1", QuotasResult{ObservedAt: time.Now().UTC(), UnavailableReason: "permission_denied", Scope: "cloud:b1g", Quotas: []Quota{}})
	bake(t, "spec.provider_verify@1", ProviderVerify{Provider: ProviderAWS, Settings: json.RawMessage(`{"region":"eu-central-1"}`), CredentialsSecret: "aws-keys", DryRun: true})
	bake(t, "spec.result.provider_verify@1", ProviderVerifyResult{OK: true, AccountID: "123", Scope: "folder", Permissions: []Permission{{Name: "compute.instances.create", Granted: true}}})
	bake(t, "spec.quotas@1", Quotas{Provider: ProviderYandex, Settings: json.RawMessage(`{"cloud_id":"b1glku4lgd6gabcdefgh","folder_id":"b1gia87mbaomkfvsleds","network":{"kind":"create"}}`), CredentialsSecret: "yc-sa-key", Location: "ru-central1-d"})
	bake(t, "spec.result.quotas@1", QuotasResult{ObservedAt: time.Now().UTC(), Quotas: []Quota{{Name: "compute.instanceCores.count", Limit: 32, Used: 8, Unit: "cores"}}})
}

func TestDecodeSegments(t *testing.T) {
	segs, err := DecodeSegments(sampleRun().Workload.Segments)
	if err != nil {
		t.Fatal(err)
	}
	if segs[0].Run.Executor != ExecutorConstantVUs || segs[0].Run.Duration.Std() != 30*time.Second || segs[0].Run.VUs != 8 {
		t.Fatalf("run decoded wrong: %+v", segs[0].Run)
	}
	if segs[0].Workload.Script != "tpcc/tx" || segs[0].Workload.Params["scale_factor"] != int64(10) {
		t.Fatalf("workload decoded wrong: %+v", segs[0].Workload)
	}
	if _, ok := segs[0].Workload.Params["script"]; ok {
		t.Fatal("discriminator leaked into params")
	}
	back, err := json.Marshal(segs[0].Workload)
	if err != nil || !strings.Contains(string(back), `"script":"tpcc/tx"`) {
		t.Fatalf("round trip: %s %v", back, err)
	}
	if _, err := DecodeSegments([]json.RawMessage{json.RawMessage(`{"name":"x","workload":{}}`)}); err == nil {
		t.Fatal("segment without script accepted")
	}
}
