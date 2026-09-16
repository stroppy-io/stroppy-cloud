package run

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/graphene-ci/pipeline/pkg/pipeline"
	"github.com/stroppy-io/stroppy-cloud/pipelines/internal/provision"
	"github.com/stroppy-io/stroppy-cloud/pipelines/spec"
)

func sampleInfra() (provision.Infra, map[string]provision.MachineInfo) {
	infra := provision.Infra{Machines: map[string]*provision.Machine{}, Order: []string{"db-1", "db-2", "runner-1"}}
	for _, n := range infra.Order {
		role := "db"
		if strings.HasPrefix(n, "runner") {
			role = "runner"
		}
		infra.Machines[n] = &provision.Machine{Spec: spec.Machine{Name: n, Role: role}}
	}
	infos := map[string]provision.MachineInfo{
		"db-1":     {ID: "1", PrivateIP: "10.0.0.11", PublicIP: "1.1.1.1"},
		"db-2":     {ID: "2", PrivateIP: "10.0.0.12"},
		"runner-1": {ID: "3", PrivateIP: "10.0.0.20"},
	}
	return infra, infos
}

func TestExpand(t *testing.T) {
	infra, infos := sampleInfra()
	a := NewAddresses(infra, infos)
	for in, want := range map[string]string{
		"postgres://u@${ip:db-1}:5432/db": "postgres://u@10.0.0.11:5432/db",
		"hosts=${ips:role:db}":            "hosts=10.0.0.11,10.0.0.12",
		"primary=${ip:role:db} pub=${public_ip:db-1} none=${public_ip:db-2}": "primary=10.0.0.11 pub=1.1.1.1 none=",
		"plain": "plain",
	} {
		got, err := a.Expand(in)
		if err != nil || got != want {
			t.Errorf("Expand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := a.Expand("${ip:nope}"); err == nil || !strings.Contains(err.Error(), "unknown machine") {
		t.Errorf("unknown machine not reported: %v", err)
	}
	if _, err := a.Expand("${ip:role:proxy}"); err == nil || !strings.Contains(err.Error(), "no machines of role") {
		t.Errorf("unknown role not reported: %v", err)
	}
	m, err := a.ExpandMap(map[string]string{"B": "${ip:db-2}", "A": "x"})
	if err != nil || m["B"] != "10.0.0.12" || m["A"] != "x" {
		t.Errorf("ExpandMap = %v, %v", m, err)
	}
}

func sampleRun() spec.Run {
	return spec.Run{
		RunID: "8f1c3f2a-0000-4000-8000-000000000001", Tenant: "acme",
		Provider: spec.Provider{Kind: spec.ProviderYandex, RegistrySecret: "reg"},
		Machines: []spec.Machine{{Name: "db-1", Role: "db"}, {Name: "runner-1", Role: "runner"}},
		Containers: []spec.Container{
			{
				Name: "postgres", Role: "db", Machine: "db-1", Image: "postgres:17",
				Env:   map[string]string{"PGDATA": "/data", "PEER": "${ip:role:runner}"},
				Ports: []spec.Port{{Container: 5432, Host: 5432}}, Scrape: "",
				Files:   []spec.File{{Path: "/etc/postgresql/postgresql.conf", Content: "x"}},
				Mounts:  []spec.Mount{{Source: "/data", Target: "/var/lib/postgresql/data"}},
				Ulimits: map[string]int64{"nofile": 65536}, Restart: "always",
			},
			{
				Name: "exporter", Role: "db", Machine: "db-1", Image: "pg-exporter", DependsOn: []string{"postgres"},
				Ports: []spec.Port{{Container: 9187, Host: 9187}}, Scrape: "/metrics",
			},
		},
		Scrapes: []spec.Scrape{{Role: "db", URL: "http://127.0.0.1:9187/metrics", Job: "postgres"}},
		Flows:   []spec.Flow{{FromRole: "runner", ToRole: "db", Protocol: "tcp", Port: 5432}, {FromRole: "db", External: "otel.stroppy.io", Protocol: "otlp", Port: 4318}},
		Workload: spec.Workload{
			RunnerRole: "runner", StroppyImage: "stroppy:6", DriverType: "postgres", URL: "postgres://u:p@${ip:role:db}:5432/db",
			Segments: []json.RawMessage{json.RawMessage(`{"name":"load","workload":{"script":"tpcc/tx"},"run":{"executor":"constant-vus","duration":"30s"}}`)},
		},
	}
}

func TestContainerSpec(t *testing.T) {
	run := sampleRun()
	infra, infos := sampleInfra()
	a := NewAddresses(infra, infos)
	files := map[string]string{"/etc/postgresql/postgresql.conf": "/ws/containers/postgres/etc/postgresql/postgresql.conf"}
	ds, err := containerSpec(run.Containers[0], run, files, a)
	if err != nil {
		t.Fatal(err)
	}
	if ds.Name != run.RunID+"-postgres" || ds.Config.Image != "postgres:17" || string(ds.Host.NetworkMode) != "host" {
		t.Errorf("spec = %+v", ds)
	}
	if strings.Join(ds.Config.Env, ",") != "PEER=10.0.0.20,PGDATA=/data" {
		t.Errorf("env = %v", ds.Config.Env)
	}
	if strings.Join(ds.Host.Binds, " ") != "/data:/var/lib/postgresql/data /ws/containers/postgres/etc/postgresql/postgresql.conf:/etc/postgresql/postgresql.conf:ro" {
		t.Errorf("binds = %v", ds.Host.Binds)
	}
	if len(ds.Host.Resources.Ulimits) != 1 || ds.Host.Resources.Ulimits[0].Soft != 65536 || string(ds.Host.RestartPolicy.Name) != "always" {
		t.Errorf("host = %+v", ds.Host)
	}
	// postgres has no scrape path but a spec-level scrape whose job matches.
	if ds.Scrape != "http://127.0.0.1:9187/metrics" {
		t.Errorf("scrape = %q", ds.Scrape)
	}
	ex, _ := containerSpec(run.Containers[1], run, nil, a)
	if ex.Scrape != "http://127.0.0.1:9187/metrics" {
		t.Errorf("exporter scrape = %q", ex.Scrape)
	}
	if _, err := containerSpec(spec.Container{Env: map[string]string{"X": "${ip:nope}"}}, run, nil, a); err == nil {
		t.Error("bad placeholder must fail")
	}
}

func TestContainerEntrypoint(t *testing.T) {
	r := sampleRun()
	infra, infos := sampleInfra()
	addrs := NewAddresses(infra, infos)
	c := r.Containers[0]
	c.Entrypoint = []string{"/bin/sh", "-c"}
	c.Cmd = []string{"exec /ydbd server --host ${ip:db-1}"}
	ds, err := containerSpec(c, r, nil, addrs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(ds.Config.Entrypoint, " ") != "/bin/sh -c" || ds.Config.Cmd[0] != "exec /ydbd server --host 10.0.0.11" {
		t.Fatalf("process contract lost: %+v", ds.Config)
	}
	c.Entrypoint = []string{"${ip:missing}"}
	if _, err = containerSpec(c, r, nil, addrs); err == nil {
		t.Fatal("unknown entrypoint placeholder accepted")
	}
	c.Entrypoint = nil
	ds, err = containerSpec(c, r, nil, addrs)
	if err != nil || ds.Config.Entrypoint != nil {
		t.Fatal("upstream image entrypoint overridden when omitted")
	}
}

func TestFlowOptionsAndDistinct(t *testing.T) {
	run := sampleRun()
	if n := len(flowOptions(run.Containers[0], run)); n != 1 { // db → external otlp
		t.Errorf("db flows = %d", n)
	}
	runner := spec.Container{Name: "stroppy", Role: "runner"}
	if n := len(flowOptions(runner, run)); n != 1 { // runner → db (first db container)
		t.Errorf("runner flows = %d", n)
	}
	if got := distinctImages(run.Containers); strings.Join(got, ",") != "pg-exporter,postgres:17" {
		t.Errorf("distinct = %v", got)
	}
}

func TestValidateAndExpectations(t *testing.T) {
	run := sampleRun()
	run.Workload.Segments = nil
	if err := validate(run); err == nil || !strings.Contains(err.Error(), "segment") {
		t.Errorf("no segments: %v", err)
	}
	run.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"load"}`)}
	if err := validate(run); err == nil {
		t.Error("segment without a script accepted")
	}
	run.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"load","workload":{"script":"simple"}}`)}
	if err := validate(run); err != nil {
		t.Errorf("valid run rejected: %v", err)
	}
	run.Workload.URL = ""
	if err := validate(run); err == nil {
		t.Error("missing url accepted")
	}
	run.Workload.URL = "noop://localhost"
	run.Workload.RunnerRole = "nope"
	if err := validate(run); err == nil {
		t.Error("missing runner role accepted")
	}
	res := spec.Result{Metrics: map[string]spec.MetricValue{"load.iterations_rate": {Value: 1}, "tps": {Value: 2}}}
	run.ResultExpectations = []string{"iterations_rate", "tps", "latency"}
	if got := missingExpectations(run, res); strings.Join(got, ",") != "latency" {
		t.Errorf("missing = %v", got)
	}
}

func TestSegmentBound(t *testing.T) {
	seg := spec.Segment{Run: spec.RunParams{Executor: spec.ExecutorConstantVUs, Duration: spec.Duration(time.Hour)}, Warmup: spec.Duration(10 * time.Minute)}
	if got := segmentBound(seg); got != time.Hour+30*time.Minute+10*time.Minute+segmentMargin {
		t.Errorf("bound = %s", got)
	}
	seg.Run = spec.RunParams{Executor: spec.ExecutorSharedIterations, Iterations: 10}
	if segmentBound(seg) != segmentIterationsBound+10*time.Minute {
		t.Error("iterations bound")
	}
}

func TestContainerIdentitySeparatesRuns(t *testing.T) {
	first := sampleRun()
	second := sampleRun()
	// Sharing the first eight UUID characters must not collapse two records.
	second.RunID = "8f1c3f2a-0000-4000-8000-000000000002"
	infra, infos := sampleInfra()
	addrs := NewAddresses(infra, infos)
	seen := map[string]bool{}
	for _, run := range []spec.Run{first, second} {
		for _, c := range run.Containers {
			d, err := containerSpec(c, run, nil, addrs)
			if err != nil {
				t.Fatal(err)
			}
			if seen[d.Name] {
				t.Fatalf("two runs share container identity %q", d.Name)
			}
			seen[d.Name] = true
			if d.Config.Labels["stroppy-container"] != c.Name {
				t.Fatal("logical container label changed")
			}
		}
	}
}

func TestContainerFlowsStayWithinRun(t *testing.T) {
	run := sampleRun()
	var options pipeline.ResourceOptions
	for _, opt := range flowOptions(spec.Container{Name: "runner", Role: "runner"}, run) {
		opt(&options)
	}
	if len(options.Flows) != 1 || options.Flows[0].To != "docker/"+run.RunID+"-postgres" {
		t.Fatalf("flow must target the database of this run: %+v", options.Flows)
	}
}

func TestValidateRejectsNestedWorkloadParamsBeforeProvisioning(t *testing.T) {
	run := sampleRun()
	run.Workload.Segments = []json.RawMessage{json.RawMessage(`{"name":"tx","workload":{"script":"tpcb/tx","params":{"scale_factor":1}}}`)}
	if err := validate(run); err == nil || !strings.Contains(err.Error(), "workload.params must be a scalar") {
		t.Fatalf("validation = %v", err)
	}
}
